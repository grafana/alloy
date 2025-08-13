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

func newStaticLabelsStage(logger log.Logger, config StaticLabelsConfig) (Stage, error) {
	err := validateLabelStaticConfig(config)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(config.Values)*2)
	for n, v := range config.Values {
		if v == nil || *v == "" {
			continue
		}

		s, err := getString(*v)
		if err != nil {
			return nil, fmt.Errorf("faild to convert static label value: %w", err)
		}

		if !model.LabelValue(s).IsValid() {
			return nil, fmt.Errorf("invalid label value: %s", s)
		}

		values = append(values, n, s)
	}

	return toStage(&staticLabelStage{
		logger: logger,
		values: values,
	}), nil
}

func validateLabelStaticConfig(c StaticLabelsConfig) error {
	if c.Values == nil {
		return ErrEmptyStaticLabelStageConfig
	}
	for labelName := range c.Values {
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(labelName).IsValid() {
			return fmt.Errorf(ErrInvalidLabelName, labelName)
		}
	}
	return nil
}

// staticLabelStage implements Stage.
type staticLabelStage struct {
	logger log.Logger
	// values packs both label names and label values and need to be devisable by 2.
	values []string
}

// Process implements Stage.
func (l *staticLabelStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	for i := 0; i < len(l.values); i += 2 {
		labels[model.LabelName(l.values[i])] = model.LabelValue(l.values[i+1])
	}
}

// Name implements Stage.
func (l *staticLabelStage) Name() string {
	return StageTypeStaticLabels
}
