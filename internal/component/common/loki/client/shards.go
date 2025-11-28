package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/common/config"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/useragent"
)

const (
	// Label reserved to override the tenant ID while processing
	// pipeline stages
	ReservedLabelTenantID = "__tenant_id__"
)

// queuedBatch is a batch specific to a tenant, that is considered ready to be sent.
type queuedBatch struct {
	TenantID string
	Batch    *batch
}

func newQueue(metrics *metrics, logger log.Logger, cfg Config) *queue {
	// Capacity is the worst case size in bytes desired for the send queue. This value is used to calculate the size of
	// the buffered channel. The worst case scenario assumed is that every batch buffered in full, hence
	// the channel capacity would be calculated as: bufferChannelSize = Capacity / BatchSize.
	// For example, assuming BatchSize is the 1 MiB default and Capacity is 100 MiB,
	// the underlying buffered channel would buffer up to 100 batches.
	capacity := max(cfg.QueueConfig.Capacity/max(cfg.BatchSize, 1), 1)

	return &queue{
		cfg:     cfg,
		metrics: metrics,
		logger:  logger,

		batches: make(map[string]*batch),
		c:       make(chan queuedBatch, capacity),
	}
}

// queue for batching and sending log entries to Loki.
// The queue maintains separate batches per tenant and enqueues batches when they
// reach the configured batch size limit.
type queue struct {
	cfg     Config
	metrics *metrics
	logger  log.Logger
	c       chan queuedBatch

	mu sync.Mutex
	// batches maintains one active batch per tenant. When a batch reaches
	// the size limit, it's moved to the channel and a new batch is created
	// for that tenant.
	batches map[string]*batch
}

// append adds a log entry to the queue for the given tenant.
// It returns true if the entry was successfully queued, false if the queue
// is full and backpressure should be applied.
func (q *queue) append(tenantID string, entry loki.Entry, segmentNum int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	batch, ok := q.batches[tenantID]
	if !ok {
		// Create a new batch for this tenant.
		batch := newBatch(q.cfg.MaxStreams)
		_ = batch.add(entry, segmentNum)
		q.batches[tenantID] = batch
		return true
	}

	// If adding this entry would exceed the batch size limit, enqueue the
	// current batch and start a new one.
	if batch.sizeBytesAfter(entry.Entry) > q.cfg.BatchSize {
		select {
		case q.c <- queuedBatch{Batch: batch, TenantID: tenantID}:
			// Successfully enqueued the batch.
		default:
			// Channel is full, signal backpressure.
			return false
		}

		batch := newBatch(q.cfg.MaxStreams)
		_ = batch.add(entry, segmentNum)
		q.batches[tenantID] = batch
		return true
	}

	// Add entry to existing batch. If we cannot add entry to batch we will drop it.
	if err := batch.add(entry, segmentNum); err != nil {
		level.Error(q.logger).Log("msg", "batch add err", "tenant", tenantID, "error", err)
		reason := reasonGeneric
		if errors.Is(err, errMaxStreamsLimitExceeded) {
			reason = reasonStreamLimited
		}
		q.metrics.droppedBytes.WithLabelValues(q.cfg.URL.Host, tenantID, reason).Add(float64(len(entry.Line)))
		q.metrics.droppedEntries.WithLabelValues(q.cfg.URL.Host, tenantID, reason).Inc()
	}

	return true
}

// channel returns the channel used to receive batches ready to be sent.
func (q *queue) channel() chan queuedBatch {
	return q.c
}

// drain retrieves all batches that are ready to be sent.
// It returns all batches currently in the channel and all batches
// from the batches map that have exceeded BatchWait.
func (q *queue) drain() []queuedBatch {
	q.mu.Lock()
	defer q.mu.Unlock()

	var batches []queuedBatch

	// First drain all batches in queue.
loop:
	for {
		select {
		case b, ok := <-q.c:
			if !ok {
				break loop
			}
			batches = append(batches, b)
		default:
			break loop
		}
	}

	// Then check batches that are not queued but should be flushed anyway.
	for tenantID, batch := range q.batches {
		if batch.age() < q.cfg.BatchWait {
			continue
		}

		// Batch has exceeded wait time, remove from map and return it.
		delete(q.batches, tenantID)
		batches = append(batches, queuedBatch{
			TenantID: tenantID,
			Batch:    batch,
		})
	}

	return batches
}

