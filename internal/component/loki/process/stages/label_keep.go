package stages

import (
	"errors"
	"slices"
	"time"

	"github.com/prometheus/common/model"
)

// ErrEmptyLabelAllowStageConfig error is returned if the config is empty.
var ErrEmptyLabelAllowStageConfig = errors.New("labelallow stage config cannot be empty")

// LabelAllowConfig contains the slice of labels to allow through.
type LabelAllowConfig struct {
	Values []string `alloy:"values,attr"`
}

func (s *LabelAllowConfig) Copy() *LabelAllowConfig {
	if s == nil {
		return nil
	}

	return &LabelAllowConfig{
		Values: slices.Clone(s.Values),
	}
}

func newLabelAllowStage(config LabelAllowConfig) (Stage, error) {
	if len(config.Values) < 1 {
		return nil, ErrEmptyLabelAllowStageConfig
	}

	labelMap := make(map[string]struct{})
	for _, label := range config.Values {
		labelMap[label] = struct{}{}
	}

	return toStage(&labelAllowStage{
		labels: labelMap,
	}), nil
}

type labelAllowStage struct {
	labels map[string]struct{}
}

// Process implements Stage.
func (l *labelAllowStage) Process(labels model.LabelSet, extracted map[string]interface{}, t *time.Time, entry *string) {
	for label := range labels {
		if _, ok := l.labels[string(label)]; !ok {
			delete(labels, label)
		}
	}
}

// Name implements Stage.
func (l *labelAllowStage) Name() string {
	return StageTypeLabelAllow
}
