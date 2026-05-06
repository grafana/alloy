package stages

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/grafana/loki/v3/pkg/logql/log/pattern"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/common/model"
)

// Config Errors.
var (
	ErrPatternRequired         = errors.New("pattern is required")
	ErrEmptyPatternStageSource = errors.New("empty source")
)

// PatternConfig configures a processing stage uses logQL patterns to
// extract values from log lines into the shared values map.
// See https://grafana.com/docs/loki/latest/query/log_queries/#pattern
type PatternConfig struct {
	Pattern          string  `alloy:"pattern,attr"`
	Source           *string `alloy:"source,attr,optional"`
	LabelsFromGroups bool    `alloy:"labels_from_groups,attr,optional"`
}

// validatePatternConfig validates the config and return a regex
func validatePatternConfig(c PatternConfig) (*pattern.Matcher, error) {
	if c.Pattern == "" {
		return nil, ErrPatternRequired
	}

	if c.Source != nil && *c.Source == "" {
		return nil, ErrEmptyPatternStageSource
	}

	matcher, err := pattern.New(c.Pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pattern: %w", err)
	}

	for _, name := range matcher.Names() {
		// TODO - support UTF8 when loki does
		if !model.LegacyValidation.IsValidLabelName(name) {
			return nil, fmt.Errorf("invalid capture label name '%s'", name)
		}
	}

	return matcher, nil
}

// patternStage sets extracted data using logQL patterns
type patternStage struct {
	config  *PatternConfig
	matcher *pattern.Matcher
	logger  *slog.Logger
}

// newPatternStage creates a newPatternStage
func newPatternStage(logger *slog.Logger, config PatternConfig) (Stage, error) {
	matcher, err := validatePatternConfig(config)
	if err != nil {
		return nil, err
	}
	return toStage(&patternStage{
		config:  &config,
		matcher: matcher,
		logger:  logger.With("stage", "pattern"),
	}), nil
}

// parsePatternConfig processes an incoming configuration into a PatternConfig
func parsePatternConfig(config any) (*PatternConfig, error) {
	cfg := &PatternConfig{}
	err := mapstructure.Decode(config, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Process implements Stage
func (r *patternStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	// If a source key is provided, the pattern stage should process it
	// from the extracted map, otherwise should fall back to the entry
	input := entry

	if r.config.Source != nil {
		if _, ok := extracted[*r.config.Source]; !ok {
			if debugEnabled(r.logger) {
				r.logger.Debug("source does not exist in the set of extracted values", "source", *r.config.Source)
			}
			return
		}

		value, err := getString(extracted[*r.config.Source])
		if err != nil {
			if debugEnabled(r.logger) {
				r.logger.Debug("failed to convert source value to string", "source", *r.config.Source, "err", err, "type", reflect.TypeOf(extracted[*r.config.Source]))
			}
			return
		}

		input = &value
	}

	if input == nil {
		if debugEnabled(r.logger) {
			r.logger.Debug("cannot parse a nil entry")
		}
		return
	}

	matches := r.matcher.Matches([]byte(*input))
	if matches == nil {
		if debugEnabled(r.logger) {
			r.logger.Debug("pattern did not match", "input", *input, "pattern", r.config.Pattern)
		}
		return
	}

	names := r.matcher.Names()[:len(matches)]
	for i, m := range matches {
		name := names[i]
		extracted[name] = string(m)
		if r.config.LabelsFromGroups {
			labelName := model.LabelName(name)
			labelValue := model.LabelValue(m)

			// TODO - support UTF8 when loki does
			if !model.LegacyValidation.IsValidLabelName(name) {
				if debugEnabled(r.logger) {
					r.logger.Debug("invalid label name from pattern capture", "labelName", labelName)
				}
				continue
			}

			if !labelValue.IsValid() {
				if debugEnabled(r.logger) {
					r.logger.Debug("invalid label value from pattern capture", "labelName", labelName, "labelValue", labelValue)
				}
				continue
			}

			// Label from capture will override existing label with same name
			if debugEnabled(r.logger) {
				oldLabelValue, ok := labels[labelName]
				if ok {
					r.logger.Debug("label from pattern capture is overriding existing label", "label", labelName, "oldValue", oldLabelValue, "newValue", labelValue)
				}
			}

			labels[labelName] = labelValue
		}
	}
	if debugEnabled(r.logger) {
		r.logger.Debug("extracted data debug in pattern stage", "extracted_data", extracted)
	}
}
