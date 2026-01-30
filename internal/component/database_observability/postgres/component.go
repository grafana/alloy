package postgres

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

	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
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

const selectServerInfo = `
SELECT
	(pg_control_system()).system_identifier,
	inet_server_addr(),
	inet_server_port(),
	setting as version
FROM pg_settings
WHERE name = 'server_version';`

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
	DataSourceName    alloytypes.Secret   `alloy:"data_source_name,attr"`
	ForwardTo         []loki.LogsReceiver `alloy:"forward_to,attr"`
	Targets           []discovery.Target  `alloy:"targets,attr"`
	EnableCollectors  []string            `alloy:"enable_collectors,attr,optional"`
	DisableCollectors []string            `alloy:"disable_collectors,attr,optional"`
	ExcludeDatabases  []string            `alloy:"exclude_databases,attr,optional"`

	CloudProvider          *CloudProvider         `alloy:"cloud_provider,block,optional"`
	QuerySampleArguments   QuerySampleArguments   `alloy:"query_samples,block,optional"`
	QueryTablesArguments   QueryTablesArguments   `alloy:"query_details,block,optional"`
	SchemaDetailsArguments SchemaDetailsArguments `alloy:"schema_details,block,optional"`
	ExplainPlansArguments  ExplainPlansArguments  `alloy:"explain_plans,block,optional"`
	HealthCheckArguments   HealthCheckArguments   `alloy:"health_check,block,optional"`
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

type QuerySampleArguments struct {
	CollectInterval       time.Duration `alloy:"collect_interval,attr,optional"`
	DisableQueryRedaction bool          `alloy:"disable_query_redaction,attr,optional"`
	ExcludeCurrentUser    bool          `alloy:"exclude_current_user,attr,optional"`
}

type QueryTablesArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

type SchemaDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	CacheEnabled    bool          `alloy:"cache_enabled,attr,optional"`
	CacheSize       int           `alloy:"cache_size,attr,optional"`
	CacheTTL        time.Duration `alloy:"cache_ttl,attr,optional"`
}

var DefaultArguments = Arguments{
	ExcludeDatabases: []string{},
	QuerySampleArguments: QuerySampleArguments{
		CollectInterval:       15 * time.Second,
		DisableQueryRedaction: false,
		ExcludeCurrentUser:    true,
	},
	QueryTablesArguments: QueryTablesArguments{
		CollectInterval: 1 * time.Minute,
	},
	SchemaDetailsArguments: SchemaDetailsArguments{
		CollectInterval: 1 * time.Minute,
		CacheEnabled:    true,
		CacheSize:       256,
		CacheTTL:        10 * time.Minute,
	},
	ExplainPlansArguments: ExplainPlansArguments{
		CollectInterval: 1 * time.Minute,
		PerCollectRatio: 1.0,
	},
	HealthCheckArguments: HealthCheckArguments{
		CollectInterval: 1 * time.Hour,
	},
}

type ExplainPlansArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	PerCollectRatio float64       `alloy:"per_collect_ratio,attr,optional"`
}

type HealthCheckArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
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
	Targets           []discovery.Target `alloy:"targets,attr"`
	ErrorLogsReceiver loki.LogsReceiver  `alloy:"error_logs_receiver,attr,optional"`
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

	errorLogsReceiver loki.LogsReceiver
	errorLogsIn       chan loki.Entry
}

func New(opts component.Options, args Arguments) (*Component, error) {
	return new(opts, args, sql.Open)
}

