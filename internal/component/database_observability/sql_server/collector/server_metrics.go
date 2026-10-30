package collector

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const (
	ServerMetricsCollector = "server_metrics"

	defaultUserSessionLimit             = 32767
	azureSQLManagedInstanceSessionLimit = 30000
)

var (
	infoDesc = newServerMetricDesc(
		"info",
		"Information about the SQL Server engine edition and product version.",
		[]string{"engine_edition", "edition_name", "product_version", "product_level", "product_build", "compatibility_level"},
	)
	processCPUSecondsDesc = newServerMetricDesc(
		"process_cpu_seconds_total",
		"Total CPU time consumed by SQL Server threads, in seconds.",
		[]string{"mode"},
	)
	uptimeDesc = newServerMetricDesc(
		"uptime_seconds",
		"Time since the SQL Server process started, in seconds.",
		nil,
	)
	hostLogicalCPUCountDesc = newServerMetricDesc(
		"host_logical_cpu_count",
		"Number of logical CPUs visible to the SQL Server host.",
		nil,
	)
	memoryManagerCommittedDesc = newServerMetricDesc(
		"memory_manager_committed_bytes",
		"Memory committed by the SQL Server memory manager, in bytes.",
		nil,
	)
	memoryManagerTargetDesc = newServerMetricDesc(
		"memory_manager_target_bytes",
		"Memory the SQL Server memory manager can consume, in bytes.",
		nil,
	)
	hostPhysicalMemoryTotalDesc = newServerMetricDesc(
		"host_physical_memory_total_bytes",
		"Total physical memory visible to the SQL Server host, in bytes.",
		nil,
	)
	hostPhysicalMemoryAvailableDesc = newServerMetricDesc(
		"host_physical_memory_available_bytes",
		"Available physical memory visible to the SQL Server host, in bytes.",
		nil,
	)
	hostCommitLimitDesc = newServerMetricDesc(
		"host_commit_limit_bytes",
		"Operating system commit limit visible to the SQL Server host, in bytes.",
		nil,
	)
	hostCommitAvailableDesc = newServerMetricDesc(
		"host_commit_available_bytes",
		"Available operating system commit visible to the SQL Server host, in bytes.",
		nil,
	)
	hostHighMemorySignalDesc = newServerMetricDesc(
		"host_high_memory_signal",
		"Whether the host high-memory resource notification is set.",
		nil,
	)
	hostLowMemorySignalDesc = newServerMetricDesc(
		"host_low_memory_signal",
		"Whether the host low-memory resource notification is set.",
		nil,
	)
	processResidentMemoryDesc = newServerMetricDesc(
		"process_resident_memory_bytes",
		"SQL Server process working set, in bytes.",
		nil,
	)
	processVirtualMemoryDesc = newServerMetricDesc(
		"process_virtual_memory_bytes",
		"Virtual address space committed by the SQL Server process, in bytes.",
		nil,
	)
	processAvailableCommitMemoryDesc = newServerMetricDesc(
		"process_available_commit_memory_bytes",
		"Memory available to be committed by the SQL Server process, in bytes.",
		nil,
	)
	processCommittedMemoryResidentRatioDesc = newServerMetricDesc(
		"process_committed_memory_resident_ratio",
		"Ratio of SQL Server committed memory that is resident in the working set.",
		nil,
	)
	processPageFaultsDesc = newServerMetricDesc(
		"process_page_faults_total",
		"Total page faults incurred by the SQL Server process.",
		nil,
	)
	processPhysicalMemoryLowDesc = newServerMetricDesc(
		"process_physical_memory_low",
		"Whether the SQL Server process is responding to a low physical-memory notification.",
		nil,
	)
	processVirtualMemoryLowDesc = newServerMetricDesc(
		"process_virtual_memory_low",
		"Whether the SQL Server process has detected a low virtual-memory condition.",
		nil,
	)
	userSessionsDesc = newServerMetricDesc(
		"user_sessions",
		"Current authenticated SQL Server user sessions.",
		[]string{"status"},
	)
	userSessionLimitDesc = newServerMetricDesc(
		"user_session_limit",
		"Maximum simultaneous SQL Server user sessions allowed by the active configuration and service limit.",
		nil,
	)
	processCPULimitDesc = newServerMetricDesc(
		"process_cpu_limit",
		"Logical CPU capacity available to the Azure SQL Managed Instance process.",
		nil,
	)
	processMemoryLimitDesc = newServerMetricDesc(
		"process_memory_limit_bytes",
		"Maximum committed memory available to the Azure SQL Managed Instance process, in bytes.",
		nil,
	)
	sosMemoryTargetDesc = newServerMetricDesc(
		"sos_memory_target_bytes",
		"Memory available to the Azure SQL Managed Instance SQL operating system, in bytes.",
		nil,
	)
	databaseCPUUtilizationDesc = newServerMetricDesc(
		"database_cpu_utilization_ratio",
		"Average database CPU utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.",
		nil,
	)
	databaseMemoryUtilizationDesc = newServerMetricDesc(
		"database_memory_utilization_ratio",
		"Average database memory utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.",
		nil,
	)
	databaseDataIOUtilizationDesc = newServerMetricDesc(
		"database_data_io_utilization_ratio",
		"Average database data I/O utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.",
		nil,
	)
	databaseLogWriteUtilizationDesc = newServerMetricDesc(
		"database_log_write_utilization_ratio",
		"Average database log-write utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.",
		nil,
	)
	databaseWorkerUtilizationDesc = newServerMetricDesc(
		"database_worker_utilization_ratio",
		"Maximum database worker utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.",
		nil,
	)
	databaseSessionUtilizationDesc = newServerMetricDesc(
		"database_session_utilization_ratio",
		"Maximum database session utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.",
		nil,
	)
	databaseResourceStatsTimestampDesc = newServerMetricDesc(
		"database_resource_stats_timestamp_seconds",
		"Unix timestamp of the latest Azure SQL Database resource statistics interval.",
		nil,
	)
	databaseVCoreLimitDesc = newServerMetricDesc(
		"database_vcore_limit",
		"vCore limit of the Azure SQL Database.",
		nil,
	)
	databaseDTULimitDesc = newServerMetricDesc(
		"database_dtu_limit",
		"DTU limit of the Azure SQL Database.",
		nil,
	)
	databaseWorkerLimitDesc = newServerMetricDesc(
		"database_worker_limit",
		"Worker thread limit of the Azure SQL Database user workload group.",
		nil,
	)
	databaseResourceGovernorMaxDegreeOfParallelismDesc = newServerMetricDesc(
		"database_resource_governor_max_degree_of_parallelism",
		"Maximum degree of parallelism enforced by the Azure SQL Database user workload group.",
		nil,
	)
	databaseMemoryGrantLimitDesc = newServerMetricDesc(
		"database_memory_grant_limit_ratio",
		"Maximum memory grant for a single Azure SQL Database request as a ratio of user workload group memory.",
		nil,
	)
	instanceCPUUtilizationDesc = newServerMetricDesc(
		"instance_cpu_utilization_ratio",
		"Average CPU utilization relative to the Azure SQL service-tier limit during the latest reporting interval.",
		nil,
	)
	instanceMemoryUtilizationDesc = newServerMetricDesc(
		"instance_memory_utilization_ratio",
		"Average memory utilization of the database engine instance hosting the Azure SQL Database during the latest reporting interval.",
		nil,
	)
	instanceVCoreLimitDesc = newServerMetricDesc(
		"instance_vcore_limit",
		"vCore limit of the Azure SQL Managed Instance.",
		nil,
	)
	instanceStorageLimitDesc = newServerMetricDesc(
		"instance_storage_limit_bytes",
		"Reserved storage capacity of the Azure SQL Managed Instance or total operating system volume capacity, in bytes.",
		nil,
	)
	instanceStorageUsedDesc = newServerMetricDesc(
		"instance_storage_used_bytes",
		"Storage used by the Azure SQL Managed Instance or across operating system volumes hosting SQL Server files, in bytes.",
		nil,
	)
	instanceResourceStatsTimestampDesc = newServerMetricDesc(
		"instance_resource_stats_timestamp_seconds",
		"Unix timestamp of the latest Azure SQL Managed Instance resource statistics interval.",
		nil,
	)
	instanceIOOperationsPerSecondDesc = newServerMetricDesc(
		"instance_io_operations_per_second",
		"Average physical I/O operations per second during the latest Azure SQL Managed Instance reporting interval.",
		nil,
	)
	instanceIOReadBytesPerSecondDesc = newServerMetricDesc(
		"instance_io_read_bytes_per_second",
		"Average physical bytes read per second during the latest Azure SQL Managed Instance reporting interval.",
		nil,
	)
	instanceIOWrittenBytesPerSecondDesc = newServerMetricDesc(
		"instance_io_written_bytes_per_second",
		"Average physical bytes written per second during the latest Azure SQL Managed Instance reporting interval.",
		nil,
	)
	databaseStorageUsedDesc = newServerMetricDesc(
		"database_storage_used_bytes",
		"Space used within the connected database files, in bytes.",
		[]string{"file_type"},
	)
	databaseStorageAllocatedDesc = newServerMetricDesc(
		"database_storage_allocated_bytes",
		"Space allocated to the connected database files, in bytes.",
		[]string{"file_type"},
	)
	databaseStorageLimitDesc = newServerMetricDesc(
		"database_storage_limit_bytes",
		"Maximum configured storage capacity of the connected database, in bytes.",
		nil,
	)
	instanceStorageAllocatedDesc = newServerMetricDesc(
		"instance_storage_allocated_bytes",
		"Total space allocated to database files on the SQL Server instance, in bytes.",
		nil,
	)
	instanceStorageAvailableDesc = newServerMetricDesc(
		"instance_storage_available_bytes",
		"Available storage capacity on operating system volumes hosting SQL Server files, in bytes.",
		nil,
	)
	batchRequestsDesc = newServerMetricDesc(
		"batch_requests_total",
		"Total Transact-SQL command batches received by SQL Server.",
		nil,
	)
	transactionsDesc = newServerMetricDesc(
		"transactions_total",
		"Total transactions started across all databases.",
		nil,
	)
	sqlCompilationsDesc = newServerMetricDesc(
		"sql_compilations_total",
		"Total SQL compilations performed by SQL Server.",
		nil,
	)
	sqlRecompilationsDesc = newServerMetricDesc(
		"sql_recompilations_total",
		"Total SQL statement recompilations performed by SQL Server.",
		nil,
	)
	pageLifeExpectancyDesc = newServerMetricDesc(
		"page_life_expectancy_seconds",
		"Duration in seconds that a data page stays in the buffer pool without references.",
		nil,
	)
	bufferCacheHitRatioDesc = newServerMetricDesc(
		"buffer_cache_hit_ratio",
		"Ratio of pages found in the buffer pool without reading from disk.",
		nil,
	)
	checkpointPagesDesc = newServerMetricDesc(
		"checkpoint_pages_total",
		"Total pages flushed to disk by checkpoint operations.",
		nil,
	)
	lazyWritesDesc = newServerMetricDesc(
		"lazy_writes_total",
		"Total pages written to disk by the lazy writer to free buffer pool pages.",
		nil,
	)
	freeListStallsDesc = newServerMetricDesc(
		"free_list_stalls_total",
		"Total requests that had to wait for a free page in the buffer pool.",
		nil,
	)
	pageSplitsDesc = newServerMetricDesc(
		"page_splits_total",
		"Total index page splits performed by SQL Server.",
		nil,
	)
	fullScansDesc = newServerMetricDesc(
		"full_scans_total",
		"Total unrestricted base-table or full-index scans performed by SQL Server.",
		nil,
	)
	memoryGrantsPendingDesc = newServerMetricDesc(
		"memory_grants_pending",
		"Current number of processes waiting for a workspace memory grant.",
		nil,
	)
	processesBlockedDesc = newServerMetricDesc(
		"processes_blocked",
		"Current number of blocked processes.",
		nil,
	)
	deadlocksDesc = newServerMetricDesc(
		"deadlocks_total",
		"Total lock requests that resulted in a deadlock.",
		nil,
	)
	lockWaitsDesc = newServerMetricDesc(
		"lock_waits_total",
		"Total lock requests that required the caller to wait.",
		nil,
	)
	lockWaitSecondsDesc = newServerMetricDesc(
		"lock_wait_seconds_total",
		"Total time spent waiting for locks, in seconds.",
		nil,
	)
	loginsDesc = newServerMetricDesc(
		"logins_total",
		"Total logins started outside the connection pool since SQL Server last started.",
		nil,
	)
	connectionResetsDesc = newServerMetricDesc(
		"connection_resets_total",
		"Total logins started from the connection pool since SQL Server last started.",
		nil,
	)
	logoutsDesc = newServerMetricDesc(
		"logouts_total",
		"Total logout operations since SQL Server last started.",
		nil,
	)
	ioReadBytesDesc = newServerMetricDesc(
		"io_read_bytes_total",
		"Total bytes read from SQL Server database and log files.",
		nil,
	)
	ioWrittenBytesDesc = newServerMetricDesc(
		"io_written_bytes_total",
		"Total bytes written to SQL Server database and log files.",
		nil,
	)
	ioReadsDesc = newServerMetricDesc(
		"io_reads_total",
		"Total physical read operations issued on SQL Server database and log files.",
		nil,
	)
	ioWritesDesc = newServerMetricDesc(
		"io_writes_total",
		"Total physical write operations issued on SQL Server database and log files.",
		nil,
	)
	ioReadStallSecondsDesc = newServerMetricDesc(
		"io_read_stall_seconds_total",
		"Total time spent waiting for read operations on SQL Server files, in seconds.",
		nil,
	)
	ioWriteStallSecondsDesc = newServerMetricDesc(
		"io_write_stall_seconds_total",
		"Total time spent waiting for write operations on SQL Server files, in seconds.",
		nil,
	)
	tempdbSpaceUsedDesc = newServerMetricDesc(
		"tempdb_space_used_bytes",
		"Space used in tempdb data files by allocation category, in bytes.",
		[]string{"category"},
	)
	tempdbSpaceUnallocatedDesc = newServerMetricDesc(
		"tempdb_space_unallocated_bytes",
		"Unallocated space in tempdb data files, in bytes.",
		nil,
	)

	serverMetricDescs = []*prometheus.Desc{
		infoDesc,
		processCPUSecondsDesc,
		uptimeDesc,
		hostLogicalCPUCountDesc,
		memoryManagerCommittedDesc,
		memoryManagerTargetDesc,
		hostPhysicalMemoryTotalDesc,
		hostPhysicalMemoryAvailableDesc,
		hostCommitLimitDesc,
		hostCommitAvailableDesc,
		hostHighMemorySignalDesc,
		hostLowMemorySignalDesc,
		processResidentMemoryDesc,
		processVirtualMemoryDesc,
		processAvailableCommitMemoryDesc,
		processCommittedMemoryResidentRatioDesc,
		processPageFaultsDesc,
		processPhysicalMemoryLowDesc,
		processVirtualMemoryLowDesc,
		userSessionsDesc,
		userSessionLimitDesc,
		processCPULimitDesc,
		processMemoryLimitDesc,
		sosMemoryTargetDesc,
		databaseCPUUtilizationDesc,
		databaseMemoryUtilizationDesc,
		databaseDataIOUtilizationDesc,
		databaseLogWriteUtilizationDesc,
		databaseWorkerUtilizationDesc,
		databaseSessionUtilizationDesc,
		databaseResourceStatsTimestampDesc,
		databaseVCoreLimitDesc,
		databaseDTULimitDesc,
		databaseWorkerLimitDesc,
		databaseResourceGovernorMaxDegreeOfParallelismDesc,
		databaseMemoryGrantLimitDesc,
		instanceCPUUtilizationDesc,
		instanceMemoryUtilizationDesc,
		instanceVCoreLimitDesc,
		instanceStorageLimitDesc,
		instanceStorageUsedDesc,
		instanceResourceStatsTimestampDesc,
		instanceIOOperationsPerSecondDesc,
		instanceIOReadBytesPerSecondDesc,
		instanceIOWrittenBytesPerSecondDesc,
		databaseStorageUsedDesc,
		databaseStorageAllocatedDesc,
		databaseStorageLimitDesc,
		instanceStorageAllocatedDesc,
		instanceStorageAvailableDesc,
		batchRequestsDesc,
		transactionsDesc,
		sqlCompilationsDesc,
		sqlRecompilationsDesc,
		pageLifeExpectancyDesc,
		bufferCacheHitRatioDesc,
		checkpointPagesDesc,
		lazyWritesDesc,
		freeListStallsDesc,
		pageSplitsDesc,
		fullScansDesc,
		memoryGrantsPendingDesc,
		processesBlockedDesc,
		deadlocksDesc,
		lockWaitsDesc,
		lockWaitSecondsDesc,
		loginsDesc,
		connectionResetsDesc,
		logoutsDesc,
		ioReadBytesDesc,
		ioWrittenBytesDesc,
		ioReadsDesc,
		ioWritesDesc,
		ioReadStallSecondsDesc,
		ioWriteStallSecondsDesc,
		tempdbSpaceUsedDesc,
		tempdbSpaceUnallocatedDesc,
	}
)

