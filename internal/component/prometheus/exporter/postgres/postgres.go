package postgres

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/postgres_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.postgres",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "postgres"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // default to component ID if no better instance key can be found
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.convert(opts.ID), defaultInstanceKey)
}

// DefaultArguments holds the default arguments for the prometheus.exporter.postgres
// component.
var DefaultArguments = Arguments{
	DisableSettingsMetrics: false,
	AutoDiscovery: AutoDiscovery{
		Enabled: false,
	},
	DisableDefaultMetrics:   false,
	CustomQueriesConfigPath: "",
}

// Arguments configures the prometheus.exporter.postgres component
type Arguments struct {
	// DataSourceNames to use to connect to Postgres. This is marked optional because it
	// may also be supplied by the POSTGRES_EXPORTER_DATA_SOURCE_NAME env var,
	// though it is not recommended to do so.
	DataSourceNames []alloytypes.Secret `alloy:"data_source_names,attr,optional"`

	// Attributes
	DisableSettingsMetrics  bool     `alloy:"disable_settings_metrics,attr,optional"`
	DisableDefaultMetrics   bool     `alloy:"disable_default_metrics,attr,optional"`
	CustomQueriesConfigPath string   `alloy:"custom_queries_config_path,attr,optional"`
	EnabledCollectors       []string `alloy:"enabled_collectors,attr,optional"`

	// Blocks
	AutoDiscovery AutoDiscovery `alloy:"autodiscovery,block,optional"`
}

func (a *Arguments) Validate() error {
	if a.DisableDefaultMetrics && a.CustomQueriesConfigPath == "" {
		return fmt.Errorf("custom_queries_config_path must be set when disable_default_metrics is true")
	}
	if a.DisableDefaultMetrics && len(a.EnabledCollectors) != 0 {
		return fmt.Errorf("enabled_collectors cannot be set when disable_default_metrics is true")
	}
	return nil
}

// AutoDiscovery controls discovery of databases outside any specified in DataSourceNames.
type AutoDiscovery struct {
	Enabled           bool     `alloy:"enabled,attr,optional"`
	DatabaseAllowlist []string `alloy:"database_allowlist,attr,optional"`
	DatabaseDenylist  []string `alloy:"database_denylist,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) convert(instanceName string) *postgres_exporter.Config {
	return &postgres_exporter.Config{
		DataSourceNames:        a.convertDataSourceNames(),
		DisableSettingsMetrics: a.DisableSettingsMetrics,
		AutodiscoverDatabases:  a.AutoDiscovery.Enabled,
		ExcludeDatabases:       a.AutoDiscovery.DatabaseDenylist,
		IncludeDatabases:       a.AutoDiscovery.DatabaseAllowlist,
		DisableDefaultMetrics:  a.DisableDefaultMetrics,
		QueryPath:              a.CustomQueriesConfigPath,
		Instance:               instanceName,
		EnabledCollectors:      a.EnabledCollectors,
	}
}

func (a *Arguments) convertDataSourceNames() []config_util.Secret {
	dataSourceNames := make([]config_util.Secret, len(a.DataSourceNames))
	for i, dataSourceName := range a.DataSourceNames {
		dataSourceNames[i] = config_util.Secret(dataSourceName)
	}
	return dataSourceNames
}
