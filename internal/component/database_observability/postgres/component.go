package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	pg_collector "github.com/prometheus-community/postgres_exporter/collector"
	pg_exporter "github.com/prometheus-community/postgres_exporter/exporter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	exporter_postgres "github.com/grafana/alloy/internal/component/prometheus/exporter/postgres"
	"github.com/grafana/alloy/internal/featuregate"
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
		Stability: featuregate.StabilityGenerallyAvailable,
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
	DataSourceName     alloytypes.Secret   `alloy:"data_source_name,attr,optional"`
	ForwardTo          []loki.LogsReceiver `alloy:"forward_to,attr"`
	Targets            []discovery.Target  `alloy:"targets,attr,optional"`
	EnableCollectors   []string            `alloy:"enable_collectors,attr,optional"`
	DisableCollectors  []string            `alloy:"disable_collectors,attr,optional"`
	ExcludeDatabases   []string            `alloy:"exclude_databases,attr,optional"`
	ExcludeUsers       []string            `alloy:"exclude_users,attr,optional"`
	ExcludeCurrentUser bool                `alloy:"exclude_current_user,attr,optional"`

	Databases []DatabaseArguments `alloy:"database_instance,block,optional"`

	CloudProvider          *CloudProvider               `alloy:"cloud_provider,block,optional"`
	QuerySampleArguments   QuerySampleArguments         `alloy:"query_samples,block,optional"`
	QueryDetailsArguments  QueryDetailsArguments        `alloy:"query_details,block,optional"`
	SchemaDetailsArguments SchemaDetailsArguments       `alloy:"schema_details,block,optional"`
	ExplainPlansArguments  ExplainPlansArguments        `alloy:"explain_plans,block,optional"`
	HealthCheckArguments   HealthCheckArguments         `alloy:"health_check,block,optional"`
	Logs                   LogsArguments                `alloy:"logs,block,optional"`
	PrometheusExporter     *PrometheusExporterArguments `alloy:"prometheus_exporter,block,optional"`
}

// DatabaseArguments configures one monitored database. When one or more
// `database_instance` blocks are defined, the top-level `data_source_name`, `targets`,
// and `cloud_provider` arguments must not be set.
type DatabaseArguments struct {
	Name           string            `alloy:",label"`
	DataSourceName alloytypes.Secret `alloy:"data_source_name,attr"`
	CloudProvider  *CloudProvider    `alloy:"cloud_provider,block,optional"`
}

type LogsArguments struct {
	EnableErrorLogsProcessing bool `alloy:"enable_error_logs_processing,attr,optional"`
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

type QuerySampleArguments struct {
	CollectInterval       time.Duration `alloy:"collect_interval,attr,optional"`
	DisableQueryRedaction bool          `alloy:"disable_query_redaction,attr,optional"`
	// Deprecated: `query_samples.exclude_current_user` is deprecated in favour of the top-level setting.
	// When set (non-nil), it takes precedence over the top-level setting for the
	// query_samples collector only and preserves the legacy behaviour.
	ExcludeCurrentUser            *bool `alloy:"exclude_current_user,attr,optional"`
	EnablePreClassifiedWaitEvents bool  `alloy:"enable_pre_classified_wait_events,attr,optional"`
}

type QueryDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	StatementsLimit int           `alloy:"statements_limit,attr,optional"`
}

type SchemaDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`

	// Deprecated settings
	CacheEnabled *bool          `alloy:"cache_enabled,attr,optional"`
	CacheSize    *int           `alloy:"cache_size,attr,optional"`
	CacheTTL     *time.Duration `alloy:"cache_ttl,attr,optional"`
}

func defaultArguments() Arguments {
	return Arguments{
		ExcludeDatabases:   database_observability.DefaultExcludedDatabases(),
		ExcludeUsers:       database_observability.DefaultExcludedUsers(),
		ExcludeCurrentUser: true,
		QuerySampleArguments: QuerySampleArguments{
			CollectInterval:       10 * time.Second,
			DisableQueryRedaction: false,
		},
		QueryDetailsArguments: QueryDetailsArguments{
			CollectInterval: 1 * time.Minute,
			StatementsLimit: 100,
		},
		SchemaDetailsArguments: SchemaDetailsArguments{
			CollectInterval: 1 * time.Minute,
		},
		ExplainPlansArguments: ExplainPlansArguments{
			CollectInterval: 1 * time.Minute,
			PerCollectRatio: 1.0,
		},
		HealthCheckArguments: HealthCheckArguments{
			CollectInterval: 1 * time.Hour,
		},
	}
}

type ExplainPlansArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	PerCollectRatio float64       `alloy:"per_collect_ratio,attr,optional"`
}

type HealthCheckArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
}

// PrometheusExporterArguments configures the embedded postgres_exporter scrapers.
// When this block is present, postgres_exporter metrics are served alongside the
// component's own metrics at the same /metrics endpoint.
//
// It is a distinct type (not an embedded struct) because the Alloy syntax
// system does not support anonymous/embedded fields.
// Note: data_source_names is ignored; the component's data_source_name is always used.
type PrometheusExporterArguments exporter_postgres.Arguments

func (a *PrometheusExporterArguments) SetToDefault() {
	*a = PrometheusExporterArguments(exporter_postgres.DefaultArguments)
}

func (a *PrometheusExporterArguments) Validate() error {
	args := exporter_postgres.Arguments(*a)
	return args.Validate()
}

func (a *Arguments) SetToDefault() {
	*a = defaultArguments()
}

// databaseNameRegex matches the identifiers the Alloy syntax parser accepts
// as block labels. It's checked again here as a backstop for programmatically
// constructed Arguments, and because the label is used in the per-database
// metrics URL path.
var databaseNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (a *Arguments) Validate() error {
	if len(a.Databases) == 0 {
		_, err := pq.ParseURL(string(a.DataSourceName)) //nolint:staticcheck // pq.ParseURL is deprecated but needed for URL validation
		if err != nil {
			return err
		}
		if a.PrometheusExporter != nil && len(a.Targets) > 0 {
			return fmt.Errorf("prometheus_exporter and targets are mutually exclusive: use prometheus_exporter to embed the exporter, or targets to scrape an external one")
		}
		return validateCloudProvider(a.CloudProvider)
	}

	// database_instance blocks are defined: per-database settings must not also be set
	// at the top level.
	if a.DataSourceName != "" {
		return fmt.Errorf("data_source_name and database_instance blocks are mutually exclusive")
	}
	if len(a.Targets) > 0 {
		return fmt.Errorf("targets and database_instance blocks are mutually exclusive: database_instance blocks always use the embedded exporter")
	}
	if a.CloudProvider != nil {
		return fmt.Errorf("cloud_provider and database_instance blocks are mutually exclusive: set cloud_provider on each database_instance block")
	}

	names := make(map[string]struct{}, len(a.Databases))
	servers := make(map[string]string, len(a.Databases))
	for _, db := range a.Databases {
		if !databaseNameRegex.MatchString(db.Name) {
			return fmt.Errorf("database_instance block label %q must be a valid identifier (letters, digits, and underscores, not starting with a digit)", db.Name)
		}
		if _, ok := names[db.Name]; ok {
			return fmt.Errorf("duplicate database_instance block label %q", db.Name)
		}
		names[db.Name] = struct{}{}

		key, err := instanceKey(string(db.DataSourceName))
		if err != nil {
			return fmt.Errorf("database_instance %q: %w", db.Name, err)
		}
		if other, ok := servers[key]; ok {
			return fmt.Errorf("database_instance blocks %q and %q resolve to the same server %q", other, db.Name, key)
		}
		servers[key] = db.Name

		if err := validateCloudProvider(db.CloudProvider); err != nil {
			return fmt.Errorf("database_instance %q: %w", db.Name, err)
		}
	}
	return nil
}

func validateCloudProvider(cp *CloudProvider) error {
	if cp == nil {
		return nil
	}
	count := 0
	if cp.AWS != nil {
		count++
	}
	if cp.Azure != nil {
		count++
	}
	if cp.GCP != nil {
		count++
	}
	if count > 1 {
		return fmt.Errorf("cloud_provider: at most one of aws, azure, or gcp must be specified")
	}
	return nil
}

type Exports struct {
	Targets      []discovery.Target `alloy:"targets,attr"`
	LogsReceiver loki.LogsReceiver  `alloy:"logs_receiver,attr,optional"`
	// LogsReceivers holds one logs receiver per database_instance block,
	// keyed by block label. It's empty in the single-DSN form, which exports
	// its receiver as logs_receiver instead.
	LogsReceivers map[string]loki.LogsReceiver `alloy:"logs_receivers,attr,optional"`
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
	opts    component.Options
	args    Arguments
	handler loki.LogsReceiver
	fanout  *loki.Fanout
	mut     sync.RWMutex
	openSQL func(driverName, dataSourceName string) (*sql.DB, error)

	// logsReceivers holds the receiver pump of each configured database,
	// keyed by database name ("" in the single-DSN form). Pumps are created
	// on demand and never removed, so the exported receivers stay valid for
	// downstream components across Updates. Guarded by mut.
	logsReceivers map[string]*receiverPump
	// pumpStop stops the receiver pump goroutines. It's closed exactly once:
	// when Run shuts down, or when the initial Update fails and the component
	// is discarded.
	pumpStop chan struct{}
	pumpWg   sync.WaitGroup

	// instances holds one dbInstance per configured database. The slice is
	// replaced wholesale on Update and stored atomically so that Handler can
	// read it without blocking on mut while an Update is connecting. Mutation
	// of dbInstance fields is guarded by mut.
	instances atomic.Pointer[[]*dbInstance]
	// handlerMux serves the per-database metrics endpoints. It's rebuilt
	// whenever the instances are replaced.
	handlerMux atomic.Pointer[http.ServeMux]
}

func (c *Component) loadInstances() []*dbInstance {
	if p := c.instances.Load(); p != nil {
		return *p
	}
	return nil
}

func (c *Component) storeInstances(instances []*dbInstance) {
	mux := http.NewServeMux()
	for _, inst := range instances {
		mux.Handle(metricsPath(inst.cfg.name), promhttp.HandlerFor(inst.registry, promhttp.HandlerOpts{}))
	}
	c.instances.Store(&instances)
	c.handlerMux.Store(mux)
}

// ensurePumps creates and starts the receiver pumps missing for the given
// configs. Existing pumps are reused so the exported receivers stay stable.
// Must be called with c.mut locked.
func (c *Component) ensurePumps(cfgs []databaseConfig) {
	for _, cfg := range cfgs {
		if _, ok := c.logsReceivers[cfg.name]; ok {
			continue
		}
		pump := newReceiverPump()
		c.logsReceivers[cfg.name] = pump
		c.pumpWg.Go(func() { pump.run(c.pumpStop) })
	}
}

func (c *Component) stopPumps() {
	close(c.pumpStop)
	c.pumpWg.Wait()
}

func New(opts component.Options, args Arguments) (*Component, error) {
	return new(opts, args, sql.Open)
}

func new(opts component.Options, args Arguments, openFn func(driverName, dataSourceName string) (*sql.DB, error)) (*Component, error) {
	c := &Component{
		opts:          opts,
		args:          args,
		fanout:        loki.NewFanout(args.ForwardTo),
		handler:       loki.NewLogsReceiver(),
		openSQL:       openFn,
		logsReceivers: make(map[string]*receiverPump),
		pumpStop:      make(chan struct{}),
	}

	// Export the logs receivers immediately, before any database is
	// connected, so that loki.source.* components can wire to them even when
	// a database is initially unreachable.
	c.mut.Lock()
	c.ensurePumps(args.databaseConfigs())
	c.exportState()
	c.mut.Unlock()

	if err := c.Update(args); err != nil {
		c.stopPumps()
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

			c.stopInstances(c.loadInstances())
		})

		// Stop the receiver pumps only after the instances are stopped, so
		// the exported receivers stay drained for the whole drain window.
		c.stopPumps()
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
				needsReconnect := false
				for _, inst := range c.loadInstances() {
					if len(inst.collectors) == 0 {
						needsReconnect = true
						break
					}
				}
				c.mut.RUnlock()

				if needsReconnect {
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

func (c *Component) reportInstanceError(inst *dbInstance, errorMsg string, err error) {
	if inst.cfg.name != "" {
		errorMsg = fmt.Sprintf("database %q: %s", inst.cfg.name, errorMsg)
	}
	c.opts.Logger.Error(fmt.Sprintf("%s: %+v", errorMsg, err))
	inst.healthErr.Store(fmt.Sprintf("%s: %+v", errorMsg, err))
}

func (c *Component) tryReconnect(ctx context.Context) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	var errs []error
	for _, inst := range c.loadInstances() {
		if len(inst.collectors) > 0 {
			continue
		}
		if err := c.connectAndStartCollectors(ctx, inst); err != nil {
			c.reportInstanceError(inst, "reconnection failed", err)
			errs = append(errs, err)
			continue
		}
		inst.healthErr.Store("")
	}

	c.exportState()
	return errors.Join(errs...)
}

// stopInstances stops the collectors of the given instances, releases their
// embedded exporters, and closes their database connections. Must be called
// with c.mut locked.
func (c *Component) stopInstances(instances []*dbInstance) {
	for _, inst := range instances {
		if pump := c.logsReceivers[inst.cfg.name]; pump != nil {
			pump.clearTarget()
		}
		for _, collector := range inst.collectors {
			collector.Stop()
		}
		inst.collectors = nil
		inst.cleanupExporterCollectors()
		if inst.dbConnection != nil {
			inst.dbConnection.Close()
			inst.dbConnection = nil
		}
	}
}

// exportState publishes the targets of all connected database instances and
// the exported logs receivers. Must be called with c.mut locked.
func (c *Component) exportState() {
	targets := make([]discovery.Target, 0)
	for _, inst := range c.loadInstances() {
		targets = append(targets, inst.exportedTargets...)
	}

	exports := Exports{Targets: targets}
	if len(c.args.Databases) == 0 {
		if pump := c.logsReceivers[""]; pump != nil {
			exports.LogsReceiver = pump.exported
		}
	} else {
		receivers := make(map[string]loki.LogsReceiver, len(c.args.Databases))
		for _, db := range c.args.Databases {
			if pump := c.logsReceivers[db.Name]; pump != nil {
				receivers[db.Name] = pump.exported
			}
		}
		exports.LogsReceivers = receivers
	}
	c.opts.OnStateChange(exports)
}

// dbConnectTimeout bounds the connectivity check and the server-info queries
// when (re)connecting to a database, so that one unresponsive server can't
// stall an Update for long while it holds the component lock.
const dbConnectTimeout = 10 * time.Second

// connectAndStartCollectors handles the full connection lifecycle of one
// database instance: closes its old connection, opens a new one, queries
// server info, and starts its collectors.
// Must be called with c.mut locked
func (c *Component) connectAndStartCollectors(ctx context.Context, inst *dbInstance) error {
	if inst.dbConnection != nil {
		inst.dbConnection.Close()
		inst.dbConnection = nil
	}
	inst.cleanupExporterCollectors()

	dbConnection, err := c.openSQL("postgres", string(inst.cfg.dsn))
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	if dbConnection == nil {
		return fmt.Errorf("nil DB connection")
	}

	connectCtx, cancel := context.WithTimeout(ctx, dbConnectTimeout)
	defer cancel()
	if err = dbConnection.PingContext(connectCtx); err != nil {
		dbConnection.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}
	inst.dbConnection = dbConnection

	rs := dbConnection.QueryRowContext(connectCtx, selectServerInfo)
	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to query engine version: %w", err)
	}

	var systemID, systemIP, systemPort, engineVersion sql.NullString
	if err := rs.Scan(&systemID, &systemIP, &systemPort, &engineVersion); err != nil {
		return fmt.Errorf("failed to scan engine version: %w", err)
	}

	generatedSystemID := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", systemID.String, systemIP.String, systemPort.String))))

	// Get the current user and compute the effective exclude users list.
	var currentUser sql.NullString
	if c.args.ExcludeCurrentUser {
		if err := dbConnection.QueryRowContext(connectCtx, "SELECT current_user").Scan(&currentUser); err != nil {
			return fmt.Errorf("failed to query current_user: %w", err)
		}
	}
	effectiveExcludeUsers := slices.Clone(c.args.ExcludeUsers)
	if currentUser.Valid {
		if !slices.Contains(effectiveExcludeUsers, currentUser.String) {
			effectiveExcludeUsers = append(effectiveExcludeUsers, currentUser.String)
		}
	}

	var cp *database_observability.CloudProvider
	if inst.cfg.cloudProvider != nil {
		cloudProvider, err := populateCloudProviderFromConfig(inst.cfg.cloudProvider)
		if err != nil {
			return fmt.Errorf("failed to collect cloud provider information from config: %w", err)
		}
		cp = cloudProvider
	} else {
		cloudProvider, err := populateCloudProviderFromDSN(string(inst.cfg.dsn))
		if err != nil {
			return fmt.Errorf("failed to collect cloud provider information from DSN: %w", err)
		}
		cp = cloudProvider
	}

	if len(inst.cfg.targets) == 0 {
		exporterArgs := exporter_postgres.DefaultArguments
		if c.args.PrometheusExporter != nil {
			exporterArgs = exporter_postgres.Arguments(*c.args.PrometheusExporter)
		}
		dsn := string(inst.cfg.dsn)

		e := pg_exporter.NewExporter(
			[]string{dsn},
			c.opts.Logger,
			pg_exporter.DisableDefaultMetrics(exporterArgs.DisableDefaultMetrics),
			pg_exporter.WithUserQueriesPath(exporterArgs.CustomQueriesConfigPath),
			pg_exporter.DisableSettingsMetrics(exporterArgs.DisableSettingsMetrics),
			pg_exporter.AutoDiscoverDatabases(true),
			pg_exporter.ExcludeDatabases(c.args.ExcludeDatabases),
			pg_exporter.WithMetricPrefix("pg"),
		)
		if err := inst.registry.Register(e); err != nil {
			return fmt.Errorf("failed to register prometheus_exporter: %w", err)
		}
		inst.exporterCollectors = append(inst.exporterCollectors, e)

		if !exporterArgs.DisableDefaultMetrics {
			collectorOpts := []pg_collector.Option{pg_collector.WithCollectionTimeout("10s")}
			if exporterArgs.StatStatementFlags != nil {
				collectorOpts = append(collectorOpts, pg_collector.WithStatStatementsConfig(pg_collector.StatStatementsConfig{
					IncludeQuery:     exporterArgs.StatStatementFlags.IncludeQuery,
					QueryLength:      exporterArgs.StatStatementFlags.QueryLength,
					Limit:            exporterArgs.StatStatementFlags.Limit,
					ExcludeDatabases: exporterArgs.StatStatementFlags.ExcludeDatabases,
					ExcludeUsers:     exporterArgs.StatStatementFlags.ExcludeUsers,
				}))
			}
			col, err := pg_collector.NewPostgresCollector(
				c.opts.Logger,
				c.args.ExcludeDatabases,
				dsn,
				exporterArgs.EnabledCollectors,
				collectorOpts...,
			)
			if err != nil {
				return fmt.Errorf("failed to create postgres collector: %w", err)
			}
			if err := inst.registry.Register(col); err != nil {
				return fmt.Errorf("failed to register postgres collector: %w", err)
			}
			inst.exporterCollectors = append(inst.exporterCollectors, col)
		}
	}

	allTargets := append([]discovery.Target{inst.baseTarget}, inst.cfg.targets...)
	targets := make([]discovery.Target, 0, len(allTargets))
	for _, t := range allTargets {
		builder := discovery.NewTargetBuilderFrom(t)
		if relabel.ProcessBuilder(builder, database_observability.GetRelabelingRules(generatedSystemID, cp)...) {
			targets = append(targets, builder.Target())
		}
	}
	inst.exportedTargets = targets

	if pump := c.logsReceivers[inst.cfg.name]; pump != nil {
		pump.clearTarget()
	}
	for _, collector := range inst.collectors {
		collector.Stop()
	}
	inst.collectors = nil

	if err := c.startCollectors(inst, generatedSystemID, engineVersion.String, cp, effectiveExcludeUsers); err != nil {
		return fmt.Errorf("failed to start collectors: %w", err)
	}

	return nil
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	// Build the new instances before touching any state, so that a failed
	// rebuild returns an error while the previous instances keep running.
	cfgs := newArgs.databaseConfigs()
	instances := make([]*dbInstance, 0, len(cfgs))
	for _, cfg := range cfgs {
		inst, err := newDBInstance(c.opts, cfg)
		if err != nil {
			return err
		}
		instances = append(instances, inst)
	}

	c.args = newArgs
	c.fanout.UpdateChildren(c.args.ForwardTo)
	c.ensurePumps(cfgs)

	// Replace the previous instances with the newly built ones.
	c.stopInstances(c.loadInstances())
	c.storeInstances(instances)

	for _, inst := range instances {
		if err := c.connectAndStartCollectors(context.Background(), inst); err != nil {
			c.reportInstanceError(inst, "failed to connect", err)
			continue
		}
		inst.healthErr.Store("")
	}

	c.exportState()
	return nil
}

func enableOrDisableCollectors(a Arguments) map[string]bool {
	// configurable collectors and their default enabled/disabled value
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

// startCollectors attempts to start all of the enabled collectors for a database instance.
// If one or more collectors fail to start, their errors are reported.
func (c *Component) startCollectors(inst *dbInstance, systemID string, engineVersion string, cloudProviderInfo *database_observability.CloudProvider, effectiveExcludeUsers []string) error {
	var startErrors []string

	logStartError := func(collectorName, action string, err error) {
		errorString := fmt.Sprintf("failed to %s %s collector: %+v", action, collectorName, err)
		c.opts.Logger.Error(errorString)
		startErrors = append(startErrors, errorString)
	}

	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), inst.instanceKey, systemID)

	var tableRegistry *collector.TableRegistry
	collectors := enableOrDisableCollectors(c.args)

	if collectors[collector.SchemaDetailsCollector] {
		if c.args.SchemaDetailsArguments.CacheEnabled != nil {
			c.opts.Logger.Warn("schema_details.cache_enabled is set, but the cache is deprecated and will be removed in a future version")
		}
		if c.args.SchemaDetailsArguments.CacheSize != nil {
			c.opts.Logger.Warn("schema_details.cache_size is set, but the cache is deprecated and will be removed in a future version")
		}
		if c.args.SchemaDetailsArguments.CacheTTL != nil {
			c.opts.Logger.Warn("schema_details.cache_ttl is set, but the cache is deprecated and will be removed in a future version")
		}

		stCollector, err := collector.NewSchemaDetails(collector.SchemaDetailsArguments{
			DB:               inst.dbConnection,
			DSN:              string(inst.cfg.dsn),
			CollectInterval:  c.args.SchemaDetailsArguments.CollectInterval,
			ExcludeDatabases: c.args.ExcludeDatabases,
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
		inst.collectors = append(inst.collectors, stCollector)
	}

	if collectors[collector.QueryDetailsCollector] {
		qCollector, err := collector.NewQueryDetails(collector.QueryDetailsArguments{
			DB:                        inst.dbConnection,
			CollectInterval:           c.args.QueryDetailsArguments.CollectInterval,
			StatementsLimit:           c.args.QueryDetailsArguments.StatementsLimit,
			ExcludeDatabases:          c.args.ExcludeDatabases,
			ExcludeUsers:              effectiveExcludeUsers,
			EntryHandler:              entryHandler,
			TableRegistry:             tableRegistry,
			EnableErrorLogsProcessing: c.args.Logs.EnableErrorLogsProcessing,
			Logger:                    c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.QueryDetailsCollector, "create", err)
		}
		if err := qCollector.Start(context.Background()); err != nil {
			logStartError(collector.QueryDetailsCollector, "start", err)
		}
		inst.collectors = append(inst.collectors, qCollector)
	}

	if collectors[collector.QuerySamplesCollector] {
		// For backward compatibility, give precedence to query_samples.exclude_current_user
		// setting over the top-level exclude_current_user: when set, preserve today's behavior;
		// when unset, inherit the top-level cascade.
		qsExcludeUsers := effectiveExcludeUsers
		qsExcludeCurrentUser := false
		if localExcludeCurrentUser := c.args.QuerySampleArguments.ExcludeCurrentUser; localExcludeCurrentUser != nil {
			c.opts.Logger.Warn("query_samples.exclude_current_user is deprecated; use the top-level exclude_current_user setting instead")
			qsExcludeUsers = c.args.ExcludeUsers
			qsExcludeCurrentUser = *localExcludeCurrentUser
		}

		aCollector, err := collector.NewQuerySamples(collector.QuerySamplesArguments{
			DB:                            inst.dbConnection,
			CollectInterval:               c.args.QuerySampleArguments.CollectInterval,
			ExcludeDatabases:              c.args.ExcludeDatabases,
			ExcludeUsers:                  qsExcludeUsers,
			EntryHandler:                  entryHandler,
			Logger:                        c.opts.Logger,
			DisableQueryRedaction:         c.args.QuerySampleArguments.DisableQueryRedaction,
			ExcludeCurrentUser:            qsExcludeCurrentUser,
			EnablePreClassifiedWaitEvents: c.args.QuerySampleArguments.EnablePreClassifiedWaitEvents,
		})
		if err != nil {
			logStartError(collector.QuerySamplesCollector, "create", err)
		}
		if err := aCollector.Start(context.Background()); err != nil {
			logStartError(collector.QuerySamplesCollector, "start", err)
		}
		inst.collectors = append(inst.collectors, aCollector)
	}

	// Connection Info collector is always enabled
	ciCollector, err := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
		DSN:           string(inst.cfg.dsn),
		Registry:      inst.registry,
		EngineVersion: engineVersion,
		CloudProvider: cloudProviderInfo,
		DB:            inst.dbConnection,
	})
	if err != nil {
		logStartError(collector.ConnectionInfoName, "create", err)
	}
	if err := ciCollector.Start(context.Background()); err != nil {
		logStartError(collector.ConnectionInfoName, "start", err)
	}

	inst.collectors = append(inst.collectors, ciCollector)

	if collectors[collector.ExplainPlanCollector] {
		epCollector, err := collector.NewExplainPlan(collector.ExplainPlansArguments{
			DB:               inst.dbConnection,
			DSN:              string(inst.cfg.dsn),
			ScrapeInterval:   c.args.ExplainPlansArguments.CollectInterval,
			PerScrapeRatio:   c.args.ExplainPlansArguments.PerCollectRatio,
			ExcludeDatabases: c.args.ExcludeDatabases,
			ExcludeUsers:     effectiveExcludeUsers,
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
		inst.collectors = append(inst.collectors, epCollector)
	}

	// HealthCheck collector is always enabled
	hcCollector, err := collector.NewHealthCheck(collector.HealthCheckArguments{
		DB:               inst.dbConnection,
		CollectInterval:  c.args.HealthCheckArguments.CollectInterval,
		ExcludeDatabases: c.args.ExcludeDatabases,
		ExcludeUsers:     effectiveExcludeUsers,
		EntryHandler:     entryHandler,
		Logger:           c.opts.Logger,
	})
	if err != nil {
		logStartError(collector.HealthCheckCollector, "create", err)
	} else {
		if err := hcCollector.Start(context.Background()); err != nil {
			logStartError(collector.HealthCheckCollector, "start", err)
		}
		inst.collectors = append(inst.collectors, hcCollector)
	}

	// Logs collector is always enabled
	logsCollector, err := collector.NewLogs(collector.LogsArguments{
		Receiver:                  inst.logsReceiver,
		EntryHandler:              entryHandler,
		Logger:                    c.opts.Logger,
		Registry:                  inst.registry,
		ExcludeDatabases:          c.args.ExcludeDatabases,
		ExcludeUsers:              effectiveExcludeUsers,
		EnableErrorLogsProcessing: c.args.Logs.EnableErrorLogsProcessing,
		DB:                        inst.dbConnection,
	})
	if err != nil {
		logStartError(collector.LogsCollector, "create", err)
	} else {
		if err := logsCollector.Start(context.Background()); err != nil {
			logStartError(collector.LogsCollector, "start", err)
		} else if pump := c.logsReceivers[inst.cfg.name]; pump != nil {
			// The logs collector is now draining the instance receiver: start
			// forwarding the exported receiver's entries into it.
			pump.setTarget(inst.logsReceiver)
		}
		inst.collectors = append(inst.collectors, logsCollector)
	}

	if len(startErrors) > 0 {
		return fmt.Errorf("failed to start some collectors: %s", strings.Join(startErrors, ", "))
	}

	return nil
}

func (c *Component) Handler() http.Handler {
	if mux := c.handlerMux.Load(); mux != nil {
		return mux
	}
	return http.NewServeMux()
}

func (c *Component) CurrentHealth() component.Health {
	var healthErrs []string
	for _, inst := range c.loadInstances() {
		if err := inst.healthErr.Load(); err != "" {
			healthErrs = append(healthErrs, err)
		}
	}
	if len(healthErrs) > 0 {
		return component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    strings.Join(healthErrs, "; "),
			UpdateTime: time.Now(),
		}
	}

	var unhealthyCollectors []string

	c.mut.RLock()
	for _, inst := range c.loadInstances() {
		for _, collector := range inst.collectors {
			if collector.Stopped() {
				name := collector.Name()
				if inst.cfg.name != "" {
					name = inst.cfg.name + "/" + name
				}
				unhealthyCollectors = append(unhealthyCollectors, name)
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

func addLokiLabels(entryHandler loki.EntryHandler, instanceKey string, systemID string) loki.EntryHandler {
	entryHandler = loki.AddLabelsMiddleware(model.LabelSet{
		"job":       database_observability.JobName,
		"instance":  model.LabelValue(instanceKey),
		"server_id": model.LabelValue(systemID),
	}).Wrap(entryHandler)

	return entryHandler
}
