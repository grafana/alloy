package mysql

import (
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type RelabelingGatherer struct {
	gatherer prometheus.Gatherer
	rules    []*alloy_relabel.Config
}

func (g *RelabelingGatherer) Gather() ([]*dto.MetricFamily, error) {
	metricFamilies, err := g.gatherer.Gather()
	if err != nil {
		return nil, err
	}

	for _, mf := range metricFamilies {
		for _, metric := range mf.GetMetric() {
			builder := newLabelBuilder(metric.GetLabel())
			alloy_relabel.ProcessBuilder(builder, g.rules...)
			metric.Label = builder.labels
		}
	}

	return metricFamilies, nil
}

// labelBuilder implements the alloy_relabel.LabelBuilder interface for dto.LabelPair slices
type labelBuilder struct {
	labels []*dto.LabelPair
}

func newLabelBuilder(labels []*dto.LabelPair) *labelBuilder {
	return &labelBuilder{labels: labels}
}

func (lb *labelBuilder) Get(label string) string {
	for _, l := range lb.labels {
		if l.GetName() == label {
			return l.GetValue()
		}
	}
	return ""
}

func (lb *labelBuilder) Range(f func(label string, value string)) {
	for _, l := range lb.labels {
		f(l.GetName(), l.GetValue())
	}
}

func (lb *labelBuilder) Set(label string, val string) {
	for i, l := range lb.labels {
		if l.GetName() == label {
			if val == "" {
				lb.labels = append(lb.labels[:i], lb.labels[i+1:]...)
				return
			}
			l.Value = &val
			return
		}
	}

	if val != "" {
		name := label
		lb.labels = append(lb.labels, &dto.LabelPair{
			Name:  &name,
			Value: &val,
		})
	}
}

func (lb *labelBuilder) Del(names ...string) {
	for _, name := range names {
		for i, l := range lb.labels {
			if l.GetName() == name {
				lb.labels = append(lb.labels[:i], lb.labels[i+1:]...)
				break
			}
		}
	}
}
