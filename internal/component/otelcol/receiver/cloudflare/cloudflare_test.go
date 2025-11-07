package cloudflare_test

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/cloudflare"
	"github.com/grafana/alloy/syntax"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	cases := []struct {
		testName string
		cfg      string
		expected cloudflarereceiver.Config
	}{
		{
			testName: "minimal configuration",
			cfg: `
				endpoint = "localhost:8080/webhook"
				output {}
			`,
			expected: cloudflarereceiver.Config{
				Logs: cloudflarereceiver.LogsConfig{
					Endpoint: "localhost:8080/webhook",
				},
			},
		},
		{
			testName: "full configuration without TLS",
			cfg: `
				secret = "my-secret"
				endpoint = "localhost:8080/cloudflare-webhook"
				attributes = {
					"service.name" = "cloudflare-logs",
					"environment" = "production",
				}
				timestamp_field = "EdgeStartTimestamp"
				timestamp_format = "unix"
				separator = "_"
				output {}
			`,
			expected: cloudflarereceiver.Config{
				Logs: cloudflarereceiver.LogsConfig{
					Secret:   "my-secret",
					Endpoint: "localhost:8080/cloudflare-webhook",
					Attributes: map[string]string{
						"service.name": "cloudflare-logs",
						"environment":  "production",
					},
					TimestampField:  "EdgeStartTimestamp",
					TimestampFormat: "unix",
					Separator:       "_",
				},
			},
		},
		{
			testName: "configuration with TLS",
			cfg: `
				secret = "my-secret"
				endpoint = "localhost:8443/secure-webhook"
				tls {
					cert_file = "/path/to/cert.pem"
					key_file = "/path/to/key.pem"
				}
				timestamp_format = "unixnano"
				output {}
			`,
			expected: cloudflarereceiver.Config{
				Logs: cloudflarereceiver.LogsConfig{
					Secret:   "my-secret",
					Endpoint: "localhost:8443/secure-webhook",
					TLS: &configtls.ServerConfig{
						Config: configtls.Config{
							CertFile: "/path/to/cert.pem",
							KeyFile:  "/path/to/key.pem",
						},
					},
					TimestampFormat: "unixnano",
				},
			},
		},
		{
			testName: "configuration with custom timestamp field",
			cfg: `
				secret = "my-secret"
				endpoint = "localhost:8080/webhook"
				timestamp_field = "RequestTimestamp"
				timestamp_format = "rfc3339"
				output {}
			`,
			expected: cloudflarereceiver.Config{
				Logs: cloudflarereceiver.LogsConfig{
					Secret:          "my-secret",
					Endpoint:        "localhost:8080/webhook",
					TimestampField:  "RequestTimestamp",
					TimestampFormat: "rfc3339",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.testName, func(t *testing.T) {
			var args cloudflare.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*cloudflarereceiver.Config)
			require.Equal(t, tc.expected, *actual)
		})
	}
}

func TestArguments_Validate(t *testing.T) {
	cases := []struct {
		testName      string
		cfg           string
		expectedError string
	}{
		{
			testName: "invalid timestamp format",
			cfg: `
				secret = "my-secret"
				endpoint = "localhost:8080/webhook"
				timestamp_format = "invalid"
				output {}
			`,
			expectedError: `invalid timestamp_format "invalid"`,
		},
		{
			testName: "TLS with missing cert file",
			cfg: `
				secret = "my-secret"
				endpoint = "localhost:8080/webhook"
				tls {
					key_file = "/path/to/key.pem"
				}
				output {}
			`,
			expectedError: "tls was configured, but no cert file was specified",
		},
		{
			testName: "TLS with missing key file",
			cfg: `
				secret = "my-secret"
				endpoint = "localhost:8080/webhook"
				tls {
					cert_file = "/path/to/cert.pem"
				}
				output {}
			`,
			expectedError: "tls was configured, but no key file was specified",
		},
		{
			testName: "missing output",
			cfg: `
				endpoint = "localhost:8080/webhook"
			`,
			expectedError: `missing required block "output"`,
		},
		{
			testName: "missing endpoint",
			cfg: `
				secret = "my-secret"
				output {}
			`,
			expectedError: `missing required attribute "endpoint"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.testName, func(t *testing.T) {
			var args cloudflare.Arguments
			require.ErrorContains(t, syntax.Unmarshal([]byte(tc.cfg), &args), tc.expectedError)
		})
	}
}
