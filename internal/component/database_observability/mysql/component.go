package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

const name = "database_observability.mysql"

const selectServerInfo = `SELECT @@server_uuid, VERSION()`

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

func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Validate() error {
	_, err := mysql.ParseDSN(string(a.DataSourceName))
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
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:      opts,
		args:      args,
		receivers: args.ForwardTo,
		handler:   loki.NewLogsReceiver(),
		registry:  prometheus.NewRegistry(),
		healthErr: atomic.NewString(""),
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

	if c.dbConnection != nil {
		c.dbConnection.Close()
	}

	c.args = args.(Arguments)

	// Always export at least the base target so the component can expose metrics
	// even if the database connection is currently unavailable.
	c.opts.OnStateChange(Exports{
		Targets: []discovery.Target{c.baseTarget},
	})

	for _, collector := range c.collectors {
		collector.Stop()
	}
	c.collectors = nil

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
		collector.QueryTablesName:    true,
		collector.SchemaTableName:    true,
		collector.SetupConsumersName: true,
		collector.QuerySampleName:    true,
		collector.ExplainPlanName:    false,
		collector.LocksName:          false,
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
	// value 1 on success and 0 on failure to establish the connection and get the engine version
	startConnInfo := func(val float64, engineVersion string, cloudProvider *database_observability.CloudProvider) {
		ciCollector, ciErr := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
			DSN:           string(c.args.DataSourceName),
			Registry:      c.registry,
			EngineVersion: engineVersion,
			CloudProvider: cloudProvider,
			Value:         val,
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

	var mysqlOpen func(driverName, dataSourceName string) (*sql.DB, error)
	if c.opts.OpenSQL != nil {
		mysqlOpen = c.opts.OpenSQL
	} else {
		mysqlOpen = sql.Open
	}

	dbConnection, err := mysqlOpen("mysql", formatDSN(string(c.args.DataSourceName), "parseTime=true"))
	if err != nil {
		err = fmt.Errorf("failed to start collectors: failed to open MySQL connection: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo(0, "", nil)
		return err
	}
	if dbConnection == nil {
		err = fmt.Errorf("failed to start collectors: nil DB connection")
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo(0, "", nil)
		return err
	}
	if err = dbConnection.Ping(); err != nil {
		err = fmt.Errorf("failed to start collectors: failed to ping MySQL: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo(0, "", nil)
		return err
	}
	c.dbConnection = dbConnection

	rs := c.dbConnection.QueryRowContext(context.Background(), selectServerInfo)
	if err = rs.Err(); err != nil {
		err = fmt.Errorf("failed to query engine version: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo(0, "", nil)
		return err
	}

	var serverUUID, engineVersion string
	if err := rs.Scan(&serverUUID, &engineVersion); err != nil {
		err = fmt.Errorf("failed to scan engine version: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo(0, "", nil)
		return err
	}

	// Update exported targets based on server UUID relabeling
	c.args.Targets = append([]discovery.Target{c.baseTarget}, c.args.Targets...)
	targets := make([]discovery.Target, 0, len(c.args.Targets)+1)
	for _, t := range c.args.Targets {
		builder := discovery.NewTargetBuilderFrom(t)
		if relabel.ProcessBuilder(builder, database_observability.GetRelabelingRules(serverUUID)...) {
			targets = append(targets, builder.Target())
		}
	}
	c.opts.OnStateChange(Exports{
		Targets: targets,
	})
	var cloudProviderInfo *database_observability.CloudProvider
	if c.args.CloudProvider != nil && c.args.CloudProvider.AWS != nil {
		arn, err := arn.Parse(c.args.CloudProvider.AWS.ARN)
		if err != nil {
			err = fmt.Errorf("failed to parse AWS cloud provider ARN: %w", err)
			level.Error(c.opts.Logger).Log("msg", err.Error())
			return err
		}
		cloudProviderInfo = &database_observability.CloudProvider{
			AWS: &database_observability.AWSCloudProviderInfo{
				ARN: arn,
			},
		}
	}

	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), c.instanceKey, serverUUID)

	collectors := enableOrDisableCollectors(c.args)

	// Best-effort start: try building/starting every enabled collector and aggregate errors.
	var startErrors []string

	if collectors[collector.QueryTablesName] {
		qtCollector, err := collector.NewQueryTables(collector.QueryTablesArguments{
			DB:              c.dbConnection,
			CollectInterval: c.args.QueryTablesArguments.CollectInterval,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
		})
		if err != nil {
			err = fmt.Errorf("failed to create %s collector: %w", collector.QueryTablesName, err)
			level.Error(c.opts.Logger).Log("msg", err.Error())
			startErrors = append(startErrors, err.Error())
		} else {
			if err := qtCollector.Start(context.Background()); err != nil {
				err = fmt.Errorf("failed to start %s collector: %w", collector.QueryTablesName, err)
				level.Error(c.opts.Logger).Log("msg", err.Error())
				startErrors = append(startErrors, err.Error())
			} else {
				c.collectors = append(c.collectors, qtCollector)
			}
		}
	}

	if collectors[collector.SchemaTableName] {
		stCollector, err := collector.NewSchemaTable(collector.SchemaTableArguments{
			DB:              c.dbConnection,
			CollectInterval: c.args.SchemaTableArguments.CollectInterval,
			CacheEnabled:    c.args.SchemaTableArguments.CacheEnabled,
			CacheSize:       c.args.SchemaTableArguments.CacheSize,
			CacheTTL:        c.args.SchemaTableArguments.CacheTTL,

			EntryHandler: entryHandler,
			Logger:       c.opts.Logger,
		})
		if err != nil {
			err = fmt.Errorf("failed to create %s collector: %w", collector.SchemaTableName, err)
			level.Error(c.opts.Logger).Log("msg", err.Error())
			startErrors = append(startErrors, err.Error())
		} else {
			if err := stCollector.Start(context.Background()); err != nil {
				err = fmt.Errorf("failed to start %s collector: %w", collector.SchemaTableName, err)
				level.Error(c.opts.Logger).Log("msg", err.Error())
				startErrors = append(startErrors, err.Error())
			} else {
				c.collectors = append(c.collectors, stCollector)
			}
		}
	}

	if collectors[collector.QuerySampleName] {
		qsCollector, err := collector.NewQuerySample(collector.QuerySampleArguments{
			DB:                          c.dbConnection,
			CollectInterval:             c.args.QuerySampleArguments.CollectInterval,
			EntryHandler:                entryHandler,
			Logger:                      c.opts.Logger,
			DisableQueryRedaction:       c.args.QuerySampleArguments.DisableQueryRedaction,
			AutoEnableSetupConsumers:    c.args.AllowUpdatePerfSchemaSettings && c.args.QuerySampleArguments.AutoEnableSetupConsumers,
			SetupConsumersCheckInterval: c.args.QuerySampleArguments.SetupConsumersCheckInterval,
		})
		if err != nil {
			err = fmt.Errorf("failed to create %s collector: %w", collector.QuerySampleName, err)
			level.Error(c.opts.Logger).Log("msg", err.Error())
			startErrors = append(startErrors, err.Error())
		} else {
			if err := qsCollector.Start(context.Background()); err != nil {
				err = fmt.Errorf("failed to start %s collector: %w", collector.QuerySampleName, err)
				level.Error(c.opts.Logger).Log("msg", err.Error())
				startErrors = append(startErrors, err.Error())
			} else {
				c.collectors = append(c.collectors, qsCollector)
			}
		}
	}

	if collectors[collector.SetupConsumersName] {
		scCollector, err := collector.NewSetupConsumer(collector.SetupConsumerArguments{
			DB:              c.dbConnection,
			Registry:        c.registry,
			Logger:          c.opts.Logger,
			CollectInterval: c.args.SetupConsumersArguments.CollectInterval,
		})
		if err != nil {
			err = fmt.Errorf("failed to create %s collector: %w", collector.SetupConsumersName, err)
			level.Error(c.opts.Logger).Log("msg", err.Error())
			startErrors = append(startErrors, err.Error())
		} else {
			if err := scCollector.Start(context.Background()); err != nil {
				err = fmt.Errorf("failed to start %s collector: %w", collector.SetupConsumersName, err)
				level.Error(c.opts.Logger).Log("msg", err.Error())
				startErrors = append(startErrors, err.Error())
			} else {
				c.collectors = append(c.collectors, scCollector)
			}
		}
	}

	if collectors[collector.LocksName] {
		locksCollector, err := collector.NewLock(collector.LockArguments{
			DB:                c.dbConnection,
			CollectInterval:   c.args.LocksArguments.CollectInterval,
			LockWaitThreshold: c.args.LocksArguments.Threshold,
			Logger:            c.opts.Logger,
			EntryHandler:      entryHandler,
		})
		if err != nil {
			err = fmt.Errorf("failed to create %s collector: %w", collector.LocksName, err)
			level.Error(c.opts.Logger).Log("msg", err.Error())
			startErrors = append(startErrors, err.Error())
		} else {
			if err := locksCollector.Start(context.Background()); err != nil {
				err = fmt.Errorf("failed to start %s collector: %w", collector.LocksName, err)
				level.Error(c.opts.Logger).Log("msg", err.Error())
				startErrors = append(startErrors, err.Error())
			} else {
				c.collectors = append(c.collectors, locksCollector)
			}
		}
	}

	if collectors[collector.ExplainPlanName] {
		epCollector, err := collector.NewExplainPlan(collector.ExplainPlanArguments{
			DB:              c.dbConnection,
			ScrapeInterval:  c.args.ExplainPlanArguments.CollectInterval,
			PerScrapeRatio:  c.args.ExplainPlanArguments.PerCollectRatio,
			Logger:          c.opts.Logger,
			DBVersion:       engineVersion,
			EntryHandler:    entryHandler,
			InitialLookback: time.Now().Add(-c.args.ExplainPlanArguments.InitialLookback),
		})
		if err != nil {
			err = fmt.Errorf("failed to create %s collector: %w", collector.ExplainPlanName, err)
			level.Error(c.opts.Logger).Log("msg", err.Error())
			startErrors = append(startErrors, err.Error())
		} else {
			if err := epCollector.Start(context.Background()); err != nil {
				err = fmt.Errorf("failed to start %s collector: %w", collector.ExplainPlanName, err)
				level.Error(c.opts.Logger).Log("msg", err.Error())
				startErrors = append(startErrors, err.Error())
			} else {
				c.collectors = append(c.collectors, epCollector)
			}
		}
	}

	// Connection Info collector is always enabled (value 1 on success)
	startConnInfo(1, engineVersion, cloudProviderInfo)

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

// instanceKey returns network(hostname:port)/dbname of the MySQL server.
// This is the same key as used by the mysqld_exporter integration.
func instanceKey(dsn string) (string, error) {
	m, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}

	if m.Addr == "" {
		m.Addr = "localhost:3306"
	}
	if m.Net == "" {
		m.Net = "tcp"
	}

	return fmt.Sprintf("%s(%s)/%s", m.Net, m.Addr, m.DBName), nil
}

// formatDSN appends the given parameters to the DSN.
// parameters are expected to be in the form of "key=value".
func formatDSN(dsn string, params ...string) string {
	if len(params) == 0 {
		return dsn
	}

	if strings.Contains(dsn, "?") {
		dsn = dsn + "&"
	} else {
		dsn = dsn + "?"
	}
	return dsn + strings.Join(params, "&")
}

func addLokiLabels(entryHandler loki.EntryHandler, instanceKey string, serverUUID string) loki.EntryHandler {
	entryHandler = loki.AddLabelsMiddleware(model.LabelSet{
		"job":       database_observability.JobName,
		"instance":  model.LabelValue(instanceKey),
		"server_id": model.LabelValue(serverUUID),
	}).Wrap(entryHandler)

	return entryHandler
}
