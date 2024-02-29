package config

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/drone/envsubst/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/build"
	"github.com/grafana/agent/internal/static/config/encoder"
	"github.com/grafana/agent/internal/static/config/features"
	"github.com/grafana/agent/internal/static/config/instrumentation"
	"github.com/grafana/agent/internal/static/logs"
	"github.com/grafana/agent/internal/static/metrics"
	"github.com/grafana/agent/internal/static/server"
	"github.com/grafana/agent/internal/static/traces"
	"github.com/grafana/agent/internal/util"
	"github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

var (
	featRemoteConfigs    = features.Feature("remote-configs")
	featIntegrationsNext = features.Feature("integrations-next")
	featExtraMetrics     = features.Feature("extra-scrape-metrics")
	featAgentManagement  = features.Feature("agent-management")

	allFeatures = []features.Feature{
		featRemoteConfigs,
		featIntegrationsNext,
		featExtraMetrics,
		featAgentManagement,
	}
)

var (
	fileTypeYAML    = "yaml"
	fileTypeDynamic = "dynamic"

	fileTypes = []string{fileTypeYAML, fileTypeDynamic}
)

// DefaultConfig holds default settings for all the subsystems.
func DefaultConfig() Config {
	defaultServerCfg := server.DefaultConfig()
	return Config{
		// All subsystems with a DefaultConfig should be listed here.
		Server:                &defaultServerCfg,
		ServerFlags:           server.DefaultFlags,
		Metrics:               metrics.DefaultConfig,
		Integrations:          DefaultVersionedIntegrations(),
		DisableSupportBundle:  false,
		EnableConfigEndpoints: false,
		EnableUsageReport:     true,
	}
}

