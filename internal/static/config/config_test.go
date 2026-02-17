package config

import (
	"flag"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	commonCfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promCfg "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/static/config/encoder"
	"github.com/grafana/alloy/internal/static/metrics/instance"
	"github.com/grafana/alloy/internal/util"
)

// TestConfig_FlagDefaults makes sure that default values of flags are kept
// when parsing the config.
func TestConfig_FlagDefaults(t *testing.T) {
	cfg := `
metrics:
  wal_directory: /tmp/wal
  global:
    scrape_timeout: 33s`

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), false, c)
	})
	require.NoError(t, err)
	require.NotEmpty(t, c.Metrics.ServiceConfig.Lifecycler.InfNames)
	require.NotZero(t, c.Metrics.ServiceConfig.Lifecycler.NumTokens)
	require.NotZero(t, c.Metrics.ServiceConfig.Lifecycler.HeartbeatPeriod)
	require.True(t, c.ServerFlags.RegisterInstrumentation)
}

// TestConfig_ConfigAPIFlag makes sure that the read API flag is passed
// when parsing the config.
func TestConfig_ConfigAPIFlag(t *testing.T) {
	t.Run("Disabled", func(t *testing.T) {
		cfg := `{}`
		fs := flag.NewFlagSet("test", flag.ExitOnError)
		c, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
			return LoadBytes([]byte(cfg), false, c)
		})
		require.NoError(t, err)
		require.False(t, c.EnableConfigEndpoints)
		require.False(t, c.Metrics.ServiceConfig.APIEnableGetConfiguration)
	})
	t.Run("Enabled", func(t *testing.T) {
		cfg := `{}`
		fs := flag.NewFlagSet("test", flag.ExitOnError)
		c, err := LoadFromFunc(fs, []string{"-config.file", "test", "-config.enable-read-api"}, func(_, _ string, _ bool, c *Config) error {
			return LoadBytes([]byte(cfg), false, c)
		})
		require.NoError(t, err)
		require.True(t, c.EnableConfigEndpoints)
		require.True(t, c.Metrics.ServiceConfig.APIEnableGetConfiguration)
	})
}

func TestConfig_OverrideDefaultsOnLoad(t *testing.T) {
	cfg := `
metrics:
  wal_directory: /tmp/wal
  global:
    scrape_timeout: 33s`
	falseVal := false
	expect := instance.GlobalConfig{
		Prometheus: promCfg.GlobalConfig{
			ScrapeInterval:             model.Duration(1 * time.Minute),
			ScrapeTimeout:              model.Duration(33 * time.Second),
			ScrapeProtocols:            promCfg.DefaultScrapeProtocols,
			EvaluationInterval:         model.Duration(1 * time.Minute),
			MetricNameValidationScheme: model.UTF8Validation,
			MetricNameEscapingScheme:   model.AllowUTF8,
			ScrapeNativeHistograms:     &falseVal,
		},
	}

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), false, c)
	})
	require.NoError(t, err)
	require.Equal(t, expect, c.Metrics.Global)
}

func TestConfig_OverrideByEnvironmentOnLoad(t *testing.T) {
	cfg := `
metrics:
  wal_directory: /tmp/wal
  global:
    scrape_timeout: ${SCRAPE_TIMEOUT}`
	falseVal := false
	expect := instance.GlobalConfig{
		Prometheus: promCfg.GlobalConfig{
			ScrapeInterval:             model.Duration(1 * time.Minute),
			ScrapeTimeout:              model.Duration(33 * time.Second),
			ScrapeProtocols:            promCfg.DefaultScrapeProtocols,
			EvaluationInterval:         model.Duration(1 * time.Minute),
			MetricNameValidationScheme: model.UTF8Validation,
			MetricNameEscapingScheme:   model.AllowUTF8,
			ScrapeNativeHistograms:     &falseVal,
		},
	}
	t.Setenv("SCRAPE_TIMEOUT", "33s")

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), true, c)
	})
	require.NoError(t, err)
	require.Equal(t, expect, c.Metrics.Global)
}

