package stages

import (
	"bytes"
	"context"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	crip "github.com/grafana/alloy/internal/component/loki/process/stages/cri"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

type Emitter2 func(ctx context.Context, entries []Entry) error

type Stage4 interface {
	Process(ctx context.Context, entries []Entry) error
	SetNext(next Emitter2)
	Cleanup()
}

var _ Stage4 = (*Pipeline4)(nil)

type Pipeline4 struct {
	stages []Stage4
}

func NewPipeline4(stages []Stage4) *Pipeline4 {
	return &Pipeline4{stages: stages}
}

func (p *Pipeline4) ProcessBatch(ctx context.Context, batch loki.Batch) error {
	entries := make([]Entry, 0, batch.EntryLen())
	_ = batch.ConsumeStreams(func(stream loki.Stream, created int64) error {
		for _, e := range stream.Entries {
			entries = append(entries, Entry{
				Extracted: make(map[string]any, len(stream.Labels)),

				Entry: loki.NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), created, e),
			})
		}
		return nil
	})

	return p.stages[0].Process(ctx, entries)
}

func (p *Pipeline4) ProcessEntry(ctx context.Context, entry loki.Entry) error {
	return p.stages[0].Process(ctx, []Entry{
		{
			Extracted: make(map[string]any, len(entry.Labels)),
			Entry:     entry,
		},
	})
}

func (p *Pipeline4) Process(ctx context.Context, entries []Entry) error {
	return p.stages[0].Process(ctx, entries)
}

func (p *Pipeline4) SetNext(next Emitter2) {
	for i, s := range slices.Backward(p.stages) {
		if i == len(p.stages)-1 {
			s.SetNext(next)
			continue
		}

		s.SetNext(p.stages[i+1].Process)
	}
}

func (p *Pipeline4) Cleanup() {}

var (
	_ Stage4 = (*staticLabelsStage4)(nil)
)

func newStaticLabelsStage4(values []string) *staticLabelsStage4 {
	return &staticLabelsStage4{values: values}
}

// staticLabelsStage4 is the simplest kind of stage. It just processes the
// current entry and always forwards it to next.
type staticLabelsStage4 struct {
	next   Emitter2
	values []string
}

// Process implements Stage4.
func (s *staticLabelsStage4) Process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		for j := 0; j < len(s.values); j += 2 {
			entries[i].Labels[model.LabelName(s.values[j])] = model.LabelValue(s.values[j+1])
		}
	}

	return s.next(ctx, entries)
}

// SetNext implements Stage4.
func (s *staticLabelsStage4) SetNext(next Emitter2) {
	s.next = next
}

func (s *staticLabelsStage4) Cleanup() {}

var _ Stage4 = (*criStage4)(nil)

func newCRIStage4(maxPartialLines int) *criStage4 {
	return &criStage4{
		maxPartialLines: maxPartialLines,
		partialLines:    make(map[model.Fingerprint]Entry, maxPartialLines),
	}
}

type criStage4 struct {
	next            Emitter2
	maxPartialLines int

	mu           sync.Mutex
	partialLines map[model.Fingerprint]Entry
}

func (c *criStage4) Process(ctx context.Context, entries []Entry) error {
	c.mu.Lock()

	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		parsed, ok := crip.ParseCRI(e.Line)
		if !ok {
			out = append(out, e)
			continue
		}
		e.Line = parsed.Content

		fingerprint := e.Labels.Fingerprint()

		if parsed.Flag == crip.FlagPartial {
			if len(c.partialLines) >= c.maxPartialLines {
				// Held state has grown too large: flush everything now,
				// then start over.
				for _, buffered := range c.partialLines {
					out = append(out, buffered)
				}
				c.partialLines = make(map[model.Fingerprint]Entry, c.maxPartialLines)
			}
			if prev, ok := c.partialLines[fingerprint]; ok {
				e.Line = prev.Line + e.Line
			}
			c.partialLines[fingerprint] = e
			continue // held: not part of this call's output
		}

		if prev, ok := c.partialLines[fingerprint]; ok {
			e.Line = prev.Line + e.Line
			delete(c.partialLines, fingerprint)
		}
		out = append(out, e)
	}

	c.mu.Unlock()

	return c.next(ctx, out)

}

func (c *criStage4) SetNext(next Emitter2) {
	c.next = next
}

func (c *criStage4) Cleanup() {}

func newMatchStage4(match func(Entry) bool, action string, stages []Stage4) Stage4 {
	switch action {
	case MatchActionKeep:
		return newMatchStageKeep4(match, stages)
	default:
		return newMatchStageDrop4(match)
	}
}

var _ Stage4 = (*matchStageKeep4)(nil)

func newMatchStageKeep4(match func(Entry) bool, stages []Stage4) *matchStageKeep4 {
	return &matchStageKeep4{
		match:    match,
		pipeline: NewPipeline4(stages),
	}
}

type matchStageKeep4 struct {
	next  Emitter2
	match func(Entry) bool

	pipeline *Pipeline4
}

func (m *matchStageKeep4) Process(ctx context.Context, entries []Entry) error {
	var matched, unmatched []Entry

	for i := range entries {
		if m.match(entries[i]) {
			matched = append(matched, entries[i])
			continue
		}
		unmatched = append(unmatched, entries[i])
	}
	if len(unmatched) > 0 {
		if err := m.next(ctx, unmatched); err != nil {
			return err
		}
	}

	if len(matched) > 0 {
		return m.pipeline.Process(ctx, matched)
	}

	return nil
}

