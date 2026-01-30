package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
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

const selectServerInfo = `SELECT @@server_uuid, @@hostname, VERSION()`

func init() {
	component.Register(component.Registration{
		Name:      name,
		Stability: featuregate.StabilityPublicPreview,
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
	Targets                       []discovery.Target  `alloy:"targets,attr"`
	EnableCollectors              []string            `alloy:"enable_collectors,attr,optional"`
	DisableCollectors             []string            `alloy:"disable_collectors,attr,optional"`
	ExcludeSchemas                []string            `alloy:"exclude_schemas,attr,optional"`
	AllowUpdatePerfSchemaSettings bool                `alloy:"allow_update_performance_schema_settings,attr,optional"`

	CloudProvider           *CloudProvider          `alloy:"cloud_provider,block,optional"`
	SetupConsumersArguments SetupConsumersArguments `alloy:"setup_consumers,block,optional"`
	SetupActorsArguments    SetupActorsArguments    `alloy:"setup_actors,block,optional"`
	QueryDetailsArguments   QueryDetailsArguments   `alloy:"query_details,block,optional"`
	SchemaDetailsArguments  SchemaDetailsArguments  `alloy:"schema_details,block,optional"`
	ExplainPlansArguments   ExplainPlansArguments   `alloy:"explain_plans,block,optional"`
	LocksArguments          LocksArguments          `alloy:"locks,block,optional"`
	QuerySamplesArguments   QuerySamplesArguments   `alloy:"query_samples,block,optional"`
	HealthCheckArguments    HealthCheckArguments    `alloy:"health_check,block,optional"`
}

type CloudProvider struct {
	AWS   *AWSCloudProviderInfo   `alloy:"aws,block,optional"`
	Azure *AzureCloudProviderInfo `alloy:"azure,block,optional"`
}

type AWSCloudProviderInfo struct {
	ARN string `alloy:"arn,attr"`
}

type AzureCloudProviderInfo struct {
	SubscriptionID string `alloy:"subscription_id,attr"`
	ResourceGroup  string `alloy:"resource_group,attr"`
	ServerName     string `alloy:"server_name,attr,optional"`
}

type QueryDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	StatementsLimit int           `alloy:"statements_limit,attr,optional"`
}

type SchemaDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	CacheEnabled    bool          `alloy:"cache_enabled,attr,optional"`
	CacheSize       int           `alloy:"cache_size,attr,optional"`
	CacheTTL        time.Duration `alloy:"cache_ttl,attr,optional"`
}

type SetupConsumersArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

type SetupActorsArguments struct {
	CollectInterval       time.Duration `alloy:"collect_interval,attr,optional"`
	AutoUpdateSetupActors bool          `alloy:"auto_update_setup_actors,attr,optional"`
}

type ExplainPlansArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	PerCollectRatio float64       `alloy:"per_collect_ratio,attr,optional"`
	InitialLookback time.Duration `alloy:"initial_lookback,attr,optional"`
}

type LocksArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	Threshold       time.Duration `alloy:"threshold,attr,optional"`
}

type QuerySamplesArguments struct {
	CollectInterval             time.Duration `alloy:"collect_interval,attr,optional"`
	DisableQueryRedaction       bool          `alloy:"disable_query_redaction,attr,optional"`
	AutoEnableSetupConsumers    bool          `alloy:"auto_enable_setup_consumers,attr,optional"`
	SetupConsumersCheckInterval time.Duration `alloy:"setup_consumers_check_interval,attr,optional"`
}

type HealthCheckArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

