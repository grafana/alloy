package postgres

import (
	"database/sql"
	"fmt"
	"path"

	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/dbinstance"
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
	// rules holds the relabel rules applied to this database's metrics and
	// logs. It's always empty in the top-level single-DSN form, which monitors
	// a single database and so can be relabeled downstream instead.
	rules []*relabel.Config
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
			rules:         db.Rules,
		})
	}
	return cfgs
}

// instanceConfig maps the component arguments for one database onto dbinstance.
func instanceConfig(args Arguments, cfg databaseConfig) dbinstance.Config {
	ic := dbinstance.Config{
		Name:               cfg.name,
		DSN:                string(cfg.dsn),
		CloudProvider:      cfg.cloudProvider,
		Rules:              cfg.rules,
		ExcludeDatabases:   args.ExcludeDatabases,
		ExcludeUsers:       args.ExcludeUsers,
		ExcludeCurrentUser: args.ExcludeCurrentUser,
		Collectors:         enableOrDisableCollectors(args),
		QuerySamples:       args.QuerySampleArguments,
		QueryDetails:       args.QueryDetailsArguments,
		SchemaDetails:      args.SchemaDetailsArguments,
		ExplainPlans:       args.ExplainPlansArguments,
		HealthCheck:        args.HealthCheckArguments,
		Logs:               args.Logs,
	}
	if len(cfg.targets) == 0 {
		exporter := exporterConfig(resolveExporterArgs(args.PrometheusExporter), string(cfg.dsn), args.ExcludeDatabases)
		ic.Exporter = &exporter
	}
	return ic
}

// dbInstance is the component's view of one monitored database: the
// dbinstance.Instance plus what only the Alloy component needs (scrape
// targets and the health error from the last connect).
type dbInstance struct {
	*dbinstance.Instance
	cfg        databaseConfig
	baseTarget discovery.Target
	healthErr  *atomic.String

	// exportedTargets holds the relabeled targets of this instance. It is set
	// once a connection is established and server info is known, since the
	// relabeling rules need the server_id. An instance whose collectors
	// failed to start still exports its targets.
	exportedTargets []discovery.Target
}

func (c *Component) newDBInstance(args Arguments, cfg databaseConfig) (*dbInstance, error) {
	inst, err := dbinstance.New(instanceConfig(args, cfg), dbinstance.Deps{
		Logger:     c.opts.Logger,
		Registerer: c.opts.Registerer,
		Entries:    loki.NewEntryHandler(c.handler.Chan(), func() {}),
		// Late-bound so a replaced openSQL applies to existing instances.
		OpenDB: func(driver, dsn string) (*sql.DB, error) { return c.openSQL(driver, dsn) },
	})
	if err != nil {
		return nil, err
	}

	baseTarget, err := getBaseTarget(c.opts, inst.InstanceKey(), cfg.name)
	if err != nil {
		return nil, err
	}

	return &dbInstance{
		Instance:   inst,
		cfg:        cfg,
		baseTarget: baseTarget,
		healthErr:  atomic.NewString(""),
	}, nil
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
