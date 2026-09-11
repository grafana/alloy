package client

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/grafana/dskit/backoff"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal/marker"
)

type endpoint struct {
	cfg     Config
	metrics *metrics
	logger  *slog.Logger
	shards  *shards
}

func newEndpoint(metrics *metrics, cfg Config, logger *slog.Logger, markerHandler marker.Tracker) (*endpoint, error) {
	logger = logger.With("component", "endpoint", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, markerHandler, cfg)
	if err != nil {
		return nil, err
	}

	c := &endpoint{
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
		shards:  shards,
	}

	c.shards.start(cfg.QueueConfig.MinShards)
	return c, nil
}

var errQueueIsFull = errors.New("queue is full")

// enqueue tries to enqueue an entry. It returns an error if the entry could not be enqueued.
// errQueueIsFull when the queue is full and BlockOnOverflow is false, or context.Canceled if
// caller canceled ctx.
func (e *endpoint) enqueue(ctx context.Context, entry loki.Entry, segmentNum int) error {
	bo := backoff.New(ctx, backoff.Config{
		MinBackoff: 5 * time.Millisecond,
		MaxBackoff: 50 * time.Millisecond,
	})

	tenantID := getTenantID(e.cfg, entry)

	for bo.Ongoing() {
		if e.shards.enqueue(tenantID, entry, segmentNum) {
			return nil
		}

		if !e.cfg.QueueConfig.BlockOnOverflow {
			e.metrics.droppedEntries.WithLabelValues(e.cfg.URL.Host, tenantID, reasonQueueIsFull).Inc()
			e.metrics.droppedBytes.WithLabelValues(e.cfg.URL.Host, tenantID, reasonQueueIsFull).Add(float64(entry.Size()))
			return errQueueIsFull
		}

		bo.Wait()
	}

	return bo.Err()
}

func (e *endpoint) stop() {
	e.shards.stop()
}

// getEndpointName computes the specific name for each endpoint config. The name is either the configured Name setting in Config,
// or a hash of the config as whole, this allows us to detect repeated configs.
func getEndpointName(cfg Config) string {
	if cfg.Name != "" {
		return cfg.Name
	}

	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%v", cfg)
	return fmt.Sprintf("%x", h.Sum(nil))[:6]
}

func getTenantID(cfg Config, e loki.Entry) string {
	// Check if it has been overridden while processing the pipeline stages
	if value, ok := e.Labels[ReservedLabelTenantID]; ok {
		return string(value)
	}

	return cfg.TenantID
}
