package blackbox_exporter

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-kit/log"
	blackbox_config "github.com/prometheus/blackbox_exporter/config"
	"github.com/prometheus/blackbox_exporter/prober"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/grafana/alloy/internal/util"
)

// DefaultConfig holds the default settings for the blackbox_exporter integration.
var DefaultConfig = Config{
	// Default value taken from https://github.com/prometheus/blackbox_exporter/blob/master/main.go#L61
	ProbeTimeoutOffset: 0.5,
}

func loadFile(filename string, log log.Logger) (*blackbox_config.Config, error) {
	r := prometheus.NewRegistry()
	sc := blackbox_config.NewSafeConfig(r)
	err := sc.ReloadConfig(filename, log)
	if err != nil {
		return nil, err
	}
	return sc.C, nil
}

// BlackboxTarget defines a target device to be used by the integration.
type BlackboxTarget struct {
	Name   string `yaml:"name"`
	Target string `yaml:"address"`
	Module string `yaml:"module"`
}

// Config configures the Blackbox integration.
type Config struct {
	BlackboxConfigFile string           `yaml:"config_file,omitempty"`
	BlackboxTargets    []BlackboxTarget `yaml:"blackbox_targets"`
	BlackboxConfig     util.RawYAML     `yaml:"blackbox_config,omitempty"`
	ProbeTimeoutOffset float64          `yaml:"probe_timeout_offset,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}

	var blackbox_config blackbox_config.Config
	return yaml.Unmarshal(c.BlackboxConfig, &blackbox_config)
}

// Name returns the name of the integration.
func (c *Config) Name() string {
	return "blackbox"
}

func (c *Config) InstanceKey(defaultKey string) (string, error) {
	return defaultKey, nil
}

// NewIntegration creates a new blackbox integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
}

// LoadBlackboxConfig loads the blackbox config from the given file or from embedded yaml block
// it also validates that targets are properly defined
func LoadBlackboxConfig(log log.Logger, configFile string, targets []BlackboxTarget, modules *blackbox_config.Config) (*blackbox_config.Config, error) {
	var err error

	if configFile != "" {
		modules, err = loadFile(configFile, log)
		if err != nil {
			return nil, fmt.Errorf("failed to load blackbox config from file %v: %w", configFile, err)
		}
	}

	// The `name` and `address` fields are mandatory for the Blackbox targets are mandatory.
	// Enforce this check and fail the creation of the integration if they're missing.
	for _, target := range targets {
		if target.Name == "" || target.Target == "" {
			return nil, fmt.Errorf("failed to load blackbox_targets; the `name` and `address` fields are mandatory")
		}
	}
	return modules, nil
}

// New creates a new blackbox_exporter integration
func New(log log.Logger, c *Config) (integrations.Integration, error) {
	if c.BlackboxConfigFile == "" && c.BlackboxConfig == nil {
		return nil, fmt.Errorf("failed to load blackbox config; no config file or config block provided")
	}

	var blackbox_config blackbox_config.Config
	err := yaml.Unmarshal(c.BlackboxConfig, &blackbox_config)
	if err != nil {
		return nil, err
	}

	modules, err := LoadBlackboxConfig(log, c.BlackboxConfigFile, c.BlackboxTargets, &blackbox_config)
	if err != nil {
		return nil, err
	}

	integration := &Integration{
		cfg:     c,
		modules: modules,
		log:     log,
	}
	return integration, nil
}

// Integration is the blackbox integration. The integration scrapes metrics
// probing of endpoints over HTTP, HTTPS, DNS, TCP, ICMP and gRPC.
type Integration struct {
	cfg     *Config
	modules *blackbox_config.Config
	log     log.Logger
}

// MetricsHandler implements Integration.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prober.Handler(w, r, i.modules, i.log, &prober.ResultHistory{}, i.cfg.ProbeTimeoutOffset, nil, nil)
	}), nil
}

// Run satisfies Integration.Run.
func (i *Integration) Run(ctx context.Context) error {
	// We don't need to do anything here, so we can just wait for the context to
	// finish.
	<-ctx.Done()
	return ctx.Err()
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (i *Integration) ScrapeConfigs() []config.ScrapeConfig {
	var res []config.ScrapeConfig
	for _, target := range i.cfg.BlackboxTargets {
		queryParams := url.Values{}
		queryParams.Add("target", target.Target)
		if target.Module != "" {
			queryParams.Add("module", target.Module)
		}
		res = append(res, config.ScrapeConfig{
			JobName:     i.cfg.Name() + "/" + target.Name,
			MetricsPath: "/metrics",
			QueryParams: queryParams,
		})
	}
	return res
}
