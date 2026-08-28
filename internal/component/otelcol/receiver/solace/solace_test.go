package solace_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/solace"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected solacereceiver.Config
	}{
		{
			testName: "Defaults",
			cfg: `
				queue = "queue://#telemetry_testprofile"
				auth {
					sasl_plain {
						username = "alloy"
						password = "password"
					}
				}
				output {}
			`,
			expected: solacereceiver.Config{
				Broker:     []string{"localhost:5671"},
				Queue:      "queue://#telemetry_testprofile",
				MaxUnacked: 1000,
				Flow: solacereceiver.FlowControl{
					DelayedRetry: configoptional.Some(solacereceiver.FlowControlDelayedRetry{
						Delay: 10 * time.Millisecond,
					}),
				},
				Auth: solacereceiver.Authentication{
					PlainText: configoptional.Some(solacereceiver.SaslPlainTextConfig{
						Username: "alloy",
						Password: "password",
					}),
				},
			},
		},
		{
			testName: "Explicit Values - External / TLS",
			cfg: `
				broker = "localhost:5672"
				max_unacknowledged = 500
				queue = "queue://#telemetry_testprofile"
				auth {
					sasl_external {}
				}
				tls {
					cert_file = "testdata/test-cert.crt"
					key_file = "testdata/test-key.key"
				}
				flow_control {
					delayed_retry {
						delay = "50ms"
					}
				}
				output {}
			`,
			expected: solacereceiver.Config{
				Broker:     []string{"localhost:5672"},
				Queue:      "queue://#telemetry_testprofile",
				MaxUnacked: 500,
				Flow: solacereceiver.FlowControl{
					DelayedRetry: configoptional.Some(solacereceiver.FlowControlDelayedRetry{
						Delay: 50 * time.Millisecond,
					}),
				},
				Auth: solacereceiver.Authentication{
					External: configoptional.Some(solacereceiver.SaslExternalConfig{}),
				},
				TLS: configtls.ClientConfig{
					Config: configtls.Config{
						CertFile: "testdata/test-cert.crt",
						KeyFile:  "testdata/test-key.key",
					},
				},
			},
		},
		{
			testName: "Explicit Values - XAuth2 / TLS",
			cfg: `
				broker = "localhost:5672"
				max_unacknowledged = 500
				queue = "queue://#telemetry_testprofile"
				auth {
					sasl_xauth2 {
						username = "alloy"
						bearer = "bearer"
					}
				}
				tls {
					cert_file = "testdata/test-cert.crt"
					key_file = "testdata/test-key.key"
				}
				flow_control {
					delayed_retry {
						delay = "50ms"
					}
				}
				output {}
			`,
			expected: solacereceiver.Config{
				Broker:     []string{"localhost:5672"},
				Queue:      "queue://#telemetry_testprofile",
				MaxUnacked: 500,
				Flow: solacereceiver.FlowControl{
					DelayedRetry: configoptional.Some(solacereceiver.FlowControlDelayedRetry{
						Delay: 50 * time.Millisecond,
					}),
				},
				Auth: solacereceiver.Authentication{
					XAuth2: configoptional.Some(solacereceiver.SaslXAuth2Config{
						Username: "alloy",
						Bearer:   "bearer",
					}),
				},
				TLS: configtls.ClientConfig{
					Config: configtls.Config{
						CertFile: "testdata/test-cert.crt",
						KeyFile:  "testdata/test-key.key",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args solace.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*solacereceiver.Config)

			require.Equal(t, tc.expected, *actual)
		})
	}
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		testName      string
		cfg           string
		expectedError string
	}{
		{
			testName: "Missing Auth",
			cfg: `
				queue = "queue://#telemetry_testprofile"
				auth {}
				output {}
			`,
			expectedError: "the auth block must contain exactly one of sasl_plain block, sasl_xauth2 block or sasl_external block",
		},
		{
			testName: "Multiple Auth",
			cfg: `
				queue = "queue://#telemetry_testprofile"
				auth {
					sasl_plain {
						username = "alloy"
						password = "password"
					}
					sasl_xauth2 {
						username = "alloy"
						bearer = "bearer"
					}
				}
				output {}
			`,
			expectedError: "the auth block must contain exactly one of sasl_plain block, sasl_xauth2 block or sasl_external block",
		},
		{
			testName: "Empty Queue",
			cfg: `
				queue = ""
				auth {
					sasl_plain {
						username = "alloy"
						password = "password"
					}
				}
				output {}
			`,
			expectedError: "queue must not be empty, queue definition has format queue://<queuename>",
		},
		{
			testName: "Wrong value for delay in delayed_retry block",
			cfg: `
				queue = "queue://#telemetry_testprofile"
				auth {
					sasl_plain {
						username = "alloy"
						password = "password"
					}
				}
				flow_control {
					delayed_retry {
						delay = "0ms"
					}
				}
				output {}
			`,
			expectedError: "the delay attribute in the delayed_retry block must be > 0, got 0",
		},
	}
	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args solace.Arguments
			require.ErrorContains(t, syntax.Unmarshal([]byte(tc.cfg), &args), tc.expectedError)
		})
	}
}
