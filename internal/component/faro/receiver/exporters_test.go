package receiver

import (
	"context"
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

var metricNames = []string{
	"faro_receiver_logs_total",
	"faro_receiver_measurements_total",
	"faro_receiver_exceptions_total",
	"faro_receiver_events_total",
}

func Test_metricsExporter_Export(t *testing.T) {
	var (
		reg = prometheus.NewRegistry()
		exp = newMetricsExporter(reg)
	)

	expect := `
		# HELP faro_receiver_logs_total Total number of ingested logs
		# TYPE faro_receiver_logs_total counter
		faro_receiver_logs_total 2

		# HELP faro_receiver_measurements_total Total number of ingested measurements
		# TYPE faro_receiver_measurements_total counter
		faro_receiver_measurements_total 3

		# HELP faro_receiver_exceptions_total Total number of ingested exceptions
		# TYPE faro_receiver_exceptions_total counter
		faro_receiver_exceptions_total 4

		# HELP faro_receiver_events_total Total number of ingested events
		# TYPE faro_receiver_events_total counter
		faro_receiver_events_total 5
	`

	p := payload.Payload{
		Logs:         make([]payload.Log, 2),
		Measurements: make([]payload.Measurement, 3),
		Exceptions:   make([]payload.Exception, 4),
		Events:       make([]payload.Event, 5),
	}
	require.NoError(t, exp.Export(context.Background(), p))

	err := promtestutil.CollectAndCompare(reg, strings.NewReader(expect), metricNames...)
	require.NoError(t, err)
}
