package client

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/useragent"
)

const (
	// Label reserved to override the tenant ID while processing
	// pipeline stages
	ReservedLabelTenantID = "__tenant_id__"

	ReasonGeneric       = "ingester_error"
	ReasonRateLimited   = "rate_limited"
	ReasonStreamLimited = "stream_limited"
	ReasonLineTooLong   = "line_too_long"
)

var Reasons = []string{ReasonGeneric, ReasonRateLimited, ReasonStreamLimited, ReasonLineTooLong}

var userAgent = useragent.Get()

// Client pushes entries to Loki and can be stopped
type Client interface {
	loki.EntryHandler
	// Stop goroutine sending batch of entries without retries.
	StopNow()
	Name() string
}

// Client for pushing logs in snappy-compressed protos over HTTP.
type client struct {
	name    string
	metrics *Metrics
	logger  log.Logger
	cfg     Config
	entries chan loki.Entry

	once sync.Once
	wg   sync.WaitGroup

	bc *batchClient

	maxStreams int
}

// New makes a new Client.
func New(metrics *Metrics, cfg Config, maxStreams int, logger log.Logger) (Client, error) {
	return newClient(metrics, cfg, maxStreams, logger)
}

func newClient(metrics *Metrics, cfg Config, maxStreams int, logger log.Logger) (*client, error) {
	logger = log.With(logger, "component", "client", "host", cfg.URL.Host)

	bc, err := newBatchClient(metrics, logger, cfg)
	if err != nil {
		return nil, err
	}

	c := &client{
		logger:     logger,
		bc:         bc,
		cfg:        cfg,
		entries:    make(chan loki.Entry),
		metrics:    metrics,
		name:       GetClientName(cfg),
		maxStreams: maxStreams,
	}
	if cfg.Name != "" {
		c.name = cfg.Name
	}

	c.wg.Add(1)
	go c.run()
	return c, nil
}

func (c *client) initBatchMetrics(tenantID string) {
	// Initialize counters to 0 so the metrics are exported before the first
	// occurrence of incrementing to avoid missing metrics.
	for _, counter := range c.metrics.countersWithHostTenantReason {
		for _, reason := range Reasons {
			counter.WithLabelValues(c.cfg.URL.Host, tenantID, reason).Add(0)
		}
	}

	for _, counter := range c.metrics.countersWithHostTenant {
		counter.WithLabelValues(c.cfg.URL.Host, tenantID).Add(0)
	}
}

func (c *client) run() {
	batches := map[string]*batch{}

	// Given the client handles multiple batches (1 per tenant) and each batch
	// can be created at a different point in time, we look for batches whose
	// max wait time has been reached every 10 times per BatchWait, so that the
	// maximum delay we have sending batches is 10% of the max waiting time.
	// We apply a cap of 10ms to the ticker, to avoid too frequent checks in
	// case the BatchWait is very low.
	minWaitCheckFrequency := 10 * time.Millisecond
	maxWaitCheckFrequency := max(c.cfg.BatchWait/10, minWaitCheckFrequency)

	maxWaitCheck := time.NewTicker(maxWaitCheckFrequency)

	defer func() {
		maxWaitCheck.Stop()
		// Send all pending batches
		for tenantID, batch := range batches {
			c.bc.sendBatch(context.Background(), tenantID, batch)
		}

		c.wg.Done()
	}()

	for {
		select {
		case e, ok := <-c.entries:
			if !ok {
				return
			}

			e, tenantID := c.processEntry(e)
			batch, ok := batches[tenantID]

			// If the batch doesn't exist yet, we create a new one with the entry
			if !ok {
				batches[tenantID] = newBatch(c.maxStreams, e)
				c.initBatchMetrics(tenantID)
				break
			}

			// If adding the entry to the batch will increase the size over the max
			// size allowed, we do send the current batch and then create a new one
			if batch.sizeBytesAfter(e.Entry) > c.cfg.BatchSize {
				c.bc.sendBatch(context.Background(), tenantID, batch)

				batches[tenantID] = newBatch(c.maxStreams, e)
				break
			}

			// The max size of the batch isn't reached, so we can add the entry
			err := batch.add(e)
			if err != nil {
				level.Error(c.logger).Log("msg", "batch add err", "tenant", tenantID, "error", err)
				reason := ReasonGeneric
				if errors.Is(err, errMaxStreamsLimitExceeded) {
					reason = ReasonStreamLimited
				}
				c.metrics.droppedBytes.WithLabelValues(c.cfg.URL.Host, tenantID, reason).Add(float64(len(e.Line)))
				c.metrics.droppedEntries.WithLabelValues(c.cfg.URL.Host, tenantID, reason).Inc()
				return
			}
		case <-maxWaitCheck.C:
			// Send all batches whose max wait time has been reached
			for tenantID, batch := range batches {
				if batch.age() < c.cfg.BatchWait {
					continue
				}

				c.bc.sendBatch(context.Background(), tenantID, batch)
				delete(batches, tenantID)
			}
		}
	}
}

func (c *client) Chan() chan<- loki.Entry {
	return c.entries
}

func (c *client) getTenantID(labels model.LabelSet) string {
	// Check if it has been overridden while processing the pipeline stages
	if value, ok := labels[ReservedLabelTenantID]; ok {
		return string(value)
	}

	// Check if has been specified in the config
	if c.cfg.TenantID != "" {
		return c.cfg.TenantID
	}

	// Defaults to an empty string, which means the X-Scope-OrgID header
	// will not be sent
	return ""
}

// Stop the client.
func (c *client) Stop() {
	c.once.Do(func() { close(c.entries) })
	c.wg.Wait()
}

// StopNow stops the client without retries
func (c *client) StopNow() {
	// stop batch client from retrying requests.
	c.bc.stop()
	c.Stop()
}

func (c *client) processEntry(e loki.Entry) (loki.Entry, string) {
	tenantID := c.getTenantID(e.Labels)
	return e, tenantID
}

func (c *client) Name() string {
	return c.name
}
