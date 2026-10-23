// Package postgres_exporter embeds https://github.com/prometheus/postgres_exporter
package postgres_exporter

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/prometheus-community/postgres_exporter/collector"
	pgconfig "github.com/prometheus-community/postgres_exporter/config"
	config_util "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// Config controls the postgres_exporter integration.
type Config struct {
	// DataSourceNames to use to connect to Postgres.
	DataSourceNames []config_util.Secret `yaml:"data_source_names,omitempty"`

	DisableSettingsMetrics bool     `yaml:"disable_settings_metrics,omitempty"`
	AutodiscoverDatabases  bool     `yaml:"autodiscover_databases,omitempty"`
	ExcludeDatabases       []string `yaml:"exclude_databases,omitempty"`
	IncludeDatabases       []string `yaml:"include_databases,omitempty"`
	DisableDefaultMetrics  bool     `yaml:"disable_default_metrics,omitempty"`
	QueryPath              string   `yaml:"query_path,omitempty"`

	//-- The fields below were not available in Grafana Agent Static. --

	// Instance is used by Alloy to specify the instance name manually. This is
	// only used when there are multiple DSNs provided.
	Instance string
	// EnabledCollectors is a list of additional collectors to enable. NOTE: Due to limitations of the postgres_exporter,
	// this is only used for the first DSN provided and only some collectors can be enabled/disabled this way. See the
	// user-facing docs for more information.
	EnabledCollectors  []string
	StatStatementFlags *StatStatementFlags
}

// Config for the stat_statement collector flags
type StatStatementFlags struct {
	IncludeQuery     bool
	QueryLength      uint
	Limit            uint
	ExcludeDatabases []string
	ExcludeUsers     []string
}

// Name returns the name of the integration this config is for.
func (c *Config) Name() string {
	return "postgres_exporter"
}

// NewIntegration converts this config into an instance of a configuration.
func (c *Config) NewIntegration(l *slog.Logger) (integrations.Integration, error) {
	return New(l, c)
}

// InstanceKey returns a simplified DSN of the first postgresql DSN, or an error if
// not exactly one DSN is provided.
func (c *Config) InstanceKey(_ string) (string, error) {
	dsn, err := c.getDataSourceNames()
	if err != nil {
		return "", err
	}
	if len(dsn) != 1 {
		if c.Instance != "" {
			return c.Instance, nil
		}
		// This should not be possible in Alloy, because `c.Instance` is always set.
		return "", fmt.Errorf("can't automatically determine a value for `instance` with %d DSN. either use 1 DSN or manually assign a value for `instance` in the integration config", len(dsn))
	}

	s, err := parsePostgresURL(dsn[0])
	if err != nil {
		return "", fmt.Errorf("cannot parse DSN: %w", err)
	}

	// Assign default values to s.
	//
	// PostgreSQL hostspecs can contain multiple host pairs. We'll assign a host
	// and port by default, but otherwise just use the hostname.
	if _, ok := s["host"]; !ok {
		s["host"] = "localhost"
		s["port"] = "5432"
	}

	hostport := s["host"]
	if p, ok := s["port"]; ok {
		hostport += fmt.Sprintf(":%s", p)
	}
	return fmt.Sprintf("postgresql://%s/%s", hostport, s["dbname"]), nil
}

