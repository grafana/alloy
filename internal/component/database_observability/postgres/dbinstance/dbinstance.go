package dbinstance

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	pgcollector "github.com/prometheus-community/postgres_exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
)

const selectServerInfo = `
SELECT
	(pg_control_system()).system_identifier,
	inet_server_addr(),
	inet_server_port(),
	setting as version
FROM pg_settings
WHERE name = 'server_version';`

// dbConnectTimeout bounds the connectivity check and the server-info queries
// when (re)connecting to a database, so that one unresponsive server can't
// stall a caller for long.
const dbConnectTimeout = 10 * time.Second

type dbCollector interface {
	Name() string
	Start(context.Context) error
	Stopped() bool
	Stop()
}

// Instance is the identity, connection, collectors, and metrics registry of
// one monitored Postgres server. It isn't safe for concurrent use: callers
// serialize Connect, Stop, and the accessors.
type Instance struct {
	cfg         Config
	deps        Deps
	instanceKey string
	registry    *prometheus.Registry

	// logsReceiver is drained by the logs collector while it runs.
	logsReceiver loki.LogsReceiver
	logsRunning  bool

	db                 *sql.DB
	collectors         []dbCollector
	exporterCollectors []prometheus.Collector
	// exporterRuntime is the embedded postgres_exporter, nil when cfg.Exporter is.
	exporterRuntime *pgcollector.Runtime

	serverID    string
	metricRules []*relabel.Config
}

// New builds an Instance without connecting to the database.
func New(cfg Config, deps Deps) (*Instance, error) {
	key, err := InstanceKeyFromDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}
	if deps.OpenDB == nil {
		deps.OpenDB = sql.Open
	}
	return &Instance{
		cfg:          cfg,
		deps:         deps,
		instanceKey:  key,
		registry:     prometheus.NewRegistry(),
		logsReceiver: loki.NewLogsReceiver(),
	}, nil
}

// ServerID is the server_id computed by the last Connect that got that far.
func (i *Instance) ServerID() string { return i.serverID }

// InstanceKey is the instance label: network(hostname:port)/dbname.
func (i *Instance) InstanceKey() string { return i.instanceKey }

// Registry holds the instance's own metrics and the embedded exporter's.
func (i *Instance) Registry() *prometheus.Registry { return i.registry }

// MetricRules returns the server_id and cloud provider relabel rules for
// metric targets, or nil before Connect computes them. Callers apply
// Config.Rules after these, with database_observability.ApplyRules.
func (i *Instance) MetricRules() []*relabel.Config { return i.metricRules }

// Started reports whether the last Connect got as far as starting collectors.
func (i *Instance) Started() bool { return len(i.collectors) > 0 }

// LogsReceiver returns the receiver for inbound Postgres logs, or nil while
// the logs collector isn't running.
func (i *Instance) LogsReceiver() loki.LogsReceiver {
	if !i.logsRunning {
		return nil
	}
	return i.logsReceiver
}

// Health reports whether every started collector is still running. When
// not, msg lists the stopped ones, prefixed with "<name>/" for named
// instances.
func (i *Instance) Health() (ok bool, msg string) {
	var stopped []string
	for _, c := range i.collectors {
		if c.Stopped() {
			name := c.Name()
			if i.cfg.Name != "" {
				name = i.cfg.Name + "/" + name
			}
			stopped = append(stopped, name)
		}
	}
	if len(stopped) > 0 {
		return false, strings.Join(stopped, ", ")
	}
	return true, ""
}

// Stop stops the collectors, releases the embedded exporter, and closes the
// database connection.
func (i *Instance) Stop() {
	i.stopCollectors()
	i.cleanupExporterCollectors()
	if i.db != nil {
		i.db.Close()
		i.db = nil
	}
}

func (i *Instance) stopCollectors() {
	for _, c := range i.collectors {
		c.Stop()
	}
	i.collectors = nil
	i.logsRunning = false
}

// cleanupExporterCollectors unregisters the embedded exporter's collectors and
// closes its runtime: the postgres_exporter keeps one connection open per
// discovered database until then.
func (i *Instance) cleanupExporterCollectors() {
	for _, col := range i.exporterCollectors {
		i.registry.Unregister(col)
	}
	i.exporterCollectors = nil
	if i.exporterRuntime != nil {
		_ = i.exporterRuntime.Close()
		i.exporterRuntime = nil
	}
}