// flushAndShutdown flushes all remaining batches and closes the channel.
// It will stop early if the done channel is closed.
func (q *queue) flushAndShutdown(done chan struct{}) {
loop:
	for q.tryEnqueueingBatch(done) {
		select {
		case <-done:
			break loop
		case <-time.After(time.Second):
		}
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	q.batches = nil
	close(q.c)
}

// tryEnqueueingBatch tries to send a batch if necessary. If sending needs to
// be retried it will return true.
func (q *queue) tryEnqueueingBatch(done <-chan struct{}) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for tenantID, batch := range q.batches {
		select {
		case q.c <- queuedBatch{Batch: batch, TenantID: tenantID}:
			// Successfully queued a batch. If we have more we should retry this.
			delete(q.batches, tenantID)
			return len(q.batches) > 0
		case <-done:
			// Shutdown timeout reached, stop trying to flush.
			return false
		default:
			// Queue is full so we should try again.
			return true
		}
	}
	return false
}

// newShards creates a new shards instance for parallel processing of log entries.
// It validates the configuration and creates an HTTP client for sending batches to Loki.
func newShards(metrics *metrics, logger log.Logger, markerHandler SentDataMarkerHandler, cfg Config) (*shards, error) {
	if cfg.URL.URL == nil {
		return nil, errors.New("endpoint needs target URL")
	}

	err := cfg.Client.Validate()
	if err != nil {
		return nil, err
	}

	client, err := config.NewClientFromConfig(cfg.Client, useragent.ProductName, config.WithHTTP2Disabled())
	if err != nil {
		return nil, err
	}

	client.Timeout = cfg.Timeout

	return &shards{
		cfg:           cfg,
		logger:        logger,
		metrics:       metrics,
		client:        client,
		markerHandler: markerHandler,
		tenants:       make(map[string]struct{}),
	}, nil
}

// shards manages multiple parallel queues for processing and sending log entries to Loki.
// It uses sharding to distribute entries across multiple worker goroutines based on label fingerprints,
// enabling parallel processing and improved throughput. Each shard has its own queue and worker goroutine.
// Entries are routed to shards using a hash of their label fingerprint.
type shards struct {
	cfg           Config
	logger        log.Logger
	metrics       *metrics
	client        *http.Client
	markerHandler SentDataMarkerHandler

	mut     sync.Mutex
	tenants map[string]struct{}
	queues  []*queue

	// running is used to track the number of running shards.
	running  atomic.Int32
	onceDone sync.Once
	// done is used to signal that all shards have finished.
	done chan struct{}

	// softShutdown is used to signal that no new entries should be accepted.
	softShutdown chan struct{}
	ctx          context.Context
	// cancel is used to cancel the context when a hard shutdown is initiated.
	cancel context.CancelFunc
}

// start initializes n shards and starts worker goroutines for each one.
// Each shard gets its own queue and a dedicated worker that processes batches
// from that queue. The number of shards determines the parallelism level.
func (s *shards) start(n int) {
	n = max(n, 1)

	s.mut.Lock()
	defer s.mut.Unlock()

	queues := make([]*queue, n)

	for i := range n {
		queues[i] = newQueue(s.metrics, s.logger, s.cfg)
	}

	s.queues = queues
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.running.Store(int32(n))
	s.onceDone = sync.Once{}
	s.done = make(chan struct{})
	s.softShutdown = make(chan struct{})

	for i := range n {
		go s.runShard(s.queues[i])
	}
}

