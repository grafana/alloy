package stages

import (
	"errors"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/syntax"
)

// ErrEmptyLabelDropStageConfig error returned if the config is empty.
var ErrEmptyLabelDropStageConfig = errors.New("labeldrop stage config cannot be empty")

// LabelDropConfig contains the slice of labels to be dropped.
type LabelDropConfig struct {
	Values []string `alloy:"values,attr"`
}

var _ syntax.Validator = (*LabelDropConfig)(nil)

// Validate implements syntax.Validator.
func (c *LabelDropConfig) Validate() error {
	if len(c.Values) < 1 {
		return ErrEmptyLabelDropStageConfig
	}
	return nil
}

func newLabelDropStage(config LabelDropConfig) (Stage, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return toStage(&labelDropStage{
		config: config,
	}), nil
}

type labelDropStage struct {
	config LabelDropConfig
}

// Process implements Stage.
func (l *labelDropStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	for _, label := range l.config.Values {
		delete(labels, model.LabelName(label))
	}
}
