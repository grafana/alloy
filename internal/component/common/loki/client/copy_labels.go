//go:build !slicelabels

package client

import (
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

// promLabelsToModelLabels copies the labels to a new model.LabelSet.
func promLabelsToModelLabels(lbs labels.Labels) model.LabelSet {
	result := make(model.LabelSet, lbs.Len())
	lbs.Range(func(l labels.Label) {
		result[model.LabelName(l.Name)] = model.LabelValue(l.Value)
	})
	return result
}
