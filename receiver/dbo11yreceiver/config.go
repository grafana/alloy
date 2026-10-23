package dbo11yreceiver

import (
	"errors"
	"fmt"
	"time"

	pgconfig "github.com/prometheus-community/postgres_exporter/config"
	"go.opentelemetry.io/collector/config/configopaque"

	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/dbinstance"
	"github.com/grafana/alloy/internal/static/integrations/postgres_exporter"
)

// Config configures the db-o11y receiver. One receiver monitors one or more
// Postgres databases.
type Config struct {
	Databases []DatabaseConfig `mapstructure:"databases"`

	EnableCollectors   []string `mapstructure:"enable_collectors"`
	DisableCollectors  []string `mapstructure:"disable_collectors"`
	ExcludeDatabases   []string `mapstructure:"exclude_databases"`
	ExcludeUsers       []string `mapstructure:"exclude_users"`
	ExcludeCurrentUser bool     `mapstructure:"exclude_current_user"`

	QuerySamples  QuerySamplesConfig `mapstructure:"query_samples"`
	QueryDetails  QueryDetailsConfig `mapstructure:"query_details"`
	SchemaDetails IntervalConfig     `mapstructure:"schema_details"`
	ExplainPlans  ExplainPlansConfig `mapstructure:"explain_plans"`
	HealthCheck   IntervalConfig     `mapstructure:"health_check"`
	Logs          LogsConfig         `mapstructure:"logs"`
	Exporter      ExporterConfig     `mapstructure:"prometheus_exporter"`

	// ScrapeInterval is how often each database's registry is gathered.
	ScrapeInterval time.Duration `mapstructure:"scrape_interval"`
	// ReconnectInterval is how often an unconnected database is retried.
	ReconnectInterval time.Duration `mapstructure:"reconnect_interval"`
}

// DatabaseConfig is one monitored database.
type DatabaseConfig struct {
	// Name is optional. It prefixes the database's collectors in health messages.
	Name           string               `mapstructure:"name"`
	DataSourceName configopaque.String  `mapstructure:"data_source_name"`
	CloudProvider  *CloudProviderConfig `mapstructure:"cloud_provider"`
}

type CloudProviderConfig struct {
	AWS *struct {
		ARN string `mapstructure:"arn"`
	} `mapstructure:"aws"`
	Azure *struct {
		SubscriptionID string `mapstructure:"subscription_id"`
		ResourceGroup  string `mapstructure:"resource_group"`
		ServerName     string `mapstructure:"server_name"`
	} `mapstructure:"azure"`
	GCP *struct {
		ConnectionName string `mapstructure:"connection_name"`
	} `mapstructure:"gcp"`
}

type QuerySamplesConfig struct {
	CollectInterval               time.Duration `mapstructure:"collect_interval"`
	DisableQueryRedaction         bool          `mapstructure:"disable_query_redaction"`
	EnablePreClassifiedWaitEvents bool          `mapstructure:"enable_pre_classified_wait_events"`
}

type QueryDetailsConfig struct {
	CollectInterval time.Duration `mapstructure:"collect_interval"`
	StatementsLimit int           `mapstructure:"statements_limit"`
}

type ExplainPlansConfig struct {
	CollectInterval time.Duration `mapstructure:"collect_interval"`
	PerCollectRatio float64       `mapstructure:"per_collect_ratio"`
}

type IntervalConfig struct {
	CollectInterval time.Duration `mapstructure:"collect_interval"`
}

type LogsConfig struct {
	EnableErrorLogsProcessing bool `mapstructure:"enable_error_logs_processing"`
}

// ExporterConfig configures the embedded postgres_exporter.
type ExporterConfig struct {
	// Disabled turns the embedded exporter off: no pg_* metrics are produced.
	Disabled                bool                 `mapstructure:"disabled"`
	DisableSettingsMetrics  bool                 `mapstructure:"disable_settings_metrics"`
	DisableDefaultMetrics   bool                 `mapstructure:"disable_default_metrics"`
	CustomQueriesConfigPath string               `mapstructure:"custom_queries_config_path"`
	EnabledCollectors       []string             `mapstructure:"enabled_collectors"`
	StatStatements          StatStatementsConfig `mapstructure:"stat_statements"`
}

