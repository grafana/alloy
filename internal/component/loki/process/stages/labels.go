package stages

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	ErrEmptyLabelStageConfig = "label stage config cannot be empty"
	ErrInvalidLabelName      = "invalid label name: %s"
)

// LabelsConfig is a set of labels to be extracted
type LabelsConfig struct {
	Values map[string]*string `alloy:"values,attr"`
}

// validateLabelsConfig validates the Label stage configuration
func validateLabelsConfig(c map[string]*string) (map[string]string, error) {
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

// newLabelStage creates a new label stage to set labels from extracted data
func newLabelStage(logger log.Logger, configs LabelsConfig) (Stage, error) {
	labelsConfig, err := validateLabelsConfig(configs.Values)
	if err != nil {
		return nil, err
	}
	return toStage(&labelStage{
		labelsConfig: labelsConfig,
		logger:       logger,
	}), nil
}

// labelStage sets labels from extracted data
type labelStage struct {
	labelsConfig map[string]string
	logger       log.Logger
}

// Process implements Stage
func (l *labelStage) Process(labels model.LabelSet, extracted map[string]any, _ *time.Time, _ *string) {
	processLabelsConfigs(l.logger, extracted, l.labelsConfig, func(labelName model.LabelName, labelValue model.LabelValue) {
		labels[labelName] = labelValue
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

// Name implements Stage
func (l *labelStage) Name() string {
	return StageTypeLabel
}
