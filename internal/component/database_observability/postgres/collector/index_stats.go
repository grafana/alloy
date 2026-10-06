package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// IndexStatsCollector emits per-index usage counters from pg_stat_user_indexes,
// scoped to every database the connection can reach rather than only the one
// named in the DSN.
//
// The queries run on a fixed collect interval, in the background, and each run
// is bounded by a timeout equal to the interval, on the client and on the
// server. pg_relation_size also waits for a lock when a DDL statement holds one
// on the index, so each query additionally has a lock timeout. A scrape only
// serves the results of the last successful run and never queries a database.
const IndexStatsCollector = "index_stats"

const selectIndexUsageStats = `
	SELECT
		s.schemaname,
		s.relname,
		s.indexrelname,
		s.idx_scan,
		i.indisprimary,
		i.indisunique,
		i.indpred IS NOT NULL AS is_partial,
		pg_relation_size(s.indexrelid) AS index_size_bytes
	FROM pg_stat_user_indexes s
	JOIN pg_index i ON i.indexrelid = s.indexrelid`

var indexLabels = []string{labelDatname, "schemaname", "relname", "indexrelname"}
var indexSizeLabels = append(append([]string{}, indexLabels...), "is_primary", "is_unique", "is_partial")

var (
	indexUsageIdxScanTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "pg_index_stats", "idx_scan_total"),
		"Number of index scans initiated on this index",
		indexLabels, nil,
	)
	indexSizeBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "pg_index_stats", "size_bytes"),
		"Total disk space used by this index, in bytes, labeled with whether it backs the primary key or a unique constraint, or is partial",
		indexSizeLabels, nil,
	)
)

type IndexStatsArguments struct {
	DB               *sql.DB
	DSN              string
	ExcludeDatabases []string
	Registry         *prometheus.Registry
	CollectInterval  time.Duration

	Logger *slog.Logger

	dbConnectionFactory databaseConnectionFactory
}

type IndexStats struct {
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

func NewIndexStats(args IndexStatsArguments) (*IndexStats, error) {
	if args.CollectInterval <= 0 {
		return nil, errors.New("collect interval must be positive")
	}

	factory := args.dbConnectionFactory
	if factory == nil {
		factory = defaultDbConnectionFactory
	}

	return &IndexStats{
		initialConnection:   args.DB,
		dbDSN:               args.DSN,
		dbConnectionFactory: factory,
		excludeDatabases:    args.ExcludeDatabases,
		registry:            args.Registry,
		collectInterval:     args.CollectInterval,
		logger:              args.Logger.With("collector", IndexStatsCollector),
		running:             &atomic.Bool{},
	}, nil
}

func (c *IndexStats) Name() string {
	return IndexStatsCollector
}

func (c *IndexStats) Start(ctx context.Context) error {
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

func (c *IndexStats) Stopped() bool {
	return !c.running.Load()
}

func (c *IndexStats) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.registry.Unregister(c)
	c.running.Store(false)
}

// Describe implements prometheus.Collector.
func (c *IndexStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- indexUsageIdxScanTotalDesc
	ch <- indexSizeBytesDesc
}

// Collect implements prometheus.Collector. It serves the results of the last
// successful runs and does not query any database.
func (c *IndexStats) Collect(ch chan<- prometheus.Metric) {
	c.cache.emit(ch)
}

// collect queries every database the connection can reach, one at a time,
// within one collect interval. A database that fails keeps its previous
// results.
func (c *IndexStats) collect(ctx context.Context) {
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
			c.logger.Error("index stats run ran out of time before reaching every database", "datname", dbName)
			return
		}

		conn, closeConn, err := connectToDatabase(c.dbDSN, dbName, c.dbConnectionFactory, c.initialConnection)
		if err != nil {
			c.logger.Error("failed to connect to database", "datname", dbName, "err", err)
			continue
		}

		metrics, err := c.queryIndexUsageStats(ctx, dbName, conn)
		closeConn()
		if err != nil {
			c.logger.Error("failed to collect pg_stat_user_indexes", "datname", dbName, "err", err)
			continue
		}
		c.cache.set(dbName, metrics)
	}
}

func (c *IndexStats) queryIndexUsageStats(ctx context.Context, dbName string, conn *sql.DB) ([]prometheus.Metric, error) {
	var metrics []prometheus.Metric
	err := inTransactionWithTimeouts(ctx, conn, c.collectInterval, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, selectIndexUsageStats)
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var schemaname, relname, indexrelname string
			var idxScan, indexSizeBytes sql.NullInt64
			var isPrimary, isUnique, isPartial bool

			if err := rows.Scan(&schemaname, &relname, &indexrelname, &idxScan, &isPrimary, &isUnique, &isPartial, &indexSizeBytes); err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			metrics = append(metrics,
				prometheus.MustNewConstMetric(indexUsageIdxScanTotalDesc, prometheus.CounterValue, float64(idxScan.Int64), dbName, schemaname, relname, indexrelname),
				prometheus.MustNewConstMetric(indexSizeBytesDesc, prometheus.GaugeValue, float64(indexSizeBytes.Int64),
					dbName, schemaname, relname, indexrelname,
					strconv.FormatBool(isPrimary), strconv.FormatBool(isUnique), strconv.FormatBool(isPartial)),
			)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return metrics, nil
}
