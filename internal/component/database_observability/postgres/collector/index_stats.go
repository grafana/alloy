package collector

import (
	"context"
	"database/sql"
	"log/slog"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// IndexStatsCollector emits per-index usage counters from pg_stat_user_indexes,
// scoped to every database the connection can reach rather than only the one
// named in the DSN.
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

	Logger *slog.Logger

	dbConnectionFactory databaseConnectionFactory
}

type IndexStats struct {
	initialConnection   *sql.DB
	dbDSN               string
	dbConnectionFactory databaseConnectionFactory
	excludeDatabases    []string
	registry            *prometheus.Registry

	logger  *slog.Logger
	running *atomic.Bool
}

func NewIndexStats(args IndexStatsArguments) (*IndexStats, error) {
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
		logger:              args.Logger.With("collector", IndexStatsCollector),
		running:             &atomic.Bool{},
	}, nil
}

func (c *IndexStats) Name() string {
	return IndexStatsCollector
}

func (c *IndexStats) Start(_ context.Context) error {
	if err := c.registry.Register(c); err != nil {
		return err
	}
	c.running.Store(true)
	return nil
}

func (c *IndexStats) Stopped() bool {
	return !c.running.Load()
}

func (c *IndexStats) Stop() {
	c.registry.Unregister(c)
	c.running.Store(false)
}

// Describe implements prometheus.Collector.
func (c *IndexStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- indexUsageIdxScanTotalDesc
	ch <- indexSizeBytesDesc
}

// Collect implements prometheus.Collector. It runs synchronously at scrape
// time, fanning out to every database the connection can reach.
func (c *IndexStats) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	databases, err := discoverDatabases(ctx, c.initialConnection, c.excludeDatabases)
	if err != nil {
		c.logger.Error("failed to discover databases", "err", err)
		return
	}

	for _, dbName := range databases {
		conn, closeConn, err := connectToDatabase(c.dbDSN, dbName, c.dbConnectionFactory, c.initialConnection)
		if err != nil {
			c.logger.Error("failed to connect to database", "datname", dbName, "err", err)
			continue
		}

		c.collectIndexUsageStats(ctx, dbName, conn, ch)

		closeConn()
	}
}

func (c *IndexStats) collectIndexUsageStats(ctx context.Context, dbName string, conn *sql.DB, ch chan<- prometheus.Metric) {
	rows, err := conn.QueryContext(ctx, selectIndexUsageStats)
	if err != nil {
		c.logger.Error("failed to query pg_stat_user_indexes", "datname", dbName, "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schemaname, relname, indexrelname string
		var idxScan, indexSizeBytes sql.NullInt64
		var isPrimary, isUnique, isPartial bool

		if err := rows.Scan(&schemaname, &relname, &indexrelname, &idxScan, &isPrimary, &isUnique, &isPartial, &indexSizeBytes); err != nil {
			c.logger.Error("failed to scan pg_stat_user_indexes row", "datname", dbName, "err", err)
			return
		}

		ch <- prometheus.MustNewConstMetric(indexUsageIdxScanTotalDesc, prometheus.CounterValue, float64(idxScan.Int64), dbName, schemaname, relname, indexrelname)
		ch <- prometheus.MustNewConstMetric(indexSizeBytesDesc, prometheus.GaugeValue, float64(indexSizeBytes.Int64),
			dbName, schemaname, relname, indexrelname,
			strconv.FormatBool(isPrimary), strconv.FormatBool(isUnique), strconv.FormatBool(isPartial))
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating pg_stat_user_indexes rows", "datname", dbName, "err", err)
	}
}
