// Package mysqld_exporter embeds https://github.com/prometheus/mysqld_exporter
package mysqld_exporter

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	config_util "github.com/prometheus/common/config"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-sql-driver/mysql"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	"github.com/prometheus/mysqld_exporter/collector"
)

// DefaultConfig holds the default settings for the mysqld_exporter integration.
var DefaultConfig = Config{
	LockWaitTimeout: 2,

	InfoSchemaProcessListProcessesByUser: true,
	InfoSchemaProcessListProcessesByHost: true,
	InfoSchemaTablesDatabases:            "*",

	PerfSchemaEventsStatementsLimit:     250,
	PerfSchemaEventsStatementsTimeLimit: 86400,
	PerfSchemaEventsStatementsTextLimit: 120,
	PerfSchemaFileInstancesFilter:       ".*",
	PerfSchemaFileInstancesRemovePrefix: "/var/lib/mysql",
	PerfSchemaMemoryEventsRemovePrefix:  "memory/",

	HeartbeatDatabase: "heartbeat",
	HeartbeatTable:    "heartbeat",
}

// Config controls the mysqld_exporter integration.
type Config struct {
	// DataSourceName to use to connect to MySQL.
	DataSourceName config_util.Secret `yaml:"data_source_name,omitempty"`

	// Collectors to mark as enabled in addition to the default.
	EnableCollectors []string `yaml:"enable_collectors,omitempty"`
	// Collectors to explicitly mark as disabled.
	DisableCollectors []string `yaml:"disable_collectors,omitempty"`

	// Overrides the default set of enabled collectors with the given list.
	SetCollectors []string `yaml:"set_collectors,omitempty"`

	// Collector-wide options
	LockWaitTimeout int  `yaml:"lock_wait_timeout,omitempty"`
	LogSlowFilter   bool `yaml:"log_slow_filter,omitempty"`

	// Collector-specific config options
	InfoSchemaProcessListMinTime         int    `yaml:"info_schema_processlist_min_time,omitempty"`
	InfoSchemaProcessListProcessesByUser bool   `yaml:"info_schema_processlist_processes_by_user,omitempty"`
	InfoSchemaProcessListProcessesByHost bool   `yaml:"info_schema_processlist_processes_by_host,omitempty"`
	InfoSchemaTablesDatabases            string `yaml:"info_schema_tables_databases,omitempty"`
	PerfSchemaEventsStatementsLimit      int    `yaml:"perf_schema_eventsstatements_limit,omitempty"`
	PerfSchemaEventsStatementsTimeLimit  int    `yaml:"perf_schema_eventsstatements_time_limit,omitempty"`
	PerfSchemaEventsStatementsTextLimit  int    `yaml:"perf_schema_eventsstatements_digtext_text_limit,omitempty"`
	PerfSchemaFileInstancesFilter        string `yaml:"perf_schema_file_instances_filter,omitempty"`
	PerfSchemaFileInstancesRemovePrefix  string `yaml:"perf_schema_file_instances_remove_prefix,omitempty"`
	PerfSchemaMemoryEventsRemovePrefix   string `yaml:"perf_schema_memory_events_remove_prefix,omitempty"`
	HeartbeatDatabase                    string `yaml:"heartbeat_database,omitempty"`
	HeartbeatTable                       string `yaml:"heartbeat_table,omitempty"`
	HeartbeatUTC                         bool   `yaml:"heartbeat_utc,omitempty"`
	MySQLUserPrivileges                  bool   `yaml:"mysql_user_privileges,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "mysqld_exporter"
}

// InstanceKey returns network(hostname:port)/dbname of the MySQL server.
func (c *Config) InstanceKey(_ string) (string, error) {
	m, err := mysql.ParseDSN(string(c.DataSourceName))
	if err != nil {
		return "", fmt.Errorf("failed to parse DSN: %w", err)
	}

	if m.Addr == "" {
		m.Addr = "localhost:3306"
	}
	if m.Net == "" {
		m.Net = "tcp"
	}

	return fmt.Sprintf("%s(%s)/%s", m.Net, m.Addr, m.DBName), nil
}

// NewIntegration converts this config into an instance of an integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("mysql"))
}

// New creates a new mysqld_exporter integration. The integration scrapes
// metrics from a mysqld process.
func New(log log.Logger, c *Config) (integrations.Integration, error) {
	dsn := c.DataSourceName
	if len(dsn) == 0 {
		dsn = config_util.Secret(os.Getenv("MYSQLD_EXPORTER_DATA_SOURCE_NAME"))
	}
	if len(dsn) == 0 {
		return nil, fmt.Errorf("cannot create mysqld_exporter; neither mysqld_exporter.data_source_name or $MYSQLD_EXPORTER_DATA_SOURCE_NAME is set")
	}

	scrapers := GetScrapers(c)
	logger := slog.New(logging.NewSlogGoKitHandler(log))
	exporter := collector.New(context.Background(), string(dsn), scrapers, logger, collector.Config{
		LockTimeout:   c.LockWaitTimeout,
		SlowLogFilter: c.LogSlowFilter,
	})

	level.Debug(log).Log("msg", "enabled mysqld_exporter scrapers")
	for _, scraper := range scrapers {
		level.Debug(log).Log("scraper", scraper.Name())
	}

	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithCollectors(exporter),
	), nil
}

// GetScrapers returns the set of *enabled* scrapers from the config.
// Configurable scrapers will have their configuration filled out matching the
// Config's settings.
func GetScrapers(c *Config) []collector.Scraper {
	scrapers := map[collector.Scraper]bool{
		&collector.ScrapeAutoIncrementColumns{}:                false,
		&collector.ScrapeBinlogSize{}:                          false,
		&collector.ScrapeClientStat{}:                          false,
		&collector.ScrapeEngineInnodbStatus{}:                  false,
		&collector.ScrapeEngineTokudbStatus{}:                  false,
		&collector.ScrapeGlobalStatus{}:                        true,
		&collector.ScrapeGlobalVariables{}:                     true,
		&collector.ScrapeInfoSchemaInnodbTablespaces{}:         false,
		&collector.ScrapeInnodbCmpMem{}:                        true,
		&collector.ScrapeInnodbCmp{}:                           true,
		&collector.ScrapeInnodbMetrics{}:                       false,
		&collector.ScrapePerfEventsStatementsSum{}:             false,
		&collector.ScrapePerfEventsWaits{}:                     false,
		&collector.ScrapePerfFileEvents{}:                      false,
		&collector.ScrapePerfIndexIOWaits{}:                    false,
		&collector.ScrapePerfReplicationApplierStatsByWorker{}: false,
		&collector.ScrapePerfReplicationGroupMemberStats{}:     false,
		&collector.ScrapePerfReplicationGroupMembers{}:         false,
		&collector.ScrapePerfTableIOWaits{}:                    false,
		&collector.ScrapePerfTableLockWaits{}:                  false,
		&collector.ScrapeQueryResponseTime{}:                   true,
		&collector.ScrapeReplicaHost{}:                         false,
		&collector.ScrapeSchemaStat{}:                          false,
		&collector.ScrapeSlaveHosts{}:                          false,
		&collector.ScrapeSlaveStatus{}:                         true,
		&collector.ScrapeTableStat{}:                           false,
		&collector.ScrapeUserStat{}:                            false,
		&collector.ScrapeSysUserSummary{}:                      false,

		// Collectors that have configuration
		&collector.ScrapeHeartbeat{
			Database: c.HeartbeatDatabase,
			Table:    c.HeartbeatTable,
			UTC:      c.HeartbeatUTC,
		}: false,

		&collector.ScrapePerfEventsStatements{
			Limit:           c.PerfSchemaEventsStatementsLimit,
			TimeLimit:       c.PerfSchemaEventsStatementsTimeLimit,
			DigestTextLimit: c.PerfSchemaEventsStatementsTextLimit,
		}: false,

		&collector.ScrapePerfFileInstances{
			Filter:       c.PerfSchemaFileInstancesFilter,
			RemovePrefix: c.PerfSchemaFileInstancesRemovePrefix,
		}: false,

		&collector.ScrapePerfMemoryEvents{
			RemovePrefix: c.PerfSchemaMemoryEventsRemovePrefix,
		}: false,

		&collector.ScrapeProcesslist{
			ProcessListMinTime:  c.InfoSchemaProcessListMinTime,
			ProcessesByHostFlag: c.InfoSchemaProcessListProcessesByHost,
			ProcessesByUserFlag: c.InfoSchemaProcessListProcessesByUser,
		}: false,

		&collector.ScrapeTableSchema{
			Databases: c.InfoSchemaTablesDatabases,
		}: false,

		&collector.ScrapeUser{
			Privileges: c.MySQLUserPrivileges,
		}: false,
	}

	// Override the defaults with the provided set of collectors if
	// set_collectors has at least one element in it.
	if len(c.SetCollectors) != 0 {
		customDefaults := map[string]struct{}{}
		for _, c := range c.SetCollectors {
			customDefaults[c] = struct{}{}
		}
		for scraper := range scrapers {
			_, enable := customDefaults[scraper.Name()]
			scrapers[scraper] = enable
		}
	}

	// Explicitly disable/enable specific collectors.
	for _, c := range c.DisableCollectors {
		for scraper := range scrapers {
			if scraper.Name() == c {
				scrapers[scraper] = false
				break
			}
		}
	}
	for _, c := range c.EnableCollectors {
		for scraper := range scrapers {
			if scraper.Name() == c {
				scrapers[scraper] = true
				break
			}
		}
	}

	enabledScrapers := []collector.Scraper{}
	for scraper, enabled := range scrapers {
		if enabled {
			enabledScrapers = append(enabledScrapers, scraper)
		}
	}
	return enabledScrapers
}
