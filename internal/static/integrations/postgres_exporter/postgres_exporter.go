// Package postgres_exporter embeds https://github.com/prometheus/postgres_exporter
package postgres_exporter //nolint:golint

import (
	"fmt"
	"os"
	"strings"

	config_util "github.com/prometheus/common/config"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/static/integrations"
	integrations_v2 "github.com/grafana/agent/internal/static/integrations/v2"
	"github.com/grafana/agent/internal/static/integrations/v2/metricsutils"
	"github.com/lib/pq"
	"github.com/prometheus-community/postgres_exporter/exporter"
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
	raw, err := pq.ParseURL(url)
	if err != nil {
		return nil, err
	}

	res := map[string]string{}

	unescaper := strings.NewReplacer(`\'`, `'`, `\\`, `\`)

	for _, keypair := range strings.Split(raw, " ") {
		parts := strings.SplitN(keypair, "=", 2)
		if len(parts) != 2 {
			panic(fmt.Sprintf("unexpected keypair %s from pq", keypair))
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
func New(log log.Logger, c *Config) (integrations.Integration, error) {
	dsn, err := c.getDataSourceNames()
	if err != nil {
		return nil, err
	}

	e := exporter.NewExporter(
		dsn,
		log,
		exporter.DisableDefaultMetrics(c.DisableDefaultMetrics),
		exporter.WithUserQueriesPath(c.QueryPath),
		exporter.DisableSettingsMetrics(c.DisableSettingsMetrics),
		exporter.AutoDiscoverDatabases(c.AutodiscoverDatabases),
		exporter.ExcludeDatabases(strings.Join(c.ExcludeDatabases, ",")),
		exporter.IncludeDatabases(strings.Join(c.IncludeDatabases, ",")),
		exporter.MetricPrefix("pg"),
	)

	return integrations.NewCollectorIntegration(c.Name(), integrations.WithCollectors(e)), nil
}