func newServerMetricDesc(name, help string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "sql_server", name),
		help,
		labels,
		nil,
	)
}

type ServerMetricsArguments struct {
	DB                 *sql.DB
	Registry           *prometheus.Registry
	Logger             *slog.Logger
	EngineEdition      int
	ProductVersion     string
	ProductLevel       string
	ProductBuild       string
	CompatibilityLevel string
	QueryTimeout       time.Duration
	ExcludeCurrentUser bool
}

type ServerMetrics struct {
	dbConnection       *sql.DB
	registry           *prometheus.Registry
	logger             *slog.Logger
	running            *atomic.Bool
	ctxMut             sync.RWMutex
	ctx                context.Context
	cancel             context.CancelFunc
	engineEdition      int
	productVersion     string
	productLevel       string
	productBuild       string
	compatibilityLevel string
	queryTimeout       time.Duration
	excludeCurrentUser bool
}

func NewServerMetrics(args ServerMetricsArguments) (*ServerMetrics, error) {
	return &ServerMetrics{
		dbConnection:       args.DB,
		registry:           args.Registry,
		logger:             args.Logger.With("collector", ServerMetricsCollector),
		running:            atomic.NewBool(false),
		engineEdition:      args.EngineEdition,
		productVersion:     args.ProductVersion,
		productLevel:       args.ProductLevel,
		productBuild:       args.ProductBuild,
		compatibilityLevel: args.CompatibilityLevel,
		queryTimeout:       queryTimeoutOrDefault(args.QueryTimeout),
		excludeCurrentUser: args.ExcludeCurrentUser,
	}, nil
}

