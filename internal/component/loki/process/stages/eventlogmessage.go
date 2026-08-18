package stages

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/util/strutil"
)

const (
	defaultSource = "message"
)

type EventLogMessageConfig struct {
	Source            string `alloy:"source,attr,optional"`
	DropInvalidLabels bool   `alloy:"drop_invalid_labels,attr,optional"`
	OverwriteExisting bool   `alloy:"overwrite_existing,attr,optional"`
}

func (e *EventLogMessageConfig) Validate() error {
	// TODO: add support for different validation schemes.
	//nolint:staticcheck
	if !model.LabelName(e.Source).IsValidLegacy() {
		return fmt.Errorf(errInvalidLabelName, e.Source)
	}
	return nil
}

func (e *EventLogMessageConfig) SetToDefault() {
	e.Source = defaultSource
}

var (
	_ Stage          = (*eventLogMessageStage)(nil)
	_ entryProcessor = (*eventLogMessageStage)(nil)
)

func newEventLogMessageStage(logger *slog.Logger, cfg *EventLogMessageConfig, next NextFn) Stage {
	return &eventLogMessageStage{
		next:   next,
		cfg:    cfg,
		logger: logger.With("stage", "eventlogmessage"),
	}
}

type eventLogMessageStage struct {
	next   NextFn
	cfg    *EventLogMessageConfig
	logger *slog.Logger
}

func (m *eventLogMessageStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			e, err := m.processEntry(e)
			if err != nil {
				continue
			}
			out <- e
		}
	}()
	return out
}

func (m *eventLogMessageStage) process(ctx context.Context, entries []Entry) error {
	var dst int
	for _, e := range entries {
		e, err := m.processEntry(e)
		if err != nil {
			continue
		}
		entries[dst] = e
		dst++
	}

	if dst == 0 {
		return nil
	}

	return m.next(ctx, entries[:dst])
}

// Process a event log message from extracted with the specified key, adding additional
// entries into the extracted map
func (m *eventLogMessageStage) processEntry(e Entry) (Entry, error) {
	value, ok := e.Extracted[m.cfg.Source]
	if !ok {
		if debugEnabled(m.logger) {
			m.logger.Debug("source not in the extracted values", "source", m.cfg.Source)
		}
		return e, nil
	}
	s, err := getString(value)
	if err != nil {
		m.logger.Warn("invalid label value parsed", "value", value)
		return e, err
	}
	for line := range strings.SplitSeq(s, "\r\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			m.logger.Debug("invalid line parsed from message", "line", line)
			continue
		}
		mkey := parts[0]
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(mkey).IsValidLegacy() {
			if m.cfg.DropInvalidLabels {
				if debugEnabled(m.logger) {
					m.logger.Debug("invalid label parsed from message", "key", mkey)
				}
				continue
			}
			mkey = strutil.SanitizeFullLabelName(mkey)
		}
		if _, ok := e.Extracted[mkey]; ok && !m.cfg.OverwriteExisting {
			m.logger.Info("extracted key already existed, appending _extracted to key", "key", mkey)
			mkey += "_extracted"
		}
		mval := strings.TrimSpace(parts[1])
		if !model.LabelValue(mval).IsValid() {
			if debugEnabled(m.logger) {
				m.logger.Debug("invalid value parsed from message", "value", mval)
			}
			continue
		}
		e.Extracted[mkey] = mval
	}
	if debugEnabled(m.logger) {
		m.logger.Debug("extracted data debug in event_log_message stage", "extracted_data", e.Extracted)
	}
	return e, nil
}

func (*eventLogMessageStage) Cleanup() {}
