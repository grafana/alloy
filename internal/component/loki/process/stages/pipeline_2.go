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

	crip "github.com/grafana/alloy/internal/component/loki/process/stages/cri"
)

// Emitter receives entries produced by a stage or a chain of stages.
type Emitter interface {
	Emit(ctx context.Context, e Entry) error
}

// EmitterFunc adapts a function to an Emitter.
type EmitterFunc func(ctx context.Context, e Entry) error

func (f EmitterFunc) Emit(ctx context.Context, e Entry) error { return f(ctx, e) }

// Stage2 is a stage in a continuation-passing pipeline: given an entry
// and an Emitter representing everything after it, Process decides
// whether, how many times, and with what mutations to call next.
type Stage2 interface {
	Process(ctx context.Context, e Entry, next Emitter) error
	Cleanup()
}

// Stage2WithFlush is a Stage2 that can also emit entries independent of
// any Process call, on its own schedule (e.g. an idle timeout). Flush is
// driven by something other than a caller -- a timer, most likely -- so
// it must take whatever lock Process takes on the same stage, the same
// way criStage2's mu already guards both paths.
type Stage2WithFlush interface {
	Stage2
	// NextDeadline reports when this stage next has something ready to
	// flush, or the zero Time if nothing is pending.
	NextDeadline() time.Time
	// Flush emits whatever is due to next.
	Flush(ctx context.Context, next Emitter) error
}

// flusherEntry pairs a Stage2WithFlush with its position in stages, so a
// flush can be routed through everything after it.
type flusherEntry struct {
	idx int
	f   Stage2WithFlush
}

// Pipeline2 chains Stage2 stages together with direct function calls
// instead of channels.
type Pipeline2 struct {
	stages   []Stage2
	flushers []flusherEntry
}

// NewPipeline2 builds a Pipeline2 from stages, in order.
func NewPipeline2(stages []Stage2) *Pipeline2 {
	p := &Pipeline2{stages: stages}
	for i, s := range stages {
		if f, ok := s.(Stage2WithFlush); ok {
			p.flushers = append(p.flushers, flusherEntry{idx: i, f: f})
		}
	}
	return p
}

// Process pushes a single entry through the pipeline, starting at the
// first stage. Whatever the last stage emits is handed to next.
func (p *Pipeline2) Process(ctx context.Context, e Entry, next Emitter) error {
	return p.chainFrom(0, next).Emit(ctx, e)
}

// NextDeadline reports the earliest deadline across every stage that
// implements Stage2WithFlush, or the zero Time if nothing is pending
// anywhere in the pipeline.
func (p *Pipeline2) NextDeadline() time.Time {
	var nearest time.Time
	for _, fl := range p.flushers {
		if dl := fl.f.NextDeadline(); !dl.IsZero() && (nearest.IsZero() || dl.Before(nearest)) {
			nearest = dl
		}
	}
	return nearest
}

// Flush flushes whichever Stage2WithFlush stages have a deadline that
// has already passed, routing whatever each one emits through the rest
// of the pipeline into next.
func (p *Pipeline2) Flush(ctx context.Context, next Emitter) error {
	now := time.Now()
	for _, fl := range p.flushers {
		dl := fl.f.NextDeadline()
		if dl.IsZero() || dl.After(now) {
			continue
		}
		next := p.chainFrom(fl.idx+1, next)
		if err := fl.f.Flush(ctx, next); err != nil {
			return err
		}
	}
	return nil
}

// chainFrom builds an Emitter that runs stages[from:] in order before
// handing whatever comes out to next.
func (p *Pipeline2) chainFrom(from int, next Emitter) Emitter {
	chain := next
	for _, stage := range slices.Backward(p.stages[from:]) {
		cur := chain
		chain = EmitterFunc(func(ctx context.Context, e Entry) error {
			return stage.Process(ctx, e, cur)
		})
	}
	return chain
}

func (p *Pipeline2) Stop() {
	// TODO: Implement Stop

}

// criStage2 is a simplified port of the real cri stage. Partial lines are
// held across Process calls in instead of being forwarded, and a later call can
// emit zero, one, or many entries once they're resolved or force-flushed.
type criStage2 struct {
	maxPartialLines int

	mu           sync.Mutex
	partialLines map[model.Fingerprint]Entry
}

func newCRIStage2(maxPartialLines int) *criStage2 {
	return &criStage2{
		maxPartialLines: maxPartialLines,
		partialLines:    make(map[model.Fingerprint]Entry, maxPartialLines),
	}
}

func (c *criStage2) Process(ctx context.Context, e Entry, next Emitter) error {
	parsed, ok := crip.ParseCRI(e.Line)
	if !ok {
		return next.Emit(ctx, e)
	}
	e.Line = parsed.Content

	fingerprint := e.Labels.Fingerprint()

	c.mu.Lock()

	if parsed.Flag == crip.FlagPartial {
		var entries []Entry
		if len(c.partialLines) >= c.maxPartialLines {
			entries = make([]Entry, 0, len(c.partialLines))
			for _, buffered := range c.partialLines {
				entries = append(entries, buffered)
			}
			c.partialLines = make(map[model.Fingerprint]Entry, c.maxPartialLines)
		}
		if prev, ok := c.partialLines[fingerprint]; ok {
			e.Line = prev.Line + e.Line
		}
		c.partialLines[fingerprint] = e
		c.mu.Unlock()

		for _, buffered := range entries {
			if err := next.Emit(ctx, buffered); err != nil {
				return err
			}
		}
		return nil // held: e itself isn't emitted this call
	}

	if prev, ok := c.partialLines[fingerprint]; ok {
		e.Line = prev.Line + e.Line
		delete(c.partialLines, fingerprint)
	}
	c.mu.Unlock()

	return next.Emit(ctx, e)
}

