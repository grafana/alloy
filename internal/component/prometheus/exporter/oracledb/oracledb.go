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

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.oracledb",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "oracledb"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
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
	errNoPassword         = errors.New("password not set in connection string with scheme")
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

	if strings.HasPrefix(string(a.ConnectionString), "oracle://") && (a.Username != "" || a.Password != "") {
		return errWrongSchema
	}

	if strings.HasPrefix(string(a.ConnectionString), "oracle://") {
		u, err := url.Parse(string(a.ConnectionString))
		if err != nil {
			return err
		}
		_, set := u.User.Password()
		if !set {
			return errNoPassword
		}
	}

	return nil
}

func (a *Arguments) Convert() *oracledb_exporter.Config {
	return &oracledb_exporter.Config{
		ConnectionString: config_util.Secret(a.ConnectionString),
		MaxIdleConns:     a.MaxIdleConns,
		MaxOpenConns:     a.MaxOpenConns,
		QueryTimeout:     a.QueryTimeout,
		CustomMetrics:    a.CustomMetrics,
		Username:         a.Username,
		Password:         config_util.Secret(a.Password),
		DefaultMetrics:   a.DefaultMetrics,
	}
}
