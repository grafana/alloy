package oracledb_exporter

import (
	"fmt"
	"log/slog"
	"net/url"
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
func (c *Config) InstanceKey(agentKey string) (string, error) {
	// Backward compatibility when the connection string needed the scheme
	if strings.HasPrefix(string(c.ConnectionString), "oracle://") {
		u, err := url.Parse(string(c.ConnectionString))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s:%s", u.Hostname(), u.Port()), nil
	}

	// New url format does not require the scheme
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

// This is for backwards compatibility with the old config where the username and password were in the connection string
// The old config has the format: oracle://user:pass@host:port/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
// The new exporter expects the format: host:port/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
func parseConnectionString(c *Config) (string, string, string, error) {
	connectionString := string(c.ConnectionString)
	username := c.Username
	password := string(c.Password)

	if strings.HasPrefix(string(c.ConnectionString), "oracle://") {
		u, err := url.Parse(connectionString)
		if err != nil {
			return "", "", "", err
		}
		pass, set := u.User.Password()
		if !set {
			return "", "", "", fmt.Errorf("password not set in connection string with scheme")
		}

		if c.Username != "" || c.Password != "" {
			return "", "", "", fmt.Errorf("username and password should not be provided in both the connection string and the arguments")
		}

		password = pass
		username = u.User.Username()
		parts := strings.Split(connectionString, "@")
		if len(parts) < 2 {
			return "", "", "", fmt.Errorf("connection string with credentials must contain an @ character")
		}
		connectionString = strings.Join(parts[1:], "@") // safety in case there are multiple @ characters
	}

	return connectionString, username, password, nil
}

// New creates a new oracledb integration. The integration scrapes metrics
// from an OracleDB exporter running with the https://github.com/oracle/oracle-db-appdev-monitoring
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	connectionString, username, password, err := parseConnectionString(c)
	if err != nil {
		return nil, err
	}

	slogLogger := slog.New(logging.NewSlogGoKitHandler(logger))

	oeExporter, err := oe.NewExporter(slogLogger, &oe.MetricsConfiguration{
		Databases: map[string]oe.DatabaseConfig{
			"default": {
				URL:      connectionString,
				Username: username,
				Password: password,
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
