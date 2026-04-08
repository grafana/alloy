package stages

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/loki/pkg/push"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// Configuration errors.
var (
	ErrMultilineStageEmptyConfig  = errors.New("multiline stage config must define `firstline` regular expression")
	ErrMultilineStageInvalidRegex = errors.New("multiline stage first line regex compilation error")
)

// MultilineConfig contains the configuration for a Multiline stage.
type MultilineConfig struct {
	Expression   string        `alloy:"firstline,attr"`
	MaxLines     uint64        `alloy:"max_lines,attr,optional"`
	MaxWaitTime  time.Duration `alloy:"max_wait_time,attr,optional"`
	TrimNewlines bool          `alloy:"trim_newlines,attr,optional"`
}

// DefaultMultilineConfig applies the default values on
var DefaultMultilineConfig = MultilineConfig{
	MaxLines:     128,
	MaxWaitTime:  3 * time.Second,
	TrimNewlines: true,
}

// SetToDefault implements syntax.Defaulter.
func (args *MultilineConfig) SetToDefault() {
	*args = DefaultMultilineConfig
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
		return nil, ErrMultilineStageEmptyConfig
	}

	expr, err := regexp.Compile(cfg.Expression)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", ErrMultilineStageInvalidRegex, err)
	}

	return expr, nil
}

// multilineStage matches lines to determine whether the following lines belong to a block and should be collapsed
type multilineStage struct {
	logger  log.Logger
	cfg     MultilineConfig
	regex   *regexp.Regexp
	streams map[model.Fingerprint]*multilineState
}

// multilineState captures the internal state of a running multiline stage.
type multilineState struct {
	buffer         *bytes.Buffer
	startLineEntry Entry  // The entry of the start line of a multiline block.
	currentLines   uint64 // The number of lines of the current multiline block.
	lastSeen       time.Time
}

// newMultilineStage creates a MulitlineStage from config
func newMultilineStage(logger log.Logger, config MultilineConfig) (Stage, error) {
	regex, err := validateMultilineConfig(config)
	if err != nil {
		return nil, err
	}

	return &multilineStage{
		logger:  log.With(logger, "component", "stage", "type", "multiline"),
		cfg:     config,
		regex:   regex,
		streams: make(map[model.Fingerprint]*multilineState),
	}, nil
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
			if d := time.Until(deadline); d > 0 {
				timer.Reset(d)
			} else {
				timer.Reset(0)
			}
			nearestDeadline = deadline
		}

		// rescanDeadline rescans all streams to find the new nearest deadline
		// after a flush removes a stream from contention. We include streams with
		// currentLines==0 (flushed at max_lines) so they are cleaned up by the
		// timer even when idle.
		rescanDeadline := func() {
			nearestDeadline = time.Time{}
			for _, state := range m.streams {
				if dl := state.lastSeen.Add(m.cfg.MaxWaitTime); nearestDeadline.IsZero() || dl.Before(nearestDeadline) {
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
					return
				}
				for _, r := range m.processEntry(e) {
					out <- r
				}
				// Arm the timer for any stream that now has the earliest deadline,
				// including streams where currentLines==0 (just hit max_lines) so
				// the timer fires to remove them if they subsequently go idle.
				if key := e.Labels.FastFingerprint(); m.streams[key] != nil {
					if dl := m.streams[key].lastSeen.Add(m.cfg.MaxWaitTime); nearestDeadline.IsZero() || dl.Before(nearestDeadline) {
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
							if Debug {
								level.Debug(m.logger).Log("msg", fmt.Sprintf("flush multiline block due to %v timeout", m.cfg.MaxWaitTime), "block", state.buffer.String())
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

// processEntry processes a single entry synchronously, returning any entries
// ready to emit. Before the first start line is seen for a stream, non-start
// lines are passed through unchanged. Once a stream is started, all lines are
// accumulated
func (m *multilineStage) processEntry(e Entry) []Entry {
	key := e.Labels.FastFingerprint()
	if m.streams == nil {
		m.streams = make(map[model.Fingerprint]*multilineState)
	}
	state, hasState := m.streams[key]

	var out []Entry

	// flush stale block before processing new entry.
	if hasState && state.currentLines > 0 && time.Since(state.lastSeen) >= m.cfg.MaxWaitTime {
		if Debug {
			level.Debug(m.logger).Log("msg", fmt.Sprintf("flush multiline block due to %v timeout", m.cfg.MaxWaitTime), "block", state.buffer.String())
		}
		out = append(out, m.flushState(state))
	}

	isFirstLine := m.regex.MatchString(e.Line)
	if !hasState {
		// Pass through entries until the first start line for this stream.
		if !isFirstLine {
			if Debug {
				level.Debug(m.logger).Log("msg", "pass through entry", "stream", key)
			}
			return append(out, e)
		}
		state = &multilineState{buffer: new(bytes.Buffer)}
		m.streams[key] = state
	}

	// Stream is active: flush current block if a new start line arrived.
	if isFirstLine && state.currentLines > 0 {
		if Debug {
			level.Debug(m.logger).Log("msg", "flush multiline block because new start line", "block", state.buffer.String(), "stream", key)
		}
		out = append(out, m.flushState(state))
	}
	// startLineEntry is only updated on start lines; it is intentionally
	// preserved across max_lines flushes to match the original behaviour.
	if isFirstLine {
		state.startLineEntry = e
	}

	if Debug {
		level.Debug(m.logger).Log("msg", "processing line", "line", e.Line, "stream", key)
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
func (*multilineStage) Cleanup() {
	// no-op
}