// stop tries to perform a graceful shutdown of all shards.
// It first attempts a soft shutdown by signaling that no new entries should be accepted
// and allowing all queues to flush their remaining batches within the drain timeout.
// If the drain timeout is exceeded, it performs a hard shutdown that will drop any remaining batches.
func (s *shards) stop() {
	s.mut.Lock()
	defer s.mut.Unlock()

	// Attempt a soft shutdown, meaning that all shards try to flush their remaining batches.
	close(s.softShutdown)

	for _, q := range s.queues {
		go q.flushAndShutdown(s.done)
	}

	select {
	case <-s.done:
		return
	case <-time.After(s.cfg.QueueConfig.DrainTimeout):
	}

	// Perform hard shutdown
	s.cancel()
	<-s.done
}

// runShard is the worker goroutine that processes batches from a single queue.
func (s *shards) runShard(q *queue) {
	// Given that a shard handles multiple batches (1 per tenant) and each batch
	// can be created at a different point in time, we look for batches whose
	// max wait time has been reached every 10 times per BatchWait, so that the
	// maximum delay we have sending batches is 10% of the max waiting time.
	// We apply a cap of 10ms to the ticker, to avoid too frequent checks in
	// case the BatchWait is very low.
	const minWaitCheckFrequency = 10 * time.Millisecond
	maxWaitCheckFrequency := max(s.cfg.BatchWait/10, minWaitCheckFrequency)

	maxWaitCheck := time.NewTicker(maxWaitCheckFrequency)
	defer func() {
		maxWaitCheck.Stop()

		if s.running.Dec() == 0 {
			s.onceDone.Do(func() { close(s.done) })
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			// Context is closed when hard shutdown is initiated.
			return
		case b, ok := <-q.channel():
			if !ok {
				// Channel is closed, when a graceful shutdown is successful.
				return
			}
			s.sendBatch(b.TenantID, b.Batch)
		case <-maxWaitCheck.C:
			// Drain all batches that have exceeded the max wait time.
			for _, b := range q.drain() {
				s.sendBatch(b.TenantID, b.Batch)
			}
		}
	}
}

// enqueue routes a log entry to the appropriate shard based on its label fingerprint.
// Returns false if we could not enqueue the entry, either because the shard is shutting down or the queue is full.
// It is up to the caller to retry or drop the entry.
func (s *shards) enqueue(entry loki.Entry, segmentNum int) bool {
	s.mut.Lock()
	defer s.mut.Unlock()

	entry, tenantID := s.processEntry(entry)
	if _, ok := s.tenants[tenantID]; !ok {
		s.tenants[tenantID] = struct{}{}
		s.initBatchMetrics(tenantID)
	}

	fingerprint := entry.Labels.FastFingerprint()
	shard := uint64(fingerprint) % uint64(len(s.queues))

	select {
	case <-s.softShutdown:
		return false
	default:
		return s.queues[shard].append(tenantID, entry, segmentNum)
	}
}

func (s *shards) initBatchMetrics(tenantID string) {
	// Initialize counters to 0 so the metrics are exported before the first
	// occurrence of incrementing to avoid missing metrics.
	for _, counter := range s.metrics.countersWithHostTenantReason {
		for _, reason := range reasons {
			counter.WithLabelValues(s.cfg.URL.Host, tenantID, reason).Add(0)
		}
	}

	for _, counter := range s.metrics.countersWithHostTenant {
		counter.WithLabelValues(s.cfg.URL.Host, tenantID).Add(0)
	}
}

func (s *shards) processEntry(e loki.Entry) (loki.Entry, string) {
	// Check if it has been overridden while processing the pipeline stages
	if value, ok := e.Labels[ReservedLabelTenantID]; ok {
		return e, string(value)
	}

	return e, s.cfg.TenantID
}

