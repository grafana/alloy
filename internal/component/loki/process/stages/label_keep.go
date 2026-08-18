package stages

import (
	"context"
	"errors"
)

// errEmptyLabelKeepStageConfig error is returned if the config is empty.
var errEmptyLabelKeepStageConfig = errors.New("labelkeep stage config cannot be empty")

// LabelKeepConfig contains the slice of labels to allow through.
type LabelKeepConfig struct {
	Values []string `alloy:"values,attr"`
}

var (
	_ Stage = (*labelKeepStage)(nil)
	_ stage = (*labelKeepStage)(nil)
)

func newLabelKeepStage(config LabelKeepConfig, next NextFn) (*labelKeepStage, error) {
	if len(config.Values) < 1 {
		return nil, errEmptyLabelKeepStageConfig
	}

	labelMap := make(map[string]struct{})
	for _, label := range config.Values {
		labelMap[label] = struct{}{}
	}

	return &labelKeepStage{
		next:   next,
		labels: labelMap,
	}, nil
}

type labelKeepStage struct {
	next   NextFn
	labels map[string]struct{}
}

// Run implements Stage.
func (l *labelKeepStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		for label := range e.Labels {
			if _, ok := l.labels[string(label)]; !ok {
				delete(e.Labels, label)
			}
		}
		return e
	})
}

// process implements stage.
func (l *labelKeepStage) process(ctx context.Context, entries []Entry) error {
	for _, e := range entries {
		for label := range e.Labels {
			if _, ok := l.labels[string(label)]; !ok {
				delete(e.Labels, label)
			}
		}
	}
	return l.next(ctx, entries)
}

// Cleanup implements Stage.
func (l *labelKeepStage) Cleanup() {}
