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

const multilineFlushTimeout = 10 * time.Second

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
	_ entryProcessor = (*multilineStage)(nil)
	_ starter        = (*multilineStage)(nil)
	_ stopper        = (*multilineStage)(nil)
)

// newMultilineStage creates a multilineStage from config
func newMultilineStage(config MultilineConfig, opts stageOpts) (*multilineStage, error) {
	regex, err := validateMultilineConfig(config)
	if err != nil {
		return nil, err
	}

	return &multilineStage{
		next:    opts.next,
		logger:  opts.slogger.With("stage", "multiline"),
		cfg:     config,
		regex:   regex,
		streams: newMultilineStreamsStriped(),
		done:    make(chan struct{}),
	}, nil
}

// multilineStage matches lines to determine whether the following lines belong to a block and should be collapsed
type multilineStage struct {
	next   nextFn
	logger *slog.Logger
	cfg    MultilineConfig
	regex  *regexp.Regexp

	streams *multilineStreamsStriped

	done chan struct{}
	once sync.Once
	wg   sync.WaitGroup
}

func (m *multilineStage) process(ctx context.Context, entries []Entry) error {
	var dst int

	for _, e := range entries {
		fp := e.Labels.FastFingerprint()
		m.streams.Mutate(fp, func(state *multilineState, hasState bool) *multilineState {
			isFirstLine := m.regex.MatchString(e.Line)

			if !hasState {
				// Stream does not have any existing state and it's not identified
				// as the first line of a multiline block so we forward as is.
				if !isFirstLine {
					entries[dst] = e
					dst++
					return nil
				}

				// First time we see start of a multiline block so we initiate empty state.
				state = &multilineState{buffer: new(bytes.Buffer)}
			}

			switch {
			// Start of new multiline block, flush previous state and set new state
			case isFirstLine:
				if state.currentLines > 0 {
					entries[dst] = state.flush()
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
				entries[dst] = state.flush()
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
				entries[dst] = state.flush()
				dst++
			}

			return state
		})
	}

	if dst == 0 {
		return nil
	}

	return m.next(ctx, entries[:dst])
}

// start implements starter.
func (m *multilineStage) start() {
	m.wg.Go(func() {
		// process already flushes an expired block as soon as the next entry
		// for that stream arrives, so this only flushes streams that went
		// quiet. Sweeping at half the age threshold caps how late such a
		// block goes out at 1.5*MaxWaitTime.
		interval := max(time.Nanosecond, m.cfg.MaxWaitTime/2)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-m.done:
				return
			case <-ticker.C:
				// FlushOlderThan releases the stripe locks before we get here,
				// so an entry arriving for a flushed stream can reach next
				// ahead of the block we are about to send. This can cause OOO
				// for entries, which is an acceptable trade-off for now since
				// it only affects blocks that already timed out. Something we
				// can revisit in the future.
				expired := m.streams.FlushOlderThan(time.Now().Add(-m.cfg.MaxWaitTime))
				if len(expired) > 0 {
					ctx, cancel := context.WithTimeout(context.Background(), multilineFlushTimeout)
					if err := m.next(ctx, expired); err != nil {
						m.logger.Error("failed to flush", "err", err)
					}
					cancel()
				}
			}
		}
	})
}

// stop implements stopper.
func (m *multilineStage) stop() {
	m.once.Do(func() { close(m.done) })
	m.wg.Wait()

	entries := m.streams.FlushAll()
	if len(entries) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), multilineFlushTimeout)
		defer cancel()
		if err := m.next(ctx, entries); err != nil {
			m.logger.Error("failed to flush", "err", err)
		}
	}
}

type multilineStripe struct {
	mu   sync.Mutex
	data map[model.Fingerprint]*multilineState
}

func newMultilineStreamsStriped() *multilineStreamsStriped {
	s := &multilineStreamsStriped{}
	for i := range s.stripes {
		s.stripes[i].data = make(map[model.Fingerprint]*multilineState)
	}
	return s
}

type multilineStreamsStriped struct {
	stripes [stripeCount]multilineStripe
}

func (m *multilineStreamsStriped) Mutate(fp model.Fingerprint, fn func(s *multilineState, ok bool) *multilineState) {
	stripe := m.stripe(fp)
	stripe.mu.Lock()
	defer stripe.mu.Unlock()

	state, hasState := stripe.data[fp]
	state = fn(state, hasState)
	if state != nil {
		stripe.data[fp] = state
	}
}

func (m *multilineStreamsStriped) stripe(fp model.Fingerprint) *multilineStripe {
	return &m.stripes[fp&(stripeCount-1)]
}

func (m *multilineStreamsStriped) FlushAll() []Entry {
	var buf []Entry

	for i := range m.stripes {
		m.stripes[i].mu.Lock()
		for _, s := range m.stripes[i].data {
			if s.currentLines > 0 {
				buf = append(buf, s.flush())
			}
		}
		clear(m.stripes[i].data)
		m.stripes[i].mu.Unlock()
	}

	return buf
}

func (m *multilineStreamsStriped) FlushOlderThan(t time.Time) []Entry {
	var expired []Entry

	for i := range m.stripes {
		m.stripes[i].mu.Lock()
		for fp, s := range m.stripes[i].data {
			if !s.lastSeen.After(t) {
				if s.currentLines > 0 {
					expired = append(expired, s.flush())
				}
				delete(m.stripes[i].data, fp)
			}
		}
		m.stripes[i].mu.Unlock()
	}

	return expired
}

// multilineState captures the internal state of a running multiline stage.
type multilineState struct {
	buffer         *bytes.Buffer
	startLineEntry Entry  // The entry of the start line of a multiline block.
	currentLines   uint64 // The number of lines of the current multiline block.
	lastSeen       time.Time
}

// flush collapses the accumulated block into a single entry and resets
// the line counter and buffer. startLineEntry is intentionally not reset so
// that subsequent lines (before the next start line) inherit its metadata.
func (s *multilineState) flush() Entry {
	// copy extracted data.
	extracted := make(map[string]any, len(s.startLineEntry.Extracted))
	for k, v := range s.startLineEntry.Extracted {
		extracted[k] = v
	}
	collapsed := Entry{
		Extracted: extracted,
		Entry: loki.NewEntryWithCreatedUnixMicro(
			s.startLineEntry.Entry.Labels.Clone(),
			s.startLineEntry.Created(),
			push.Entry{
				Timestamp:          s.startLineEntry.Entry.Entry.Timestamp,
				Line:               s.buffer.String(),
				StructuredMetadata: slices.Clone(s.startLineEntry.Entry.Entry.StructuredMetadata),
			}),
	}

	s.buffer.Reset()
	s.currentLines = 0

	return collapsed
}
