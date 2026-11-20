package fanoutconsumer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestTracesFromConsumers_NilConsumers(t *testing.T) {
	// Case 1: All nil consumers or empty
	require.NotNil(t, TracesFromConsumers([]otelconsumer.Traces{}))
	require.NotNil(t, TracesFromConsumers([]otelconsumer.Traces{nil}))
	require.NotNil(t, TracesFromConsumers([]otelconsumer.Traces{nil, nil}))

	// Case 2: Nil consumer at end (issue #2448)
	c1 := consumertest.NewNop()
	c2 := consumertest.NewNop()
	fanoutEnd := TracesFromConsumers([]otelconsumer.Traces{c1, c2, nil})
	require.NotNil(t, fanoutEnd)
	err := fanoutEnd.ConsumeTraces(context.Background(), ptrace.NewTraces())
	require.NoError(t, err)

	// Case 3: Nil consumer in middle
	fanoutMid := TracesFromConsumers([]otelconsumer.Traces{c1, nil, c2})
	require.NotNil(t, fanoutMid)
	err = fanoutMid.ConsumeTraces(context.Background(), ptrace.NewTraces())
	require.NoError(t, err)

	// Case 4: Nil consumer at start
	fanoutStart := TracesFromConsumers([]otelconsumer.Traces{nil, c1, c2})
	require.NotNil(t, fanoutStart)
	err = fanoutStart.ConsumeTraces(context.Background(), ptrace.NewTraces())
	require.NoError(t, err)

	// Case 5: Single valid consumer with nils
	single := TracesFromConsumers([]otelconsumer.Traces{nil, c1, nil})
	require.Equal(t, c1, single)
}

func TestMetricsFromConsumers_NilConsumers(t *testing.T) {
	// Case 1: All nil consumers or empty
	require.NotNil(t, MetricsFromConsumers([]otelconsumer.Metrics{}))
	require.NotNil(t, MetricsFromConsumers([]otelconsumer.Metrics{nil}))
	require.NotNil(t, MetricsFromConsumers([]otelconsumer.Metrics{nil, nil}))

	// Case 2: Nil consumer at end (issue #2448)
	c1 := consumertest.NewNop()
	c2 := consumertest.NewNop()
	fanoutEnd := MetricsFromConsumers([]otelconsumer.Metrics{c1, c2, nil})
	require.NotNil(t, fanoutEnd)
	err := fanoutEnd.ConsumeMetrics(context.Background(), pmetric.NewMetrics())
	require.NoError(t, err)

	// Case 3: Nil consumer in middle
	fanoutMid := MetricsFromConsumers([]otelconsumer.Metrics{c1, nil, c2})
	require.NotNil(t, fanoutMid)
	err = fanoutMid.ConsumeMetrics(context.Background(), pmetric.NewMetrics())
	require.NoError(t, err)

	// Case 4: Nil consumer at start
	fanoutStart := MetricsFromConsumers([]otelconsumer.Metrics{nil, c1, c2})
	require.NotNil(t, fanoutStart)
	err = fanoutStart.ConsumeMetrics(context.Background(), pmetric.NewMetrics())
	require.NoError(t, err)

	// Case 5: Single valid consumer with nils
	single := MetricsFromConsumers([]otelconsumer.Metrics{nil, c1, nil})
	require.Equal(t, c1, single)
}

func TestLogsFromConsumers_NilConsumers(t *testing.T) {
	// Case 1: All nil consumers or empty
	require.NotNil(t, LogsFromConsumers([]otelconsumer.Logs{}))
	require.NotNil(t, LogsFromConsumers([]otelconsumer.Logs{nil}))
	require.NotNil(t, LogsFromConsumers([]otelconsumer.Logs{nil, nil}))

	// Case 2: Nil consumer at end (issue #2448)
	c1 := consumertest.NewNop()
	c2 := consumertest.NewNop()
	fanoutEnd := LogsFromConsumers([]otelconsumer.Logs{c1, c2, nil})
	require.NotNil(t, fanoutEnd)
	err := fanoutEnd.ConsumeLogs(context.Background(), plog.NewLogs())
	require.NoError(t, err)

	// Case 3: Nil consumer in middle
	fanoutMid := LogsFromConsumers([]otelconsumer.Logs{c1, nil, c2})
	require.NotNil(t, fanoutMid)
	err = fanoutMid.ConsumeLogs(context.Background(), plog.NewLogs())
	require.NoError(t, err)

	// Case 4: Nil consumer at start
	fanoutStart := LogsFromConsumers([]otelconsumer.Logs{nil, c1, c2})
	require.NotNil(t, fanoutStart)
	err = fanoutStart.ConsumeLogs(context.Background(), plog.NewLogs())
	require.NoError(t, err)

	// Case 5: Single valid consumer with nils
	single := LogsFromConsumers([]otelconsumer.Logs{nil, c1, nil})
	require.Equal(t, c1, single)
}
