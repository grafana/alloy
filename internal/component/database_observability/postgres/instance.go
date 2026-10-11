package postgres

import (
	"database/sql"
	"fmt"
	"path"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax/alloytypes"
)

// databaseConfig is the normalized configuration of one monitored database,
// derived either from the component's top-level arguments (single-DSN form)
// or from one `database_instance` block.
type databaseConfig struct {
	name          string // empty in the top-level single-DSN form
	dsn           alloytypes.Secret
	cloudProvider *CloudProvider
	// targets holds the external exporter targets of the top-level single-DSN
	// form. It's always empty for `database_instance` blocks, which embed
	// their own postgres_exporter.
	targets []discovery.Target
}

// databaseConfigs returns one databaseConfig per monitored database. When no
// `database_instance` blocks are defined, the top-level arguments describe a single
// unnamed database.
func (a Arguments) databaseConfigs() []databaseConfig {
	if len(a.Databases) == 0 {
		return []databaseConfig{{
			dsn:           a.DataSourceName,
			cloudProvider: a.CloudProvider,
			targets:       a.Targets,
		}}
	}

	cfgs := make([]databaseConfig, 0, len(a.Databases))
	for _, db := range a.Databases {
		cfgs = append(cfgs, databaseConfig{
			name:          db.Name,
			dsn:           db.DataSourceName,
			cloudProvider: db.CloudProvider,
		})
	}
	return cfgs
}

// dbInstance holds the per-database state of the component: the identity,
// connection, collectors, and metrics registry of a single monitored
// Postgres server.
type dbInstance struct {
	cfg                databaseConfig
	instanceKey        string
	baseTarget         discovery.Target
	registry           *prometheus.Registry
	collectors         []Collector
	dbConnection       *sql.DB
	healthErr          *atomic.String
	exporterCollectors []prometheus.Collector

	// logsReceiver is the instance's internal logs receiver. The component's
	// receiver pump forwards entries from the database's exported receiver
	// into it, and the instance's logs collector drains it while running.
	logsReceiver loki.LogsReceiver

	// exportedTargets holds the relabeled targets of this instance. It is set
	// once a connection is established and server info is known (before
	// collectors start), since the relabeling rules need the server_id. An
	// instance whose collectors failed to start still exports its targets.
	exportedTargets []discovery.Target
}

func newDBInstance(opts component.Options, cfg databaseConfig) (*dbInstance, error) {
	key, err := instanceKey(string(cfg.dsn))
	if err != nil {
		return nil, err
	}

	baseTarget, err := getBaseTarget(opts, key, cfg.name)
	if err != nil {
		return nil, err
	}

	return &dbInstance{
		cfg:          cfg,
		instanceKey:  key,
		baseTarget:   baseTarget,
		registry:     prometheus.NewRegistry(),
		healthErr:    atomic.NewString(""),
		logsReceiver: loki.NewLogsReceiver(),
	}, nil
}

// cleanupExporterCollectors releases resources held by embedded exporter
// collectors: the postgres_exporter keeps one connection open per discovered
// database until its servers are closed. Callers must hold the component mut.
func (inst *dbInstance) cleanupExporterCollectors() {
	for _, col := range inst.exporterCollectors {
		if closable, ok := col.(interface{ CloseServers() }); ok {
			closable.CloseServers()
		}
		inst.registry.Unregister(col)
	}
	inst.exporterCollectors = nil
}

// metricsPath returns the HTTP path of a database's metrics endpoint,
// relative to the component's root HTTP path. The unnamed database of the
// top-level single-DSN form keeps the historical /metrics path.
func metricsPath(name string) string {
	if name == "" {
		return "/metrics"
	}
	return "/db/" + name + "/metrics"
}

// getBaseTarget returns the scrape target of the component's own metrics
// endpoint for a database instance.
func getBaseTarget(opts component.Options, instanceKey, name string) (discovery.Target, error) {
	data, err := opts.GetServiceData(http_service.ServiceName)
	if err != nil {
		return discovery.EmptyTarget, fmt.Errorf("failed to get HTTP information: %w", err)
	}
	httpData := data.(http_service.Data)

	return discovery.NewTargetFromMap(map[string]string{
		model.AddressLabel:     httpData.MemoryListenAddr,
		model.SchemeLabel:      "http",
		model.MetricsPathLabel: path.Join(httpData.HTTPPathForComponent(opts.ID), metricsPath(name)),
		"instance":             instanceKey,
		"job":                  database_observability.JobName,
	}), nil
}

// instanceKey returns network(hostname:port)/dbname of the Postgres server.
// This is the same key as used by the postgres static integration.
func instanceKey(dsn string) (string, error) {
	s, err := collector.ParseURL(dsn)
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
