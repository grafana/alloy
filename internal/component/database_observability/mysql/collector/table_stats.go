package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// TableStatsCollector emits the fetch count of the index="NONE" ("no index
// used") row per table, from
// performance_schema.table_io_waits_summary_by_index_usage.
const TableStatsCollector = "table_stats"

const selectTableIOWaitsNoIndex = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE INDEX_NAME IS NULL AND OBJECT_SCHEMA NOT IN %s`

// mysql.innodb_table_stats holds InnoDB's persistent optimizer statistics,
// refreshed by MySQL itself -- the sibling table to mysql.innodb_index_stats,
// already used by index_stats.go for index size.
const selectTableRowCount = `
	SELECT database_name, table_name, n_rows
	FROM mysql.innodb_table_stats
	WHERE database_name NOT IN %s`

const (
	labelSchema = "schema"
	labelTable  = "table"
)

var tableLabels = []string{labelSchema, labelTable}

var (
	tableStatsNoIdxFetchDesc = prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "mysql_table_stats", "no_idx_fetch_total"),
		"Count of index I/O wait events for fetch operations that did not use an index",
		tableLabels, nil,
	)
	tableStatsRowCountDesc = prometheus.NewDesc(
		prometheus.BuildFQName("database_observability", "mysql_table_stats", "row_count"),
		"Estimated number of rows in this table",
		tableLabels, nil,
	)
)

type TableStatsArguments struct {
	DB             *sql.DB
	ExcludeSchemas []string
	Registry       *prometheus.Registry

	Logger *slog.Logger
}

type TableStats struct {
	dbConnection   *sql.DB
	excludeSchemas []string
	registry       *prometheus.Registry

	logger  *slog.Logger
	running *atomic.Bool
}

func NewTableStats(args TableStatsArguments) (*TableStats, error) {
	return &TableStats{
		dbConnection:   args.DB,
		excludeSchemas: args.ExcludeSchemas,
		registry:       args.Registry,
		logger:         args.Logger.With("collector", TableStatsCollector),
		running:        &atomic.Bool{},
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
	ch <- tableStatsNoIdxFetchDesc
	ch <- tableStatsRowCountDesc
}

// Collect implements prometheus.Collector. It runs synchronously at scrape time.
func (c *TableStats) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	c.collectNoIdxFetch(ctx, ch)
	c.collectRowCount(ctx, ch)
}

func (c *TableStats) collectNoIdxFetch(ctx context.Context, ch chan<- prometheus.Metric) {
	query := fmt.Sprintf(selectTableIOWaitsNoIndex, buildExcludedSchemasClause(c.excludeSchemas))
	rows, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		c.logger.Error("failed to query table_io_waits_summary_by_index_usage", "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var objectSchema, objectName string
		var countFetch uint64

		if err := rows.Scan(&objectSchema, &objectName, &countFetch); err != nil {
			c.logger.Error("failed to scan table_io_waits_summary_by_index_usage row", "err", err)
			return
		}

		ch <- prometheus.MustNewConstMetric(tableStatsNoIdxFetchDesc, prometheus.CounterValue, float64(countFetch), objectSchema, objectName)
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating table_io_waits_summary_by_index_usage rows", "err", err)
	}
}

func (c *TableStats) collectRowCount(ctx context.Context, ch chan<- prometheus.Metric) {
	query := fmt.Sprintf(selectTableRowCount, buildExcludedSchemasClause(c.excludeSchemas))
	rows, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		c.logger.Error("failed to query mysql.innodb_table_stats", "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var databaseName, tableName string
		var rowCount int64

		if err := rows.Scan(&databaseName, &tableName, &rowCount); err != nil {
			c.logger.Error("failed to scan mysql.innodb_table_stats row", "err", err)
			return
		}

		ch <- prometheus.MustNewConstMetric(tableStatsRowCountDesc, prometheus.GaugeValue, float64(rowCount), databaseName, tableName)
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating mysql.innodb_table_stats rows", "err", err)
	}
}