func TestConfig_OverrideByEnvironmentOnLoad_NoDigits(t *testing.T) {
	cfg := `
metrics:
  wal_directory: /tmp/wal
  global:
    external_labels:
      foo: ${1}`
	expect := labels.FromStrings("foo", "${1}")

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), true, c)
	})
	require.NoError(t, err)
	require.Equal(t, expect, c.Metrics.Global.Prometheus.ExternalLabels)
}

func TestConfig_FlagsAreAccepted(t *testing.T) {
	cfg := `
metrics:
  global:
    scrape_timeout: 33s`

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	args := []string{
		"-config.file", "test",
		"-metrics.wal-directory", "/tmp/wal",
		"-config.expand-env",
	}

	c, err := LoadFromFunc(fs, args, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), false, c)
	})
	require.NoError(t, err)
	require.Equal(t, "/tmp/wal", c.Metrics.WALDir)
}

func TestConfig_StrictYamlParsing(t *testing.T) {
	t.Run("duplicate key", func(t *testing.T) {
		cfg := `
metrics:
  wal_directory: /tmp/wal
  global:
    scrape_timeout: 10s
    scrape_timeout: 15s`
		var c Config
		err := LoadBytes([]byte(cfg), false, &c)
		require.Error(t, err)
	})

	t.Run("non existing key", func(t *testing.T) {
		cfg := `
metrics:
  wal_directory: /tmp/wal
  global:
  scrape_timeout: 10s`
		var c Config
		err := LoadBytes([]byte(cfg), false, &c)
		require.Error(t, err)
	})
}

func TestConfig_Defaults(t *testing.T) {
	var c Config
	err := LoadBytes([]byte(`{}`), false, &c)
	require.NoError(t, err)

	defaultConfig := DefaultConfig()
	util.DefaultConfigFromFlags(&defaultConfig)

	assert.Equal(t, defaultConfig.Metrics, c.Metrics)
	assert.Equal(t, DefaultVersionedIntegrations(), c.Integrations)
}

func TestConfig_TracesLokiValidates(t *testing.T) {
	tests := []struct {
		cfg string
	}{
		{
			cfg: `
loki:
  configs:
  - name: default
    positions:
      filename: /tmp/positions.yaml
    clients:
    - url: http://loki:3100/loki/api/v1/push
traces:
  configs:
  - name: default
    automatic_logging:
      backend: loki
      loki_name: default
      spans: true`,
		},
		{
			cfg: `
loki:
  configs:
  - name: default
    positions:
      filename: /tmp/positions.yaml
    clients:
    - url: http://loki:3100/loki/api/v1/push
traces:
  configs:
  - name: default
    automatic_logging:
      backend: stdout
      loki_name: doesnt_exist
      spans: true`,
		},
	}

	for _, tc := range tests {
		fs := flag.NewFlagSet("test", flag.ExitOnError)
		_, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
			return LoadBytes([]byte(tc.cfg), false, c)
		})

		require.NoError(t, err)
	}
}

func TestConfig_LokiNameMigration(t *testing.T) {
	input := util.Untab(`
loki:
  configs:
  - name: foo
    positions:
      filename: /tmp/positions.yaml
    clients:
    - url: http://loki:3100/loki/api/v1/push
`)
	var cfg Config
	require.NoError(t, LoadBytes([]byte(input), false, &cfg))
	require.NoError(t, cfg.Validate(nil))

	require.NotNil(t, cfg.Logs)
	require.Equal(t, "foo", cfg.Logs.Configs[0].Name)
	require.Equal(t, []string{"`loki` has been deprecated in favor of `logs`"}, cfg.Deprecations)
}

func TestConfig_PrometheusNonNil(t *testing.T) {
	tt := []struct {
		name  string
		input string
	}{
		{
			name:  "missing",
			input: `{}`,
		},
		{
			name:  "null",
			input: `metrics: null`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			require.NoError(t, LoadBytes([]byte(tc.input), false, &cfg))
			require.NoError(t, cfg.Validate(nil))

			require.NotNil(t, cfg.Metrics)
		})
	}
}

