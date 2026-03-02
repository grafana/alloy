package stages

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/loki/pkg/push"
)

const (
	ErrEmptyLabelStageConfig = "label stage config cannot be empty"
	ErrInvalidLabelName      = "invalid label name: %s"
	ErrInvalidSourceType     = "invalid labels source_type: %s. Can only be 'extracted' or 'structured_metadata'"

	LabelsSourceStructuredMetadata string = "structured_metadata"
	LabelsSourceExtractedMap       string = "extracted"
)

// LabelsConfig is a set of labels to be extracted
type LabelsConfig struct {
	Values     map[string]*string `alloy:"values,attr"`
	SourceType SourceType         `alloy:"source_type,attr,optional"`
}

// validateLabelsConfig validates the Label stage configuration
func validateLabelsConfig(cfg *LabelsConfig) error {
	if cfg.Values == nil {
		return errors.New(ErrEmptyLabelStageConfig)
	}

	if cfg.SourceType == "" {
		cfg.SourceType = SourceTypeExtractedMap
	}

	switch cfg.SourceType {
	case SourceTypeExtractedMap, SourceTypeStructuredMetadata:
	default:
		return fmt.Errorf(ErrInvalidSourceType, cfg.SourceType)
	}

	// We must not mutate the c.Values, create a copy with changes we need.
	returnValues := map[string]*string{}
	for labelName, labelSrc := range cfg.Values {
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(labelName).IsValid() {
			return fmt.Errorf(ErrInvalidLabelName, labelName)
		}
		// If no label source was specified, use the key name
		if labelSrc == nil || *labelSrc == "" {
			returnValues[labelName] = &labelName
		} else {
			returnValues[labelName] = labelSrc
		}
	}
	cfg.Values = returnValues
	return nil
}

// newLabelStage creates a new label stage to set labels from extracted data
func newLabelStage(logger log.Logger, configs LabelsConfig) (Stage, error) {
	err := validateLabelsConfig(&configs)
	if err != nil {
		return nil, err
	}
	return &labelStage{
		cfg:    &configs,
		logger: logger,
	}, nil
}

// labelStage sets labels from extracted data
type labelStage struct {
	cfg    *LabelsConfig
	logger log.Logger
}

// Run implements Stage
func (l *labelStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			switch l.cfg.SourceType {
			case SourceTypeExtractedMap:
				l.addLabelFromExtractedMap(e.Labels, e.Extracted)
			case SourceTypeStructuredMetadata:
				l.addLabelsFromStructuredMetadata(e.Labels, e.StructuredMetadata)
			}
			out <- e
		}
	}()
	return out
}

func (l *labelStage) addLabelFromExtractedMap(labels model.LabelSet, extracted map[string]any) {
	for lName, lSrc := range l.cfg.Values {
		if lValue, ok := extracted[*lSrc]; ok {
			s, err := getString(lValue)
			if err != nil {
				if Debug {
					level.Debug(l.logger).Log("msg", "failed to convert extracted label value to string", "err", err, "type", reflect.TypeOf(lValue))
				}
				continue
			}
			labelValue := model.LabelValue(s)
			if !labelValue.IsValid() {
				if Debug {
					level.Debug(l.logger).Log("msg", "invalid label value parsed", "value", labelValue)
				}
				continue
			}

			labels[model.LabelName(lName)] = labelValue
		}
	}
}

func (l *labelStage) addLabelsFromStructuredMetadata(labels model.LabelSet, metadata push.LabelsAdapter) {
	for lName, lSrc := range l.cfg.Values {
		for _, kv := range metadata {
			if kv.Name != *lSrc {
				continue
			}

			labelValue := model.LabelValue(kv.Value)
			if !labelValue.IsValid() {
				if Debug {
					level.Debug(l.logger).Log("msg", "invalid structured metadata label value", "label", lName, "value", labelValue)
				}
				break
			}

			labels[model.LabelName(lName)] = labelValue
			break
		}
	}
}

// Cleanup implements Stage.
func (*labelStage) Cleanup() {
	// no-op
}
