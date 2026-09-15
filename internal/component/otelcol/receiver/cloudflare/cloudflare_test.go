package cloudflare_test

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/cloudflare"
	"github.com/grafana/alloy/syntax"
)

// expectedConfig returns the upstream factory defaults with override applied,
// so each case only spells out what it actually overrides.
func expectedConfig(override func(logs *cloudflarereceiver.LogsConfig)) cloudflarereceiver.Config {
	cfg := cloudflarereceiver.NewFactory().CreateDefaultConfig().(*cloudflarereceiver.Config)
	override(&cfg.Logs)
	return *cfg
}

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
			expected: expectedConfig(func(logs *cloudflarereceiver.LogsConfig) {
				logs.Endpoint = "localhost:8080/webhook"
			}),
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
			expected: expectedConfig(func(logs *cloudflarereceiver.LogsConfig) {
				logs.Secret = "my-secret"
				logs.Endpoint = "localhost:8080/cloudflare-webhook"
				logs.Attributes = map[string]string{
					"service.name": "cloudflare-logs",
					"environment":  "production",
				}
				logs.TimestampField = "EdgeStartTimestamp"
				logs.TimestampFormat = "unix"
				logs.Separator = "_"
			}),
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
			expected: expectedConfig(func(logs *cloudflarereceiver.LogsConfig) {
				logs.Secret = "my-secret"
				logs.Endpoint = "localhost:8443/secure-webhook"
				logs.TLS = &configtls.ServerConfig{
					Config: configtls.Config{
						CertFile: "/path/to/cert.pem",
						KeyFile:  "/path/to/key.pem",
					},
				}
				logs.TimestampFormat = "unixnano"
			}),
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
			expected: expectedConfig(func(logs *cloudflarereceiver.LogsConfig) {
				logs.Secret = "my-secret"
				logs.Endpoint = "localhost:8080/webhook"
				logs.TimestampField = "RequestTimestamp"
				logs.TimestampFormat = "rfc3339"
			}),
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
