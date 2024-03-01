package mysql

import (
	"github.com/go-sql-driver/mysql"
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/mysqld_exporter"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.mysql",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "mysql"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
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
	DataSourceName rivertypes.Secret `river:"data_source_name,attr,optional"`

	// Collectors to mark as enabled in addition to the default.
	EnableCollectors []string `river:"enable_collectors,attr,optional"`
	// Collectors to explicitly mark as disabled.
	DisableCollectors []string `river:"disable_collectors,attr,optional"`

	// Overrides the default set of enabled collectors with the given list.
	SetCollectors []string `river:"set_collectors,attr,optional"`

	// Collector-wide options
	LockWaitTimeout int  `river:"lock_wait_timeout,attr,optional"`
	LogSlowFilter   bool `river:"log_slow_filter,attr,optional"`

	// Collector-specific config options
	InfoSchemaProcessList      InfoSchemaProcessList      `river:"info_schema.processlist,block,optional"`
	InfoSchemaTables           InfoSchemaTables           `river:"info_schema.tables,block,optional"`
	PerfSchemaEventsStatements PerfSchemaEventsStatements `river:"perf_schema.eventsstatements,block,optional"`
	PerfSchemaFileInstances    PerfSchemaFileInstances    `river:"perf_schema.file_instances,block,optional"`
	PerfSchemaMemoryEvents     PerfSchemaMemoryEvents     `river:"perf_schema.memory_events,block,optional"`

	Heartbeat Heartbeat `river:"heartbeat,block,optional"`
	MySQLUser MySQLUser `river:"mysql.user,block,optional"`
}

// InfoSchemaProcessList configures the info_schema.processlist collector
type InfoSchemaProcessList struct {
	MinTime         int  `river:"min_time,attr,optional"`
	ProcessesByUser bool `river:"processes_by_user,attr,optional"`
	ProcessesByHost bool `river:"processes_by_host,attr,optional"`
}

// InfoSchemaTables configures the info_schema.tables collector
type InfoSchemaTables struct {
	Databases string `river:"databases,attr,optional"`
}

// PerfSchemaEventsStatements configures the perf_schema.eventsstatements collector
type PerfSchemaEventsStatements struct {
	Limit     int `river:"limit,attr,optional"`
	TimeLimit int `river:"time_limit,attr,optional"`
	TextLimit int `river:"text_limit,attr,optional"`
}

// PerfSchemaFileInstances configures the perf_schema.file_instances collector
type PerfSchemaFileInstances struct {
	Filter       string `river:"filter,attr,optional"`
	RemovePrefix string `river:"remove_prefix,attr,optional"`
}

// PerfSchemaMemoryEvents configures the perf_schema.memory_events collector
type PerfSchemaMemoryEvents struct {
	RemovePrefix string `river:"remove_prefix,attr,optional"`
}

// Heartbeat controls the heartbeat collector
type Heartbeat struct {
	Database string `river:"database,attr,optional"`
	Table    string `river:"table,attr,optional"`
	UTC      bool   `river:"utc,attr,optional"`
}

// MySQLUser controls the mysql.user collector
type MySQLUser struct {
	Privileges bool `river:"privileges,attr,optional"`
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
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
