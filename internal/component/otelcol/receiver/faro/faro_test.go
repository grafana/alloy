package faro_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/faro"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		alloyCfg string
		expected faro.Arguments
	}{
		{
			testName: "default",
			alloyCfg: `
			output {}
			`,
			expected: faro.Arguments{
				HTTPServer: otelcol.HTTPServerArguments{
					Endpoint:              "localhost:8080",
					CompressionAlgorithms: otelcol.DefaultCompressionAlgorithms,
				},
				Output: &otelcol.ConsumerArguments{},
			},
		},
		{
			testName: "custom_endpoint",
			alloyCfg: `
			endpoint = "localhost:9999"
			output {}
			`,
			expected: faro.Arguments{
				HTTPServer: otelcol.HTTPServerArguments{
					Endpoint:              "localhost:9999",
					CompressionAlgorithms: otelcol.DefaultCompressionAlgorithms,
				},
				Output: &otelcol.ConsumerArguments{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args faro.Arguments
			err := syntax.Unmarshal([]byte(tc.alloyCfg), &args)
			require.NoError(t, err)

			actual, err := args.Convert()
			require.NoError(t, err)

			expected, err := tc.expected.Convert()
			require.NoError(t, err)

			require.Equal(t, expected.(*faroreceiver.Config), actual.(*faroreceiver.Config))
		})
	}
}
