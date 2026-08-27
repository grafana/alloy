package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// TableStatsCollector emits the minimal set of table-level metrics needed for
// the missing-index KG insight: the fetch-operation count of the index="NONE"
// ("no index used") row per table, from
// performance_schema.table_io_waits_summary_by_index_usage.
//
// Named and labeled to match the metric mysqld_exporter's perf_schema.indexiowaits
// scraper already emits (mysql_perf_schema_index_io_waits_total{schema,name,index,operation}),
// but restricted to the fetch operation on the table-level (no-index) row.
const TableStatsCollector = "table_stats"

const selectTableIOWaitsNoIndex = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE INDEX_NAME IS NULL AND OBJECT_SCHEMA NOT IN %s`

var indexIOWaitsFetchDesc = prometheus.NewDesc(
	"mysql_perf_schema_index_io_waits_total",
	"The total number of index I/O wait events for each index and operation.",
	[]string{"schema", "name", "index", "operation"}, nil,
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
	ch <- indexIOWaitsFetchDesc
}

// Collect implements prometheus.Collector. It runs synchronously at scrape time.
func (c *TableStats) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

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

		ch <- prometheus.MustNewConstMetric(indexIOWaitsFetchDesc, prometheus.CounterValue, float64(countFetch), objectSchema, objectName, "NONE", "fetch")
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating table_io_waits_summary_by_index_usage rows", "err", err)
	}
}
