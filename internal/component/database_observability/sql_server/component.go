package sql_server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/grafana/alloy/internal/util"
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

const (
	selectOriginalLogin    = `SELECT ORIGINAL_LOGIN()`
	defaultQueryTimeout    = collector.DefaultQueryTimeout
	databaseConnectTimeout = 10 * time.Second
)

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
	DataSourceName alloytypes.Secret   `alloy:"data_source_name,attr"`
	ForwardTo      []loki.LogsReceiver `alloy:"forward_to,attr"`
	Targets        []discovery.Target  `alloy:"targets,attr,optional"`

	QueryTimeout time.Duration `alloy:"query_timeout,attr,optional"`

	EnableCollectors   []string `alloy:"enable_collectors,attr,optional"`
	DisableCollectors  []string `alloy:"disable_collectors,attr,optional"`
	ExcludeSchemas     []string `alloy:"exclude_schemas,attr,optional"`
	ExcludeDatabases   []string `alloy:"exclude_databases,attr,optional"`
	ExcludeUsers       []string `alloy:"exclude_users,attr,optional"`
	ExcludeCurrentUser bool     `alloy:"exclude_current_user,attr,optional"`

	CloudProvider          *CloudProvider         `alloy:"cloud_provider,block,optional"`
	SchemaDetailsArguments SchemaDetailsArguments `alloy:"schema_details,block,optional"`
	QueryMetricsArguments  QueryMetricsArguments  `alloy:"query_metrics,block,optional"`
	QuerySamplesArguments  QuerySamplesArguments  `alloy:"query_samples,block,optional"`
	QueryDetailsArguments  QueryDetailsArguments  `alloy:"query_details,block,optional"`
	ExplainPlansArguments  ExplainPlansArguments  `alloy:"explain_plans,block,optional"`
	HealthCheckArguments   HealthCheckArguments   `alloy:"health_check,block,optional"`
}

type CloudProvider struct {
	AWS   *AWSCloudProviderInfo   `alloy:"aws,block,optional"`
	Azure *AzureCloudProviderInfo `alloy:"azure,block,optional"`
	GCP   *GCPCloudProviderInfo   `alloy:"gcp,block,optional"`
}

type AWSCloudProviderInfo struct {
	ARN string `alloy:"arn,attr"`
}

type AzureCloudProviderInfo struct {
	SubscriptionID string `alloy:"subscription_id,attr"`
	ResourceGroup  string `alloy:"resource_group,attr"`
	ServerName     string `alloy:"server_name,attr,optional"`
}

type GCPCloudProviderInfo struct {
	ConnectionName string `alloy:"connection_name,attr"`
}

type SchemaDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

type QueryMetricsArguments struct {
	CollectInterval    time.Duration `alloy:"collect_interval,attr,optional"`
	StatementsLimit    int           `alloy:"statements_limit,attr,optional"`
	StatementsLookback time.Duration `alloy:"statements_lookback,attr,optional"`
}

type QueryDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

type QuerySamplesArguments struct {
	CollectInterval       time.Duration `alloy:"collect_interval,attr,optional"`
	DisableQueryRedaction bool          `alloy:"disable_query_redaction,attr,optional"`
}

type ExplainPlansArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

type HealthCheckArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

