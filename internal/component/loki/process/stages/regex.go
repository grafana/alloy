package stages

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/mitchellh/mapstructure"
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
	Expression       string  `alloy:"expression,attr"`
	Source           *string `alloy:"source,attr,optional"`
	LabelsFromGroups bool    `alloy:"labels_from_groups,attr,optional"`
}

// validateRegexConfig validates the config and return a regex
func validateRegexConfig(c RegexConfig) (*regexp.Regexp, error) {
	if c.Expression == "" {
		return nil, ErrExpressionRequired
	}

	if c.Source != nil && *c.Source == "" {
		return nil, ErrEmptyRegexStageSource
	}

	expr, err := regexp.Compile(c.Expression)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", ErrCouldNotCompileRegex, err)
	}

	return expr, nil
}

// regexStage sets extracted data using regular expressions
type regexStage struct {
	config     *RegexConfig
	expression *regexp.Regexp
	logger     log.Logger
}

// newRegexStage creates a newRegexStage
func newRegexStage(logger log.Logger, config RegexConfig) (Stage, error) {
	expression, err := validateRegexConfig(config)
	if err != nil {
		return nil, err
	}
	return toStage(&regexStage{
		config:     &config,
		expression: expression,
		logger:     log.With(logger, "component", "stage", "type", "regex"),
	}), nil
}

// parseRegexConfig processes an incoming configuration into a RegexConfig
func parseRegexConfig(config any) (*RegexConfig, error) {
	cfg := &RegexConfig{}
	err := mapstructure.Decode(config, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
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

	match := r.expression.FindStringSubmatch(*input)
	if match == nil {
		if Debug {
			level.Debug(r.logger).Log("msg", "regex did not match", "input", *input, "regex", r.expression)
		}
		return
	}

	for i, name := range r.expression.SubexpNames() {
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

// Name implements Stage
func (r *regexStage) Name() string {
	return StageTypeRegex
}
