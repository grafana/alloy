package mongodb_exporter

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/go-kit/log"
	"github.com/percona/mongodb_exporter/exporter"
	config_util "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

var DefaultConfig = Config{
	CompatibleMode:           true,
	CollectAll:               true,
	DirectConnect:            true,
	DiscoveringMode:          true,
	EnableDBStats:            false,
	EnableDBStatsFreeStorage: false,
	EnableDiagnosticData:     false,
	EnableReplicasetStatus:   false,
	EnableReplicasetConfig:   false,
	EnableCurrentopMetrics:   false,
	EnableTopMetrics:         false,
	EnableIndexStats:         false,
	EnableCollStats:          false,
	EnableProfile:            false,
	EnableShards:             false,
	EnableFCV:                false,
	EnablePBMMetrics:         false,
}

// Config controls mongodb_exporter
type Config struct {
	// MongoDB connection URI. example:mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"
	URI                      config_util.Secret `yaml:"mongodb_uri"`
	CompatibleMode           bool               `yaml:"compatible_mode,omitempty"`
	CollectAll               bool               `yaml:"collect_all,omitempty"`
	DirectConnect            bool               `yaml:"direct_connect,omitempty"`
	DiscoveringMode          bool               `yaml:"discovering_mode,omitempty"`
	EnableDBStats            bool               `yaml:"enable_db_stats,omitempty"`
	EnableDBStatsFreeStorage bool               `yaml:"enable_db_stats_free_storage,omitempty"`
	EnableDiagnosticData     bool               `yaml:"enable_diagnostic_data,omitempty"`
	EnableReplicasetStatus   bool               `yaml:"enable_replicaset_status,omitempty"`
	EnableReplicasetConfig   bool               `yaml:"enable_replicaset_config,omitempty"`
	EnableCurrentopMetrics   bool               `yaml:"enable_currentop_metrics,omitempty"`
	EnableTopMetrics         bool               `yaml:"enable_top_metrics,omitempty"`
	EnableIndexStats         bool               `yaml:"enable_index_stats,omitempty"`
	EnableCollStats          bool               `yaml:"enable_coll_stats,omitempty"`
	EnableProfile            bool               `yaml:"enable_profile,omitempty"`
	EnableShards             bool               `yaml:"enable_shards,omitempty"`
	EnableFCV                bool               `yaml:"enable_fcv,omitempty"`
	EnablePBMMetrics         bool               `yaml:"enable_pbm_metrics,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig
	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "mongodb_exporter"
}

// InstanceKey returns the address:port of the mongodb server being queried.
func (c *Config) InstanceKey(_ string) (string, error) {
	u, err := url.Parse(string(c.URI))
	if err != nil {
		return "", fmt.Errorf("could not parse mongodb_uri: %w", errors.Unwrap(err))
	}
	return u.Host, nil
}

// NewIntegration creates a new mongodb_exporter
func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	return New(logger, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("mongodb"))
}

// New creates a new mongodb_exporter integration.
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	logrusLogger := slog.New(logging.NewSlogGoKitHandler(logger))

	exp := exporter.New(&exporter.Opts{
		URI:                    string(c.URI),
		Logger:                 logrusLogger,
		DisableDefaultRegistry: true,

		CompatibleMode:           c.CompatibleMode,
		CollectAll:               c.CollectAll,
		DirectConnect:            c.DirectConnect,
		DiscoveringMode:          c.DiscoveringMode,
		EnableDBStats:            c.EnableDBStats,
		EnableDBStatsFreeStorage: c.EnableDBStatsFreeStorage,
		EnableDiagnosticData:     c.EnableDiagnosticData,
		EnableReplicasetStatus:   c.EnableReplicasetStatus,
		EnableReplicasetConfig:   c.EnableReplicasetConfig,
		EnableCurrentopMetrics:   c.EnableCurrentopMetrics,
		EnableTopMetrics:         c.EnableTopMetrics,
		EnableIndexStats:         c.EnableIndexStats,
		EnableCollStats:          c.EnableCollStats,
		EnableProfile:            c.EnableProfile,
		EnableShards:             c.EnableShards,
		EnableFCV:                c.EnableFCV,
		EnablePBMMetrics:         c.EnablePBMMetrics,
	})

	return integrations.NewHandlerIntegration(c.Name(), exp.Handler()), nil
}
