package stages

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

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
	var (
		logger         = opts.slogger.With("stage", "cri")
		linesTruncated = getLinesTruncatedMetric(opts.registerer)
		linesFlushed   = getLinesFlushedMetric(opts.registerer)
	)

	return &criStage{
		cfg:                 cfg,
		next:                opts.next,
		logger:              logger,
		partialLines:        make(map[model.Fingerprint]Entry, cfg.MaxPartialLines),
		partialLinesStriped: newPartialLinesStriped(cfg, logger, linesTruncated, linesFlushed),
		linesFlushed:        linesFlushed,
		linesTruncated:      linesTruncated,
	}
}

func getLinesFlushedMetric(registerer prometheus.Registerer) prometheus.Counter {
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

	_ stopper        = (*criStage)(nil)
	_ entryProcessor = (*criStage)(nil)
)

type criStage struct {
	next   nextFn
	cfg    CRIConfig
	logger *slog.Logger

	// partialLines is only used by Run, where a single goroutine passes one
	// entry at a time through the stage, so no locking is needed.
	partialLines map[model.Fingerprint]Entry
	// partialLinesStriped is only used by process, where many callers can pass
	// batches through the stage concurrently.
	partialLinesStriped *partialLinesStriped

	linesFlushed   prometheus.Counter
	linesTruncated prometheus.Counter
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
			if len(c.partialLines) >= c.cfg.MaxPartialLines {
				c.logger.Warn("partial lines upperbound exceeded, merging it to single line", "threshold", c.cfg.MaxPartialLines)
				c.linesFlushed.Add(float64(len(c.partialLines)))

				// Merge existing partialLines
				entries := make([]Entry, 0, len(c.partialLines))
				for _, v := range c.partialLines {
					entries = append(entries, v)
				}

				c.partialLines = make(map[model.Fingerprint]Entry, c.cfg.MaxPartialLines)
				e.Line = ensureTruncateIfRequired("", e.Line, c.cfg, c.linesTruncated)
				c.partialLines[fingerprint] = e

				return entries, false
			}

			prev := c.partialLines[fingerprint]
			e.Line = ensureTruncateIfRequired(prev.Line, e.Line, c.cfg, c.linesTruncated)
			c.partialLines[fingerprint] = e

			// it's a partial-line so skip it.
			return nil, true
		}

		// We got full-line 'F'.
		// If any old partial lines matches with this full-line stream, merge it,
		// else just return the full line.
		prev, ok := c.partialLines[fingerprint]
		if ok {
			e.Line = ensureTruncateIfRequired(prev.Line, e.Line, c.cfg, c.linesTruncated)
			delete(c.partialLines, fingerprint)
		}

		return []Entry{e}, false
	})
}

func (c *criStage) process(ctx context.Context, entries []Entry) error {
	var dst int
	for _, e := range entries {
		parsed, ok := crip.ParseCRI(e.Line)
		if !ok {
			entries[dst] = e
			dst++
			continue
		}

		setCRIProperties(&e, parsed)

		fp := e.Labels.Fingerprint()
		if parsed.Flag == crip.FlagPartial {
			// it's a partial-line buffer it and move on.
			c.partialLinesStriped.Append(fp, e)
			continue
		}

		// If any old partial lines matches with this full-line stream, merge it,
		// else just return the full line.
		entries[dst] = c.partialLinesStriped.Complete(fp, e)
		dst++
	}

	out := entries[:dst]
	// One check per batch keeps the limit approximate. It only prevents leaks.
	// If we have buffered too many, flush all partial lines globally. This
	// includes streams this batch does not own, so their lines go out through
	// our next call and a concurrent batch can reach next ahead of them. That
	// can cause OOO for entries. We consider this an acceptable trade-off,
	// since it is a safety mechanism and no logs are lost.
	out = append(out, c.partialLinesStriped.FlushIfExceeded()...)

	if len(out) == 0 {
		return nil
	}

	return c.next(ctx, out)
}

func (c *criStage) stop() {
	const flushTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	entries := c.partialLinesStriped.FlushAll()
	if len(entries) == 0 {
		return
	}

	if err := c.next(ctx, entries); err != nil {
		c.logger.Error("failed to flush held partial lines on stop", "err", err)
	}
}

func (c *criStage) Cleanup() {}

const (
	stripeCount = 16

	// maxPreallocPerStripe of 3k caps the up-front allocation at about 8 MiB across all 16 stripes.
	maxPreallocPerStripe = 3000
)

