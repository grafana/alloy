package mysql

import (
	"github.com/go-sql-driver/mysql"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/mysqld_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.mysql",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "mysql"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the mysqld_exporter integration.
var DefaultArguments = Arguments{
	LockWaitTimeout: 2,
	InfoSchemaProcessList: InfoSchemaProcessList{
		ProcessesByUser: true,
		ProcessesByHost: true,
	},
	InfoSchemaTables: InfoSchemaTables{
		Databases: "*",
	},
	PerfSchemaEventsStatements: PerfSchemaEventsStatements{
		Limit:     250,
		TimeLimit: 86400,
		TextLimit: 120,
	},
	PerfSchemaFileInstances: PerfSchemaFileInstances{
		Filter:       ".*",
		RemovePrefix: "/var/lib/mysql",
	},
	PerfSchemaMemoryEvents: PerfSchemaMemoryEvents{
		RemovePrefix: "memory/",
	},
	Heartbeat: Heartbeat{
		Database: "heartbeat",
		Table:    "heartbeat",
	},
}

// Arguments controls the mysql component.
type Arguments struct {
	// DataSourceName to use to connect to MySQL.
	DataSourceName alloytypes.Secret `alloy:"data_source_name,attr,optional"`

	// Collectors to mark as enabled in addition to the default.
	EnableCollectors []string `alloy:"enable_collectors,attr,optional"`
	// Collectors to explicitly mark as disabled.
	DisableCollectors []string `alloy:"disable_collectors,attr,optional"`

	// Overrides the default set of enabled collectors with the given list.
	SetCollectors []string `alloy:"set_collectors,attr,optional"`

	// Collector-wide options
	LockWaitTimeout int  `alloy:"lock_wait_timeout,attr,optional"`
	LogSlowFilter   bool `alloy:"log_slow_filter,attr,optional"`

	// Collector-specific config options
	InfoSchemaProcessList      InfoSchemaProcessList      `alloy:"info_schema.processlist,block,optional"`
	InfoSchemaTables           InfoSchemaTables           `alloy:"info_schema.tables,block,optional"`
	PerfSchemaEventsStatements PerfSchemaEventsStatements `alloy:"perf_schema.eventsstatements,block,optional"`
	PerfSchemaFileInstances    PerfSchemaFileInstances    `alloy:"perf_schema.file_instances,block,optional"`
	PerfSchemaMemoryEvents     PerfSchemaMemoryEvents     `alloy:"perf_schema.memory_events,block,optional"`

	Heartbeat Heartbeat `alloy:"heartbeat,block,optional"`
	MySQLUser MySQLUser `alloy:"mysql.user,block,optional"`
}

// InfoSchemaProcessList configures the info_schema.processlist collector
type InfoSchemaProcessList struct {
	MinTime         int  `alloy:"min_time,attr,optional"`
	ProcessesByUser bool `alloy:"processes_by_user,attr,optional"`
	ProcessesByHost bool `alloy:"processes_by_host,attr,optional"`
}

// InfoSchemaTables configures the info_schema.tables collector
type InfoSchemaTables struct {
	Databases string `alloy:"databases,attr,optional"`
}

// PerfSchemaEventsStatements configures the perf_schema.eventsstatements collector
type PerfSchemaEventsStatements struct {
	Limit     int `alloy:"limit,attr,optional"`
	TimeLimit int `alloy:"time_limit,attr,optional"`
	TextLimit int `alloy:"text_limit,attr,optional"`
}

// PerfSchemaFileInstances configures the perf_schema.file_instances collector
type PerfSchemaFileInstances struct {
	Filter       string `alloy:"filter,attr,optional"`
	RemovePrefix string `alloy:"remove_prefix,attr,optional"`
}

// PerfSchemaMemoryEvents configures the perf_schema.memory_events collector
type PerfSchemaMemoryEvents struct {
	RemovePrefix string `alloy:"remove_prefix,attr,optional"`
}

// Heartbeat controls the heartbeat collector
type Heartbeat struct {
	Database string `alloy:"database,attr,optional"`
	Table    string `alloy:"table,attr,optional"`
	UTC      bool   `alloy:"utc,attr,optional"`
}

// MySQLUser controls the mysql.user collector
type MySQLUser struct {
	Privileges bool `alloy:"privileges,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	_, err := mysql.ParseDSN(string(a.DataSourceName))
	if err != nil {
		return err
	}
	return nil
}

func (a *Arguments) Convert() *mysqld_exporter.Config {
	return &mysqld_exporter.Config{
		DataSourceName:                       config_util.Secret(a.DataSourceName),
		EnableCollectors:                     a.EnableCollectors,
		DisableCollectors:                    a.DisableCollectors,
		SetCollectors:                        a.SetCollectors,
		LockWaitTimeout:                      a.LockWaitTimeout,
		LogSlowFilter:                        a.LogSlowFilter,
		InfoSchemaProcessListMinTime:         a.InfoSchemaProcessList.MinTime,
		InfoSchemaProcessListProcessesByUser: a.InfoSchemaProcessList.ProcessesByUser,
		InfoSchemaProcessListProcessesByHost: a.InfoSchemaProcessList.ProcessesByHost,
		InfoSchemaTablesDatabases:            a.InfoSchemaTables.Databases,
		PerfSchemaEventsStatementsLimit:      a.PerfSchemaEventsStatements.Limit,
		PerfSchemaEventsStatementsTimeLimit:  a.PerfSchemaEventsStatements.TimeLimit,
		PerfSchemaEventsStatementsTextLimit:  a.PerfSchemaEventsStatements.TextLimit,
		PerfSchemaFileInstancesFilter:        a.PerfSchemaFileInstances.Filter,
		PerfSchemaFileInstancesRemovePrefix:  a.PerfSchemaFileInstances.RemovePrefix,
		PerfSchemaMemoryEventsRemovePrefix:   a.PerfSchemaMemoryEvents.RemovePrefix,
		HeartbeatDatabase:                    a.Heartbeat.Database,
		HeartbeatTable:                       a.Heartbeat.Table,
		HeartbeatUTC:                         a.Heartbeat.UTC,
		MySQLUserPrivileges:                  a.MySQLUser.Privileges,
	}
}
