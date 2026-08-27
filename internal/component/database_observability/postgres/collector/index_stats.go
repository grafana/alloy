package collector

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// IndexStatsCollector emits the minimal set of metrics needed for the
// missing-index and unused-index KG insights: table-level scan counts (from
// pg_stat_user_tables) and per-index scan counts (from pg_stat_user_indexes),
// scoped to every database the connection can reach rather than only the one
// named in the DSN.
const IndexStatsCollector = "index_stats"

const (
	selectTableScanStats = `
		SELECT
			schemaname,
			relname,
			seq_scan,
			idx_scan,
			n_live_tup
		FROM pg_stat_user_tables`

	selectIndexUsageStats = `
		SELECT
			s.schemaname,
			s.relname,
			s.indexrelname,
			s.idx_scan,
			i.indisprimary,
			pg_relation_size(s.indexrelid) AS index_size_bytes
		FROM pg_stat_user_indexes s
		JOIN pg_index i ON i.indexrelid = s.indexrelid`
)

const labelDatname = "datname"

var tableLabels = []string{labelDatname, "schemaname", "relname"}

var indexLabels = []string{labelDatname, "schemaname", "relname", "indexrelname"}

var (
	tableScanStatsSeqScanDesc = prometheus.NewDesc(
		prometheus.BuildFQName("pg", "stat_user_tables", "seq_scan"),
		"Number of sequential scans initiated on this table",
		tableLabels, nil,
	)
	tableScanStatsIdxScanDesc = prometheus.NewDesc(
		prometheus.BuildFQName("pg", "stat_user_tables", "idx_scan"),
		"Number of index scans initiated on this table",
		tableLabels, nil,
	)
	tableScanStatsNLiveTupDesc = prometheus.NewDesc(
		prometheus.BuildFQName("pg", "stat_user_tables", "n_live_tup"),
		"Estimated number of live rows",
		tableLabels, nil,
	)

	// Named to match the metrics proposed by the (currently unmerged) upstream
	// prometheus-community/postgres_exporter#1071, so that adopting the real
	// upstream collector later, if/when it lands, needs no rule changes.
	indexUsageIdxScanTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName("pg", "stat_user_indexes", "idx_scan_total"),
		"Number of index scans initiated on this index",
		indexLabels, nil,
	)
	indexPropertiesDesc = prometheus.NewDesc(
		"pg_index_properties",
		"Properties of an index; a constant 1 with is_primary set to whether the index backs a primary key",
		append(append([]string{}, indexLabels...), "is_primary"), nil,
	)
	indexSizeBytesDesc = prometheus.NewDesc(
		"pg_index_size_bytes",
		"Total disk space used by this index, in bytes",
		indexLabels, nil,
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
	ch <- tableScanStatsSeqScanDesc
	ch <- tableScanStatsIdxScanDesc
	ch <- tableScanStatsNLiveTupDesc
	ch <- indexUsageIdxScanTotalDesc
	ch <- indexPropertiesDesc
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

		c.collectTableScanStats(ctx, dbName, conn, ch)
		c.collectIndexUsageStats(ctx, dbName, conn, ch)

		closeConn()
	}
}

func (c *IndexStats) collectTableScanStats(ctx context.Context, dbName string, conn *sql.DB, ch chan<- prometheus.Metric) {
	rows, err := conn.QueryContext(ctx, selectTableScanStats)
	if err != nil {
		c.logger.Error("failed to query pg_stat_user_tables", "datname", dbName, "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schemaname, relname string
		var seqScan, idxScan, nLiveTup sql.NullInt64

		if err := rows.Scan(&schemaname, &relname, &seqScan, &idxScan, &nLiveTup); err != nil {
			c.logger.Error("failed to scan pg_stat_user_tables row", "datname", dbName, "err", err)
			return
		}

		ch <- prometheus.MustNewConstMetric(tableScanStatsSeqScanDesc, prometheus.CounterValue, float64(seqScan.Int64), dbName, schemaname, relname)
		ch <- prometheus.MustNewConstMetric(tableScanStatsIdxScanDesc, prometheus.CounterValue, float64(idxScan.Int64), dbName, schemaname, relname)
		ch <- prometheus.MustNewConstMetric(tableScanStatsNLiveTupDesc, prometheus.GaugeValue, float64(nLiveTup.Int64), dbName, schemaname, relname)
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating pg_stat_user_tables rows", "datname", dbName, "err", err)
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
		var isPrimary bool

		if err := rows.Scan(&schemaname, &relname, &indexrelname, &idxScan, &isPrimary, &indexSizeBytes); err != nil {
			c.logger.Error("failed to scan pg_stat_user_indexes row", "datname", dbName, "err", err)
			return
		}

		isPrimaryLabel := "false"
		if isPrimary {
			isPrimaryLabel = "true"
		}

		ch <- prometheus.MustNewConstMetric(indexUsageIdxScanTotalDesc, prometheus.CounterValue, float64(idxScan.Int64), dbName, schemaname, relname, indexrelname)
		ch <- prometheus.MustNewConstMetric(indexPropertiesDesc, prometheus.GaugeValue, 1, dbName, schemaname, relname, indexrelname, isPrimaryLabel)
		ch <- prometheus.MustNewConstMetric(indexSizeBytesDesc, prometheus.GaugeValue, float64(indexSizeBytes.Int64), dbName, schemaname, relname, indexrelname)
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating pg_stat_user_indexes rows", "datname", dbName, "err", err)
	}
}
