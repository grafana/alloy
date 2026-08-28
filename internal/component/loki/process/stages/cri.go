package stages

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
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
		partialLines:              make(map[model.Fingerprint]Entry, cfg.MaxPartialLines),
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

	mut          sync.Mutex
	partialLines map[model.Fingerprint]Entry

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
			entries := make([]Entry, 0, len(c.partialLines))
			entries = c.flushPartialLinesIfExceeded(entries)
			// it's a partial-line buffer it and move on.
			c.addPartialLine(fingerprint, e)
			return entries, len(entries) == 0
		}

		// We got full-line 'F'.
		// If any old partial lines matches with this full-line stream, merge it,
		// else just return the full line.
		return []Entry{c.completeFullLine(fingerprint, e)}, false
	})
}

// process implements entryProcessor and is only used by our new pipeline.
func (c *criStage) process(ctx context.Context, entries []Entry) error {
	c.mut.Lock()

	// dst compacts entries in place, extra only grows when the
	// MaxPartialLines branch below flushes held partial lines. So in
	// the common case this call never allocates a new slice.
	var (
		dst   int
		extra []Entry
	)
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
			// flush any partial lines if we have buffered too many.
			extra = c.flushPartialLinesIfExceeded(extra)
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

	c.mut.Unlock()

	out := entries[:dst]
	if len(extra) > 0 {
		// NOTE: We append to extra to make sure buffered entries are
		// ordered before passed in entries.
		out = append(extra, out...)
	}

	if len(out) == 0 {
		return nil
	}

	return c.next(ctx, out)
}

func (c *criStage) addPartialLine(fp model.Fingerprint, e Entry) {
	prev, ok := c.partialLines[fp]
	if ok {
		e.Line = prev.Line + e.Line
	}
	c.ensureTruncateIfRequired(&e)
	c.partialLines[fp] = e
}

func (c *criStage) completeFullLine(fp model.Fingerprint, e Entry) Entry {
	prev, ok := c.partialLines[fp]
	if ok {
		e.Line = prev.Line + e.Line
		c.ensureTruncateIfRequired(&e)
		delete(c.partialLines, fp)
	}
	return e
}

func (c *criStage) flushPartialLinesIfExceeded(buf []Entry) []Entry {
	if len(c.partialLines) < c.cfg.MaxPartialLines {
		return buf
	}

	buf = slices.Grow(buf, len(buf)+len(c.partialLines))

	c.logger.Warn("partial lines upperbound exceeded, merging it to single line", "threshold", c.cfg.MaxPartialLines)
	if c.partialLinesFlushedMetric != nil {
		c.partialLinesFlushedMetric.Add(float64(len(c.partialLines)))
	}

	for _, v := range c.partialLines {
		buf = append(buf, v)
	}

	c.partialLines = make(map[model.Fingerprint]Entry, c.cfg.MaxPartialLines)

	return buf
}

// stop implements stopper and is only used by our new pipeline.
func (c *criStage) stop() {
	const flushTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	c.mut.Lock()
	defer c.mut.Unlock()

	if len(c.partialLines) == 0 {
		return
	}

	out := make([]Entry, 0, len(c.partialLines))
	for _, e := range c.partialLines {
		out = append(out, e)
	}
	c.partialLines = make(map[model.Fingerprint]Entry)

	if err := c.next(ctx, out); err != nil {
		c.logger.Error("failed to flush held partial lines on stop", "err", err)
	}
}

func (c *criStage) ensureTruncateIfRequired(e *Entry) {
	if c.cfg.MaxPartialLineSizeTruncate && len(e.Line) > int(c.cfg.MaxPartialLineSize) {
		e.Line = e.Line[:c.cfg.MaxPartialLineSize]
		if c.linesTruncatedMetric != nil {
			c.linesTruncatedMetric.Inc()
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
