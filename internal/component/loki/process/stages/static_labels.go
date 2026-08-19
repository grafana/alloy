package stages

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/common/model"
)

// errEmptyStaticLabelStageConfig error returned if the config is empty.
var errEmptyStaticLabelStageConfig = errors.New("static_labels stage config cannot be empty")

// StaticLabelsConfig contains a map of static labels to be set.
type StaticLabelsConfig struct {
	Values map[string]*string `alloy:"values,attr"`
}

var (
	_ Stage          = (*staticLabelStage)(nil)
	_ entryProcessor = (*staticLabelStage)(nil)
)

func newStaticLabelsStage(config StaticLabelsConfig, next NextFn) (Stage, error) {
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

	return &staticLabelStage{next, values}, nil
}

func validateLabelStaticConfig(c StaticLabelsConfig) error {
	if c.Values == nil {
		return errEmptyStaticLabelStageConfig
	}
	for labelName := range c.Values {
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(labelName).IsValid() {
			return fmt.Errorf(errInvalidLabelName, labelName)
		}
	}
	return nil
}

// staticLabelStage implements Stage.
type staticLabelStage struct {
	next NextFn
	// values packs both label names and label values and need to be divisible by 2.
	values []string
}

// Run implements Stage.
func (l *staticLabelStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		for i := 0; i < len(l.values); i += 2 {
			e.Labels[model.LabelName(l.values[i])] = model.LabelValue(l.values[i+1])
		}
		return e
	})
}

// process implements stage.
func (l *staticLabelStage) process(ctx context.Context, entries []Entry) error {
	for _, e := range entries {
		for i := 0; i < len(l.values); i += 2 {
			e.Labels[model.LabelName(l.values[i])] = model.LabelValue(l.values[i+1])
		}
	}
	return l.next(ctx, entries)
}

// Cleanup implements Stage.
func (l *staticLabelStage) Cleanup() {}