// Connect handles the full connection lifecycle: closes the old connection,
// opens a new one, queries server info, computes server_id, and starts the
// collectors.
func (i *Instance) Connect(ctx context.Context) error {
	if i.db != nil {
		i.db.Close()
		i.db = nil
	}
	i.cleanupExporterCollectors()

	dbConnection, err := i.deps.OpenDB("postgres", i.cfg.DSN)
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
	i.db = dbConnection

	rs := dbConnection.QueryRowContext(connectCtx, selectServerInfo)
	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to query engine version: %w", err)
	}

	var systemID, systemIP, systemPort, engineVersion sql.NullString
	if err := rs.Scan(&systemID, &systemIP, &systemPort, &engineVersion); err != nil {
		return fmt.Errorf("failed to scan engine version: %w", err)
	}

	generatedSystemID := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", systemID.String, systemIP.String, systemPort.String))))
	i.serverID = generatedSystemID

	// Get the current user and compute the effective exclude users list.
	var currentUser sql.NullString
	if i.cfg.ExcludeCurrentUser {
		if err := dbConnection.QueryRowContext(connectCtx, "SELECT current_user").Scan(&currentUser); err != nil {
			return fmt.Errorf("failed to query current_user: %w", err)
		}
	}
	effectiveExcludeUsers := slices.Clone(i.cfg.ExcludeUsers)
	if currentUser.Valid {
		if !slices.Contains(effectiveExcludeUsers, currentUser.String) {
			effectiveExcludeUsers = append(effectiveExcludeUsers, currentUser.String)
		}
	}

	var cp *database_observability.CloudProvider
	if i.cfg.CloudProvider != nil {
		cloudProvider, err := populateCloudProviderFromConfig(i.cfg.CloudProvider)
		if err != nil {
			return fmt.Errorf("failed to collect cloud provider information from config: %w", err)
		}
		cp = cloudProvider
	} else {
		cloudProvider, err := populateCloudProviderFromDSN(i.cfg.DSN)
		if err != nil {
			return fmt.Errorf("failed to collect cloud provider information from DSN: %w", err)
		}
		cp = cloudProvider
	}

	if i.cfg.Exporter != nil {
		validated, err := i.cfg.Exporter.Validate()
		if err != nil {
			return fmt.Errorf("invalid prometheus_exporter config: %w", err)
		}
		runtime, err := pgcollector.NewRuntime(validated, i.deps.Logger)
		if err != nil {
			return fmt.Errorf("failed to create prometheus_exporter: %w", err)
		}
		i.exporterRuntime = runtime
		for _, col := range runtime.Collectors() {
			if err := i.registry.Register(col); err != nil {
				return fmt.Errorf("failed to register prometheus_exporter: %w", err)
			}
			i.exporterCollectors = append(i.exporterCollectors, col)
		}
	}

	i.metricRules = database_observability.GetRelabelingRules(generatedSystemID, cp)

	i.stopCollectors()

	if err := i.startCollectors(generatedSystemID, engineVersion.String, cp, effectiveExcludeUsers); err != nil {
		return fmt.Errorf("failed to start collectors: %w", err)
	}

	return nil
}

