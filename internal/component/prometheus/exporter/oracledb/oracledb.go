package oracledb

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/oracledb_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
)

const (
	oracleScheme = "oracle://"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.oracledb",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "oracledb"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the oracledb exporter
var DefaultArguments = Arguments{
	MaxIdleConns: 0,
	MaxOpenConns: 10,
	QueryTimeout: 5,
}
var (
	errNoConnectionString = errors.New("no connection string was provided")
	errWrongSchema        = errors.New("connection string should not contain a scheme if the username and password are provided")
	errBothConfigModes    = errors.New("cannot set connection_string together with database blocks; use one or the other")
	errDuplicateDBName    = errors.New("duplicate database name")
)

// DatabaseTarget configures one Oracle database when using multiple targets in a single component.
type DatabaseTarget struct {
	Name             string            `alloy:"name,attr"`
	ConnectionString alloytypes.Secret `alloy:"connection_string,attr"`
	Username         string            `alloy:"username,attr,optional"`
	Password         alloytypes.Secret `alloy:"password,attr,optional"`
	Labels           map[string]string `alloy:"labels,attr,optional"`
}

// DatabaseTargets is a list of database blocks.
type DatabaseTargets []DatabaseTarget

// Arguments controls the oracledb exporter.
type Arguments struct {
	ConnectionString alloytypes.Secret `alloy:"connection_string,attr,optional"` // Deprecated in favor of the "database" block.
	MaxIdleConns     int               `alloy:"max_idle_conns,attr,optional"`
	MaxOpenConns     int               `alloy:"max_open_conns,attr,optional"`
	QueryTimeout     int               `alloy:"query_timeout,attr,optional"`
	DefaultMetrics   string            `alloy:"default_metrics,attr,optional"`
	CustomMetrics    []string          `alloy:"custom_metrics,attr,optional"`
	Username         string            `alloy:"username,attr,optional"` // Deprecated in favor of the "database" block.
	Password         alloytypes.Secret `alloy:"password,attr,optional"` // Deprecated in favor of the "database" block.
	Databases        DatabaseTargets   `alloy:"database,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if len(a.Databases) > 0 && a.ConnectionString != "" {
		return errBothConfigModes
	}
	if len(a.Databases) == 0 && a.ConnectionString == "" {
		return errNoConnectionString
	}

	seen := make(map[string]struct{}, len(a.Databases))
	for _, d := range a.Databases {
		if strings.TrimSpace(d.Name) == "" {
			return fmt.Errorf("database block requires a non-empty name")
		}
		if _, dup := seen[d.Name]; dup {
			return fmt.Errorf("%w: %q", errDuplicateDBName, d.Name)
		}
		seen[d.Name] = struct{}{}
		if d.ConnectionString == "" {
			return fmt.Errorf("database %q: connection_string is required", d.Name)
		}
		if err := validateOracleConnection(d.ConnectionString, d.Username, d.Password); err != nil {
			return fmt.Errorf("database %q: %w", d.Name, err)
		}
	}

	if len(a.Databases) == 0 {
		return validateOracleConnection(a.ConnectionString, a.Username, a.Password)
	}
	return nil
}

func validateOracleConnection(cs alloytypes.Secret, username string, password alloytypes.Secret) error {
	if hasOracleScheme(cs) {
		if username != "" || password != "" {
			return errWrongSchema
		}
		_, err := url.Parse(string(cs))
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Arguments) Convert() *oracledb_exporter.Config {
	cfg := &oracledb_exporter.Config{
		MaxIdleConns:   a.MaxIdleConns,
		MaxOpenConns:   a.MaxOpenConns,
		QueryTimeout:   a.QueryTimeout,
		CustomMetrics:  a.CustomMetrics,
		DefaultMetrics: a.DefaultMetrics,
	}
	if len(a.Databases) > 0 {
		cfg.Databases = make([]oracledb_exporter.DatabaseInstance, 0, len(a.Databases))
		for _, d := range a.Databases {
			u, user, pass := oracledb_exporter.NormalizeConnectionString(string(d.ConnectionString), d.Username, string(d.Password))
			cfg.Databases = append(cfg.Databases, oracledb_exporter.DatabaseInstance{
				Name:             d.Name,
				ConnectionString: config_util.Secret(u),
				Username:         user,
				Password:         config_util.Secret(pass),
				Labels:           d.Labels,
			})
		}
		return cfg
	}
	cs, user, pass := oracledb_exporter.NormalizeConnectionString(string(a.ConnectionString), a.Username, string(a.Password))
	cfg.ConnectionString = config_util.Secret(cs)
	cfg.Username = user
	cfg.Password = config_util.Secret(pass)
	return cfg
}

func hasOracleScheme(connectionString alloytypes.Secret) bool {
	return strings.HasPrefix(string(connectionString), oracleScheme)
}
