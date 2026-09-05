package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// TableStatsCollector emits the minimal table-level metric needed for the
// missing-index KG insight: the fetch count of the index="NONE" ("no index
// used") row per table, from
// performance_schema.table_io_waits_summary_by_index_usage.
const TableStatsCollector = "table_stats"

const selectTableIOWaitsNoIndex = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE INDEX_NAME IS NULL AND OBJECT_SCHEMA NOT IN (%s)`

const (
	labelSchema = "schema"
	labelTable  = "table"
)

var tableStatsSeqScanDesc = prometheus.NewDesc(
	prometheus.BuildFQName("mysql", "table_stats", "seq_scan_total"),
	"Number of row fetches against this table that did not use an index",
	[]string{labelSchema, labelTable}, nil,
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
	ch <- tableStatsSeqScanDesc
}

// Collect implements prometheus.Collector. It runs synchronously at scrape time.
func (c *TableStats) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	args := excludedSchemasArgs(c.excludeSchemas)
	query := fmt.Sprintf(selectTableIOWaitsNoIndex, sqlPlaceholders(len(args)))
	rows, err := c.dbConnection.QueryContext(ctx, query, args...)
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

		ch <- prometheus.MustNewConstMetric(tableStatsSeqScanDesc, prometheus.CounterValue, float64(countFetch), objectSchema, objectName)
	}

	if err := rows.Err(); err != nil {
		c.logger.Error("error iterating table_io_waits_summary_by_index_usage rows", "err", err)
	}
}
