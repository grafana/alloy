package stages

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/regexp"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/common/model"
)

// Config Errors.
var (
	ErrExpressionRequired    = errors.New("expression is required")
	ErrCouldNotCompileRegex  = errors.New("could not compile regular expression")
	ErrEmptyRegexStageSource = errors.New("empty source")
)

// RegexConfig configures a processing stage uses regular expressions to
// extract values from log lines into the shared values map.
type RegexConfig struct {
	Expression       *regexp.NonEmptyRegexp `alloy:"expression,attr"`
	Source           *string                `alloy:"source,attr,optional"`
	LabelsFromGroups bool                   `alloy:"labels_from_groups,attr,optional"`
}

// validateRegexConfig validates the config
func validateRegexConfig(c RegexConfig) error {
	if c.Source != nil && *c.Source == "" {
		return ErrEmptyRegexStageSource
	}

	return nil
}

// regexStage sets extracted data using regular expressions
type regexStage struct {
	config *RegexConfig
	logger log.Logger
}

// newRegexStage creates a newRegexStage
func newRegexStage(logger log.Logger, config RegexConfig) (Stage, error) {
	if err := validateRegexConfig(config); err != nil {
		return nil, err
	}

	return toStage(&regexStage{
		config: &config,
		logger: log.With(logger, "component", "stage", "type", "regex"),
	}), nil
}

// Process implements Stage
func (r *regexStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	// If a source key is provided, the regex stage should process it
	// from the extracted map, otherwise should fall back to the entry
	input := entry

	if r.config.Source != nil {
		if _, ok := extracted[*r.config.Source]; !ok {
			if Debug {
				level.Debug(r.logger).Log("msg", "source does not exist in the set of extracted values", "source", *r.config.Source)
			}
			return
		}

		value, err := getString(extracted[*r.config.Source])
		if err != nil {
			if Debug {
				level.Debug(r.logger).Log("msg", "failed to convert source value to string", "source", *r.config.Source, "err", err, "type", reflect.TypeOf(extracted[*r.config.Source]))
			}
			return
		}

		input = &value
	}

	if input == nil {
		if Debug {
			level.Debug(r.logger).Log("msg", "cannot parse a nil entry")
		}
		return
	}

	match := r.config.Expression.FindStringSubmatch(*input)
	if match == nil {
		if Debug {
			level.Debug(r.logger).Log("msg", "regex did not match", "input", *input, "regex", r.config.Expression)
		}
		return
	}

	for i, name := range r.config.Expression.SubexpNames() {
		if i != 0 && name != "" {
			extracted[name] = match[i]
			if r.config.LabelsFromGroups {
				labelName := model.LabelName(name)
				labelValue := model.LabelValue(match[i])

				// TODO: add support for different validation schemes.
				//nolint:staticcheck
				if !labelName.IsValid() {
					if Debug {
						level.Debug(r.logger).Log("msg", "invalid label name from regex capture group", "labelName", labelName)
					}
					continue
				}

				if !labelValue.IsValid() {
					if Debug {
						level.Debug(r.logger).Log("msg", "invalid label value from regex capture group", "labelName", labelName, "labelValue", labelValue)
					}
					continue
				}

				oldLabelValue, ok := labels[labelName]

				// Label from capture group will override existing label with same name
				if Debug && ok {
					level.Debug(r.logger).Log("msg", "label from regex capture group is overriding existing label", "label", labelName, "oldValue", oldLabelValue, "newValue", labelValue)
				}

				labels[labelName] = labelValue
			}
		}
	}
	if Debug {
		level.Debug(r.logger).Log("msg", "extracted data debug in regex stage", "extracted data", fmt.Sprintf("%v", extracted))
	}
}
