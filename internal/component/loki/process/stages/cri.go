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
	return &criStage{
		next:                      opts.next,
		logger:                    opts.slogger.With("stage", "cri"),
		cfg:                       cfg,
		partialLines:              newStripedMap[Entry](cfg.MaxPartialLines),
		partialLinesFlushedMetric: getPartialLinesFlushedMetric(opts.registerer),
		linesTruncatedMetric:      getLinesTruncatedMetric(opts.registerer),
	}
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

	partialLines *stripedMap[Entry]

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

		var entries []Entry

		setCRIProperties(&e, parsed)

		fingerprint := e.Labels.Fingerprint()
		// We received partial-line (tag: "P")
		if parsed.Flag == crip.FlagPartial {
			// flush any partial lines if we have buffered too many.
			entries := c.flushPartialLinesIfExceeded()
			// it's a partial-line buffer it and move on.
			c.addPartialLine(fingerprint, e)
			return entries, len(entries) == 0
		} else {
			// We got full-line 'F'.
			entries = []Entry{c.completeFullLine(fingerprint, e)}
		}

		if extra := c.flushPartialLinesIfExceeded(); len(extra) > 0 {
			entries = append(entries, extra...)
		}

		return entries, len(entries) == 0
	})
}

// process implements entryProcessor and is only used by our new pipeline.
// c.mut is released before calling next(), so flushPartialLinesIfExceeded
// can sweep up and forward a stream owned by a different, concurrently-
// running caller through this caller's next() call, causing OOO for that
// stream. For now this is an acceptable tradeoff and we consider this a
// failure case. We can revisit this in the future.
func (c *criStage) process(ctx context.Context, entries []Entry) error {

	// dst compacts entries in place, extra only grows when the
	// MaxPartialLines branch below flushes held partial lines. So in
	// the common case this call never allocates a new slice.
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
		// We received partial-line (tag: "P")
		if parsed.Flag == crip.FlagPartial {
			// it's a partial-line buffer it and move on.
			c.addPartialLine(fingerprint, e)
			continue
		}

		// We got full-line 'F'.
		// If any old partial lines matches with this full-line stream, merge it,
		// else just return the full line.
		entries[dst] = c.completeFullLine(fingerprint, e)
		dst++
	}

	out := entries[:dst]
	// flush any partial lines if we have buffered too many.
	if extra := c.flushPartialLinesIfExceeded(); len(extra) > 0 {
		out = append(out, extra...)
	}

	if len(out) == 0 {
		return nil
	}

	return c.next(ctx, out)
}

func (c *criStage) addPartialLine(fp model.Fingerprint, e Entry) {
	c.partialLines.Update(uint64(fp), func(prev Entry) Entry {
		e.Line = c.ensureTruncateIfRequired(prev.Line, e.Line)
		return e
	})
}

func (c *criStage) completeFullLine(fp model.Fingerprint, e Entry) Entry {
	prev, ok := c.partialLines.Take(uint64(fp))
	if ok {
		e.Line = c.ensureTruncateIfRequired(prev.Line, e.Line)
	}
	return e
}

func (c *criStage) flushPartialLinesIfExceeded() []Entry {
	entries := c.partialLines.DrainIfAtLeast(c.cfg.MaxPartialLines)
	if len(entries) == 0 {
		return nil
	}

	c.logger.Warn("partial lines upperbound exceeded, merging it to single line", "threshold", c.cfg.MaxPartialLines)
	c.partialLinesFlushedMetric.Add(float64(len(entries)))

	return entries
}

// stop implements stopper and is only used by our new pipeline.
func (c *criStage) stop() {
	const flushTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	entries := c.partialLines.DrainAll()
	if len(entries) == 0 {
		return
	}

	if err := c.next(ctx, entries); err != nil {
		c.logger.Error("failed to flush held partial lines on stop", "err", err)
	}
}

func (c *criStage) ensureTruncateIfRequired(prevLine, newLine string) string {
	if !c.cfg.MaxPartialLineSizeTruncate {
		return prevLine + newLine
	}

	// If prev line is already at max size we don't have to concatinate new line.
	if len(prevLine) == int(c.cfg.MaxPartialLineSize) {
		if c.linesTruncatedMetric != nil {
			c.linesTruncatedMetric.Inc()
		}
		return prevLine
	}

	line := prevLine + newLine

	if len(line) > int(c.cfg.MaxPartialLineSize) {
		line = line[:c.cfg.MaxPartialLineSize]
		if c.linesTruncatedMetric != nil {
			c.linesTruncatedMetric.Inc()
		}
	}

	return line
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
