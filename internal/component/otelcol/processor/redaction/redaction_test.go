package redaction_test

import (
	"testing"

	"github.com/go-viper/mapstructure/v2"
	"github.com/grafana/alloy/internal/component/otelcol/processor/redaction"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected map[string]any
		errMsg   string
	}{
		{
			testName: "Default",
			cfg: `
			output {}
			`,
			expected: map[string]any{},
		},
		{
			testName: "FullTopLevel",
			cfg: `
			allow_all_keys       = true
			allowed_keys         = ["description", "group"]
			ignored_keys         = ["safe_attribute"]
			blocked_key_patterns = [".*token.*"]
			ignored_key_patterns = [".*safe.*"]
			allowed_values       = ["^https?://.*"]
			blocked_values       = ["4[0-9]{12}"]
			redact_all_types     = true
			summary              = "debug"
			output {}
			`,
			expected: map[string]any{
				"allow_all_keys":       true,
				"allowed_keys":         []string{"description", "group"},
				"ignored_keys":         []string{"safe_attribute"},
				"blocked_key_patterns": []string{".*token.*"},
				"ignored_key_patterns": []string{".*safe.*"},
				"allowed_values":       []string{"^https?://.*"},
				"blocked_values":       []string{"4[0-9]{12}"},
				"redact_all_types":     true,
				"summary":              "debug",
			},
		},
		{
			testName: "URLSanitizer",
			cfg: `
			url_sanitizer {
				enabled            = true
				attributes         = ["http.url", "url.full"]
				sanitize_span_name = true
			}
			output {}
			`,
			expected: map[string]any{
				"url_sanitizer": map[string]any{
					"enabled":            true,
					"attributes":         []string{"http.url", "url.full"},
					"sanitize_span_name": true,
				},
			},
		},
		{
			testName: "DBSanitizer",
			cfg: `
			db_sanitizer {
				sanitize_span_name = true
				sql {
					enabled    = true
					attributes = ["db.statement"]
				}
				redis {
					enabled = true
				}
			}
			output {}
			`,
			expected: map[string]any{
				"db_sanitizer": map[string]any{
					"sanitize_span_name": true,
					"sql": map[string]any{
						"enabled":    true,
						"attributes": []string{"db.statement"},
					},
					"redis": map[string]any{
						"enabled": true,
					},
				},
			},
		},
		{
			testName: "DBSanitizerAllTechs",
			cfg: `
			db_sanitizer {
				sanitize_span_name = false
				sql {
					enabled    = true
					attributes = ["db.statement"]
				}
				redis {
					enabled = true
				}
				valkey {
					enabled = true
				}
				memcached {
					enabled = true
				}
				mongo {
					enabled = true
				}
				opensearch {
					enabled = true
				}
				es {
					enabled = true
				}
			}
			output {}
			`,
			expected: map[string]any{
				"db_sanitizer": map[string]any{
					"sanitize_span_name": false,
					"sql": map[string]any{
						"enabled":    true,
						"attributes": []string{"db.statement"},
					},
					"redis":      map[string]any{"enabled": true},
					"valkey":     map[string]any{"enabled": true},
					"memcached":  map[string]any{"enabled": true},
					"mongo":      map[string]any{"enabled": true},
					"opensearch": map[string]any{"enabled": true},
					"es":         map[string]any{"enabled": true},
				},
			},
		},
		{
			testName: "ValidHMAC",
			cfg: `
			hash_function = "hmac-sha256"
			hmac_key      = "0123456789abcdef0123456789abcdef"
			output {}
			`,
			expected: map[string]any{
				"hash_function": "hmac-sha256",
				"hmac_key":      "0123456789abcdef0123456789abcdef",
			},
		},
		{
			testName: "InvalidHashFunction",
			cfg: `
			hash_function = "sha256"
			output {}
			`,
			errMsg: `invalid hash_function "sha256"`,
		},
		{
			testName: "HMACMissingKey",
			cfg: `
			hash_function = "hmac-sha256"
			output {}
			`,
			errMsg: "hmac_key must not be empty",
		},
		{
			testName: "HMACShortKey",
			cfg: `
			hash_function = "hmac-sha256"
			hmac_key      = "tooshort"
			output {}
			`,
			errMsg: "hmac_key must be at least 32 bytes long",
		},
		{
			testName: "HMACSHA512ShortKey",
			cfg: `
			hash_function = "hmac-sha512"
			hmac_key      = "0123456789abcdef0123456789abcdef"
			output {}
			`,
			errMsg: "hmac_key must be at least 64 bytes long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			var args redaction.Arguments
			err := syntax.Unmarshal([]byte(tt.cfg), &args)
			if tt.errMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)
			actual := actualPtr.(*redactionprocessor.Config)

			var expectedCfg redactionprocessor.Config
			require.NoError(t, mapstructure.Decode(tt.expected, &expectedCfg))

			require.Equal(t, expectedCfg, *actual)
		})
	}
}

func TestDefaultArguments(t *testing.T) {
	var args redaction.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	require.Equal(t, &redactionprocessor.Config{}, cfg.(*redactionprocessor.Config))
}
