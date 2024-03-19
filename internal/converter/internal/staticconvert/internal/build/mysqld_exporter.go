package build

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/mysql"
	"github.com/grafana/agent/internal/static/integrations/mysqld_exporter"
	"github.com/grafana/river/rivertypes"
)

func (b *ConfigBuilder) appendMysqldExporter(config *mysqld_exporter.Config, instanceKey *string) discovery.Exports {
	args := toMysqldExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "mysql")
}

func toMysqldExporter(config *mysqld_exporter.Config) *mysql.Arguments {
	return &mysql.Arguments{
		DataSourceName:    rivertypes.Secret(config.DataSourceName),
		EnableCollectors:  config.EnableCollectors,
		DisableCollectors: config.DisableCollectors,
		SetCollectors:     config.SetCollectors,
		LockWaitTimeout:   config.LockWaitTimeout,
		LogSlowFilter:     config.LogSlowFilter,
		InfoSchemaProcessList: mysql.InfoSchemaProcessList{
			MinTime:         config.InfoSchemaProcessListMinTime,
			ProcessesByUser: config.InfoSchemaProcessListProcessesByUser,
			ProcessesByHost: config.InfoSchemaProcessListProcessesByHost,
		},
		InfoSchemaTables: mysql.InfoSchemaTables{
			Databases: config.InfoSchemaTablesDatabases,
		},
		PerfSchemaEventsStatements: mysql.PerfSchemaEventsStatements{
			Limit:     config.PerfSchemaEventsStatementsLimit,
			TimeLimit: config.PerfSchemaEventsStatementsTimeLimit,
			TextLimit: config.PerfSchemaEventsStatementsTextLimit,
		},
		PerfSchemaFileInstances: mysql.PerfSchemaFileInstances{
			Filter:       config.PerfSchemaFileInstancesFilter,
			RemovePrefix: config.PerfSchemaFileInstancesRemovePrefix,
		},
		PerfSchemaMemoryEvents: mysql.PerfSchemaMemoryEvents{
			RemovePrefix: config.PerfSchemaMemoryEventsRemovePrefix,
		},
		Heartbeat: mysql.Heartbeat{
			Database: config.HeartbeatDatabase,
			Table:    config.HeartbeatTable,
			UTC:      config.HeartbeatUTC,
		},
		MySQLUser: mysql.MySQLUser{
			Privileges: config.MySQLUserPrivileges,
		},
	}
}
