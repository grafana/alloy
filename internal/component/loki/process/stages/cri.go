package stages

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

	flushMu sync.RWMutex

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
	// Run owns this stage exclusively on one dedicated goroutine for its
	// whole lifetime, so it's safe to use partialLines without locking.
	partialLines := c.partialLines

	return RunWithSkipOrSendMany(in, func(e Entry) ([]Entry, bool) {
		parsed, ok := crip.ParseCRI(e.Line)
		if !ok {
			return []Entry{e}, false
		}

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

		fingerprint := e.Labels.Fingerprint()
		// We received partial-line (tag: "P")
		if parsed.Flag == crip.FlagPartial {
			if len(partialLines) >= c.cfg.MaxPartialLines {
				c.logger.Warn("partial lines upperbound exceeded, merging it to single line", "threshold", c.cfg.MaxPartialLines)
				if c.partialLinesFlushedMetric != nil {
					c.partialLinesFlushedMetric.Add(float64(len(partialLines)))
				}

				// Merge existing partialLines
				entries := make([]Entry, 0, len(partialLines))
				for _, v := range partialLines {
					entries = append(entries, v)
				}

				partialLines = make(map[model.Fingerprint]Entry, c.cfg.MaxPartialLines)
				c.ensureTruncateIfRequired(&e)
				(partialLines)[fingerprint] = e

				return entries, false
			}

			prev, ok := partialLines[fingerprint]
			if ok {
				var builder strings.Builder
				builder.WriteString(prev.Line)
				builder.WriteString(e.Line)
				e.Line = builder.String()
			}
			c.ensureTruncateIfRequired(&e)
			partialLines[fingerprint] = e

			// it's a partial-line so skip it.
			return nil, true
		}

		// We got full-line 'F'.
		// If any old partial lines matches with this full-line stream, merge it,
		// else just return the full line.
		prev, ok := partialLines[fingerprint]
		if ok {
			var builder strings.Builder
			builder.WriteString(prev.Line)
			builder.WriteString(e.Line)
			e.Line = builder.String()
			c.ensureTruncateIfRequired(&e)
			delete(partialLines, fingerprint)
		}

		return []Entry{e}, false
	})
}

func (c *criStage) process2(ctx context.Context, entries []Entry) error {
	// We don't worry about allocations now. We can use object pooling or reuse the entries slice.
	output := make([]Entry, 0, len(entries))
	// Process the batch and build the list of entries to send.
	for _, e := range entries {
		parsed, ok := crip.ParseCRI(e.Line)

		// If not CRI, just pass through the entry.
		if !ok {
			output = append(output, e)
			continue
		}

		// CRI. Set the properties for backwards compatibility.
		setCriProperties(&e, parsed)

		// Partial line. Store it until we get a full line.
		if parsed.Flag == crip.FlagPartial {
			linesToFlush := c.addPartialLine(&e)
			if len(linesToFlush) > 0 {
				output = append(output, linesToFlush...)
			}
			continue
		}

		// Full line. Flush any existing partial lines for this stream.
		c.assembleEntryFromPartialLines(&e)
		output = append(output, e)
	}

	// Send the output to the next stage.
	if len(output) == 0 {
		return nil
	}
	return c.next(ctx, output)
}

func (c *criStage) assembleEntryFromPartialLines(entry *Entry) {
	fp := entry.Labels.Fingerprint()

	// Multiple goroutines can access the partialLines map, so we need to lock it.
	c.mut.Lock()
	prev, ok := c.partialLines[fp]
	if ok { // remove the partial lines as we will flush them now.
		delete(c.partialLines, fp)
	}
	c.mut.Unlock()

	if !ok { // there were no partial lines waiting, nothing to do
		return
	}

	// Update the entry to contain the previous partial lines.
	var builder strings.Builder
	builder.WriteString(prev.Line)
	builder.WriteString(entry.Line)
	entry.Line = builder.String()
	c.ensureTruncateIfRequired(entry)
}

