package remotewrite

import (
	"net/url"
	"testing"
	"time"

	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/storage/remote/azuread"
	"github.com/prometheus/sigv4"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func expectedCfg(transform func(c *config.Config)) *config.Config {
	// Initialize this with the default expected config
	res := &config.Config{
		GlobalConfig: config.GlobalConfig{
			ExternalLabels: labels.Labels{},
		},
		RemoteWriteConfigs: []*config.RemoteWriteConfig{
			{
				URL: &commonconfig.URL{
					URL: &url.URL{
						Scheme: "http",
						Host:   "0.0.0.0:11111",
						Path:   `/api/v1/write`,
					},
				},
				RemoteTimeout:       model.Duration(30 * time.Second),
				WriteRelabelConfigs: []*relabel.Config{},
				SendExemplars:       true,
				HTTPClientConfig: commonconfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     false,
				},
				QueueConfig: config.QueueConfig{
					Capacity:          10000,
					MaxShards:         50,
					MinShards:         1,
					MaxSamplesPerSend: 2000,
					BatchSendDeadline: model.Duration(5 * time.Second),
					MinBackoff:        model.Duration(30 * time.Millisecond),
					MaxBackoff:        model.Duration(5 * time.Second),
					RetryOnRateLimit:  true,
				},
				MetadataConfig: config.MetadataConfig{
					Send:              true,
					SendInterval:      model.Duration(1 * time.Minute),
					MaxSamplesPerSend: 2000,
				},
			},
		},
	}

	if transform != nil {
		transform(res)
	}
	return res
}

