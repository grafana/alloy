package client

import (
	"context"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/dskit/backoff"
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
}

// Client for pushing logs in snappy-compressed protos over HTTP.
type client struct {
	metrics *Metrics
	cfg     Config
	entries chan loki.Entry

	once sync.Once
	wg   sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc

	shards *shards
}

// New makes a new Client.
func New(metrics *Metrics, cfg Config, logger log.Logger) (Client, error) {
	return newClient(metrics, cfg, logger)
}

func newClient(metrics *Metrics, cfg Config, logger log.Logger) (*client, error) {
	logger = log.With(logger, "component", "client", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &client{
		cfg:     cfg,
		entries: make(chan loki.Entry),
		metrics: metrics,
		shards:  shards,
		ctx:     ctx,
		cancel:  cancel,
	}

	c.wg.Go(func() { c.run() })
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
	tenants := make(map[string]struct{})
	c.shards.start(1)

	for {
		select {
		case <-c.ctx.Done():
			return
		case e := <-c.entries:

			e, tenantID := c.processEntry(e)

			if _, ok := tenants[tenantID]; ok {
				c.initBatchMetrics(tenantID)
			}

			backoff := backoff.New(c.ctx, backoff.Config{
				MinBackoff: 5 * time.Millisecond,
				MaxBackoff: 50 * time.Millisecond,
			})
			for {
				if c.shards.enqueue(tenantID, e) {
					break
				}

				if !backoff.Ongoing() {
					break
				}
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
	c.shards.stop()
	c.cancel()
	c.wg.Wait()
}

func (c *client) StopNow() {
	c.Stop()
}

func (c *client) processEntry(e loki.Entry) (loki.Entry, string) {
	tenantID := c.getTenantID(e.Labels)
	return e, tenantID
}
