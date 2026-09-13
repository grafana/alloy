package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// IndexStatsCollector emits the fetch count of each named-index row, from
// performance_schema.table_io_waits_summary_by_index_usage, and its size in
// bytes, from mysql.innodb_index_stats.
const IndexStatsCollector = "index_stats"

const selectIndexIOWaits = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, INDEX_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE INDEX_NAME IS NOT NULL AND OBJECT_SCHEMA NOT IN %s`

// mysql.innodb_index_stats holds InnoDB's persistent optimizer statistics,
// refreshed by MySQL itself. NON_UNIQUE comes from information_schema.statistics,
// joined on seq_in_index = 1 since uniqueness is a property of the index, not of
// any one column within it, and would otherwise repeat once per indexed column.
const selectIndexSizeBytes = `
	SELECT
		s.database_name,
		s.table_name,
		s.index_name,
		s.stat_value * @@innodb_page_size,
		stats.NON_UNIQUE
	FROM mysql.innodb_index_stats s
	LEFT JOIN information_schema.statistics stats
		ON stats.TABLE_SCHEMA = s.database_name
		AND stats.TABLE_NAME = s.table_name
		AND stats.INDEX_NAME = s.index_name
		AND stats.SEQ_IN_INDEX = 1
	WHERE s.stat_name = 'size' AND s.database_name NOT IN %s`

var indexLabels = []string{labelSchema, labelTable, "index"}
var indexSizeLabels = append(append([]string{}, indexLabels...), "is_primary", "is_unique")

var (
	indexStatsIdxFetchDesc = prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "mysql_index_stats", "idx_fetch_total"),
		"Count of index I/O wait events for fetch operations",
		indexLabels, nil,
	)
	indexStatsSizeBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "mysql_index_stats", "size_bytes"),
		"Total disk space used by this index, in bytes, labeled with whether it backs the primary key or a unique constraint",
		indexSizeLabels, nil,
	)
)

type IndexStatsArguments struct {
	DB             *sql.DB
	ExcludeSchemas []string
	Registry       *prometheus.Registry

	Logger *slog.Logger
}

type IndexStats struct {
	dbConnection   *sql.DB
	excludeSchemas []string
	registry       *prometheus.Registry

	logger  *slog.Logger
	running *atomic.Bool
}

func NewIndexStats(args IndexStatsArguments) (*IndexStats, error) {
	return &IndexStats{
		dbConnection:   args.DB,
		excludeSchemas: args.ExcludeSchemas,
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
	ch <- indexStatsIdxFetchDesc
	ch <- indexStatsSizeBytesDesc
}

// Collect implements prometheus.Collector. It runs synchronously at scrape time.
func (c *IndexStats) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	c.collectIdxFetch(ctx, ch)
	c.collectIndexSize(ctx, ch)
}

func (c *IndexStats) collectIdxFetch(ctx context.Context, ch chan<- prometheus.Metric) {
	query := fmt.Sprintf(selectIndexIOWaits, buildExcludedSchemasClause(c.excludeSchemas))
	rows, err := c.dbConnection.QueryContext(ctx, query)
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

		ch <- prometheus.MustNewConstMetric(indexStatsIdxFetchDesc, prometheus.CounterValue, float64(countFetch), objectSchema, objectName, indexName)
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating table_io_waits_summary_by_index_usage rows", "err", err)
	}
}

func (c *IndexStats) collectIndexSize(ctx context.Context, ch chan<- prometheus.Metric) {
	query := fmt.Sprintf(selectIndexSizeBytes, buildExcludedSchemasClause(c.excludeSchemas))
	rows, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		c.logger.Error("failed to query mysql.innodb_index_stats", "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var databaseName, tableName, indexName string
		var sizeBytes float64
		var nonUnique sql.NullInt64

		if err := rows.Scan(&databaseName, &tableName, &indexName, &sizeBytes, &nonUnique); err != nil {
			c.logger.Error("failed to scan mysql.innodb_index_stats row", "err", err)
			return
		}

		isPrimary := indexName == "PRIMARY"
		isUnique := nonUnique.Valid && nonUnique.Int64 == 0

		ch <- prometheus.MustNewConstMetric(indexStatsSizeBytesDesc, prometheus.GaugeValue, sizeBytes,
			databaseName, tableName, indexName,
			strconv.FormatBool(isPrimary), strconv.FormatBool(isUnique))
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating mysql.innodb_index_stats rows", "err", err)
	}
}
