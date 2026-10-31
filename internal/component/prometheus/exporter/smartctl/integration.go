package smartctl

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"
)

// Config controls the smartctl_exporter integration.
type Config struct {
	SmartctlPath    string        `yaml:"smartctl_path,omitempty"`
	ScanInterval    time.Duration `yaml:"scan_interval,omitempty"`
	RescanInterval  time.Duration `yaml:"rescan_interval,omitempty"`
	Devices         []string      `yaml:"devices,omitempty"`
	DeviceExclude   string        `yaml:"device_exclude,omitempty"`
	DeviceInclude   string        `yaml:"device_include,omitempty"`
	ScanDeviceTypes []string      `yaml:"scan_device_types,omitempty"`
	PowermodeCheck  string        `yaml:"powermode_check,omitempty"`
}

var _ integrations.Config = (*Config)(nil)

// Name returns the name of the integration.
func (c *Config) Name() string {
	return "smartctl_exporter"
}

// InstanceKey returns the instance key for the integration.
func (c *Config) InstanceKey(defaultKey string) (string, error) {
	return "localhost", nil
}

// NewIntegration creates a new smartctl_exporter integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	collector, err := newSmartctlCollector(l, c)
	if err != nil {
		return nil, fmt.Errorf("failed to create smartctl collector: %w", err)
	}

	level.Info(l).Log("msg", "smartctl_exporter integration initialized",
		"smartctl_path", c.SmartctlPath,
		"scan_interval", c.ScanInterval,
		"rescan_interval", c.RescanInterval)

	return &integration{
		cfg:       c,
		logger:    l,
		collector: collector,
	}, nil
}

func (c *Config) validate() error {
	if c.DeviceExclude != "" && c.DeviceInclude != "" {
		return fmt.Errorf("device_exclude and device_include are mutually exclusive")
	}

	validPowermodes := map[string]bool{
		"never": true, "sleep": true, "standby": true, "idle": true,
	}
	if c.PowermodeCheck != "" && !validPowermodes[c.PowermodeCheck] {
		return fmt.Errorf("invalid powermode_check: %s (must be never, sleep, standby, or idle)", c.PowermodeCheck)
	}

	return nil
}

// integration is the smartctl_exporter integration.
type integration struct {
	cfg       *Config
	logger    log.Logger
	collector *smartctlCollector
}

// MetricsHandler implements Integration.
func (i *integration) MetricsHandler() (http.Handler, error) {
	registry := prometheus.NewRegistry()
	if err := registry.Register(i.collector); err != nil {
		return nil, fmt.Errorf("couldn't register smartctl collector: %w", err)
	}
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), nil
}

// Run satisfies Integration.Run.
func (i *integration) Run(ctx context.Context) error {
	return i.collector.Run(ctx)
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (i *integration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     i.cfg.Name(),
		MetricsPath: "/metrics",
	}}
}
