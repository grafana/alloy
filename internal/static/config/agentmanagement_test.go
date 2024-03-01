package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"testing"
	"time"

	"github.com/grafana/agent/internal/static/config/features"
	"github.com/grafana/agent/internal/static/server"
	"github.com/grafana/agent/internal/util"
	"github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// testRemoteConfigProvider is an implementation of remoteConfigProvider that can be
// used for testing. It allows setting the values to return for both fetching the
// remote config bytes & errors as well as the cached config & errors.
type testRemoteConfigProvider struct {
	InitialConfig *AgentManagementConfig

	fetchedConfigBytesToReturn []byte
	fetchedConfigErrorToReturn error
	fetchRemoteConfigCallCount int

	cachedConfigToReturn      []byte
	cachedConfigErrorToReturn error
	getCachedConfigCallCount  int
	didCacheRemoteConfig      bool
}

func (t *testRemoteConfigProvider) GetCachedRemoteConfig() ([]byte, error) {
	t.getCachedConfigCallCount += 1
	return t.cachedConfigToReturn, t.cachedConfigErrorToReturn
}

func (t *testRemoteConfigProvider) FetchRemoteConfig() ([]byte, error) {
	t.fetchRemoteConfigCallCount += 1
	return t.fetchedConfigBytesToReturn, t.fetchedConfigErrorToReturn
}

func (t *testRemoteConfigProvider) CacheRemoteConfig(r []byte) error {
	t.didCacheRemoteConfig = true
	return nil
}

func (t *testRemoteConfigProvider) GetPollingInterval() time.Duration {
	return t.InitialConfig.PollingInterval
}

var validAgentManagementConfig = AgentManagementConfig{
	Enabled: true,
	Host:    "localhost:1234",
	HTTPClientConfig: config.HTTPClientConfig{
		BasicAuth: &config.BasicAuth{
			Username:     "test",
			PasswordFile: "/test/path",
		},
	},

	Protocol:        "https",
	PollingInterval: time.Minute,
	RemoteConfiguration: RemoteConfiguration{
		Labels:        labelMap{"b": "B", "a": "A"},
		Namespace:     "test_namespace",
		CacheLocation: "/test/path/",
	},
}

var cachedConfig = []byte(`{"base_config":"","snippets":[]}`)

func TestUnmarshalDefault(t *testing.T) {
	cfg := `host: "localhost:1234"
protocol: "https"
polling_interval: "1m"
remote_configuration:
  namespace: "test_namespace"`
	var am AgentManagementConfig
	err := yaml.Unmarshal([]byte(cfg), &am)
	assert.NoError(t, err)
	assert.True(t, am.RemoteConfiguration.AcceptHTTPNotModified)
	assert.Equal(t, "https", am.Protocol)
	assert.Equal(t, time.Minute, am.PollingInterval)
	assert.Equal(t, "test_namespace", am.RemoteConfiguration.Namespace)
}

func TestValidateValidConfig(t *testing.T) {
	assert.NoError(t, validAgentManagementConfig.Validate())
}

func TestValidateInvalidBasicAuth(t *testing.T) {
	invalidConfig := &AgentManagementConfig{
		Enabled:          true,
		Host:             "localhost:1234",
		HTTPClientConfig: config.HTTPClientConfig{},
		Protocol:         "https",
		PollingInterval:  time.Minute,
		RemoteConfiguration: RemoteConfiguration{
			Namespace:     "test_namespace",
			CacheLocation: "/test/path/",
		},
	}
	// This should error as the BasicAuth is nil
	assert.Error(t, invalidConfig.Validate())

	// This should error as the BasicAuth is empty
	invalidConfig.HTTPClientConfig.BasicAuth = &config.BasicAuth{}
	assert.Error(t, invalidConfig.Validate())

	invalidConfig.HTTPClientConfig.BasicAuth.Username = "test"
	assert.Error(t, invalidConfig.Validate()) // Should still error as there is no password file set

	invalidConfig.HTTPClientConfig.BasicAuth.Username = ""
	invalidConfig.HTTPClientConfig.BasicAuth.PasswordFile = "/test/path"
	assert.Error(t, invalidConfig.Validate()) // Should still error as there is no username set
}

