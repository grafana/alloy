package stages

import (
	"context"
	"errors"

	"github.com/prometheus/common/model"
)

// errEmptyLabelDropStageConfig error returned if the config is empty.
var errEmptyLabelDropStageConfig = errors.New("labeldrop stage config cannot be empty")

// LabelDropConfig contains the slice of labels to be dropped.
type LabelDropConfig struct {
	Values []string `alloy:"values,attr"`
}

var (
	_ Stage = (*labelDropStage)(nil)
	_ stage = (*labelDropStage)(nil)
)

func newLabelDropStage(config LabelDropConfig, next NextFn) (*labelDropStage, error) {
	if len(config.Values) < 1 {
		return nil, errEmptyLabelDropStageConfig
	}

	return &labelDropStage{
		next:   next,
		config: config,
	}, nil
}

type labelDropStage struct {
	next   NextFn
	config LabelDropConfig
}

// Run implements Stage.
func (l *labelDropStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		for _, label := range l.config.Values {
			delete(e.Labels, model.LabelName(label))
		}
		return e
	})
}

// process implements stage.
func (l *labelDropStage) process(ctx context.Context, entries []Entry) error {
	for _, e := range entries {
		for _, label := range l.config.Values {
			delete(e.Labels, model.LabelName(label))
		}
	}
	return l.next(ctx, entries)
}

// Cleanup implements Stage.
func (l *labelDropStage) Cleanup() {}
