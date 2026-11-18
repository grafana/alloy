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

var _ entryProcessor = (*labelDropStage)(nil)

func newLabelDropStage(config LabelDropConfig, opts stageOpts) (*labelDropStage, error) {
	if len(config.Values) < 1 {
		return nil, errEmptyLabelDropStageConfig
	}

	return &labelDropStage{
		next:   opts.next,
		config: config,
	}, nil
}

type labelDropStage struct {
	next   nextFn
	config LabelDropConfig
}

func (l *labelDropStage) process(ctx context.Context, entries []Entry) error {
	for _, e := range entries {
		for _, label := range l.config.Values {
			delete(e.Labels, model.LabelName(label))
		}
	}
	return l.next(ctx, entries)
}