var DefaultArguments = Arguments{
	ExcludeSchemas:                []string{},
	AllowUpdatePerfSchemaSettings: false,

	QueryDetailsArguments: QueryDetailsArguments{
		CollectInterval: 1 * time.Minute,
		StatementsLimit: 250,
	},

	SchemaDetailsArguments: SchemaDetailsArguments{
		CollectInterval: 1 * time.Minute,
		CacheEnabled:    true,
		CacheSize:       256,
		CacheTTL:        10 * time.Minute,
	},

	SetupConsumersArguments: SetupConsumersArguments{
		CollectInterval: 1 * time.Hour,
	},

	SetupActorsArguments: SetupActorsArguments{
		CollectInterval:       1 * time.Hour,
		AutoUpdateSetupActors: false,
	},

	ExplainPlansArguments: ExplainPlansArguments{
		CollectInterval: 1 * time.Minute,
		PerCollectRatio: 1.0,
		InitialLookback: 24 * time.Hour,
	},

	LocksArguments: LocksArguments{
		CollectInterval: 30 * time.Second,
		Threshold:       1 * time.Second,
	},

	QuerySamplesArguments: QuerySamplesArguments{
		CollectInterval:             10 * time.Second,
		DisableQueryRedaction:       false,
		AutoEnableSetupConsumers:    false,
		SetupConsumersCheckInterval: 1 * time.Hour,
	},
	HealthCheckArguments: HealthCheckArguments{
		CollectInterval: 1 * time.Hour,
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

func New(opts component.Options, args Arguments) (*Component, error) {
	return new(opts, args, sql.Open)
}

func new(opts component.Options, args Arguments, openFn func(driverName, dataSourceName string) (*sql.DB, error)) (*Component, error) {
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

// The result of SELECT version() is something like:
// for MariaDB: "10.5.17-MariaDB-1:10.5.17+maria~ubu2004-log"
// for MySQL: "8.0.36-28.1"
var versionRegex = regexp.MustCompile(`^((\d+)(\.\d+)(\.\d+))`)

func (c *Component) reportError(errorMsg string, err error) {
	level.Error(c.opts.Logger).Log("msg", fmt.Sprintf("%s: %+v", errorMsg, err))
	c.healthErr.Store(fmt.Sprintf("%s: %+v", errorMsg, err))
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.dbConnection != nil {
		c.dbConnection.Close()
	}

	c.args = args.(Arguments)

	dbConnection, err := c.openSQL("mysql", formatDSN(string(c.args.DataSourceName), "parseTime=true"))
	if err != nil {
		c.reportError("failed to open database connection", err)
		return nil
	}

	if dbConnection == nil {
		c.reportError("nil DB connection", nil)
		return nil
	}

	if err = dbConnection.Ping(); err != nil {
		c.reportError("failed to ping database", err)
		return nil
	}
	c.dbConnection = dbConnection

	rs := c.dbConnection.QueryRowContext(context.Background(), selectServerInfo)
	if err = rs.Err(); err != nil {
		c.reportError("failed to query engine version", err)
		return nil
	}

	var serverUUID, hostname, engineVersion string
	if err := rs.Scan(&serverUUID, &hostname, &engineVersion); err != nil {
		c.reportError("failed to scan engine version", err)
		return nil
	}

	// Generate server_id hash from server_uuid and hostname, similar to Postgres collector
	generatedServerID := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s", serverUUID, hostname))))

	var parsedEngineVersion semver.Version
	matches := versionRegex.FindStringSubmatch(engineVersion)
	if len(matches) > 1 {
		parsedEngineVersion, err = semver.ParseTolerant(matches[1])
		if err != nil {
			c.reportError("failed to parse engine version", err)
			return nil
		}
	}

	var cp *database_observability.CloudProvider
	if c.args.CloudProvider != nil {
		cloudProvider, err := populateCloudProviderFromConfig(c.args.CloudProvider)
		if err != nil {
			c.reportError("failed to collect cloud provider information from config", err)
			return nil
		}
		cp = cloudProvider
	} else {
		cloudProvider, err := populateCloudProviderFromDSN(string(c.args.DataSourceName))
		if err != nil {
			c.reportError("failed to collect cloud provider information from DSN", err)
			return nil
		}
		cp = cloudProvider
	}

	c.args.Targets = append([]discovery.Target{c.baseTarget}, c.args.Targets...)
	targets := make([]discovery.Target, 0, len(c.args.Targets)+1)
	for _, t := range c.args.Targets {
		builder := discovery.NewTargetBuilderFrom(t)
		if relabel.ProcessBuilder(builder, database_observability.GetRelabelingRules(generatedServerID, cp)...) {
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

	if err := c.startCollectors(generatedServerID, engineVersion, parsedEngineVersion, cp); err != nil {
		c.reportError("failed to start collectors", err)
		return nil
	}

	c.healthErr.Store("")
	return nil
}

func enableOrDisableCollectors(a Arguments) map[string]bool {
	// configurable collectors and their default enabled/disabled value
	collectors := map[string]bool{
		collector.QueryDetailsCollector:   true,
		collector.SchemaDetailsCollector:  true,
		collector.SetupConsumersCollector: true,
		collector.SetupActorsCollector:    true,
		collector.QuerySamplesCollector:   true,
		collector.ExplainPlansCollector:   true,
		collector.LocksCollector:          false,
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

// startCollectors attempts to start all of the enabled collectors. If one or more collectors fail to start, their errors are reported
func (c *Component) startCollectors(serverID string, engineVersion string, parsedEngineVersion semver.Version, cloudProviderInfo *database_observability.CloudProvider) error {
	var startErrors []string

	logStartError := func(collectorName, action string, err error) {
		errorString := fmt.Sprintf("failed to %s %s collector: %+v", action, collectorName, err)
		level.Error(c.opts.Logger).Log("msg", errorString)
		startErrors = append(startErrors, errorString)
	}
	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), c.instanceKey, serverID)

	collectors := enableOrDisableCollectors(c.args)

	if collectors[collector.QueryDetailsCollector] {
		qtCollector, err := collector.NewQueryDetails(collector.QueryDetailsArguments{
			DB:              c.dbConnection,
			CollectInterval: c.args.QueryDetailsArguments.CollectInterval,
			StatementsLimit: c.args.QueryDetailsArguments.StatementsLimit,
			ExcludeSchemas:  c.args.ExcludeSchemas,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.QueryDetailsCollector, "create", err)
		} else {
			if err := qtCollector.Start(context.Background()); err != nil {
				logStartError(collector.QueryDetailsCollector, "start", err)
			}
			c.collectors = append(c.collectors, qtCollector)
		}
	}

	if collectors[collector.SchemaDetailsCollector] {
		stCollector, err := collector.NewSchemaDetails(collector.SchemaDetailsArguments{
			DB:              c.dbConnection,
			CollectInterval: c.args.SchemaDetailsArguments.CollectInterval,
			ExcludeSchemas:  c.args.ExcludeSchemas,
			CacheEnabled:    c.args.SchemaDetailsArguments.CacheEnabled,
			CacheSize:       c.args.SchemaDetailsArguments.CacheSize,
			CacheTTL:        c.args.SchemaDetailsArguments.CacheTTL,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
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

	if collectors[collector.QuerySamplesCollector] {
		if c.args.QuerySamplesArguments.AutoEnableSetupConsumers && !c.args.AllowUpdatePerfSchemaSettings {
			level.Warn(c.opts.Logger).Log("msg", "auto_enable_setup_consumers is true but allow_update_performance_schema_settings is false, setup_consumers will not be enabled")
		}

		qsCollector, err := collector.NewQuerySamples(collector.QuerySamplesArguments{
			DB:                          c.dbConnection,
			EngineVersion:               parsedEngineVersion,
			CollectInterval:             c.args.QuerySamplesArguments.CollectInterval,
			ExcludeSchemas:              c.args.ExcludeSchemas,
			EntryHandler:                entryHandler,
			Logger:                      c.opts.Logger,
			DisableQueryRedaction:       c.args.QuerySamplesArguments.DisableQueryRedaction,
			AutoEnableSetupConsumers:    c.args.AllowUpdatePerfSchemaSettings && c.args.QuerySamplesArguments.AutoEnableSetupConsumers,
			SetupConsumersCheckInterval: c.args.QuerySamplesArguments.SetupConsumersCheckInterval,
		})
		if err != nil {
			logStartError(collector.QuerySamplesCollector, "create", err)
		} else {
			if err := qsCollector.Start(context.Background()); err != nil {
				logStartError(collector.QuerySamplesCollector, "start", err)
			}
			c.collectors = append(c.collectors, qsCollector)
		}
	}

	if collectors[collector.SetupConsumersCollector] {
		scCollector, err := collector.NewSetupConsumers(collector.SetupConsumersArguments{
			DB:              c.dbConnection,
			Registry:        c.registry,
			Logger:          c.opts.Logger,
			CollectInterval: c.args.SetupConsumersArguments.CollectInterval,
		})
		if err != nil {
			logStartError(collector.SetupConsumersCollector, "create", err)
		} else {
			if err := scCollector.Start(context.Background()); err != nil {
				logStartError(collector.SetupConsumersCollector, "start", err)
			}
			c.collectors = append(c.collectors, scCollector)
		}
	}

	if collectors[collector.SetupActorsCollector] {
		if c.args.SetupActorsArguments.AutoUpdateSetupActors && !c.args.AllowUpdatePerfSchemaSettings {
			level.Warn(c.opts.Logger).Log("msg", "auto_update_setup_actors is true but allow_update_performance_schema_settings is false, setup_actors will not be updated")
		}

		saCollector, err := collector.NewSetupActors(collector.SetupActorsArguments{
			DB:                    c.dbConnection,
			Logger:                c.opts.Logger,
			CollectInterval:       c.args.SetupActorsArguments.CollectInterval,
			AutoUpdateSetupActors: c.args.AllowUpdatePerfSchemaSettings && c.args.SetupActorsArguments.AutoUpdateSetupActors,
		})
		if err != nil {
			logStartError(collector.SetupActorsCollector, "create", err)
		} else {
			if err := saCollector.Start(context.Background()); err != nil {
				logStartError(collector.SetupActorsCollector, "start", err)
			}
			c.collectors = append(c.collectors, saCollector)
		}
	}

	if collectors[collector.LocksCollector] {
		locksCollector, err := collector.NewLocks(collector.LocksArguments{
			DB:                c.dbConnection,
			CollectInterval:   c.args.LocksArguments.CollectInterval,
			LockWaitThreshold: c.args.LocksArguments.Threshold,
			Logger:            c.opts.Logger,
			EntryHandler:      entryHandler,
		})
		if err != nil {
			logStartError(collector.LocksCollector, "create", err)
		} else {
			if err := locksCollector.Start(context.Background()); err != nil {
				logStartError(collector.LocksCollector, "start", err)
			}
			c.collectors = append(c.collectors, locksCollector)
		}
	}

	if collectors[collector.ExplainPlansCollector] {
		epCollector, err := collector.NewExplainPlans(collector.ExplainPlansArguments{
			DB:              c.dbConnection,
			ScrapeInterval:  c.args.ExplainPlansArguments.CollectInterval,
			PerScrapeRatio:  c.args.ExplainPlansArguments.PerCollectRatio,
			ExcludeSchemas:  c.args.ExcludeSchemas,
			InitialLookback: time.Now().Add(-c.args.ExplainPlansArguments.InitialLookback),
			Logger:          c.opts.Logger,
			DBVersion:       engineVersion,
			EntryHandler:    entryHandler,
		})
		if err != nil {
			logStartError(collector.ExplainPlansCollector, "create", err)
		} else {
			if err := epCollector.Start(context.Background()); err != nil {
				logStartError(collector.ExplainPlansCollector, "start", err)
			}
			c.collectors = append(c.collectors, epCollector)
		}
	}

	// Connection Info collector is always enabled
	ciCollector, err := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
		DSN:           string(c.args.DataSourceName),
		Registry:      c.registry,
		EngineVersion: engineVersion,
		CloudProvider: cloudProviderInfo,
	})
	if err != nil {
		logStartError(collector.ConnectionInfoName, "create", err)
	} else {
		if err := ciCollector.Start(context.Background()); err != nil {
			logStartError(collector.ConnectionInfoName, "start", err)
		}
		c.collectors = append(c.collectors, ciCollector)
	}

	// HealthCheck collector is always enabled
	hcCollector, err := collector.NewHealthCheck(collector.HealthCheckArguments{
		DB:              c.dbConnection,
		CollectInterval: c.args.HealthCheckArguments.CollectInterval,
		EntryHandler:    entryHandler,
		Logger:          c.opts.Logger,
	})
	if err != nil {
		logStartError(collector.HealthCheckCollector, "create", err)
	} else {
		if err := hcCollector.Start(context.Background()); err != nil {
			logStartError(collector.HealthCheckCollector, "start", err)
		}
		c.collectors = append(c.collectors, hcCollector)
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

func addLokiLabels(entryHandler loki.EntryHandler, instanceKey string, serverID string) loki.EntryHandler {
	entryHandler = loki.AddLabelsMiddleware(model.LabelSet{
		"job":       database_observability.JobName,
		"instance":  model.LabelValue(instanceKey),
		"server_id": model.LabelValue(serverID),
	}).Wrap(entryHandler)

	return entryHandler
}
