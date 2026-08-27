package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// IndexIOWaitsCollector emits the minimal set of metrics needed for the
// missing-index and unused-index KG insights: the fetch-operation count per
// (schema, table, index) row of performance_schema.table_io_waits_summary_by_index_usage.
// An index="NONE" row's fetch count is the "no index used" signal (missing-index);
// each named row's fetch count is the per-index usage signal (unused-index).
//
// Named and labeled to match the metric mysqld_exporter's perf_schema.indexiowaits
// scraper already emits (mysql_perf_schema_index_io_waits_total{schema,name,index,operation}),
// but restricted to the fetch operation, which is all either insight reads.
const IndexIOWaitsCollector = "index_io_waits"

const selectIndexIOWaits = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, ifnull(INDEX_NAME, 'NONE') as INDEX_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE OBJECT_SCHEMA NOT IN %s`

var indexIOWaitsFetchDesc = prometheus.NewDesc(
	"mysql_perf_schema_index_io_waits_total",
	"The total number of index I/O wait events for each index and operation.",
	[]string{"schema", "name", "index", "operation"}, nil,
)

type IndexIOWaitsArguments struct {
	DB             *sql.DB
	ExcludeSchemas []string
	Registry       *prometheus.Registry

	Logger *slog.Logger
}

type IndexIOWaits struct {
	dbConnection   *sql.DB
	excludeSchemas []string
	registry       *prometheus.Registry

	logger  *slog.Logger
	running *atomic.Bool
}

func NewIndexIOWaits(args IndexIOWaitsArguments) (*IndexIOWaits, error) {
	return &IndexIOWaits{
		dbConnection:   args.DB,
		excludeSchemas: args.ExcludeSchemas,
		registry:       args.Registry,
		logger:         args.Logger.With("collector", IndexIOWaitsCollector),
		running:        &atomic.Bool{},
	}, nil
}

func (c *IndexIOWaits) Name() string {
	return IndexIOWaitsCollector
}

func (c *IndexIOWaits) Start(_ context.Context) error {
	if err := c.registry.Register(c); err != nil {
		return err
	}
	c.running.Store(true)
	return nil
}

func (c *IndexIOWaits) Stopped() bool {
	return !c.running.Load()
}

func (c *IndexIOWaits) Stop() {
	c.registry.Unregister(c)
	c.running.Store(false)
}

// Describe implements prometheus.Collector.
func (c *IndexIOWaits) Describe(ch chan<- *prometheus.Desc) {
	ch <- indexIOWaitsFetchDesc
}

// Collect implements prometheus.Collector. It runs synchronously at scrape time.
func (c *IndexIOWaits) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

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

		ch <- prometheus.MustNewConstMetric(indexIOWaitsFetchDesc, prometheus.CounterValue, float64(countFetch), objectSchema, objectName, indexName, "fetch")
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating table_io_waits_summary_by_index_usage rows", "err", err)
	}
}
