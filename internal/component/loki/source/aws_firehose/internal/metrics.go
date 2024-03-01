package internal

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	errorsAPIRequest *prometheus.CounterVec
	recordsReceived  *prometheus.CounterVec
	errorsRecord     *prometheus.CounterVec
	batchSize        *prometheus.HistogramVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := Metrics{}
	m.errorsAPIRequest = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_awsfirehose_request_errors",
		Help: "Number of errors while receiving AWS Firehose API requests",
	}, []string{"reason"})

	m.errorsRecord = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_awsfirehose_record_errors",
		Help: "Number of errors while decoding AWS Firehose records",
	}, []string{"reason"})

	m.recordsReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_awsfirehose_records_received",
		Help: "Number of records received from AWS Firehose",
	}, []string{"type"})

	m.batchSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "loki_source_awsfirehose_batch_size",
		Help: "AWS Firehose received batch size in number of records",
	}, nil)

	if reg != nil {
		reg.MustRegister(
			m.errorsAPIRequest,
			m.recordsReceived,
			m.errorsRecord,
			m.batchSize,
		)
	}

	return &m
}
