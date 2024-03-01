package processortest

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func CompareMetrics(t *testing.T, expected, actual pmetric.Metrics) {
	err := pmetrictest.CompareMetrics(
		expected,
		actual,
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreSummaryDataPointValueAtQuantileSliceOrder(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
	)
	require.NoError(t, err)
}

func CompareLogs(t *testing.T, expected, actual plog.Logs) {
	err := plogtest.CompareLogs(
		expected,
		actual,
	)
	require.NoError(t, err)
}

func CompareTraces(t *testing.T, expected, actual ptrace.Traces) {
	err := ptracetest.CompareTraces(
		expected,
		actual,
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
	)
	require.NoError(t, err)
}