func TestConfig_PrometheusNameMigration(t *testing.T) {
	input := util.Untab(`
prometheus:
	wal_directory: /tmp
  configs:
  - name: default
`)
	var cfg Config
	require.NoError(t, LoadBytes([]byte(input), false, &cfg))
	require.NoError(t, cfg.Validate(nil))

	require.Equal(t, "default", cfg.Metrics.Configs[0].Name)
	require.Equal(t, "/tmp", cfg.Metrics.WALDir)
	require.Equal(t, []string{"`prometheus` has been deprecated in favor of `metrics`"}, cfg.Deprecations)
}

func TestConfig_TracesLokiFailsValidation(t *testing.T) {
	tests := []struct {
		cfg           string
		expectedError string
	}{
		{
			cfg: `
loki:
  configs:
  - name: foo
    positions:
      filename: /tmp/positions.yaml
    clients:
    - url: http://loki:3100/loki/api/v1/push
traces:
  configs:
  - name: default
    automatic_logging:
      backend: logs_instance
      logs_instance_name: default
      spans: true`,
			expectedError: "error in config file: failed to validate automatic_logging for traces config default: specified logs config default not found in agent config",
		},
	}

	for _, tc := range tests {
		fs := flag.NewFlagSet("test", flag.ExitOnError)
		_, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
			return LoadBytes([]byte(tc.cfg), false, c)
		})

		require.EqualError(t, err, tc.expectedError)
	}
}

func TestConfig_TempoNameMigration(t *testing.T) {
	input := util.Untab(`
tempo:
  configs:
  - name: default
    automatic_logging:
      backend: stdout
      loki_name: doesnt_exist
      spans: true`)
	var cfg Config
	require.NoError(t, LoadBytes([]byte(input), false, &cfg))
	require.NoError(t, cfg.Validate(nil))

	require.NotNil(t, cfg.Traces)

	require.Equal(t, "default", cfg.Traces.Configs[0].Name)
	require.Equal(t, []string{"`tempo` has been deprecated in favor of `traces`"}, cfg.Deprecations)
}

func TestConfig_TempoTracesDuplicateMigration(t *testing.T) {
	input := util.Untab(`
traces:
  configs:
  - name: default
    automatic_logging:
      backend: stdout
      loki_name: doesnt_exist
      spans: true
tempo:
  configs:
  - name: default
    automatic_logging:
      backend: stdout
      loki_name: doesnt_exist
      spans: true`)
	var cfg Config
	require.EqualError(t, LoadBytes([]byte(input), false, &cfg), "at most one of tempo and traces should be specified")
}

func TestConfig_ExpandEnvRegex(t *testing.T) {
	cfg := `
logs:
  configs:
  - name: default
    positions:
      filename: /tmp/positions.yaml
    scrape_configs:
      - job_name: test
        pipeline_stages:
        - regex:
          source: filename
          expression: '\\temp\\Logs\\(?P<log_app>.+?)\\'`
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	myCfg, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), true, c)
	})
	require.NoError(t, err)
	pipelineStages := myCfg.Logs.Configs[0].ScrapeConfig[0].PipelineStages[0].(map[any]any)
	expected := `\\temp\\Logs\\(?P<log_app>.+?)\\`
	require.Equal(t, expected, pipelineStages["expression"].(string))
}

func TestConfig_ObscureSecrets(t *testing.T) {
	cfgText := `
metrics:
  wal_directory: /tmp
  scraping_service:
    enabled: true
    kvstore:
      store: consul
      consul:
        acl_token: verysecret
    lifecycler:
      ring:
        kvstore:
          store: consul
          consul:
            acl_token: verysecret
`

	var cfg Config
	require.NoError(t, LoadBytes([]byte(cfgText), false, &cfg))

	require.Equal(t, "verysecret", cfg.Metrics.ServiceConfig.KVStore.Consul.ACLToken.String())
	require.Equal(t, "verysecret", cfg.Metrics.ServiceConfig.Lifecycler.RingConfig.KVStore.Consul.ACLToken.String())

	bb, err := yaml.Marshal(&cfg)
	require.NoError(t, err)

	require.False(t, strings.Contains(string(bb), "verysecret"), "secrets did not get obscured")
	require.True(t, strings.Contains(string(bb), "********"), "secrets did not get obscured properly")

	// Re-validate that the config object has not changed
	require.Equal(t, "verysecret", cfg.Metrics.ServiceConfig.KVStore.Consul.ACLToken.String())
	require.Equal(t, "verysecret", cfg.Metrics.ServiceConfig.Lifecycler.RingConfig.KVStore.Consul.ACLToken.String())
}