// sendBatch encodes a batch and sends it to Loki with retry logic.
func (s *shards) sendBatch(tenantID string, batch *batch) {
	defer batch.reportAsSentData(s.markerHandler)
	buf, entriesCount, err := batch.encode()

	if err != nil {
		level.Error(s.logger).Log("msg", "error encoding batch", "error", err)
		return
	}

	bufBytes := float64(len(buf))
	s.metrics.encodedBytes.WithLabelValues(s.cfg.URL.Host, tenantID).Add(bufBytes)

	backoff := backoff.New(s.ctx, s.cfg.BackoffConfig)
	var status int
	for {
		start := time.Now()
		// send uses `timeout` internally, so `context.Background` is good enough.
		status, err = s.send(context.Background(), tenantID, buf)

		s.metrics.requestDuration.WithLabelValues(strconv.Itoa(status), s.cfg.URL.Host, tenantID).Observe(time.Since(start).Seconds())

		// Immediately drop rate limited batches to avoid HOL blocking for other tenants not experiencing throttling
		if s.cfg.DropRateLimitedBatches && batchIsRateLimited(status) {
			level.Warn(s.logger).Log("msg", "dropping batch due to rate limiting applied at ingester")
			s.metrics.droppedBytes.WithLabelValues(s.cfg.URL.Host, tenantID, reasonRateLimited).Add(bufBytes)
			s.metrics.droppedEntries.WithLabelValues(s.cfg.URL.Host, tenantID, reasonRateLimited).Add(float64(entriesCount))
			return
		}

		if err == nil {
			s.metrics.sentBytes.WithLabelValues(s.cfg.URL.Host, tenantID).Add(bufBytes)
			s.metrics.sentEntries.WithLabelValues(s.cfg.URL.Host, tenantID).Add(float64(entriesCount))
			return
		}

		// Only retry 429s, 500s and connection-level errors.
		if status > 0 && !batchIsRateLimited(status) && status/100 != 5 {
			break
		}

		level.Debug(s.logger).Log("msg", "error sending batch, will retry", "status", status, "tenant", tenantID, "error", err)
		s.metrics.batchRetries.WithLabelValues(s.cfg.URL.Host, tenantID).Inc()
		backoff.Wait()

		// Make sure it sends at least once before checking for retry.
		if !backoff.Ongoing() {
			break
		}
	}

	level.Error(s.logger).Log("msg", "final error sending batch, no retries left, dropping data", "status", status, "tenant", tenantID, "error", err)
	// If the reason for the last retry error was rate limiting, count the drops as such, even if the previous errors
	// were for a different reason
	dropReason := reasonGeneric
	if batchIsRateLimited(status) {
		dropReason = reasonRateLimited
	}
	s.metrics.droppedBytes.WithLabelValues(s.cfg.URL.Host, tenantID, dropReason).Add(bufBytes)
	s.metrics.droppedEntries.WithLabelValues(s.cfg.URL.Host, tenantID, dropReason).Add(float64(entriesCount))
}

var userAgent = useragent.Get()

// send performs the HTTP POST request to send a batch to Loki.
func (s *shards) send(ctx context.Context, tenantID string, buf []byte) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.URL.String(), bytes.NewReader(buf))
	if err != nil {
		return -1, err
	}

	const contentType = "application/x-protobuf"
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", userAgent)

	// If the tenant ID is not empty alloy is running in multi-tenant mode, so
	// we should send it to Loki
	if tenantID != "" {
		req.Header.Set("X-Scope-OrgID", tenantID)
	}

	// Add custom headers on request
	if len(s.cfg.Headers) > 0 {
		for k, v := range s.cfg.Headers {
			if req.Header.Get(k) == "" {
				req.Header.Add(k, v)
			} else {
				level.Warn(s.logger).Log("msg", "custom header key already exists, skipping", "key", k)
			}
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return -1, err
	}
	defer util.LogError(s.logger, "closing response body", resp.Body.Close)

	if resp.StatusCode/100 != 2 {
		const maxErrMsgLen = 1024
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("server returned HTTP status %s (%d): %s", resp.Status, resp.StatusCode, line)
	}
	return resp.StatusCode, err
}

func batchIsRateLimited(status int) bool {
	return status == 429
}