type StatStatementsConfig struct {
	IncludeQuery     bool     `mapstructure:"include_query"`
	QueryLength      uint     `mapstructure:"query_length"`
	Limit            uint     `mapstructure:"limit"`
	ExcludeDatabases []string `mapstructure:"exclude_databases"`
	ExcludeUsers     []string `mapstructure:"exclude_users"`
}

// createDefaultConfig mirrors defaultArguments() in
// database_observability.postgres.
func createDefaultConfig() *Config {
	return &Config{
		ExcludeDatabases:   database_observability.DefaultExcludedDatabases(),
		ExcludeUsers:       database_observability.DefaultExcludedUsers(),
		ExcludeCurrentUser: true,
		QuerySamples:       QuerySamplesConfig{CollectInterval: 10 * time.Second},
		QueryDetails:       QueryDetailsConfig{CollectInterval: time.Minute, StatementsLimit: 100},
		SchemaDetails:      IntervalConfig{CollectInterval: time.Minute},
		ExplainPlans:       ExplainPlansConfig{CollectInterval: time.Minute, PerCollectRatio: 1.0},
		HealthCheck:        IntervalConfig{CollectInterval: time.Hour},
		ScrapeInterval:     15 * time.Second,
		// Same period as the Alloy component's reconnect ticker.
		ReconnectInterval: 30 * time.Second,
	}
}

func (cfg *Config) Validate() error {
	if len(cfg.Databases) == 0 {
		return errors.New("at least one entry in databases is required")
	}
	if cfg.ScrapeInterval <= 0 {
		return errors.New("scrape_interval must be positive")
	}
	if cfg.ReconnectInterval <= 0 {
		return errors.New("reconnect_interval must be positive")
	}
	keys := make(map[string]int, len(cfg.Databases))
	for i, db := range cfg.Databases {
		key, err := dbinstance.InstanceKeyFromDSN(string(db.DataSourceName))
		if err != nil {
			return fmt.Errorf("databases[%d]: %w", i, err)
		}
		if other, ok := keys[key]; ok {
			return fmt.Errorf("databases[%d] and databases[%d] resolve to the same server %q", other, i, key)
		}
		keys[key] = i
		if err := dbinstance.ValidateCloudProvider(db.CloudProvider.toDBInstance()); err != nil {
			return fmt.Errorf("databases[%d]: %w", i, err)
		}
	}
	if !cfg.Exporter.Disabled {
		e := cfg.Exporter
		if err := postgres_exporter.ValidateMetricSelection(e.DisableDefaultMetrics, e.CustomQueriesConfigPath, e.EnabledCollectors); err != nil {
			return fmt.Errorf("prometheus_exporter: %w", err)
		}
	}
	return nil
}

// instanceConfig maps the receiver config for one database onto dbinstance.
func (cfg *Config) instanceConfig(db DatabaseConfig) dbinstance.Config {
	ic := dbinstance.Config{
		Name:               db.Name,
		DSN:                string(db.DataSourceName),
		CloudProvider:      db.CloudProvider.toDBInstance(),
		ExcludeDatabases:   cfg.ExcludeDatabases,
		ExcludeUsers:       cfg.ExcludeUsers,
		ExcludeCurrentUser: cfg.ExcludeCurrentUser,
		Collectors:         cfg.collectors(),
		QuerySamples: dbinstance.QuerySampleArguments{
			CollectInterval:               cfg.QuerySamples.CollectInterval,
			DisableQueryRedaction:         cfg.QuerySamples.DisableQueryRedaction,
			EnablePreClassifiedWaitEvents: cfg.QuerySamples.EnablePreClassifiedWaitEvents,
		},
		QueryDetails: dbinstance.QueryDetailsArguments{
			CollectInterval: cfg.QueryDetails.CollectInterval,
			StatementsLimit: cfg.QueryDetails.StatementsLimit,
		},
		SchemaDetails: dbinstance.SchemaDetailsArguments{CollectInterval: cfg.SchemaDetails.CollectInterval},
		ExplainPlans: dbinstance.ExplainPlansArguments{
			CollectInterval: cfg.ExplainPlans.CollectInterval,
			PerCollectRatio: cfg.ExplainPlans.PerCollectRatio,
		},
		HealthCheck: dbinstance.HealthCheckArguments{CollectInterval: cfg.HealthCheck.CollectInterval},
		Logs:        dbinstance.LogsArguments{EnableErrorLogsProcessing: cfg.Logs.EnableErrorLogsProcessing},
	}
	if !cfg.Exporter.Disabled {
		exp := cfg.Exporter.upstreamConfig(ic.DSN, cfg.ExcludeDatabases)
		ic.Exporter = &exp
	}
	return ic
}

