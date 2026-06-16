package stages

import (
	"errors"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/syntax"
)

// ErrEmptyLabelAllowStageConfig error is returned if the config is empty.
var ErrEmptyLabelAllowStageConfig = errors.New("labelallow stage config cannot be empty")

// LabelAllowConfig contains the slice of labels to allow through.
type LabelAllowConfig struct {
	Values []string `alloy:"values,attr"`
}

var _ syntax.Validator = (*LabelAllowConfig)(nil)

// Validate implements syntax.Validator.
func (c *LabelAllowConfig) Validate() error {
	if len(c.Values) < 1 {
		return ErrEmptyLabelAllowStageConfig
	}
	return nil
}

func newLabelAllowStage(config LabelAllowConfig) (Stage, error) {
	if err := config.Validate(); err != nil {
		return nil, err
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
func (l *labelAllowStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	for label := range labels {
		if _, ok := l.labels[string(label)]; !ok {
			delete(labels, label)
		}
	}
}
