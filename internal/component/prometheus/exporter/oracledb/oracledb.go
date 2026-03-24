package oracledb

import (
	"errors"
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
)

// Arguments controls the oracledb exporter.
type Arguments struct {
	ConnectionString alloytypes.Secret `alloy:"connection_string,attr"`
	MaxIdleConns     int               `alloy:"max_idle_conns,attr,optional"`
	MaxOpenConns     int               `alloy:"max_open_conns,attr,optional"`
	QueryTimeout     int               `alloy:"query_timeout,attr,optional"`
	DefaultMetrics   string            `alloy:"default_metrics,attr,optional"`
	CustomMetrics    []string          `alloy:"custom_metrics,attr,optional"`
	Username         string            `alloy:"username,attr,optional"`
	Password         alloytypes.Secret `alloy:"password,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.ConnectionString == "" {
		return errNoConnectionString
	}

	if hasOracleScheme(a.ConnectionString) {
		if a.Username != "" || a.Password != "" {
			return errWrongSchema
		}
		_, err := url.Parse(string(a.ConnectionString))
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Arguments) Convert() *oracledb_exporter.Config {
	connectionString, username, password := a.convertConnectionString()
	return &oracledb_exporter.Config{
		ConnectionString: config_util.Secret(connectionString),
		MaxIdleConns:     a.MaxIdleConns,
		MaxOpenConns:     a.MaxOpenConns,
		QueryTimeout:     a.QueryTimeout,
		CustomMetrics:    a.CustomMetrics,
		Username:         username,
		Password:         config_util.Secret(password),
		DefaultMetrics:   a.DefaultMetrics,
	}
}

// This is for backwards compatibility with the old config where the username and password were in the connection string
func (a *Arguments) convertConnectionString() (string, string, string) {
	connectionString := string(a.ConnectionString)
	username := a.Username
	password := string(a.Password)
	if hasOracleScheme(a.ConnectionString) {
		u, _ := url.Parse(string(a.ConnectionString))
		if pass, set := u.User.Password(); set && strings.Contains(connectionString, "@") {
			// with credentials
			// oracle://user:pass@host:port/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
			password = pass
			username = u.User.Username()
			connectionString = strings.Join(strings.Split(connectionString, "@")[1:], "@")
		} else {
			// without credentials
			// oracle://host:port/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
			connectionString = strings.TrimPrefix(connectionString, oracleScheme)
		}
	}
	// connectionString is now in the format: host:port/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
	return connectionString, username, password
}

func hasOracleScheme(connectionString alloytypes.Secret) bool {
	return strings.HasPrefix(string(connectionString), oracleScheme)
}
