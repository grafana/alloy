package client

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/grafana/dskit/backoff"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal/marker"
)

type endpoint struct {
	cfg     Config
	metrics *metrics
	logger  *slog.Logger
	entries chan loki.Entry

	ctx    context.Context
	cancel context.CancelFunc

	shards  *shards
	backoff *backoff.Backoff

	// drainDeadline is when the shutdown drain budget expires, or nil while not draining.
	drainDeadline atomic.Pointer[time.Time]
}

func newEndpoint(metrics *metrics, cfg Config, logger *slog.Logger, markerHandler marker.Tracker) (*endpoint, error) {
	logger = logger.With("component", "endpoint", "host", cfg.URL.Host)

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
// errQueueIsFull when the queue is full and either BlockOnOverflow is false or the drain budget
// is spent, or context.Canceled when endpoint is stopped.
func (e *endpoint) enqueue(entry loki.Entry, segmentNum int) error {
	defer e.backoff.Reset()

	tenantID := getTenantID(e.cfg, entry)
	for !e.shards.enqueue(tenantID, entry, segmentNum) {
		if !e.cfg.QueueConfig.BlockOnOverflow || e.drainExpired() {
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

// stopAccepting gives enqueue DrainTimeout to place whatever it is still holding, after which it
// drops on a full queue rather than blocking. Callers must run this before waiting on whichever
// goroutine feeds enqueue, otherwise that wait deadlocks against the blocked enqueue.
func (e *endpoint) stopAccepting() {
	deadline := time.Now().Add(e.cfg.QueueConfig.DrainTimeout)
	// Keep the deadline set by the first caller; shutdown paths call this more than once.
	e.drainDeadline.CompareAndSwap(nil, &deadline)
}

func (e *endpoint) drainExpired() bool {
	deadline := e.drainDeadline.Load()
	return deadline != nil && !time.Now().Before(*deadline)
}

func (e *endpoint) stop() {
	e.stopAccepting()
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
