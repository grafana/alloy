package oracledb_exporter

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/go-kit/log"
	oe "github.com/iamseth/oracledb_exporter/collector"

	"github.com/grafana/alloy/internal/static/integrations"

	// required driver for integration
	_ "github.com/sijms/go-ora/v2"

	config_util "github.com/prometheus/common/config"

	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// DefaultConfig is the default config for the oracledb v2 integration
var DefaultConfig = Config{
	ConnectionString: config_util.Secret(os.Getenv("DATA_SOURCE_NAME")),
	MaxOpenConns:     10,
	MaxIdleConns:     0,
	QueryTimeout:     5,
	// CustomMetrics:    "",
}

var (
	errNoConnectionString = errors.New("no connection string was provided")
	errNoHostname         = errors.New("no hostname in connection string")
)

// Config is the configuration for the oracledb v2 integration
type Config struct {
	ConnectionString config_util.Secret `yaml:"connection_string"`
	MaxIdleConns     int                `yaml:"max_idle_connections"`
	MaxOpenConns     int                `yaml:"max_open_connections"`
	QueryTimeout     int                `yaml:"query_timeout"`
	CustomMetrics    string             `yaml:"custom_metrics,omitempty"`
}

// ValidateConnString attempts to ensure the connection string supplied is valid
// to connect to an OracleDB instance
func validateConnString(connStr string) error {
	if connStr == "" {
		return errNoConnectionString
	}
	u, err := url.Parse(connStr)
	if err != nil {
		return fmt.Errorf("unable to parse connection string: %w", err)
	}

	if u.Scheme != "oracle" {
		return fmt.Errorf("unexpected scheme of type '%s'. Was expecting 'oracle': %w", u.Scheme, err)
	}

	// hostname is required for identification
	if u.Hostname() == "" {
		return errNoHostname
	}
	return nil
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the integration name this config is associated with.
func (c *Config) Name() string {
	return "oracledb"
}

// InstanceKey returns the addr of the oracle instance.
func (c *Config) InstanceKey(_ string) (string, error) {
	u, err := url.Parse(string(c.ConnectionString))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", u.Hostname(), u.Port()), nil
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
// from an OracleDB exporter running with the https://github.com/iamseth/oracledb_exporter
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	if err := validateConnString(string(c.ConnectionString)); err != nil {
		return nil, fmt.Errorf("invalid connection string: %w", err)
	}

	oeExporter, err := oe.NewExporter(logger, &oe.Config{
		DSN:           string(c.ConnectionString),
		MaxIdleConns:  c.MaxIdleConns,
		MaxOpenConns:  c.MaxOpenConns,
		CustomMetrics: c.CustomMetrics,
		QueryTimeout:  c.QueryTimeout,
	})

	if err != nil {
		return nil, err
	}
	return integrations.NewCollectorIntegration(c.Name(), integrations.WithCollectors(oeExporter)), nil
}