func TestMissingCacheLocation(t *testing.T) {
	invalidConfig := &AgentManagementConfig{
		Enabled: true,
		Host:    "localhost:1234",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username:     "test",
				PasswordFile: "/test/path",
			},
		},
		Protocol:        "https",
		PollingInterval: 1 * time.Minute,
		RemoteConfiguration: RemoteConfiguration{
			Namespace: "test_namespace",
		},
	}
	assert.Error(t, invalidConfig.Validate())
}

func TestValidateLabelManagement(t *testing.T) {
	cfg := &AgentManagementConfig{
		Enabled: true,
		Host:    "localhost:1234",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username:     "test",
				PasswordFile: "/test/path",
			},
		},
		Protocol:        "https",
		PollingInterval: time.Minute,
		RemoteConfiguration: RemoteConfiguration{
			Namespace:              "test_namespace",
			CacheLocation:          "/test/path/",
			LabelManagementEnabled: true,
		},
	}
	// This should error as there is no agent_id set
	assert.Error(t, cfg.Validate())

	cfg.RemoteConfiguration.AgentID = "test_agent_id"
	assert.NoError(t, cfg.Validate())
}

func TestSleepTime(t *testing.T) {
	cfg := `
api_url: "http://localhost"
basic_auth:
  username: "initial_user"
protocol: "http"
polling_interval: "1m"
remote_configuration:
  namespace: "new_namespace"
  cache_location:  "/etc"`

	var am AgentManagementConfig
	yaml.Unmarshal([]byte(cfg), &am)
	assert.Equal(t, time.Minute, am.SleepTime())
}

func TestFuzzJitterTime(t *testing.T) {
	am := validAgentManagementConfig
	pollingInterval := 2 * time.Minute
	am.PollingInterval = pollingInterval

	zero := time.Duration(0)

	for i := 0; i < 10_000; i++ {
		j := am.JitterTime()
		assert.GreaterOrEqual(t, j, zero)
		assert.Less(t, j, pollingInterval)
	}
}

func TestFullUrl(t *testing.T) {
	c := validAgentManagementConfig
	actual, err := c.fullUrl()
	assert.NoError(t, err)
	assert.Equal(t, "https://localhost:1234/agent-management/api/agent/v2/namespace/test_namespace/remote_config?a=A&b=B", actual)
}

func TestRemoteConfigHashCheck(t *testing.T) {
	// not a truly valid Agent Management config, but used for testing against
	// precomputed sha256 hash
	ic := AgentManagementConfig{
		Protocol: "http",
	}
	marshalled, err := yaml.Marshal(ic)
	require.NoError(t, err)
	icHashBytes := sha256.Sum256(marshalled)
	icHash := hex.EncodeToString(icHashBytes[:])

	rcCache := remoteConfigCache{
		InitialConfigHash: icHash,
		Config:            "server:\\n log_level: debug",
	}

	require.NoError(t, initialConfigHashCheck(ic, rcCache))
	rcCache.InitialConfigHash = "abc"
	require.Error(t, initialConfigHashCheck(ic, rcCache))

	differentIc := validAgentManagementConfig
	require.Error(t, initialConfigHashCheck(differentIc, rcCache))
}

func TestNewRemoteConfigProvider_ValidInitialConfig(t *testing.T) {
	invalidAgentManagementConfig := &AgentManagementConfig{
		Enabled: true,
		Host:    "localhost:1234",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username:     "test",
				PasswordFile: "/test/path",
			},
		},
		Protocol:        "https",
		PollingInterval: time.Minute,
		RemoteConfiguration: RemoteConfiguration{
			Labels:        labelMap{"b": "B", "a": "A"},
			Namespace:     "test_namespace",
			CacheLocation: "/test/path/",
		},
	}

	cfg := Config{
		AgentManagement: *invalidAgentManagementConfig,
	}
	_, err := newRemoteConfigProvider(&cfg)
	assert.NoError(t, err)
}

func TestNewRemoteConfigProvider_InvalidProtocol(t *testing.T) {
	invalidAgentManagementConfig := &AgentManagementConfig{
		Enabled: true,
		Host:    "localhost:1234",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username:     "test",
				PasswordFile: "/test/path",
			},
		},
		Protocol:        "ws",
		PollingInterval: time.Minute,
		RemoteConfiguration: RemoteConfiguration{
			Labels:        labelMap{"b": "B", "a": "A"},
			Namespace:     "test_namespace",
			CacheLocation: "/test/path/",
		},
	}

	cfg := Config{
		AgentManagement: *invalidAgentManagementConfig,
	}
	_, err := newRemoteConfigProvider(&cfg)
	assert.Error(t, err)
}

