package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-sql-driver/mysql"
	"github.com/grafana/ckit/shard"
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
	"github.com/grafana/alloy/internal/service/cluster"
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
	DataSourceName                alloytypes.Secret   `alloy:"data_source_name,attr,optional"`
	ForwardTo                     []loki.LogsReceiver `alloy:"forward_to,attr"`
	Targets                       []discovery.Target  `alloy:"targets,attr,optional"`
	EnableCollectors              []string            `alloy:"enable_collectors,attr,optional"`
	DisableCollectors             []string            `alloy:"disable_collectors,attr,optional"`
	ExcludeSchemas                []string            `alloy:"exclude_schemas,attr,optional"`
	AllowUpdatePerfSchemaSettings bool                `alloy:"allow_update_performance_schema_settings,attr,optional"`

	Databases []DatabaseArguments `alloy:"database_instance,block,optional"`

	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`

	CloudProvider           *CloudProvider               `alloy:"cloud_provider,block,optional"`
	SetupConsumersArguments SetupConsumersArguments      `alloy:"setup_consumers,block,optional"`
	SetupActorsArguments    SetupActorsArguments         `alloy:"setup_actors,block,optional"`
	QueryDetailsArguments   QueryDetailsArguments        `alloy:"query_details,block,optional"`
	SchemaDetailsArguments  SchemaDetailsArguments       `alloy:"schema_details,block,optional"`
	ExplainPlansArguments   ExplainPlansArguments        `alloy:"explain_plans,block,optional"`
	LocksArguments          LocksArguments               `alloy:"locks,block,optional"`
	QuerySamplesArguments   QuerySamplesArguments        `alloy:"query_samples,block,optional"`
	HealthCheckArguments    HealthCheckArguments         `alloy:"health_check,block,optional"`
	PrometheusExporter      *PrometheusExporterArguments `alloy:"prometheus_exporter,block,optional"`
}

// DatabaseArguments configures one monitored database. When one or more
// `database_instance` blocks are defined, the top-level `data_source_name`, `targets`,
// and `cloud_provider` arguments must not be set.
type DatabaseArguments struct {
	Name           string            `alloy:",label"`
	DataSourceName alloytypes.Secret `alloy:"data_source_name,attr"`
	CloudProvider  *CloudProvider    `alloy:"cloud_provider,block,optional"`
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

type QueryDetailsArguments struct {
	CollectInterval time.Duration `alloy:"collect_interval,attr,optional"`
	StatementsLimit int           `alloy:"statements_limit,attr,optional"`
}

type SchemaDetailsArguments struct {
	CollectInterval time.Duration  `alloy:"collect_interval,attr,optional"`
	CacheEnabled    *bool          `alloy:"cache_enabled,attr,optional"`
	CacheSize       *int           `alloy:"cache_size,attr,optional"`
	CacheTTL        *time.Duration `alloy:"cache_ttl,attr,optional"`
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
	CollectInterval               time.Duration `alloy:"collect_interval,attr,optional"`
	DisableQueryRedaction         bool          `alloy:"disable_query_redaction,attr,optional"`
	AutoEnableSetupConsumers      bool          `alloy:"auto_enable_setup_consumers,attr,optional"`
	SetupConsumersCheckInterval   time.Duration `alloy:"setup_consumers_check_interval,attr,optional"`
	SampleMinDuration             time.Duration `alloy:"sample_min_duration,attr,optional"`
	WaitEventMinDuration          time.Duration `alloy:"wait_event_min_duration,attr,optional"`
	EnablePreClassifiedWaitEvents bool          `alloy:"enable_pre_classified_wait_events,attr,optional"`
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

func (a *PrometheusExporterArguments) Validate() error {
	args := exporter_mysql.Arguments(*a)
	return args.Validate()
}

func defaultArguments() Arguments {
	return Arguments{
		ExcludeSchemas:                database_observability.DefaultExcludedSchemas(),
		AllowUpdatePerfSchemaSettings: false,

		QueryDetailsArguments: QueryDetailsArguments{
			CollectInterval: 1 * time.Minute,
			StatementsLimit: 250,
		},

		SchemaDetailsArguments: SchemaDetailsArguments{
			CollectInterval: 1 * time.Minute,
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
			SampleMinDuration:           0 * time.Millisecond,
			WaitEventMinDuration:        1 * time.Microsecond,
		},
		HealthCheckArguments: HealthCheckArguments{
			CollectInterval: 1 * time.Hour,
		},
	}
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
		_, err := mysql.ParseDSN(string(a.DataSourceName))
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
	Targets []discovery.Target `alloy:"targets,attr"`
}

var (
	_ component.Component       = (*Component)(nil)
	_ http_service.Component    = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
	_ cluster.Component         = (*Component)(nil)
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

	cluster cluster.Cluster
	// clusterChanged wakes the Run loop to reconcile database ownership. It
	// has capacity 1 so notifications coalesce.
	clusterChanged chan struct{}

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

func New(opts component.Options, args Arguments) (*Component, error) {
	return new(opts, args, sql.Open)
}

func new(opts component.Options, args Arguments, openFn func(driverName, dataSourceName string) (*sql.DB, error)) (*Component, error) {
	c := &Component{
		opts:           opts,
		args:           args,
		fanout:         loki.NewFanout(args.ForwardTo),
		handler:        loki.NewLogsReceiver(),
		openSQL:        openFn,
		clusterChanged: make(chan struct{}, 1),
	}

	data, err := opts.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get information about cluster: %w", err)
	}
	c.cluster = data.(cluster.Cluster)

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

			c.stopInstances(c.loadInstances())
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
			case <-c.clusterChanged:
				c.reconcileCluster()
			case <-ticker.C:
				// Reconcile ownership on the periodic tick as well, as a
				// backstop in case a cluster notification was missed.
				c.reconcileCluster()

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

// The result of SELECT version() is something like:
// for MariaDB: "10.5.17-MariaDB-1:10.5.17+maria~ubu2004-log"
// for MySQL: "8.0.36-28.1"
var versionRegex = regexp.MustCompile(`^((\d+)(\.\d+)(\.\d+))`)

func (c *Component) reportInstanceError(inst *dbInstance, errorMsg string, err error) {
	if inst.cfg.name != "" {
		errorMsg = fmt.Sprintf("database %q: %s", inst.cfg.name, errorMsg)
	}
	c.opts.Logger.Error(fmt.Sprintf("%s: %+v", errorMsg, err))
	inst.healthErr.Store(fmt.Sprintf("%s: %+v", errorMsg, err))
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	// Build the new instances before touching any state, so that a failed
	// rebuild returns an error while the previous instances keep running.
	owned := c.ownedDatabases(newArgs.databaseConfigs(), newArgs.Clustering.Enabled)
	instances, err := c.buildInstances(owned)
	if err != nil {
		return err
	}

	c.args = newArgs
	c.fanout.UpdateChildren(c.args.ForwardTo)

	c.replaceInstances(instances)
	return nil
}

// buildInstances constructs, but doesn't connect, one dbInstance per config.
func (c *Component) buildInstances(cfgs []databaseConfig) ([]*dbInstance, error) {
	instances := make([]*dbInstance, 0, len(cfgs))
	for _, cfg := range cfgs {
		inst, err := newDBInstance(c.opts, cfg)
		if err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

// replaceInstances stops the running instances, publishes the new ones,
// connects them, and re-exports targets. Must be called with c.mut locked.
func (c *Component) replaceInstances(instances []*dbInstance) {
	c.stopInstances(c.loadInstances())
	c.storeInstances(instances)

	for _, inst := range instances {
		if err := c.connectAndStartCollectors(context.Background(), inst); err != nil {
			c.reportInstanceError(inst, "failed to connect", err)
			continue
		}
		inst.healthErr.Store("")
	}

	c.exportTargets()
}

// ownedDatabases returns the subset of configs this node is responsible for
// collecting. With clustering disabled, all databases are owned locally.
// While the cluster isn't ready to admit traffic, no databases are owned, so
// that nodes don't collect duplicates while the cluster is still forming.
// Ownership lookup errors fail open to local ownership, matching the
// semantics of discovery.DistributedTargets.
func (c *Component) ownedDatabases(cfgs []databaseConfig, clusteringEnabled bool) []databaseConfig {
	if !clusteringEnabled {
		return cfgs
	}
	if !c.cluster.Ready() {
		c.opts.Logger.Info("cluster is not ready to admit traffic, not collecting from any database")
		return nil
	}

	owned := make([]databaseConfig, 0, len(cfgs))
	for _, cfg := range cfgs {
		key, err := instanceKey(string(cfg.dsn))
		if err != nil {
			// Validate guarantees the DSN parses; fail open to local ownership.
			owned = append(owned, cfg)
			continue
		}
		peers, err := c.cluster.Lookup(shard.StringKey(key), 1, shard.OpReadWrite)
		if err != nil || len(peers) == 0 || peers[0].Self {
			owned = append(owned, cfg)
		}
	}
	return owned
}

// NotifyClusterChange implements cluster.Component. It must never block: the
// cluster service notifies components sequentially, and reconciliation may be
// waiting on the component lock while it's held across database connects.
func (c *Component) NotifyClusterChange() {
	select {
	case c.clusterChanged <- struct{}{}:
	default:
	}
}

// reconcileCluster recomputes database ownership and moves only the databases
// whose ownership changed: instances this node no longer owns are stopped,
// newly owned databases are built and connected, and unmoved instances keep
// running untouched.
func (c *Component) reconcileCluster() {
	c.mut.Lock()
	defer c.mut.Unlock()

	if !c.args.Clustering.Enabled {
		return
	}

	owned := c.ownedDatabases(c.args.databaseConfigs(), true)

	running := c.loadInstances()
	runningByKey := make(map[string]*dbInstance, len(running))
	for _, inst := range running {
		runningByKey[inst.instanceKey] = inst
	}

	ownedKeys := make(map[string]struct{}, len(owned))
	var gainedCfgs []databaseConfig
	for _, cfg := range owned {
		key, err := instanceKey(string(cfg.dsn))
		if err != nil {
			// Validate guarantees the DSN parses.
			continue
		}
		ownedKeys[key] = struct{}{}
		if _, ok := runningByKey[key]; !ok {
			gainedCfgs = append(gainedCfgs, cfg)
		}
	}

	var lost []*dbInstance
	for _, inst := range running {
		if _, ok := ownedKeys[inst.instanceKey]; !ok {
			lost = append(lost, inst)
		}
	}

	if len(gainedCfgs) == 0 && len(lost) == 0 {
		return
	}

	builtByKey := make(map[string]*dbInstance, len(gainedCfgs))
	for _, cfg := range gainedCfgs {
		inst, err := newDBInstance(c.opts, cfg)
		if err != nil {
			// The running set still differs from the owned set, so the
			// periodic reconcile retries this database.
			c.opts.Logger.Error("failed to build database instance after cluster change", "database", cfg.name, "err", err)
			continue
		}
		builtByKey[inst.instanceKey] = inst
	}

	// Assemble the new instance set in config order, matching Update.
	instances := make([]*dbInstance, 0, len(owned))
	var gained []*dbInstance
	for _, cfg := range owned {
		key, err := instanceKey(string(cfg.dsn))
		if err != nil {
			continue
		}
		if inst, ok := runningByKey[key]; ok {
			instances = append(instances, inst)
		} else if inst, ok := builtByKey[key]; ok {
			instances = append(instances, inst)
			gained = append(gained, inst)
		}
	}

	c.stopInstances(lost)
	c.storeInstances(instances)

	for _, inst := range gained {
		if err := c.connectAndStartCollectors(context.Background(), inst); err != nil {
			c.reportInstanceError(inst, "failed to connect", err)
			continue
		}
		inst.healthErr.Store("")
	}

	c.exportTargets()
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

	c.exportTargets()
	return errors.Join(errs...)
}

// stopInstances stops the collectors of the given instances and closes their
// database connections. Must be called with c.mut locked.
func (c *Component) stopInstances(instances []*dbInstance) {
	for _, inst := range instances {
		for _, collector := range inst.collectors {
			collector.Stop()
		}
		inst.collectors = nil
		if inst.dbConnection != nil {
			inst.dbConnection.Close()
			inst.dbConnection = nil
		}
	}
}

// exportTargets publishes the targets of all connected database instances.
// Must be called with c.mut locked.
func (c *Component) exportTargets() {
	targets := make([]discovery.Target, 0)
	for _, inst := range c.loadInstances() {
		targets = append(targets, inst.exportedTargets...)
	}

	c.opts.OnStateChange(Exports{
		Targets: targets,
	})
}

// dbConnectTimeout bounds the connectivity check and the server-info query
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

	dbConnection, err := c.openSQL("mysql", formatDSN(string(inst.cfg.dsn), "parseTime=true"))
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

	rs := inst.dbConnection.QueryRowContext(connectCtx, selectServerInfo)
	if err = rs.Err(); err != nil {
		return fmt.Errorf("failed to query engine version: %w", err)
	}

	var serverUUID, hostname, engineVersion string
	if err := rs.Scan(&serverUUID, &hostname, &engineVersion); err != nil {
		return fmt.Errorf("failed to scan engine version: %w", err)
	}

	generatedServerID := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s", serverUUID, hostname))))

	var parsedEngineVersion semver.Version
	matches := versionRegex.FindStringSubmatch(engineVersion)
	if len(matches) > 1 {
		parsedEngineVersion, err = semver.ParseTolerant(matches[1])
		if err != nil {
			return fmt.Errorf("failed to parse engine version: %w", err)
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

	if inst.exporterCollector != nil {
		inst.registry.Unregister(inst.exporterCollector)
		inst.exporterCollector = nil
	}

	if len(inst.cfg.targets) == 0 {
		exporterArgs := exporter_mysql.DefaultArguments
		if c.args.PrometheusExporter != nil {
			exporterArgs = exporter_mysql.Arguments(*c.args.PrometheusExporter)
		}
		exporterCfg := exporterArgs.Convert()
		scrapers := mysqld_exporter.GetScrapers(exporterCfg)
		exporter := mysqld_collector.New(context.Background(), string(inst.cfg.dsn), scrapers, c.opts.Logger,
			mysqld_collector.EnableLockWaitTimeout(exporterCfg.EnableLockWaitTimeout),
			mysqld_collector.SetLockWaitTimeout(exporterCfg.LockWaitTimeout),
			mysqld_collector.SetSlowLogFilter(exporterCfg.LogSlowFilter),
		)
		if err := inst.registry.Register(exporter); err != nil {
			return fmt.Errorf("failed to register prometheus_exporter collector: %w", err)
		}
		inst.exporterCollector = exporter
	}

	allTargets := append([]discovery.Target{inst.baseTarget}, inst.cfg.targets...)
	targets := make([]discovery.Target, 0, len(allTargets))
	for _, t := range allTargets {
		builder := discovery.NewTargetBuilderFrom(t)
		if relabel.ProcessBuilder(builder, database_observability.GetRelabelingRules(generatedServerID, cp)...) {
			targets = append(targets, builder.Target())
		}
	}
	inst.exportedTargets = targets

	for _, collector := range inst.collectors {
		collector.Stop()
	}
	inst.collectors = nil

	if err := c.startCollectors(inst, generatedServerID, engineVersion, parsedEngineVersion, cp); err != nil {
		return fmt.Errorf("failed to start collectors: %w", err)
	}

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

// startCollectors attempts to start all of the enabled collectors for a database instance.
// If one or more collectors fail to start, their errors are reported
func (c *Component) startCollectors(inst *dbInstance, serverID string, engineVersion string, parsedEngineVersion semver.Version, cloudProviderInfo *database_observability.CloudProvider) error {
	var startErrors []string

	logStartError := func(collectorName, action string, err error) {
		errorString := fmt.Sprintf("failed to %s %s collector: %+v", action, collectorName, err)
		c.opts.Logger.Error(errorString)
		startErrors = append(startErrors, errorString)
	}
	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), inst.instanceKey, serverID)

	collectors := enableOrDisableCollectors(c.args)

	if collectors[collector.QueryDetailsCollector] {
		qtCollector, err := collector.NewQueryDetails(collector.QueryDetailsArguments{
			DB:              inst.dbConnection,
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
			inst.collectors = append(inst.collectors, qtCollector)
		}
	}

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
			DB:              inst.dbConnection,
			CollectInterval: c.args.SchemaDetailsArguments.CollectInterval,
			ExcludeSchemas:  c.args.ExcludeSchemas,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.SchemaDetailsCollector, "create", err)
		} else {
			if err := stCollector.Start(context.Background()); err != nil {
				logStartError(collector.SchemaDetailsCollector, "start", err)
			}
			inst.collectors = append(inst.collectors, stCollector)
		}
	}

	if collectors[collector.QuerySamplesCollector] {
		if c.args.QuerySamplesArguments.AutoEnableSetupConsumers && !c.args.AllowUpdatePerfSchemaSettings {
			c.opts.Logger.Warn("auto_enable_setup_consumers is true but allow_update_performance_schema_settings is false, setup_consumers will not be enabled")
		}
		if c.args.QuerySamplesArguments.SampleMinDuration > 0 && c.args.QuerySamplesArguments.WaitEventMinDuration > c.args.QuerySamplesArguments.SampleMinDuration {
			c.opts.Logger.Warn("wait_event_min_duration is greater than sample_min_duration, which may result in query samples with no associated wait events")
		}

		qsCollector, err := collector.NewQuerySamples(collector.QuerySamplesArguments{
			DB:                            inst.dbConnection,
			EngineVersion:                 parsedEngineVersion,
			CollectInterval:               c.args.QuerySamplesArguments.CollectInterval,
			ExcludeSchemas:                c.args.ExcludeSchemas,
			EntryHandler:                  entryHandler,
			Registry:                      inst.registry,
			Logger:                        c.opts.Logger,
			DisableQueryRedaction:         c.args.QuerySamplesArguments.DisableQueryRedaction,
			AutoEnableSetupConsumers:      c.args.AllowUpdatePerfSchemaSettings && c.args.QuerySamplesArguments.AutoEnableSetupConsumers,
			SetupConsumersCheckInterval:   c.args.QuerySamplesArguments.SetupConsumersCheckInterval,
			SampleMinDuration:             c.args.QuerySamplesArguments.SampleMinDuration,
			WaitEventMinDuration:          c.args.QuerySamplesArguments.WaitEventMinDuration,
			EnablePreClassifiedWaitEvents: c.args.QuerySamplesArguments.EnablePreClassifiedWaitEvents,
		})
		if err != nil {
			logStartError(collector.QuerySamplesCollector, "create", err)
		} else {
			if err := qsCollector.Start(context.Background()); err != nil {
				logStartError(collector.QuerySamplesCollector, "start", err)
			}
			inst.collectors = append(inst.collectors, qsCollector)
		}
	}

	if collectors[collector.SetupConsumersCollector] {
		scCollector, err := collector.NewSetupConsumers(collector.SetupConsumersArguments{
			DB:              inst.dbConnection,
			Registry:        inst.registry,
			Logger:          c.opts.Logger,
			CollectInterval: c.args.SetupConsumersArguments.CollectInterval,
		})
		if err != nil {
			logStartError(collector.SetupConsumersCollector, "create", err)
		} else {
			if err := scCollector.Start(context.Background()); err != nil {
				logStartError(collector.SetupConsumersCollector, "start", err)
			}
			inst.collectors = append(inst.collectors, scCollector)
		}
	}

	if collectors[collector.SetupActorsCollector] {
		if c.args.SetupActorsArguments.AutoUpdateSetupActors && !c.args.AllowUpdatePerfSchemaSettings {
			c.opts.Logger.Warn("auto_update_setup_actors is true but allow_update_performance_schema_settings is false, setup_actors will not be updated")
		}

		saCollector, err := collector.NewSetupActors(collector.SetupActorsArguments{
			DB:                    inst.dbConnection,
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
			inst.collectors = append(inst.collectors, saCollector)
		}
	}

	if collectors[collector.LocksCollector] {
		locksCollector, err := collector.NewLocks(collector.LocksArguments{
			DB:                inst.dbConnection,
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
			inst.collectors = append(inst.collectors, locksCollector)
		}
	}

	if collectors[collector.ExplainPlansCollector] {
		epCollector, err := collector.NewExplainPlans(collector.ExplainPlansArguments{
			DB:              inst.dbConnection,
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
			inst.collectors = append(inst.collectors, epCollector)
		}
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
	} else {
		if err := ciCollector.Start(context.Background()); err != nil {
			logStartError(collector.ConnectionInfoName, "start", err)
		}
		inst.collectors = append(inst.collectors, ciCollector)
	}

	// HealthCheck collector is always enabled
	hcCollector, err := collector.NewHealthCheck(collector.HealthCheckArguments{
		DB:              inst.dbConnection,
		CollectInterval: c.args.HealthCheckArguments.CollectInterval,
		ExcludeSchemas:  c.args.ExcludeSchemas,
		EntryHandler:    entryHandler,
		Logger:          c.opts.Logger,
	})
	if err != nil {
		logStartError(collector.HealthCheckCollector, "create", err)
	} else {
		if err := hcCollector.Start(context.Background()); err != nil {
			logStartError(collector.HealthCheckCollector, "start", err)
		}
		inst.collectors = append(inst.collectors, hcCollector)
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
	clusteringEnabled := c.args.Clustering.Enabled
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

	if clusteringEnabled && len(c.loadInstances()) == 0 {
		return component.Health{
			Health:     component.HealthTypeHealthy,
			Message:    "clustering is enabled and no databases are currently owned by this node",
			UpdateTime: time.Now(),
		}
	}

	return component.Health{
		Health:     component.HealthTypeHealthy,
		Message:    "All collectors are healthy",
		UpdateTime: time.Now(),
	}
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
