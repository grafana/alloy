package client

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/grafana/dskit/backoff"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal/savepoint"
)

type endpoint struct {
	cfg     Config
	metrics *metrics
	logger  *slog.Logger
	shards  *shards
}

func newEndpoint(metrics *metrics, cfg Config, logger *slog.Logger, tracker savepoint.Tracker) (*endpoint, error) {
	logger = logger.With("component", "endpoint", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, tracker, cfg)
	if err != nil {
		return nil, err
	}

	return &endpoint{
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
		shards:  shards,
	}, nil
}

// start starts the endpoint and must be called before the first call to enqueue.
func (e *endpoint) start() {
	e.shards.start(e.cfg.QueueConfig.MinShards)
}

// enqueue tries to enqueue an entry. It waits for room while BlockOnOverflow
// is set and drops the entry when it is not, returning errQueueIsFull. It will
// return context error if caller cancels context or loki.ErrConsumerStopped
// if endpoint has been stopped.
func (e *endpoint) enqueue(ctx context.Context, entry loki.Entry, segmentNum int) error {
	bo := backoff.New(ctx, backoff.Config{
		MinBackoff: 5 * time.Millisecond,
		MaxBackoff: 50 * time.Millisecond,
	})

	tenantID := getTenantID(e.cfg, entry)

	for bo.Ongoing() {
		err := e.shards.enqueue(tenantID, entry, segmentNum)

		if err == nil {
			return nil
		}

		if errors.Is(err, loki.ErrConsumerStopped) {
			return err
		}

		if errors.Is(err, errQueueIsFull) && !e.cfg.QueueConfig.BlockOnOverflow {
			e.metrics.droppedEntries.WithLabelValues(e.cfg.URL.Host, tenantID, reasonQueueIsFull).Inc()
			e.metrics.droppedBytes.WithLabelValues(e.cfg.URL.Host, tenantID, reasonQueueIsFull).Add(float64(entry.Size()))
			return errQueueIsFull
		}

		bo.Wait()
	}

	return bo.Err()
}

// stop stops the endpoint, waiting up to the configured drain timeout for queued
// entries to be sent before canceling in-flight requests.
func (e *endpoint) stop() {
	e.shards.stop()
}

func getTenantID(cfg Config, e loki.Entry) string {
	// Check if it has been overridden while processing the pipeline stages
	if value, ok := e.Labels[ReservedLabelTenantID]; ok {
		return string(value)
	}

	return cfg.TenantID
}