func TestNewRemoteConfigHTTPProvider_InvalidInitialConfig(t *testing.T) {
	// this is invalid because it is missing the password file
	invalidAgentManagementConfig := &AgentManagementConfig{
		Enabled: true,
		Host:    "localhost:1234",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username: "test",
			},
		},
		Protocol:        "https",
		PollingInterval: time.Minute,
		RemoteConfiguration: RemoteConfiguration{
			Labels:        labelMap{"b": "B", "a": "A"},
			Namespace:     "test_namespace",
			CacheLocation: "/test/path/",
		},
	}

	cfg := Config{
		AgentManagement: *invalidAgentManagementConfig,
	}
	_, err := newRemoteConfigHTTPProvider(&cfg)
	assert.Error(t, err)
}

func TestGetRemoteConfig_UnmarshallableRemoteConfig(t *testing.T) {
	defaultCfg := DefaultConfig()
	brokenCfg := `completely invalid config (maybe it got corrupted, maybe it was somehow set this way)`

	invalidCfgBytes := []byte(brokenCfg)

	am := validAgentManagementConfig
	logger := server.NewLogger(defaultCfg.Server)
	testProvider := testRemoteConfigProvider{InitialConfig: &am}
	testProvider.fetchedConfigBytesToReturn = invalidCfgBytes
	testProvider.cachedConfigToReturn = cachedConfig

	// flagset is required because some default values are extracted from it.
	// In addition, some flags are defined as dependencies for validation
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	features.Register(fs, allFeatures)
	defaultCfg.RegisterFlags(fs)

	cfg, err := getRemoteConfig(true, &testProvider, logger, fs, false)
	assert.NoError(t, err)
	assert.False(t, testProvider.didCacheRemoteConfig)

	// check that the returned config is the cached one
	// Note: Validate is required for the comparison as it mutates the config
	expected := defaultCfg
	expected.Validate(fs)
	assert.True(t, util.CompareYAML(*cfg, expected))
}

func TestGetRemoteConfig_RemoteFetchFails(t *testing.T) {
	defaultCfg := DefaultConfig()

	am := validAgentManagementConfig
	logger := server.NewLogger(defaultCfg.Server)
	testProvider := testRemoteConfigProvider{InitialConfig: &am}
	testProvider.fetchedConfigErrorToReturn = errors.New("connection refused")
	testProvider.cachedConfigToReturn = cachedConfig

	// flagset is required because some default values are extracted from it.
	// In addition, some flags are defined as dependencies for validation
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	features.Register(fs, allFeatures)
	defaultCfg.RegisterFlags(fs)

	cfg, err := getRemoteConfig(true, &testProvider, logger, fs, false)
	assert.NoError(t, err)
	assert.False(t, testProvider.didCacheRemoteConfig)

	// check that the returned config is the cached one
	// Note: Validate is required for the comparison as it mutates the config
	expected := defaultCfg
	expected.Validate(fs)
	assert.True(t, util.CompareYAML(*cfg, expected))
}

func TestGetRemoteConfig_SemanticallyInvalidBaseConfig(t *testing.T) {
	defaultCfg := DefaultConfig()

	// this is semantically invalid because it has two scrape_configs with
	// the same job_name
	invalidConfig := `
{
  "base_config": "metrics:\n  configs:\n  - name: Metrics Snippets\n    scrape_configs:\n    - job_name: 'prometheus'\n      scrape_interval: 15s\n      static_configs:\n      - targets: ['localhost:12345']\n    - job_name: 'prometheus'\n      scrape_interval: 15s\n      static_configs:\n      - targets: ['localhost:12345']\n",
  "snippets": []
}`
	invalidCfgBytes := []byte(invalidConfig)

	am := validAgentManagementConfig
	logger := server.NewLogger(defaultCfg.Server)
	testProvider := testRemoteConfigProvider{InitialConfig: &am}
	testProvider.fetchedConfigBytesToReturn = invalidCfgBytes
	testProvider.cachedConfigToReturn = cachedConfig

	// flagset is required because some default values are extracted from it.
	// In addition, some flags are defined as dependencies for validation
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	features.Register(fs, allFeatures)
	defaultCfg.RegisterFlags(fs)

	cfg, err := getRemoteConfig(true, &testProvider, logger, fs, false)
	assert.NoError(t, err)
	assert.False(t, testProvider.didCacheRemoteConfig)

	// check that the returned config is the cached one
	// Note: Validate is required for the comparison as it mutates the config
	expected := defaultCfg
	expected.Validate(fs)
	assert.True(t, util.CompareYAML(*cfg, expected))
}

