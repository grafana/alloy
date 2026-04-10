package stages

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"slices"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

type StructuredMetadataConfig struct {
	Values map[string]*string `alloy:"values,attr,optional"`
	Regex  string             `alloy:"regex,attr,optional"`
}

// validateStructuredMetadataConfig validates the structured metadata stage config.
func validateStructuredMetadataConfig(c map[string]*string) (map[string]string, error) {
	// We must not mutate the c.Values, create a copy with changes we need.
	ret := map[string]string{}
	if c == nil {
		return nil, errors.New(ErrEmptyLabelStageConfig)
	}
	for labelName, labelSrc := range c {
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(labelName).IsValid() {
			return nil, fmt.Errorf(ErrInvalidLabelName, labelName)
		}
		// If no label source was specified, use the key name
		if labelSrc == nil || *labelSrc == "" {
			ret[labelName] = labelName
		} else {
			ret[labelName] = *labelSrc
		}
	}
	return ret, nil
}

func newStructuredMetadataStage(logger *slog.Logger, configs StructuredMetadataConfig) (Stage, error) {
	var validatedLabelsConfig map[string]string
	var err error

	if len(configs.Values) > 0 {
		validatedLabelsConfig, err = validateStructuredMetadataConfig(configs.Values)
		if err != nil {
			return nil, err
		}
	}

	re, err := regexp.Compile(configs.Regex)
	if err != nil {
		return nil, err
	}
	return &structuredMetadataStage{
		labelsConfig: validatedLabelsConfig,
		regex:        *re,
		logger:       logger.With("stage", "structured_metadata"),
	}, nil
}

type structuredMetadataStage struct {
	labelsConfig map[string]string
	regex        regexp.Regexp
	logger       *slog.Logger
}

// Cleanup implements Stage.
func (*structuredMetadataStage) Cleanup() {
	// no-op
}

func (s *structuredMetadataStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		appendStructureMetadata := func(labelName model.LabelName, labelValue model.LabelValue) {
			metadata := push.LabelAdapter{Name: string(labelName), Value: string(labelValue)}

			i := slices.IndexFunc(e.StructuredMetadata, func(label push.LabelAdapter) bool {
				return label.Name == metadata.Name
			})
			if i != -1 {
				e.StructuredMetadata[i] = metadata
				return
			}

			e.StructuredMetadata = append(e.StructuredMetadata, metadata)
		}

		// Try to add structured metadata from extracted map using labelsConfig.
		processExtractedLabelsByConfig(s.logger, e.Extracted, s.labelsConfig, appendStructureMetadata)

		// Try to add structured metadata from extracted map using regex.
		processExtractedLabelsByRegex(s.logger, e.Extracted, s.regex, appendStructureMetadata)

		// Try to add structured metadata from labels using labelsConfig.
		processEntryLabelsByConfig(e.Labels, s.labelsConfig, appendStructureMetadata)

		// Try to add structured metadata from labels using regex.
		processEntryLabelsByRegex(e.Labels, s.regex, appendStructureMetadata)

		return e
	})
}

type labelsConsumer func(labelName model.LabelName, labelValue model.LabelValue)

// processExtractedLabelsByConfig adds structured metadata from extracted values selected by labelsConfig.
func processExtractedLabelsByConfig(logger *slog.Logger, extracted map[string]any, labelsConfig map[string]string, consumer labelsConsumer) {
	for lName, lSrc := range labelsConfig {
		if lValue, ok := extracted[lSrc]; ok {
			s, err := getString(lValue)
			if err != nil {
				if Debug {
					logger.Debug("failed to convert extracted label value to string", "err", err, "type", reflect.TypeOf(lValue))
				}
				continue
			}
			labelValue := model.LabelValue(s)
			if !labelValue.IsValid() {
				if Debug {
					logger.Debug("invalid label value parsed", "value", labelValue)
				}
				continue
			}
			consumer(model.LabelName(lName), labelValue)
		}
	}
}

// processExtractedLabelsByRegex adds structured metadata from extracted values whose keys match the configured regex.
func processExtractedLabelsByRegex(logger *slog.Logger, extracted map[string]any, regex regexp.Regexp, consumer labelsConsumer) {
	if regex.String() == "" {
		return
	}

	for lName, lValue := range extracted {
		if !regex.MatchString(lName) {
			continue
		}

		str, err := getString(lValue)
		if err != nil {
			if Debug {
				logger.Debug("failed to convert extracted label value to string", "err", err, "type", reflect.TypeOf(lValue))
			}
			continue
		}

		labelValue := model.LabelValue(str)
		if !labelValue.IsValid() {
			if Debug {
				logger.Debug("invalid label value parsed", "value", labelValue)
			}
			continue
		}

		consumer(model.LabelName(lName), labelValue)
	}
}

// processEntryLabelsByConfig adds structured metadata from entry labels selected by explicit config mappings and removes those labels.
func processEntryLabelsByConfig(labels model.LabelSet, labelsConfig map[string]string, consumer labelsConsumer) {
	foundLabels := make([]model.LabelName, 0, len(labelsConfig))

	for lName, lSrc := range labelsConfig {
		labelKey := model.LabelName(lSrc)
		if lValue, ok := labels[labelKey]; ok {
			consumer(model.LabelName(lName), lValue)
			foundLabels = append(foundLabels, labelKey)
		}
	}

	for _, fl := range foundLabels {
		delete(labels, fl)
	}
}

// processEntryLabelsByRegex adds structured metadata from entry labels whose keys match the configured regex and removes those labels.
func processEntryLabelsByRegex(labels model.LabelSet, regex regexp.Regexp, consumer labelsConsumer) {
	if regex.String() == "" {
		return
	}

	foundLabels := make([]model.LabelName, 0)
	for lName, lValue := range labels {
		if regex.MatchString(string(lName)) {
			consumer(lName, lValue)
			foundLabels = append(foundLabels, lName)
		}
	}

	for _, fl := range foundLabels {
		delete(labels, fl)
	}
}
