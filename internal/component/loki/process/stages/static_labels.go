package stages

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/service/logging/level"
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

	return toStage(&staticLabelStage{
		config: config,
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
	config StaticLabelsConfig
	logger log.Logger
}

// Process implements Stage.
func (l *staticLabelStage) Process(labels model.LabelSet, extracted map[string]interface{}, t *time.Time, entry *string) {
	for lName, lSrc := range l.config.Values {
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
