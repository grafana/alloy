package oracledb_exporter

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"
	oe "github.com/oracle/oracle-db-appdev-monitoring/collector"

	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	config_util "github.com/prometheus/common/config"
	"k8s.io/utils/ptr"
)

const oracleScheme = "oracle://"

// DefaultConfig is the default config for the oracledb v2 integration
var DefaultConfig = Config{
	ConnectionString: config_util.Secret(os.Getenv("DATA_SOURCE_NAME")),
	MaxOpenConns:     10,
	MaxIdleConns:     0,
	QueryTimeout:     5,
	CustomMetrics:    []string{},
}

// DatabaseInstance configures one Oracle database when using multi-database mode.
type DatabaseInstance struct {
	Name             string             `yaml:"name"`
	ConnectionString config_util.Secret `yaml:"connection_string"`
	Username         string             `yaml:"username,omitempty"`
	Password         config_util.Secret `yaml:"password,omitempty"`
	Labels           map[string]string  `yaml:"labels,omitempty"`
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
	// Databases, when non-empty, defines multiple targets. In that case ConnectionString must be empty.
	Databases []DatabaseInstance `yaml:"databases,omitempty"`
}

type normalizedOracleDB struct {
	name   string
	url    string
	user   string
	pass   string
	labels map[string]string
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

// InstanceKey returns the addr of the oracle instance when exactly one database
// is configured. For multi-database configs, it returns defaultKey (typically
// the component ID) to avoid high-cardinality, unstable combined instance
// labels; this is the standard pattern used by multi-target exporters.
func (c *Config) InstanceKey(defaultKey string) (string, error) {
	targets, err := c.normalizedTargets()
	if err != nil {
		return "", err
	}
	if len(targets) == 0 {
		return "", fmt.Errorf("no databases configured")
	}
	if len(targets) > 1 {
		return defaultKey, nil
	}
	parts := strings.Split(targets[0].url, "/")
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

// NormalizeConnectionString strips an optional oracle:// scheme and optional
// embedded credentials, returning host-style URL, username, and password.
// Example:
//
//	NormalizeConnectionString("oracle://scott:tiger@db.example.com:1521/ORCL", "", "")
//	returns ("db.example.com:1521/ORCL", "scott", "tiger").
func NormalizeConnectionString(connectionString, username, password string) (string, string, string) {
	cs := connectionString
	user := username
	pass := password
	urlOut := ""
	if strings.HasPrefix(cs, oracleScheme) {
		u, _ := url.Parse(cs)
		if u != nil && u.User != nil {
			if p, set := u.User.Password(); set && strings.Contains(cs, "@") {
				pass = p
				user = u.User.Username()
				urlOut = strings.Join(strings.Split(cs, "@")[1:], "@")
			} else {
				urlOut = strings.TrimPrefix(cs, oracleScheme)
			}
		} else {
			urlOut = strings.TrimPrefix(cs, oracleScheme)
		}
	} else {
		urlOut = cs
	}
	return urlOut, user, pass
}

func (c *Config) normalizedTargets() ([]normalizedOracleDB, error) {
	if len(c.Databases) > 0 {
		out := make([]normalizedOracleDB, 0, len(c.Databases))
		for _, d := range c.Databases {
			if strings.TrimSpace(d.Name) == "" {
				return nil, fmt.Errorf("database name is required for each entry in databases")
			}
			u, user, pass := NormalizeConnectionString(string(d.ConnectionString), d.Username, string(d.Password))
			out = append(out, normalizedOracleDB{
				name:   d.Name,
				url:    u,
				user:   user,
				pass:   pass,
				labels: d.Labels,
			})
		}
		return out, nil
	}
	if c.ConnectionString == "" {
		return nil, nil
	}
	u, user, pass := NormalizeConnectionString(string(c.ConnectionString), c.Username, string(c.Password))
	return []normalizedOracleDB{{
		name: "default",
		url:  u,
		user: user,
		pass: pass,
	}}, nil
}

// New creates a new oracledb integration. The integration scrapes metrics
// from an OracleDB exporter running with the https://github.com/oracle/oracle-db-appdev-monitoring
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	slogLogger := slog.New(logging.NewSlogGoKitHandler(logger))

	targets, err := c.normalizedTargets()
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, errors.New("no databases configured")
	}

	databases := make(map[string]oe.DatabaseConfig, len(targets))
	for _, t := range targets {
		databases[t.name] = oe.DatabaseConfig{
			URL:      t.url,
			Username: t.user,
			Password: t.pass,
			ConnectConfig: oe.ConnectConfig{
				MaxIdleConns: ptr.To(c.MaxIdleConns),
				MaxOpenConns: ptr.To(c.MaxOpenConns),
				QueryTimeout: ptr.To(c.QueryTimeout),
			},
			Labels: t.labels,
		}
	}

	// ScrapeInterval must be non-nil: the collector's Collect path dereferences it.
	scrapeInterval := time.Duration(0)
	oeExporter := oe.NewExporter(slogLogger, &oe.MetricsConfiguration{
		Databases: databases,
		Metrics: oe.MetricsFilesConfig{
			Custom:         c.CustomMetrics,
			Default:        c.DefaultMetrics,
			ScrapeInterval: &scrapeInterval,
		},
	})

	if oeExporter == nil {
		return nil, errors.New("failed to create oracledb exporter")
	}
	return integrations.NewCollectorIntegration(c.Name(), integrations.WithCollectors(oeExporter)), nil
}
