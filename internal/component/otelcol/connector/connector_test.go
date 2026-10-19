package connector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectorTypesAreDistinctBits(t *testing.T) {
	types := map[string]int{
		"ConnectorTracesToTraces":   ConnectorTracesToTraces,
		"ConnectorTracesToMetrics":  ConnectorTracesToMetrics,
		"ConnectorTracesToLogs":     ConnectorTracesToLogs,
		"ConnectorMetricsToTraces":  ConnectorMetricsToTraces,
		"ConnectorMetricsToMetrics": ConnectorMetricsToMetrics,
		"ConnectorMetricsToLogs":    ConnectorMetricsToLogs,
		"ConnectorLogsToTraces":     ConnectorLogsToTraces,
		"ConnectorLogsToMetrics":    ConnectorLogsToMetrics,
		"ConnectorLogsToLogs":       ConnectorLogsToLogs,
	}

	seen := make(map[int]string, len(types))
	for name, value := range types {
		require.NotZero(t, value, "%s must not be zero", name)
		require.Zero(t, value&(value-1), "%s must be a single bit, got %d", name, value)

		if other, ok := seen[value]; ok {
			require.Failf(t, "duplicate connector type", "%s and %s are both %d", name, other, value)
		}
		seen[value] = name
	}
}

func TestOutputCapabilities(t *testing.T) {
	tests := []struct {
		testName      string
		connectorType int
		metrics       bool
		logs          bool
		traces        bool
	}{
		{
			testName:      "TracesToMetrics",
			connectorType: ConnectorTracesToMetrics,
			metrics:       true,
		},
		{
			testName:      "TracesToLogs",
			connectorType: ConnectorTracesToLogs,
			logs:          true,
		},
		{
			testName:      "TracesToTraces",
			connectorType: ConnectorTracesToTraces,
			traces:        true,
		},
		{
			testName:      "MetricsToMetrics",
			connectorType: ConnectorMetricsToMetrics,
			metrics:       true,
		},
		{
			testName:      "MetricsToLogs",
			connectorType: ConnectorMetricsToLogs,
			logs:          true,
		},
		{
			testName:      "MetricsToTraces",
			connectorType: ConnectorMetricsToTraces,
			traces:        true,
		},
		{
			testName:      "LogsToMetrics",
			connectorType: ConnectorLogsToMetrics,
			metrics:       true,
		},
		{
			testName:      "LogsToLogs",
			connectorType: ConnectorLogsToLogs,
			logs:          true,
		},
		{
			testName:      "LogsToTraces",
			connectorType: ConnectorLogsToTraces,
			traces:        true,
		},
		{
			// The combination declared by otelcol.connector.count and
			// otelcol.connector.signaltometrics.
			testName:      "AllSignalsToMetrics",
			connectorType: ConnectorTracesToMetrics | ConnectorMetricsToMetrics | ConnectorLogsToMetrics,
			metrics:       true,
		},
		{
			testName:      "MetricsAndLogs",
			connectorType: ConnectorTracesToMetrics | ConnectorTracesToLogs,
			metrics:       true,
			logs:          true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			require.Equal(t, tc.metrics, outputsToMetrics(tc.connectorType), "outputsToMetrics")
			require.Equal(t, tc.logs, outputsToLogs(tc.connectorType), "outputsToLogs")
			require.Equal(t, tc.traces, outputsToTraces(tc.connectorType), "outputsToTraces")
		})
	}
}
