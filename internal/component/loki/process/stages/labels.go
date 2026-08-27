package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/prometheus/common/model"

	"github.com/grafana/loki/pkg/push"
)

const (
	errEmptyLabelStageConfig = "label stage config cannot be empty"
	errInvalidLabelName      = "invalid label name: %s"
	errInvalidSourceType     = "invalid labels source_type: %s. Can only be 'extracted' or 'structured_metadata'"
)

// LabelsConfig is a set of labels to be extracted
type LabelsConfig struct {
	Values     map[string]*string `alloy:"values,attr"`
	SourceType SourceType         `alloy:"source_type,attr,optional"`
}

// validateLabelsConfig validates the Label stage configuration
func validateLabelsConfig(cfg *LabelsConfig) (map[string]string, error) {
	if cfg.Values == nil {
		return nil, errors.New(errEmptyLabelStageConfig)
	}

	if cfg.SourceType == "" {
		cfg.SourceType = SourceTypeExtractedMap
	}

	switch cfg.SourceType {
	case SourceTypeExtractedMap, SourceTypeStructuredMetadata:
	default:
		return nil, fmt.Errorf(errInvalidSourceType, cfg.SourceType)
	}

	// We must not mutate the c.Values, create a copy with changes we need.
	ret := map[string]string{}
	if cfg.Values == nil {
		return nil, errors.New(errEmptyLabelStageConfig)
	}
	for labelName, labelSrc := range cfg.Values {
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(labelName).IsValid() {
			return nil, fmt.Errorf(errInvalidLabelName, labelName)
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

var (
	_ Stage          = (*labelStage)(nil)
	_ entryProcessor = (*labelStage)(nil)
)

// newLabelStage creates a new label stage to set labels from extracted data
func newLabelStage(logger *slog.Logger, configs LabelsConfig, next NextFn) (*labelStage, error) {
	labelsConfig, err := validateLabelsConfig(&configs)
	if err != nil {
		return nil, err
	}
	return &labelStage{
		next:         next,
		cfg:          &configs,
		labelsConfig: labelsConfig,
		logger:       logger.With("stage", "labels"),
	}, nil
}

// labelStage sets labels from extracted data
type labelStage struct {
	next         NextFn
	cfg          *LabelsConfig
	labelsConfig map[string]string
	logger       *slog.Logger
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

// process implements stage.
func (l *labelStage) process(ctx context.Context, entries []Entry) error {
	for _, e := range entries {
		// FIXME(kallep): The function we call could be determined
		// when the stage is created and we can avoid this switch.
		switch l.cfg.SourceType {
		case SourceTypeExtractedMap:
			l.addLabelFromExtractedMap(e.Labels, e.Extracted)
		case SourceTypeStructuredMetadata:
			l.addLabelsFromStructuredMetadata(e.Labels, e.StructuredMetadata)
		}
	}
	return l.next(ctx, entries)
}

func (l *labelStage) addLabelFromExtractedMap(labels model.LabelSet, extracted map[string]any) {
	for lName, lSrc := range l.labelsConfig {
		if lValue, ok := extracted[lSrc]; ok {
			s, err := getString(lValue)
			if err != nil {
				if debugEnabled(l.logger) {
					l.logger.Debug("failed to convert extracted label value to string", "err", err, "type", reflect.TypeOf(lValue))
				}
				continue
			}
			labelValue := model.LabelValue(s)
			if !labelValue.IsValid() {
				if debugEnabled(l.logger) {
					l.logger.Debug("invalid label value parsed", "value", labelValue)
				}
				continue
			}

			labels[model.LabelName(lName)] = labelValue
		}
	}
}

func (l *labelStage) addLabelsFromStructuredMetadata(labels model.LabelSet, metadata push.LabelsAdapter) {
	for lName, lSrc := range l.labelsConfig {
		for _, kv := range metadata {
			if kv.Name != lSrc {
				continue
			}

			labelValue := model.LabelValue(kv.Value)
			if !labelValue.IsValid() {
				if debugEnabled(l.logger) {
					l.logger.Debug("invalid structured metadata label value", "label", lName, "value", labelValue)
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
