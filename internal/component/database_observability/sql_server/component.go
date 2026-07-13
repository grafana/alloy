package sql_server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/sql_server/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

const name = "database_observability.sql_server"

// selectServerInfo returns a stable identifier for the server instance plus
// its product version, used to derive a server_id label and to expose the
// engine version on the connection_info metric.
const selectServerInfo = `
SELECT
    CONVERT(NVARCHAR(128), SERVERPROPERTY('ServerName')) AS server_name,
    CONVERT(NVARCHAR(128), SERVERPROPERTY('MachineName')) AS machine_name,
    CONVERT(NVARCHAR(128), SERVERPROPERTY('ProductVersion')) AS product_version`

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
	Targets           []discovery.Target  `alloy:"targets,attr,optional"`
	EnableCollectors  []string            `alloy:"enable_collectors,attr,optional"`
	DisableCollectors []string            `alloy:"disable_collectors,attr,optional"`
	ExcludeSchemas    []string            `alloy:"exclude_schemas,attr,optional"`
	ExcludeDatabases  []string            `alloy:"exclude_databases,attr,optional"`

	SchemaDetailsArguments SchemaDetailsArguments `alloy:"schema_details,block,optional"`
}

type SchemaDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

func defaultArguments() Arguments {
	return Arguments{
		ExcludeSchemas:   database_observability.DefaultExcludedSchemas(),
		ExcludeDatabases: database_observability.DefaultExcludedDatabases(),

		SchemaDetailsArguments: SchemaDetailsArguments{
			CollectInterval: 1 * time.Minute,
		},
	}
}

func (a *Arguments) SetToDefault() {
	*a = defaultArguments()
}

func (a *Arguments) Validate() error {
	_, err := msdsn.Parse(string(a.DataSourceName))
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
	handler      loki.LogsReceiver
	fanout       *loki.Fanout
	mut          sync.RWMutex
	registry     *prometheus.Registry
	baseTarget   discovery.Target
	collectors   []Collector
	instanceKey  string
	dbConnection *sql.DB
	healthErr    *atomic.String
	openSQL      func(driverName, dataSourceName string) (*sql.DB, error)
}

func New(opts component.Options, args Arguments) (*Component, error) {
	return newComponent(opts, args, sql.Open)
}

func newComponent(opts component.Options, args Arguments, openFn func(driverName, dataSourceName string) (*sql.DB, error)) (*Component, error) {
	c := &Component{
		opts:      opts,
		args:      args,
		fanout:    loki.NewFanout(args.ForwardTo),
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

func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.opts.Logger.Info(name + " component shutting down, stopping collectors")

		loki.Drain(c.handler, c.fanout, loki.DefaultDrainTimeout, func() {
			c.mut.Lock()
			defer c.mut.Unlock()

			for _, collector := range c.collectors {
				collector.Stop()
			}
			if c.dbConnection != nil {
				c.dbConnection.Close()
			}
		})
	}()

	var (
		wg                 sync.WaitGroup
		consumeCtx, cancel = context.WithCancel(context.Background())
	)

	wg.Go(func() { loki.Consume(consumeCtx, c.handler, c.fanout) })

	wg.Go(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.mut.RLock()
				hasCollectors := len(c.collectors) > 0
				c.mut.RUnlock()

				if !hasCollectors {
					c.opts.Logger.Debug("attempting to reconnect to database")
					if err := c.tryReconnect(ctx); err != nil {
						c.opts.Logger.Error("reconnection attempt failed", "err", err)
					}
				}
			}
		}
	})

	wg.Wait()
	return nil
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

func (c *Component) reportError(errorMsg string, err error) {
	c.opts.Logger.Error(fmt.Sprintf("%s: %+v", errorMsg, err))
	c.healthErr.Store(fmt.Sprintf("%s: %+v", errorMsg, err))
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.args = args.(Arguments)
	c.fanout.UpdateChildren(c.args.ForwardTo)

	if err := c.connectAndStartCollectors(context.Background()); err != nil {
		c.reportError("failed to connect", err)
		return nil
	}

	c.healthErr.Store("")
	return nil
}

func (c *Component) tryReconnect(ctx context.Context) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	if err := c.connectAndStartCollectors(ctx); err != nil {
		c.reportError("reconnection failed", err)
		return err
	}

	c.healthErr.Store("")
	return nil
}

