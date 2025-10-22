package stages

import (
	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

func newStructuredMetadataStage(logger log.Logger, configs LabelsConfig) (Stage, error) {
	labelsConfig, err := validateLabelsConfig(configs)
	if err != nil {
		return nil, err
	}
	return &structuredMetadataStage{
		labelsConfig: labelsConfig,
		logger:       logger,
	}, nil
}

type structuredMetadataStage struct {
	labelsConfig map[string]string
	logger       log.Logger
}

func (s *structuredMetadataStage) Name() string {
	return StageTypeStructuredMetadata
}

// Cleanup implements Stage.
func (*structuredMetadataStage) Cleanup() {
	// no-op
}

func (s *structuredMetadataStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		processLabelsConfigs(s.logger, e.Extracted, s.labelsConfig, func(labelName model.LabelName, labelValue model.LabelValue) {
			e.StructuredMetadata = append(e.StructuredMetadata, push.LabelAdapter{Name: string(labelName), Value: string(labelValue)})
		})
		return s.extractFromLabels(e)
	})
}

func (s *structuredMetadataStage) extractFromLabels(e Entry) Entry {
	labels := e.Labels
	foundLabels := []model.LabelName{}

	for lName, lSrc := range s.labelsConfig {
		labelKey := model.LabelName(lSrc)
		if lValue, ok := labels[labelKey]; ok {
			e.StructuredMetadata = append(e.StructuredMetadata, push.LabelAdapter{Name: lName, Value: string(lValue)})
			foundLabels = append(foundLabels, labelKey)
		}
	}

	// Remove found labels, do this after append to structure metadata
	for _, fl := range foundLabels {
		delete(labels, fl)
	}
	e.Labels = labels
	return e
}
