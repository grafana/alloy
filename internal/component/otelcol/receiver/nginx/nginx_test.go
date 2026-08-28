package nginx_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/nginx"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName               string
		cfg                    string
		wantCollectionInterval time.Duration
		wantEndpoint           string
		wantInitialDelay       time.Duration
	}{
		{
			testName: "default configuration",
			cfg: `
				endpoint = "http://localhost:80/status"
				output {}
			`,
			wantEndpoint:           "http://localhost:80/status",
			wantCollectionInterval: 10 * time.Second,
			wantInitialDelay:       1 * time.Second,
		},
		{
			testName: "configuration with collection interval",
			cfg: `
				endpoint = "http://localhost:80/status"
				collection_interval = "60s"
				output {}
			`,
			wantEndpoint:           "http://localhost:80/status",
			wantCollectionInterval: 60 * time.Second,
			wantInitialDelay:       1 * time.Second,
		},
		{
			testName: "configuration with initial delay",
			cfg: `
				endpoint = "http://localhost:80/status"
				initial_delay = "10s"
				output {}
			`,
			wantEndpoint:           "http://localhost:80/status",
			wantCollectionInterval: 10 * time.Second,
			wantInitialDelay:       10 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args nginx.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*nginxreceiver.Config)

			want := nginxreceiver.NewFactory().CreateDefaultConfig().(*nginxreceiver.Config)
			want.ControllerConfig.CollectionInterval = tc.wantCollectionInterval
			want.ControllerConfig.InitialDelay = tc.wantInitialDelay
			want.ClientConfig.Endpoint = tc.wantEndpoint
			require.Equal(t, want, actual)
		})
	}
}
