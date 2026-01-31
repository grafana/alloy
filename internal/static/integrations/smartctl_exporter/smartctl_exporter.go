// Package smartctl_exporter embeds S.M.A.R.T. drive monitoring functionality.
// It collects disk health metrics using the smartctl binary from smartmontools.
package smartctl_exporter

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
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// DefaultConfig holds the default settings for the smartctl_exporter integration.
var DefaultConfig = Config{
	SmartctlPath:   "/usr/sbin/smartctl",
	ScanInterval:   60 * time.Second,
	RescanInterval: 10 * time.Minute,
	PowermodeCheck: "standby",
}

// Config controls the smartctl_exporter integration.
type Config struct {
	// SmartctlPath is the path to the smartctl binary
	SmartctlPath string `yaml:"smartctl_path,omitempty"`

	// ScanInterval is how often to poll smartctl for device data
	ScanInterval time.Duration `yaml:"scan_interval,omitempty"`

	// RescanInterval is how often to rescan for new/removed devices
	// Set to 0 to disable automatic rescanning
	RescanInterval time.Duration `yaml:"rescan_interval,omitempty"`

	// Devices is a list of specific devices to monitor (e.g., /dev/sda, /dev/nvme0n1)
	Devices []string `yaml:"devices,omitempty"`

	// DeviceExclude is a regex to exclude devices from automatic scanning
	// Mutually exclusive with DeviceInclude
	DeviceExclude string `yaml:"device_exclude,omitempty"`

	// DeviceInclude is a regex to include devices in automatic scanning
	// Mutually exclusive with DeviceExclude
	DeviceInclude string `yaml:"device_include,omitempty"`

	// ScanDeviceTypes controls device type scanning (e.g., "sat", "scsi", "nvme")
	// Common values: "sat" (SATA), "scsi" (SAS/SCSI), "nvme" (NVMe), "auto"
	ScanDeviceTypes []string `yaml:"scan_device_types,omitempty"`

	// PowermodeCheck determines when to check device power mode
	// Options: "never", "sleep", "standby", "idle"
	// Default: "standby" (skip devices in standby to avoid waking them)
	PowermodeCheck string `yaml:"powermode_check,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration.
func (c *Config) Name() string {
	return "smartctl_exporter"
}

// InstanceKey returns the instance key for the integration.
// For local disk monitoring, we use localhost as the instance.
func (c *Config) InstanceKey(defaultKey string) (string, error) {
	return "localhost", nil
}

// NewIntegration creates a new smartctl_exporter integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeSingleton, metricsutils.NewNamedShim("smartctl"))
}

// Integration is the smartctl_exporter integration.
type Integration struct {
	cfg       *Config
	logger    log.Logger
	collector *smartctlCollector
}

// New creates a new smartctl_exporter integration.
func New(logger log.Logger, cfg *Config) (integrations.Integration, error) {
	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	collector, err := newSmartctlCollector(logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create smartctl collector: %w", err)
	}

	level.Info(logger).Log("msg", "smartctl_exporter integration initialized",
		"smartctl_path", cfg.SmartctlPath,
		"scan_interval", cfg.ScanInterval,
		"rescan_interval", cfg.RescanInterval)

	return &Integration{
		cfg:       cfg,
		logger:    logger,
		collector: collector,
	}, nil
}

// validate checks the configuration for errors.
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

// MetricsHandler implements Integration.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	registry := prometheus.NewRegistry()
	if err := registry.Register(i.collector); err != nil {
		return nil, fmt.Errorf("couldn't register smartctl collector: %w", err)
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return handler, nil
}

// Run satisfies Integration.Run.
// The collector handles device scanning in background goroutines.
func (i *Integration) Run(ctx context.Context) error {
	// Start the collector's background scanning
	return i.collector.Run(ctx)
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (i *Integration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     i.cfg.Name(),
		MetricsPath: "/metrics",
	}}
}
