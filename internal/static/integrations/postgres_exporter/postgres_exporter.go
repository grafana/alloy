// Package postgres_exporter embeds https://github.com/prometheus/postgres_exporter
package postgres_exporter

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	"github.com/lib/pq"
	"github.com/prometheus-community/postgres_exporter/collector"
	postgres_exporter "github.com/prometheus-community/postgres_exporter/exporter"
	config_util "github.com/prometheus/common/config"
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
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
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

	raw, err := pq.ParseURL(url)
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
func New(log log.Logger, cfg *Config) (integrations.Integration, error) {
	dsns, err := cfg.getDataSourceNames()
	if err != nil {
		return nil, err
	}

	logger := slog.New(logging.NewSlogGoKitHandler(log))

	e := postgres_exporter.NewExporter(
		dsns,
		logger,
		postgres_exporter.DisableDefaultMetrics(cfg.DisableDefaultMetrics),
		postgres_exporter.WithUserQueriesPath(cfg.QueryPath),
		postgres_exporter.DisableSettingsMetrics(cfg.DisableSettingsMetrics),
		postgres_exporter.AutoDiscoverDatabases(cfg.AutodiscoverDatabases),
		postgres_exporter.ExcludeDatabases(cfg.ExcludeDatabases),
		postgres_exporter.IncludeDatabases(strings.Join(cfg.IncludeDatabases, ",")),
		postgres_exporter.WithMetricPrefix("pg"),
	)

	if cfg.DisableDefaultMetrics {
		// Don't include the collector metrics if the default metrics are disabled.
		return integrations.NewCollectorIntegration(cfg.Name(), integrations.WithCollectors(e)), nil
	}

	// This is a hack to force the command line flag values for the stat_statements collector.
	// These flags are not exposed outside the package and cannot be mutated afterwards.
	if cfg.StatStatementFlags != nil && cfg.StatStatementFlags.IncludeQuery {
		includeQueryFlag := kingpin.CommandLine.GetFlag("collector.stat_statements.include_query")
		queryLengthFlag := kingpin.CommandLine.GetFlag("collector.stat_statements.query_length")

		if includeQueryFlag == nil || queryLengthFlag == nil {
			return nil, fmt.Errorf("failed to find collector.stat_statements.include_query or collector.stat_statements.query_length in postgres_exporter")
		}

		err := includeQueryFlag.Model().Value.Set("true")
		if err != nil {
			return nil, fmt.Errorf("failed to set include query flag using Kingpin: %w", err)
		}

		err = queryLengthFlag.Model().Value.Set(fmt.Sprintf("%d", cfg.StatStatementFlags.QueryLength))
		if err != nil {
			return nil, fmt.Errorf("failed to set query length flag using Kingpin: %w", err)
		}
	}
	if cfg.StatStatementFlags != nil && cfg.StatStatementFlags.Limit != 0 {
		limitFlag := kingpin.CommandLine.GetFlag("collector.stat_statements.limit")
		if limitFlag == nil {
			return nil, fmt.Errorf("failed to find collector.stat_statements.limit in postgres_exporter")
		}

		err := limitFlag.Model().Value.Set(fmt.Sprintf("%d", cfg.StatStatementFlags.Limit))
		if err != nil {
			return nil, fmt.Errorf("failed to set limit flag using Kingpin: %w", err)
		}
	}
	if cfg.StatStatementFlags != nil && len(cfg.StatStatementFlags.ExcludeDatabases) > 0 {
		excludeDatabasesFlag := kingpin.CommandLine.GetFlag("collector.stat_statements.exclude_databases")
		if excludeDatabasesFlag == nil {
			return nil, fmt.Errorf("failed to find collector.stat_statements.exclude_databases in postgres_exporter")
		}

		err := excludeDatabasesFlag.Model().Value.Set(strings.Join(cfg.StatStatementFlags.ExcludeDatabases, ","))
		if err != nil {
			return nil, fmt.Errorf("failed to set exclude databases flag using Kingpin: %w", err)
		}
	}
	if cfg.StatStatementFlags != nil && len(cfg.StatStatementFlags.ExcludeUsers) > 0 {
		excludeUsersFlag := kingpin.CommandLine.GetFlag("collector.stat_statements.exclude_users")
		if excludeUsersFlag == nil {
			return nil, fmt.Errorf("failed to find collector.stat_statements.exclude_users in postgres_exporter")
		}

		err := excludeUsersFlag.Model().Value.Set(strings.Join(cfg.StatStatementFlags.ExcludeUsers, ","))
		if err != nil {
			return nil, fmt.Errorf("failed to set exclude users flag using Kingpin: %w", err)
		}
	}

	// On top of the exporter's metrics, the postgres exporter also has metrics exposed via collector package.
	// However, these can only work for the first DSN provided. This matches the current implementation of the exporter.
	// TODO: Once https://github.com/prometheus-community/postgres_exporter/issues/999 is addressed, update the exporter
	// and change this.
	c, err := collector.NewPostgresCollector(
		logger,
		cfg.ExcludeDatabases,
		dsns[0],
		cfg.EnabledCollectors,
		collector.WithCollectionTimeout("10s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres_exporter collector: %w", err)
	}

	return integrations.NewCollectorIntegration(cfg.Name(), integrations.WithCollectors(e, c)), nil
}