func parsePostgresURL(url string) (map[string]string, error) {
	if url == "postgresql://" || url == "postgres://" {
		return map[string]string{}, nil
	}

	raw, err := pq.ParseURL(url) //nolint:staticcheck // pq.ParseURL is deprecated but needed to extract DSN key-value pairs
	if err != nil {
		return nil, err
	}

	res := map[string]string{}

	unescaper := strings.NewReplacer(`\'`, `'`, `\\`, `\`)

	for keypair := range strings.SplitSeq(raw, " ") {
		parts := strings.SplitN(keypair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected keypair %s from pq", keypair)
		}

		key := parts[0]
		value := parts[1]

		// Undo all the transformations ParseURL did: remove wrapping
		// quotes and then unescape the escaped characters.
		value = strings.TrimPrefix(value, "'")
		value = strings.TrimSuffix(value, "'")
		value = unescaper.Replace(value)

		res[key] = value
	}

	return res, nil
}

// getDataSourceNames loads data source names from the config or from the
// environment, if set.
func (c *Config) getDataSourceNames() ([]string, error) {
	dsn := c.DataSourceNames
	var stringDsn []string
	if len(dsn) == 0 {
		envDsn, present := os.LookupEnv("POSTGRES_EXPORTER_DATA_SOURCE_NAME")
		if !present {
			return nil, fmt.Errorf("cannot create postgres_exporter; neither postgres_exporter.data_source_name or $POSTGRES_EXPORTER_DATA_SOURCE_NAME is set")
		}
		stringDsn = append(stringDsn, strings.Split(envDsn, ",")...)
	} else {
		for _, d := range dsn {
			stringDsn = append(stringDsn, string(d))
		}
	}
	return stringDsn, nil
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("postgres"))
}

// New creates a new postgres_exporter integration. The integration scrapes
// metrics from a postgres process.
func New(logger *slog.Logger, cfg *Config) (integrations.Integration, error) {
	dsns, err := cfg.getDataSourceNames()
	if err != nil {
		return nil, err
	}

	validated, err := cfg.UpstreamConfig(dsns).Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid postgres_exporter config: %w", err)
	}
	// The collector package only runs against the first DSN; the exporter runs against all of them.
	runtime, err := collector.NewRuntime(validated, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres_exporter collector: %w", err)
	}

	run := func(ctx context.Context) error {
		<-ctx.Done()
		return runtime.Close()
	}

	return integrations.NewCollectorIntegration(
		cfg.Name(),
		integrations.WithLogger(logger),
		integrations.WithCollectors(runtime.Collectors()...),
		integrations.WithRunner(run),
	), nil
}

const collectionTimeout = 10 * time.Second

// UpstreamConfig maps c onto upstream's library config for dsns. The result still needs Validate.
func (c *Config) UpstreamConfig(dsns []string) pgconfig.Config {
	cfg := pgconfig.NewConfigWithDefaults()
	cfg.DataSourceNames = dsns
	cfg.MetricPrefix = "pg"
	cfg.CollectionTimeout = collectionTimeout
	// The fork never wrapped counters; upstream wraps them at 2^53 by default.
	cfg.WrapLargeCounters = false
	cfg.DisableDefaultMetrics = c.DisableDefaultMetrics
	cfg.AutoDiscoverDatabases = c.AutodiscoverDatabases
	cfg.UserQueriesPath = c.QueryPath
	cfg.ExcludeDatabases = c.ExcludeDatabases
	cfg.IncludeDatabases = c.IncludeDatabases
	cfg.Collectors = collectorStates(c.EnabledCollectors, c.DisableDefaultMetrics, c.DisableSettingsMetrics)
	cfg.PGStatStatements = upstreamStatStatements(c.StatStatementFlags)
	return cfg
}

// legacyDefaultCollectors are the upstream collectors that replaced the exporter's built-in
// queries, which used to run whenever default metrics were enabled, whatever enabled_collectors said.
var legacyDefaultCollectors = []string{
	pgconfig.CollectorSettings,
	pgconfig.CollectorStatActivity,
	pgconfig.CollectorStatArchiver,
	pgconfig.CollectorStatReplication,
}

// collectorStates builds upstream's Config.Collectors, which is merged over upstream's defaults.
// A non-empty enabled is an allowlist: every collector not in it is turned off.
func collectorStates(enabled []string, disableDefaultMetrics, disableSettingsMetrics bool) map[string]bool {
	states := pgconfig.DefaultCollectorConfig()
	if disableDefaultMetrics {
		for name := range states {
			states[name] = false
		}
		return states
	}
	if len(enabled) > 0 {
		for name := range states {
			states[name] = false
		}
		// Unknown names are kept so that upstream's Validate rejects them.
		for _, name := range enabled {
			states[name] = true
		}
	}
	for _, name := range legacyDefaultCollectors {
		states[name] = true
	}
	states[pgconfig.CollectorSettings] = !disableSettingsMetrics
	return states
}

// upstreamStatStatements maps the stat_statements flags onto upstream's config. Zero Limit and
// QueryLength mean "unset" here, but upstream rejects them, so they get upstream's defaults.
func upstreamStatStatements(f *StatStatementFlags) pgconfig.PGStatStatementsConfig {
	out := pgconfig.PGStatStatementsConfig{
		IncludeQuery: pgconfig.DefaultPGStatStatementsIncludeQuery,
		QueryLength:  pgconfig.DefaultPGStatStatementsQueryLength,
		Limit:        pgconfig.DefaultPGStatStatementsLimit,
	}
	if f == nil {
		return out
	}
	out.IncludeQuery = f.IncludeQuery
	out.ExcludeDatabases = slices.Clone(f.ExcludeDatabases)
	out.ExcludeUsers = slices.Clone(f.ExcludeUsers)
	if f.QueryLength > 0 {
		out.QueryLength = f.QueryLength
	}
	if f.Limit > 0 {
		out.Limit = f.Limit
	}
	return out
}
