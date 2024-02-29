package app_agent_receiver

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// ReceiverMetricsExporter is an app agent receiver exporter that will capture metrics
// about counts of logs, exceptions, measurements, traces being ingested
type ReceiverMetricsExporter struct {
	totalLogs         prometheus.Counter
	totalMeasurements prometheus.Counter
	totalExceptions   prometheus.Counter
	totalEvents       prometheus.Counter
}

// NewReceiverMetricsExporter creates a new ReceiverMetricsExporter
func NewReceiverMetricsExporter(reg prometheus.Registerer) AppAgentReceiverExporter {
	exp := &ReceiverMetricsExporter{
		totalLogs: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "app_agent_receiver_logs_total",
			Help: "Total number of ingested logs",
		}),
		totalMeasurements: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "app_agent_receiver_measurements_total",
			Help: "Total number of ingested measurements",
		}),
		totalExceptions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "app_agent_receiver_exceptions_total",
			Help: "Total number of ingested exceptions",
		}),
		totalEvents: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "app_agent_receiver_events_total",
			Help: "Total number of ingested events",
		}),
	}

	reg.MustRegister(exp.totalLogs, exp.totalExceptions, exp.totalMeasurements, exp.totalEvents)

	return exp
}

// Name of the exporter, for logging purposes
func (re *ReceiverMetricsExporter) Name() string {
	return "receiver metrics exporter"
}

// Export implements the AppDataExporter interface
func (re *ReceiverMetricsExporter) Export(ctx context.Context, payload Payload) error {
	re.totalExceptions.Add(float64(len(payload.Exceptions)))
	re.totalLogs.Add(float64(len(payload.Logs)))
	re.totalMeasurements.Add(float64(len(payload.Measurements)))
	re.totalEvents.Add(float64(len(payload.Events)))
	return nil
}

// Static typecheck tests
var (
	_ AppAgentReceiverExporter = (*ReceiverMetricsExporter)(nil)
)
