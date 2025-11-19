package client

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/dskit/backoff"
)

func NewFanoutConsumer(logger log.Logger, reg prometheus.Registerer, clientCfgs ...Config) (*FanoutConsumer, error) {
	if len(clientCfgs) == 0 {
		return nil, fmt.Errorf("at least one client config must be provided")
	}

	m := &FanoutConsumer{
		clients: make([]*client, 0, len(clientCfgs)),
		recv:    make(chan loki.Entry),
	}

	var (
		metrics      = NewMetrics(reg)
		clientsCheck = make(map[string]struct{})
	)

	for _, cfg := range clientCfgs {
		// Don't allow duplicate clients, we have client specific metrics that need at least one unique label value (name).
		clientName := getClientName(cfg)
		if _, ok := clientsCheck[clientName]; ok {
			return nil, fmt.Errorf("duplicate client configs are not allowed, found duplicate for name: %s", cfg.Name)
		}

		clientsCheck[clientName] = struct{}{}
		client, err := newClient(metrics, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("error starting client: %w", err)
		}

		m.clients = append(m.clients, client)
	}

	m.wg.Go(m.run)
	return m, nil
}

var _ Consumer = (*FanoutConsumer)(nil)

type FanoutConsumer struct {
	clients []*client
	wg      sync.WaitGroup
	once    sync.Once
	recv    chan loki.Entry
}

func (c *FanoutConsumer) run() {
	for e := range c.recv {
		for _, c := range c.clients {
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
	// Stop all clients.
	for _, c := range c.clients {
		stopWG.Go(func() {
			c.Stop()
		})
	}

	// Wait for all clients to stop.
	stopWG.Wait()
}

// getClientName computes the specific name for each client config. The name is either the configured Name setting in Config,
// or a hash of the config as whole, this allows us to detect repeated configs.
func getClientName(cfg Config) string {
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

// Client for pushing logs in snappy-compressed protos over HTTP.
type client struct {
	cfg     Config
	entries chan loki.Entry

	wg sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc

	shards *shards
}

func newClient(metrics *Metrics, cfg Config, logger log.Logger) (*client, error) {
	logger = log.With(logger, "component", "client", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, internal.NewNopMarkerHandler(), cfg)
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

	c.shards.start(cfg.Queue.MinShards)

	c.wg.Go(func() { c.run() })
	return c, nil
}

func (c *client) run() {
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

func (c *client) Chan() chan<- loki.Entry {
	return c.entries
}

func (c *client) Stop() {
	c.shards.stop()
	c.cancel()
	c.wg.Wait()
}
