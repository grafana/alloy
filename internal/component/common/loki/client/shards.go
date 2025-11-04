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
	"github.com/prometheus/common/config"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/dskit/backoff"
	lokiutil "github.com/grafana/loki/v3/pkg/util"
)

func newQueue2(metrics *Metrics, logger log.Logger, cfg Config) *queue2 {
	capacity := cfg.Queue.Capacity / cfg.BatchSize
	return &queue2{
		cfg:     cfg,
		metrics: metrics,
		logger:  logger,

		batches: map[string]*batch{},
		c:       make(chan queuedBatch, capacity),
	}
}

type queue2 struct {
	cfg     Config
	metrics *Metrics
	logger  log.Logger
	c       chan queuedBatch

	mu      sync.Mutex
	batches map[string]*batch // we need to have seperate batches per tenant

}

func (q *queue2) Append(tenantID string, entry loki.Entry) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	batch, ok := q.batches[tenantID]
	if !ok {
		q.batches[tenantID] = newBatch(q.cfg.MaxStreams, entry)
		return true
	}

	if batch.sizeBytesAfter(entry.Entry) > q.cfg.BatchSize {
		select {
		case q.c <- queuedBatch{Batch: batch, TenantID: tenantID}:
		default:
			return false
		}
		q.batches[tenantID] = newBatch(q.cfg.MaxStreams, entry)
		return true
	}

	// if we cannot add entry to batch we will drop it.
	if err := batch.add(entry); err != nil {
		level.Error(q.logger).Log("msg", "batch add err", "tenant", tenantID, "error", err)
		reason := ReasonGeneric
		if errors.Is(err, errMaxStreamsLimitExceeded) {
			reason = ReasonStreamLimited
		}
		q.metrics.droppedBytes.WithLabelValues(q.cfg.URL.Host, tenantID, reason).Add(float64(len(entry.Line)))
		q.metrics.droppedEntries.WithLabelValues(q.cfg.URL.Host, tenantID, reason).Inc()
	}

	return true
}

func (q *queue2) Chan() chan queuedBatch {
	return q.c
}

func (q *queue2) Batches() []queuedBatch {
	q.mu.Lock()
	defer q.mu.Unlock()

	var batches []queuedBatch

loop:
	for {
		select {
		case b := <-q.c:
			batches = append(batches, b)
		default:
			for tenantID, batch := range q.batches {
				if batch.age() < time.Duration(q.cfg.BatchWait) {
					continue
				}

				delete(q.batches, tenantID)
				batches = append(batches, queuedBatch{
					TenantID: tenantID,
					Batch:    batch,
				})
			}
			break loop
		}

	}
	return batches
}

func (q *queue2) FlushAndShutdown(done chan struct{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

loop:
	for tenantID, batch := range q.batches {
		select {
		case q.c <- queuedBatch{Batch: batch, TenantID: tenantID}:
		case <-done:
			break loop
		}
	}

	q.batches = nil
	close(q.c)
}

func newShards(metrics *Metrics, logger log.Logger, cfg Config) (*shards, error) {
	if cfg.URL.URL == nil {
		return nil, errors.New("client needs target URL")
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
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
		client:  client,
	}, nil
}

type shards struct {
	cfg     Config
	logger  log.Logger
	metrics *Metrics
	client  *http.Client

	mut    sync.Mutex
	queues []*queue2

	running atomic.Int32
	done    chan struct{}

	softShutdown chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
}

func (s *shards) start(n int) {
	s.mut.Lock()
	defer s.mut.Unlock()

	queues := make([]*queue2, n)

	for i := range n {
		queues[i] = newQueue2(s.metrics, s.logger, s.cfg)
	}

	s.queues = queues
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.running.Store(int32(n))
	s.done = make(chan struct{})
	s.softShutdown = make(chan struct{})

	for i := range n {
		go s.runShard(s.queues[i])
	}
}

func (s *shards) stop() {
	s.mut.Lock()
	defer s.mut.Unlock()

	// attempt a soft showdown, meaning that all shards tries to flush their remaning batches.
	close(s.softShutdown)

	for _, q := range s.queues {
		go q.FlushAndShutdown(s.done)
	}

	select {
	case <-s.done:
		return
	case <-time.After(s.cfg.Queue.DrainTimeout):

	}

	// perform hard shutdown
	s.cancel()
	<-s.done
}

func (s *shards) runShard(q *queue2) {
	// Given the a shart handles multiple batches (1 per tenant) and each batch
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
			close(s.done)
		}
	}()

	select {
	case <-s.ctx.Done():
		return
	case b, ok := <-q.Chan():
		if !ok {
			return
		}
		s.sendBatch(b.TenantID, b.Batch)
	case <-maxWaitCheck.C:
		for _, b := range q.Batches() {
			s.sendBatch(b.TenantID, b.Batch)
		}
	}
}

func (s *shards) enqueue(tenantID string, entry loki.Entry) bool {
	s.mut.Lock()
	defer s.mut.Unlock()

	fingerprint := entry.Labels.FastFingerprint()
	shard := uint64(fingerprint) % uint64(len(s.queues))

	select {
	case <-s.softShutdown:
		return false
	default:
		return s.queues[shard].Append(tenantID, entry)
	}
}

func (s *shards) sendBatch(tenantID string, batch *batch) {
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
			s.metrics.droppedBytes.WithLabelValues(s.cfg.URL.Host, tenantID, ReasonRateLimited).Add(bufBytes)
			s.metrics.droppedEntries.WithLabelValues(s.cfg.URL.Host, tenantID, ReasonRateLimited).Add(float64(entriesCount))
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
	dropReason := ReasonGeneric
	if batchIsRateLimited(status) {
		dropReason = ReasonRateLimited
	}
	s.metrics.droppedBytes.WithLabelValues(s.cfg.URL.Host, tenantID, dropReason).Add(bufBytes)
	s.metrics.droppedEntries.WithLabelValues(s.cfg.URL.Host, tenantID, dropReason).Add(float64(entriesCount))
}

func (s *shards) send(ctx context.Context, tenantID string, buf []byte) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.URL.String(), bytes.NewReader(buf))
	if err != nil {
		return -1, err
	}
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
	defer lokiutil.LogError("closing response body", resp.Body.Close)

	if resp.StatusCode/100 != 2 {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("server returned HTTP status %s (%d): %s", resp.Status, resp.StatusCode, line)
	}
	return resp.StatusCode, err
}
