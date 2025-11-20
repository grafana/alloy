package client

import (
	"fmt"
	"sync"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
)

func NewFanoutConsumer(logger log.Logger, reg prometheus.Registerer, cfgs ...Config) (*FanoutConsumer, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("at least one endpoint config must be provided")
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
		endpoint, err := newEndpoint(metrics, cfg, logger, internal.NewNopMarkerHandler())
		if err != nil {
			return nil, fmt.Errorf("error starting endpoint: %w", err)
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
			c.enqueue(e, 0)
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
