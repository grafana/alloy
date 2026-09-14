package client

import (
	"context"
	"errors"
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
	}

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

	return c, nil
}

var _ Consumer = (*FanoutConsumer)(nil)

type FanoutConsumer struct {
	endpoints []*endpoint
}

func (c *FanoutConsumer) ConsumeEntry(ctx context.Context, entry loki.Entry) error {
	for _, e := range c.endpoints {
		err := e.enqueue(ctx, entry, 0)

		if err != nil {
			// If we get errQueueIsFull we skipped the entry for this endpoints
			// and should try next.
			if errors.Is(err, errQueueIsFull) {
				continue
			}
			// The other errors we can get are the context error and loki.ErrConsumerStopped.
			// In both these cases there is no point trying to enqueue for other endpoints.
			return err
		}
	}
	return nil
}

func (c *FanoutConsumer) Stop() {
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
