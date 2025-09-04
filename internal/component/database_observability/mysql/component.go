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

type Arguments struct {
	DataSourceName                alloytypes.Secret   `alloy:"data_source_name,attr"`
	ForwardTo                     []loki.LogsReceiver `alloy:"forward_to,attr"`
	Targets                       []discovery.Target  `alloy:"targets,attr,optional"`
	EnableCollectors              []string            `alloy:"enable_collectors,attr,optional"`
	DisableCollectors             []string            `alloy:"disable_collectors,attr,optional"`
	AllowUpdatePerfSchemaSettings bool                `alloy:"allow_update_performance_schema_settings,attr,optional"`

	CloudProvider           *CloudProvider          `alloy:"cloud_provider,block,optional"`
	SetupConsumersArguments SetupConsumersArguments `alloy:"setup_consumers,block,optional"`
	QueryTablesArguments    QueryTablesArguments    `alloy:"query_details,block,optional"`
	SchemaTableArguments    SchemaTableArguments    `alloy:"schema_details,block,optional"`
	ExplainPlanArguments    ExplainPlanArguments    `alloy:"explain_plans,block,optional"`
	LocksArguments          LocksArguments          `alloy:"locks,block,optional"`
	QuerySampleArguments    QuerySampleArguments    `alloy:"query_samples,block,optional"`
	ConnectionInfoArguments ConnectionInfoArguments `alloy:"connection_info,block,optional"`
}

type CloudProvider struct {
	AWS *AWSCloudProviderInfo `alloy:"aws,block,optional"`
}

type AWSCloudProviderInfo struct {
	ARN string `alloy:"arn,attr"`
}

type QueryTablesArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

type SchemaTableArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	CacheEnabled    bool          `alloy:"cache_enabled,attr,optional"`
	CacheSize       int           `alloy:"cache_size,attr,optional"`
	CacheTTL        time.Duration `alloy:"cache_ttl,attr,optional"`
}

type SetupConsumersArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

type ExplainPlanArguments struct {
	CollectInterval           time.Duration `alloy:"collect_interval,attr,optional"`
	PerCollectRatio           float64       `alloy:"per_collect_ratio,attr,optional"`
	InitialLookback           time.Duration `alloy:"initial_lookback,attr,optional"`
	ExplainPlanExcludeSchemas []string      `alloy:"explain_plan_exclude_schemas,attr,optional"`
}

type LocksArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	Threshold       time.Duration `alloy:"threshold,attr,optional"`
}

type QuerySampleArguments struct {
	CollectInterval             time.Duration `alloy:"collect_interval,attr,optional"`
	DisableQueryRedaction       bool          `alloy:"disable_query_redaction,attr,optional"`
	AutoEnableSetupConsumers    bool          `alloy:"auto_enable_setup_consumers,attr,optional"`
	SetupConsumersCheckInterval time.Duration `alloy:"setup_consumers_check_interval,attr,optional"`
}

type ConnectionInfoArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