func (m *matchStageKeep4) SetNext(next Emitter2) {
	m.next = next
	m.pipeline.SetNext(next)
}

func (m *matchStageKeep4) Cleanup() {
	m.pipeline.Cleanup()
}

var _ Stage4 = (*matchStageDrop4)(nil)

func newMatchStageDrop4(match func(Entry) bool) *matchStageDrop4 {
	return &matchStageDrop4{match: match}
}

type matchStageDrop4 struct {
	next  Emitter2
	match func(Entry) bool
}

// Process implements Stage4.
func (m *matchStageDrop4) Process(ctx context.Context, entries []Entry) error {
	var dst int
	for _, e := range entries {
		if m.match(e) {
			entries[dst] = e
			dst++
			continue
		}
	}

	return m.next(ctx, entries[:dst])
}

// SetNext implements Stage4.
func (m *matchStageDrop4) SetNext(next Emitter2) {
	m.next = next
}

func (m *matchStageDrop4) Cleanup() {}

var _ Stage4 = (*multilineStage4)(nil)

func newMultilineStage4(cfg MultilineConfig) (*multilineStage4, error) {
	regex, err := validateMultilineConfig(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.MaxLines < 2 {
		panic("useless")
	}

	m := &multilineStage4{
		cfg:     cfg,
		regex:   regex,
		streams: make(map[model.Fingerprint]*multilineState),
		stop:    make(chan struct{}),
	}

	// FIXME(kalleep): Start and Stop?
	m.wg.Go(m.flushLoop)
	return m, nil
}

type multilineStage4 struct {
	next  Emitter2
	cfg   MultilineConfig
	regex *regexp.Regexp

	mu      sync.Mutex
	streams map[model.Fingerprint]*multilineState

	stop chan struct{}
	once sync.Once
	wg   sync.WaitGroup
}

// Process implements Stage4.
func (m *multilineStage4) Process(ctx context.Context, entries []Entry) error {
	m.mu.Lock()
	var dst int
	for i := range entries {
		key := entries[i].Labels.Fingerprint()
		if flushed, ok := m.processEntry(key, entries[i]); ok {
			entries[dst] = flushed
			dst++
		}
	}
	m.mu.Unlock()

	if dst == 0 {
		return nil
	}
	return m.next(ctx, entries[:dst])
}

func (m *multilineStage4) processEntry(key model.Fingerprint, e Entry) (Entry, bool) {
	state, hasState := m.streams[key]

	isFirstLine := m.regex.MatchString(e.Line)
	if !hasState {
		if !isFirstLine {
			return e, true
		}
		state = &multilineState{buffer: new(bytes.Buffer)}
		m.streams[key] = state
	}

	var (
		flushed Entry
		ok      bool
	)
	if isFirstLine && state.currentLines > 0 {
		flushed, ok = m.flushState(state), true
	}
	if isFirstLine {
		state.startLineEntry = e
	}

	if state.buffer.Len() > 0 {
		state.buffer.WriteRune('\n')
	}
	line := e.Line
	if m.cfg.TrimNewlines {
		line = strings.TrimRight(line, "\r\n")
	}
	state.buffer.WriteString(line)
	state.currentLines++
	state.lastSeen = time.Now()

	// Safe to overwrite flushed/ok here without checking ok first: if the
	// new-start-line flush above fired, it reset currentLines to 0, so this
	// entry can bring it to at most 1 -- which can only equal MaxLines if
	// MaxLines were 1, ruled out by the MaxLines >= 2 check in
	// newMultilineStage4. So the two flushes can never both fire for the
	// same entry.
	if state.currentLines == m.cfg.MaxLines {
		flushed, ok = m.flushState(state), true
	}

	return flushed, ok
}

// flushState mirrors multilineStage.flushState. Must be called with mu held.
func (m *multilineStage4) flushState(s *multilineState) Entry {
	collapsed := Entry{
		Extracted: maps.Clone(s.startLineEntry.Extracted),
		Entry: loki.NewEntryWithCreatedUnixMicro(s.startLineEntry.Entry.Labels.Clone(), s.startLineEntry.Created(), push.Entry{
			Timestamp:          s.startLineEntry.Entry.Entry.Timestamp,
			Line:               s.buffer.String(),
			StructuredMetadata: slices.Clone(s.startLineEntry.Entry.Entry.StructuredMetadata),
		}),
	}

	s.buffer.Reset()
	s.currentLines = 0

	return collapsed
}

// SetNext implements Stage4.
func (m *multilineStage4) SetNext(next Emitter2) {
	m.next = next
}

func (m *multilineStage4) Cleanup() {
	m.once.Do(func() { close(m.stop) })
	m.wg.Wait()

	m.mu.Lock()
	out := make([]Entry, 0, len(m.streams))
	for _, state := range m.streams {
		if state.currentLines > 0 {
			out = append(out, m.flushState(state))
		}
	}
	m.streams = nil
	m.mu.Unlock()

	if len(out) > 0 && m.next != nil {
		_ = m.next(context.Background(), out)
	}
}

func (m *multilineStage4) flushLoop() {
	ticker := time.NewTicker(m.cfg.MaxWaitTime)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			entries := m.expired()
			if len(entries) > 0 {
				_ = m.next(context.Background(), entries)
			}
		}
	}
}

func (m *multilineStage4) expired() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	var out []Entry
	for key, state := range m.streams {
		if !state.lastSeen.Add(m.cfg.MaxWaitTime).After(now) {
			if state.currentLines > 0 {
				out = append(out, m.flushState(state))
			}
			delete(m.streams, key)
		}
	}
	return out
}
