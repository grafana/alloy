package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"

	"github.com/prometheus/common/model"
)

var (
	errExpressionRequired    = errors.New("expression is required")
	errCouldNotCompileRegex  = errors.New("could not compile regular expression")
	errEmptyRegexStageSource = errors.New("empty source")
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
		return nil, errExpressionRequired
	}

	if c.Source != nil && *c.Source == "" {
		return nil, errEmptyRegexStageSource
	}

	expr, err := regexp.Compile(c.Expression)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", errCouldNotCompileRegex, err)
	}

	return expr, nil
}

var (
	_ Stage          = (*regexStage)(nil)
	_ entryProcessor = (*regexStage)(nil)
)

// newRegexStage creates a regexStage
func newRegexStage(logger *slog.Logger, config RegexConfig, next NextFn) (*regexStage, error) {
	expression, err := validateRegexConfig(config)
	if err != nil {
		return nil, err
	}
	return &regexStage{
		next:       next,
		config:     &config,
		expression: expression,
		logger:     logger.With("stage", "regex"),
	}, nil
}

// regexStage sets extracted data using regular expressions
type regexStage struct {
	next       NextFn
	config     *RegexConfig
	expression *regexp.Regexp
	logger     *slog.Logger
}

// Run implements Stage.
func (r *regexStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return r.processEntry(e)
	})
}

// process implements stage.
func (r *regexStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = r.processEntry(entries[i])
	}
	return r.next(ctx, entries)
}

// Cleanup implements Stage.
func (r *regexStage) Cleanup() {}

func (r *regexStage) processEntry(e Entry) Entry {
	// If a source key is provided, the regex stage should process it
	// from the extracted map, otherwise should fall back to the line.
	input := e.Line

	if r.config.Source != nil {
		source := *r.config.Source
		if _, ok := e.Extracted[source]; !ok {
			if debugEnabled(r.logger) {
				r.logger.Debug("source does not exist in the set of extracted values", "source", source)
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

	match := r.expression.FindStringSubmatch(input)
	if match == nil {
		if debugEnabled(r.logger) {
			r.logger.Debug("regex did not match", "input", input, "regex", r.expression)
		}
		return e
	}

	for i, name := range r.expression.SubexpNames() {
		if i != 0 && name != "" {
			e.Extracted[name] = match[i]
			if r.config.LabelsFromGroups {
				labelName := model.LabelName(name)
				labelValue := model.LabelValue(match[i])

				// TODO: add support for different validation schemes.
				//nolint:staticcheck
				if !labelName.IsValid() {
					if debugEnabled(r.logger) {
						r.logger.Debug("invalid label name from regex capture group", "labelName", labelName)
					}
					continue
				}

				if !labelValue.IsValid() {
					if debugEnabled(r.logger) {
						r.logger.Debug("invalid label value from regex capture group", "labelName", labelName, "labelValue", labelValue)
					}
					continue
				}

				oldLabelValue, ok := e.Labels[labelName]

				// Label from capture group will override existing label with same name
				if debugEnabled(r.logger) && ok {
					r.logger.Debug("label from regex capture group is overriding existing label", "label", labelName, "oldValue", oldLabelValue, "newValue", labelValue)
				}

				e.Labels[labelName] = labelValue
			}
		}
	}

	if debugEnabled(r.logger) {
		r.logger.Debug("extracted data debug in regex stage", "extracted_data", e.Extracted)
	}

	return e
}
