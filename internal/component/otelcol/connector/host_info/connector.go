package host_info

import (
	"context"
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const (
	hostInfoMetric     = "traces_host_info"
	hostIdentifierAttr = "grafana.host.id"
)

var _ connector.Traces = (*connectorImp)(nil)

type connectorImp struct {
	config Config
	logger *zap.Logger

	started      bool
	done         chan struct{}
	shutdownOnce sync.Once

	metricsConsumer consumer.Metrics
	hostMetrics     *hostMetrics
}

func newConnector(logger *zap.Logger, config component.Config) *connectorImp {
	logger.Info("Building host_info connector")
	cfg := config.(*Config)
	return &connectorImp{
		config:      *cfg,
		logger:      logger,
		done:        make(chan struct{}),
		hostMetrics: newHostMetrics(),
	}
}

// Capabilities implements connector.Traces.
func (c *connectorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements connector.Traces.
func (c *connectorImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			attrs := resourceSpan.Resource().Attributes()
			mapping := attrs.AsRaw()

			for key, val := range mapping {
				if slices.Contains(c.config.HostIdentifiers, key) {
					c.hostMetrics.add(val.(string))
				}
			}
		}
	}
	return nil
}

// Start implements connector.Traces.
func (c *connectorImp) Start(ctx context.Context, host component.Host) error {
	c.logger.Info("Starting host_info connector")
	c.started = true
	ticker := time.NewTicker(c.config.MetricsFlushInterval)
	go func() {
		for {
			select {
			case <-c.done:
				ticker.Stop()
				return
			case <-ticker.C:
				if err := c.flush(ctx); err != nil {
					c.logger.Error("Error consuming metrics", zap.Error(err))
				}
			}
		}
	}()
	return nil
}

// Shutdown implements connector.Traces.
func (c *connectorImp) Shutdown(ctx context.Context) error {
	c.shutdownOnce.Do(func() {
		c.logger.Info("Stopping host_info connector")
		if c.started {
			// flush metrics on shutdown
			if err := c.flush(ctx); err != nil {
				c.logger.Error("Error consuming metrics", zap.Error(err))
			}
			c.done <- struct{}{}
			c.started = false
		}
	})
	return nil
}

func (c *connectorImp) flush(ctx context.Context) error {
	var err error

	metrics, count := c.hostMetrics.metrics()
	if count > 0 {
		c.hostMetrics.reset()
		c.logger.Debug("Flushing metrics", zap.Int("count", count))
		err = c.metricsConsumer.ConsumeMetrics(ctx, *metrics)
	}
	return err
}
