package stages

import (
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/common/model"
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
		return fmt.Errorf(ErrInvalidLabelName, e.Source)
	}
	return nil
}

func (e *EventLogMessageConfig) SetToDefault() {
	e.Source = defaultSource
}

type eventLogMessageStage struct {
	cfg    *EventLogMessageConfig
	logger log.Logger
}

// Create a event log message stage, including validating any supplied configuration
func newEventLogMessageStage(logger log.Logger, cfg *EventLogMessageConfig) Stage {
	return &eventLogMessageStage{
		cfg:    cfg,
		logger: log.With(logger, "component", "stage", "type", "eventlogmessage"),
	}
}

func (m *eventLogMessageStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	key := m.cfg.Source
	go func() {
		defer close(out)
		for e := range in {
			err := m.processEntry(e.Extracted, key)
			if err != nil {
				continue
			}
			out <- e
		}
	}()
	return out
}

// Process a event log message from extracted with the specified key, adding additional
// entries into the extracted map
func (m *eventLogMessageStage) processEntry(extracted map[string]any, key string) error {
	value, ok := extracted[key]
	if !ok {
		if Debug {
			level.Debug(m.logger).Log("msg", "source not in the extracted values", "source", key)
		}
		return nil
	}
	s, err := getString(value)
	if err != nil {
		level.Warn(m.logger).Log("msg", "invalid label value parsed", "value", value)
		return err
	}
	for line := range strings.SplitSeq(s, "\r\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			level.Debug(m.logger).Log("msg", "invalid line parsed from message", "line", line)
			continue
		}
		mkey := parts[0]
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(mkey).IsValidLegacy() {
			if m.cfg.DropInvalidLabels {
				if Debug {
					level.Debug(m.logger).Log("msg", "invalid label parsed from message", "key", mkey)
				}
				continue
			}
			mkey = SanitizeFullLabelName(mkey)
		}
		if _, ok := extracted[mkey]; ok && !m.cfg.OverwriteExisting {
			level.Info(m.logger).Log("msg", "extracted key that already existed, appending _extracted to key",
				"key", mkey)
			mkey += "_extracted"
		}
		mval := strings.TrimSpace(parts[1])
		if !model.LabelValue(mval).IsValid() {
			if Debug {
				level.Debug(m.logger).Log("msg", "invalid value parsed from message", "value", mval)
			}
			continue
		}
		extracted[mkey] = mval
	}
	if Debug {
		level.Debug(m.logger).Log("msg", "extracted data debug in event_log_message stage",
			"extracted_data", fmt.Sprintf("%v", extracted))
	}
	return nil
}

func (m *eventLogMessageStage) Name() string {
	return StageTypeEventLogMessage
}

// Cleanup implements Stage.
func (*eventLogMessageStage) Cleanup() {
	// no-op
}

// Sanitize a input string to convert it into a valid prometheus label
// TODO: switch to prometheus/prometheus/util/strutil/SanitizeFullLabelName
func SanitizeFullLabelName(input string) string {
	if len(input) == 0 {
		return "_"
	}
	var validSb strings.Builder
	for i, b := range input {
		if !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || (b >= '0' && b <= '9' && i > 0)) {
			validSb.WriteRune('_')
		} else {
			validSb.WriteRune(b)
		}
	}
	return validSb.String()
}
