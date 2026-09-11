package client

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal/marker"
)

func NewFanoutConsumer(logger *slog.Logger, reg prometheus.Registerer, cfgs ...Config) (*FanoutConsumer, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("at least one endpoint config must be provided")
	}

	c := &FanoutConsumer{
		endpoints: make([]*endpoint, 0, len(cfgs)),
		recv:      make(chan loki.Entry),
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())

	var (
		metrics        = newMetrics(reg)
		endpointsCheck = make(map[string]struct{})
	)

	for _, cfg := range cfgs {
		// Don't allow duplicate endpoints, we have endpoint specific metrics that need at least one unique label value (name).
		name := getEndpointName(cfg)
		if _, ok := endpointsCheck[name]; ok {
			return nil, fmt.Errorf("duplicate endpoint configs are not allowed, found duplicate for name: %s", cfg.Name)
		}

		endpointsCheck[name] = struct{}{}
		endpoint, err := newEndpoint(metrics, cfg, logger, marker.NewNopTracker())
		if err != nil {
			return nil, fmt.Errorf("error starting endpoint: %w", err)
		}

		c.endpoints = append(c.endpoints, endpoint)
	}

	c.wg.Go(c.run)
	return c, nil
}

var _ Consumer = (*FanoutConsumer)(nil)

type FanoutConsumer struct {
	endpoints []*endpoint

	wg     sync.WaitGroup
	once   sync.Once
	recv   chan loki.Entry
	ctx    context.Context
	cancel context.CancelFunc
}

func (c *FanoutConsumer) run() {
	for e := range c.recv {
		for _, endpoint := range c.endpoints {
			// NOTE: For now it's fine to ignore error because we can't act on it.
			_ = endpoint.enqueue(c.ctx, e, 0)
		}
	}
}

func (c *FanoutConsumer) Chan() chan<- loki.Entry {
	return c.recv
}

func (c *FanoutConsumer) Stop() {
	// First stop the receiving channel.
	c.once.Do(func() {
		close(c.recv)
		c.cancel()
	})

	c.wg.Wait()

	var stopWG sync.WaitGroup
	// Stop all endpoints.
	for _, c := range c.endpoints {
		stopWG.Go(func() {
			c.stop()
		})
	}

	// Wait for all endpoints to stop.
	stopWG.Wait()
}
