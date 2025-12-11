package stages

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
)

// ErrEmptyStaticLabelStageConfig error returned if the config is empty.
var ErrEmptyStaticLabelStageConfig = errors.New("static_labels stage config cannot be empty")

// StaticLabelsConfig contains a map of static labels to be set.
type StaticLabelsConfig struct {
	Values map[string]*string `alloy:"values,attr"`
}

func newStaticLabelsStage(_ log.Logger, config StaticLabelsConfig) (Stage, error) {
	err := validateLabelStaticConfig(config)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(config.Values)*2)
	for n, v := range config.Values {
		if v == nil || *v == "" {
			continue
		}

		value := *v
		if !model.LabelValue(value).IsValid() {
			return nil, fmt.Errorf("invalid label value: %s", value)
		}

		values = append(values, n, value)
	}

	return toStage(&staticLabelStage{values}), nil
}

func validateLabelStaticConfig(c StaticLabelsConfig) error {
	if c.Values == nil {
		return ErrEmptyStaticLabelStageConfig
	}
	for labelName := range c.Values {
		//nolint:staticcheck
		if !model.LabelName(labelName).IsValid() {
			return fmt.Errorf(ErrInvalidLabelName, labelName)
		}
	}
	return nil
}

// staticLabelStage implements Stage.
type staticLabelStage struct {
	// values packs both label names and label values and need to be divisible by 2.
	values []string
}

// Process implements Stage.
func (l *staticLabelStage) Process(labels model.LabelSet, _ map[string]any, _ *time.Time, _ *string) {
	for i := 0; i < len(l.values); i += 2 {
		labels[model.LabelName(l.values[i])] = model.LabelValue(l.values[i+1])
	}
}

// Name implements Stage.
func (l *staticLabelStage) Name() string {
	return StageTypeStaticLabels
}