func (c *criStage2) Cleanup() {}

type multilineState2 struct {
	buffer       strings.Builder
	startLine    Entry
	currentLines int
	lastSeen     time.Time
}

// multilineStage2 is a simplified port of the real multiline stage. Like
// criStage2 it holds per-stream state across Process calls, but it also
// implements Stage2WithFlush, since a block can become due with no new
// entry ever arriving.
type multilineStage2 struct {
	regex        *regexp.Regexp
	maxLines     int
	maxWaitTime  time.Duration
	trimNewlines bool

	mu      sync.Mutex
	streams map[model.Fingerprint]*multilineState2
}

func newMultilineStage2(regex *regexp.Regexp, maxLines int, maxWaitTime time.Duration, trimNewlines bool) *multilineStage2 {
	return &multilineStage2{
		regex:        regex,
		maxLines:     maxLines,
		maxWaitTime:  maxWaitTime,
		trimNewlines: trimNewlines,
		streams:      make(map[model.Fingerprint]*multilineState2),
	}
}

func (m *multilineStage2) Process(ctx context.Context, e Entry, next Emitter) error {
	fp := e.Labels.Fingerprint()

	m.mu.Lock()

	var (
		resolved    []Entry
		state, ok   = m.streams[fp]
		isFirstLine = m.regex.MatchString(e.Line)
	)

	if !ok && !isFirstLine {
		m.mu.Unlock()
		// never part of a block: pass through
		return next.Emit(ctx, e)
	}

	// A new entry arriving for a stream that's already gone stale
	// resolves the old block now, without waiting for a Flush call.
	if ok && state.currentLines > 0 && time.Since(state.lastSeen) >= m.maxWaitTime {
		resolved = append(resolved, m.collapse(state))
	}

	if !ok {
		state = &multilineState2{}
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

	m.mu.Unlock()

	for _, r := range resolved {
		if err := next.Emit(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// NextDeadline implements Stage2WithFlush.
func (m *multilineStage2) NextDeadline() time.Time {
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

// Flush implements Stage2WithFlush, collapsing every stream past its
// deadline.
func (m *multilineStage2) Flush(ctx context.Context, next Emitter) error {
	m.mu.Lock()
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
	m.mu.Unlock()

	for _, e := range resolved {
		if err := next.Emit(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// collapse emits the accumulated block and resets it. startLine isn't
// reset, so Labels/Extracted are cloned here to keep a later mutation
// from leaking into it.
func (m *multilineStage2) collapse(state *multilineState2) Entry {
	collapsed := state.startLine
	collapsed.Labels = state.startLine.Labels.Clone()
	collapsed.Extracted = maps.Clone(state.startLine.Extracted)
	collapsed.Line = state.buffer.String()

	state.buffer.Reset()
	state.currentLines = 0
	return collapsed
}

func (m *multilineStage2) Cleanup() {}

// staticLabelsStage2 is the simplest kind of stage. It just processes the
// current entry and always forwards it to next.
type staticLabelsStage2 struct {
	values []string
}

func newStaticLabelsStage2(values []string) *staticLabelsStage2 {
	return &staticLabelsStage2{values: values}
}

func (s *staticLabelsStage2) Process(ctx context.Context, e Entry, next Emitter) error {
	for i := 0; i < len(s.values); i += 2 {
		e.Labels[model.LabelName(s.values[i])] = model.LabelValue(s.values[i+1])
	}
	return next.Emit(ctx, e)
}

func (s *staticLabelsStage2) Cleanup() {}

// matchStage2 is a simplified port of matcherStage. Its unique
// property is sub pipeline when using MatchActionKeep, a matched entry is handed
// to a Pipeline2 with the same next, so whatever that pipeline
// emits continues on as if this stage had emitted it directly.
type matchStage2 struct {
	match    func(Entry) bool
	action   string // MatchActionKeep or MatchActionDrop
	pipeline *Pipeline2
}

func newMatchStage2(match func(Entry) bool, action string, pipeline *Pipeline2) *matchStage2 {
	return &matchStage2{match: match, action: action, pipeline: pipeline}
}

func (m *matchStage2) Process(ctx context.Context, e Entry, next Emitter) error {
	if !m.match(e) {
		return next.Emit(ctx, e)
	}

	switch m.action {
	case MatchActionDrop:
		return nil
	case MatchActionKeep:
		return m.pipeline.Process(ctx, e, next)
	default:
		return next.Emit(ctx, e)
	}
}

func (m *matchStage2) Cleanup() {}

// NextDeadline and Flush make matchStage2 itself a Stage2WithFlush by
// delegating to the nested pipeline. Without this, a Stage2WithFlush
// nested inside a match block (e.g. multilineStage2) would be invisible
// to the outer Pipeline2's flush scan, since that scan only looks at its
// own stages, not into each one's internals.
func (m *matchStage2) NextDeadline() time.Time {
	if m.pipeline == nil {
		return time.Time{}
	}
	return m.pipeline.NextDeadline()
}

func (m *matchStage2) Flush(ctx context.Context, next Emitter) error {
	if m.pipeline == nil {
		return nil
	}
	return m.pipeline.Flush(ctx, next)
}
