package googlecloudpubsub

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/grafana/alloy/syntax"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected googlecloudpubsubreceiver.Config
	}{
		{
			testName: "default configuration",
			cfg: `
				subscription = "projects/test-project/subscriptions/test-subscription"
				output {}
			`,
			expected: googlecloudpubsubreceiver.Config{
				Subscription: "projects/test-project/subscriptions/test-subscription",
				UserAgent:    "opentelemetry-collector-contrib {{version}}",
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 12 * time.Second,
				},
			},
		},
		{
			testName: "full configuration",
			cfg: `
				project = "test-project"
				user_agent = "custom-user-agent"
				endpoint = "https://www.googleapis.com/"
				insecure = true
				subscription = "projects/test-project/subscriptions/test-subscription"
				encoding = "otlp_proto_log"
				compression = "gzip"
				ignore_encoding_error = true
				client_id = "123"
				timeout = "15s"

				output {}
			`,
			expected: googlecloudpubsubreceiver.Config{
				ProjectID:           "test-project",
				UserAgent:           "custom-user-agent",
				Endpoint:            "https://www.googleapis.com/",
				Insecure:            true,
				Subscription:        "projects/test-project/subscriptions/test-subscription",
				Encoding:            "otlp_proto_log",
				Compression:         "gzip",
				IgnoreEncodingError: true,
				ClientID:            "123",
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 15 * time.Second,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*googlecloudpubsubreceiver.Config)

			require.Equal(t, tc.expected, *actual)
		})
	}
}

func TestValidate(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       *Arguments
		expectErr bool
	}{
		{
			name: "valid config",
			cfg: &Arguments{
				Subscription: "projects/project-id/subscriptions/subscription-name",
			},
			expectErr: false,
		},
		{
			name: "invalid config",
			cfg: &Arguments{
				Subscription: "name",
			},
			expectErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
