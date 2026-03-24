package stages

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
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

func newStructuredMetadataStage(logger log.Logger, configs StructuredMetadataConfig) (Stage, error) {
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
		logger:       logger,
	}, nil
}

type structuredMetadataStage struct {
	labelsConfig map[string]string
	regex        regexp.Regexp
	logger       log.Logger
}

// Cleanup implements Stage.
func (*structuredMetadataStage) Cleanup() {
	// no-op
}

func (s *structuredMetadataStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		// Handle extracted values in values map
		processLabelsConfigs(s.logger, e.Extracted, s.labelsConfig, func(labelName model.LabelName, labelValue model.LabelValue) {
			e.StructuredMetadata = append(e.StructuredMetadata, push.LabelAdapter{Name: string(labelName), Value: string(labelValue)})
		})
		// Handle extracted values matching the regex
		if s.regex.String() != "" {
			for lName, lValue := range e.Extracted {
				if s.regex.MatchString(lName) {
					str, err := getString(lValue)
					if err != nil {
						if Debug {
							level.Debug(s.logger).Log("msg", "failed to convert extracted label value to string", "err", err, "type", reflect.TypeOf(lValue))
						}
						continue
					}
					labelValue := model.LabelValue(str)
					if !labelValue.IsValid() {
						if Debug {
							level.Debug(s.logger).Log("msg", "invalid label value parsed", "value", labelValue)
						}
						continue
					}
					e.StructuredMetadata = append(e.StructuredMetadata, push.LabelAdapter{Name: lName, Value: string(labelValue)})
				}
			}
		}

		return s.extractFromLabels(e)
	})
}

type labelsConsumer func(labelName model.LabelName, labelValue model.LabelValue)

func processLabelsConfigs(logger log.Logger, extracted map[string]any, labelsConfig map[string]string, consumer labelsConsumer) {
	for lName, lSrc := range labelsConfig {
		if lValue, ok := extracted[lSrc]; ok {
			s, err := getString(lValue)
			if err != nil {
				if Debug {
					level.Debug(logger).Log("msg", "failed to convert extracted label value to string", "err", err, "type", reflect.TypeOf(lValue))
				}
				continue
			}
			labelValue := model.LabelValue(s)
			if !labelValue.IsValid() {
				if Debug {
					level.Debug(logger).Log("msg", "invalid label value parsed", "value", labelValue)
				}
				continue
			}
			consumer(model.LabelName(lName), labelValue)
		}
	}
}

func (s *structuredMetadataStage) extractFromLabels(e Entry) Entry {
	labels := e.Labels
	foundLabels := []model.LabelName{}

	// Handle labels in values map
	for lName, lSrc := range s.labelsConfig {
		labelKey := model.LabelName(lSrc)
		if lValue, ok := labels[labelKey]; ok {
			e.StructuredMetadata = append(e.StructuredMetadata, push.LabelAdapter{Name: lName, Value: string(lValue)})
			foundLabels = append(foundLabels, labelKey)
		}
	}

	// Remove found labels, do this after append to structure metadata
	for _, fl := range foundLabels {
		delete(labels, fl)
	}

	if s.regex.String() != "" {
		// Handle remaining labels matching the regex
		foundLabels = []model.LabelName{}
		for lName, lValue := range labels {
			if s.regex.MatchString(string(lName)) {
				e.StructuredMetadata = append(e.StructuredMetadata, push.LabelAdapter{Name: string(lName), Value: string(lValue)})
				foundLabels = append(foundLabels, lName)
			}
		}
		for _, fl := range foundLabels {
			delete(labels, fl)
		}
	}

	e.Labels = labels
	return e
}
