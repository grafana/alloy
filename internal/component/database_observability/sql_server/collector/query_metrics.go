package collector

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const QueryMetricsCollector = "query_metrics"

type QueryMetricsArguments struct {
	DB                 *sql.DB
	Registry           *prometheus.Registry
	CollectInterval    time.Duration
	QueryTimeout       time.Duration
	Limit              int
	Lookback           time.Duration
	ExcludeCurrentUser bool
	Logger             *slog.Logger
}

type QueryMetrics struct {
	dbConnection       *sql.DB
	registry           *prometheus.Registry
	queryTimeout       time.Duration
	collectInterval    time.Duration
	limit              int
	lookback           time.Duration
	excludeCurrentUser bool
	logger             *slog.Logger

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
		dbConnection:       args.DB,
		registry:           args.Registry,
		collectInterval:    args.CollectInterval,
		queryTimeout:       queryTimeoutOrDefault(args.QueryTimeout),
		limit:              args.Limit,
		lookback:           args.Lookback,
		excludeCurrentUser: args.ExcludeCurrentUser,
		logger:             args.Logger.With("collector", QueryMetricsCollector),
		executionsMetric:   executionsMetric,
		errorsMetric:       errorsMetric,
		durationMetric:     durationMetric,
		metrics:            []prometheus.Collector{executionsMetric, errorsMetric, durationMetric},
		state:              newQueryMetricsState(executionsMetric, errorsMetric, durationMetric, ttl),
		running:            atomic.NewBool(false),
	}, nil
}

func (c *QueryMetrics) Name() string {
	return QueryMetricsCollector
}

// Tracker exposes the collector's query-tracking state so other collectors
// (e.g. query_details) can restrict their work to the query_hashes that are
// currently tracked here. The returned value is safe for concurrent use and
// remains valid across collector restarts.
func (c *QueryMetrics) Tracker() QueryTracker {
	return c.state
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
			if err := c.collect(c.ctx); err != nil {
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

func (c *QueryMetrics) collect(ctx context.Context) error {
	database, ok := checkQueryStoreState(ctx, c.dbConnection, c.queryTimeout, c.logger)
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

func (c *QueryMetrics) fetchQueryStoreMetrics(ctx context.Context, database string) ([]queryMetricSource, error) {
	var sources []queryMetricSource
	err := withQueryTimeout(ctx, c.queryTimeout, func(queryCtx context.Context) error {
		rows, err := c.dbConnection.QueryContext(
			queryCtx, selectQueryMetrics,
			sql.Named("limit", c.limit),
			sql.Named("lookback_window", int(c.lookback/time.Second)),
			sql.Named("exclude_current_user", c.excludeCurrentUser),
			sql.Named("collector_marker", QueryMarker),
		)
		if err != nil {
			return fmt.Errorf("failed to query Query Store metrics: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var hash []byte
			var executions, errorExecutions int64
			var durationMicroseconds float64
			if err := rows.Scan(&hash, &executions, &errorExecutions, &durationMicroseconds); err != nil {
				return fmt.Errorf("failed to scan Query Store metrics: %w", err)
			}

			queryHash, err := formatQueryHash(hash)
			if err != nil {
				return err
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
			return fmt.Errorf("failed to read Query Store metrics: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
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
