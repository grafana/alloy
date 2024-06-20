package remotewrite

import (
	"github.com/prometheus/client_golang/prometheus"
)

type translator struct {
	destination     prometheus.Registerer
	source          prometheus.Registerer
	numActiveSeries prometheus.Gauge
}

func (t translator) Register(collector prometheus.Collector) error {
	collector.Describe(nil)
	// TODO implement me
	collector.Collect()
	panic("implement me")
}

func (t translator) MustRegister(collector ...prometheus.Collector) {
	// TODO implement me
	panic("implement me")
}

func (t translator) Unregister(collector prometheus.Collector) bool {
	// TODO implement me
	panic("implement me")
}

func (t translator) Describe(descs chan<- *prometheus.Desc) {
	// TODO implement me
	panic("implement me")
}

func (t translator) Collect(metrics chan<- prometheus.Metric) {
	// TODO implement me
	panic("implement me")
}

/*
	m.numActiveSeries = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prometheus_remote_write_wal_storage_active_series",
		Help: "Current number of active series being tracked by the WAL storage",
	})

	m.numDeletedSeries = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prometheus_remote_write_wal_storage_deleted_series",
		Help: "Current number of series marked for deletion from memory",
	})

	m.totalOutOfOrderSamples = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_out_of_order_samples_total",
		Help: "Total number of out of order samples ingestion failed attempts.",
	})

	m.totalCreatedSeries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_storage_created_series_total",
		Help: "Total number of created series appended to the WAL",
	})

	m.totalRemovedSeries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_storage_removed_series_total",
		Help: "Total number of created series removed from the WAL",
	})

	m.totalAppendedSamples = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_samples_appended_total",
		Help: "Total number of samples appended to the WAL",
	})

	m.totalAppendedExemplars = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_exemplars_appended_total",
		Help: "Total number of exemplars appended to the WAL",
	})
*/
