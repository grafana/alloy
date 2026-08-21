package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/grafana/loki/v3/pkg/logql/log/pattern"
	"github.com/prometheus/common/model"
)

var (
	errPatternRequired         = errors.New("pattern is required")
	errEmptyPatternStageSource = errors.New("empty source")
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
		return nil, errPatternRequired
	}

	if c.Source != nil && *c.Source == "" {
		return nil, errEmptyPatternStageSource
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

var (
	_ Stage = (*patternStage)(nil)
	_ stage = (*patternStage)(nil)
)

// newPatternStage creates a newPatternStage
func newPatternStage(logger *slog.Logger, config PatternConfig, next NextFn) (*patternStage, error) {
	matcher, err := validatePatternConfig(config)
	if err != nil {
		return nil, err
	}
	return &patternStage{
		next:    next,
		config:  &config,
		matcher: matcher,
		logger:  logger.With("stage", "pattern"),
	}, nil
}

// patternStage sets extracted data using logQL patterns
type patternStage struct {
	next    NextFn
	config  *PatternConfig
	matcher *pattern.Matcher
	logger  *slog.Logger
}

// Run implements Stage.
func (r *patternStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return r.processEntry(e)
	})
}

// process implements stage.
func (r *patternStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = r.processEntry(entries[i])
	}
	return r.next(ctx, entries)
}

// Cleanup implements Stage.
func (r *patternStage) Cleanup() {}

func (r *patternStage) processEntry(e Entry) Entry {
	// If a source key is provided, the pattern stage should process it
	// from the extracted map, otherwise should fall back to the line.
	input := e.Line

	if r.config.Source != nil {
		source := *r.config.Source
		if _, ok := e.Extracted[source]; !ok {
			if debugEnabled(r.logger) {
				r.logger.Debug("source does not exist in the set of extracted values", "source", *r.config.Source)
			}
			return e
		}

		value, err := getString(e.Extracted[source])
		if err != nil {
			if debugEnabled(r.logger) {
				r.logger.Debug("failed to convert source value to string", "source", *r.config.Source, "err", err, "type", reflect.TypeOf(e.Extracted[source]))
			}
			return e
		}

		input = value
	}

	matches := r.matcher.Matches([]byte(input))
	if matches == nil {
		if debugEnabled(r.logger) {
			r.logger.Debug("pattern did not match", "input", input, "pattern", r.config.Pattern)
		}
		return e
	}

	names := r.matcher.Names()[:len(matches)]
	for i, m := range matches {
		name := names[i]
		e.Extracted[name] = string(m)
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
				oldLabelValue, ok := e.Labels[labelName]
				if ok {
					r.logger.Debug("label from pattern capture is overriding existing label", "label", labelName, "oldValue", oldLabelValue, "newValue", labelValue)
				}
			}

			e.Labels[labelName] = labelValue
		}
	}

	if debugEnabled(r.logger) {
		r.logger.Debug("extracted data debug in pattern stage", "extracted_data", e.Extracted)
	}

	return e
}
