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

// TableStatsCollector emits table-level scan counters from pg_stat_user_tables,
// scoped to every database the connection can reach rather than only the one
// named in the DSN.
//
// The queries run on a fixed collect interval, in the background, and each run
// is bounded by a timeout equal to the interval, on the client and on the
// server. A scrape only serves the results of the last successful run and never
// queries a database.
const TableStatsCollector = "table_stats"

const selectTableScanStats = `
	SELECT
		schemaname,
		relname,
		seq_scan,
		idx_scan,
		n_live_tup
	FROM pg_stat_user_tables`

const labelDatname = "datname"

var tableLabels = []string{labelDatname, "schemaname", "relname"}

var (
	tableScanStatsSeqScanDesc = prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "pg_table_stats", "seq_scan_total"),
		"Number of sequential scans initiated on this table",
		tableLabels, nil,
	)
	tableScanStatsIdxScanDesc = prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "pg_table_stats", "idx_scan_total"),
		"Number of index scans initiated on this table",
		tableLabels, nil,
	)
	tableStatsRowCountDesc = prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "pg_table_stats", "row_count"),
		"Estimated number of live rows in this table",
		tableLabels, nil,
	)
)

type TableStatsArguments struct {
	DB               *sql.DB
	DSN              string
	ExcludeDatabases []string
	Registry         *prometheus.Registry
	CollectInterval  time.Duration

	Logger *slog.Logger

	dbConnectionFactory databaseConnectionFactory
}

type TableStats struct {
	initialConnection   *sql.DB
	dbDSN               string
	dbConnectionFactory databaseConnectionFactory
	excludeDatabases    []string
	registry            *prometheus.Registry
	collectInterval     time.Duration

	// Results of the last successful run against each database.
	cache cachedDatabaseMetrics

	logger  *slog.Logger
	running *atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewTableStats(args TableStatsArguments) (*TableStats, error) {
	if args.CollectInterval <= 0 {
		return nil, errors.New("collect interval must be positive")
	}

	factory := args.dbConnectionFactory
	if factory == nil {
		factory = defaultDbConnectionFactory
	}

	return &TableStats{
		initialConnection:   args.DB,
		dbDSN:               args.DSN,
		dbConnectionFactory: factory,
		excludeDatabases:    args.ExcludeDatabases,
		registry:            args.Registry,
		collectInterval:     args.CollectInterval,
		logger:              args.Logger.With("collector", TableStatsCollector),
		running:             &atomic.Bool{},
	}, nil
}

func (c *TableStats) Name() string {
	return TableStatsCollector
}

func (c *TableStats) Start(ctx context.Context) error {
	if err := c.registry.Register(c); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.running.Store(true)

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			c.collect(ctx)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})

	return nil
}

func (c *TableStats) Stopped() bool {
	return !c.running.Load()
}

func (c *TableStats) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.registry.Unregister(c)
	c.running.Store(false)
}

// Describe implements prometheus.Collector.
func (c *TableStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- tableScanStatsSeqScanDesc
	ch <- tableScanStatsIdxScanDesc
	ch <- tableStatsRowCountDesc
}

// Collect implements prometheus.Collector. It serves the results of the last
// successful runs and does not query any database.
func (c *TableStats) Collect(ch chan<- prometheus.Metric) {
	c.cache.emit(ch)
}

// collect queries every database the connection can reach, one at a time,
// within one collect interval. A database that fails keeps its previous
// results.
func (c *TableStats) collect(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, c.collectInterval)
	defer cancel()

	databases, err := discoverDatabases(ctx, c.initialConnection, c.excludeDatabases)
	if err != nil {
		c.logger.Error("failed to discover databases", "err", err)
		return
	}
	c.cache.keepOnly(databases)

	for _, dbName := range databases {
		if ctx.Err() != nil {
			c.logger.Error("table stats run ran out of time before reaching every database", "datname", dbName)
			return
		}

		conn, closeConn, err := connectToDatabase(c.dbDSN, dbName, c.dbConnectionFactory, c.initialConnection)
		if err != nil {
			c.logger.Error("failed to connect to database", "datname", dbName, "err", err)
			continue
		}

		metrics, err := c.queryTableScanStats(ctx, dbName, conn)
		closeConn()
		if err != nil {
			c.logger.Error("failed to collect pg_stat_user_tables", "datname", dbName, "err", err)
			continue
		}
		c.cache.set(dbName, metrics)
	}
}

func (c *TableStats) queryTableScanStats(ctx context.Context, dbName string, conn *sql.DB) ([]prometheus.Metric, error) {
	var metrics []prometheus.Metric
	err := inTransactionWithTimeouts(ctx, conn, c.collectInterval, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, selectTableScanStats)
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var schemaname, relname string
			var seqScan, idxScan, rowCount sql.NullInt64

			if err := rows.Scan(&schemaname, &relname, &seqScan, &idxScan, &rowCount); err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			metrics = append(metrics,
				prometheus.MustNewConstMetric(tableScanStatsSeqScanDesc, prometheus.CounterValue, float64(seqScan.Int64), dbName, schemaname, relname),
				prometheus.MustNewConstMetric(tableScanStatsIdxScanDesc, prometheus.CounterValue, float64(idxScan.Int64), dbName, schemaname, relname),
				prometheus.MustNewConstMetric(tableStatsRowCountDesc, prometheus.GaugeValue, float64(rowCount.Int64), dbName, schemaname, relname),
			)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return metrics, nil
}
