package googlecloud_test

import (
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/googlecloud"
	googlecloudconfig "github.com/grafana/alloy/internal/component/otelcol/exporter/googlecloud/config"
	"github.com/grafana/alloy/syntax"
)

func TestConfigConversion(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected googlecloudexporter.Config
	}{
		{
			testName: "default",
			agentCfg: `
			`,
			expected: googlecloudexporter.Config{
				Config: collector.Config{
					ProjectID:         "",
					UserAgent:         "opentelemetry-collector-contrib {{version}}",
					ImpersonateConfig: collector.ImpersonateConfig{},
					MetricConfig: collector.MetricConfig{
						Prefix:       "workload.googleapis.com",
						KnownDomains: []string{"googleapis.com", "kubernetes.io", "istio.io", "knative.dev"},
						ClientConfig: collector.ClientConfig{
							Endpoint: "monitoring.googleapis.com:443",
						},
						InstrumentationLibraryLabels:     true,
						CreateMetricDescriptorBufferSize: 10,
						ServiceResourceLabels:            true,
						ResourceFilters:                  make([]collector.ResourceFilter, 0),
						CumulativeNormalization:          true,
					},
					TraceConfig: collector.TraceConfig{
						ClientConfig: collector.ClientConfig{
							Endpoint: "cloudtrace.googleapis.com:443",
						},
						AttributeMappings: make([]collector.AttributeMapping, 0),
					},
					LogConfig: collector.LogConfig{
						ClientConfig: collector.ClientConfig{
							Endpoint: "logging.googleapis.com:443",
						},
						ResourceFilters:       make([]collector.ResourceFilter, 0),
						ServiceResourceLabels: true,
					},
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
				destination_project_quota = true
				user_agent = "custom-user-agent"

				impersonate {
					target_principal = "target@example.com"
					subject = "jwt-sub-test"
					delegates = ["foo", "bar"]
				}

				metric {
					prefix = "custom-prefix.example.com"
					endpoint = "monitoring.example.com:443"
					compression = "gzip"
					grpc_pool_size = 5
					use_insecure = true
					known_domains = ["googleapis.com", "example.com"]
					skip_create_descriptor = true
					instrumentation_library_labels = false
					create_service_timeseries = true
					create_metric_descriptor_buffer_size = 6
					service_resource_labels = false
					resource_filters = [
						{ prefix = "my-prefix", regex = "my-regex" },
					]
					cumulative_normalization = false
					sum_of_squared_deviation = true
					experimental_wal {
						directory = "./foo"
						max_backoff = "2h"
					}
				}

				trace {
					endpoint = "cloudtrace.example.com:443"
					grpc_pool_size = 7
					use_insecure = true
					attribute_mappings = [
						{ key = "baz", replacement = "qux" },
					]
				}

				log {
					endpoint = "logging.example.com:443"
					compression = "gzip"
					grpc_pool_size = 8
					use_insecure = true
					default_log_name = "my-default-log-name"
					resource_filters = [
						{ prefix = "foo", regex = "bar" },
					]
					service_resource_labels = false
					error_reporting_type = true
				}

				sending_queue {
					enabled = false
				}
			`,
			expected: googlecloudexporter.Config{
				Config: collector.Config{
					ProjectID:               "foo-bar",
					DestinationProjectQuota: true,
					UserAgent:               "custom-user-agent",
					ImpersonateConfig: collector.ImpersonateConfig{
						TargetPrincipal: "target@example.com",
						Subject:         "jwt-sub-test",
						Delegates:       []string{"foo", "bar"},
					},
					MetricConfig: collector.MetricConfig{
						Prefix: "custom-prefix.example.com",
						ClientConfig: collector.ClientConfig{
							Endpoint:     "monitoring.example.com:443",
							Compression:  "gzip",
							UseInsecure:  true,
							GRPCPoolSize: 5,
						},
						KnownDomains:                     []string{"googleapis.com", "example.com"},
						SkipCreateMetricDescriptor:       true,
						InstrumentationLibraryLabels:     false,
						CreateServiceTimeSeries:          true,
						CreateMetricDescriptorBufferSize: 6,
						ServiceResourceLabels:            false,
						ResourceFilters: []collector.ResourceFilter{
							{Prefix: "my-prefix", Regex: "my-regex"},
						},
						CumulativeNormalization:     false,
						EnableSumOfSquaredDeviation: true,
						WALConfig: &collector.WALConfig{
							Directory:  "./foo",
							MaxBackoff: 2 * time.Hour,
						},
					},
					TraceConfig: collector.TraceConfig{
						ClientConfig: collector.ClientConfig{
							Endpoint:     "cloudtrace.example.com:443",
							GRPCPoolSize: 7,
							UseInsecure:  true,
						},
						AttributeMappings: []collector.AttributeMapping{
							{Key: "baz", Replacement: "qux"},
						},
					},
					LogConfig: collector.LogConfig{
						ClientConfig: collector.ClientConfig{
							Endpoint:     "logging.example.com:443",
							Compression:  "gzip",
							UseInsecure:  true,
							GRPCPoolSize: 8,
						},
						DefaultLogName: "my-default-log-name",
						ResourceFilters: []collector.ResourceFilter{
							{Prefix: "foo", Regex: "bar"},
						},
						ServiceResourceLabels: false,
						ErrorReportingType:    true,
					},
				},
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 12 * time.Second,
				},
				QueueSettings: configoptional.None[exporterhelper.QueueBatchConfig](),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args googlecloud.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.agentCfg), &args))
			actual, err := args.Convert()
			require.NoError(t, err)
			actualCfg := actual.(*googlecloudexporter.Config)
			// testify cannot test for function field equality, so set them to nil to correctly compare other fields
			actualCfg.MetricConfig.GetMetricName = nil
			actualCfg.MetricConfig.MapMonitoredResource = nil
			actualCfg.LogConfig.MapMonitoredResource = nil
			require.Equal(t, &tc.expected, actualCfg)
		})
	}
}

func TestValidate(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       *googlecloud.Arguments
		expectErr bool
	}{
		{
			name: "invalid config",
			cfg: &googlecloud.Arguments{
				Metric: googlecloudconfig.GoogleCloudMetricArguments{
					ResourceFilters: []googlecloudconfig.ResourceFilter{
						{
							Regex: "invalid regex(",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name:      "valid config",
			cfg:       &googlecloud.Arguments{},
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