func (c *ServerMetrics) Name() string {
	return ServerMetricsCollector
}

func (c *ServerMetrics) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	c.ctxMut.Lock()
	c.ctx = runCtx
	c.cancel = cancel
	c.ctxMut.Unlock()

	if err := c.registry.Register(c); err != nil {
		cancel()
		return err
	}
	c.running.Store(true)
	return nil
}

func (c *ServerMetrics) Stopped() bool {
	return !c.running.Load()
}

func (c *ServerMetrics) Stop() {
	c.ctxMut.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.ctxMut.Unlock()
	if cancel != nil {
		cancel()
	}
	c.registry.Unregister(c)
	c.running.Store(false)
}

func (c *ServerMetrics) collectContext() context.Context {
	c.ctxMut.RLock()
	defer c.ctxMut.RUnlock()
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}

func (c *ServerMetrics) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range serverMetricDescs {
		ch <- desc
	}
}

func editionNameFromEngineEdition(engineEdition int) string {
	switch engineEdition {
	case 1:
		return "personal"
	case 2:
		return "standard"
	case 3:
		return "enterprise"
	case 4:
		return "express"
	case azureSQLDatabaseEngineEdition:
		return "azure_sql_database"
	case 6:
		return "azure_synapse_analytics"
	case azureSQLManagedInstanceEngineEdition:
		return "azure_sql_managed_instance"
	case 9:
		return "azure_sql_edge"
	case 11:
		return "azure_synapse_serverless"
	default:
		return "unknown"
	}
}

