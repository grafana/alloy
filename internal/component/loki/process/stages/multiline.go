package stages

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
)

var (
	errMultilineStageEmptyConfig  = errors.New("multiline stage config must define `firstline` regular expression")
	errMultilineStageInvalidRegex = errors.New("multiline stage first line regex compilation error")
)

// MultilineConfig contains the configuration for a Multiline stage.
type MultilineConfig struct {
	Expression   string        `alloy:"firstline,attr"`
	MaxLines     uint64        `alloy:"max_lines,attr,optional"`
	MaxWaitTime  time.Duration `alloy:"max_wait_time,attr,optional"`
	TrimNewlines bool          `alloy:"trim_newlines,attr,optional"`
}

// defaultMultilineConfig applies the default values on
var defaultMultilineConfig = MultilineConfig{
	MaxLines:     128,
	MaxWaitTime:  3 * time.Second,
	TrimNewlines: true,
}

// SetToDefault implements syntax.Defaulter.
func (args *MultilineConfig) SetToDefault() {
	*args = defaultMultilineConfig
}

// Validate implements syntax.Validator.
func (args *MultilineConfig) Validate() error {
	if args.MaxWaitTime <= 0 {
		return fmt.Errorf("max_wait_time must be greater than 0")
	}

	return nil
}

func validateMultilineConfig(cfg MultilineConfig) (*regexp.Regexp, error) {
	if cfg.Expression == "" {
		return nil, errMultilineStageEmptyConfig
	}

	expr, err := regexp.Compile(cfg.Expression)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", errMultilineStageInvalidRegex, err)
	}

	return expr, nil
}

var (
	_ Stage = (*multilineStage)(nil)

	_ entryProcessor = (*multilineStage)(nil)
	_ starter        = (*multilineStage)(nil)
	_ stopper        = (*multilineStage)(nil)
)

// newMultilineStage creates a mulitlineStage from config
func newMultilineStage(logger *slog.Logger, config MultilineConfig, next NextFn) (Stage, error) {
	regex, err := validateMultilineConfig(config)
	if err != nil {
		return nil, err
	}

	return &multilineStage{
		next:    next,
		logger:  logger.With("stage", "multiline"),
		cfg:     config,
		regex:   regex,
		streams: make(map[model.Fingerprint]*multilineState),
		done:    make(chan struct{}),
	}, nil
}

// multilineStage matches lines to determine whether the following lines belong to a block and should be collapsed
type multilineStage struct {
	next   NextFn
	logger *slog.Logger
	cfg    MultilineConfig
	regex  *regexp.Regexp

	// mut is only used when we are running with pipeline2.
	mut     sync.Mutex
	streams map[model.Fingerprint]*multilineState

	done chan struct{}
	once sync.Once
	wg   sync.WaitGroup
}

// multilineState captures the internal state of a running multiline stage.
type multilineState struct {
	buffer         *bytes.Buffer
	startLineEntry Entry  // The entry of the start line of a multiline block.
	currentLines   uint64 // The number of lines of the current multiline block.
	lastSeen       time.Time
}