func TestGetRemoteConfig_InvalidSnippet(t *testing.T) {
	defaultCfg := DefaultConfig()

	// this is semantically invalid because it has two scrape_configs with
	// the same job_name
	invalidConfig := `
{
  "base_config": "server:\n  log_level: info\n  log_format: logfmt\n",
  "snippets": [
    {
      "config": "metrics_scrape_configs:\n- job_name: 'prometheus'\n- job_name: 'prometheus'\n"
    }
  ]
}`
	invalidCfgBytes := []byte(invalidConfig)

	am := validAgentManagementConfig
	logger := server.NewLogger(defaultCfg.Server)
	testProvider := testRemoteConfigProvider{InitialConfig: &am}
	testProvider.fetchedConfigBytesToReturn = invalidCfgBytes
	testProvider.cachedConfigToReturn = cachedConfig

	// flagset is required because some default values are extracted from it.
	// In addition, some flags are defined as dependencies for validation
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	features.Register(fs, allFeatures)
	defaultCfg.RegisterFlags(fs)

	cfg, err := getRemoteConfig(true, &testProvider, logger, fs, false)
	assert.NoError(t, err)
	assert.False(t, testProvider.didCacheRemoteConfig)

	// check that the returned config is the cached one
	// Note: Validate is required for the comparison as it mutates the config
	expected := defaultCfg
	expected.Validate(fs)
	assert.True(t, util.CompareYAML(*cfg, expected))
}

func TestGetRemoteConfig_EmptyBaseConfig(t *testing.T) {
	defaultCfg := DefaultConfig()

	validConfig := `
{
  "base_config": "",
  "snippets": []
}`
	cfgBytes := []byte(validConfig)
	am := validAgentManagementConfig
	logger := server.NewLogger(defaultCfg.Server)
	testProvider := testRemoteConfigProvider{InitialConfig: &am}
	testProvider.fetchedConfigBytesToReturn = cfgBytes
	testProvider.cachedConfigToReturn = cachedConfig

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	features.Register(fs, allFeatures)
	defaultCfg.RegisterFlags(fs)

	cfg, err := getRemoteConfig(true, &testProvider, logger, fs, false)
	assert.NoError(t, err)
	assert.True(t, testProvider.didCacheRemoteConfig)

	// check that the returned config is not the cached one
	assert.NotEqual(t, "debug", cfg.Server.LogLevel.String())
}

func TestGetRemoteConfig_ValidBaseConfig(t *testing.T) {
	defaultCfg := DefaultConfig()
	validConfig := `
{
  "base_config": "server:\n  log_level: debug\n  log_format: logfmt\nlogs:\n  positions_directory: /tmp\n  global:\n    clients:\n      - basic_auth:\n          password_file: key.txt\n          username: 278220\n        url: https://logs-prod-eu-west-0.grafana.net/loki/api/v1/push\nintegrations:\n  agent:\n    enabled: false\n",
  "snippets": [
    {
      "config": "metrics_scrape_configs:\n- job_name: 'prometheus'\n  scrape_interval: 15s\n  static_configs:\n  - targets: ['localhost:12345']\nlogs_scrape_configs:\n- job_name: yologs\n  static_configs:\n    - targets: [localhost]\n      labels:\n        job: yologs\n        __path__: /tmp/yo.log\n",
      "selector": {
        "hostname": "machine-1",
        "team": "team-a"
      }
    }
  ]
}`
	cfgBytes := []byte(validConfig)
	am := validAgentManagementConfig
	logger := server.NewLogger(defaultCfg.Server)
	testProvider := testRemoteConfigProvider{InitialConfig: &am}
	testProvider.fetchedConfigBytesToReturn = cfgBytes
	testProvider.cachedConfigToReturn = cachedConfig

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	features.Register(fs, allFeatures)
	defaultCfg.RegisterFlags(fs)

	cfg, err := getRemoteConfig(true, &testProvider, logger, fs, false)
	assert.NoError(t, err)
	assert.True(t, testProvider.didCacheRemoteConfig)

	// check that the returned config is not the cached one
	assert.False(t, util.CompareYAML(*cfg, defaultCfg))

	// check some fields to make sure the config was parsed correctly
	assert.Equal(t, "debug", cfg.Server.LogLevel.String())
	assert.Equal(t, "278220", cfg.Logs.Global.ClientConfigs[0].Client.BasicAuth.Username)
	assert.Equal(t, "prometheus", cfg.Metrics.Configs[0].ScrapeConfigs[0].JobName)
	assert.Equal(t, "yologs", cfg.Logs.Configs[0].ScrapeConfig[0].JobName)
	assert.Equal(t, 1, len(cfg.Integrations.ConfigV1.Integrations))
}

