package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const (
	ServerConfigurationCollector = "server_configuration"

	defaultServerConfigurationCollectInterval = 10 * time.Minute

	azureSQLDatabaseEngineEdition        = 5
	azureSQLManagedInstanceEngineEdition = 8
)

type serverConfigurationMetric struct {
	desc       *prometheus.Desc
	multiplier float64
}

var (
	userConnectionsConfigurationDesc = newServerConfigurationMetricDesc(
		"user_connections",
		"Configured and running values of the SQL Server user connections setting.",
		true,
	)
	remoteAdminConnectionsConfigurationDesc = newServerConfigurationMetricDesc(
		"remote_admin_connections",
		"Configured and running values of the SQL Server remote admin connections setting.",
		true,
	)
	networkPacketSizeConfigurationDesc = newServerConfigurationMetricDesc(
		"network_packet_size_bytes",
		"Configured and running SQL Server network packet size, in bytes.",
		true,
	)
	remoteLoginTimeoutConfigurationDesc = newServerConfigurationMetricDesc(
		"remote_login_timeout_seconds",
		"Configured and running SQL Server timeout for remote login attempts, in seconds.",
		true,
	)
	maxServerMemoryConfigurationDesc = newServerConfigurationMetricDesc(
		"max_server_memory_bytes",
		"Configured and running maximum memory available to the SQL Server memory manager, in bytes.",
		true,
	)
	minServerMemoryConfigurationDesc = newServerConfigurationMetricDesc(
		"min_server_memory_bytes",
		"Configured and running minimum memory retained by the SQL Server memory manager, in bytes.",
		true,
	)
	maxWorkerThreadsConfigurationDesc = newServerConfigurationMetricDesc(
		"max_worker_threads",
		"Configured and running values of the SQL Server maximum worker threads setting.",
		true,
	)
	maxDegreeOfParallelismConfigurationDesc = newServerConfigurationMetricDesc(
		"max_degree_of_parallelism",
		"Configured and running values of the SQL Server maximum degree of parallelism setting.",
		true,
	)
	costThresholdForParallelismConfigurationDesc = newServerConfigurationMetricDesc(
		"cost_threshold_for_parallelism",
		"Configured and running values of the SQL Server cost threshold for parallelism setting.",
		true,
	)
	optimizeForAdHocWorkloadsConfigurationDesc = newServerConfigurationMetricDesc(
		"optimize_for_ad_hoc_workloads",
		"Configured and running values of the SQL Server optimize for ad hoc workloads setting.",
		true,
	)
	backupCompressionDefaultConfigurationDesc = newServerConfigurationMetricDesc(
		"backup_compression_default",
		"Configured and running values of the SQL Server backup compression default setting.",
		true,
	)
	minMemoryPerQueryConfigurationDesc = newServerConfigurationMetricDesc(
		"min_memory_per_query_bytes",
		"Configured and running minimum memory allocated for query execution, in bytes.",
		true,
	)
	indexCreateMemoryConfigurationDesc = newServerConfigurationMetricDesc(
		"index_create_memory_bytes",
		"Configured and running minimum memory allocated for index creation, in bytes.",
		true,
	)
	databaseMaxDegreeOfParallelismDesc = newServerConfigurationMetricDesc(
		"database_max_degree_of_parallelism",
		"Maximum degree of parallelism configured for the current Azure SQL Database.",
		false,
	)

	serverConfigurationMetrics = map[string]serverConfigurationMetric{
		"user connections": {
			desc:       userConnectionsConfigurationDesc,
			multiplier: 1,
		},
		"remote admin connections": {
			desc:       remoteAdminConnectionsConfigurationDesc,
			multiplier: 1,
		},
		"network packet size (B)": {
			desc:       networkPacketSizeConfigurationDesc,
			multiplier: 1,
		},
		"remote login timeout (s)": {
			desc:       remoteLoginTimeoutConfigurationDesc,
			multiplier: 1,
		},
		"max server memory (MB)": {
			desc:       maxServerMemoryConfigurationDesc,
			multiplier: 1024 * 1024,
		},
		"min server memory (MB)": {
			desc:       minServerMemoryConfigurationDesc,
			multiplier: 1024 * 1024,
		},
		"max worker threads": {
			desc:       maxWorkerThreadsConfigurationDesc,
			multiplier: 1,
		},
		"max degree of parallelism": {
			desc:       maxDegreeOfParallelismConfigurationDesc,
			multiplier: 1,
		},
		"cost threshold for parallelism": {
			desc:       costThresholdForParallelismConfigurationDesc,
			multiplier: 1,
		},
		"optimize for ad hoc workloads": {
			desc:       optimizeForAdHocWorkloadsConfigurationDesc,
			multiplier: 1,
		},
		"backup compression default": {
			desc:       backupCompressionDefaultConfigurationDesc,
			multiplier: 1,
		},
		"min memory per query (KB)": {
			desc:       minMemoryPerQueryConfigurationDesc,
			multiplier: 1024,
		},
		"index create memory (KB)": {
			desc:       indexCreateMemoryConfigurationDesc,
			multiplier: 1024,
		},
	}

	serverConfigurationDescs = []*prometheus.Desc{
		userConnectionsConfigurationDesc,
		remoteAdminConnectionsConfigurationDesc,
		networkPacketSizeConfigurationDesc,
		remoteLoginTimeoutConfigurationDesc,
		maxServerMemoryConfigurationDesc,
		minServerMemoryConfigurationDesc,
		maxWorkerThreadsConfigurationDesc,
		maxDegreeOfParallelismConfigurationDesc,
		costThresholdForParallelismConfigurationDesc,
		optimizeForAdHocWorkloadsConfigurationDesc,
		backupCompressionDefaultConfigurationDesc,
		minMemoryPerQueryConfigurationDesc,
		indexCreateMemoryConfigurationDesc,
		databaseMaxDegreeOfParallelismDesc,
	}
)