func (c *ServerMetrics) Collect(ch chan<- prometheus.Metric) {
	ctx := c.collectContext()
	if ctx.Err() != nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(
		infoDesc,
		prometheus.GaugeValue,
		1,
		strconv.Itoa(c.engineEdition),
		editionNameFromEngineEdition(c.engineEdition),
		c.productVersion,
		c.productLevel,
		c.productBuild,
		c.compatibilityLevel,
	)

	if c.engineEdition == azureSQLDatabaseEngineEdition {
		c.collectAzureSQLDatabaseResourceMetrics(ctx, ch)
		c.collectAzureSQLDatabaseUserSessions(ctx, ch)
		c.collectAzureSQLDatabaseLimits(ctx, ch)
		c.collectDatabaseStorage(ctx, ch)
		c.collectPerformanceCounters(ctx, ch)
		c.collectFileIOStats(ctx, ch)
		return
	}

	c.collectBaseMetrics(ctx, ch)
	c.collectUserSessions(ctx, c.engineEdition, ch)
	c.collectDatabaseStorage(ctx, ch)
	c.collectPerformanceCounters(ctx, ch)
	c.collectFileIOStats(ctx, ch)
	c.collectTempdbSpaceUsage(ctx, ch)
	if c.engineEdition == azureSQLManagedInstanceEngineEdition {
		c.collectManagedInstanceMetrics(ctx, ch)
		c.collectManagedInstanceResourceMetrics(ctx, ch)
	} else {
		c.collectVolumeStorage(ctx, ch)
	}
}

func (c *ServerMetrics) queryRow(ctx context.Context, query string, destinations ...any) error {
	return c.queryRowWithArgs(ctx, query, nil, destinations...)
}

func (c *ServerMetrics) queryRowWithArgs(ctx context.Context, query string, args []any, destinations ...any) error {
	return withQueryTimeout(ctx, c.queryTimeout, func(queryCtx context.Context) error {
		return c.dbConnection.QueryRowContext(queryCtx, query, args...).Scan(destinations...)
	})
}

