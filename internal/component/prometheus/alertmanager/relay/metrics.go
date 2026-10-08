package relay

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type metrics struct {
	webhookRequests         prometheus.Counter
	webhookRequestDuration  prometheus.Histogram
	receivedAlerts          prometheus.Counter
	forwardedAlerts         prometheus.Counter
	failedAlerts            prometheus.Counter
	outboundRequests        prometheus.Counter
	outboundRequestFailures *prometheus.CounterVec
	outboundRequestDuration prometheus.Histogram
	activeRequests          prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		webhookRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_alertmanager_relay_webhook_requests_total",
			Help: "Total number of webhook requests received by the Alertmanager relay.",
		}),
		webhookRequestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:                            "prometheus_alertmanager_relay_webhook_request_duration_seconds",
			Help:                            "Duration of Alertmanager webhook requests.",
			Buckets:                         prometheus.DefBuckets,
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: time.Hour,
		}),
		receivedAlerts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_alertmanager_relay_received_alerts_total",
			Help: "Total number of alerts received in valid webhook envelopes.",
		}),
		forwardedAlerts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_alertmanager_relay_forwarded_alerts_total",
			Help: "Total number of alerts successfully forwarded to Alertmanager.",
		}),
		failedAlerts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_alertmanager_relay_failed_alerts_total",
			Help: "Total number of received alerts that could not be forwarded.",
		}),
		outboundRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_alertmanager_relay_outbound_requests_total",
			Help: "Total number of requests sent to the destination Alertmanager.",
		}),
		outboundRequestFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "prometheus_alertmanager_relay_outbound_request_failures_total",
			Help: "Total number of failed requests to the destination Alertmanager.",
		}, []string{"reason"}),
		outboundRequestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:                            "prometheus_alertmanager_relay_outbound_request_duration_seconds",
			Help:                            "Duration of requests to the destination Alertmanager.",
			Buckets:                         prometheus.DefBuckets,
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: time.Hour,
		}),
		activeRequests: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "prometheus_alertmanager_relay_active_requests",
			Help: "Current number of active webhook requests.",
		}),
	}

	m.webhookRequests = util.MustRegisterOrGet(reg, m.webhookRequests).(prometheus.Counter)
	m.webhookRequestDuration = util.MustRegisterOrGet(reg, m.webhookRequestDuration).(prometheus.Histogram)
	m.receivedAlerts = util.MustRegisterOrGet(reg, m.receivedAlerts).(prometheus.Counter)
	m.forwardedAlerts = util.MustRegisterOrGet(reg, m.forwardedAlerts).(prometheus.Counter)
	m.failedAlerts = util.MustRegisterOrGet(reg, m.failedAlerts).(prometheus.Counter)
	m.outboundRequests = util.MustRegisterOrGet(reg, m.outboundRequests).(prometheus.Counter)
	m.outboundRequestFailures = util.MustRegisterOrGet(reg, m.outboundRequestFailures).(*prometheus.CounterVec)
	m.outboundRequestDuration = util.MustRegisterOrGet(reg, m.outboundRequestDuration).(prometheus.Histogram)
	m.activeRequests = util.MustRegisterOrGet(reg, m.activeRequests).(prometheus.Gauge)

	return m
}
