//go:build slicelabels

package client

import (
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

// promLabelsToModelLabels copies the labels to a new model.LabelSet.
// The slicelabels implementation uses strings.Clone as the strings used
// for labels are backed by reused buffers. This avoids memory corruption.
func promLabelsToModelLabels(lbs labels.Labels) model.LabelSet {
	result := make(model.LabelSet, lbs.Len())
	lbs.Range(func(l labels.Label) {
		result[model.LabelName(strings.Clone(l.Name))] = model.LabelValue(strings.Clone(l.Value))
	})
	return result
}