func (c *ServerMetrics) userSessionsQueryArgs() []any {
	return []any{
		sql.Named("exclude_current_user", c.excludeCurrentUser),
		sql.Named("collector_app_name", AppName),
	}
}

func (c *ServerMetrics) collectBaseMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		processUserCPUSeconds               float64
		processKernelCPUSeconds             float64
		uptimeSeconds                       float64
		hostLogicalCPUCount                 float64
		memoryManagerCommitted              float64
		memoryManagerTarget                 float64
		hostPhysicalMemoryTotal             float64
		hostPhysicalMemoryAvailable         float64
		hostCommitLimit                     float64
		hostCommitAvailable                 float64
		hostHighMemorySignal                float64
		hostLowMemorySignal                 float64
		processResidentMemory               float64
		processVirtualMemory                float64
		processAvailableCommitMemory        float64
		processCommittedMemoryResidentRatio float64
		processPageFaults                   float64
		processPhysicalMemoryLow            float64
		processVirtualMemoryLow             float64
	)

	err := c.queryRow(ctx, selectServerMetrics,
		&processUserCPUSeconds,
		&processKernelCPUSeconds,
		&uptimeSeconds,
		&hostLogicalCPUCount,
		&memoryManagerCommitted,
		&memoryManagerTarget,
		&hostPhysicalMemoryTotal,
		&hostPhysicalMemoryAvailable,
		&hostCommitLimit,
		&hostCommitAvailable,
		&hostHighMemorySignal,
		&hostLowMemorySignal,
		&processResidentMemory,
		&processVirtualMemory,
		&processAvailableCommitMemory,
		&processCommittedMemoryResidentRatio,
		&processPageFaults,
		&processPhysicalMemoryLow,
		&processVirtualMemoryLow,
	)
	if err != nil {
		c.logger.Error("failed to collect server metrics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(processCPUSecondsDesc, prometheus.CounterValue, processUserCPUSeconds, "user")
	ch <- prometheus.MustNewConstMetric(processCPUSecondsDesc, prometheus.CounterValue, processKernelCPUSeconds, "kernel")
	ch <- prometheus.MustNewConstMetric(uptimeDesc, prometheus.GaugeValue, uptimeSeconds)
	ch <- prometheus.MustNewConstMetric(memoryManagerCommittedDesc, prometheus.GaugeValue, memoryManagerCommitted)
	ch <- prometheus.MustNewConstMetric(memoryManagerTargetDesc, prometheus.GaugeValue, memoryManagerTarget)
	ch <- prometheus.MustNewConstMetric(processResidentMemoryDesc, prometheus.GaugeValue, processResidentMemory)
	ch <- prometheus.MustNewConstMetric(processVirtualMemoryDesc, prometheus.GaugeValue, processVirtualMemory)
	ch <- prometheus.MustNewConstMetric(processAvailableCommitMemoryDesc, prometheus.GaugeValue, processAvailableCommitMemory)
	ch <- prometheus.MustNewConstMetric(processCommittedMemoryResidentRatioDesc, prometheus.GaugeValue, processCommittedMemoryResidentRatio)
	ch <- prometheus.MustNewConstMetric(processPageFaultsDesc, prometheus.CounterValue, processPageFaults)
	ch <- prometheus.MustNewConstMetric(processPhysicalMemoryLowDesc, prometheus.GaugeValue, processPhysicalMemoryLow)
	ch <- prometheus.MustNewConstMetric(processVirtualMemoryLowDesc, prometheus.GaugeValue, processVirtualMemoryLow)

	if c.engineEdition != azureSQLManagedInstanceEngineEdition {
		ch <- prometheus.MustNewConstMetric(hostLogicalCPUCountDesc, prometheus.GaugeValue, hostLogicalCPUCount)
		ch <- prometheus.MustNewConstMetric(hostPhysicalMemoryTotalDesc, prometheus.GaugeValue, hostPhysicalMemoryTotal)
		ch <- prometheus.MustNewConstMetric(hostPhysicalMemoryAvailableDesc, prometheus.GaugeValue, hostPhysicalMemoryAvailable)
		ch <- prometheus.MustNewConstMetric(hostCommitLimitDesc, prometheus.GaugeValue, hostCommitLimit)
		ch <- prometheus.MustNewConstMetric(hostCommitAvailableDesc, prometheus.GaugeValue, hostCommitAvailable)
		ch <- prometheus.MustNewConstMetric(hostHighMemorySignalDesc, prometheus.GaugeValue, hostHighMemorySignal)
		ch <- prometheus.MustNewConstMetric(hostLowMemorySignalDesc, prometheus.GaugeValue, hostLowMemorySignal)
	}
}

func (c *ServerMetrics) collectUserSessions(ctx context.Context, engineEdition int, ch chan<- prometheus.Metric) {
	var (
		runningSessions    float64
		sleepingSessions   float64
		dormantSessions    float64
		preconnectSessions float64
		configuredLimit    sql.NullInt64
	)

	err := c.queryRowWithArgs(ctx, selectUserSessions, c.userSessionsQueryArgs(),
		&runningSessions,
		&sleepingSessions,
		&dormantSessions,
		&preconnectSessions,
		&configuredLimit,
	)
	if err != nil {
		c.logger.Error("failed to collect user session metrics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(userSessionsDesc, prometheus.GaugeValue, runningSessions, "running")
	ch <- prometheus.MustNewConstMetric(userSessionsDesc, prometheus.GaugeValue, sleepingSessions, "sleeping")
	ch <- prometheus.MustNewConstMetric(userSessionsDesc, prometheus.GaugeValue, dormantSessions, "dormant")
	ch <- prometheus.MustNewConstMetric(userSessionsDesc, prometheus.GaugeValue, preconnectSessions, "preconnect")
	ch <- prometheus.MustNewConstMetric(userSessionLimitDesc, prometheus.GaugeValue, float64(effectiveUserSessionLimit(int(configuredLimit.Int64), engineEdition)))
}

func effectiveUserSessionLimit(configuredLimit, engineEdition int) int {
	if configuredLimit == 0 {
		configuredLimit = defaultUserSessionLimit
	}
	if engineEdition == azureSQLManagedInstanceEngineEdition && configuredLimit > azureSQLManagedInstanceSessionLimit {
		return azureSQLManagedInstanceSessionLimit
	}
	return configuredLimit
}

func (c *ServerMetrics) collectManagedInstanceMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		processCPULimit    float64
		processMemoryLimit float64
		sosMemoryTarget    float64
	)

	err := c.queryRow(ctx, selectManagedInstanceMetrics,
		&processCPULimit,
		&processMemoryLimit,
		&sosMemoryTarget,
	)
	if err != nil {
		c.logger.Error("failed to collect Azure SQL Managed Instance metrics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(processCPULimitDesc, prometheus.GaugeValue, processCPULimit)
	ch <- prometheus.MustNewConstMetric(processMemoryLimitDesc, prometheus.GaugeValue, processMemoryLimit)
	ch <- prometheus.MustNewConstMetric(sosMemoryTargetDesc, prometheus.GaugeValue, sosMemoryTarget)
}

func (c *ServerMetrics) collectAzureSQLDatabaseResourceMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		endTime                     sql.NullTime
		databaseCPUUtilization      sql.NullFloat64
		databaseMemoryUtilization   sql.NullFloat64
		databaseDataIOUtilization   sql.NullFloat64
		databaseLogWriteUtilization sql.NullFloat64
		databaseWorkerUtilization   sql.NullFloat64
		databaseSessionUtilization  sql.NullFloat64
		instanceCPUUtilization      sql.NullFloat64
		instanceMemoryUtilization   sql.NullFloat64
	)

	err := c.queryRow(ctx, selectAzureSQLDatabaseResourceMetrics,
		&endTime,
		&databaseCPUUtilization,
		&databaseMemoryUtilization,
		&databaseDataIOUtilization,
		&databaseLogWriteUtilization,
		&databaseWorkerUtilization,
		&databaseSessionUtilization,
		&instanceCPUUtilization,
		&instanceMemoryUtilization,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		c.logger.Error("failed to collect Azure SQL Database resource metrics", "err", err)
		return
	}

	emitValidMetric(ch, databaseCPUUtilizationDesc, databaseCPUUtilization)
	emitValidMetric(ch, databaseMemoryUtilizationDesc, databaseMemoryUtilization)
	emitValidMetric(ch, databaseDataIOUtilizationDesc, databaseDataIOUtilization)
	emitValidMetric(ch, databaseLogWriteUtilizationDesc, databaseLogWriteUtilization)
	emitValidMetric(ch, databaseWorkerUtilizationDesc, databaseWorkerUtilization)
	emitValidMetric(ch, databaseSessionUtilizationDesc, databaseSessionUtilization)
	emitValidMetric(ch, instanceCPUUtilizationDesc, instanceCPUUtilization)
	emitValidMetric(ch, instanceMemoryUtilizationDesc, instanceMemoryUtilization)
	if endTime.Valid {
		ch <- prometheus.MustNewConstMetric(databaseResourceStatsTimestampDesc, prometheus.GaugeValue, float64(endTime.Time.UnixNano())/float64(time.Second))
	}
}

func (c *ServerMetrics) collectAzureSQLDatabaseUserSessions(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		runningSessions    float64
		sleepingSessions   float64
		dormantSessions    float64
		preconnectSessions float64
	)

	err := c.queryRowWithArgs(ctx, selectAzureSQLDatabaseUserSessions, c.userSessionsQueryArgs(),
		&runningSessions,
		&sleepingSessions,
		&dormantSessions,
		&preconnectSessions,
	)
	if err != nil {
		c.logger.Error("failed to collect Azure SQL Database user session metrics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(userSessionsDesc, prometheus.GaugeValue, runningSessions, "running")
	ch <- prometheus.MustNewConstMetric(userSessionsDesc, prometheus.GaugeValue, sleepingSessions, "sleeping")
	ch <- prometheus.MustNewConstMetric(userSessionsDesc, prometheus.GaugeValue, dormantSessions, "dormant")
	ch <- prometheus.MustNewConstMetric(userSessionsDesc, prometheus.GaugeValue, preconnectSessions, "preconnect")
}

func (c *ServerMetrics) collectAzureSQLDatabaseLimits(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		vCoreLimit   sql.NullFloat64
		dtuLimit     sql.NullFloat64
		sessionLimit sql.NullFloat64
		workerLimit  sql.NullFloat64
		maxDOPLimit  sql.NullFloat64
		memoryGrant  sql.NullFloat64
	)

	err := c.queryRow(ctx, selectAzureSQLDatabaseLimits,
		&vCoreLimit,
		&dtuLimit,
		&sessionLimit,
		&workerLimit,
		&maxDOPLimit,
		&memoryGrant,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		c.logger.Error("failed to collect Azure SQL Database resource limits", "err", err)
		return
	}

	emitPositiveMetric(ch, databaseVCoreLimitDesc, vCoreLimit)
	emitPositiveMetric(ch, databaseDTULimitDesc, dtuLimit)
	emitPositiveMetric(ch, userSessionLimitDesc, sessionLimit)
	emitPositiveMetric(ch, databaseWorkerLimitDesc, workerLimit)
	emitValidMetric(ch, databaseResourceGovernorMaxDegreeOfParallelismDesc, maxDOPLimit)
	emitValidMetric(ch, databaseMemoryGrantLimitDesc, memoryGrant)
}

func (c *ServerMetrics) collectManagedInstanceResourceMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		startTime      sql.NullTime
		endTime        sql.NullTime
		vCoreLimit     sql.NullFloat64
		cpuUtilization sql.NullFloat64
		storageLimit   sql.NullFloat64
		storageUsed    sql.NullFloat64
		ioOperations   sql.NullFloat64
		ioBytesRead    sql.NullFloat64
		ioBytesWritten sql.NullFloat64
	)

	err := c.queryRow(ctx, selectManagedInstanceResourceMetrics,
		&startTime,
		&endTime,
		&vCoreLimit,
		&cpuUtilization,
		&storageLimit,
		&storageUsed,
		&ioOperations,
		&ioBytesRead,
		&ioBytesWritten,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		c.logger.Error("failed to collect Azure SQL Managed Instance resource metrics", "err", err)
		return
	}

	emitValidMetric(ch, instanceVCoreLimitDesc, vCoreLimit)
	emitValidMetric(ch, instanceCPUUtilizationDesc, cpuUtilization)
	emitValidMetric(ch, instanceStorageLimitDesc, storageLimit)
	emitValidMetric(ch, instanceStorageUsedDesc, storageUsed)
	if endTime.Valid {
		ch <- prometheus.MustNewConstMetric(instanceResourceStatsTimestampDesc, prometheus.GaugeValue, float64(endTime.Time.UnixNano())/float64(time.Second))
	}

	if !startTime.Valid || !endTime.Valid {
		return
	}

	intervalSeconds := endTime.Time.Sub(startTime.Time).Seconds()
	if intervalSeconds <= 0 {
		c.logger.Error(
			"invalid Azure SQL Managed Instance resource metrics interval",
			"start_time", startTime.Time,
			"end_time", endTime.Time,
		)
		return
	}

	if ioOperations.Valid {
		ch <- prometheus.MustNewConstMetric(instanceIOOperationsPerSecondDesc, prometheus.GaugeValue, ioOperations.Float64/intervalSeconds)
	}
	if ioBytesRead.Valid {
		ch <- prometheus.MustNewConstMetric(instanceIOReadBytesPerSecondDesc, prometheus.GaugeValue, ioBytesRead.Float64/intervalSeconds)
	}
	if ioBytesWritten.Valid {
		ch <- prometheus.MustNewConstMetric(instanceIOWrittenBytesPerSecondDesc, prometheus.GaugeValue, ioBytesWritten.Float64/intervalSeconds)
	}
}

