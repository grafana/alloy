package stages

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	ErrEmptyLabelStageConfig = "label stage config cannot be empty"
	ErrInvalidLabelName      = "invalid label name: %s"
	ErrInvalidLabelRegexp    = "invalid label regexp: %s"
)

// LabelsConfig is a set of labels to be extracted
type LabelsConfig struct {
	Values map[string]*string `alloy:"values,attr"`
	Map    string             `alloy:"map,attr,optional"`
}

// validateLabelsConfig validates the Label stage configuration
func validateLabelsConfig(c LabelsConfig) (map[string]string, *regexp.Regexp, error) {
	// We must not mutate the c.Values, create a copy with changes we need.
	labels := map[string]string{}
	var mapRegexp *regexp.Regexp

	if c.Values == nil {
		return nil, nil, errors.New(ErrEmptyLabelStageConfig)
	}
	for labelName, labelSrc := range c.Values {
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(labelName).IsValid() {
			return nil, nil, fmt.Errorf(ErrInvalidLabelName, labelName)
		}
		// If no label source was specified, use the key name
		if labelSrc == nil || *labelSrc == "" {
			labels[labelName] = labelName
		} else {
			labels[labelName] = *labelSrc
		}
	}

	if c.Map != "" {
		var err error
		mapRegexp, err = regexp.Compile(c.Map)
		if err != nil {
			return nil, nil, fmt.Errorf(ErrInvalidLabelRegexp, c.Map)
		}
	}
	return labels, mapRegexp, nil
}

// newLabelStage creates a new label stage to set labels from extracted data
func newLabelStage(logger log.Logger, configs LabelsConfig) (Stage, error) {
	labelsConfig, mapRegexp, err := validateLabelsConfig(configs)
	if err != nil {
		return nil, err
	}
	return toStage(&labelStage{
		labelsConfig: labelsConfig,
		mapRegexp:    mapRegexp,
		logger:       logger,
	}), nil
}

// labelStage sets labels from extracted data
type labelStage struct {
	labelsConfig map[string]string
	mapRegexp    *regexp.Regexp
	logger       log.Logger
}

// Process implements Stage
func (l *labelStage) Process(labels model.LabelSet, extracted map[string]interface{}, _ *time.Time, _ *string) {
	processLabelsConfigs(l.logger, extracted, l.labelsConfig, l.mapRegexp, func(labelName model.LabelName, labelValue model.LabelValue) {
		labels[labelName] = labelValue
	})
}

type labelsConsumer func(labelName model.LabelName, labelValue model.LabelValue)

func labelValue(logger log.Logger, v interface{}) (model.LabelValue, error) {
	s, err := getString(v)
	if err != nil {
		if Debug {
			level.Debug(logger).Log("msg", "failed to convert extracted label value to string", "err", err, "type", reflect.TypeOf(v))
		}
		return "", err
	}
	lval := model.LabelValue(s)
	if !lval.IsValid() {
		if Debug {
			level.Debug(logger).Log("msg", "invalid label value parsed", "value", lval)
		}
		return "", err
	}

	return lval, nil
}

func processLabelsConfigs(logger log.Logger, extracted map[string]interface{}, labelsConfig map[string]string, mapRegexp *regexp.Regexp, consumer labelsConsumer) {
	if mapRegexp != nil {
		for l, v := range extracted {
			if sub := mapRegexp.FindSubmatch([]byte(l)); sub != nil {
				labelValue, err := labelValue(logger, v)
				if err != nil {
					continue
				}
				consumer(model.LabelName(sub[1]), labelValue)
			}
		}
	}

	for lName, lSrc := range labelsConfig {
		if lValue, ok := extracted[lSrc]; ok {
			labelValue, err := labelValue(logger, lValue)
			if err != nil {
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
