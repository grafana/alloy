package client

import (
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/common/config"
)

// Config describes configuration for an HTTP pusher client.
type Config struct {
	Name      string
	URL       flagext.URLValue
	BatchWait time.Duration
	BatchSize int

	Client  config.HTTPClientConfig
	Headers map[string]string

	BackoffConfig backoff.Config
	Timeout       time.Duration

	// The tenant ID to use when pushing logs to Loki (empty string means
	// single tenant mode)
	TenantID string

	// Max number of streams that can be added to a batch.
	MaxStreams int

	// When enabled, Promtail will not retry batches that get a
	// 429 'Too Many Requests' response from the distributor. Helps
	// prevent HOL blocking in multitenant deployments.
	DropRateLimitedBatches bool

	// QueueConfig controls how shards and queues are configured for endpoints.
	QueueConfig QueueConfig
}

// QueueConfig controls how shards and queue are configured for client.
type QueueConfig struct {
	// Capacity is the worst case size in bytes desired for the send queue. This value is used to calculate the size of
	// the buffered channel used underneath. The worst case scenario assumed is that every batch buffered in full, hence
	// the channel capacity would be calculated as: bufferChannelSize = Capacity / BatchSize.
	//
	// For example, assuming BatchSize
	// is the 1 MiB default, and a capacity of 100 MiB, the underlying buffered channel would buffer up to 100 batches.
	Capacity int

	// MinShards is the minimum number of concurrent shards sending batches to the endpoint.
	MinShards int

	// DrainTimeout controls the maximum time that draining the queue can take.
	DrainTimeout time.Duration
}