func TestAlloyConfig(t *testing.T) {
	tests := []struct {
		testName    string
		cfg         string
		expectedCfg *config.Config
		errorMsg    string
	}{
		{
			testName: "Endpoint_Defaults",
			cfg: `
			endpoint {
				url = "http://0.0.0.0:11111/api/v1/write"
			}
			`,
			expectedCfg: expectedCfg(func(c *config.Config) {
				c.RemoteWriteConfigs[0].ProtobufMessage = config.RemoteWriteProtoMsgV1
			}),
		},
		{
			testName: "Endpoint_ProtobufMessage_V1",
			cfg: `
			endpoint {
				url = "http://0.0.0.0:11111/api/v1/write"
				protobuf_message = "prometheus.WriteRequest"
			}
			`,
			expectedCfg: expectedCfg(func(c *config.Config) {
				c.RemoteWriteConfigs[0].ProtobufMessage = config.RemoteWriteProtoMsgV1
			}),
		},
		{
			testName: "Endpoint_ProtobufMessage_V2",
			cfg: `
			endpoint {
				url = "http://0.0.0.0:11111/api/v1/write"
				protobuf_message = "io.prometheus.write.v2.Request"
			}
			`,
			expectedCfg: expectedCfg(func(c *config.Config) {
				c.RemoteWriteConfigs[0].ProtobufMessage = config.RemoteWriteProtoMsgV2
			}),
		},
		{
			testName: "Endpoint_ProtobufMessage_Invalid",
			cfg: `
			endpoint {
				url = "http://0.0.0.0:11111/api/v1/write"
				protobuf_message = "invalid.message"
			}
			`,
			errorMsg: "unknown remote write protobuf message invalid.message",
		},
		{
			testName: "RelabelConfig",
			cfg: `
			external_labels = {
				cluster = "local",
			}

			endpoint {
				name           = "test-url"
				url            = "http://0.0.0.0:11111/api/v1/write"
				remote_timeout = "100ms"

				queue_config {
					batch_send_deadline = "100ms"
				}

				write_relabel_config {
					source_labels = ["instance"]
					target_label  = "instance"
					action        = "lowercase"
				}
			}`,
			expectedCfg: expectedCfg(func(c *config.Config) {
				relabelCfg := &relabel.DefaultRelabelConfig
				relabelCfg.SourceLabels = model.LabelNames{"instance"}
				relabelCfg.TargetLabel = "instance"
				relabelCfg.Action = "lowercase"

				c.GlobalConfig.ExternalLabels = labels.FromMap(map[string]string{
					"cluster": "local",
				})
				c.RemoteWriteConfigs[0].Name = "test-url"
				c.RemoteWriteConfigs[0].RemoteTimeout = model.Duration(100 * time.Millisecond)
				c.RemoteWriteConfigs[0].QueueConfig.BatchSendDeadline = model.Duration(100 * time.Millisecond)
				c.RemoteWriteConfigs[0].WriteRelabelConfigs = []*relabel.Config{
					relabelCfg,
				}
				c.RemoteWriteConfigs[0].ProtobufMessage = config.RemoteWriteProtoMsgV1
			}),
		},
		{
			testName: "AzureAD_Defaults",
			cfg: `
			endpoint {
				url  = "http://0.0.0.0:11111/api/v1/write"

				azuread {
					managed_identity {
						client_id = "f47ac10b-58cc-0372-8567-0e02b2c3d479"
					}
				}
			}`,
			expectedCfg: expectedCfg(func(c *config.Config) {
				c.RemoteWriteConfigs[0].AzureADConfig = &azuread.AzureADConfig{
					Cloud: "AzurePublic",
					ManagedIdentity: &azuread.ManagedIdentityConfig{
						ClientID: "f47ac10b-58cc-0372-8567-0e02b2c3d479",
					},
				}
				c.RemoteWriteConfigs[0].ProtobufMessage = config.RemoteWriteProtoMsgV1
			}),
		},
		{
			testName: "AzureAD_Explicit",
			cfg: `
			endpoint {
				url  = "http://0.0.0.0:11111/api/v1/write"

				azuread {
					cloud = "AzureChina"
					managed_identity {
						client_id = "f47ac10b-58cc-0372-8567-0e02b2c3d479"
					}
				}
			}`,
			expectedCfg: expectedCfg(func(c *config.Config) {
				c.RemoteWriteConfigs[0].AzureADConfig = &azuread.AzureADConfig{
					Cloud: "AzureChina",
					ManagedIdentity: &azuread.ManagedIdentityConfig{
						ClientID: "f47ac10b-58cc-0372-8567-0e02b2c3d479",
					},
				}
				c.RemoteWriteConfigs[0].ProtobufMessage = config.RemoteWriteProtoMsgV1
			}),
		},
		{
			testName: "SigV4_Defaults",
			cfg: `
			endpoint {
				url  = "http://0.0.0.0:11111/api/v1/write"

				sigv4 {}
			}`,
			expectedCfg: expectedCfg(func(c *config.Config) {
				c.RemoteWriteConfigs[0].SigV4Config = &sigv4.SigV4Config{}
				c.RemoteWriteConfigs[0].ProtobufMessage = config.RemoteWriteProtoMsgV1
			}),
		},
		{
			testName: "SigV4_Explicit",
			cfg: `
			endpoint {
				url  = "http://0.0.0.0:11111/api/v1/write"

				sigv4 {
					region     = "us-east-1"
					access_key = "example_access_key"
					secret_key = "example_secret_key"
					profile    = "example_profile"
					role_arn   = "example_role_arn"
				}
			}`,
			expectedCfg: expectedCfg(func(c *config.Config) {
				c.RemoteWriteConfigs[0].SigV4Config = &sigv4.SigV4Config{
					Region:    "us-east-1",
					AccessKey: "example_access_key",
					SecretKey: commonconfig.Secret("example_secret_key"),
					Profile:   "example_profile",
					RoleARN:   "example_role_arn",
				}
				c.RemoteWriteConfigs[0].ProtobufMessage = config.RemoteWriteProtoMsgV1
			}),
		},
		{
			testName: "TooManyAuth1",
			cfg: `
			endpoint {
				url  = "http://0.0.0.0:11111/api/v1/write"

				sigv4 {}
				bearer_token = "token"
			}`,
			errorMsg: "at most one of sigv4, azuread, basic_auth, oauth2, bearer_token & bearer_token_file must be configured",
		},
		{
			testName: "TooManyAuth2",
			cfg: `
			endpoint {
				url  = "http://0.0.0.0:11111/api/v1/write"

				sigv4 {}
				azuread {
					managed_identity {
						client_id = "00000000-0000-0000-0000-000000000000"
					}
				}
			}`,
			errorMsg: "at most one of sigv4, azuread, basic_auth, oauth2, bearer_token & bearer_token_file must be configured",
		},
		{
			testName: "BadAzureClientId",
			cfg: `
			endpoint {
				url  = "http://0.0.0.0:11111/api/v1/write"

				azuread {
					managed_identity {
						client_id = "bad_client_id"
					}
				}
			}`,
			errorMsg: "the provided Azure Managed Identity client_id is invalid",
		},
		{
			// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
			testName: "BadBearerConfig",
			cfg: `
			external_labels = {
				cluster = "local",
			}

			endpoint {
				name           = "test-url"
				url            = "http://0.0.0.0:11111/api/v1/write"
				remote_timeout = "100ms"
				bearer_token = "token"
				bearer_token_file = "/path/to/file.token"

				queue_config {
					batch_send_deadline = "100ms"
				}
			}`,
			errorMsg: "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)

			if tc.errorMsg != "" {
				require.ErrorContains(t, err, tc.errorMsg)
				return
			}
			require.NoError(t, err)

			promCfg, err := convertConfigs(args)
			require.NoError(t, err)

			require.Equal(t, tc.expectedCfg, promCfg)
		})
	}
}
