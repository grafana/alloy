package scrape

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type deltaMetricCollector struct {
	manager *Manager
	desc    *prometheus.Desc
}

func newDeltaMetricCollector(manager *Manager) *deltaMetricCollector {
	return &deltaMetricCollector{
		manager: manager,
		desc:    prometheus.NewDesc("pyroscope_delta_map_size2", "Size of the DeltaMap", []string{"target_namespace", "controller"}, nil),
	}
}

func (d *deltaMetricCollector) Describe(descs chan<- *prometheus.Desc) {
	descs <- d.desc
}

func (d *deltaMetricCollector) Collect(metrics chan<- prometheus.Metric) {
	m := d.manager
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()
	type aggregationKey struct {
		ns         string
		controller string
	}

	aggregation := make(map[aggregationKey]int64)

	loops := m.sp.activeScrapeLoops()
	for _, loop := range loops {
		if da, ok := loop.appender.(*deltaAppender); ok {
			size := da.deltaMapSize()
			ns := loop.Target.allLabels.Get("namespace")
			controller := fmt.Sprintf("%s-%s-%s",
				loop.Target.allLabels.Get("container"),
				loop.Target.allLabels.Get("__meta_kubernetes_pod_container_name"),
				loop.Target.allLabels.Get("__meta_kubernetes_pod_controller_name"),
			)
			a := aggregationKey{ns, controller}
			aggregation[a] += int64(size)
		}
	}

	for key, size := range aggregation {
		metric, err := prometheus.NewConstMetric(d.desc, prometheus.GaugeValue, float64(size), key.ns, key.controller)
		if err != nil {
			continue
		}
		metrics <- metric
	}
}
