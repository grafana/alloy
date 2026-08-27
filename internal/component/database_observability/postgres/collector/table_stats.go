package collector

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// TableStatsCollector emits the minimal set of table-level metrics needed for
// the missing-index KG insight, from pg_stat_user_tables, scoped to every
// database the connection can reach rather than only the one named in the DSN.
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
)

type TableStatsArguments struct {
	DB               *sql.DB
	DSN              string
	ExcludeDatabases []string
	Registry         *prometheus.Registry

	Logger *slog.Logger

	dbConnectionFactory databaseConnectionFactory
}

type TableStats struct {
	initialConnection   *sql.DB
	dbDSN               string
	dbConnectionFactory databaseConnectionFactory
	excludeDatabases    []string
	registry            *prometheus.Registry

	logger  *slog.Logger
	running *atomic.Bool
}

func NewTableStats(args TableStatsArguments) (*TableStats, error) {
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
		logger:              args.Logger.With("collector", TableStatsCollector),
		running:             &atomic.Bool{},
	}, nil
}

func (c *TableStats) Name() string {
	return TableStatsCollector
}

func (c *TableStats) Start(_ context.Context) error {
	if err := c.registry.Register(c); err != nil {
		return err
	}
	c.running.Store(true)
	return nil
}

func (c *TableStats) Stopped() bool {
	return !c.running.Load()
}

func (c *TableStats) Stop() {
	c.registry.Unregister(c)
	c.running.Store(false)
}

// Describe implements prometheus.Collector.
func (c *TableStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- tableScanStatsSeqScanDesc
	ch <- tableScanStatsIdxScanDesc
	ch <- tableScanStatsNLiveTupDesc
}

// Collect implements prometheus.Collector. It runs synchronously at scrape
// time, fanning out to every database the connection can reach.
func (c *TableStats) Collect(ch chan<- prometheus.Metric) {
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

		closeConn()
	}
}

func (c *TableStats) collectTableScanStats(ctx context.Context, dbName string, conn *sql.DB, ch chan<- prometheus.Metric) {
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
