package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// TableStatsCollector emits the fetch count of the index="NONE" ("no index
// used") row per table, from
// performance_schema.table_io_waits_summary_by_index_usage.
//
// The queries run on a fixed collect interval, in the background, and each run
// is bounded by a timeout equal to the interval. A scrape only serves the
// results of the last successful run and never queries the database, so slow
// queries on servers with many tables cannot pile up behind scrapes.
const TableStatsCollector = "table_stats"

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
	DB              *sql.DB
	ExcludeSchemas  []string
	Registry        *prometheus.Registry
	CollectInterval time.Duration

	// IOWaits is the shared scan of table_io_waits_summary_by_index_usage. A
	// scan of its own is used when it is nil.
	IOWaits *IOWaitsScan

	Logger *slog.Logger
}

type TableStats struct {
	dbConnection    *sql.DB
	excludeSchemas  []string
	registry        *prometheus.Registry
	collectInterval time.Duration
	ioWaits         *IOWaitsScan

	// Results of the last successful run of each query.
	noIdxFetch cachedMetrics
	rowCount   cachedMetrics

	logger  *slog.Logger
	running *atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewTableStats(args TableStatsArguments) (*TableStats, error) {
	if args.CollectInterval <= 0 {
		return nil, errors.New("collect interval must be positive")
	}

	ioWaits := args.IOWaits
	if ioWaits == nil {
		ioWaits = NewIOWaitsScan(args.DB, args.ExcludeSchemas)
	}

	return &TableStats{
		ioWaits:         ioWaits,
		dbConnection:    args.DB,
		excludeSchemas:  args.ExcludeSchemas,
		registry:        args.Registry,
		collectInterval: args.CollectInterval,
		logger:          args.Logger.With("collector", TableStatsCollector),
		running:         &atomic.Bool{},
	}, nil
}

func (c *TableStats) Name() string {
	return TableStatsCollector
}

func (c *TableStats) Start(ctx context.Context) error {
	if err := c.registry.Register(c); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.running.Store(true)

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			c.collect(ctx)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})

	return nil
}

func (c *TableStats) Stopped() bool {
	return !c.running.Load()
}

func (c *TableStats) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.registry.Unregister(c)
	c.running.Store(false)
}

// Describe implements prometheus.Collector.
func (c *TableStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- tableStatsNoIdxFetchDesc
	ch <- tableStatsRowCountDesc
}

// Collect implements prometheus.Collector. It serves the results of the last
// successful run and does not query the database.
func (c *TableStats) Collect(ch chan<- prometheus.Metric) {
	c.noIdxFetch.emit(ch)
	c.rowCount.emit(ch)
}

// collect runs both queries, bounded by a timeout of one collect interval. A
// query that fails leaves its previous results in place.
func (c *TableStats) collect(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, c.collectInterval)
	defer cancel()

	if metrics, err := c.queryNoIdxFetch(ctx); err != nil {
		c.logger.Error("failed to collect table_io_waits_summary_by_index_usage", "err", err)
	} else {
		c.noIdxFetch.set(metrics)
	}

	if metrics, err := c.queryRowCount(ctx); err != nil {
		c.logger.Error("failed to collect mysql.innodb_table_stats", "err", err)
	} else {
		c.rowCount.set(metrics)
	}
}

func (c *TableStats) queryNoIdxFetch(ctx context.Context) ([]prometheus.Metric, error) {
	rows, err := c.ioWaits.get(ctx, c.collectInterval/2)
	if err != nil {
		return nil, err
	}

	var metrics []prometheus.Metric
	for _, r := range rows {
		if r.index.Valid {
			continue
		}
		metrics = append(metrics, prometheus.MustNewConstMetric(tableStatsNoIdxFetchDesc, prometheus.CounterValue, float64(r.countFetch), r.schema, r.object))
	}
	return metrics, nil
}

func (c *TableStats) queryRowCount(ctx context.Context) ([]prometheus.Metric, error) {
	query := fmt.Sprintf(selectTableRowCount, buildExcludedSchemasClause(c.excludeSchemas))
	rows, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var metrics []prometheus.Metric
	for rows.Next() {
		var databaseName, tableName string
		var rowCount int64

		if err := rows.Scan(&databaseName, &tableName, &rowCount); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		metrics = append(metrics, prometheus.MustNewConstMetric(tableStatsRowCountDesc, prometheus.GaugeValue, float64(rowCount), databaseName, tableName))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate: %w", err)
	}
	return metrics, nil
}
