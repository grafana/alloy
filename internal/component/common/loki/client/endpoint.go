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

	return &endpoint{
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
		shards:  shards,
	}, nil
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

func (e *endpoint) start() {
	e.shards.start(e.cfg.QueueConfig.MinShards)
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
