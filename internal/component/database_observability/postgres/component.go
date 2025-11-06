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

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/blang/semver/v4"
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

	CloudProvider          *CloudProvider         `alloy:"cloud_provider,block,optional"`
	QuerySampleArguments   QuerySampleArguments   `alloy:"query_samples,block,optional"`
	QueryTablesArguments   QueryTablesArguments   `alloy:"query_details,block,optional"`
	SchemaDetailsArguments SchemaDetailsArguments `alloy:"schema_details,block,optional"`
	ExplainPlanArguments   ExplainPlanArguments   `alloy:"explain_plans,block,optional"`
}

type CloudProvider struct {
	AWS *AWSCloudProviderInfo `alloy:"aws,block,optional"`
}

type AWSCloudProviderInfo struct {
	ARN string `alloy:"arn,attr"`
}

type QuerySampleArguments struct {
	CollectInterval       time.Duration `alloy:"collect_interval,attr,optional"`
	DisableQueryRedaction bool          `alloy:"disable_query_redaction,attr,optional"`
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
	QuerySampleArguments: QuerySampleArguments{
		CollectInterval:       15 * time.Second,
		DisableQueryRedaction: false,
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
	ExplainPlanArguments: ExplainPlanArguments{
		CollectInterval: 1 * time.Minute,
		PerCollectRatio: 1.0,
	},
}

type ExplainPlanArguments struct {
	CollectInterval           time.Duration `alloy:"collect_interval,attr,optional"`
	PerCollectRatio           float64       `alloy:"per_collect_ratio,attr,optional"`
	ExplainPlanExcludeSchemas []string      `alloy:"explain_plan_exclude_schemas,attr,optional"`
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

	dbConnection, err := c.openSQL("postgres", string(c.args.DataSourceName))
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

	rs := dbConnection.QueryRowContext(context.Background(), selectServerInfo)
	err = rs.Err()
	if err != nil {
		c.reportError("failed to query engine version", err)
		return nil
	}

	var systemID, systemIP, systemPort, engineVersion sql.NullString
	if err := rs.Scan(&systemID, &systemIP, &systemPort, &engineVersion); err != nil {
		c.reportError("failed to scan engine version", err)
		return nil
	}

	generatedSystemID := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", systemID.String, systemIP.String, systemPort.String))))

	c.args.Targets = append([]discovery.Target{c.baseTarget}, c.args.Targets...)
	targets := make([]discovery.Target, 0, len(c.args.Targets)+1)
	for _, t := range c.args.Targets {
		builder := discovery.NewTargetBuilderFrom(t)
		if relabel.ProcessBuilder(builder, database_observability.GetRelabelingRules(generatedSystemID)...) {
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

	if err := c.startCollectors(generatedSystemID, engineVersion.String); err != nil {
		c.reportError("failed to start collectors", err)
		return nil
	}

	c.healthErr.Store("")
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

// startCollectors attempts to start all of the enabled collectors. If one or more collectors fail to start, their errors are reported
func (c *Component) startCollectors(systemID string, engineVersion string) error {
	var startErrors []string

	logStartError := func(collectorName, action string, err error) {
		errorString := fmt.Sprintf("failed to %s %s collector: %+v", action, collectorName, err)
		level.Error(c.opts.Logger).Log("msg", errorString)
		startErrors = append(startErrors, errorString)
	}

	var cloudProviderInfo *database_observability.CloudProvider
	if c.args.CloudProvider != nil && c.args.CloudProvider.AWS != nil {
		arn, err := arn.Parse(c.args.CloudProvider.AWS.ARN)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to parse AWS cloud provider ARN", "err", err)
		}
		cloudProviderInfo = &database_observability.CloudProvider{
			AWS: &database_observability.AWSCloudProviderInfo{
				ARN: arn,
			},
		}
	}

	entryHandler := addLokiLabels(loki.NewEntryHandler(c.handler.Chan(), func() {}), c.instanceKey, systemID)

	collectors := enableOrDisableCollectors(c.args)

	if collectors[collector.QueryDetailsCollector] {
		qCollector, err := collector.NewQueryDetails(collector.QueryDetailsArguments{
			DB:              c.dbConnection,
			CollectInterval: c.args.QueryTablesArguments.CollectInterval,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
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
			EntryHandler:          entryHandler,
			Logger:                c.opts.Logger,
			DisableQueryRedaction: c.args.QuerySampleArguments.DisableQueryRedaction,
		})
		if err != nil {
			logStartError(collector.QuerySamplesCollector, "create", err)
		}
		if err := aCollector.Start(context.Background()); err != nil {
			logStartError(collector.QuerySamplesCollector, "start", err)
		}
		c.collectors = append(c.collectors, aCollector)
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
	}
	if err := ciCollector.Start(context.Background()); err != nil {
		logStartError(collector.ConnectionInfoName, "start", err)
	}

	c.collectors = append(c.collectors, ciCollector)

	if collectors[collector.SchemaDetailsCollector] {
		stCollector, err := collector.NewSchemaDetails(collector.SchemaDetailsArguments{
			DB:              c.dbConnection,
			DSN:             string(c.args.DataSourceName),
			CollectInterval: c.args.SchemaDetailsArguments.CollectInterval,
			CacheEnabled:    c.args.SchemaDetailsArguments.CacheEnabled,
			CacheSize:       c.args.SchemaDetailsArguments.CacheSize,
			CacheTTL:        c.args.SchemaDetailsArguments.CacheTTL,
			EntryHandler:    entryHandler,
			Logger:          c.opts.Logger,
		})
		if err != nil {
			logStartError(collector.SchemaDetailsCollector, "create", err)
		}
		if err := stCollector.Start(context.Background()); err != nil {
			logStartError(collector.SchemaDetailsCollector, "start", err)
		}
		c.collectors = append(c.collectors, stCollector)
	}

	if collectors[collector.ExplainPlanCollector] {
		engineSemver, err := semver.ParseTolerant(engineVersion)
		if err != nil {
			logStartError(collector.ExplainPlanCollector, "parse version", err)
		}
		epCollector, err := collector.NewExplainPlan(collector.ExplainPlanArguments{
			DB:             c.dbConnection,
			DSN:            string(c.args.DataSourceName),
			ScrapeInterval: c.args.ExplainPlanArguments.CollectInterval,
			PerScrapeRatio: c.args.ExplainPlanArguments.PerCollectRatio,
			Logger:         c.opts.Logger,
			DBVersion:      engineSemver,
			EntryHandler:   entryHandler,
		})
		if err != nil {
			logStartError(collector.ExplainPlanCollector, "create", err)
		}
		if err := epCollector.Start(context.Background()); err != nil {
			logStartError(collector.ExplainPlanCollector, "start", err)
		}
		c.collectors = append(c.collectors, epCollector)
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
