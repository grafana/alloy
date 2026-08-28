package static

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var _ prometheus.Gatherer = (*staticGatherer)(nil)

type staticGatherer struct {
	families []*dto.MetricFamily
}

func newStaticGatherer(familiesByName map[string]*dto.MetricFamily) *staticGatherer {
	families := make([]*dto.MetricFamily, 0, len(familiesByName))
	for _, f := range familiesByName {
		families = append(families, f)
	}

	return &staticGatherer{families: families}
}

func (s *staticGatherer) Gather() ([]*dto.MetricFamily, error) {
	return s.families, nil
}
