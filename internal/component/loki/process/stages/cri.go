package stages

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	crip "github.com/grafana/alloy/internal/component/loki/process/stages/cri"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

type CRIConfig struct {
	MaxPartialLines            int    `alloy:"max_partial_lines,attr,optional"`
	MaxPartialLineSize         uint64 `alloy:"max_partial_line_size,attr,optional"`
	MaxPartialLineSizeTruncate bool   `alloy:"max_partial_line_size_truncate,attr,optional"`
}

var (
	_ syntax.Defaulter = (*CRIConfig)(nil)
	_ syntax.Validator = (*CRIConfig)(nil)
)

var defaultCRIConfig = CRIConfig{
	MaxPartialLines:            100,
	MaxPartialLineSize:         0,
	MaxPartialLineSizeTruncate: false,
}

// SetToDefault implements syntax.Defaulter.
func (args *CRIConfig) SetToDefault() {
	*args = defaultCRIConfig
}

// Validate implements syntax.Validator.
func (args *CRIConfig) Validate() error {
	if args.MaxPartialLines <= 0 {
		return fmt.Errorf("max_partial_lines must be greater than 0")
	}

	return nil
}

func newCRIStage(cfg CRIConfig, opts stageOpts) *criStage {
	c := &criStage{
		next:                      opts.next,
		logger:                    opts.slogger.With("stage", "cri"),
		cfg:                       cfg,
		partialLinesFlushedMetric: getPartialLinesFlushedMetric(opts.registerer),
		linesTruncatedMetric:      getLinesTruncatedMetric(opts.registerer),
	}
	c.partialLines = newPartialLineStore(cfg, c.linesTruncatedMetric)
	return c
}

func getPartialLinesFlushedMetric(registerer prometheus.Registerer) prometheus.Counter {
	metric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_process_cri_partial_lines_flushed_total",
		Help: "A count of partial lines that were flushed prematurely due to the max_partial_lines limit being exceeded",
	})
	return util.MustRegisterOrGet(registerer, metric).(prometheus.Counter)
}

func getLinesTruncatedMetric(registerer prometheus.Registerer) prometheus.Counter {
	metric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_process_cri_lines_truncated_total",
		Help: "A count of lines that were truncated due to the max_partial_line_size limit",
	})
	return util.MustRegisterOrGet(registerer, metric).(prometheus.Counter)
}

var (
	_ Stage = (*criStage)(nil)

	_ entryProcessor = (*criStage)(nil)
	_ stopper        = (*criStage)(nil)
)

type criStage struct {
	next   nextFn
	cfg    CRIConfig
	logger *slog.Logger

	partialLines *partialLineStore

	partialLinesFlushedMetric prometheus.Counter
	linesTruncatedMetric      prometheus.Counter
}

const (
	criFlags   = "flags"
	criStream  = "stream"
	criContent = "content"
	criTime    = "time"
)

func (c *criStage) Run(in chan Entry) chan Entry {
	return RunWithSkipOrSendMany(in, func(e Entry) ([]Entry, bool) {
		parsed, ok := crip.ParseCRI(e.Line)
		if !ok {
			return []Entry{e}, false
		}

		setCRIProperties(&e, parsed)

		fingerprint := e.Labels.Fingerprint()
		// We received partial-line (tag: "P")
		if parsed.Flag == crip.FlagPartial {
			// flush any partial lines if we have buffered too many.
			entries := c.flushPartialLinesIfExceeded()
			// it's a partial-line buffer it and move on.
			c.partialLines.Append(fingerprint, e)
			return entries, len(entries) == 0
		}

		// We got full-line 'F'.
		// If any old partial lines matches with this full-line stream, merge it,
		// else just return the full line.
		return []Entry{c.completeFullLine(fingerprint, e)}, false
	})
}

// process implements entryProcessor and is only used by our new pipeline. A
// flush can forward a stream owned by a different, concurrently-running caller
// through this caller's next() call, causing OOO for that stream. We accept
// that tradeoff for now.
func (c *criStage) process(ctx context.Context, entries []Entry) error {
	// dst compacts entries in place.
	var dst int
	for _, e := range entries {
		parsed, ok := crip.ParseCRI(e.Line)
		if !ok {
			entries[dst] = e
			dst++
			continue
		}

		setCRIProperties(&e, parsed)

		fingerprint := e.Labels.Fingerprint()
		if parsed.Flag == crip.FlagPartial {
			c.partialLines.Append(fingerprint, e)
			continue
		}

		entries[dst] = c.completeFullLine(fingerprint, e)
		dst++
	}

	out := entries[:dst]

	// Checking once per batch lets the store overshoot by up to one batch, which
	// is acceptable because those entries already fit in memory as part of it.
	// In the future a background goroutine could own this check, so that batches
	// do not pay the cost of calling this at all.
	extra := c.flushPartialLinesIfExceeded()
	if len(extra) > 0 {
		// Buffered entries go out before the entries passed in. We are assuming that
		// these are older entries, but this can still lead to some out-of-order results,
		// which we consider acceptable for this safety mechanism.
		out = append(extra, out...)
	}

	if len(out) == 0 {
		return nil
	}

	return c.next(ctx, out)
}

func (c *criStage) completeFullLine(fp model.Fingerprint, e Entry) Entry {
	prev, ok := c.partialLines.Take(fp)
	if ok {
		e.Line = prev.Line + e.Line
		truncatePartialLine(&e, c.cfg, c.linesTruncatedMetric)
	}
	return e
}

func (c *criStage) flushPartialLinesIfExceeded() []Entry {
	flushed := c.partialLines.DrainIfAtLeast(c.cfg.MaxPartialLines)
	if len(flushed) == 0 {
		return nil
	}

	c.logger.Warn("partial lines upperbound exceeded, merging it to single line", "threshold", c.cfg.MaxPartialLines)
	if c.partialLinesFlushedMetric != nil {
		c.partialLinesFlushedMetric.Add(float64(len(flushed)))
	}

	return flushed
}

// stop implements stopper and is only used by our new pipeline.
func (c *criStage) stop() {
	const flushTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	out := c.partialLines.DrainAll()
	if len(out) == 0 {
		return
	}

	if err := c.next(ctx, out); err != nil {
		c.logger.Error("failed to flush held partial lines on stop", "err", err)
	}
}

func truncatePartialLine(e *Entry, cfg CRIConfig, linesTruncated prometheus.Counter) {
	if cfg.MaxPartialLineSizeTruncate && len(e.Line) > int(cfg.MaxPartialLineSize) {
		e.Line = e.Line[:cfg.MaxPartialLineSize]
		if linesTruncated != nil {
			linesTruncated.Inc()
		}
	}
}

func (c *criStage) Cleanup() {}

func setCRIProperties(e *Entry, parsed crip.Parsed) {
	// NOTE: Previous implementation used a "sub-pipeline"
	// to parse CRI logs where the regex stage added these fields
	// as "extracted" values so the other stages could operate on them.
	// We don't need this anymore but it would be a breaking change to
	// no longer set these.
	e.Extracted[criFlags] = parsed.Flag.String()
	e.Extracted[criStream] = parsed.Stream.String()
	e.Extracted[criContent] = parsed.Content
	e.Extracted[criTime] = parsed.Timestamp

	e.Line = parsed.Content

	ts, err := time.Parse(time.RFC3339Nano, parsed.Timestamp)
	if err == nil {
		e.Timestamp = ts
	}

	e.Labels[criStream] = model.LabelValue(parsed.Stream.String())
}
