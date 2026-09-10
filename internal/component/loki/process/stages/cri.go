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
		pl     partialLines
		logger = opts.slogger.With("stage", "cri")
	)

	if opts.next == nil {
		pl = newPartialLinesMap(cfg, logger, getLinesTruncatedMetric(opts.registerer), getLinesFlushedMetric(opts.registerer))
	} else {
		pl = newPartialLinesStriped(cfg, logger, getLinesTruncatedMetric(opts.registerer), getLinesFlushedMetric(opts.registerer))
	}

	return &criStage{
		next:         opts.next,
		logger:       logger,
		partialLines: pl,
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

	_ entryProcessor = (*criStage)(nil)
	_ stopper        = (*criStage)(nil)
)

type criStage struct {
	next   nextFn
	cfg    CRIConfig
	logger *slog.Logger

	partialLines partialLines

	linesTruncatedMetric      prometheus.Counter
	partialLinesFlushedMetric prometheus.Counter
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

		fp := e.Labels.Fingerprint()
		// We received partial-line (tag: "P")
		if parsed.Flag == crip.FlagPartial {
			entries := c.partialLines.FlushIfExceeded()
			// it's a partial-line buffer it and move on.
			c.partialLines.Append(fp, e)
			return entries, false
		}

		// We got full-line 'F'.
		return []Entry{c.partialLines.Complete(fp, e)}, false
	})
}

// process implements entryProcessor and is only used by our new pipeline.
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
		// We received partial-line (tag: "P")
		if parsed.Flag == crip.FlagPartial {
			// it's a partial-line buffer it and move on.
			c.partialLines.Append(fp, e)
			continue
		}

		// We got full-line 'F'.
		// If any old partial lines matches with this full-line stream, merge it,
		// else just return the full line.
		entries[dst] = c.partialLines.Complete(fp, e)
		dst++
	}

	out := entries[:dst]
	// flush any partial lines if we have buffered too many.
	// This can cause OOO for entries since we don't serialize the whole
	// pipeline. This is an acceptable trade-off since this is a failure case
	// and is something we can revisit in the future.
	if extra := c.partialLines.FlushIfExceeded(); len(extra) > 0 {
		out = append(out, extra...)
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

	entries := c.partialLines.FlushAll()
	if len(entries) == 0 {
		return
	}

	if err := c.next(ctx, entries); err != nil {
		c.logger.Error("failed to flush held partial lines on stop", "err", err)
	}
}

func (c *criStage) Cleanup() {}

type partialLines interface {
	Append(fp model.Fingerprint, e Entry)
	Complete(fp model.Fingerprint, e Entry) Entry

	FlushAll() []Entry
	FlushIfExceeded() []Entry
}

var _ partialLines = (*partialLinesMap)(nil)

func newPartialLinesMap(cfg CRIConfig, logger *slog.Logger, linesTruncated prometheus.Counter, linesFlushed prometheus.Counter) *partialLinesMap {
	return &partialLinesMap{
		cfg:            cfg,
		logger:         logger,
		linesTruncated: linesTruncated,
		linesFlushed:   linesFlushed,
		inner:          make(map[model.Fingerprint]Entry, cfg.MaxPartialLines),
	}
}

// partialLinesMap is an implementation of partialLines that is not safe for concurrent use.
// It will only be used with the "old" pipeline where entries are passed through channels between
// each stage.
type partialLinesMap struct {
	cfg CRIConfig

	logger         *slog.Logger
	linesTruncated prometheus.Counter
	linesFlushed   prometheus.Counter

	inner map[model.Fingerprint]Entry
}

func (m *partialLinesMap) Append(fp model.Fingerprint, e Entry) {
	e.Line = ensureTruncateIfRequired(m.inner[fp].Line, e.Line, m.cfg, m.linesTruncated)
	m.inner[fp] = e
}

func (m *partialLinesMap) Complete(fp model.Fingerprint, e Entry) Entry {
	v, ok := m.inner[fp]
	if ok {
		delete(m.inner, fp)
		e.Line = ensureTruncateIfRequired(v.Line, e.Line, m.cfg, m.linesTruncated)
	}
	return e
}

func (m *partialLinesMap) FlushAll() []Entry {
	buf := make([]Entry, 0, len(m.inner))
	for _, v := range m.inner {
		buf = append(buf, v)
	}
	clear(m.inner)
	return buf
}

func (m *partialLinesMap) FlushIfExceeded() []Entry {
	if len(m.inner) < m.cfg.MaxPartialLines {
		return nil
	}

	entries := m.FlushAll()

	if len(entries) == 0 {
		return nil
	}

	m.logger.Warn("partial lines upperbound exceeded, merging it to single line", "threshold", m.cfg.MaxPartialLines)
	m.linesFlushed.Add(float64(len(entries)))
	return entries
}

const stripeCount = 16

var _ partialLines = (*partialLinesStriped)(nil)

// partialLinesStriped is an implementation of partialLines that is safe for concurrent use.
// It will only be used with the "new" pipeline where many callers can pass through the stage
// concurrently.
type partialLinesStriped struct {
	// size is only exact while every stripe lock is held. Other readers treat it
	// as a hint. flushing keeps concurrent callers from all queueing on lockAll.
	size     atomic.Int64
	flushing atomic.Bool

	stripes [stripeCount]stripe

	cfg            CRIConfig
	logger         *slog.Logger
	linesTruncated prometheus.Counter
	linesFlushed   prometheus.Counter
}

type stripe struct {
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
	perStripe := m.cfg.MaxPartialLines/stripeCount + 1
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

	return m.drainLocked()
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

func (m *partialLinesStriped) stripe(fp model.Fingerprint) *stripe {
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
		linesTruncated.Inc()
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
