package stages

import (
	"errors"
	"time"

	"github.com/prometheus/common/model"
)

// ErrEmptyLabelDropStageConfig error returned if the config is empty.
var ErrEmptyLabelDropStageConfig = errors.New("labeldrop stage config cannot be empty")

// LabelDropConfig contains the slice of labels to be dropped.
type LabelDropConfig struct {
	Values []string `river:"values,attr"`
}

func newLabelDropStage(config LabelDropConfig) (Stage, error) {
	if len(config.Values) < 1 {
		return nil, ErrEmptyLabelDropStageConfig
	}

	return toStage(&labelDropStage{
		config: config,
	}), nil
}

type labelDropStage struct {
	config LabelDropConfig
}

// Process implements Stage.
func (l *labelDropStage) Process(labels model.LabelSet, extracted map[string]interface{}, t *time.Time, entry *string) {
	for _, label := range l.config.Values {
		delete(labels, model.LabelName(label))
	}
}

// Name implements Stage.
func (l *labelDropStage) Name() string {
	return StageTypeLabelDrop
}