func newServerConfigurationMetricDesc(name, help string, hasState bool) *prometheus.Desc {
	var labels []string
	if hasState {
		labels = []string{"state"}
	}
	return prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "sql_server_config", name),
		help,
		labels,
		nil,
	)
}

type ServerConfigurationArguments struct {
	DB              *sql.DB
	Registry        *prometheus.Registry
	CollectInterval time.Duration
	QueryTimeout    time.Duration
	Logger          *slog.Logger
	EngineEdition   int
}

type ServerConfiguration struct {
	dbConnection    *sql.DB
	registry        *prometheus.Registry
	collectInterval time.Duration
	queryTimeout    time.Duration
	logger          *slog.Logger
	engineEdition   int

	metricsMut sync.RWMutex
	metrics    []prometheus.Metric
	registered bool

	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewServerConfiguration(args ServerConfigurationArguments) (*ServerConfiguration, error) {
	collectInterval := args.CollectInterval
	if collectInterval <= 0 {
		collectInterval = defaultServerConfigurationCollectInterval
	}

	return &ServerConfiguration{
		dbConnection:    args.DB,
		registry:        args.Registry,
		collectInterval: collectInterval,
		queryTimeout:    queryTimeoutOrDefault(args.QueryTimeout),
		logger:          args.Logger.With("collector", ServerConfigurationCollector),
		engineEdition:   args.EngineEdition,
		running:         atomic.NewBool(false),
	}, nil
}

func (c *ServerConfiguration) Name() string {
	return ServerConfigurationCollector
}

func (c *ServerConfiguration) Start(ctx context.Context) error {
	if err := c.registry.Register(c); err != nil {
		return fmt.Errorf("failed to register server configuration metrics: %w", err)
	}
	c.registered = true

	c.running.Store(true)
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			if err := c.collectWithTimeout(c.ctx); err != nil {
				c.logger.Error("collector error", "err", err)
			}

			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
	return nil
}

func (c *ServerConfiguration) Stopped() bool {
	return !c.running.Load()
}

func (c *ServerConfiguration) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	if c.registered {
		c.registry.Unregister(c)
		c.registered = false
	}
}

func (c *ServerConfiguration) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range serverConfigurationDescs {
		ch <- desc
	}
}

func (c *ServerConfiguration) Collect(ch chan<- prometheus.Metric) {
	c.metricsMut.RLock()
	defer c.metricsMut.RUnlock()
	for _, metric := range c.metrics {
		ch <- metric
	}
}

func (c *ServerConfiguration) collectWithTimeout(ctx context.Context) error {
	var metrics []prometheus.Metric
	if c.engineEdition != azureSQLDatabaseEngineEdition {
		if err := withQueryTimeout(ctx, c.queryTimeout, func(queryCtx context.Context) error {
			var err error
			metrics, err = c.collectServer(queryCtx, selectServerConfiguration)
			return err
		}); err != nil {
			return err
		}
		c.replaceMetrics(metrics)
		return nil
	}

	var serverMetrics []prometheus.Metric
	if err := withQueryTimeout(ctx, c.queryTimeout, func(queryCtx context.Context) error {
		var err error
		serverMetrics, err = c.collectServer(queryCtx, selectAzureSQLDatabaseServerConfiguration)
		return err
	}); err != nil {
		return err
	}

	var databaseMetrics []prometheus.Metric
	if err := withQueryTimeout(ctx, c.queryTimeout, func(queryCtx context.Context) error {
		var err error
		databaseMetrics, err = c.collectAzureSQLDatabase(queryCtx)
		return err
	}); err != nil {
		return err
	}

	c.replaceMetrics(append(serverMetrics, databaseMetrics...))
	return nil
}

func (c *ServerConfiguration) replaceMetrics(metrics []prometheus.Metric) {
	c.metricsMut.Lock()
	c.metrics = metrics
	c.metricsMut.Unlock()
}

func (c *ServerConfiguration) collectServer(ctx context.Context, query string) ([]prometheus.Metric, error) {
	rows, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query server configuration: %w", err)
	}
	defer rows.Close()

	var metrics []prometheus.Metric
	for rows.Next() {
		var (
			name            string
			configuredValue float64
			inUseValue      float64
		)
		if err := rows.Scan(&name, &configuredValue, &inUseValue); err != nil {
			return nil, fmt.Errorf("failed to scan server configuration: %w", err)
		}

		definition, ok := serverConfigurationMetrics[name]
		if !ok {
			continue
		}
		metrics = append(metrics,
			prometheus.MustNewConstMetric(definition.desc, prometheus.GaugeValue, configuredValue*definition.multiplier, "configured"),
			prometheus.MustNewConstMetric(definition.desc, prometheus.GaugeValue, inUseValue*definition.multiplier, "in_use"),
		)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read server configuration: %w", err)
	}
	return metrics, nil
}

func (c *ServerConfiguration) collectAzureSQLDatabase(ctx context.Context) ([]prometheus.Metric, error) {
	var value float64
	err := c.dbConnection.QueryRowContext(ctx, selectDatabaseMaxDegreeOfParallelism).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query Azure SQL Database configuration: %w", err)
	}
	return []prometheus.Metric{
		prometheus.MustNewConstMetric(databaseMaxDegreeOfParallelismDesc, prometheus.GaugeValue, value),
	}, nil
}
