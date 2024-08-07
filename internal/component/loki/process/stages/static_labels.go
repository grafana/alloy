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

// ErrEmptyStaticLabelStageConfig error returned if the config is empty.
var ErrEmptyStaticLabelStageConfig = errors.New("static_labels stage config cannot be empty")

// StaticLabelsConfig contains a map of static labels to be set.
type StaticLabelsConfig struct {
	Values map[string]*string `alloy:"values,attr"`
}

func (c *StaticLabelsConfig) Copy() *StaticLabelsConfig {
	if c == nil {
		return nil
	}

	values := make(map[string]*string, len(c.Values))
	for k, v := range c.Values {
		values[k] = copyStrPtr(v)
	}

	return &StaticLabelsConfig{
		Values: values,
	}
}

func newStaticLabelsStage(logger log.Logger, config StaticLabelsConfig) (Stage, error) {
	err := validateLabelStaticConfig(config)
	if err != nil {
		return nil, err
	}

	return toStage(&staticLabelStage{
		Config: config,
		logger: logger,
	}), nil
}

func validateLabelStaticConfig(c StaticLabelsConfig) error {
	if c.Values == nil {
		return ErrEmptyStaticLabelStageConfig
	}
	for labelName := range c.Values {
		if !model.LabelName(labelName).IsValid() {
			return fmt.Errorf(ErrInvalidLabelName, labelName)
		}
	}
	return nil
}

// staticLabelStage implements Stage.
type staticLabelStage struct {
	Config StaticLabelsConfig
	logger log.Logger
}

// Process implements Stage.
func (l *staticLabelStage) Process(labels model.LabelSet, extracted map[string]interface{}, t *time.Time, entry *string) {
	for lName, lSrc := range l.Config.Values {
		if lSrc == nil || *lSrc == "" {
			continue
		}
		s, err := getString(*lSrc)
		if err != nil {
			level.Debug(l.logger).Log("msg", "failed to convert static label value to string", "err", err, "type", reflect.TypeOf(lSrc))
			continue
		}
		lvalue := model.LabelValue(s)
		if !lvalue.IsValid() {
			level.Debug(l.logger).Log("msg", "invalid label value parsed", "value", lvalue)
			continue
		}
		lname := model.LabelName(lName)
		labels[lname] = lvalue
	}
}

// Name implements Stage.
func (l *staticLabelStage) Name() string {
	return StageTypeStaticLabels
}
