package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/grafana/ckit/shard"
	"github.com/lib/pq"
	pgconfig "github.com/prometheus-community/postgres_exporter/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/dbinstance"
	"github.com/grafana/alloy/internal/component/discovery"
	exporter_postgres "github.com/grafana/alloy/internal/component/prometheus/exporter/postgres"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/cluster"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/static/integrations/postgres_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

const name = "database_observability.postgres"

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

	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`

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
	// Rules holds relabel rules applied to the metrics and the logs of this
	// database only. They run after the labels the component sets itself, so a
	// rule can use those as `source_labels`.
	Rules []*relabel.Config `alloy:"rule,block,optional"`
}

// The per-collector blocks are defined in dbinstance, which reads them as is.
type (
	LogsArguments          = dbinstance.LogsArguments
	QuerySampleArguments   = dbinstance.QuerySampleArguments
	QueryDetailsArguments  = dbinstance.QueryDetailsArguments
	SchemaDetailsArguments = dbinstance.SchemaDetailsArguments
	ExplainPlansArguments  = dbinstance.ExplainPlansArguments
	HealthCheckArguments   = dbinstance.HealthCheckArguments
)

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

// defaultEnabledCollectors is a replacement allowlist, not additive, so this is the only collector enabled by default here.
var defaultEnabledCollectors = []string{"stat_statements"}

// resolveExporterArgs injects defaultEnabledCollectors after decoding, since SetToDefault runs before the user's block is decoded onto it.
func resolveExporterArgs(userBlock *PrometheusExporterArguments) exporter_postgres.Arguments {
	exporterArgs := exporter_postgres.DefaultArguments
	if userBlock != nil {
		exporterArgs = exporter_postgres.Arguments(*userBlock)
	}
	if !exporterArgs.DisableDefaultMetrics && len(exporterArgs.EnabledCollectors) == 0 {
		exporterArgs.EnabledCollectors = defaultEnabledCollectors
	}

	statStatementsFlags := exporter_postgres.StatStatementFlags{}
	if exporterArgs.StatStatementFlags != nil {
		statStatementsFlags = *exporterArgs.StatStatementFlags
	}
	if statStatementsFlags.Limit == 0 {
		statStatementsFlags.Limit = 100
	}
	exporterArgs.StatStatementFlags = &statStatementsFlags

	return exporterArgs
}

// exporterConfig maps resolved exporter arguments onto upstream's library config. It always
// auto-discovers databases and ignores the block's data_source_names and autodiscovery settings.
func exporterConfig(args exporter_postgres.Arguments, dsn string, excludeDatabases []string) pgconfig.Config {
	cfg := postgres_exporter.Config{
		DisableSettingsMetrics: args.DisableSettingsMetrics,
		AutodiscoverDatabases:  true,
		ExcludeDatabases:       excludeDatabases,
		DisableDefaultMetrics:  args.DisableDefaultMetrics,
		QueryPath:              args.CustomQueriesConfigPath,
		EnabledCollectors:      args.EnabledCollectors,
		StatStatementFlags:     args.StatStatementFlags.Convert(),
	}
	return cfg.UpstreamConfig([]string{dsn})
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
	// instances tracks the instance label each block rewrites to, where a rule
	// fixes it at load time, so that two blocks can't collapse onto one series.
	instances := make(map[string]string, len(a.Databases))
	for _, db := range a.Databases {
		if !databaseNameRegex.MatchString(db.Name) {
			return fmt.Errorf("database_instance block label %q must be a valid identifier (letters, digits, and underscores, not starting with a digit)", db.Name)
		}
		if _, ok := names[db.Name]; ok {
			return fmt.Errorf("duplicate database_instance block label %q", db.Name)
		}
		names[db.Name] = struct{}{}

		key, err := dbinstance.InstanceKeyFromDSN(string(db.DataSourceName))
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

		if err := database_observability.ValidateRelabelRules(db.Rules); err != nil {
			return fmt.Errorf("database_instance %q: %w", db.Name, err)
		}

		if value, known := database_observability.StaticInstanceLabel(db.Rules); known {
			if other, ok := instances[value]; ok {
				return fmt.Errorf("database_instance blocks %q and %q both set the instance label to %q, which would collapse them onto one series", other, db.Name, value)
			}
			instances[value] = db.Name
		}
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
	_ cluster.Component         = (*Component)(nil)
)

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

	// logsReceivers holds the receiver pump of each configured database,
	// keyed by database name ("" in the single-DSN form). ensurePumps
	// creates a pump on demand and reuses it across Updates; removeStalePumps
	// stops and removes the pump for a database_instance block that was
	// removed or renamed. Guarded by mut.
	logsReceivers map[string]*receiverPump

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
		mux.Handle(metricsPath(inst.cfg.name), promhttp.HandlerFor(inst.Registry(), promhttp.HandlerOpts{}))
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
		pump.wg.Go(func() { pump.run(pump.stop) })
	}
}

// removeStalePumps stops and removes the receiver pump of every database no
// longer present in cfgs, so a database_instance block that's removed or
// renamed doesn't leak its pump goroutine for the rest of the component's
// lifetime. Must be called with c.mut locked.
func (c *Component) removeStalePumps(cfgs []databaseConfig) {
	keep := make(map[string]struct{}, len(cfgs))
	for _, cfg := range cfgs {
		keep[cfg.name] = struct{}{}
	}
	for name, pump := range c.logsReceivers {
		if _, ok := keep[name]; ok {
			continue
		}
		close(pump.stop)
		delete(c.logsReceivers, name)
	}
}

// stopPumps stops every remaining receiver pump and waits for each one's
// goroutine to exit.
func (c *Component) stopPumps() {
	c.mut.Lock()
	pumps := make([]*receiverPump, 0, len(c.logsReceivers))
	for _, pump := range c.logsReceivers {
		close(pump.stop)
		pumps = append(pumps, pump)
	}
	c.mut.Unlock()

	for _, pump := range pumps {
		pump.wg.Wait()
	}
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
		logsReceivers:  make(map[string]*receiverPump),
		clusterChanged: make(chan struct{}, 1),
	}

	data, err := opts.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get information about cluster: %w", err)
	}
	c.cluster = data.(cluster.Cluster)

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
			case <-c.clusterChanged:
				c.reconcileCluster()
			case <-ticker.C:
				// Reconcile ownership on the periodic tick as well, as a
				// backstop in case a cluster notification was missed.
				c.reconcileCluster()

				c.mut.RLock()
				needsReconnect := false
				for _, inst := range c.loadInstances() {
					if !inst.Started() {
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
		if inst.Started() {
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
		inst.Stop()
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

// connectAndStartCollectors connects one database instance and starts its
// collectors, then exports its targets and wires its logs receiver pump.
// Must be called with c.mut locked
func (c *Component) connectAndStartCollectors(ctx context.Context, inst *dbInstance) error {
	pump := c.logsReceivers[inst.cfg.name]
	if pump != nil {
		pump.clearTarget()
	}

	err := inst.Connect(ctx)

	// MetricRules is set once Connect gets past server info, even if
	// collectors then fail to start.
	if builtinRules := inst.MetricRules(); builtinRules != nil {
		inst.exportedTargets = relabelTargets(inst, builtinRules)
	}

	if pump != nil {
		if receiver := inst.LogsReceiver(); receiver != nil {
			// The logs collector is now draining the instance receiver: start
			// forwarding the exported receiver's entries into it.
			pump.setTarget(receiver)
		}
	}

	return err
}

func relabelTargets(inst *dbInstance, builtinRules []*relabel.Config) []discovery.Target {
	allTargets := append([]discovery.Target{inst.baseTarget}, inst.cfg.targets...)
	targets := make([]discovery.Target, 0, len(allTargets))
	for _, t := range allTargets {
		builder := discovery.NewTargetBuilderFrom(t)
		if !relabel.ProcessBuilder(builder, builtinRules...) {
			continue
		}
		// The per-database rules run second, so that a rule can use the labels
		// the component's own rules set, such as server_id, as source_labels.
		if !database_observability.ApplyRules(builder, inst.cfg.rules) {
			continue
		}
		targets = append(targets, builder.Target())
	}
	return targets
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	// Build the new instances before touching any state, so that a failed
	// rebuild returns an error while the previous instances keep running.
	owned := c.ownedDatabases(newArgs.databaseConfigs(), newArgs.Clustering.Enabled)
	instances, err := c.buildInstances(newArgs, owned)
	if err != nil {
		return err
	}

	c.args = newArgs

	c.warnOnStatementLimitMismatch()

	c.fanout.UpdateChildren(c.args.ForwardTo)
	// Every configured database gets a pump, owned or not: non-owned
	// databases still export a receiver that discards entries, so upstream
	// components never block on a node that doesn't collect from them.
	c.ensurePumps(newArgs.databaseConfigs())
	c.removeStalePumps(newArgs.databaseConfigs())

	c.replaceInstances(instances)
	return nil
}

func (c *Component) warnOnStatementLimitMismatch() {
	queryDetailsEnabled := enableOrDisableCollectors(c.args)[collector.QueryDetailsCollector]
	if !queryDetailsEnabled {
		return
	}

	usesEmbeddedPostgresExporter := len(c.args.Targets) == 0
	if !usesEmbeddedPostgresExporter {
		return
	}

	exporterArgs := resolveExporterArgs(c.args.PrometheusExporter)
	statStatementsEnabled := !exporterArgs.DisableDefaultMetrics && slices.Contains(exporterArgs.EnabledCollectors, "stat_statements")

	if !statStatementsEnabled {
		return
	}

	if exporterArgs.StatStatementFlags != nil {
		queryDetailsLimit := c.args.QueryDetailsArguments.StatementsLimit
		statStatementsLimit := exporterArgs.StatStatementFlags.Limit
		limitsDiffer := queryDetailsLimit != int(statStatementsLimit)
		if !limitsDiffer {
			return
		}
		c.opts.Logger.Warn(
			"query_details and prometheus_exporter stat_statements limits differ",
			"query_details_limit", queryDetailsLimit,
			"stat_statements_limit", exporterArgs.StatStatementFlags.Limit,
		)
	}
}

// buildInstances constructs, but doesn't connect, one dbInstance per config.
func (c *Component) buildInstances(args Arguments, cfgs []databaseConfig) ([]*dbInstance, error) {
	instances := make([]*dbInstance, 0, len(cfgs))
	for _, cfg := range cfgs {
		inst, err := c.newDBInstance(args, cfg)
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
		key, err := dbinstance.InstanceKeyFromDSN(string(cfg.dsn))
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
		runningByKey[inst.InstanceKey()] = inst
	}

	ownedKeys := make(map[string]struct{}, len(owned))
	var gainedCfgs []databaseConfig
	for _, cfg := range owned {
		key, err := dbinstance.InstanceKeyFromDSN(string(cfg.dsn))
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
		if _, ok := ownedKeys[inst.InstanceKey()]; !ok {
			lost = append(lost, inst)
		}
	}

	if len(gainedCfgs) == 0 && len(lost) == 0 {
		return
	}

	builtByKey := make(map[string]*dbInstance, len(gainedCfgs))
	for _, cfg := range gainedCfgs {
		inst, err := c.newDBInstance(c.args, cfg)
		if err != nil {
			// The running set still differs from the owned set, so the
			// periodic reconcile retries this database.
			c.opts.Logger.Error("failed to build database instance after cluster change", "database", cfg.name, "err", err)
			continue
		}
		builtByKey[inst.InstanceKey()] = inst
	}

	// Assemble the new instance set in config order, matching Update.
	instances := make([]*dbInstance, 0, len(owned))
	var gained []*dbInstance
	for _, cfg := range owned {
		key, err := dbinstance.InstanceKeyFromDSN(string(cfg.dsn))
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

func enableOrDisableCollectors(a Arguments) map[string]bool {
	// configurable collectors and their default enabled/disabled value
	collectors := map[string]bool{
		collector.QueryDetailsCollector:  true,
		collector.QuerySamplesCollector:  true,
		collector.SchemaDetailsCollector: true,
		collector.ExplainPlanCollector:   true,
		collector.TableStatsCollector:    false,
		collector.IndexStatsCollector:    false,
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
		if ok, stopped := inst.Health(); !ok {
			unhealthyCollectors = append(unhealthyCollectors, stopped)
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
