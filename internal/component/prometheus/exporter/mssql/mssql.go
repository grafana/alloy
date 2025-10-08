package mssql

import (
	"errors"
	"fmt"
	"time"

	"github.com/burningalchemist/sql_exporter/config"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/mssql"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
	"gopkg.in/yaml.v2"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.mssql",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "mssql"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the mssql exporter
var DefaultArguments = Arguments{
	MaxIdleConnections: 3,
	MaxOpenConnections: 3,
	Timeout:            10 * time.Second,
}

// Arguments controls the mssql exporter.
type Arguments struct {
	ConnectionString   alloytypes.Secret         `alloy:"connection_string,attr"`
	ConnectionName     string                    `alloy:"connection_name,attr,optional"`
	MaxIdleConnections int                       `alloy:"max_idle_connections,attr,optional"`
	MaxOpenConnections int                       `alloy:"max_open_connections,attr,optional"`
	Timeout            time.Duration             `alloy:"timeout,attr,optional"`
	QueryConfig        alloytypes.OptionalSecret `alloy:"query_config,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.MaxOpenConnections < 1 {
		return errors.New("max_open_connections must be at least 1")
	}

	if a.MaxIdleConnections < 1 {
		return errors.New("max_idle_connections must be at least 1")
	}

	if a.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}

	var collectorConfig config.CollectorConfig
	err := yaml.UnmarshalStrict([]byte(a.QueryConfig.Value), &collectorConfig)
	if err != nil {
		return fmt.Errorf("invalid query_config: %s", err)
	}

	return nil
}

func (a *Arguments) Convert() *mssql.Config {
	return &mssql.Config{
		ConnectionString:   config_util.Secret(a.ConnectionString),
		MaxIdleConnections: a.MaxIdleConnections,
		MaxOpenConnections: a.MaxOpenConnections,
		Timeout:            a.Timeout,
		QueryConfig:        util.RawYAML(a.QueryConfig.Value),
	}
}