// Config contains underlying configurations for the agent
type Config struct {
	Server          *server.Config        `yaml:"server,omitempty"`
	Metrics         metrics.Config        `yaml:"metrics,omitempty"`
	Integrations    VersionedIntegrations `yaml:"integrations,omitempty"`
	Traces          traces.Config         `yaml:"traces,omitempty"`
	Logs            *logs.Config          `yaml:"logs,omitempty"`
	AgentManagement AgentManagementConfig `yaml:"agent_management,omitempty"`

	// Flag-only fields
	ServerFlags server.Flags `yaml:"-"`

	// Deprecated fields user has used. Generated during UnmarshalYAML.
	Deprecations []string `yaml:"-"`

	// Remote config options
	BasicAuthUser     string `yaml:"-"`
	BasicAuthPassFile string `yaml:"-"`

	// Toggle for config endpoint(s)
	EnableConfigEndpoints bool `yaml:"-"`

	// Toggle for support bundle generation.
	DisableSupportBundle bool `yaml:"-"`

	// Report enabled features options
	EnableUsageReport bool     `yaml:"-"`
	EnabledFeatures   []string `yaml:"-"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Apply defaults to the config from our struct and any defaults inherited
	// from flags before unmarshaling.
	*c = DefaultConfig()
	util.DefaultConfigFromFlags(c)

	type baseConfig Config

	type config struct {
		baseConfig `yaml:",inline"`

		// Deprecated field names:
		Prometheus *metrics.Config `yaml:"prometheus,omitempty"`
		Loki       *logs.Config    `yaml:"loki,omitempty"`
		Tempo      *traces.Config  `yaml:"tempo,omitempty"`
	}

	var fc config
	fc.baseConfig = baseConfig(*c)

	if err := unmarshal(&fc); err != nil {
		return err
	}

	// Migrate old fields to the new name
	if fc.Prometheus != nil && fc.Metrics.Unmarshaled && fc.Prometheus.Unmarshaled {
		return fmt.Errorf("at most one of prometheus and metrics should be specified")
	} else if fc.Prometheus != nil && fc.Prometheus.Unmarshaled {
		fc.Deprecations = append(fc.Deprecations, "`prometheus` has been deprecated in favor of `metrics`")
		fc.Metrics = *fc.Prometheus
		fc.Prometheus = nil
	}

	if fc.Logs != nil && fc.Loki != nil {
		return fmt.Errorf("at most one of loki and logs should be specified")
	} else if fc.Logs == nil && fc.Loki != nil {
		fc.Deprecations = append(fc.Deprecations, "`loki` has been deprecated in favor of `logs`")
		fc.Logs = fc.Loki
		fc.Loki = nil
	}

	if fc.Tempo != nil && fc.Traces.Unmarshaled {
		return fmt.Errorf("at most one of tempo and traces should be specified")
	} else if fc.Tempo != nil && fc.Tempo.Unmarshaled {
		fc.Deprecations = append(fc.Deprecations, "`tempo` has been deprecated in favor of `traces`")
		fc.Traces = *fc.Tempo
		fc.Tempo = nil
	}

	*c = Config(fc.baseConfig)
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (c Config) MarshalYAML() (interface{}, error) {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)

	type config Config
	if err := enc.Encode((config)(c)); err != nil {
		return nil, err
	}

	// Use a yaml.MapSlice rather than a map[string]interface{} so
	// order of keys is retained compared to just calling MarshalConfig.
	var m yaml.MapSlice
	if err := yaml.Unmarshal(buf.Bytes(), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// LogDeprecations will log use of any deprecated fields to l as warn-level
// messages.
func (c *Config) LogDeprecations(l log.Logger) {
	for _, d := range c.Deprecations {
		level.Warn(l).Log("msg", fmt.Sprintf("DEPRECATION NOTICE: %s", d))
	}
}

// Validate validates the config, flags, and sets default values.
func (c *Config) Validate(fs *flag.FlagSet) error {
	if c.Server == nil {
		return fmt.Errorf("an empty server config is invalid")
	}

	if err := c.Metrics.ApplyDefaults(); err != nil {
		return err
	}

	if c.Logs != nil {
		if err := c.Logs.ApplyDefaults(); err != nil {
			return err
		}
	}

	// Need to propagate the listen address to the host and grpcPort
	_, grpcPort, err := c.ServerFlags.GRPC.ListenHostPort()
	if err != nil {
		return err
	}
	c.Metrics.ServiceConfig.Lifecycler.ListenPort = grpcPort

	// TODO(jcreixell): Make this method side-effect free and, if necessary, implement a
	// method bundling defaults application and validation. Rationale: sometimes (for example
	// in tests) we want to validate a config without mutating it, or apply all defaults
	// for comparison.
	if err := c.Integrations.ApplyDefaults(&c.ServerFlags, &c.Metrics); err != nil {
		return err
	}

	// since the Traces config might rely on an existing Loki config
	// this check is made here to look for cross config issues before we attempt to load
	if err := c.Traces.Validate(c.Logs); err != nil {
		return err
	}

	if c.AgentManagement.Enabled {
		if err := c.AgentManagement.Validate(); err != nil {
			return fmt.Errorf("invalid agent management config: %w", err)
		}
	}

	c.Metrics.ServiceConfig.APIEnableGetConfiguration = c.EnableConfigEndpoints

	// Don't validate flags if there's no FlagSet. Used for testing.
	if fs == nil {
		return nil
	}
	deps := []features.Dependency{
		{Flag: "config.url.basic-auth-user", Feature: featRemoteConfigs},
		{Flag: "config.url.basic-auth-password-file", Feature: featRemoteConfigs},
	}
	return features.Validate(fs, deps)
}

// RegisterFlags registers flags in underlying configs
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	c.Metrics.RegisterFlags(f)
	c.ServerFlags.RegisterFlags(f)

	f.StringVar(&c.BasicAuthUser, "config.url.basic-auth-user", "",
		"basic auth username for fetching remote config. (requires remote-configs experiment to be enabled")
	f.StringVar(&c.BasicAuthPassFile, "config.url.basic-auth-password-file", "",
		"path to file containing basic auth password for fetching remote config. (requires remote-configs experiment to be enabled")

	f.BoolVar(&c.EnableConfigEndpoints, "config.enable-read-api", false, "Enables the /-/config and /agent/api/v1/configs/{name} APIs. Be aware that secrets could be exposed by enabling these endpoints!")
}

// LoadFile reads a file and passes the contents to Load
func LoadFile(filename string, expandEnvVars bool, c *Config) error {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading config file %w", err)
	}
	instrumentation.InstrumentConfig(buf)
	return LoadBytes(buf, expandEnvVars, c)
}

// loadFromAgentManagementAPI loads and merges a config from an Agent Management API.
//  1. Read local initial config.
//  2. Get the remote config.
//     a) Fetch from remote. If this fails or is invalid:
//     b) Read the remote config from cache. If this fails, return an error.
//  4. Merge the initial and remote config into c.
func loadFromAgentManagementAPI(path string, expandEnvVars bool, c *Config, log *server.Logger, fs *flag.FlagSet) error {
	// Load the initial config from disk without instrumenting the config hash
	buf, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading initial config file %w", err)
	}

	err = LoadBytes(buf, expandEnvVars, c)
	if err != nil {
		return fmt.Errorf("failed to load initial config: %w", err)
	}

	configProvider, err := newRemoteConfigProvider(c)
	if err != nil {
		return err
	}
	remoteConfig, err := getRemoteConfig(expandEnvVars, configProvider, log, fs, true)
	if err != nil {
		return err
	}
	mergeEffectiveConfig(c, remoteConfig)

	effectiveConfigBytes, err := yaml.Marshal(c)
	if err != nil {
		level.Warn(log).Log("msg", "error marshalling config for instrumenting config version", "err", err)
	} else {
		instrumentation.InstrumentConfig(effectiveConfigBytes)
	}

	return nil
}

// mergeEffectiveConfig overwrites any values in initialConfig with those in remoteConfig
func mergeEffectiveConfig(initialConfig *Config, remoteConfig *Config) {
	initialConfig.Server = remoteConfig.Server
	initialConfig.Metrics = remoteConfig.Metrics
	initialConfig.Integrations = remoteConfig.Integrations
	initialConfig.Traces = remoteConfig.Traces
	initialConfig.Logs = remoteConfig.Logs
}

// LoadRemote reads a config from url
func LoadRemote(url string, expandEnvVars bool, c *Config) error {
	remoteOpts := &remoteOpts{}
	if c.BasicAuthUser != "" && c.BasicAuthPassFile != "" {
		remoteOpts.HTTPClientConfig = &config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username:     c.BasicAuthUser,
				PasswordFile: c.BasicAuthPassFile,
			},
		}
	}

	if remoteOpts.HTTPClientConfig != nil {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		remoteOpts.HTTPClientConfig.SetDirectory(dir)
	}

	rc, err := newRemoteProvider(url, remoteOpts)
	if err != nil {
		return fmt.Errorf("error reading remote config: %w", err)
	}
	// fall back to file if no scheme is passed
	if rc == nil {
		return LoadFile(url, expandEnvVars, c)
	}
	bb, _, err := rc.retrieve()
	if err != nil {
		return fmt.Errorf("error retrieving remote config: %w", err)
	}

	instrumentation.InstrumentConfig(bb)

	return LoadBytes(bb, expandEnvVars, c)
}

func performEnvVarExpansion(buf []byte, expandEnvVars bool) ([]byte, error) {
	utf8Buf, err := encoder.EnsureUTF8(buf, false)
	if err != nil {
		return nil, err
	}
	// (Optionally) expand with environment variables
	if expandEnvVars {
		s, err := envsubst.Eval(string(utf8Buf), getenv)
		if err != nil {
			return nil, fmt.Errorf("unable to substitute config with environment variables: %w", err)
		}
		return []byte(s), nil
	}
	return utf8Buf, nil
}

// LoadBytes unmarshals a config from a buffer. Defaults are not
// applied to the file and must be done manually if LoadBytes
// is called directly.
func LoadBytes(buf []byte, expandEnvVars bool, c *Config) error {
	expandedBuf, err := performEnvVarExpansion(buf, expandEnvVars)
	if err != nil {
		return err
	}
	// Unmarshal yaml config
	return yaml.UnmarshalStrict(expandedBuf, c)
}

// getenv is a wrapper around os.Getenv that ignores patterns that are numeric
// regex capture groups (ie "${1}").
func getenv(name string) string {
	numericName := true

	for _, r := range name {
		if !unicode.IsDigit(r) {
			numericName = false
			break
		}
	}

	if numericName {
		// We need to add ${} back in since envsubst removes it.
		return fmt.Sprintf("${%s}", name)
	}
	return os.Getenv(name)
}

// Load loads a config file from a flagset. Flags will be registered
// to the flagset before parsing them with the values specified by
// args.
func Load(fs *flag.FlagSet, args []string, log *server.Logger) (*Config, error) {
	cfg, error := LoadFromFunc(fs, args, func(path, fileType string, expandEnvVars bool, c *Config) error {
		switch fileType {
		case fileTypeYAML:
			if features.Enabled(fs, featRemoteConfigs) {
				return LoadRemote(path, expandEnvVars, c)
			}
			if features.Enabled(fs, featAgentManagement) {
				return loadFromAgentManagementAPI(path, expandEnvVars, c, log, fs)
			}
			return LoadFile(path, expandEnvVars, c)
		default:
			return fmt.Errorf("unknown file type %q. accepted values: %s", fileType, strings.Join(fileTypes, ", "))
		}
	})

	instrumentation.InstrumentLoad(error == nil)
	return cfg, error
}

type loaderFunc func(path string, fileType string, expandEnvVars bool, target *Config) error

func applyIntegrationValuesFromFlagset(fs *flag.FlagSet, args []string, path string, cfg *Config) error {
	// Parse the flags again to override any YAML values with command line flag
	// values.
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("error parsing flags: %w", err)
	}

	// Complete unmarshaling integrations using the version from the flag. This
	// MUST be called before ApplyDefaults.
	version := IntegrationsVersion1
	if features.Enabled(fs, featIntegrationsNext) {
		version = IntegrationsVersion2
	}

	if err := cfg.Integrations.setVersion(version); err != nil {
		return fmt.Errorf("error loading config file %s: %w", path, err)
	}
	return nil
}

// LoadFromFunc injects a function for retrieving the config file that
// doesn't require having a literal file on disk.
func LoadFromFunc(fs *flag.FlagSet, args []string, loader loaderFunc) (*Config, error) {
	var (
		cfg = DefaultConfig()

		printVersion          bool
		file                  string
		fileType              string
		configExpandEnv       bool
		disableReporting      bool
		disableSupportBundles bool
	)

	fs.StringVar(&file, "config.file", "", "configuration file to load")
	fs.StringVar(&fileType, "config.file.type", "yaml", fmt.Sprintf("Type of file pointed to by -config.file flag. Supported values: %s. %s requires dynamic-config and integrations-next features to be enabled.", strings.Join(fileTypes, ", "), fileTypeDynamic))
	fs.BoolVar(&printVersion, "version", false, "Print this build's version information.")
	fs.BoolVar(&configExpandEnv, "config.expand-env", false, "Expands ${var} in config according to the values of the environment variables.")
	fs.BoolVar(&disableReporting, "disable-reporting", false, "Disable reporting of enabled feature flags to Grafana.")
	fs.BoolVar(&disableSupportBundles, "disable-support-bundle", false, "Disable functionality for generating support bundles.")
	cfg.RegisterFlags(fs)

	features.Register(fs, allFeatures)

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error parsing flags: %w", err)
	}

	if printVersion {
		fmt.Println(build.Print("agent"))
		os.Exit(0)
	}

	if file == "" {
		return nil, fmt.Errorf("-config.file flag required")
	} else if err := loader(file, fileType, configExpandEnv, &cfg); err != nil {
		return nil, fmt.Errorf("error loading config file %s: %w", file, err)
	}

	if err := applyIntegrationValuesFromFlagset(fs, args, file, &cfg); err != nil {
		return nil, err
	}

	if features.Enabled(fs, featExtraMetrics) {
		cfg.Metrics.Global.ExtraMetrics = true
	}

	if disableReporting {
		cfg.EnableUsageReport = false
	} else {
		cfg.EnabledFeatures = features.GetAllEnabled(fs)
	}

	cfg.AgentManagement.Enabled = features.Enabled(fs, featAgentManagement)

	if disableSupportBundles {
		cfg.DisableSupportBundle = true
	}

	// Finally, apply defaults to config that wasn't specified by file or flag
	if err := cfg.Validate(fs); err != nil {
		return nil, fmt.Errorf("error in config file: %w", err)
	}
	return &cfg, nil
}

// CheckSecret is a helper function to ensure the original value is overwritten with <secret>
func CheckSecret(t *testing.T, rawCfg string, originalValue string) {
	var cfg Config
	err := LoadBytes([]byte(rawCfg), false, &cfg)
	require.NoError(t, err)

	// Set integrations version to make sure our marshal function goes through
	// the custom marshaling code.
	err = cfg.Integrations.setVersion(IntegrationsVersion1)
	require.NoError(t, err)

	bb, err := yaml.Marshal(&cfg)
	require.NoError(t, err)

	require.True(t, strings.Contains(string(bb), "<secret>"))
	require.False(t, strings.Contains(string(bb), originalValue))
}
