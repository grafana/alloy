package sql_server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/grafana/ckit/shard"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/sql_server/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/cluster"
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
	DataSourceName alloytypes.Secret   `alloy:"data_source_name,attr,optional"`
	ForwardTo      []loki.LogsReceiver `alloy:"forward_to,attr"`
	Targets        []discovery.Target  `alloy:"targets,attr,optional"`

	QueryTimeout time.Duration `alloy:"query_timeout,attr,optional"`

	EnableCollectors   []string `alloy:"enable_collectors,attr,optional"`
	DisableCollectors  []string `alloy:"disable_collectors,attr,optional"`
	ExcludeSchemas     []string `alloy:"exclude_schemas,attr,optional"`
	ExcludeDatabases   []string `alloy:"exclude_databases,attr,optional"`
	ExcludeUsers       []string `alloy:"exclude_users,attr,optional"`
	ExcludeCurrentUser bool     `alloy:"exclude_current_user,attr,optional"`

	Databases []DatabaseArguments `alloy:"database_instance,block,optional"`

	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`

	CloudProvider          *CloudProvider         `alloy:"cloud_provider,block,optional"`
	SchemaDetailsArguments SchemaDetailsArguments `alloy:"schema_details,block,optional"`
	QueryMetricsArguments  QueryMetricsArguments  `alloy:"query_metrics,block,optional"`
	QuerySamplesArguments  QuerySamplesArguments  `alloy:"query_samples,block,optional"`
	QueryDetailsArguments  QueryDetailsArguments  `alloy:"query_details,block,optional"`
	ExplainPlansArguments  ExplainPlansArguments  `alloy:"explain_plans,block,optional"`
}

// DatabaseArguments configures one monitored SQL Server instance. When one or
// more `database_instance` blocks are defined, the top-level `data_source_name`,
// `targets`, and `cloud_provider` arguments must not be set.
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
		if _, err := msdsn.Parse(string(a.DataSourceName)); err != nil {
			return err
		}
		if err := validateCloudProvider(a.CloudProvider); err != nil {
			return err
		}
	} else {
		// database_instance blocks are defined: per-instance settings must not
		// also be set at the top level.
		if a.DataSourceName != "" {
			return fmt.Errorf("data_source_name and database_instance blocks are mutually exclusive")
		}
		if len(a.Targets) > 0 {
			return fmt.Errorf("targets and database_instance blocks are mutually exclusive")
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
	return newComponent(opts, args, sql.Open)
}

func newComponent(opts component.Options, args Arguments, openFn func(driverName, dataSourceName string) (*sql.DB, error)) (*Component, error) {
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
// connects them, and re-exports the component state. Must be called with
// c.mut locked.
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

	c.exportState()
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

	c.exportState()
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

// exportState publishes the targets of all connected database instances.
// Must be called with c.mut locked.
func (c *Component) exportState() {
	targets := make([]discovery.Target, 0)
	for _, inst := range c.loadInstances() {
		targets = append(targets, inst.exportedTargets...)
	}
	c.opts.OnStateChange(Exports{Targets: targets})
}

// connectAndStartCollectors handles the full connection lifecycle of one
// database instance: closes its old connection, opens a new one, queries
// server info, and starts its collectors.
// Must be called with c.mut locked.
func (c *Component) connectAndStartCollectors(ctx context.Context, inst *dbInstance) error {
	if inst.dbConnection != nil {
		inst.dbConnection.Close()
		inst.dbConnection = nil
	}

	dbConnection, err := c.openSQL("sqlserver", string(inst.cfg.dsn))
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
	inst.dbConnection = dbConnection

	queryCtx, cancelQuery := context.WithTimeout(ctx, c.args.QueryTimeout)
	var serverName, machineName, engineVersion sql.NullString
	err = inst.dbConnection.QueryRowContext(queryCtx, selectServerInfo).Scan(&serverName, &machineName, &engineVersion)
	cancelQuery()
	if err != nil {
		return fmt.Errorf("failed to query server information: %w", err)
	}

	excludeCurrentUser := c.args.ExcludeCurrentUser && enableOrDisableCollectors(c.args)[collector.QuerySamplesCollector]
	effectiveExcludeUsers, err := resolveExcludeUsers(ctx, inst.dbConnection, c.args.QueryTimeout, c.args.ExcludeUsers, excludeCurrentUser)
	if err != nil {
		return fmt.Errorf("failed to resolve current login for query_samples user exclusion: %w", err)
	}

	generatedServerID := fmt.Sprintf("%x", sha256.Sum256(fmt.Appendf(nil, "%s:%s", serverName.String, machineName.String)))

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

	if err := c.startCollectors(ctx, inst, generatedServerID, engineVersion.String, cp, effectiveExcludeUsers); err != nil {
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

// startCollectors attempts to start all of the enabled collectors for a
// database instance. If one or more collectors fail to start, their errors
// are reported.
func (c *Component) startCollectors(ctx context.Context, inst *dbInstance, serverID string, engineVersion string, cloudProviderInfo *database_observability.CloudProvider, effectiveExcludeUsers []string) error {
	var startErrors []string

	logStartError := func(collectorName, action string, err error) {
		errorString := fmt.Sprintf("failed to %s %s collector: %+v", action, collectorName, err)
		c.opts.Logger.Error(errorString)
		startErrors = append(startErrors, errorString)
	}
	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), inst.instanceKey, serverID)

	collectors := enableOrDisableCollectors(c.args)

	if collectors[collector.SchemaDetailsCollector] {
		stCollector, err := collector.NewSchemaDetails(collector.SchemaDetailsArguments{
			DB:               inst.dbConnection,
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
			inst.collectors = append(inst.collectors, stCollector)
		}
	}

	var queryMetricsCollector *collector.QueryMetrics
	if collectors[collector.QueryMetricsCollector] {
		qmCollector, err := collector.NewQueryMetrics(collector.QueryMetricsArguments{
			DB:              inst.dbConnection,
			Registry:        inst.registry,
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
			inst.collectors = append(inst.collectors, qmCollector)
		}
	}

	if collectors[collector.QuerySamplesCollector] {
		qsArgs := collector.QuerySamplesArguments{
			DB:                    inst.dbConnection,
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
			inst.collectors = append(inst.collectors, qsCollector)
		}
	}

	if collectors[collector.QueryDetailsCollector] {
		qdArgs := collector.QueryDetailsArguments{
			DB:              inst.dbConnection,
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
			inst.collectors = append(inst.collectors, qdCollector)
		}
	}

	if collectors[collector.ExplainPlansCollector] {
		epArgs := collector.ExplainPlansArguments{
			DB:              inst.dbConnection,
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
		if err := ciCollector.Start(ctx); err != nil {
			logStartError(collector.ConnectionInfoName, "start", err)
		}
		inst.collectors = append(inst.collectors, ciCollector)
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

func addLokiLabels(entryHandler loki.EntryHandler, instanceKey string, serverID string) loki.EntryHandler {
	entryHandler = loki.AddLabelsMiddleware(model.LabelSet{
		"job":       database_observability.JobName,
		"instance":  model.LabelValue(instanceKey),
		"server_id": model.LabelValue(serverID),
		"engine":    model.LabelValue(collector.EngineName),
	}).Wrap(entryHandler)

	return entryHandler
}
