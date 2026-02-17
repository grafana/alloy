package googlecloudpubsub_test

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/googlecloudpubsub"
	"github.com/grafana/alloy/syntax"
)

func TestConfigConversion(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected googlecloudpubsubexporter.Config
	}{
		{
			testName: "default",
			agentCfg: `
				topic = "projects/foo-bar/topics/custom-topic"
			`,
			expected: googlecloudpubsubexporter.Config{
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					RandomizationFactor: 0.5,
					Multiplier:          1.5,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
				},
				ProjectID:   "",
				UserAgent:   "opentelemetry-collector-contrib {{version}}",
				Topic:       "projects/foo-bar/topics/custom-topic",
				Compression: "",
				Watermark: googlecloudpubsubexporter.WatermarkConfig{
					Behavior:     "",
					AllowedDrift: time.Duration(0),
				},
				Endpoint: "",
				Insecure: false,
				Ordering: googlecloudpubsubexporter.OrderingConfig{
					Enabled:                 false,
					FromResourceAttribute:   "",
					RemoveResourceAttribute: false,
				},
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 12 * time.Second,
				},
				QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			},
		},
		{
			testName: "customized",
			agentCfg: `
				project = "foo-bar"
				user_agent = "custom-user-agent"
				topic = "projects/foo-bar/topics/custom-topic"
				compression = "gzip"

				watermark {
					behavior = "earliest"
					allowed_drift = "10s"
				}

				endpoint = "https://www.googleapis.com/"
				insecure = true

				ordering {
					enabled = true
					from_resource_attribute = "attr"
					remove_resource_attribute = true
				}

				sending_queue {
					enabled = false
				}

				timeout = "15s"
			`,
			expected: googlecloudpubsubexporter.Config{
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					RandomizationFactor: 0.5,
					Multiplier:          1.5,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
				},
				ProjectID:   "foo-bar",
				UserAgent:   "custom-user-agent",
				Topic:       "projects/foo-bar/topics/custom-topic",
				Compression: "gzip",
				Watermark: googlecloudpubsubexporter.WatermarkConfig{
					Behavior:     "earliest",
					AllowedDrift: time.Second * 10,
				},
				Endpoint: "https://www.googleapis.com/",
				Insecure: true,
				Ordering: googlecloudpubsubexporter.OrderingConfig{
					Enabled:                 true,
					FromResourceAttribute:   "attr",
					RemoveResourceAttribute: true,
				},
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 15 * time.Second,
				},
				QueueSettings: configoptional.None[exporterhelper.QueueBatchConfig](),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args googlecloudpubsub.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.agentCfg), &args))
			actual, err := args.Convert()
			require.NoError(t, err)
			require.Equal(t, &tc.expected, actual.(*googlecloudpubsubexporter.Config))
		})
	}
}

func TestValidate(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       *googlecloudpubsub.Arguments
		expectErr bool
	}{
		{
			name: "invalid config",
			cfg: &googlecloudpubsub.Arguments{
				Topic: "name",
			},
			expectErr: true,
		},
		{
			name: "valid config",
			cfg: &googlecloudpubsub.Arguments{
				Topic: "projects/project-id/topics/topic-name",
			},
			expectErr: false,
		},
	} {
		tt := tt
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
