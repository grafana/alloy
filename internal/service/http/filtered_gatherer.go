package http

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type filteredGatherer struct {
	base       prometheus.Gatherer
	components map[string]struct{}
}

func newFilteredGatherer(base prometheus.Gatherer, filters []string) *filteredGatherer {
	cset := make(map[string]struct{}, len(filters))
	for _, c := range filters {
		cset[c] = struct{}{}
	}
	return &filteredGatherer{base: base, components: cset}
}

func (f *filteredGatherer) Gather() ([]*dto.MetricFamily, error) {
	families, err := f.base.Gather()
	if err != nil {
		return nil, err
	}

	// If no filters are specified, return all metrics
	if len(f.components) == 0 {
		return families, nil
	}

	var filtered []*dto.MetricFamily
	for _, mf := range families {
		var kept []*dto.Metric
		for _, m := range mf.Metric {
			var componentValue string
			var hasComponentLabel bool
			for _, label := range m.Label {
				if label.GetName() == "component" {
					componentValue = label.GetValue()
					hasComponentLabel = true
					break
				}
			}
			// Only include metrics that have a component label and match one of the filters
			if hasComponentLabel {
				if _, ok := f.components[componentValue]; ok {
					// Create a new metric with only the matching component label
					newMetric := &dto.Metric{
						Label:       m.Label,
						Counter:     m.Counter,
						Gauge:       m.Gauge,
						Untyped:     m.Untyped,
						Summary:     m.Summary,
						Histogram:   m.Histogram,
						TimestampMs: m.TimestampMs,
					}
					kept = append(kept, newMetric)
				}
			}
		}
		if len(kept) > 0 {
			filtered = append(filtered, &dto.MetricFamily{
				Name:   mf.Name,
				Help:   mf.Help,
				Type:   mf.Type,
				Metric: kept,
			})
		}
	}

	return filtered, nil
}
