package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
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
	mysqld_collector "github.com/prometheus/mysqld_exporter/collector"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	exporter_mysql "github.com/grafana/alloy/internal/component/prometheus/exporter/mysql"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/static/integrations/mysqld_exporter"
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

type TargetArguments struct {
	DataSourceName alloytypes.Secret `alloy:"data_source_name,attr"`
	CloudProvider  *CloudProvider    `alloy:"cloud_provider,block,optional"`
}

type Arguments struct {
	Targets                       []TargetArguments   `alloy:"target,block"`
	ForwardTo                     []loki.LogsReceiver `alloy:"forward_to,attr"`
	ScrapeTargets                 []discovery.Target  `alloy:"targets,attr,optional"`
	EnableCollectors              []string            `alloy:"enable_collectors,attr,optional"`
	DisableCollectors             []string            `alloy:"disable_collectors,attr,optional"`
	ExcludeSchemas                []string            `alloy:"exclude_schemas,attr,optional"`
	AllowUpdatePerfSchemaSettings bool                `alloy:"allow_update_performance_schema_settings,attr,optional"`

	SetupConsumersArguments SetupConsumersArguments `alloy:"setup_consumers,block,optional"`
	SetupActorsArguments    SetupActorsArguments    `alloy:"setup_actors,block,optional"`
	QueryDetailsArguments   QueryDetailsArguments   `alloy:"query_details,block,optional"`
	SchemaDetailsArguments  SchemaDetailsArguments  `alloy:"schema_details,block,optional"`
	ExplainPlansArguments   ExplainPlansArguments   `alloy:"explain_plans,block,optional"`
	LocksArguments          LocksArguments          `alloy:"locks,block,optional"`
	QuerySamplesArguments   QuerySamplesArguments   `alloy:"query_samples,block,optional"`
	HealthCheckArguments    HealthCheckArguments    `alloy:"health_check,block,optional"`
	PrometheusExporter      *PrometheusExporterArguments `alloy:"prometheus_exporter,block,optional"`
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

// PrometheusExporterArguments configures the embedded mysqld_exporter scrapers.
// When this block is present, mysqld_exporter metrics are served alongside the
// component's own metrics at the same /metrics endpoint.
//
// It is a distinct type (not an embedded struct) because the Alloy syntax
// system does not support anonymous/embedded fields.
type PrometheusExporterArguments exporter_mysql.Arguments

func (a *PrometheusExporterArguments) SetToDefault() {
	*a = PrometheusExporterArguments(exporter_mysql.DefaultArguments)
}

func (a *PrometheusExporterArguments) Validate() error { return nil }

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
	if len(a.Targets) != 1 {
		return fmt.Errorf("exactly one target block is required")
	}
	for _, t := range a.Targets {
		_, err := mysql.ParseDSN(string(t.DataSourceName))
		if err != nil {
			return err
		}
	}
	if a.PrometheusExporter != nil && len(a.ScrapeTargets) > 0 {
		return fmt.Errorf("prometheus_exporter and targets are mutually exclusive: use prometheus_exporter to embed the exporter, or targets to scrape an external one")
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

type targetState struct {
	instanceKey  string
	dbConnection *sql.DB
	registry     *prometheus.Registry
	collectors   []Collector
}

type Component struct {
	opts      component.Options
	args      Arguments
	mut       sync.RWMutex
	receivers []loki.LogsReceiver
	handler   loki.LogsReceiver
	registry  *prometheus.Registry
	baseTarget discovery.Target
	targets    []*targetState
	healthErr  *atomic.String
	openSQL    func(driverName, dataSourceName string) (*sql.DB, error)
	exporterCollector prometheus.Collector
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

	// Compute the instance key from the first (only) target for use in the base target.
	instance, err := instanceKey(string(args.Targets[0].DataSourceName))
	if err != nil {
		return nil, err
	}

	// Store a temporary placeholder so getBaseTarget can use it.
	// The real per-target state is in c.targets after connectAndStartAllTargets runs.
	c.targets = []*targetState{{instanceKey: instance}}

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
		for _, t := range c.targets {
			for _, col := range t.collectors {
				col.Stop()
			}
			if t.dbConnection != nil {
				t.dbConnection.Close()
			}
		}
		c.mut.RUnlock()
	}()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.mut.RLock()
				needsReconnect := len(c.targets) == 0
				if !needsReconnect && len(c.targets) > 0 {
					for _, col := range c.targets[0].collectors {
						if col.Stopped() {
							needsReconnect = true
							break
						}
					}
				}
				c.mut.RUnlock()

				if needsReconnect {
					level.Debug(c.opts.Logger).Log("msg", "attempting to reconnect to database")
					if err := c.tryReconnect(ctx); err != nil {
						level.Error(c.opts.Logger).Log("msg", "reconnection attempt failed", "err", err)
					}
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
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

	// Use instance key from the first target if available.
	var iKey string
	if len(c.targets) > 0 {
		iKey = c.targets[0].instanceKey
	}

	return discovery.NewTargetFromMap(map[string]string{
		model.AddressLabel:     httpData.MemoryListenAddr,
		model.SchemeLabel:      "http",
		model.MetricsPathLabel: path.Join(httpData.HTTPPathForComponent(c.opts.ID), "metrics"),
		"instance":             iKey,
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

	c.args = args.(Arguments)

	if err := c.connectAndStartAllTargets(context.Background()); err != nil {
		c.reportError("failed to connect", err)
		return nil
	}

	c.healthErr.Store("")
	return nil
}

func (c *Component) tryReconnect(ctx context.Context) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	if err := c.connectAndStartAllTargets(ctx); err != nil {
		c.reportError("reconnection failed", err)
		return err
	}

	c.healthErr.Store("")
	return nil
}

// connectAndStartAllTargets handles the full connection lifecycle:
// closes old connections, opens new ones, queries server info, and starts collectors.
// Must be called with c.mut locked.
func (c *Component) connectAndStartAllTargets(ctx context.Context) error {
	// Stop all collectors and close all connections from previous targets.
	for _, t := range c.targets {
		for _, col := range t.collectors {
			col.Stop()
		}
		if t.dbConnection != nil {
			t.dbConnection.Close()
		}
	}
	c.targets = nil

	// Process the single target (multi-target unlocked in a later phase).
	tArgs := c.args.Targets[0]

	iKey, err := instanceKey(string(tArgs.DataSourceName))
	if err != nil {
		return fmt.Errorf("failed to compute instance key: %w", err)
	}

	t := &targetState{
		instanceKey: iKey,
		registry:    c.registry,
	}

	dbConnection, err := c.openSQL("mysql", formatDSN(string(tArgs.DataSourceName), "parseTime=true"))
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
	t.dbConnection = dbConnection

	rs := t.dbConnection.QueryRowContext(ctx, selectServerInfo)
	if err = rs.Err(); err != nil {
		t.dbConnection.Close()
		return fmt.Errorf("failed to query engine version: %w", err)
	}

	var serverUUID, hostname, engineVersion string
	if err := rs.Scan(&serverUUID, &hostname, &engineVersion); err != nil {
		t.dbConnection.Close()
		return fmt.Errorf("failed to scan engine version: %w", err)
	}

	generatedServerID := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s", serverUUID, hostname))))

	var parsedEngineVersion semver.Version
	matches := versionRegex.FindStringSubmatch(engineVersion)
	if len(matches) > 1 {
		parsedEngineVersion, err = semver.ParseTolerant(matches[1])
		if err != nil {
			t.dbConnection.Close()
			return fmt.Errorf("failed to parse engine version: %w", err)
		}
	}

	var cp *database_observability.CloudProvider
	if tArgs.CloudProvider != nil {
		cloudProvider, err := populateCloudProviderFromConfig(tArgs.CloudProvider)
		if err != nil {
			t.dbConnection.Close()
			return fmt.Errorf("failed to collect cloud provider information from config: %w", err)
		}
		cp = cloudProvider
	} else {
		cloudProvider, err := populateCloudProviderFromDSN(string(tArgs.DataSourceName))
		if err != nil {
			t.dbConnection.Close()
			return fmt.Errorf("failed to collect cloud provider information from DSN: %w", err)
		}
		cp = cloudProvider
	}

	if c.exporterCollector != nil {
		c.registry.Unregister(c.exporterCollector)
		c.exporterCollector = nil
	}

	if c.args.PrometheusExporter != nil {
		exporterArgs := exporter_mysql.Arguments(*c.args.PrometheusExporter)
		exporterCfg := exporterArgs.Convert()
		scrapers := mysqld_exporter.GetScrapers(exporterCfg)
		slogLogger := slog.New(logging.NewSlogGoKitHandler(c.opts.Logger))
		exporter := mysqld_collector.New(context.Background(), string(tArgs.DataSourceName), scrapers, slogLogger, mysqld_collector.Config{
			LockTimeout:   exporterCfg.LockWaitTimeout,
			SlowLogFilter: exporterCfg.LogSlowFilter,
		})
		if err := c.registry.Register(exporter); err != nil {
			t.dbConnection.Close()
			return fmt.Errorf("failed to register mysqld_exporter collector: %w", err)
		}
		c.exporterCollector = exporter
	}

	scrapeTargets := append([]discovery.Target{c.baseTarget}, c.args.ScrapeTargets...)
	targets := make([]discovery.Target, 0, len(scrapeTargets)+1)
	for _, st := range scrapeTargets {
		builder := discovery.NewTargetBuilderFrom(st)
		if relabel.ProcessBuilder(builder, database_observability.GetRelabelingRules(generatedServerID, cp)...) {
			targets = append(targets, builder.Target())
		}
	}

	c.opts.OnStateChange(Exports{
		Targets: targets,
	})

	collectors, err := c.startCollectorsForTarget(t, generatedServerID, engineVersion, parsedEngineVersion, cp)
	if err != nil {
		t.dbConnection.Close()
		return fmt.Errorf("failed to start collectors: %w", err)
	}
	t.collectors = collectors

	c.targets = append(c.targets, t)

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

// startCollectorsForTarget attempts to start all of the enabled collectors for the given target.
// If one or more collectors fail to start, their errors are accumulated and returned.
// Returns the list of started collectors and any error.
func (c *Component) startCollectorsForTarget(t *targetState, serverID string, engineVersion string, parsedEngineVersion semver.Version, cloudProviderInfo *database_observability.CloudProvider) ([]Collector, error) {
	var startErrors []string
	var collectors []Collector

	logStartError := func(collectorName, action string, err error) {
		errorString := fmt.Sprintf("failed to %s %s collector: %+v", action, collectorName, err)
		level.Error(c.opts.Logger).Log("msg", errorString)
		startErrors = append(startErrors, errorString)
	}
	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), t.instanceKey, serverID)

	enabledCollectors := enableOrDisableCollectors(c.args)

	if enabledCollectors[collector.QueryDetailsCollector] {
		qtCollector, err := collector.NewQueryDetails(collector.QueryDetailsArguments{
			DB:              t.dbConnection,
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
			collectors = append(collectors, qtCollector)
		}
	}

	if enabledCollectors[collector.SchemaDetailsCollector] {
		stCollector, err := collector.NewSchemaDetails(collector.SchemaDetailsArguments{
			DB:              t.dbConnection,
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
			collectors = append(collectors, stCollector)
		}
	}

	if enabledCollectors[collector.QuerySamplesCollector] {
		if c.args.QuerySamplesArguments.AutoEnableSetupConsumers && !c.args.AllowUpdatePerfSchemaSettings {
			level.Warn(c.opts.Logger).Log("msg", "auto_enable_setup_consumers is true but allow_update_performance_schema_settings is false, setup_consumers will not be enabled")
		}

		qsCollector, err := collector.NewQuerySamples(collector.QuerySamplesArguments{
			DB:                          t.dbConnection,
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
			collectors = append(collectors, qsCollector)
		}
	}

	if enabledCollectors[collector.SetupConsumersCollector] {
		scCollector, err := collector.NewSetupConsumers(collector.SetupConsumersArguments{
			DB:              t.dbConnection,
			Registry:        t.registry,
			Logger:          c.opts.Logger,
			CollectInterval: c.args.SetupConsumersArguments.CollectInterval,
		})
		if err != nil {
			logStartError(collector.SetupConsumersCollector, "create", err)
		} else {
			if err := scCollector.Start(context.Background()); err != nil {
				logStartError(collector.SetupConsumersCollector, "start", err)
			}
			collectors = append(collectors, scCollector)
		}
	}

	if enabledCollectors[collector.SetupActorsCollector] {
		if c.args.SetupActorsArguments.AutoUpdateSetupActors && !c.args.AllowUpdatePerfSchemaSettings {
			level.Warn(c.opts.Logger).Log("msg", "auto_update_setup_actors is true but allow_update_performance_schema_settings is false, setup_actors will not be updated")
		}

		saCollector, err := collector.NewSetupActors(collector.SetupActorsArguments{
			DB:                    t.dbConnection,
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
			collectors = append(collectors, saCollector)
		}
	}

	if enabledCollectors[collector.LocksCollector] {
		locksCollector, err := collector.NewLocks(collector.LocksArguments{
			DB:                t.dbConnection,
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
			collectors = append(collectors, locksCollector)
		}
	}

	if enabledCollectors[collector.ExplainPlansCollector] {
		epCollector, err := collector.NewExplainPlans(collector.ExplainPlansArguments{
			DB:              t.dbConnection,
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
			collectors = append(collectors, epCollector)
		}
	}

	// Connection Info collector is always enabled
	ciCollector, err := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
		DSN:           string(c.args.Targets[0].DataSourceName),
		Registry:      t.registry,
		EngineVersion: engineVersion,
		CloudProvider: cloudProviderInfo,
	})
	if err != nil {
		logStartError(collector.ConnectionInfoName, "create", err)
	} else {
		if err := ciCollector.Start(context.Background()); err != nil {
			logStartError(collector.ConnectionInfoName, "start", err)
		}
		collectors = append(collectors, ciCollector)
	}

	// HealthCheck collector is always enabled
	hcCollector, err := collector.NewHealthCheck(collector.HealthCheckArguments{
		DB:              t.dbConnection,
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
		collectors = append(collectors, hcCollector)
	}

	if len(startErrors) > 0 {
		return collectors, fmt.Errorf("failed to start some collectors: %s", strings.Join(startErrors, ", "))
	}

	return collectors, nil
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
	for _, t := range c.targets {
		for _, col := range t.collectors {
			if col.Stopped() {
				unhealthyCollectors = append(unhealthyCollectors, col.Name())
			}
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
