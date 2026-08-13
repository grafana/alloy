package stages

import (
	"context"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	crip "github.com/grafana/alloy/internal/component/loki/process/stages/cri"
)

// Stage3 processes a whole slice of entries at once and returns whatever
// should continue on: fewer entries to drop some, more to fan out, the
// same entries mutated in place for a plain transform. There is no next
// parameter -- a stage just returns what comes out, and Pipeline3 feeds
// that into the next stage itself.
type Stage3 interface {
	Process(ctx context.Context, entries []Entry) ([]Entry, error)
	Cleanup()
}

// Stage3WithFlush is a Stage3 that can also produce entries independent
// of any Process call, on its own schedule (e.g. an idle timeout).
type Stage3WithFlush interface {
	Stage3
	// NextDeadline reports when this stage next has something ready to
	// flush, or the zero Time if nothing is pending.
	NextDeadline() time.Time
	// Flush returns whatever is due.
	Flush(ctx context.Context) ([]Entry, error)
}

type AsyncStage interface {
	Stage3

	SetWaker(chan struct{})
}

// flusher3Entry pairs a Stage3WithFlush with its position in stages, so a
// flush can be routed through everything after it.
type flusher3Entry struct {
	idx int
	f   Stage3WithFlush
}

// Pipeline3 chains Stage3 stages together with direct function calls.
type Pipeline3 struct {
	stages   []Stage3
	flushers []flusher3Entry
}

// NewPipeline3 builds a Pipeline3 from stages, in order.
func NewPipeline3(stages []Stage3) *Pipeline3 {
	p := &Pipeline3{stages: stages}
	for i, s := range stages {
		if f, ok := s.(Stage3WithFlush); ok {
			p.flushers = append(p.flushers, flusher3Entry{idx: i, f: f})
		}
	}
	return p
}

func (p *Pipeline3) ProcessEntry(ctx context.Context, entry loki.Entry) ([]Entry, error) {
	return p.processFrom(ctx, 0, []Entry{
		{
			Extracted: make(map[string]any, len(entry.Labels)),
			Entry:     entry,
		},
	})
}

func (p *Pipeline3) ProcessBatch(ctx context.Context, batch loki.Batch) (loki.Batch, error) {
	out := loki.NewBatchWithCreatedUnixMicro(batch.Created())

	var entries []Entry
	err := batch.ConsumeStreams(func(stream loki.Stream, _ int64) error {
		// Ensure we have enough capacity.
		entries = slices.Grow(entries, len(stream.Entries))
		// Reset lenght to ensure we don't reuse from previous runs.
		entries = entries[:0]

		for _, e := range stream.Entries {
			entries = append(entries, Entry{
				Extracted: make(map[string]any, len(stream.Labels)),
				Entry: loki.Entry{
					Labels: stream.Labels.Clone(),
					Entry:  e,
				},
			})
		}

		var err error
		entries, err = p.Process(ctx, entries)
		if err != nil {
			return err
		}

		for _, e := range entries {
			out.AddEntry(e.Labels, e.Entry.Entry)
		}

		return nil
	})

	return out, err
}

// Process runs entries through every stage in order.
func (p *Pipeline3) Process(ctx context.Context, entries []Entry) ([]Entry, error) {
	return p.processFrom(ctx, 0, entries)
}