func new(opts component.Options, args Arguments, openFn func(driverName, dataSourceName string) (*sql.DB, error)) (*Component, error) {
	c := &Component{
		opts:              opts,
		args:              args,
		receivers:         args.ForwardTo,
		handler:           loki.NewLogsReceiver(),
		registry:          prometheus.NewRegistry(),
		healthErr:         atomic.NewString(""),
		openSQL:           openFn,
		errorLogsReceiver: loki.NewLogsReceiver(),
		errorLogsIn:       make(chan loki.Entry),
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

	// Export error_logs receiver immediately (stable for component lifetime).
	// Prevents nil pointer panics in loki.source.file when database is unavailable.
	opts.OnStateChange(Exports{
		Targets:           []discovery.Target{},
		ErrorLogsReceiver: c.errorLogsReceiver,
	})

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

	// Bridge exported receiver to internal channel or drop if collector not running
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		drainMode := false
		for {
			select {
			case <-ctx.Done():
				return
			case entry := <-c.errorLogsReceiver.Chan():
				c.mut.RLock()
				hasErrorLogsCollector := false
				for _, collector := range c.collectors {
					if collector.Name() == "error_logs" {
						hasErrorLogsCollector = true
						break
					}
				}
				c.mut.RUnlock()

				if !hasErrorLogsCollector {
					if !drainMode {
						level.Warn(c.opts.Logger).Log(
							"msg", "database unavailable: dropping error log entries (error_logs collector not started)",
						)
						drainMode = true
					}
				} else {
					if drainMode {
						level.Info(c.opts.Logger).Log("msg", "database reconnected: error_logs collector now processing entries")
						drainMode = false
					}
					select {
					case c.errorLogsIn <- entry:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	// Automatic reconnection ticker
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
				hasCollectors := len(c.collectors) > 0
				c.mut.RUnlock()

				if !hasCollectors {
					level.Debug(c.opts.Logger).Log("msg", "attempting to reconnect to database")
					if err := c.tryReconnect(ctx); err == nil {
						level.Info(c.opts.Logger).Log("msg", "successfully reconnected to database and started collectors")
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

func (c *Component) tryReconnect(ctx context.Context) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	if err := c.connectAndStartCollectors(ctx); err != nil {
		return err
	}

	c.healthErr.Store("")
	return nil
}

func (c *Component) connectAndStartCollectors(ctx context.Context) error {
	if c.dbConnection != nil {
		c.dbConnection.Close()
		c.dbConnection = nil
	}

	dbConnection, err := c.openSQL("postgres", string(c.args.DataSourceName))
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

	rs := dbConnection.QueryRowContext(ctx, selectServerInfo)
	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to query engine version: %w", err)
	}

	var systemID, systemIP, systemPort, engineVersion sql.NullString
	if err := rs.Scan(&systemID, &systemIP, &systemPort, &engineVersion); err != nil {
		return fmt.Errorf("failed to scan engine version: %w", err)
	}

	generatedSystemID := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", systemID.String, systemIP.String, systemPort.String))))

	var cp *database_observability.CloudProvider
	if c.args.CloudProvider != nil {
		cloudProvider, err := populateCloudProviderFromConfig(c.args.CloudProvider)
		if err != nil {
			return fmt.Errorf("failed to collect cloud provider information from config: %w", err)
		}
		cp = cloudProvider
	} else {
		cloudProvider, err := populateCloudProviderFromDSN(string(c.args.DataSourceName))
		if err != nil {
			return fmt.Errorf("failed to collect cloud provider information from DSN: %w", err)
		}
		cp = cloudProvider
	}

	allTargets := append([]discovery.Target{c.baseTarget}, c.args.Targets...)
	targets := make([]discovery.Target, 0, len(allTargets))
	for _, t := range allTargets {
		builder := discovery.NewTargetBuilderFrom(t)
		if relabel.ProcessBuilder(builder, database_observability.GetRelabelingRules(generatedSystemID, cp)...) {
			targets = append(targets, builder.Target())
		}
	}

	exports := Exports{
		Targets:           targets,
		ErrorLogsReceiver: c.errorLogsReceiver,
	}
	c.opts.OnStateChange(exports)

	for _, collector := range c.collectors {
		collector.Stop()
	}
	c.collectors = nil

	if err := c.startCollectors(generatedSystemID, engineVersion.String, cp); err != nil {
		return fmt.Errorf("failed to start collectors: %w", err)
	}

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
	level.Error(c.opts.Logger).Log("msg", fmt.Sprintf("%s: %+v", errorMsg, err))
	c.healthErr.Store(fmt.Sprintf("%s: %+v", errorMsg, err))
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.args = args.(Arguments)

	if err := c.connectAndStartCollectors(context.Background()); err != nil {
		c.reportError("failed to connect and start collectors", err)
		return nil
	}

	c.healthErr.Store("")
	return nil
}

func enableOrDisableCollectors(a Arguments) map[string]bool {
	collectors := map[string]bool{
		collector.QueryDetailsCollector:  true,
		collector.QuerySamplesCollector:  true,
		collector.SchemaDetailsCollector: true,
		collector.ExplainPlanCollector:   true,
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
func (c *Component) startCollectors(systemID string, engineVersion string, cloudProviderInfo *database_observability.CloudProvider) error {
	var startErrors []string

	logStartError := func(collectorName, action string, err error) {
		errorString := fmt.Sprintf("failed to %s %s collector: %+v", action, collectorName, err)
		level.Error(c.opts.Logger).Log("msg", errorString)
		startErrors = append(startErrors, errorString)
	}

	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), c.instanceKey, systemID)

	var tableRegistry *collector.TableRegistry
	collectors := enableOrDisableCollectors(c.args)

	if collectors[collector.SchemaDetailsCollector] {
		stCollector, err := collector.NewSchemaDetails(collector.SchemaDetailsArguments{
			DB:               c.dbConnection,
			DSN:              string(c.args.DataSourceName),
			CollectInterval:  c.args.SchemaDetailsArguments.CollectInterval,
			ExcludeDatabases: c.args.ExcludeDatabases,
			CacheEnabled:     c.args.SchemaDetailsArguments.CacheEnabled,
			CacheSize:        c.args.SchemaDetailsArguments.CacheSize,
			CacheTTL:         c.args.SchemaDetailsArguments.CacheTTL,
			EntryHandler:     entryHandler,
			Logger:           c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.SchemaDetailsCollector, "create", err)
		}
		tableRegistry = stCollector.GetTableRegistry()
		if err := stCollector.Start(context.Background()); err != nil {
			logStartError(collector.SchemaDetailsCollector, "start", err)
		}
		c.collectors = append(c.collectors, stCollector)
	}

	if collectors[collector.QueryDetailsCollector] {
		qCollector, err := collector.NewQueryDetails(collector.QueryDetailsArguments{
			DB:               c.dbConnection,
			CollectInterval:  c.args.QueryTablesArguments.CollectInterval,
			ExcludeDatabases: c.args.ExcludeDatabases,
			EntryHandler:     entryHandler,
			TableRegistry:    tableRegistry,
			Logger:           c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.QueryDetailsCollector, "create", err)
		}
		if err := qCollector.Start(context.Background()); err != nil {
			logStartError(collector.QueryDetailsCollector, "start", err)
		}
		c.collectors = append(c.collectors, qCollector)
	}

	if collectors[collector.QuerySamplesCollector] {
		aCollector, err := collector.NewQuerySamples(collector.QuerySamplesArguments{
			DB:                    c.dbConnection,
			CollectInterval:       c.args.QuerySampleArguments.CollectInterval,
			ExcludeDatabases:      c.args.ExcludeDatabases,
			EntryHandler:          entryHandler,
			Logger:                c.opts.Logger,
			DisableQueryRedaction: c.args.QuerySampleArguments.DisableQueryRedaction,
			ExcludeCurrentUser:    c.args.QuerySampleArguments.ExcludeCurrentUser,
		})
		if err != nil {
			logStartError(collector.QuerySamplesCollector, "create", err)
		}
		if err := aCollector.Start(context.Background()); err != nil {
			logStartError(collector.QuerySamplesCollector, "start", err)
		}
		c.collectors = append(c.collectors, aCollector)
	}

	ciCollector, err := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
		DSN:           string(c.args.DataSourceName),
		Registry:      c.registry,
		EngineVersion: engineVersion,
		CloudProvider: cloudProviderInfo,
	})
	if err != nil {
		logStartError(collector.ConnectionInfoName, "create", err)
	}
	if err := ciCollector.Start(context.Background()); err != nil {
		logStartError(collector.ConnectionInfoName, "start", err)
	}

	c.collectors = append(c.collectors, ciCollector)

	if collectors[collector.ExplainPlanCollector] {
		epCollector, err := collector.NewExplainPlan(collector.ExplainPlansArguments{
			DB:               c.dbConnection,
			DSN:              string(c.args.DataSourceName),
			ScrapeInterval:   c.args.ExplainPlansArguments.CollectInterval,
			PerScrapeRatio:   c.args.ExplainPlansArguments.PerCollectRatio,
			ExcludeDatabases: c.args.ExcludeDatabases,
			Logger:           c.opts.Logger,
			DBVersion:        engineVersion,
			EntryHandler:     entryHandler,
		})
		if err != nil {
			logStartError(collector.ExplainPlanCollector, "create", err)
		}
		if err := epCollector.Start(context.Background()); err != nil {
			logStartError(collector.ExplainPlanCollector, "start", err)
		}
		c.collectors = append(c.collectors, epCollector)
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
		} else {
			c.collectors = append(c.collectors, hcCollector)
		}
	}

	// ErrorLogs collector is always enabled
	errorLogsInternalReceiver := loki.NewLogsReceiver(loki.WithChannel(c.errorLogsIn))

	elCollector, err := collector.NewErrorLogs(collector.ErrorLogsArguments{
		Receiver:     errorLogsInternalReceiver,
		EntryHandler: entryHandler,
		Logger:       c.opts.Logger,
		InstanceKey:  c.instanceKey,
		SystemID:     systemID,
		Registry:     c.registry,
	})
	if err != nil {
		logStartError(collector.ErrorLogsCollector, "create", err)
	} else {
		if err := elCollector.Start(context.Background()); err != nil {
			logStartError(collector.ErrorLogsCollector, "start", err)
		} else {
			c.collectors = append(c.collectors, elCollector)
		}
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

func addLokiLabels(entryHandler loki.EntryHandler, instanceKey string, systemID string) loki.EntryHandler {
	entryHandler = loki.AddLabelsMiddleware(model.LabelSet{
		"job":       database_observability.JobName,
		"instance":  model.LabelValue(instanceKey),
		"server_id": model.LabelValue(systemID),
	}).Wrap(entryHandler)

	return entryHandler
}