func (c *ServerMetrics) collectDatabaseStorage(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		dataUsed      float64
		dataAllocated float64
		logUsed       float64
		logAllocated  float64
		storageLimit  sql.NullFloat64
	)

	err := c.queryRow(ctx, selectDatabaseStorage,
		&dataUsed,
		&dataAllocated,
		&logUsed,
		&logAllocated,
		&storageLimit,
	)
	if err != nil {
		c.logger.Error("failed to collect database storage metrics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(databaseStorageUsedDesc, prometheus.GaugeValue, dataUsed, "rows")
	ch <- prometheus.MustNewConstMetric(databaseStorageUsedDesc, prometheus.GaugeValue, logUsed, "log")
	ch <- prometheus.MustNewConstMetric(databaseStorageAllocatedDesc, prometheus.GaugeValue, dataAllocated, "rows")
	ch <- prometheus.MustNewConstMetric(databaseStorageAllocatedDesc, prometheus.GaugeValue, logAllocated, "log")
	emitPositiveMetric(ch, databaseStorageLimitDesc, storageLimit)
}

func (c *ServerMetrics) collectVolumeStorage(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		allocatedBytes sql.NullFloat64
		limitBytes     sql.NullFloat64
		usedBytes      sql.NullFloat64
		availableBytes sql.NullFloat64
	)

	err := c.queryRow(ctx, selectVolumeStorage,
		&allocatedBytes,
		&limitBytes,
		&usedBytes,
		&availableBytes,
	)
	if err != nil {
		c.logger.Error("failed to collect volume storage metrics", "err", err)
		return
	}

	emitPositiveMetric(ch, instanceStorageAllocatedDesc, allocatedBytes)
	emitPositiveMetric(ch, instanceStorageLimitDesc, limitBytes)
	emitPositiveMetric(ch, instanceStorageUsedDesc, usedBytes)
	emitPositiveMetric(ch, instanceStorageAvailableDesc, availableBytes)
}

