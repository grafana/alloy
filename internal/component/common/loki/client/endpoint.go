package client

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
	"github.com/grafana/dskit/backoff"
)

type endpoint struct {
	cfg     Config
	entries chan loki.Entry

	ctx    context.Context
	cancel context.CancelFunc

	shards *shards
}

func newEndpoint(metrics *Metrics, cfg Config, logger log.Logger, markerHandler internal.MarkerHandler) (*endpoint, error) {
	logger = log.With(logger, "component", "endpoint", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, markerHandler, cfg)
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

	c.shards.start(cfg.QueueConfig.MinShards)
	return c, nil
}

// enqueue will try to enqueue entry. If endpoint is stopped any active attempts will
// be stopped and false will be returned.
func (c *endpoint) enqueue(entry loki.Entry, segmentNum int) bool {
	backoff := backoff.New(c.ctx, backoff.Config{
		MinBackoff: 5 * time.Millisecond,
		MaxBackoff: 50 * time.Millisecond,
	})
	for !c.shards.enqueue(entry, segmentNum) {
		backoff.Wait()
		if !backoff.Ongoing() {
			return false
		}
	}
	return true
}

func (c *endpoint) Stop() {
	c.shards.stop()
	c.cancel()
}

// getEndpointName computes the specific name for each endpoint config. The name is either the configured Name setting in Config,
// or a hash of the config as whole, this allows us to detect repeated configs.
func getEndpointName(cfg Config) string {
	if cfg.Name != "" {
		return cfg.Name
	}

	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%v", cfg)
	return fmt.Sprintf("%x", h.Sum(nil))[:6]
}
