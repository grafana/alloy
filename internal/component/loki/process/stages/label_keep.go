package stages

import (
	"context"
	"errors"

	"github.com/grafana/alloy/syntax"
)

// errEmptyLabelKeepStageConfig error is returned if the config is empty.
var errEmptyLabelKeepStageConfig = errors.New("labelkeep stage config cannot be empty")

// LabelKeepConfig contains the slice of labels to allow through.
type LabelKeepConfig struct {
	Values []string `alloy:"values,attr"`
}

var _ syntax.Validator = (*LabelKeepConfig)(nil)

func (l *LabelKeepConfig) Validate() error {
	if len(l.Values) < 1 {
		return errEmptyLabelKeepStageConfig
	}

	return nil
}

var _ entryProcessor = (*labelKeepStage)(nil)

func newLabelKeepStage(config LabelKeepConfig, opts stageOpts) *labelKeepStage {
	labelMap := make(map[string]struct{})
	for _, label := range config.Values {
		labelMap[label] = struct{}{}
	}

	return &labelKeepStage{
		next:   opts.next,
		labels: labelMap,
	}
}

type labelKeepStage struct {
	next   nextFn
	labels map[string]struct{}
}

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
