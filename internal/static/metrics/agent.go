// Package metrics implements a Prometheus-lite client for service discovery,
// scraping metrics into a WAL, and remote_write. Clients are broken into a
// set of instances, each of which contain their own set of configs.
package metrics

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/static/metrics/cluster"
	"github.com/grafana/alloy/internal/static/metrics/cluster/client"
	"github.com/grafana/alloy/internal/static/metrics/instance"
	"github.com/grafana/alloy/internal/util"
)

// DefaultConfig is the default settings for the Prometheus-lite client.
var DefaultConfig = Config{
	Global:                 instance.DefaultGlobalConfig,
	InstanceRestartBackoff: 5 * time.Second,
	// The following legacy WALDir path is intentionally kept for config conversion from static to Alloy.
	// Consult Alloy maintainers for changes.
	WALDir:              "data-agent/",
	WALCleanupAge:       12 * time.Hour,
	WALCleanupPeriod:    30 * time.Minute,
	ServiceConfig:       cluster.DefaultConfig,
	ServiceClientConfig: client.DefaultConfig,
	InstanceMode:        instance.DefaultMode,
}

// Config defines the configuration for the entire set of Prometheus client
// instances, along with a global configuration.
type Config struct {
	Global                 instance.GlobalConfig `yaml:"global,omitempty"`
	WALDir                 string                `yaml:"wal_directory,omitempty"`
	WALCleanupAge          time.Duration         `yaml:"wal_cleanup_age,omitempty"`
	WALCleanupPeriod       time.Duration         `yaml:"wal_cleanup_period,omitempty"`
	ServiceConfig          cluster.Config        `yaml:"scraping_service,omitempty"`
	ServiceClientConfig    client.Config         `yaml:"scraping_service_client,omitempty"`
	Configs                []instance.Config     `yaml:"configs,omitempty"`
	InstanceRestartBackoff time.Duration         `yaml:"instance_restart_backoff,omitempty"`
	InstanceMode           instance.Mode         `yaml:"instance_mode,omitempty"`
	DisableKeepAlives      bool                  `yaml:"http_disable_keepalives,omitempty"`
	IdleConnTimeout        time.Duration         `yaml:"http_idle_conn_timeout,omitempty"`

	// Unmarshaled is true when the Config was unmarshaled from YAML.
	Unmarshaled bool `yaml:"-"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig
	util.DefaultConfigFromFlags(c)
	c.Unmarshaled = true

	type plain Config
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}

	c.ServiceConfig.Client = c.ServiceClientConfig
	return nil
}

// ApplyDefaults applies default values to the Config and validates it.
func (c *Config) ApplyDefaults() error {
	needWAL := len(c.Configs) > 0 || c.ServiceConfig.Enabled
	if needWAL && c.WALDir == "" {
		return errors.New("no wal_directory configured")
	}

	if c.ServiceConfig.Enabled && len(c.Configs) > 0 {
		return errors.New("cannot use configs when scraping_service mode is enabled")
	}

	c.Global.DisableKeepAlives = c.DisableKeepAlives
	c.Global.IdleConnTimeout = c.IdleConnTimeout
	usedNames := map[string]struct{}{}

	for i := range c.Configs {
		name := c.Configs[i].Name
		if err := c.Configs[i].ApplyDefaults(c.Global); err != nil {
			// Try to show a helpful name in the error
			if name == "" {
				name = fmt.Sprintf("at index %d", i)
			}

			return fmt.Errorf("error validating instance %s: %w", name, err)
		}

		if _, ok := usedNames[name]; ok {
			return fmt.Errorf(
				"prometheus instance names must be unique. found multiple instances with name %s",
				name,
			)
		}
		usedNames[name] = struct{}{}
	}

	return nil
}

// RegisterFlags defines flags corresponding to the Config.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	c.RegisterFlagsWithPrefix("metrics.", f)
}

// RegisterFlagsWithPrefix defines flags with the provided prefix.
func (c *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	f.StringVar(&c.WALDir, prefix+"wal-directory", DefaultConfig.WALDir, "base directory to store the WAL in")
	f.DurationVar(&c.WALCleanupAge, prefix+"wal-cleanup-age", DefaultConfig.WALCleanupAge, "remove abandoned (unused) WALs older than this")
	f.DurationVar(&c.WALCleanupPeriod, prefix+"wal-cleanup-period", DefaultConfig.WALCleanupPeriod, "how often to check for abandoned WALs")
	f.DurationVar(&c.InstanceRestartBackoff, prefix+"instance-restart-backoff", DefaultConfig.InstanceRestartBackoff, "how long to wait before restarting a failed Prometheus instance")

	c.ServiceConfig.RegisterFlagsWithPrefix(prefix+"service.", f)
	c.ServiceClientConfig.RegisterFlagsWithPrefix(prefix, f)
}
