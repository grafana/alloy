package oracledb

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/oracledb_exporter"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.oracledb",
		Stability: featuregate.StabilityStable,
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
	errNoHostname         = errors.New("no hostname in connection string")
)

// Arguments controls the oracledb exporter.
type Arguments struct {
	ConnectionString rivertypes.Secret `river:"connection_string,attr"`
	MaxIdleConns     int               `river:"max_idle_conns,attr,optional"`
	MaxOpenConns     int               `river:"max_open_conns,attr,optional"`
	QueryTimeout     int               `river:"query_timeout,attr,optional"`
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
func (a *Arguments) Validate() error {
	if a.ConnectionString == "" {
		return errNoConnectionString
	}
	u, err := url.Parse(string(a.ConnectionString))
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

func (a *Arguments) Convert() *oracledb_exporter.Config {
	return &oracledb_exporter.Config{
		ConnectionString: config_util.Secret(a.ConnectionString),
		MaxIdleConns:     a.MaxIdleConns,
		MaxOpenConns:     a.MaxOpenConns,
		QueryTimeout:     a.QueryTimeout,
	}
}