// processFrom runs entries through stages[from:] in order.
func (p *Pipeline3) processFrom(ctx context.Context, from int, entries []Entry) ([]Entry, error) {
	var err error
	for _, stage := range p.stages[from:] {
		entries, err = stage.Process(ctx, entries)
		if err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// NextDeadline reports the earliest deadline across every stage that
// implements Stage3WithFlush, or the zero Time if nothing is pending
// anywhere in the pipeline.
func (p *Pipeline3) NextDeadline() time.Time {
	var nearest time.Time
	for _, fl := range p.flushers {
		if dl := fl.f.NextDeadline(); !dl.IsZero() && (nearest.IsZero() || dl.Before(nearest)) {
			nearest = dl
		}
	}
	return nearest
}

// Flush flushes whichever Stage3WithFlush stages have a deadline that has
// already passed, running whatever each one returns through the rest of
// the pipeline.
// FIXME(maybe iterator)
func (p *Pipeline3) Flush(ctx context.Context) ([]Entry, error) {
	now := time.Now()
	var out []Entry
	for _, fl := range p.flushers {
		dl := fl.f.NextDeadline()
		if dl.IsZero() || dl.After(now) {
			continue
		}
		flushed, err := fl.f.Flush(ctx)
		if err != nil {
			return nil, err
		}
		rest, err := p.processFrom(ctx, fl.idx+1, flushed)
		if err != nil {
			return nil, err
		}
		out = append(out, rest...)
	}
	return out, nil
}

// criStage3 is the slice-based port of criStage2: partial lines are held
// across Process calls, keyed by stream fingerprint, and a Process call
// can return fewer, the same, or more entries than it was given.
type criStage3 struct {
	maxPartialLines int

	mu           sync.Mutex
	partialLines map[model.Fingerprint]Entry
}

func newCRIStage3(maxPartialLines int) *criStage3 {
	return &criStage3{
		maxPartialLines: maxPartialLines,
		partialLines:    make(map[model.Fingerprint]Entry, maxPartialLines),
	}
}

func (c *criStage3) Process(_ context.Context, entries []Entry) ([]Entry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

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
	return out, nil
}

func (c *criStage3) Cleanup() {}

// multiline3State is per-stream state for multilineStage3.
type multiline3State struct {
	buffer       strings.Builder
	startLine    Entry
	currentLines int
	lastSeen     time.Time
}

// multilineStage3 is the slice-based port of multilineStage2: like
// criStage3 it holds per-stream state across Process calls, and it also
// implements Stage3WithFlush, since a block can become due with no new
// entry ever arriving.
type multilineStage3 struct {
	regex        *regexp.Regexp
	maxLines     int
	maxWaitTime  time.Duration
	trimNewlines bool

	mu      sync.Mutex
	streams map[model.Fingerprint]*multiline3State
}

func newMultilineStage3(regex *regexp.Regexp, maxLines int, maxWaitTime time.Duration, trimNewlines bool) *multilineStage3 {
	return &multilineStage3{
		regex:        regex,
		maxLines:     maxLines,
		maxWaitTime:  maxWaitTime,
		trimNewlines: trimNewlines,
		streams:      make(map[model.Fingerprint]*multiline3State),
	}
}

func (m *multilineStage3) Process(_ context.Context, entries []Entry) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		out = append(out, m.processEntry(e)...)
	}
	return out, nil
}

// processEntry is the same logic multilineStage2.Process has, just
// returning the resolved entries instead of emitting them. Callers must
// hold m.mu.
func (m *multilineStage3) processEntry(e Entry) []Entry {
	fp := e.Labels.Fingerprint()

	var (
		resolved    []Entry
		state, ok   = m.streams[fp]
		isFirstLine = m.regex.MatchString(e.Line)
	)

	if !ok && !isFirstLine {
		return []Entry{e} // never part of a block: pass through
	}

	// A new entry arriving for a stream that's already gone stale
	// resolves the old block now, without waiting for a Flush call.
	if ok && state.currentLines > 0 && time.Since(state.lastSeen) >= m.maxWaitTime {
		resolved = append(resolved, m.collapse(state))
	}

	if !ok {
		state = &multiline3State{}
		m.streams[fp] = state
	} else if isFirstLine && state.currentLines > 0 {
		// A new block is starting: flush whatever was accumulated so far.
		resolved = append(resolved, m.collapse(state))
	}

	if isFirstLine {
		state.startLine = e
	}

	line := e.Line
	if m.trimNewlines {
		line = strings.TrimRight(line, "\r\n")
	}
	if state.buffer.Len() > 0 {
		state.buffer.WriteByte('\n')
	}
	state.buffer.WriteString(line)
	state.currentLines++
	state.lastSeen = time.Now()

	if state.currentLines == m.maxLines {
		resolved = append(resolved, m.collapse(state))
	}

	return resolved
}

