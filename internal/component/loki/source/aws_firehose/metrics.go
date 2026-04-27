package aws_firehose

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type metrics struct {
	entriesWritten           prometheus.Counter
	errorsAPIRequest         *prometheus.CounterVec
	recordsReceived          *prometheus.CounterVec
	errorsRecord             *prometheus.CounterVec
	batchSize                prometheus.Observer
	invalidStaticLabelsCount *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	return &metrics{
		entriesWritten: util.MustRegisterOrGet(reg, prometheus.NewCounter(prometheus.CounterOpts{
			Name: "loki_source_awsfirehose_entries_written",
			Help: "Total number of entries written.",
		})).(prometheus.Counter),
		errorsAPIRequest: util.MustRegisterOrGet(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "loki_source_awsfirehose_request_errors",
			Help: "Number of errors while receiving AWS Firehose API requests",
		}, []string{"reason"})).(*prometheus.CounterVec),
		errorsRecord: util.MustRegisterOrGet(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "loki_source_awsfirehose_record_errors",
			Help: "Number of errors while decoding AWS Firehose records",
		}, []string{"reason"})).(*prometheus.CounterVec),
		recordsReceived: util.MustRegisterOrGet(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "loki_source_awsfirehose_records_received",
			Help: "Number of records received from AWS Firehose",
		}, []string{"type"})).(*prometheus.CounterVec),
		batchSize: util.MustRegisterOrGet(reg, prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "loki_source_awsfirehose_batch_size",
			Help: "AWS Firehose received batch size in number of records",
		})).(prometheus.Observer),
		invalidStaticLabelsCount: util.MustRegisterOrGet(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "loki_source_awsfirehose_invalid_static_labels_errors",
			Help: "Number of errors while processing AWS Firehose static labels",
		}, []string{"reason", "tenant_id"})).(*prometheus.CounterVec),
	}
}

func (m *metrics) IncRequestError(reason string) {
	m.errorsAPIRequest.WithLabelValues(reason).Inc()
}

func (m *metrics) IncRecordError(reason string) {
	m.errorsRecord.WithLabelValues(reason).Inc()
}

func (m *metrics) IncRecordsReceived(recordType string) {
	m.recordsReceived.WithLabelValues(recordType).Inc()
}

func (m *metrics) ObserveBatchSize(size float64) {
	m.batchSize.Observe(size)
}

func (m *metrics) IncInvalidStaticLabels(reason, tenantID string) {
	m.invalidStaticLabelsCount.WithLabelValues(reason, tenantID).Inc()
}