func (m *multilineStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)

		// timer fires at the earliest per-stream deadline (lastSeen + MaxWaitTime).
		// Start it stopped; it is armed on the first entry that starts a block.
		timer := time.NewTimer(0)
		if !timer.Stop() {
			<-timer.C
		}

		// nearestDeadline tracks the earliest active stream deadline so that
		// we can easily update the timer if the incoming entry has a newer deadline.
		var nearestDeadline time.Time

		armTimer := func(deadline time.Time) {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(max(0, time.Until(deadline)))
			nearestDeadline = deadline
		}

		// isEarlierDeadline reports whether candidate should replace current as
		// the nearest deadline (i.e. it is earlier, or there is no current deadline).
		isEarlierDeadline := func(candidate, current time.Time) bool {
			return current.IsZero() || candidate.Before(current)
		}

		// rescanDeadline rescans all streams to find the new nearest deadline
		// after a flush removes a stream from contention. We include streams with
		// currentLines==0 (flushed at max_lines) so they are cleaned up by the
		// timer even when idle.
		rescanDeadline := func() {
			nearestDeadline = time.Time{}
			for _, state := range m.streams {
				if dl := state.lastSeen.Add(m.cfg.MaxWaitTime); isEarlierDeadline(dl, nearestDeadline) {
					nearestDeadline = dl
				}
			}
			if nearestDeadline.IsZero() {
				return // no streams; leave timer stopped
			}
			armTimer(nearestDeadline)
		}

		for {
			select {
			case e, ok := <-in:
				if !ok {
					// Flush all per-stream buffers when the input closes.
					for _, state := range m.streams {
						if state.currentLines > 0 {
							out <- m.flushState(state)
						}
					}
					m.streams = nil
					timer.Stop()
					return
				}
				// Capture the stream key before emitting entries downstream.
				// A downstream stage goroutine may mutate e.Labels concurrently
				// once the entry is sent on out, which would race with a
				// post-emit FastFingerprint() call.
				key := e.Labels.FastFingerprint()
				for _, r := range m.processEntry(key, e) {
					out <- r
				}
				// Arm the timer for any stream that now has the earliest deadline,
				// including streams where currentLines==0 (just hit max_lines) so
				// the timer fires to remove them if they subsequently go idle.
				if m.streams[key] != nil {
					if dl := m.streams[key].lastSeen.Add(m.cfg.MaxWaitTime); isEarlierDeadline(dl, nearestDeadline) {
						armTimer(dl)
					}
				}
			case <-timer.C:
				nearestDeadline = time.Time{}
				// Remove every stream whose deadline has been reached. Flush its
				// buffer if it has accumulated lines; streams with currentLines==0
				// (flushed at max_lines and then gone idle) are deleted.
				now := time.Now()
				for key, state := range m.streams {
					if !state.lastSeen.Add(m.cfg.MaxWaitTime).After(now) {
						if state.currentLines > 0 {
							if debugEnabled(m.logger) {
								m.logger.Debug("flush multiline block due to timeout", "timeout", m.cfg.MaxWaitTime, "block", state.buffer.String())
							}
							out <- m.flushState(state)
						}
						delete(m.streams, key)
					}
				}
				rescanDeadline()
			}
		}
	}()
	return out
}

// process implements stage.
func (m *multilineStage) process(ctx context.Context, entries []Entry) error {
	m.mut.Lock()
	var dst int

	for _, e := range entries {
		key := e.Labels.FastFingerprint()

		if m.streams == nil {
			m.streams = make(map[model.Fingerprint]*multilineState)
		}

		state, hasState := m.streams[key]
		isFirstLine := m.regex.MatchString(e.Line)

		if !hasState {
			// Stream does not have any existing state and it's not identified
			// as the first line of a multiline block so we forward as is.
			if !isFirstLine {
				entries[dst] = e
				dst++
				continue
			}

			// First time we see start of a multiline block so we initiate empty state.
			state = &multilineState{buffer: new(bytes.Buffer)}
			m.streams[key] = state
		}

		switch {
		// Start of new multiline block, flush previous state and set new state
		case isFirstLine:
			if state.currentLines > 0 {
				entries[dst] = m.flushState(state)
				dst++
			}

			state.startLineEntry = e
			line := e.Line
			if m.cfg.TrimNewlines {
				line = strings.TrimRight(line, "\r\n")
			}

			state.buffer.WriteString(line)
			state.currentLines++
			state.lastSeen = time.Now()
		// Not a new block but we have a stale block that we need to flush.
		case state.currentLines > 0 && time.Since(state.lastSeen) >= m.cfg.MaxWaitTime:
			entries[dst] = m.flushState(state)
			dst++

			line := e.Line
			if m.cfg.TrimNewlines {
				line = strings.TrimRight(line, "\r\n")
			}

			state.buffer.WriteString(line)
			state.currentLines++
			state.lastSeen = time.Now()
		// Append to existing multiline block.
		default:
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

		}

		// Three places can write to entries[dst]: the isFirstLine case,
		// stale-block case, and this check. The two cases are
		// mutually exclusive switch branches and whichever does resets
		// currentLines to 0 before this entry is appended.
		// So if any of these cases wrote state.currentLines will be 1
		// and can only match if MaxLines == 1. But if MaxLines == 1 we
		// always flush the entry and none of the cases above will ever
		// perform their write.
		if state.currentLines == m.cfg.MaxLines {
			entries[dst] = m.flushState(state)
			dst++
		}
	}

	m.mut.Unlock()

	if dst == 0 {
		return nil
	}

	// FIXME(kallep): If we fail future down in the pipeline we should restore the state.
	return m.next(ctx, entries[:dst])
}

