package client

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
	"github.com/grafana/alloy/internal/useragent"
)

func NewFanoutConsumer(logger log.Logger, reg prometheus.Registerer, cfgs ...Config) (*FanoutConsumer, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("at least one client config must be provided")
	}

	m := &FanoutConsumer{
		endpoints: make([]*endpoint, 0, len(cfgs)),
		recv:      make(chan loki.Entry),
	}

	var (
		metrics        = NewMetrics(reg)
		endpointsCheck = make(map[string]struct{})
	)

	for _, cfg := range cfgs {
		// Don't allow duplicate endpoints, we have endpoint specific metrics that need at least one unique label value (name).
		name := getEndpointName(cfg)
		if _, ok := endpointsCheck[name]; ok {
			return nil, fmt.Errorf("duplicate endpoint configs are not allowed, found duplicate for name: %s", cfg.Name)
		}

		endpointsCheck[name] = struct{}{}
		endpoint, err := newEndpoint(metrics, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("error starting client: %w", err)
		}

		m.endpoints = append(m.endpoints, endpoint)
	}

	m.wg.Go(m.run)
	return m, nil
}

var _ Consumer = (*FanoutConsumer)(nil)

type FanoutConsumer struct {
	endpoints []*endpoint
	wg        sync.WaitGroup
	once      sync.Once
	recv      chan loki.Entry
}

func (c *FanoutConsumer) run() {
	for e := range c.recv {
		for _, c := range c.endpoints {
			c.Chan() <- e
		}
	}
}

func (c *FanoutConsumer) Chan() chan<- loki.Entry {
	return c.recv
}

func (c *FanoutConsumer) Stop() {
	// First stop the receiving channel.
	c.once.Do(func() { close(c.recv) })
	c.wg.Wait()

	var stopWG sync.WaitGroup
	// Stop all endpoints.
	for _, c := range c.endpoints {
		stopWG.Go(func() {
			c.Stop()
		})
	}

	// Wait for all endpoints to stop.
	stopWG.Wait()
}

// getEndpointName computes the specific name for each endpoint config. The name is either the configured Name setting in Config,
// or a hash of the config as whole, this allows us to detect repeated configs.
func getEndpointName(cfg Config) string {
	if cfg.Name != "" {
		return cfg.Name
	}
	return asSha256(cfg)
}

func asSha256(o any) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%v", o)

	temp := fmt.Sprintf("%x", h.Sum(nil))
	return temp[:6]
}

var userAgent = useragent.Get()

type endpoint struct {
	cfg     Config
	entries chan loki.Entry

	wg sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc

	shards *shards
}

func newEndpoint(metrics *Metrics, cfg Config, logger log.Logger) (*endpoint, error) {
	logger = log.With(logger, "component", "endpoint", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, internal.NewNopMarkerHandler(), cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &endpoint{
		cfg:     cfg,
		entries: make(chan loki.Entry),
		shards:  shards,
		ctx:     ctx,
		cancel:  cancel,
	}

	c.shards.start(cfg.Queue.MinShards)

	c.wg.Go(func() { c.run() })
	return c, nil
}

func (c *endpoint) run() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case e := <-c.entries:
			backoff := backoff.New(c.ctx, backoff.Config{
				MinBackoff: 5 * time.Millisecond,
				MaxBackoff: 50 * time.Millisecond,
			})
			for !c.shards.enqueue(e, 0) {
				if !backoff.Ongoing() {
					break
				}
			}
		}
	}
}

func (c *endpoint) Chan() chan<- loki.Entry {
	return c.entries
}

func (c *endpoint) Stop() {
	c.shards.stop()
	c.cancel()
	c.wg.Wait()
}