// startCollectors attempts to start all of the enabled collectors.
// If one or more collectors fail to start, their errors are reported.
func (i *Instance) startCollectors(systemID string, engineVersion string, cloudProviderInfo *database_observability.CloudProvider, effectiveExcludeUsers []string) error {
	var startErrors []string
	logger := i.deps.Logger
	cfg := i.cfg

	logStartError := func(collectorName, action string, err error) {
		errorString := fmt.Sprintf("failed to %s %s collector: %+v", action, collectorName, err)
		logger.Error(errorString)
		startErrors = append(startErrors, errorString)
	}

	entryHandler := addLokiLabels(i.deps.Entries, i.instanceKey, systemID, cfg.Rules)

	var tableRegistry *collector.TableRegistry
	collectors := cfg.Collectors

	if collectors[collector.SchemaDetailsCollector] {
		if cfg.SchemaDetails.CacheEnabled != nil {
			logger.Warn("schema_details.cache_enabled is set, but the cache is deprecated and will be removed in a future version")
		}
		if cfg.SchemaDetails.CacheSize != nil {
			logger.Warn("schema_details.cache_size is set, but the cache is deprecated and will be removed in a future version")
		}
		if cfg.SchemaDetails.CacheTTL != nil {
			logger.Warn("schema_details.cache_ttl is set, but the cache is deprecated and will be removed in a future version")
		}

		stCollector, err := collector.NewSchemaDetails(collector.SchemaDetailsArguments{
			DB:               i.db,
			DSN:              cfg.DSN,
			CollectInterval:  cfg.SchemaDetails.CollectInterval,
			ExcludeDatabases: cfg.ExcludeDatabases,
			EntryHandler:     entryHandler,
			Logger:           logger,
		})
		if err != nil {
			logStartError(collector.SchemaDetailsCollector, "create", err)
		}
		tableRegistry = stCollector.GetTableRegistry()
		if err := stCollector.Start(context.Background()); err != nil {
			logStartError(collector.SchemaDetailsCollector, "start", err)
		}
		i.collectors = append(i.collectors, stCollector)
	}

	if collectors[collector.QueryDetailsCollector] {
		qCollector, err := collector.NewQueryDetails(collector.QueryDetailsArguments{
			DB:                        i.db,
			CollectInterval:           cfg.QueryDetails.CollectInterval,
			StatementsLimit:           cfg.QueryDetails.StatementsLimit,
			ExcludeDatabases:          cfg.ExcludeDatabases,
			ExcludeUsers:              effectiveExcludeUsers,
			EntryHandler:              entryHandler,
			TableRegistry:             tableRegistry,
			EnableErrorLogsProcessing: cfg.Logs.EnableErrorLogsProcessing,
			Logger:                    logger,
		})
		if err != nil {
			logStartError(collector.QueryDetailsCollector, "create", err)
		}
		if err := qCollector.Start(context.Background()); err != nil {
			logStartError(collector.QueryDetailsCollector, "start", err)
		}
		i.collectors = append(i.collectors, qCollector)
	}

	if collectors[collector.QuerySamplesCollector] {
		// For backward compatibility, give precedence to query_samples.exclude_current_user
		// setting over the top-level exclude_current_user: when set, preserve today's behavior;
		// when unset, inherit the top-level cascade.
		qsExcludeUsers := effectiveExcludeUsers
		qsExcludeCurrentUser := false
		if localExcludeCurrentUser := cfg.QuerySamples.ExcludeCurrentUser; localExcludeCurrentUser != nil {
			logger.Warn("query_samples.exclude_current_user is deprecated; use the top-level exclude_current_user setting instead")
			qsExcludeUsers = cfg.ExcludeUsers
			qsExcludeCurrentUser = *localExcludeCurrentUser
		}

		aCollector, err := collector.NewQuerySamples(collector.QuerySamplesArguments{
			DB:                            i.db,
			CollectInterval:               cfg.QuerySamples.CollectInterval,
			ExcludeDatabases:              cfg.ExcludeDatabases,
			ExcludeUsers:                  qsExcludeUsers,
			EntryHandler:                  entryHandler,
			Logger:                        logger,
			DisableQueryRedaction:         cfg.QuerySamples.DisableQueryRedaction,
			ExcludeCurrentUser:            qsExcludeCurrentUser,
			EnablePreClassifiedWaitEvents: cfg.QuerySamples.EnablePreClassifiedWaitEvents,
		})
		if err != nil {
			logStartError(collector.QuerySamplesCollector, "create", err)
		}
		if err := aCollector.Start(context.Background()); err != nil {
			logStartError(collector.QuerySamplesCollector, "start", err)
		}
		i.collectors = append(i.collectors, aCollector)
	}

	// Connection Info collector is always enabled
	ciCollector, err := collector.NewConnectionInfo(collector.ConnectionInfoArguments{
		DSN:           cfg.DSN,
		Registry:      i.registry,
		EngineVersion: engineVersion,
		CloudProvider: cloudProviderInfo,
		DB:            i.db,
	})
	if err != nil {
		logStartError(collector.ConnectionInfoName, "create", err)
	}
	if err := ciCollector.Start(context.Background()); err != nil {
		logStartError(collector.ConnectionInfoName, "start", err)
	}

	i.collectors = append(i.collectors, ciCollector)

	if collectors[collector.ExplainPlanCollector] {
		epCollector, err := collector.NewExplainPlan(collector.ExplainPlansArguments{
			DB:               i.db,
			DSN:              cfg.DSN,
			ScrapeInterval:   cfg.ExplainPlans.CollectInterval,
			PerScrapeRatio:   cfg.ExplainPlans.PerCollectRatio,
			ExcludeDatabases: cfg.ExcludeDatabases,
			ExcludeUsers:     effectiveExcludeUsers,
			Logger:           logger,
			DBVersion:        engineVersion,
			EntryHandler:     entryHandler,
			TableRegistry:    tableRegistry,
		})
		if err != nil {
			logStartError(collector.ExplainPlanCollector, "create", err)
		}
		if err := epCollector.Start(context.Background()); err != nil {
			logStartError(collector.ExplainPlanCollector, "start", err)
		}
		i.collectors = append(i.collectors, epCollector)
	}

	if collectors[collector.TableStatsCollector] {
		tsCollector, err := collector.NewTableStats(collector.TableStatsArguments{
			DB:               i.db,
			DSN:              cfg.DSN,
			ExcludeDatabases: cfg.ExcludeDatabases,
			Registry:         i.registry,
			Logger:           logger,
		})
		if err != nil {
			logStartError(collector.TableStatsCollector, "create", err)
		} else {
			if err := tsCollector.Start(context.Background()); err != nil {
				logStartError(collector.TableStatsCollector, "start", err)
			}
			i.collectors = append(i.collectors, tsCollector)
		}
	}

	if collectors[collector.IndexStatsCollector] {
		isCollector, err := collector.NewIndexStats(collector.IndexStatsArguments{
			DB:               i.db,
			DSN:              cfg.DSN,
			ExcludeDatabases: cfg.ExcludeDatabases,
			Registry:         i.registry,
			Logger:           logger,
		})
		if err != nil {
			logStartError(collector.IndexStatsCollector, "create", err)
		} else {
			if err := isCollector.Start(context.Background()); err != nil {
				logStartError(collector.IndexStatsCollector, "start", err)
			}
			i.collectors = append(i.collectors, isCollector)
		}
	}

	// HealthCheck collector is always enabled
	hcCollector, err := collector.NewHealthCheck(collector.HealthCheckArguments{
		DB:               i.db,
		CollectInterval:  cfg.HealthCheck.CollectInterval,
		ExcludeDatabases: cfg.ExcludeDatabases,
		ExcludeUsers:     effectiveExcludeUsers,
		EntryHandler:     entryHandler,
		Logger:           logger,
	})
	if err != nil {
		logStartError(collector.HealthCheckCollector, "create", err)
	} else {
		if err := hcCollector.Start(context.Background()); err != nil {
			logStartError(collector.HealthCheckCollector, "start", err)
		}
		i.collectors = append(i.collectors, hcCollector)
	}

	// Logs collector is always enabled
	logsCollector, err := collector.NewLogs(collector.LogsArguments{
		Receiver:                  i.logsReceiver,
		EntryHandler:              entryHandler,
		Logger:                    logger,
		Registry:                  i.registry,
		ExcludeDatabases:          cfg.ExcludeDatabases,
		ExcludeUsers:              effectiveExcludeUsers,
		EnableErrorLogsProcessing: cfg.Logs.EnableErrorLogsProcessing,
		DB:                        i.db,
	})
	if err != nil {
		logStartError(collector.LogsCollector, "create", err)
	} else {
		if err := logsCollector.Start(context.Background()); err != nil {
			logStartError(collector.LogsCollector, "start", err)
		} else {
			// The logs collector is now draining logsReceiver.
			i.logsRunning = true
		}
		i.collectors = append(i.collectors, logsCollector)
	}

	if len(startErrors) > 0 {
		return fmt.Errorf("failed to start some collectors: %s", strings.Join(startErrors, ", "))
	}

	return nil
}

func addLokiLabels(entryHandler loki.EntryHandler, instanceKey string, systemID string, rules []*relabel.Config) loki.EntryHandler {
	// One middleware adds the labels and applies the per-database rules, in
	// that order, so the rules can read these labels as source_labels.
	entryHandler = database_observability.AddLabelsAndRelabelMiddleware(model.LabelSet{
		"job":       database_observability.JobName,
		"instance":  model.LabelValue(instanceKey),
		"server_id": model.LabelValue(systemID),
		"engine":    model.LabelValue(collector.EngineName),
	}, rules).Wrap(entryHandler)

	return entryHandler
}

// InstanceKeyFromDSN returns network(hostname:port)/dbname of the Postgres
// server. This is the same key as used by the postgres static integration.
func InstanceKeyFromDSN(dsn string) (string, error) {
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