// connectAndStartCollectors handles the full connection lifecycle:
// closes old connection, opens new one, queries server info, and starts collectors.
// Must be called with c.mut locked.
func (c *Component) connectAndStartCollectors(ctx context.Context) error {
	if c.dbConnection != nil {
		c.dbConnection.Close()
		c.dbConnection = nil
	}

	dbConnection, err := c.openSQL("sqlserver", string(c.args.DataSourceName))
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	if dbConnection == nil {
		return fmt.Errorf("nil DB connection")
	}

	if err = dbConnection.Ping(); err != nil {
		dbConnection.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}
	c.dbConnection = dbConnection

	rs := c.dbConnection.QueryRowContext(ctx, selectServerInfo)
	if err = rs.Err(); err != nil {
		return fmt.Errorf("failed to query engine version: %w", err)
	}

	var serverName, machineName, engineVersion sql.NullString
	if err := rs.Scan(&serverName, &machineName, &engineVersion); err != nil {
		return fmt.Errorf("failed to scan engine version: %w", err)
	}

	generatedServerID := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s", serverName.String, machineName.String))))

	c.args.Targets = append([]discovery.Target{c.baseTarget}, c.args.Targets...)
	targets := make([]discovery.Target, 0, len(c.args.Targets)+1)
	for _, t := range c.args.Targets {
		builder := discovery.NewTargetBuilderFrom(t)
		if relabel.ProcessBuilder(builder, database_observability.GetRelabelingRules(generatedServerID, nil)...) {
			targets = append(targets, builder.Target())
		}
	}

	c.opts.OnStateChange(Exports{
		Targets: targets,
	})

	for _, collector := range c.collectors {
		collector.Stop()
	}
	c.collectors = nil

	if err := c.startCollectors(generatedServerID, engineVersion.String); err != nil {
		return fmt.Errorf("failed to start collectors: %w", err)
	}

	return nil
}

func enableOrDisableCollectors(a Arguments) map[string]bool {
	collectors := map[string]bool{
		collector.SchemaDetailsCollector: true,
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

// startCollectors attempts to start all of the enabled collectors. If one or more collectors fail to start, their errors are reported.
func (c *Component) startCollectors(serverID string, engineVersion string) error {
	var startErrors []string

	logStartError := func(collectorName, action string, err error) {
		errorString := fmt.Sprintf("failed to %s %s collector: %+v", action, collectorName, err)
		c.opts.Logger.Error(errorString)
		startErrors = append(startErrors, errorString)
	}
	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), c.instanceKey, serverID)

	collectors := enableOrDisableCollectors(c.args)

	if collectors[collector.SchemaDetailsCollector] {
		stCollector, err := collector.NewSchemaDetails(collector.SchemaDetailsArguments{
			DB:               c.dbConnection,
			CollectInterval:  c.args.SchemaDetailsArguments.CollectInterval,
			ExcludeSchemas:   c.args.ExcludeSchemas,
			ExcludeDatabases: c.args.ExcludeDatabases,
			EntryHandler:     entryHandler,
			Logger:           c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.SchemaDetailsCollector, "create", err)
		} else {
			if err := stCollector.Start(context.Background()); err != nil {
				logStartError(collector.SchemaDetailsCollector, "start", err)
			}
			c.collectors = append(c.collectors, stCollector)
		}
	}

	// Connection Info collector is always enabled
	ciCollector, err := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
		DSN:           string(c.args.DataSourceName),
		Registry:      c.registry,
		EngineVersion: engineVersion,
		DB:            c.dbConnection,
	})
	if err != nil {
		logStartError(collector.ConnectionInfoName, "create", err)
	} else {
		if err := ciCollector.Start(context.Background()); err != nil {
			logStartError(collector.ConnectionInfoName, "start", err)
		}
		c.collectors = append(c.collectors, ciCollector)
	}

	if len(startErrors) > 0 {
		return fmt.Errorf("failed to start some collectors: %s", strings.Join(startErrors, ", "))
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

// instanceKey returns a connection-string-derived identifier for the SQL Server
// instance, in the form "host:port/database". This mirrors what mysql/postgres
// components use to label metrics and logs.
func instanceKey(dsn string) (string, error) {
	cfg, err := msdsn.Parse(dsn)
	if err != nil {
		return "", err
	}

	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.Port
	if port == 0 {
		port = 1433
	}

	return fmt.Sprintf("%s:%d/%s", host, port, cfg.Database), nil
}

func addLokiLabels(entryHandler loki.EntryHandler, instanceKey string, serverID string) loki.EntryHandler {
	entryHandler = loki.AddLabelsMiddleware(model.LabelSet{
		"job":       database_observability.JobName,
		"instance":  model.LabelValue(instanceKey),
		"server_id": model.LabelValue(serverID),
	}).Wrap(entryHandler)

	return entryHandler
}
