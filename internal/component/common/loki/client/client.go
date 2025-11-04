package client

import (
	"context"
	"sync"
	"time"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
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
}

// Client for pushing logs in snappy-compressed protos over HTTP.
type client struct {
	cfg     Config
	entries chan loki.Entry

	wg sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc

	shards *shards
}

func New(metrics *Metrics, cfg Config, logger log.Logger) (Client, error) {
	return newClient(metrics, cfg, logger)
}

func newClient(metrics *Metrics, cfg Config, logger log.Logger) (*client, error) {
	logger = log.With(logger, "component", "client", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, internal.NewNoopMarkerHandler(), cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &client{
		cfg:     cfg,
		entries: make(chan loki.Entry),
		shards:  shards,
		ctx:     ctx,
		cancel:  cancel,
	}

	c.wg.Go(func() { c.run() })
	return c, nil
}

func (c *client) run() {
	c.shards.start(1)

	for {
		select {
		case <-c.ctx.Done():
			return
		case e := <-c.entries:

			backoff := backoff.New(c.ctx, backoff.Config{
				MinBackoff: 5 * time.Millisecond,
				MaxBackoff: 50 * time.Millisecond,
			})
			for {
				if c.shards.enqueue(e, 0) {
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

func (c *client) Stop() {
	c.shards.stop()
	c.cancel()
	c.wg.Wait()
}
