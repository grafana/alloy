package client

import (
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/common/config"
)

// Config describes configuration for an HTTP pusher client.
type Config struct {
	Name      string `yaml:"name,omitempty"`
	URL       flagext.URLValue
	BatchWait time.Duration `yaml:"batchwait"`
	BatchSize int           `yaml:"batchsize"`

	Client  config.HTTPClientConfig `yaml:",inline"`
	Headers map[string]string       `yaml:"headers,omitempty"`

	BackoffConfig backoff.Config `yaml:"backoff_config"`
	Timeout       time.Duration  `yaml:"timeout"`

	// The tenant ID to use when pushing logs to Loki (empty string means
	// single tenant mode)
	TenantID string `yaml:"tenant_id"`

	// When enabled, Promtail will not retry batches that get a
	// 429 'Too Many Requests' response from the distributor. Helps
	// prevent HOL blocking in multitenant deployments.
	DropRateLimitedBatches bool `yaml:"drop_rate_limited_batches"`

	// Queue controls configuration parameters specific to the queue client
	Queue QueueConfig
}

// QueueConfig holds configurations for the queue-based remote-write client.
type QueueConfig struct {
	// Capacity is the worst case size in bytes desired for the send queue. This value is used to calculate the size of
	// the buffered channel used underneath. The worst case scenario assumed is that every batch buffered in full, hence
	// the channel capacity would be calculated as: bufferChannelSize = Capacity / BatchSize.
	//
	// For example, assuming BatchSize
	// is the 1 MiB default, and a capacity of 100 MiB, the underlying buffered channel would buffer up to 100 batches.
	Capacity int

	// DrainTimeout controls the maximum time that draining the send queue can take.
	DrainTimeout time.Duration
}
