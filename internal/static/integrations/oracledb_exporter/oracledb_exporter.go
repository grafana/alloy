package oracledb_exporter

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"
	oe "github.com/oracle/oracle-db-appdev-monitoring/collector"

	// required driver for integration
	_ "github.com/sijms/go-ora/v2"

	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	config_util "github.com/prometheus/common/config"
)

// DefaultConfig is the default config for the oracledb v2 integration
var DefaultConfig = Config{
	ConnectionString: config_util.Secret(os.Getenv("DATA_SOURCE_NAME")),
	MaxOpenConns:     10,
	MaxIdleConns:     0,
	QueryTimeout:     5,
	CustomMetrics:    []string{},
}

// Config is the configuration for the oracledb v2 integration
type Config struct {
	ConnectionString config_util.Secret `yaml:"connection_string"`
	MaxIdleConns     int                `yaml:"max_idle_connections"`
	MaxOpenConns     int                `yaml:"max_open_connections"`
	QueryTimeout     int                `yaml:"query_timeout"`
	DefaultMetrics   string             `yaml:"default_metrics,omitempty"`
	CustomMetrics    []string           `yaml:"custom_metrics,omitempty"`
	Username         string             `yaml:"username,omitempty"`
	Password         config_util.Secret `yaml:"password,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the integration name this config is associated with.
func (c *Config) Name() string {
	return "oracledb"
}

// InstanceKey returns the addr of the oracle instance.
func (c *Config) InstanceKey(agentKey string) (string, error) {
	parts := strings.Split(string(c.ConnectionString), "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid connection string format")
	}
	return parts[0], nil
}

// NewIntegration returns the OracleDB Exporter Integration
func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	return New(logger, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("oracledb"))
}

// New creates a new oracledb integration. The integration scrapes metrics
// from an OracleDB exporter running with the https://github.com/oracle/oracle-db-appdev-monitoring
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	slogLogger := slog.New(logging.NewSlogGoKitHandler(logger))

	oeExporter, err := oe.NewExporter(slogLogger, &oe.MetricsConfiguration{
		Databases: map[string]oe.DatabaseConfig{
			"default": {
				URL:      string(c.ConnectionString),
				Username: c.Username,
				Password: string(c.Password),
				ConnectConfig: oe.ConnectConfig{
					MaxIdleConns: c.MaxIdleConns,
					MaxOpenConns: c.MaxOpenConns,
					QueryTimeout: c.QueryTimeout,
				},
			},
		},
		Metrics: oe.MetricsFilesConfig{
			Custom:  c.CustomMetrics,
			Default: c.DefaultMetrics,
		},
	})

	if err != nil {
		return nil, err
	}
	return integrations.NewCollectorIntegration(c.Name(), integrations.WithCollectors(oeExporter)), nil
}
