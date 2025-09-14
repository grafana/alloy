package awsecscontainermetrics_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/awsecscontainermetrics"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected awsecscontainermetricsreceiver.Config
	}{
		{
			testName: "default configuration",
			cfg: `
				output {}
			`,
			expected: awsecscontainermetricsreceiver.Config{
				CollectionInterval: 20 * time.Second,
			},
		},
		{
			testName: "configuration with collection interval",
			cfg: `
				collection_interval = "60s"
				output {}
			`,
			expected: awsecscontainermetricsreceiver.Config{
				CollectionInterval: 60 * time.Second,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args awsecscontainermetrics.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*awsecscontainermetricsreceiver.Config)

			require.Equal(t, tc.expected, *actual)
		})
	}
}
