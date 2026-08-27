package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// IndexStatsCollector emits the minimal per-index metrics needed for the
// unused-index KG insight: the fetch count of each named-index row per
// index, from performance_schema.table_io_waits_summary_by_index_usage, and
// (MySQL >= 5.6.6 only, see minIndexSizeEngineVersion) its size in bytes,
// from mysql.innodb_index_stats.
const IndexStatsCollector = "index_stats"

const selectIndexIOWaits = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, INDEX_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE INDEX_NAME IS NOT NULL AND OBJECT_SCHEMA NOT IN (%s)`

// mysql.innodb_index_stats holds InnoDB's persistent optimizer statistics,
// refreshed by MySQL itself (never by us) -- introduced in MySQL 5.6.6, its
// schema (and the "size" stat, in pages) hasn't changed since. Reading it
// requires the monitoring role to be granted SELECT on this one table, which
// is not part of today's default grant set (see deployment_tools).
const selectIndexSizeBytes = `
	SELECT database_name, table_name, index_name, stat_value * @@innodb_page_size
	FROM mysql.innodb_index_stats
	WHERE stat_name = 'size' AND database_name NOT IN (%s)`

// minIndexSizeEngineVersion is the first MySQL version where
// mysql.innodb_index_stats exists at all (persistent optimizer statistics,
// introduced 5.6.6) -- versions older than this have no size source, so the
// query is skipped entirely below it rather than attempted and failing.
var minIndexSizeEngineVersion = semver.MustParse("5.6.6")

var indexLabels = []string{labelSchema, labelTable, "index"}

var (
	indexStatsIdxScanDesc = prometheus.NewDesc(
		prometheus.BuildFQName("mysql", "index_stats", "idx_scan_total"),
		"Number of row fetches against this index",
		indexLabels, nil,
	)
	indexStatsSizeBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("mysql", "index_stats", "size_bytes"),
		"Total disk space used by this index, in bytes",
		indexLabels, nil,
	)
)

type IndexStatsArguments struct {
	DB             *sql.DB
	ExcludeSchemas []string
	EngineVersion  semver.Version
	Registry       *prometheus.Registry

	Logger *slog.Logger
}

type IndexStats struct {
	dbConnection   *sql.DB
	excludeSchemas []string
	engineVersion  semver.Version
	registry       *prometheus.Registry

	logger  *slog.Logger
	running *atomic.Bool
}

func NewIndexStats(args IndexStatsArguments) (*IndexStats, error) {
	return &IndexStats{
		dbConnection:   args.DB,
		excludeSchemas: args.ExcludeSchemas,
		engineVersion:  args.EngineVersion,
		registry:       args.Registry,
		logger:         args.Logger.With("collector", IndexStatsCollector),
		running:        &atomic.Bool{},
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
	ch <- indexStatsIdxScanDesc
	ch <- indexStatsSizeBytesDesc
}

// Collect implements prometheus.Collector. It runs synchronously at scrape time.
func (c *IndexStats) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	c.collectIdxScan(ctx, ch)

	if c.engineVersion.GE(minIndexSizeEngineVersion) {
		c.collectIndexSize(ctx, ch)
	}
}

func (c *IndexStats) collectIdxScan(ctx context.Context, ch chan<- prometheus.Metric) {
	args := excludedSchemasArgs(c.excludeSchemas)
	query := fmt.Sprintf(selectIndexIOWaits, sqlPlaceholders(len(args)))
	rows, err := c.dbConnection.QueryContext(ctx, query, args...)
	if err != nil {
		c.logger.Error("failed to query table_io_waits_summary_by_index_usage", "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var objectSchema, objectName, indexName string
		var countFetch uint64

		if err := rows.Scan(&objectSchema, &objectName, &indexName, &countFetch); err != nil {
			c.logger.Error("failed to scan table_io_waits_summary_by_index_usage row", "err", err)
			return
		}

		ch <- prometheus.MustNewConstMetric(indexStatsIdxScanDesc, prometheus.CounterValue, float64(countFetch), objectSchema, objectName, indexName)
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating table_io_waits_summary_by_index_usage rows", "err", err)
	}
}

func (c *IndexStats) collectIndexSize(ctx context.Context, ch chan<- prometheus.Metric) {
	args := excludedSchemasArgs(c.excludeSchemas)
	query := fmt.Sprintf(selectIndexSizeBytes, sqlPlaceholders(len(args)))
	rows, err := c.dbConnection.QueryContext(ctx, query, args...)
	if err != nil {
		c.logger.Error("failed to query mysql.innodb_index_stats", "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var databaseName, tableName, indexName string
		var sizeBytes float64

		if err := rows.Scan(&databaseName, &tableName, &indexName, &sizeBytes); err != nil {
			c.logger.Error("failed to scan mysql.innodb_index_stats row", "err", err)
			return
		}

		ch <- prometheus.MustNewConstMetric(indexStatsSizeBytesDesc, prometheus.GaugeValue, sizeBytes, databaseName, tableName, indexName)
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating mysql.innodb_index_stats rows", "err", err)
	}
}
