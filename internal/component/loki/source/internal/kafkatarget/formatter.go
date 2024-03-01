package kafkatarget

// This code is copied from Promtail (https://github.com/grafana/loki/commit/065bee7e72b00d800431f4b70f0d673d6e0e7a2b). The kafkatarget package is used to
// configure and run the targets that can read kafka entries and forward them
// to other loki components.

import (
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
)

func format(lbs labels.Labels, cfg []*relabel.Config) model.LabelSet {
	if len(lbs) == 0 {
		return nil
	}
	processed, _ := relabel.Process(lbs, cfg...)
	labelOut := model.LabelSet(LabelsToMetric(processed))
	for k := range labelOut {
		if strings.HasPrefix(string(k), "__") {
			delete(labelOut, k)
		}
	}
	return labelOut
}

// LabelsToMetric converts a Labels to Metric
// Don't do this on any performance sensitive paths.
func LabelsToMetric(ls labels.Labels) model.Metric {
	m := make(model.Metric, len(ls))
	for _, l := range ls {
		m[model.LabelName(l.Name)] = model.LabelValue(l.Value)
	}
	return m
}