// start implements starter.
func (m *multilineStage) start() {
	m.wg.Go(func() {
		ticker := time.NewTicker(m.cfg.MaxWaitTime)
		defer ticker.Stop()

		for {
			select {
			case <-m.done:
				return
			case <-ticker.C:
				m.mut.Lock()
				var (
					expired []Entry
					now     = time.Now()
				)

				for key, state := range m.streams {
					if !state.lastSeen.Add(m.cfg.MaxWaitTime).After(now) {
						if state.currentLines > 0 {
							expired = append(expired, m.flushState(state))
						}
						delete(m.streams, key)
					}
				}

				if len(expired) > 0 {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					// FIXME(kallep): If we fail future down in the pipeline we should restore the state.
					if err := m.next(ctx, expired); err != nil {
						m.logger.Error("failed to flush", "err", err)
					}
					cancel()
				}
				m.mut.Unlock()
			}
		}
	})
}

// stop implements stopper.
func (m *multilineStage) stop() {
	m.once.Do(func() { close(m.done) })
	m.wg.Wait()

	m.mut.Lock()
	entries := make([]Entry, 0, len(m.streams))
	for _, state := range m.streams {
		if state.currentLines > 0 {
			entries = append(entries, m.flushState(state))
		}
	}
	m.streams = nil
	m.mut.Unlock()

	if len(entries) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := m.next(ctx, entries); err != nil {
			m.logger.Error("failed to flush", "err", err)
		}
	}
}

// processEntry processes a single entry synchronously, returning any entries
// ready to emit. Before the first start line is seen for a stream, non-start
// lines are passed through unchanged. Once a stream is started, all lines are
// accumulated
func (m *multilineStage) processEntry(key model.Fingerprint, e Entry) []Entry {
	if m.streams == nil {
		m.streams = make(map[model.Fingerprint]*multilineState)
	}
	state, hasState := m.streams[key]

	var out []Entry

	// flush stale block before processing new entry.
	if hasState && state.currentLines > 0 && time.Since(state.lastSeen) >= m.cfg.MaxWaitTime {
		if debugEnabled(m.logger) {
			m.logger.Debug("flush multiline block due to timeout", "timeout", m.cfg.MaxWaitTime, "block", state.buffer.String())
		}
		out = append(out, m.flushState(state))
	}

	isFirstLine := m.regex.MatchString(e.Line)
	if !hasState {
		// Pass through entries until the first start line for this stream.
		if !isFirstLine {
			if debugEnabled(m.logger) {
				m.logger.Debug("pass through entry", "stream", key)
			}
			return append(out, e)
		}
		state = &multilineState{buffer: new(bytes.Buffer)}
		m.streams[key] = state
	}

	// Stream is active: flush current block if a new start line arrived.
	if isFirstLine && state.currentLines > 0 {
		if debugEnabled(m.logger) {
			m.logger.Debug("flush multiline block because new start line", "block", state.buffer.String(), "stream", key)
		}
		out = append(out, m.flushState(state))
	}
	// startLineEntry is only updated on start lines; it is intentionally
	// preserved across max_lines flushes to match the original behaviour.
	if isFirstLine {
		state.startLineEntry = e
	}

	if debugEnabled(m.logger) {
		m.logger.Debug("processing line", "line", e.Line, "stream", key)
	}
	// Append line to buffer.
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

	if state.currentLines == m.cfg.MaxLines {
		out = append(out, m.flushState(state))
	}

	return out
}

// flushState collapses the accumulated block into a single entry and resets
// the line counter and buffer. startLineEntry is intentionally not reset so
// that subsequent lines (before the next start line) inherit its metadata.
func (m *multilineStage) flushState(s *multilineState) Entry {
	// copy extracted data.
	extracted := make(map[string]any, len(s.startLineEntry.Extracted))
	for k, v := range s.startLineEntry.Extracted {
		extracted[k] = v
	}
	collapsed := Entry{
		Extracted: extracted,
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

// Cleanup implements Stage.
func (*multilineStage) Cleanup() {}
