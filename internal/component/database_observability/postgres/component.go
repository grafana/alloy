package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

const name = "database_observability.postgres"

const selectEngineVersion = `SHOW server_version`

func init() {
	component.Register(component.Registration{
		Name:      name,
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

var (
	_ syntax.Defaulter = (*Arguments)(nil)
	_ syntax.Validator = (*Arguments)(nil)
)

type Arguments struct {
	DataSourceName    alloytypes.Secret   `alloy:"data_source_name,attr"`
	ForwardTo         []loki.LogsReceiver `alloy:"forward_to,attr"`
	EnableCollectors  []string            `alloy:"enable_collectors,attr,optional"`
	DisableCollectors []string            `alloy:"disable_collectors,attr,optional"`

	QuerySampleArguments    QuerySampleArguments    `alloy:"query_samples,block,optional"`
	QueryTablesArguments    QueryTablesArguments    `alloy:"query_details,block,optional"`
	ConnectionInfoArguments ConnectionInfoArguments `alloy:"connection_info,block,optional"`
}

type QuerySampleArguments struct {
	CollectInterval       time.Duration `alloy:"collect_interval,attr,optional"`
	DisableQueryRedaction bool          `alloy:"disable_query_redaction,attr,optional"`
}

type QueryTablesArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

type ConnectionInfoArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

var DefaultArguments = Arguments{
	QuerySampleArguments: QuerySampleArguments{
		CollectInterval:       15 * time.Second,
		DisableQueryRedaction: false,
	},
	QueryTablesArguments: QueryTablesArguments{
		CollectInterval: 1 * time.Minute,
	},
	ConnectionInfoArguments: ConnectionInfoArguments{
		CollectInterval: 15 * time.Second,
	},
}

func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Validate() error {
	_, err := pq.ParseURL(string(a.DataSourceName))
	if err != nil {
		return err
	}
	return nil
}

type Exports struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}

var (
	_ component.Component       = (*Component)(nil)
	_ http_service.Component    = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

type Collector interface {
	Name() string
	Start(context.Context) error
	Stopped() bool
	Stop()
}

type Component struct {
	opts         component.Options
	args         Arguments
	mut          sync.RWMutex
	receivers    []loki.LogsReceiver
	handler      loki.LogsReceiver
	registry     *prometheus.Registry
	baseTarget   discovery.Target
	collectors   []Collector
	instanceKey  string
	dbConnection *sql.DB
	healthErr    *atomic.String
	openSQL      func(driverName, dataSourceName string) (*sql.DB, error)
}

func newWithOpen(opts component.Options, args Arguments, openFn func(driverName, dataSourceName string) (*sql.DB, error)) (*Component, error) {
	c := &Component{
		opts:      opts,
		args:      args,
		receivers: args.ForwardTo,
		handler:   loki.NewLogsReceiver(),
		registry:  prometheus.NewRegistry(),
		healthErr: atomic.NewString(""),
		openSQL:   openFn,
	}

	instance, err := instanceKey(string(args.DataSourceName))
	if err != nil {
		return nil, err
	}
	c.instanceKey = instance

	baseTarget, err := c.getBaseTarget()
	if err != nil {
		return nil, err
	}
	c.baseTarget = baseTarget

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func New(opts component.Options, args Arguments) (*Component, error) {
	return newWithOpen(opts, args, sql.Open)
}

func (c *Component) Run(ctx context.Context) error {
	defer func() {
		level.Info(c.opts.Logger).Log("msg", name+" component shutting down, stopping collectors")
		c.mut.RLock()
		for _, collector := range c.collectors {
			collector.Stop()
		}
		if c.dbConnection != nil {
			c.dbConnection.Close()
		}
		c.mut.RUnlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler.Chan():
			c.mut.RLock()
			for _, receiver := range c.receivers {
				receiver.Chan() <- entry
			}
			c.mut.RUnlock()
		}
	}
}

func (c *Component) getBaseTarget() (discovery.Target, error) {
	data, err := c.opts.GetServiceData(http_service.ServiceName)
	if err != nil {
		return discovery.EmptyTarget, fmt.Errorf("failed to get HTTP information: %w", err)
	}
	httpData := data.(http_service.Data)

	return discovery.NewTargetFromMap(map[string]string{
		model.AddressLabel:     httpData.MemoryListenAddr,
		model.SchemeLabel:      "http",
		model.MetricsPathLabel: path.Join(httpData.HTTPPathForComponent(c.opts.ID), "metrics"),
		"instance":             c.instanceKey,
		"job":                  database_observability.JobName,
	}), nil
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.opts.OnStateChange(Exports{
		Targets: []discovery.Target{c.baseTarget},
	})

	for _, collector := range c.collectors {
		collector.Stop()
	}
	c.collectors = nil

	if c.dbConnection != nil {
		c.dbConnection.Close()
	}

	c.args = args.(Arguments)

	if err := c.startCollectors(); err != nil {
		c.healthErr.Store(err.Error())
		return nil
	}

	c.healthErr.Store("")
	return nil
}

func enableOrDisableCollectors(a Arguments) map[string]bool {
	// configurable collectors and their default enabled/disabled value
	collectors := map[string]bool{
		collector.QueryTablesName: false,
		collector.QuerySampleName: false,
		collector.SchemaTableName: false,
	}

	for _, disabled := range a.DisableCollectors {
		if _, ok := collectors[disabled]; ok {
			collectors[disabled] = false
		}
	}
	for _, enabled := range a.EnableCollectors {
		if _, ok := collectors[enabled]; ok {
			collectors[enabled] = true
		}
	}

	return collectors
}

func (c *Component) startCollectors() error {
	// Connection Info collector is always enabled
	// value 1 on success and 0 on failure to establish the connection and check the engine version
	startConnInfo := func(engineVersion string) {
		ciCollector, ciErr := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
			DSN:           string(c.args.DataSourceName),
			Registry:      c.registry,
			EngineVersion: engineVersion,
			CheckInterval: c.args.ConnectionInfoArguments.CollectInterval,
			DB:            c.dbConnection,
			HealthErr:     c.healthErr,
		})
		if ciErr != nil {
			level.Error(c.opts.Logger).Log("msg", fmt.Errorf("failed to create %s collector: %w", collector.ConnectionInfoName, ciErr).Error())
			return
		}
		if err := ciCollector.Start(context.Background()); err != nil {
			level.Error(c.opts.Logger).Log("msg", fmt.Errorf("failed to start %s collector: %w", collector.ConnectionInfoName, err).Error())
			return
		}
		c.collectors = append(c.collectors, ciCollector)
	}

	dbConnection, err := c.openSQL("postgres", string(c.args.DataSourceName))
	if err != nil {
		err = fmt.Errorf("failed to start collectors: failed to open Postgres connection: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("")
		return err
	}

	if dbConnection == nil {
		err = fmt.Errorf("failed to start collectors: nil DB connection")
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("")
		return err
	}
	if err = dbConnection.Ping(); err != nil {
		err = fmt.Errorf("failed to start collectors: failed to ping Postgres: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("")
		return err
	}
	c.dbConnection = dbConnection

	rs := dbConnection.QueryRowContext(context.Background(), selectEngineVersion)
	err = rs.Err()
	if err != nil {
		err = fmt.Errorf("failed to query engine version for collector %s: %w", collector.ConnectionInfoName, err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("")
		return err
	}

	var engineVersion string
	if err := rs.Scan(&engineVersion); err != nil {
		err = fmt.Errorf("failed to scan engine version for collector %s: %w", collector.ConnectionInfoName, err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("")
		return err
	}

	// Start the connection_info collector first, so we can report the connection status.
	startConnInfo(engineVersion)

	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), c.instanceKey)

	collectors := enableOrDisableCollectors(c.args)
	// Best-effort start: attempt to build/start all enabled collectors and aggregate errors.
	var startErrors []string

	// logStartError wraps, logs, and records collector start errors consistently.
	logStartError := func(collectorName, action string, err error) {
		wrapped := fmt.Errorf("failed to %s %s collector: %w", action, collectorName, err)
		level.Error(c.opts.Logger).Log("msg", wrapped.Error())
		startErrors = append(startErrors, wrapped.Error())
	}

	if collectors[collector.QueryTablesName] {
		qCollector, err := collector.NewQueryTables(collector.QueryTablesArguments{
			DB:              dbConnection,
			CollectInterval: c.args.QueryTablesArguments.CollectInterval,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.QueryTablesName, "create", err)
		} else {
			if err := qCollector.Start(context.Background()); err != nil {
				logStartError(collector.QueryTablesName, "start", err)
			} else {
				c.collectors = append(c.collectors, qCollector)
			}
		}
	}

	if collectors[collector.QuerySampleName] {
		aCollector, err := collector.NewQuerySample(collector.QuerySampleArguments{
			DB:                    dbConnection,
			CollectInterval:       c.args.QuerySampleArguments.CollectInterval,
			EntryHandler:          entryHandler,
			Logger:                c.opts.Logger,
			DisableQueryRedaction: c.args.QuerySampleArguments.DisableQueryRedaction,
		})
		if err != nil {
			logStartError(collector.QuerySampleName, "create", err)
		} else {
			if err := aCollector.Start(context.Background()); err != nil {
				logStartError(collector.QuerySampleName, "start", err)
			} else {
				c.collectors = append(c.collectors, aCollector)
			}
		}
	}

	if collectors[collector.SchemaTableName] {
		stCollector, err := collector.NewSchemaTable(collector.SchemaTableArguments{
			DB:           dbConnection,
			EntryHandler: entryHandler,
			Logger:       c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.SchemaTableName, "create", err)
		} else {
			if err := stCollector.Start(context.Background()); err != nil {
				logStartError(collector.SchemaTableName, "start", err)
			} else {
				c.collectors = append(c.collectors, stCollector)
			}
		}
	}
	if len(startErrors) > 0 {
		return fmt.Errorf("failed to start collectors: %s", strings.Join(startErrors, "; "))
	}
	return nil
}

func (c *Component) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

func (c *Component) CurrentHealth() component.Health {
	if err := c.healthErr.Load(); err != "" {
		return component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    err,
			UpdateTime: time.Now(),
		}
	}

	var unhealthyCollectors []string

	c.mut.RLock()
	for _, collector := range c.collectors {
		if collector.Stopped() {
			unhealthyCollectors = append(unhealthyCollectors, collector.Name())
		}
	}
	c.mut.RUnlock()

	if len(unhealthyCollectors) > 0 {
		return component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    "One or more collectors are unhealthy: [" + strings.Join(unhealthyCollectors, ", ") + "]",
			UpdateTime: time.Now(),
		}
	}

	return component.Health{
		Health:     component.HealthTypeHealthy,
		Message:    "All collectors are healthy",
		UpdateTime: time.Now(),
	}
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

func addLokiLabels(entryHandler loki.EntryHandler, instanceKey string) loki.EntryHandler {
	entryHandler = loki.AddLabelsMiddleware(model.LabelSet{
		"job":      database_observability.JobName,
		"instance": model.LabelValue(instanceKey),
	}).Wrap(entryHandler)

	return entryHandler
}
