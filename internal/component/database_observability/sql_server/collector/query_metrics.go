package collector

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const QueryMetricsCollector = "query_metrics"

// selectQueryStoreState reads the connected database's Query Store state
const selectQueryStoreState = `
SELECT
	DB_NAME(),
	actual_state_desc,
	query_capture_mode_desc,
	readonly_reason
FROM sys.database_query_store_options`

// selectQueryMetrics ranks the top-N queries by duration within the recent lookback
// window, then sums their full retained Query Store history so the emitted
// counters stay monotonic across interval rollover.
//
// Note: the final SELECT sums the full retained history per ranked hash
// (no lookback filter on the aggregation), while the lookback window only
// constrains the top-N ranking/eligibility in the eligible/ranked CTEs.
// In internal state, on a Query Store bucket rollover (when interval ends),
// the cumulative totals keep growing → deltas stay non-negative → the
// Prometheus counters remain monotonic. The only time totals decrease
// is when Query Store is cleared, or when retention/cleanup drops an old
// interval: in that case the state re-baselines rather than emitting a spike.
const selectQueryMetrics = `
WITH eligible AS (
	SELECT DISTINCT q.query_hash
	FROM sys.query_store_query q
	WHERE q.is_internal_query = 0
		AND q.query_hash IS NOT NULL
		AND q.last_execution_time >= DATEADD(SECOND, -CONVERT(int, @lookback_window), SYSUTCDATETIME())
),
ranked AS (
	SELECT TOP (@limit) q.query_hash
	FROM sys.query_store_query q
	JOIN eligible e ON e.query_hash = q.query_hash
	JOIN sys.query_store_plan p ON p.query_id = q.query_id
	JOIN sys.query_store_runtime_stats rs ON rs.plan_id = p.plan_id
	JOIN sys.query_store_runtime_stats_interval i
		ON i.runtime_stats_interval_id = rs.runtime_stats_interval_id
	WHERE q.is_internal_query = 0
		AND i.end_time >= DATEADD(SECOND, -CONVERT(int, @lookback_window), SYSUTCDATETIME())
	GROUP BY q.query_hash
	ORDER BY SUM(CONVERT(float, rs.avg_duration) * CONVERT(float, rs.count_executions)) DESC,
		q.query_hash ASC
)
SELECT
	q.query_hash,
	SUM(CONVERT(bigint, rs.count_executions)) AS executions,
	SUM(CASE
		WHEN rs.execution_type IN (3, 4)
		THEN CONVERT(bigint, rs.count_executions)
		ELSE 0 END) AS errors,
	SUM(CONVERT(float, rs.avg_duration) * CONVERT(float, rs.count_executions)) AS total_duration_us
FROM sys.query_store_query q
JOIN ranked r ON r.query_hash = q.query_hash
JOIN sys.query_store_plan p ON p.query_id = q.query_id
JOIN sys.query_store_runtime_stats rs ON rs.plan_id = p.plan_id
WHERE q.is_internal_query = 0
GROUP BY q.query_hash
ORDER BY q.query_hash ASC`

type QueryMetricsArguments struct {
	DB              *sql.DB
	Registry        *prometheus.Registry
	CollectInterval time.Duration
	Limit           int
	Lookback        time.Duration
	Logger          *slog.Logger
}

type QueryMetrics struct {
	dbConnection    *sql.DB
	registry        *prometheus.Registry
	collectInterval time.Duration
	limit           int
	lookback        time.Duration
	logger          *slog.Logger

	executionsMetric *prometheus.CounterVec
	errorsMetric     *prometheus.CounterVec
	durationMetric   *prometheus.CounterVec
	metrics          []prometheus.Collector
	registered       bool
	state            *queryMetricsState

	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewQueryMetrics(args QueryMetricsArguments) (*QueryMetrics, error) {
	labels := []string{"database", "query_hash"}
	executionsMetric := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "database_observability",
		Name:      "query_executions_total",
		Help:      "The count of query executions by query_hash.",
	}, labels)
	errorsMetric := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "database_observability",
		Name:      "query_errors_total",
		Help:      "The count of failed query executions by query_hash.",
	}, labels)
	durationMetric := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "database_observability",
		Name:      "query_duration_seconds_total",
		Help:      "The total query execution duration in seconds by query_hash.",
	}, labels)

	ttl := args.Lookback
	if floor := 5 * args.CollectInterval; floor > ttl {
		ttl = floor
	}

	return &QueryMetrics{
		dbConnection:     args.DB,
		registry:         args.Registry,
		collectInterval:  args.CollectInterval,
		limit:            args.Limit,
		lookback:         args.Lookback,
		logger:           args.Logger.With("collector", QueryMetricsCollector),
		executionsMetric: executionsMetric,
		errorsMetric:     errorsMetric,
		durationMetric:   durationMetric,
		metrics:          []prometheus.Collector{executionsMetric, errorsMetric, durationMetric},
		state:            newQueryMetricsState(executionsMetric, errorsMetric, durationMetric, ttl),
		running:          atomic.NewBool(false),
	}, nil
}