func TestConfig_RemoteWriteDefaults(t *testing.T) {
	cfg := `
metrics:
  global:
    remote_write:
      - name: "foo"
        url: "https://test/url"`

	var c Config
	err := LoadBytes([]byte(cfg), false, &c)
	require.NoError(t, err)

	expected := &promCfg.DefaultRemoteWriteConfig
	expected.Name = "foo"
	testURL, _ := url.Parse("https://test/url")
	expected.URL = &commonCfg.URL{
		URL: testURL,
	}
	require.Equal(t, expected, c.Metrics.Global.RemoteWrite[0])
	require.True(t, c.Metrics.Global.RemoteWrite[0].SendExemplars)
}

func TestAgent_OmitEmptyFields(t *testing.T) {
	var cfg Config
	yml, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	require.Equal(t, "{}\n", string(yml))
}

func TestConfig_EmptyServerConfigFails(t *testing.T) {
	// Since we are testing defaults via config.Load, we need a file instead of a string.
	// This test file has an empty server stanza, we expect default values out.
	fs := flag.NewFlagSet("", flag.ExitOnError)

	_, err := LoadFromFunc(fs, []string{"--config.file", "./testdata/server_empty.yml"}, func(path, fileType string, expandEnvVars bool, target *Config) error {
		bb, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return LoadBytes(bb, expandEnvVars, target)
	})
	require.Error(t, err)
}

func TestConfig_ValidateConfigWithIntegrationV1(t *testing.T) {
	input := util.Untab(`
integrations:
	agent:
		enabled: true
`)
	var cfg Config
	require.NoError(t, LoadBytes([]byte(input), false, &cfg))
	require.NoError(t, cfg.Validate(nil))
}

func TestConfig_ValidateConfigWithIntegrationV2(t *testing.T) {
	input := util.Untab(`
integrations:
	agent:
		autoscrape:
			enabled: true
`)
	var cfg Config
	require.NoError(t, LoadBytes([]byte(input), false, &cfg))
	require.NoError(t, cfg.Validate(nil))
}

func TestConfigEncoding(t *testing.T) {
	type testCase struct {
		filename string
		success  bool
	}
	cases := []testCase{
		{filename: "test_encoding_unknown.txt", success: false},
		{filename: "test_encoding_utf8.txt", success: true},
		{filename: "test_encoding_utf8bom.txt", success: true},
		{filename: "test_encoding_utf16le.txt", success: true},
		{filename: "test_encoding_utf16be.txt", success: true},
		{filename: "test_encoding_utf32be.txt", success: true},
		{filename: "test_encoding_utf32le.txt", success: true},
	}
	for _, tt := range cases {
		t.Run(tt.filename, func(t *testing.T) {
			buf, err := os.ReadFile(path.Join("encoder", tt.filename))
			t.Setenv("TEST", "debug")
			require.NoError(t, err)
			c := &Config{}
			err = LoadBytes(buf, true, c)
			if tt.success {
				require.NoError(t, err)
				require.True(t, c.Server.LogLevel.String() == "debug")
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestConfigEncodingStrict(t *testing.T) {
	buf, err := os.ReadFile(path.Join("encoder", "test_encoding_utf16le.txt"))
	require.NoError(t, err)
	_, err = encoder.EnsureUTF8(buf, false)
	require.NoError(t, err)
	_, err = encoder.EnsureUTF8(buf, true)
	require.Error(t, err)
}
