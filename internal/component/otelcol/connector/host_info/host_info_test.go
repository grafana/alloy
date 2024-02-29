package host_info

import (
	"testing"
	"time"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalRiver(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected Config
		errorMsg string
	}{
		{
			testName: "Defaults",
			cfg: `
				output {}
			`,
			expected: Config{
				HostIdentifiers:      []string{"host.id"},
				MetricsFlushInterval: 60 * time.Second,
			},
		},
		{
			testName: "ExplicitValues",
			cfg: `
				metrics_flush_interval = "10s"
				host_identifiers = ["host.id", "host.name"]
				output {}
				`,
			expected: Config{
				HostIdentifiers:      []string{"host.id", "host.name"},
				MetricsFlushInterval: 10 * time.Second,
			},
		},
		{
			testName: "InvalidHostIdentifiers",
			cfg: `
				host_identifiers = []
				output {}
				`,
			errorMsg: "host_identifiers must not be empty",
		},
		{
			testName: "InvalidMetricsFlushInterval",
			cfg: `
				metrics_flush_interval = "0s"
				output {}
				`,
			errorMsg: "metrics_flush_interval must be greater than 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args Arguments
			err := river.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.ErrorContains(t, err, tc.errorMsg)
				return
			}

			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*Config)

			require.Equal(t, tc.expected, *actual)
		})
	}
}
