package scrape

import (
	"github.com/prometheus/client_golang/prometheus"
)

type deltaMetricCollector struct {
	manager *Manager
	desc    *prometheus.Desc
	label   string
}

func newDeltaMetricCollector(manager *Manager) *deltaMetricCollector {
	return &deltaMetricCollector{
		manager: manager,
		desc:    prometheus.NewDesc("pyroscope_delta_map_size", "Size of the DeltaMap", []string{"aggregation"}, nil),
		label:   "namespace",
	}
}

func (d *deltaMetricCollector) Describe(descs chan<- *prometheus.Desc) {
	descs <- d.desc
}

func (d *deltaMetricCollector) Collect(metrics chan<- prometheus.Metric) {
	m := d.manager
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()

	aggregation := make(map[string]int64)
	for _, sp := range m.targetsGroups {
		loops := sp.activeScrapeLoops()
		for _, loop := range loops {
			if da, ok := loop.appender.(*deltaAppender); ok {
				size := da.deltaMapSize()
				k := loop.Target.allLabels.Get(d.label)
				aggregation[k] += int64(size)
			}
		}
	}

	for namespace, size := range aggregation {
		metric, err := prometheus.NewConstMetric(d.desc, prometheus.GaugeValue, float64(size), namespace)
		if err != nil {
			continue
		}
		metrics <- metric
	}
}