// NextDeadline implements Stage3WithFlush.
func (m *multilineStage3) NextDeadline() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()

	var nearest time.Time
	for _, state := range m.streams {
		if state.currentLines == 0 {
			continue
		}
		if dl := state.lastSeen.Add(m.maxWaitTime); nearest.IsZero() || dl.Before(nearest) {
			nearest = dl
		}
	}
	return nearest
}

// Flush implements Stage3WithFlush, collapsing every stream past its
// deadline.
func (m *multilineStage3) Flush(_ context.Context) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var resolved []Entry
	for _, state := range m.streams {
		if state.currentLines == 0 {
			continue
		}
		if state.lastSeen.Add(m.maxWaitTime).After(now) {
			continue
		}
		resolved = append(resolved, m.collapse(state))
	}
	return resolved, nil
}

// collapse emits the accumulated block and resets it. startLine isn't
// reset, so Labels/Extracted are cloned here to keep a later mutation
// from leaking into it.
func (m *multilineStage3) collapse(state *multiline3State) Entry {
	collapsed := state.startLine
	collapsed.Labels = state.startLine.Labels.Clone()
	collapsed.Extracted = maps.Clone(state.startLine.Extracted)
	collapsed.Line = state.buffer.String()

	state.buffer.Reset()
	state.currentLines = 0
	return collapsed
}

func (m *multilineStage3) Cleanup() {}

// staticLabelsStage3 is the simplest kind of stage: it mutates every
// entry it's given in place and returns the same slice, unchanged in
// length.
type staticLabelsStage3 struct {
	values []string
}

func newStaticLabelsStage3(values []string) *staticLabelsStage3 {
	return &staticLabelsStage3{values: values}
}

func (s *staticLabelsStage3) Process(_ context.Context, entries []Entry) ([]Entry, error) {
	for i := range entries {
		for j := 0; j < len(s.values); j += 2 {
			entries[i].Labels[model.LabelName(s.values[j])] = model.LabelValue(s.values[j+1])
		}
	}
	return entries, nil
}

func (s *staticLabelsStage3) Cleanup() {}

// matchStage3 is the slice-based port of matchStage2. Instead of forwarding
// the same next into a nested pipeline, it partitions entries into
// matched and unmatched, runs the nested pipeline on just the matched
// ones, and appends the result to the unmatched ones. Relative order
// between matched and unmatched entries isn't preserved -- the nested
// pipeline can change how many matched entries there are, so there's no
// single "right" position to splice them back into.
type matchStage3 struct {
	match    func(Entry) bool
	action   string // MatchActionKeep or MatchActionDrop
	pipeline *Pipeline3
}

func newMatchStage3(match func(Entry) bool, action string, pipeline *Pipeline3) *matchStage3 {
	return &matchStage3{match: match, action: action, pipeline: pipeline}
}

func (m *matchStage3) Process(ctx context.Context, entries []Entry) ([]Entry, error) {
	var unmatched, matched []Entry
	for _, e := range entries {
		if m.match(e) {
			matched = append(matched, e)
		} else {
			unmatched = append(unmatched, e)
		}
	}

	switch m.action {
	case MatchActionDrop:
		return unmatched, nil
	case MatchActionKeep:
		kept, err := m.pipeline.Process(ctx, matched)
		if err != nil {
			return nil, err
		}
		return append(unmatched, kept...), nil
	default:
		return entries, nil
	}
}

func (m *matchStage3) Cleanup() {}

// NextDeadline and Flush make matchStage3 itself a Stage3WithFlush by
// delegating to the nested pipeline, the same way matchStage2 does.
func (m *matchStage3) NextDeadline() time.Time {
	if m.pipeline == nil {
		return time.Time{}
	}
	return m.pipeline.NextDeadline()
}

func (m *matchStage3) Flush(ctx context.Context) ([]Entry, error) {
	if m.pipeline == nil {
		return nil, nil
	}
	return m.pipeline.Flush(ctx)
}