func defaultArguments() Arguments {
	return Arguments{
		QueryTimeout: defaultQueryTimeout,

		ExcludeSchemas:     database_observability.DefaultExcludedSchemas(),
		ExcludeDatabases:   database_observability.DefaultExcludedDatabases(),
		ExcludeUsers:       database_observability.DefaultExcludedUsers(),
		ExcludeCurrentUser: true,

		SchemaDetailsArguments: SchemaDetailsArguments{
			CollectInterval: 1 * time.Minute,
		},

		QueryMetricsArguments: QueryMetricsArguments{
			CollectInterval:    1 * time.Minute,
			StatementsLimit:    50,
			StatementsLookback: 1 * time.Hour,
		},

		QueryDetailsArguments: QueryDetailsArguments{
			CollectInterval: 1 * time.Minute,
		},

		QuerySamplesArguments: QuerySamplesArguments{
			CollectInterval: 10 * time.Second,
		},

		ExplainPlansArguments: ExplainPlansArguments{
			CollectInterval: 1 * time.Minute,
		},

		HealthCheckArguments: HealthCheckArguments{
			CollectInterval: 1 * time.Hour,
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

	if a.QueryTimeout <= 0 {
		return fmt.Errorf("query_timeout must be greater than zero")
	}

	if enableOrDisableCollectors(*a)[collector.QueryMetricsCollector] {
		if a.QueryMetricsArguments.CollectInterval <= 0 {
			return fmt.Errorf("query_metrics.collect_interval must be greater than zero")
		}
		if a.QueryMetricsArguments.StatementsLimit <= 0 {
			return fmt.Errorf("query_metrics.statements_limit must be greater than zero")
		}
		lookback := a.QueryMetricsArguments.StatementsLookback
		if lookback < time.Second || lookback > math.MaxInt32*time.Second {
			return fmt.Errorf("query_metrics.statements_lookback must be between 1s and %ds", math.MaxInt32)
		}
	}

	if enableOrDisableCollectors(*a)[collector.QueryDetailsCollector] {
		if a.QueryDetailsArguments.CollectInterval <= 0 {
			return fmt.Errorf("query_details.collect_interval must be greater than zero")
		}
	}

	if enableOrDisableCollectors(*a)[collector.QuerySamplesCollector] {
		if a.QuerySamplesArguments.CollectInterval <= 0 {
			return fmt.Errorf("query_samples.collect_interval must be greater than zero")
		}
	}

	if enableOrDisableCollectors(*a)[collector.ExplainPlansCollector] {
		if a.ExplainPlansArguments.CollectInterval <= 0 {
			return fmt.Errorf("explain_plans.collect_interval must be greater than zero")
		}
	}

	// health_check is always enabled, so this isn't gated by enableOrDisableCollectors.
	if a.HealthCheckArguments.CollectInterval <= 0 {
		return fmt.Errorf("health_check.collect_interval must be greater than zero")
	}

	if a.CloudProvider != nil {
		count := 0
		if a.CloudProvider.AWS != nil {
			count++
		}
		if a.CloudProvider.Azure != nil {
			count++
		}
		if a.CloudProvider.GCP != nil {
			count++
		}
		if count > 1 {
			return fmt.Errorf("cloud_provider: at most one of aws, azure, or gcp must be specified")
		}
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
		consumeCtx, cancel = context.WithCancel(ctx)
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

	connectCtx, cancelConnect := context.WithTimeout(ctx, databaseConnectTimeout)
	err = dbConnection.PingContext(connectCtx)
	cancelConnect()
	if err != nil {
		dbConnection.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}
	c.dbConnection = dbConnection

	queryCtx, cancelQuery := context.WithTimeout(ctx, c.args.QueryTimeout)
	var serverName, machineName, engineVersion sql.NullString
	err = c.dbConnection.QueryRowContext(queryCtx, selectServerInfo).Scan(&serverName, &machineName, &engineVersion)
	cancelQuery()
	if err != nil {
		return fmt.Errorf("failed to query server information: %w", err)
	}

	excludeCurrentUser := c.args.ExcludeCurrentUser && enableOrDisableCollectors(c.args)[collector.QuerySamplesCollector]
	effectiveExcludeUsers, err := resolveExcludeUsers(ctx, c.dbConnection, c.args.QueryTimeout, c.args.ExcludeUsers, excludeCurrentUser)
	if err != nil {
		return fmt.Errorf("failed to resolve current login for query_samples user exclusion: %w", err)
	}

	generatedServerID := fmt.Sprintf("%x", sha256.Sum256(fmt.Appendf(nil, "%s:%s", serverName.String, machineName.String)))

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

	if err := c.startCollectors(ctx, generatedServerID, engineVersion.String, cp, effectiveExcludeUsers); err != nil {
		return fmt.Errorf("failed to start collectors: %w", err)
	}

	return nil
}

func enableOrDisableCollectors(a Arguments) map[string]bool {
	collectors := map[string]bool{
		collector.SchemaDetailsCollector: true,
		collector.QueryMetricsCollector:  true,
		collector.QuerySamplesCollector:  true,
		collector.QueryDetailsCollector:  true,
		collector.ExplainPlansCollector:  true,
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
func (c *Component) startCollectors(ctx context.Context, serverID string, engineVersion string, cloudProviderInfo *database_observability.CloudProvider, effectiveExcludeUsers []string) error {
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
			QueryTimeout:     c.args.QueryTimeout,
			ExcludeSchemas:   c.args.ExcludeSchemas,
			ExcludeDatabases: c.args.ExcludeDatabases,
			EntryHandler:     entryHandler,
			Logger:           c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.SchemaDetailsCollector, "create", err)
		} else {
			if err := stCollector.Start(ctx); err != nil {
				logStartError(collector.SchemaDetailsCollector, "start", err)
			}
			c.collectors = append(c.collectors, stCollector)
		}
	}

	var queryMetricsCollector *collector.QueryMetrics
	if collectors[collector.QueryMetricsCollector] {
		qmCollector, err := collector.NewQueryMetrics(collector.QueryMetricsArguments{
			DB:              c.dbConnection,
			Registry:        c.registry,
			QueryTimeout:    c.args.QueryTimeout,
			CollectInterval: c.args.QueryMetricsArguments.CollectInterval,
			Limit:           c.args.QueryMetricsArguments.StatementsLimit,
			Lookback:        c.args.QueryMetricsArguments.StatementsLookback,
			Logger:          c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.QueryMetricsCollector, "create", err)
		} else {
			queryMetricsCollector = qmCollector
			if err := qmCollector.Start(ctx); err != nil {
				logStartError(collector.QueryMetricsCollector, "start", err)
			}
			c.collectors = append(c.collectors, qmCollector)
		}
	}

	if collectors[collector.QuerySamplesCollector] {
		qsArgs := collector.QuerySamplesArguments{
			DB:                    c.dbConnection,
			CollectInterval:       c.args.QuerySamplesArguments.CollectInterval,
			QueryTimeout:          c.args.QueryTimeout,
			EntryHandler:          entryHandler,
			Logger:                c.opts.Logger,
			DisableQueryRedaction: c.args.QuerySamplesArguments.DisableQueryRedaction,
			ExcludeDatabases:      c.args.ExcludeDatabases,
			ExcludeUsers:          effectiveExcludeUsers,
		}
		if queryMetricsCollector != nil {
			qsArgs.Tracker = queryMetricsCollector.Tracker()
		}

		qsCollector, err := collector.NewQuerySamples(qsArgs)
		if err != nil {
			logStartError(collector.QuerySamplesCollector, "create", err)
		} else {
			if err := qsCollector.Start(ctx); err != nil {
				logStartError(collector.QuerySamplesCollector, "start", err)
			}
			c.collectors = append(c.collectors, qsCollector)
		}
	}

	if collectors[collector.QueryDetailsCollector] {
		qdArgs := collector.QueryDetailsArguments{
			DB:              c.dbConnection,
			CollectInterval: c.args.QueryDetailsArguments.CollectInterval,
			QueryTimeout:    c.args.QueryTimeout,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
		}

		// NOTE: this might change in the future, but for now query_details has
		// an hard dependency on query_metrics to determine which query_hashes to log.
		if queryMetricsCollector != nil {
			qdArgs.Tracker = queryMetricsCollector.Tracker()
		}

		qdCollector, err := collector.NewQueryDetails(qdArgs)
		if err != nil {
			logStartError(collector.QueryDetailsCollector, "create", err)
		} else {
			if err := qdCollector.Start(ctx); err != nil {
				logStartError(collector.QueryDetailsCollector, "start", err)
			}
			c.collectors = append(c.collectors, qdCollector)
		}
	}

	if collectors[collector.ExplainPlansCollector] {
		epArgs := collector.ExplainPlansArguments{
			DB:              c.dbConnection,
			CollectInterval: c.args.ExplainPlansArguments.CollectInterval,
			QueryTimeout:    c.args.QueryTimeout,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
		}

		// explain_plans has the same hard dependency on query_metrics's tracked
		// query_hash set as query_details does, so its candidate set stays
		// bounded by query_metrics.statements_limit rather than being an
		// independent, unbounded discovery query.
		if queryMetricsCollector != nil {
			epArgs.Tracker = queryMetricsCollector.Tracker()
		}

		epCollector, err := collector.NewExplainPlans(epArgs)
		if err != nil {
			logStartError(collector.ExplainPlansCollector, "create", err)
		} else {
			if err := epCollector.Start(ctx); err != nil {
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
		DB:            c.dbConnection,
	})
	if err != nil {
		logStartError(collector.ConnectionInfoName, "create", err)
	} else {
		if err := ciCollector.Start(ctx); err != nil {
			logStartError(collector.ConnectionInfoName, "start", err)
		}
		c.collectors = append(c.collectors, ciCollector)
	}

	// HealthCheck collector is always enabled
	hcCollector, err := collector.NewHealthCheck(collector.HealthCheckArguments{
		DB:               c.dbConnection,
		CollectInterval:  c.args.HealthCheckArguments.CollectInterval,
		QueryTimeout:     c.args.QueryTimeout,
		ExcludeDatabases: c.args.ExcludeDatabases,
		EntryHandler:     entryHandler,
		Logger:           c.opts.Logger,
	})
	if err != nil {
		logStartError(collector.HealthCheckCollector, "create", err)
	} else {
		if err := hcCollector.Start(ctx); err != nil {
			logStartError(collector.HealthCheckCollector, "start", err)
		}
		c.collectors = append(c.collectors, hcCollector)
	}

	if len(startErrors) > 0 {
		return fmt.Errorf("failed to start some collectors: %s", strings.Join(startErrors, ", "))
	}

	return nil
}

// resolveExcludeUsers merges the configured user exclusions with Alloy's own
// connecting login (when excludeCurrentUser is set) so query_samples can drop
// Alloy's own monitoring sessions.
//
// ORIGINAL_LOGIN() pairs with the sys.dm_exec_sessions.original_login_name
// column that the query_samples filter matches against.
func resolveExcludeUsers(ctx context.Context, db *sql.DB, queryTimeout time.Duration, configured []string, excludeCurrentUser bool) ([]string, error) {
	effective := slices.Clone(configured)
	if !excludeCurrentUser {
		return effective, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var originalLogin sql.NullString
	if err := db.QueryRowContext(queryCtx, selectOriginalLogin).Scan(&originalLogin); err != nil {
		return nil, fmt.Errorf("failed to query original login: %w", err)
	}
	if !originalLogin.Valid || originalLogin.String == "" {
		return nil, fmt.Errorf("failed to query original login: empty result")
	}
	if !slices.Contains(effective, originalLogin.String) {
		effective = append(effective, originalLogin.String)
	}
	return effective, nil
}

func (c *Component) Handler() http.Handler {
	return util.PromHTTPHandlerFor(c.registry, c.opts.Logger, promhttp.HandlerOpts{})
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
		"engine":    model.LabelValue(collector.EngineName),
	}).Wrap(entryHandler)

	return entryHandler
}
