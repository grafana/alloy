package metadata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_inferMetadata(t *testing.T) {
	tests := []struct {
		name     string
		expected Metadata
	}{
		{
			name:     "discovery.dns",
			expected: Metadata{exports: []Type{TypeTargets}},
		},
		{
			name: "discovery.relabel",
			expected: Metadata{
				accepts: []Type{TypeTargets},
				exports: []Type{TypeTargets},
			},
		},
		{
			name:     "loki.echo",
			expected: Metadata{exports: []Type{TypeLokiLogs}},
		},
		{
			name: "loki.source.file",
			expected: Metadata{
				accepts: []Type{TypeTargets, TypeLokiLogs},
			},
		},
		{
			name: "loki.process",
			expected: Metadata{
				accepts: []Type{TypeLokiLogs},
				exports: []Type{TypeLokiLogs},
			},
		},
		{
			name: "prometheus.relabel",
			expected: Metadata{
				accepts: []Type{TypePromMetricsReceiver},
				exports: []Type{TypePromMetricsReceiver},
			},
		},
		{
			name: "prometheus.remote_write",
			expected: Metadata{
				accepts: []Type{},
				exports: []Type{TypePromMetricsReceiver},
			},
		},
		{
			name: "otelcol.exporter.otlp",
			expected: Metadata{
				accepts: []Type{},
				exports: []Type{TypeOTELReceiver},
			},
		},
		{
			name: "otelcol.processor.filter",
			expected: Metadata{
				accepts: []Type{TypeOTELReceiver},
				exports: []Type{TypeOTELReceiver},
			},
		},
		{
			name: "faro.receiver",
			expected: Metadata{
				accepts: []Type{TypeLokiLogs, TypeOTELReceiver},
				exports: []Type{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ForComponent(tt.name)
			require.NoError(t, err)

			compareSlices := func(expected, actual []Type, name string) {
				require.Equal(t, len(expected), len(actual), "expected %d %s types, got %d; expected: %v, actual: %v", len(expected), name, len(actual), expected, actual)
				for i := range expected {
					require.Equal(t, expected[i].Name, actual[i].Name, "expected %s type at %d to be %q, got %q", name, i, expected[i].Name, actual[i].Name)
				}
			}

			compareSlices(tt.expected.AllTypesAccepted(), actual.AllTypesAccepted(), "accepted")
			compareSlices(tt.expected.AllTypesExported(), actual.AllTypesExported(), "exported")
		})
	}
}
