package stages

import (
	"context"
	"errors"
	"fmt"

	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/common/model"
)

// errEmptyStaticLabelStageConfig error returned if the config is empty.
var errEmptyStaticLabelStageConfig = errors.New("static_labels stage config cannot be empty")

// StaticLabelsConfig contains a map of static labels to be set.
type StaticLabelsConfig struct {
	Values map[string]*string `alloy:"values,attr"`
}

var (
	_ Stage            = (*staticLabelStage)(nil)
	_ entryProcessor   = (*staticLabelStage)(nil)
	_ syntax.Validator = (*StaticLabelsConfig)(nil)
)

func newStaticLabelsStage(config StaticLabelsConfig, opts stageOpts) *staticLabelStage {
	values := make([]string, 0, len(config.Values)*2)
	for n, v := range config.Values {
		if v == nil || *v == "" {
			continue
		}
		values = append(values, n, *v)
	}

	return &staticLabelStage{opts.next, values}
}

func (c *StaticLabelsConfig) Validate() error {
	if c.Values == nil {
		return errEmptyStaticLabelStageConfig
	}
	for labelName, v := range c.Values {
		// TODO: add support for different validation schemes.
		if !model.UTF8Validation.IsValidLabelName(labelName) {
			return fmt.Errorf(errInvalidLabelName, labelName)
		}
		if v == nil || *v == "" {
			continue
		}
		if !model.LabelValue(*v).IsValid() {
			return fmt.Errorf("invalid label value: %s", *v)
		}
	}
	return nil
}

// staticLabelStage implements Stage.
type staticLabelStage struct {
	next nextFn
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