func (c *criStage) addPartialLine(entry *Entry) []Entry {
	fp := entry.Labels.Fingerprint()

	c.mut.Lock()
	defer c.mut.Unlock()

	// Safety mechanism check: flush all lines if the limit is reached.
	// NOTE: this select for flushing lines that belong to different streams, which
	// could race with other goroutines trying to add more partial lines or
	// to assemble full line for this same stream. It can result in lines being
	// out of order. We consider this to be acceptable as this is a safety
	// mechanism and while lines can be out of order or fragmented, they won't be lost.
	if len(c.partialLines) >= c.cfg.MaxPartialLines {
		c.logger.Warn("partial lines upperbound exceeded, merging it to single line", "threshold", c.cfg.MaxPartialLines)
		if c.partialLinesFlushedMetric != nil {
			c.partialLinesFlushedMetric.Add(float64(len(c.partialLines)))
		}

		// Allocation here. Should be fine as this is a safety mechanism.
		flushed := make([]Entry, 0, len(c.partialLines))
		for _, v := range c.partialLines {
			flushed = append(flushed, v)
		}

		c.partialLines = make(map[model.Fingerprint]Entry, c.cfg.MaxPartialLines)
		c.ensureTruncateIfRequired(entry)
		c.partialLines[fp] = *entry

		return flushed
	}

	// No need to flush, store the line in partialLines.
	prev, ok := c.partialLines[fp]
	if ok {
		var builder strings.Builder
		builder.WriteString(prev.Line)
		builder.WriteString(entry.Line)
		entry.Line = builder.String()
	}
	c.ensureTruncateIfRequired(entry)
	c.partialLines[fp] = *entry

	return nil
}

func setCriProperties(e *Entry, parsed crip.Parsed) {
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

// process implements entryProcessor and is only used by our new pipeline.
func (c *criStage) process(ctx context.Context, entries []Entry) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	// dst compacts entries in place, extra only grows when the
	// MaxPartialLines branch below flushes held partial lines. So in
	// the common case this call never allocates a new slice.
	var (
		dst   int
		extra []Entry
	)
	for _, e := range entries {
		parsed, ok := crip.ParseCRI(e.Line)
		if !ok { // not a CRI line, pass through
			entries[dst] = e
			dst++
			continue
		}

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

		fingerprint := e.Labels.Fingerprint() // TODO: use FastFingerprint()?
		// We received partial-line (tag: "P")
		if parsed.Flag == crip.FlagPartial {

			// TODO: If we have too many partial lines...
			if len(c.partialLines) >= c.cfg.MaxPartialLines {
				c.logger.Warn("partial lines upperbound exceeded, merging it to single line", "threshold", c.cfg.MaxPartialLines)
				if c.partialLinesFlushedMetric != nil {
					c.partialLinesFlushedMetric.Add(float64(len(c.partialLines)))
				}

				// Add existing partialLines
				for _, v := range c.partialLines {
					extra = append(extra, v)
				}

				c.partialLines = make(map[model.Fingerprint]Entry, c.cfg.MaxPartialLines)
				c.ensureTruncateIfRequired(&e)
				c.partialLines[fingerprint] = e
				continue
			}

			prev, ok := c.partialLines[fingerprint]
			if ok {
				var builder strings.Builder
				builder.WriteString(prev.Line)
				builder.WriteString(e.Line)
				e.Line = builder.String()
			}
			c.ensureTruncateIfRequired(&e)
			c.partialLines[fingerprint] = e
			// it's a partial-line so skip it.
			continue
		}

		// We got full-line 'F'.
		// If any old partial lines matches with this full-line stream, merge it,
		// else just return the full line.
		prev, ok := c.partialLines[fingerprint]
		if ok {
			var builder strings.Builder
			builder.WriteString(prev.Line)
			builder.WriteString(e.Line)
			e.Line = builder.String()
			c.ensureTruncateIfRequired(&e)
			delete(c.partialLines, fingerprint)
		}

		entries[dst] = e
		dst++
	}

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
