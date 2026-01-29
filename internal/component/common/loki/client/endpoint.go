package client

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type endpoint struct {
	cfg     Config
	metrics *metrics
	logger  log.Logger
	entries chan loki.Entry

	ctx    context.Context
	cancel context.CancelFunc

	shards  *shards
	backoff *backoff.Backoff
}

func newEndpoint(metrics *metrics, cfg Config, logger log.Logger, markerHandler internal.MarkerHandler) (*endpoint, error) {
	logger = log.With(logger, "component", "endpoint", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, markerHandler, cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &endpoint{
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
		entries: make(chan loki.Entry),
		ctx:     ctx,
		cancel:  cancel,
		shards:  shards,
		backoff: backoff.New(ctx, backoff.Config{
			MinBackoff: 5 * time.Millisecond,
			MaxBackoff: 50 * time.Millisecond,
		}),
	}

	c.shards.start(cfg.QueueConfig.MinShards)
	return c, nil
}

var errQueueIsFull = errors.New("queue is full")

// enqueue tries to enqueue an entry. It returns an error if the entry could not be enqueued.
// errQueueIsFull when the queue is full and BlockOnOverflow is false, or context.Canceled when
// endpoint is stopped.
func (e *endpoint) enqueue(entry loki.Entry, segmentNum int) error {
	defer e.backoff.Reset()

	tenantID := getTenantID(e.cfg, entry)
	for !e.shards.enqueue(tenantID, entry, segmentNum) {
		if !e.cfg.QueueConfig.BlockOnOverflow {
			level.Warn(e.logger).Log("msg", "dropping entry", "err", errQueueIsFull)
			e.metrics.droppedEntries.WithLabelValues(e.cfg.URL.Host, tenantID, reasonQueueIsFull).Inc()
			e.metrics.droppedBytes.WithLabelValues(e.cfg.URL.Host, tenantID, reasonQueueIsFull).Add(float64(entry.Size()))
			return errQueueIsFull
		}

		e.backoff.Wait()
		if !e.backoff.Ongoing() {
			return e.backoff.Err()
		}
	}

	return nil
}

func (e *endpoint) stop() {
	e.cancel()
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
