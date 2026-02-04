package stages

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/loki/pkg/push"
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
	logger log.Logger
	cfg    MultilineConfig
	regex  *regexp.Regexp
}

// multilineState captures the internal state of a running multiline stage.
type multilineState struct {
	buffer         *bytes.Buffer // The lines of the current multiline block.
	startLineEntry Entry         // The entry of the start line of a multiline block.
	currentLines   uint64        // The number of lines of the current multiline block.
}

// newMultilineStage creates a MulitlineStage from config
func newMultilineStage(logger log.Logger, config MultilineConfig) (Stage, error) {
	regex, err := validateMultilineConfig(config)
	if err != nil {
		return nil, err
	}

	return &multilineStage{
		logger: log.With(logger, "component", "stage", "type", "multiline"),
		cfg:    config,
		regex:  regex,
	}, nil
}

func (m *multilineStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)

		streams := make(map[model.Fingerprint]chan Entry)
		wg := new(sync.WaitGroup)

		for e := range in {
			key := e.Labels.FastFingerprint()
			s, ok := streams[key]
			if !ok {
				// Pass through entries until we hit first start line.
				if !m.regex.MatchString(e.Line) {
					level.Debug(m.logger).Log("msg", "pass through entry", "stream", key)
					out <- e
					continue
				}

				level.Debug(m.logger).Log("msg", "creating new stream", "stream", key)
				s = make(chan Entry)
				streams[key] = s

				wg.Add(1)
				go m.runMultiline(s, out, wg)
			}
			level.Debug(m.logger).Log("msg", "pass entry", "stream", key, "line", e.Line)
			s <- e
		}

		// Close all streams and wait for them to finish being processed.
		for _, s := range streams {
			close(s)
		}
		wg.Wait()
	}()
	return out
}

func (m *multilineStage) runMultiline(in chan Entry, out chan Entry, wg *sync.WaitGroup) {
	defer wg.Done()

	state := &multilineState{
		buffer:       new(bytes.Buffer),
		currentLines: 0,
	}

	for {
		select {
		case <-time.After(m.cfg.MaxWaitTime):
			level.Debug(m.logger).Log("msg", fmt.Sprintf("flush multiline block due to %v timeout", m.cfg.MaxWaitTime), "block", state.buffer.String())
			m.flush(out, state)
		case e, ok := <-in:
			level.Debug(m.logger).Log("msg", "processing line", "line", e.Line, "stream", e.Labels.FastFingerprint())

			if !ok {
				level.Debug(m.logger).Log("msg", "flush multiline block because inbound closed", "block", state.buffer.String(), "stream", e.Labels.FastFingerprint())
				m.flush(out, state)
				return
			}

			isFirstLine := m.regex.MatchString(e.Line)
			if isFirstLine {
				level.Debug(m.logger).Log("msg", "flush multiline block because new start line", "block", state.buffer.String(), "stream", e.Labels.FastFingerprint())
				m.flush(out, state)

				// The start line entry is used to set timestamp and labels in the flush method.
				// The timestamps for following lines are ignored for now.
				state.startLineEntry = e
			}

			// Append block line
			if state.buffer.Len() > 0 {
				state.buffer.WriteRune('\n')
			}
			line := e.Line
			if m.cfg.TrimNewlines {
				line = strings.TrimRight(line, "\r\n")
			}
			state.buffer.WriteString(line)
			state.currentLines++

			if state.currentLines == m.cfg.MaxLines {
				m.flush(out, state)
			}
		}
	}
}

func (m *multilineStage) flush(out chan Entry, s *multilineState) {
	if s.buffer.Len() == 0 {
		level.Debug(m.logger).Log("msg", "nothing to flush", "buffer_len", s.buffer.Len())
		return
	}
	// copy extracted data.
	extracted := make(map[string]any, len(s.startLineEntry.Extracted))
	for k, v := range s.startLineEntry.Extracted {
		extracted[k] = v
	}
	collapsed := Entry{
		Extracted: extracted,
		Entry: loki.Entry{
			Labels: s.startLineEntry.Entry.Labels.Clone(),
			Entry: push.Entry{
				Timestamp:          s.startLineEntry.Entry.Entry.Timestamp,
				Line:               s.buffer.String(),
				StructuredMetadata: slices.Clone(s.startLineEntry.Entry.Entry.StructuredMetadata),
			},
		},
	}
	s.buffer.Reset()
	s.currentLines = 0

	out <- collapsed
}

// Name implements Stage
func (m *multilineStage) Name() string {
	return StageTypeMultiline
}

// Cleanup implements Stage.
func (*multilineStage) Cleanup() {
	// no-op
}