// collectors mirrors enableOrDisableCollectors in database_observability.postgres.
// POC-HACK: copied defaults, which can drift from the component's. A real
// version moves the default set into dbinstance for both callers.
func (cfg *Config) collectors() map[string]bool {
	collectors := map[string]bool{
		collector.QueryDetailsCollector:  true,
		collector.QuerySamplesCollector:  true,
		collector.SchemaDetailsCollector: true,
		collector.ExplainPlanCollector:   true,
		collector.TableStatsCollector:    false,
		collector.IndexStatsCollector:    false,
	}
	for _, name := range cfg.DisableCollectors {
		if _, ok := collectors[name]; ok {
			collectors[name] = false
		}
	}
	for _, name := range cfg.EnableCollectors {
		if _, ok := collectors[name]; ok {
			collectors[name] = true
		}
	}
	return collectors
}

// upstreamConfig mirrors resolveExporterArgs and exporterConfig in
// database_observability.postgres, then reuses the static integration's
// mapping onto upstream's config (F-4).
// POC-HACK: the stat_statements-only default and the limit of 100 are copied
// from the component. A real version moves them next to UpstreamConfig.
func (e ExporterConfig) upstreamConfig(dsn string, excludeDatabases []string) pgconfig.Config {
	enabled := e.EnabledCollectors
	if !e.DisableDefaultMetrics && len(enabled) == 0 {
		enabled = []string{"stat_statements"}
	}
	flags := postgres_exporter.StatStatementFlags{
		IncludeQuery:     e.StatStatements.IncludeQuery,
		QueryLength:      e.StatStatements.QueryLength,
		Limit:            e.StatStatements.Limit,
		ExcludeDatabases: e.StatStatements.ExcludeDatabases,
		ExcludeUsers:     e.StatStatements.ExcludeUsers,
	}
	if flags.Limit == 0 {
		flags.Limit = 100
	}
	c := postgres_exporter.Config{
		DisableSettingsMetrics: e.DisableSettingsMetrics,
		AutodiscoverDatabases:  true,
		ExcludeDatabases:       excludeDatabases,
		DisableDefaultMetrics:  e.DisableDefaultMetrics,
		QueryPath:              e.CustomQueriesConfigPath,
		EnabledCollectors:      enabled,
		StatStatementFlags:     &flags,
	}
	return c.UpstreamConfig([]string{dsn})
}

func (c *CloudProviderConfig) toDBInstance() *dbinstance.CloudProvider {
	if c == nil {
		return nil
	}
	cp := &dbinstance.CloudProvider{}
	if c.AWS != nil {
		cp.AWS = &dbinstance.AWSCloudProviderInfo{ARN: c.AWS.ARN}
	}
	if c.Azure != nil {
		cp.Azure = &dbinstance.AzureCloudProviderInfo{
			SubscriptionID: c.Azure.SubscriptionID,
			ResourceGroup:  c.Azure.ResourceGroup,
			ServerName:     c.Azure.ServerName,
		}
	}
	if c.GCP != nil {
		cp.GCP = &dbinstance.GCPCloudProviderInfo{ConnectionName: c.GCP.ConnectionName}
	}
	return cp
}
