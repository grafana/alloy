package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// IndexStatsCollector emits the fetch count of each named-index row, from
// performance_schema.table_io_waits_summary_by_index_usage, and its size in
// bytes, from mysql.innodb_index_stats.
//
// The queries run on a fixed collect interval, in the background, and each run
// is bounded by a timeout equal to the interval. A scrape only serves the
// results of the last successful run and never queries the database, so slow
// queries on servers with many tables cannot pile up behind scrapes.
const IndexStatsCollector = "index_stats"

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
	DB              *sql.DB
	ExcludeSchemas  []string
	Registry        *prometheus.Registry
	CollectInterval time.Duration

	// IOWaits is the shared scan of table_io_waits_summary_by_index_usage. A
	// scan of its own is used when it is nil.
	IOWaits *IOWaitsScan

	// SchemaFilter selects the schemas whose indexes are reported. The shared
	// scan reads every schema that any of its collectors selects.
	SchemaFilter SchemaFilter

	Logger *slog.Logger
}

type IndexStats struct {
	dbConnection    *sql.DB
	excludeSchemas  []string
	registry        *prometheus.Registry
	collectInterval time.Duration
	ioWaits         *IOWaitsScan
	schemaFilter    SchemaFilter

	// Results of the last successful run of each query.
	idxFetch  cachedMetrics
	sizeBytes cachedMetrics

	logger  *slog.Logger
	running *atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewIndexStats(args IndexStatsArguments) (*IndexStats, error) {
	if args.CollectInterval <= 0 {
		return nil, errors.New("collect interval must be positive")
	}

	ioWaits := args.IOWaits
	if ioWaits == nil {
		ioWaits = NewIOWaitsScan(args.DB, args.ExcludeSchemas, args.Logger)
	}
	ioWaits.use(args.SchemaFilter)

	return &IndexStats{
		ioWaits:         ioWaits,
		schemaFilter:    args.SchemaFilter,
		dbConnection:    args.DB,
		excludeSchemas:  args.ExcludeSchemas,
		registry:        args.Registry,
		collectInterval: args.CollectInterval,
		logger:          args.Logger.With("collector", IndexStatsCollector),
		running:         &atomic.Bool{},
	}, nil
}

func (c *IndexStats) Name() string {
	return IndexStatsCollector
}

func (c *IndexStats) Start(ctx context.Context) error {
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

func (c *IndexStats) Stopped() bool {
	return !c.running.Load()
}

func (c *IndexStats) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.registry.Unregister(c)
	c.running.Store(false)
}

// Describe implements prometheus.Collector.
func (c *IndexStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- indexStatsIdxFetchDesc
	ch <- indexStatsSizeBytesDesc
}

// Collect implements prometheus.Collector. It serves the results of the last
// successful run and does not query the database.
func (c *IndexStats) Collect(ch chan<- prometheus.Metric) {
	c.idxFetch.emit(ch)
	c.sizeBytes.emit(ch)
}

// collect runs both queries, bounded by a timeout of one collect interval. A
// query that fails leaves its previous results in place.
func (c *IndexStats) collect(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, c.collectInterval)
	defer cancel()

	if metrics, err := c.queryIdxFetch(ctx); err != nil {
		c.logger.Error("failed to collect table_io_waits_summary_by_index_usage", "err", err)
	} else {
		c.idxFetch.set(metrics)
	}

	if metrics, err := c.querySizeBytes(ctx); err != nil {
		c.logger.Error("failed to collect mysql.innodb_index_stats", "err", err)
	} else {
		c.sizeBytes.set(metrics)
	}
}

func (c *IndexStats) queryIdxFetch(ctx context.Context) ([]prometheus.Metric, error) {
	rows, err := c.ioWaits.get(ctx, c.collectInterval/2)
	if err != nil {
		return nil, err
	}

	var metrics []prometheus.Metric
	for _, r := range rows {
		if !r.index.Valid || !c.schemaFilter.Match(r.schema) {
			continue
		}
		metrics = append(metrics, prometheus.MustNewConstMetric(indexStatsIdxFetchDesc, prometheus.CounterValue, float64(r.countFetch), r.schema, r.object, r.index.String))
	}
	return metrics, nil
}

func (c *IndexStats) querySizeBytes(ctx context.Context) ([]prometheus.Metric, error) {
	query := fmt.Sprintf(selectIndexSizeBytes, buildExcludedSchemasClause(c.excludeSchemas))
	rows, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var metrics []prometheus.Metric
	for rows.Next() {
		var databaseName, tableName, indexName string
		var sizeBytes float64
		var nonUnique sql.NullInt64

		if err := rows.Scan(&databaseName, &tableName, &indexName, &sizeBytes, &nonUnique); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if !c.schemaFilter.Match(databaseName) {
			continue
		}

		isPrimary := indexName == "PRIMARY"
		isUnique := nonUnique.Valid && nonUnique.Int64 == 0

		metrics = append(metrics, prometheus.MustNewConstMetric(indexStatsSizeBytesDesc, prometheus.GaugeValue, sizeBytes,
			databaseName, tableName, indexName,
			strconv.FormatBool(isPrimary), strconv.FormatBool(isUnique)))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate: %w", err)
	}
	return metrics, nil
}