func TestGetRemoteConfig_ExpandsEnvVars(t *testing.T) {
	defaultCfg := DefaultConfig()
	validConfig := `
{
  "base_config": "server:\n  log_level: info\n  log_format: ${LOG_FORMAT}\nlogs:\n  positions_directory: /tmp\n  global:\n    clients:\n      - basic_auth:\n          password_file: key.txt\n          username: 278220\n        url: https://logs-prod-eu-west-0.grafana.net/loki/api/v1/push\nintegrations:\n  agent:\n    enabled: false\n",
  "snippets": [
    {
      "config": "metrics_scrape_configs:\n- job_name: 'prometheus'\n  scrape_interval: ${SCRAPE_INTERVAL}\n  static_configs:\n  - targets: ['localhost:12345']\n",
      "selector": {
        "hostname": "machine-1",
        "team": "team-a"
      }
    }
  ]
}`
	t.Setenv("SCRAPE_INTERVAL", "15s")
	t.Setenv("LOG_FORMAT", "json")

	cfgBytes := []byte(validConfig)
	am := validAgentManagementConfig
	logger := server.NewLogger(defaultCfg.Server)
	testProvider := testRemoteConfigProvider{InitialConfig: &am}
	testProvider.fetchedConfigBytesToReturn = cfgBytes
	testProvider.cachedConfigToReturn = cachedConfig

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	var configExpandEnv bool
	fs.BoolVar(&configExpandEnv, "config.expand-env", false, "")
	features.Register(fs, allFeatures)
	defaultCfg.RegisterFlags(fs)

	cfg, err := getRemoteConfig(true, &testProvider, logger, fs, false)
	assert.NoError(t, err)
	assert.Equal(t, "15s", cfg.Metrics.Configs[0].ScrapeConfigs[0].ScrapeInterval.String())
	assert.Equal(t, "json", cfg.Server.LogFormat)
}

func TestGetCachedConfig_DefaultConfigFallback(t *testing.T) {
	defaultCfg := DefaultConfig()
	am := validAgentManagementConfig
	logger := server.NewLogger(defaultCfg.Server)
	testProvider := testRemoteConfigProvider{InitialConfig: &am}
	testProvider.cachedConfigErrorToReturn = errors.New("no cached config")

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	features.Register(fs, allFeatures)
	defaultCfg.RegisterFlags(fs)

	cfg, err := getCachedRemoteConfig(true, &testProvider, fs, logger)
	assert.NoError(t, err)

	// check that the returned config is the default one
	assert.True(t, util.CompareYAML(*cfg, defaultCfg))
}

func TestGetCachedConfig_RetryAfter(t *testing.T) {
	defaultCfg := DefaultConfig()
	am := validAgentManagementConfig
	logger := server.NewLogger(defaultCfg.Server)
	testProvider := testRemoteConfigProvider{InitialConfig: &am}
	testProvider.fetchedConfigErrorToReturn = retryAfterError{retryAfter: time.Duration(0)}
	testProvider.cachedConfigToReturn = cachedConfig

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	features.Register(fs, allFeatures)
	defaultCfg.RegisterFlags(fs)

	_, err := getRemoteConfig(true, &testProvider, logger, fs, true)
	assert.NoError(t, err)
	assert.False(t, testProvider.didCacheRemoteConfig)

	// check that FetchRemoteConfig was called only once on the TestProvider
	assert.Equal(t, 1, testProvider.fetchRemoteConfigCallCount)

	// the cached config should have been retrieved once, on the second
	// attempt to fetch the remote config
	assert.Equal(t, 1, testProvider.getCachedConfigCallCount)
}
