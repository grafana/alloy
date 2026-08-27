package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// IndexStatsCollector emits the minimal set of per-index metrics needed for
// the unused-index KG insight: the fetch-operation count of each named-index
// row per index, from performance_schema.table_io_waits_summary_by_index_usage.
//
// Named and labeled to match the metric mysqld_exporter's perf_schema.indexiowaits
// scraper already emits (mysql_perf_schema_index_io_waits_total{schema,name,index,operation}),
// but restricted to the fetch operation on named-index rows.
const IndexStatsCollector = "index_stats"

const selectIndexIOWaits = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, INDEX_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE INDEX_NAME IS NOT NULL AND OBJECT_SCHEMA NOT IN (%s)`

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
	ch <- indexIOWaitsFetchDesc
}

// Collect implements prometheus.Collector. It runs synchronously at scrape time.
func (c *IndexStats) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

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

		ch <- prometheus.MustNewConstMetric(indexIOWaitsFetchDesc, prometheus.CounterValue, float64(countFetch), objectSchema, objectName, indexName, "fetch")
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating table_io_waits_summary_by_index_usage rows", "err", err)
	}
}
