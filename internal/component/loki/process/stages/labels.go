package stages

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/common/model"
)

const (
	ErrEmptyLabelStageConfig = "label stage config cannot be empty"
	ErrInvalidLabelName      = "invalid label name: %s"
)

// LabelsConfig is a set of labels to be extracted
type LabelsConfig struct {
	Values map[string]*string `alloy:"values,attr"`
}

func (s *LabelsConfig) Copy() *LabelsConfig {
	if s == nil {
		return nil
	}

	values := make(map[string]*string, len(s.Values))
	for k, v := range s.Values {
		values[k] = copyStrPtr(v)
	}

	return &LabelsConfig{
		Values: values,
	}
}

// validateLabelsConfig validates the Label stage configuration
func validateLabelsConfig(c LabelsConfig) error {
	if c.Values == nil {
		return errors.New(ErrEmptyLabelStageConfig)
	}
	for labelName, labelSrc := range c.Values {
		if !model.LabelName(labelName).IsValid() {
			return fmt.Errorf(ErrInvalidLabelName, labelName)
		}
		// If no label source was specified, use the key name
		if labelSrc == nil || *labelSrc == "" {
			lName := labelName
			c.Values[labelName] = &lName
		}
	}
	return nil
}

// newLabelStage creates a new label stage to set labels from extracted data
func newLabelStage(logger log.Logger, configs LabelsConfig) (Stage, error) {
	err := validateLabelsConfig(configs)
	if err != nil {
		return nil, err
	}
	return toStage(&labelStage{
		cfgs:   configs,
		logger: logger,
	}), nil
}

// labelStage sets labels from extracted data
type labelStage struct {
	cfgs   LabelsConfig
	logger log.Logger
}

// Process implements Stage
func (l *labelStage) Process(labels model.LabelSet, extracted map[string]interface{}, _ *time.Time, _ *string) {
	processLabelsConfigs(l.logger, extracted, l.cfgs, func(labelName model.LabelName, labelValue model.LabelValue) {
		labels[labelName] = labelValue
	})
}

type labelsConsumer func(labelName model.LabelName, labelValue model.LabelValue)

func processLabelsConfigs(logger log.Logger, extracted map[string]interface{}, configs LabelsConfig, consumer labelsConsumer) {
	for lName, lSrc := range configs.Values {
		if lValue, ok := extracted[*lSrc]; ok {
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