func (c *QueryMetrics) Name() string {
	return QueryMetricsCollector
}

func (c *QueryMetrics) Start(ctx context.Context) error {
	c.logger.Debug("collector started")

	var registered []prometheus.Collector
	for _, m := range c.metrics {
		if err := c.registry.Register(m); err != nil {
			for _, done := range registered {
				c.registry.Unregister(done)
			}
			return fmt.Errorf("failed to register query metrics: %w", err)
		}
		registered = append(registered, m)
	}
	c.registered = true

	c.running.Store(true)
	c.ctx, c.cancel = context.WithCancel(ctx)

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			if err := c.collectWithTimeout(c.ctx); err != nil {
				c.logger.Error("collector error", "err", err)
			}

			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	})

	return nil
}

func (c *QueryMetrics) Stopped() bool {
	return !c.running.Load()
}

func (c *QueryMetrics) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	if c.registered {
		for _, m := range c.metrics {
			c.registry.Unregister(m)
		}
		c.registered = false
	}
	c.state.reset()
}

func (c *QueryMetrics) collectWithTimeout(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.collect(ctx)
}

func (c *QueryMetrics) collect(ctx context.Context) error {
	database, ok := c.checkQueryStoreState(ctx)
	if !ok {
		// Query Store is unavailable on the connected database, skip this cycle.
		// Existing counters and baselines are preserved if this is a transient error.
		return nil
	}

	sources, err := c.fetchQueryStoreMetrics(ctx, database)
	if err != nil {
		return err
	}

	c.state.update(sources)
	return nil
}

// checkQueryStoreState reports whether Query Store is available on the connected database
// and returns that database's name.
func (c *QueryMetrics) checkQueryStoreState(ctx context.Context) (string, bool) {
	var database, actualState, captureMode sql.NullString
	var readonlyReason sql.NullInt64

	err := c.dbConnection.QueryRowContext(ctx, selectQueryStoreState).
		Scan(&database, &actualState, &captureMode, &readonlyReason)

	if errors.Is(err, sql.ErrNoRows) {
		c.logger.Warn("Query Store options are unavailable: the login may lack VIEW DATABASE STATE, or the connected database has no Query Store")
		return "", false
	}
	if err != nil {
		c.logger.Warn("failed to inspect Query Store state; skipping collection", "err", err)
		return "", false
	}

	if state := strings.ToUpper(strings.TrimSpace(actualState.String)); state != "READ_WRITE" {
		c.logger.Warn("Query Store is not READ_WRITE; skipping collection",
			"actual_state", actualState.String,
			"capture_mode", captureMode.String,
			"readonly_reason", readonlyReason.Int64)
		return "", false
	}

	return database.String, true
}

func (c *QueryMetrics) fetchQueryStoreMetrics(ctx context.Context, database string) ([]queryMetricSource, error) {
	rows, err := c.dbConnection.QueryContext(ctx, selectQueryMetrics,
		sql.Named("limit", c.limit),
		sql.Named("lookback_window", int(c.lookback/time.Second)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query Query Store metrics: %w", err)
	}
	defer rows.Close()

	var sources []queryMetricSource
	for rows.Next() {
		var hash []byte
		var executions, errorExecutions int64
		var durationMicroseconds float64
		if err := rows.Scan(&hash, &executions, &errorExecutions, &durationMicroseconds); err != nil {
			return nil, fmt.Errorf("failed to scan Query Store metrics: %w", err)
		}

		queryHash, err := formatQueryHash(hash)
		if err != nil {
			return nil, err
		}

		sources = append(sources, queryMetricSource{
			database:        database,
			queryHash:       queryHash,
			executions:      executions,
			errors:          errorExecutions,
			durationSeconds: durationMicroseconds / 1e6,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read Query Store metrics: %w", err)
	}

	return sources, nil
}

// formatQueryHash formats the Query Store binary(8) query_hash as a fixed
// 16-character lowercase hex string.
func formatQueryHash(hash []byte) (string, error) {
	if len(hash) != 8 {
		return "", fmt.Errorf("invalid Query Store query hash length %d, expected 8 bytes", len(hash))
	}
	return hex.EncodeToString(hash), nil
}