// partialLinesStriped holds partial lines in a map sharded into a fixed
// number of independently locked stripes. It is safe for concurrent use.
type partialLinesStriped struct {
	// size is only exact while every stripe lock is held. Other readers treat it as a hint.
	size atomic.Int64
	// flushing is used to prevent concurrent callers from all queueing on lockAll.
	flushing atomic.Bool

	stripes [stripeCount]partialLinesStripe

	cfg            CRIConfig
	logger         *slog.Logger
	linesTruncated prometheus.Counter
	linesFlushed   prometheus.Counter
}

type partialLinesStripe struct {
	mu   sync.Mutex
	data map[model.Fingerprint]Entry
}

func newPartialLinesStriped(cfg CRIConfig, logger *slog.Logger, linesTruncated prometheus.Counter, linesFlushed prometheus.Counter) *partialLinesStriped {
	m := &partialLinesStriped{
		cfg:            cfg,
		logger:         logger,
		linesFlushed:   linesFlushed,
		linesTruncated: linesTruncated,
	}
	// MaxPartialLines is user supplied and has no upper bound, so cap what we
	// pre-allocate. The stripe maps grow on demand, so a larger limit still
	// works, it just pays for the growth as it goes.
	perStripe := min(cfg.MaxPartialLines/stripeCount+1, maxPreallocPerStripe)
	for i := range m.stripes {
		m.stripes[i].data = make(map[model.Fingerprint]Entry, perStripe)
	}
	return m
}

func (m *partialLinesStriped) Append(fp model.Fingerprint, e Entry) {
	s := m.stripe(fp)
	s.mu.Lock()
	defer s.mu.Unlock()

	prev, existed := s.data[fp]
	e.Line = ensureTruncateIfRequired(prev.Line, e.Line, m.cfg, m.linesTruncated)
	s.data[fp] = e
	if !existed {
		m.size.Add(1)
	}
}

func (m *partialLinesStriped) Complete(fp model.Fingerprint, e Entry) Entry {
	s := m.stripe(fp)
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.data[fp]
	if ok {
		delete(s.data, fp)
		m.size.Add(-1)
		e.Line = ensureTruncateIfRequired(v.Line, e.Line, m.cfg, m.linesTruncated)
	}

	return e
}

func (m *partialLinesStriped) FlushAll() []Entry {
	m.lockAll()
	defer m.unlockAll()

	entries := m.drainLocked()
	m.linesFlushed.Add(float64(len(entries)))
	return entries
}

func (m *partialLinesStriped) FlushIfExceeded() []Entry {
	if int(m.size.Load()) < m.cfg.MaxPartialLines || !m.flushing.CompareAndSwap(false, true) {
		return nil
	}
	defer m.flushing.Store(false)

	m.lockAll()
	defer m.unlockAll()

	// Another caller may have drained the map while this one waited for the
	// stripe locks, so re-check the exact size now that everything is held.
	if int(m.size.Load()) < m.cfg.MaxPartialLines {
		return nil
	}
	entries := m.drainLocked()

	if len(entries) == 0 {
		return nil
	}

	m.logger.Warn("partial lines upperbound exceeded, merging it to single line", "threshold", m.cfg.MaxPartialLines)
	m.linesFlushed.Add(float64(len(entries)))
	return entries
}

func (m *partialLinesStriped) stripe(fp model.Fingerprint) *partialLinesStripe {
	return &m.stripes[fp&(stripeCount-1)]
}

func (m *partialLinesStriped) lockAll() {
	for i := range m.stripes {
		m.stripes[i].mu.Lock()
	}
}

func (m *partialLinesStriped) unlockAll() {
	for i := len(m.stripes) - 1; i >= 0; i-- {
		m.stripes[i].mu.Unlock()
	}
}

func (m *partialLinesStriped) drainLocked() []Entry {
	buf := make([]Entry, 0, m.size.Load())
	for i := range m.stripes {
		for _, v := range m.stripes[i].data {
			buf = append(buf, v)
		}
		clear(m.stripes[i].data)
	}
	m.size.Store(0)
	return buf
}

func ensureTruncateIfRequired(prevLine, newLine string, cfg CRIConfig, linesTruncated prometheus.Counter) string {
	if !cfg.MaxPartialLineSizeTruncate {
		return prevLine + newLine
	}

	// If prev line is already at max size we don't have to concatenate new line.
	if len(prevLine) >= int(cfg.MaxPartialLineSize) {
		// An empty new line discards nothing, so it is not a truncation.
		if len(newLine) > 0 {
			linesTruncated.Inc()
		}
		return prevLine[:cfg.MaxPartialLineSize]
	}

	line := prevLine + newLine

	if len(line) > int(cfg.MaxPartialLineSize) {
		linesTruncated.Inc()
		line = line[:cfg.MaxPartialLineSize]
	}

	return line
}

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