func (c *ServerMetrics) collectPerformanceCounters(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		batchRequests       float64
		transactions        float64
		sqlCompilations     float64
		sqlRecompilations   float64
		pageLifeExpectancy  float64
		bufferCacheHitCount float64
		bufferCacheHitBase  float64
		checkpointPages     float64
		lazyWrites          float64
		freeListStalls      float64
		pageSplits          float64
		fullScans           float64
		memoryGrants        float64
		processesBlocked    float64
		deadlocks           float64
		lockWaits           float64
		lockWaitSeconds     float64
		logins              float64
		connectionResets    float64
		logouts             float64
	)

	err := c.queryRow(ctx, selectPerformanceCounters,
		&batchRequests,
		&transactions,
		&sqlCompilations,
		&sqlRecompilations,
		&pageLifeExpectancy,
		&bufferCacheHitCount,
		&bufferCacheHitBase,
		&checkpointPages,
		&lazyWrites,
		&freeListStalls,
		&pageSplits,
		&fullScans,
		&memoryGrants,
		&processesBlocked,
		&deadlocks,
		&lockWaits,
		&lockWaitSeconds,
		&logins,
		&connectionResets,
		&logouts,
	)
	if err != nil {
		c.logger.Error("failed to collect performance counter metrics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(batchRequestsDesc, prometheus.CounterValue, batchRequests)
	ch <- prometheus.MustNewConstMetric(transactionsDesc, prometheus.CounterValue, transactions)
	ch <- prometheus.MustNewConstMetric(sqlCompilationsDesc, prometheus.CounterValue, sqlCompilations)
	ch <- prometheus.MustNewConstMetric(sqlRecompilationsDesc, prometheus.CounterValue, sqlRecompilations)
	ch <- prometheus.MustNewConstMetric(pageLifeExpectancyDesc, prometheus.GaugeValue, pageLifeExpectancy)
	if bufferCacheHitBase > 0 {
		ratio := bufferCacheHitCount / bufferCacheHitBase
		ratio = max(0, min(1, ratio))
		ch <- prometheus.MustNewConstMetric(bufferCacheHitRatioDesc, prometheus.GaugeValue, ratio)
	}
	ch <- prometheus.MustNewConstMetric(checkpointPagesDesc, prometheus.CounterValue, checkpointPages)
	ch <- prometheus.MustNewConstMetric(lazyWritesDesc, prometheus.CounterValue, lazyWrites)
	ch <- prometheus.MustNewConstMetric(freeListStallsDesc, prometheus.CounterValue, freeListStalls)
	ch <- prometheus.MustNewConstMetric(pageSplitsDesc, prometheus.CounterValue, pageSplits)
	ch <- prometheus.MustNewConstMetric(fullScansDesc, prometheus.CounterValue, fullScans)
	ch <- prometheus.MustNewConstMetric(memoryGrantsPendingDesc, prometheus.GaugeValue, memoryGrants)
	ch <- prometheus.MustNewConstMetric(processesBlockedDesc, prometheus.GaugeValue, processesBlocked)
	ch <- prometheus.MustNewConstMetric(deadlocksDesc, prometheus.CounterValue, deadlocks)
	ch <- prometheus.MustNewConstMetric(lockWaitsDesc, prometheus.CounterValue, lockWaits)
	ch <- prometheus.MustNewConstMetric(lockWaitSecondsDesc, prometheus.CounterValue, lockWaitSeconds)
	ch <- prometheus.MustNewConstMetric(loginsDesc, prometheus.CounterValue, logins)
	ch <- prometheus.MustNewConstMetric(connectionResetsDesc, prometheus.CounterValue, connectionResets)
	ch <- prometheus.MustNewConstMetric(logoutsDesc, prometheus.CounterValue, logouts)
}

func (c *ServerMetrics) collectFileIOStats(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		ioReadBytes         float64
		ioWrittenBytes      float64
		ioReads             float64
		ioWrites            float64
		ioReadStallSeconds  float64
		ioWriteStallSeconds float64
	)

	err := c.queryRow(ctx, selectFileIOStats,
		&ioReadBytes,
		&ioWrittenBytes,
		&ioReads,
		&ioWrites,
		&ioReadStallSeconds,
		&ioWriteStallSeconds,
	)
	if err != nil {
		c.logger.Error("failed to collect file I/O statistics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(ioReadBytesDesc, prometheus.CounterValue, ioReadBytes)
	ch <- prometheus.MustNewConstMetric(ioWrittenBytesDesc, prometheus.CounterValue, ioWrittenBytes)
	ch <- prometheus.MustNewConstMetric(ioReadsDesc, prometheus.CounterValue, ioReads)
	ch <- prometheus.MustNewConstMetric(ioWritesDesc, prometheus.CounterValue, ioWrites)
	ch <- prometheus.MustNewConstMetric(ioReadStallSecondsDesc, prometheus.CounterValue, ioReadStallSeconds)
	ch <- prometheus.MustNewConstMetric(ioWriteStallSecondsDesc, prometheus.CounterValue, ioWriteStallSeconds)
}

func (c *ServerMetrics) collectTempdbSpaceUsage(ctx context.Context, ch chan<- prometheus.Metric) {
	var (
		userObjects     float64
		internalObjects float64
		versionStore    float64
		mixedExtents    float64
		unallocated     float64
	)

	err := c.queryRow(ctx, selectTempdbSpaceUsage,
		&userObjects,
		&internalObjects,
		&versionStore,
		&mixedExtents,
		&unallocated,
	)
	if err != nil {
		c.logger.Error("failed to collect tempdb space usage metrics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(tempdbSpaceUsedDesc, prometheus.GaugeValue, userObjects, "user_objects")
	ch <- prometheus.MustNewConstMetric(tempdbSpaceUsedDesc, prometheus.GaugeValue, internalObjects, "internal_objects")
	ch <- prometheus.MustNewConstMetric(tempdbSpaceUsedDesc, prometheus.GaugeValue, versionStore, "version_store")
	ch <- prometheus.MustNewConstMetric(tempdbSpaceUsedDesc, prometheus.GaugeValue, mixedExtents, "mixed_extents")
	ch <- prometheus.MustNewConstMetric(tempdbSpaceUnallocatedDesc, prometheus.GaugeValue, unallocated)
}

func emitPositiveMetric(ch chan<- prometheus.Metric, desc *prometheus.Desc, value sql.NullFloat64) {
	if value.Valid && value.Float64 > 0 {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value.Float64)
	}
}

func emitValidMetric(ch chan<- prometheus.Metric, desc *prometheus.Desc, value sql.NullFloat64) {
	if value.Valid {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value.Float64)
	}
}