var DefaultArguments = Arguments{
	AllowUpdatePerfSchemaSettings: false,

	QueryTablesArguments: QueryTablesArguments{
		CollectInterval: 1 * time.Minute,
	},

	SchemaTableArguments: SchemaTableArguments{
		CollectInterval: 1 * time.Minute,
		CacheEnabled:    true,
		CacheSize:       256,
		CacheTTL:        10 * time.Minute,
	},

	SetupConsumersArguments: SetupConsumersArguments{
		CollectInterval: 1 * time.Hour,
	},

	ExplainPlanArguments: ExplainPlanArguments{
		CollectInterval: 1 * time.Minute,
		PerCollectRatio: 1.0,
		InitialLookback: 24 * time.Hour,
	},

	LocksArguments: LocksArguments{
		CollectInterval: 30 * time.Second,
		Threshold:       1 * time.Second,
	},

	QuerySampleArguments: QuerySampleArguments{
		CollectInterval:             1 * time.Minute,
		DisableQueryRedaction:       false,
		AutoEnableSetupConsumers:    false,
		SetupConsumersCheckInterval: 1 * time.Hour,
	},
	ConnectionInfoArguments: ConnectionInfoArguments{
		CollectInterval: 15 * time.Second,
	},
}

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

	if c.dbConnection != nil {
		c.dbConnection.Close()
	}

	c.args = args.(Arguments)

	for _, collector := range c.collectors {
		collector.Stop()
	}
	c.collectors = nil

	if err := c.startCollectors(); err != nil {
		c.healthErr.Store(err.Error())
		// Export at least the base target so the component can expose metrics
		// even if the database connection is currently unavailable.
		c.opts.OnStateChange(Exports{
			Targets: []discovery.Target{c.baseTarget},
		})
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
	startConnInfo := func(engineVersion string, cloudProvider *database_observability.CloudProvider) {
		ciCollector, ciErr := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
			DSN:           string(c.args.DataSourceName),
			Registry:      c.registry,
			EngineVersion: engineVersion,
			CloudProvider: cloudProvider,
			CheckInterval: c.args.ConnectionInfoArguments.CollectInterval,
			DB:            c.dbConnection,
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

	dbConnection, err := c.openSQL("mysql", formatDSN(string(c.args.DataSourceName), "parseTime=true"))
	if err != nil {
		err = fmt.Errorf("failed to start collectors: failed to open MySQL connection: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("", nil)
		return err
	}
	if dbConnection == nil {
		err = fmt.Errorf("failed to start collectors: nil DB connection")
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("", nil)
		return err
	}
	if err = dbConnection.Ping(); err != nil {
		err = fmt.Errorf("failed to start collectors: failed to ping MySQL: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("", nil)
		return err
	}
	c.dbConnection = dbConnection

	rs := c.dbConnection.QueryRowContext(context.Background(), selectServerInfo)
	if err = rs.Err(); err != nil {
		err = fmt.Errorf("failed to query engine version: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("", nil)
		return err
	}

	var serverUUID, engineVersion string
	if err := rs.Scan(&serverUUID, &engineVersion); err != nil {
		err = fmt.Errorf("failed to scan engine version: %w", err)
		level.Error(c.opts.Logger).Log("msg", err.Error())
		startConnInfo("", nil)
		return err
	}

	// Update exported targets based on server UUID relabeling
	sourceTargets := append([]discovery.Target{c.baseTarget}, c.args.Targets...)
	targets := make([]discovery.Target, 0, len(sourceTargets)+1)
	for _, t := range sourceTargets {
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

	// Start the connection_info collector first, so we can report the connection status.
	startConnInfo(engineVersion, cloudProviderInfo)

	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), c.instanceKey, serverUUID)

	collectors := enableOrDisableCollectors(c.args)

	// Best-effort start: try building/starting every enabled collector and aggregate errors.
	var startErrors []string

	logStartError := func(collectorName, action string, err error) {
		wrapped := fmt.Errorf("failed to %s %s collector: %w", action, collectorName, err)
		level.Error(c.opts.Logger).Log("msg", wrapped.Error())
		startErrors = append(startErrors, wrapped.Error())
	}

	if collectors[collector.QueryTablesName] {
		qtCollector, err := collector.NewQueryTables(collector.QueryTablesArguments{
			DB:              c.dbConnection,
			CollectInterval: c.args.QueryTablesArguments.CollectInterval,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.QueryTablesName, "create", err)
		} else {
			if err := qtCollector.Start(context.Background()); err != nil {
				logStartError(collector.QueryTablesName, "start", err)
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
			logStartError(collector.SchemaTableName, "create", err)
		} else {
			if err := stCollector.Start(context.Background()); err != nil {
				logStartError(collector.SchemaTableName, "start", err)
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
			logStartError(collector.QuerySampleName, "create", err)
		} else {
			if err := qsCollector.Start(context.Background()); err != nil {
				logStartError(collector.QuerySampleName, "start", err)
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
			logStartError(collector.SetupConsumersName, "create", err)
		} else {
			if err := scCollector.Start(context.Background()); err != nil {
				logStartError(collector.SetupConsumersName, "start", err)
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
			logStartError(collector.LocksName, "create", err)
		} else {
			if err := locksCollector.Start(context.Background()); err != nil {
				logStartError(collector.LocksName, "start", err)
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
			logStartError(collector.ExplainPlanName, "create", err)
		} else {
			if err := epCollector.Start(context.Background()); err != nil {
				logStartError(collector.ExplainPlanName, "start", err)
			} else {
				c.collectors = append(c.collectors, epCollector)
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
