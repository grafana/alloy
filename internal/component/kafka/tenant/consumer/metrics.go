package consumer

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type metrics struct {
	recordsConsumed    *prometheus.CounterVec
	dropped            *prometheus.CounterVec
	processDuration    *prometheus.HistogramVec
	assignedPartitions prometheus.Gauge
	rebalances         prometheus.Counter
	lag                *prometheus.GaugeVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		recordsConsumed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_tenant_consumer_records_consumed_total",
			Help: "Total number of records forwarded downstream.",
		}, []string{"signal", "format"}),
		dropped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_tenant_consumer_dropped_total",
			Help: "Total number of records dropped.",
		}, []string{"reason", "format"}),
		processDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kafka_tenant_consumer_process_duration_seconds",
			Help:    "Time spent processing one record, including retries.",
			Buckets: []float64{.001, .005, .01, .05, .1, .5, 1, 5, 10, 30},
		}, []string{"format"}),
		assignedPartitions: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kafka_tenant_consumer_assigned_partitions",
			Help: "Number of partitions currently assigned to this consumer.",
		}),
		rebalances: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "kafka_tenant_consumer_rebalances_total",
			Help: "Total number of times partitions were assigned to this consumer.",
		}),
		lag: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kafka_tenant_consumer_lag",
			Help: "Records between the last processed record and the partition's high watermark, as of its fetch.",
		}, []string{"topic", "partition"}),
	}
	m.recordsConsumed = util.MustRegisterOrGet(reg, m.recordsConsumed).(*prometheus.CounterVec)
	m.dropped = util.MustRegisterOrGet(reg, m.dropped).(*prometheus.CounterVec)
	m.processDuration = util.MustRegisterOrGet(reg, m.processDuration).(*prometheus.HistogramVec)
	m.assignedPartitions = util.MustRegisterOrGet(reg, m.assignedPartitions).(prometheus.Gauge)
	m.rebalances = util.MustRegisterOrGet(reg, m.rebalances).(prometheus.Counter)
	m.lag = util.MustRegisterOrGet(reg, m.lag).(*prometheus.GaugeVec)
	return m
}
