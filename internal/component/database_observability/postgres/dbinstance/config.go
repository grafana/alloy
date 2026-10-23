// Package dbinstance runs db-o11y against one Postgres server without the
// Alloy runtime: one connection, the collectors, the embedded
// postgres_exporter, and the server_id that ties their output together.
package dbinstance

import (
	"database/sql"
	"log/slog"
	"time"

	pgconfig "github.com/prometheus-community/postgres_exporter/config"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
)

// Config is everything one Instance reads.
type Config struct {
	// Name is the database_instance label, empty in the single-DSN form.
	Name          string
	DSN           string
	CloudProvider *CloudProvider
	// Rules are the per-database relabel rules. Connect applies them to logs;
	// callers apply them to metric targets after MetricRules.
	Rules []*relabel.Config

	ExcludeDatabases   []string
	ExcludeUsers       []string
	ExcludeCurrentUser bool

	// Collectors maps each optional collector name to whether it runs.
	Collectors map[string]bool

	QuerySamples  QuerySampleArguments
	QueryDetails  QueryDetailsArguments
	SchemaDetails SchemaDetailsArguments
	ExplainPlans  ExplainPlansArguments
	HealthCheck   HealthCheckArguments
	Logs          LogsArguments

	// Exporter configures the embedded postgres_exporter. Nil means an
	// external exporter supplies the pg_* metrics.
	Exporter *pgconfig.Config
}

// Deps are the runtime dependencies of an Instance.
type Deps struct {
	Logger *slog.Logger
	// Registerer is unused for now: every metric lands in Registry().
	Registerer prometheus.Registerer
	// Entries receives the log entries of every collector, labeled.
	Entries loki.EntryHandler
	// OpenDB opens the database connection. Defaults to sql.Open.
	OpenDB func(driver, dsn string) (*sql.DB, error)
}

type CloudProvider struct {
	AWS   *AWSCloudProviderInfo   `alloy:"aws,block,optional"`
	Azure *AzureCloudProviderInfo `alloy:"azure,block,optional"`
	GCP   *GCPCloudProviderInfo   `alloy:"gcp,block,optional"`
}

type AWSCloudProviderInfo struct {
	ARN string `alloy:"arn,attr"`
}

type AzureCloudProviderInfo struct {
	SubscriptionID string `alloy:"subscription_id,attr"`
	ResourceGroup  string `alloy:"resource_group,attr"`
	ServerName     string `alloy:"server_name,attr,optional"`
}

type GCPCloudProviderInfo struct {
	ConnectionName string `alloy:"connection_name,attr"`
}

type QuerySampleArguments struct {
	CollectInterval       time.Duration `alloy:"collect_interval,attr,optional"`
	DisableQueryRedaction bool          `alloy:"disable_query_redaction,attr,optional"`
	// Deprecated: `query_samples.exclude_current_user` is deprecated in favour of the top-level setting.
	// When set (non-nil), it takes precedence over the top-level setting for the
	// query_samples collector only and preserves the legacy behaviour.
	ExcludeCurrentUser            *bool `alloy:"exclude_current_user,attr,optional"`
	EnablePreClassifiedWaitEvents bool  `alloy:"enable_pre_classified_wait_events,attr,optional"`
}

type QueryDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	StatementsLimit int           `alloy:"statements_limit,attr,optional"`
}

type SchemaDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`

	// Deprecated settings
	CacheEnabled *bool          `alloy:"cache_enabled,attr,optional"`
	CacheSize    *int           `alloy:"cache_size,attr,optional"`
	CacheTTL     *time.Duration `alloy:"cache_ttl,attr,optional"`
}

type ExplainPlansArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	PerCollectRatio float64       `alloy:"per_collect_ratio,attr,optional"`
}

type HealthCheckArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

type LogsArguments struct {
	EnableErrorLogsProcessing bool `alloy:"enable_error_logs_processing,attr,optional"`
}
