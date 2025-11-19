package write

import (
	"fmt"
	"net/url"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki/client"

	"github.com/alecthomas/units"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"

	types "github.com/grafana/alloy/internal/component/common/config"
)

// EndpointOptions describes an individual location to send logs to.
type EndpointOptions struct {
	Name              string                  `alloy:"name,attr,optional"`
	URL               string                  `alloy:"url,attr"`
	BatchWait         time.Duration           `alloy:"batch_wait,attr,optional"`
	BatchSize         units.Base2Bytes        `alloy:"batch_size,attr,optional"`
	RemoteTimeout     time.Duration           `alloy:"remote_timeout,attr,optional"`
	Headers           map[string]string       `alloy:"headers,attr,optional"`
	MinBackoff        time.Duration           `alloy:"min_backoff_period,attr,optional"`  // start backoff at this level
	MaxBackoff        time.Duration           `alloy:"max_backoff_period,attr,optional"`  // increase exponentially to this level
	MaxBackoffRetries int                     `alloy:"max_backoff_retries,attr,optional"` // give up after this many; zero means infinite retries
	TenantID          string                  `alloy:"tenant_id,attr,optional"`
	RetryOnHTTP429    bool                    `alloy:"retry_on_http_429,attr,optional"`
	HTTPClientConfig  *types.HTTPClientConfig `alloy:",squash"`
	QueueConfig       QueueConfig             `alloy:"queue_config,block,optional"`
}

// GetDefaultEndpointOptions defines the default settings for sending logs to a
// remote endpoint.
// The backoff schedule with the default parameters:
// 0.5s, 1s, 2s, 4s, 8s, 16s, 32s, 64s, 128s, 256s(4.267m)
// For a total time of 511.5s (8.5m) before logs are lost.
func GetDefaultEndpointOptions() EndpointOptions {
	var defaultEndpointOptions = EndpointOptions{
		BatchWait:         1 * time.Second,
		BatchSize:         1 * units.MiB,
		RemoteTimeout:     10 * time.Second,
		MinBackoff:        500 * time.Millisecond,
		MaxBackoff:        5 * time.Minute,
		MaxBackoffRetries: 10,
		HTTPClientConfig:  types.CloneDefaultHTTPClientConfig(),
		RetryOnHTTP429:    true,
	}

	return defaultEndpointOptions
}

// SetToDefault implements syntax.Defaulter.
func (r *EndpointOptions) SetToDefault() {
	*r = GetDefaultEndpointOptions()
}

// Validate implements syntax.Validator.
func (r *EndpointOptions) Validate() error {
	if _, err := url.Parse(r.URL); err != nil {
		return fmt.Errorf("failed to parse remote url %q: %w", r.URL, err)
	}

	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	if r.HTTPClientConfig != nil {
		return r.HTTPClientConfig.Validate()
	}

	return nil
}

// QueueConfig controls how shards and queue are configured for client.
type QueueConfig struct {
	Capacity     units.Base2Bytes `alloy:"capacity,attr,optional"`
	MinShards    int              `alloy:"min_shards,attr,optional"`
	DrainTimeout time.Duration    `alloy:"drain_timeout,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (q *QueueConfig) SetToDefault() {
	*q = QueueConfig{
		Capacity:     10 * units.MiB, // considering the default BatchSize of 1MiB, this gives us a default buffered channel of size 10
		MinShards:    1,
		DrainTimeout: 15 * time.Second,
	}
}

func (args Arguments) convertClientConfigs() []client.Config {
	var res []client.Config
	for _, cfg := range args.Endpoints {
		url, _ := url.Parse(cfg.URL)
		cc := client.Config{
			Name:      cfg.Name,
			URL:       flagext.URLValue{URL: url},
			Headers:   cfg.Headers,
			BatchWait: cfg.BatchWait,
			BatchSize: int(cfg.BatchSize),
			Client:    *cfg.HTTPClientConfig.Convert(),
			BackoffConfig: backoff.Config{
				MinBackoff: cfg.MinBackoff,
				MaxBackoff: cfg.MaxBackoff,
				MaxRetries: cfg.MaxBackoffRetries,
			},
			Timeout:                cfg.RemoteTimeout,
			TenantID:               cfg.TenantID,
			MaxStreams:             args.MaxStreams,
			DropRateLimitedBatches: !cfg.RetryOnHTTP429,
			Queue: client.QueueConfig{
				Capacity:     int(cfg.QueueConfig.Capacity),
				MinShards:    cfg.QueueConfig.MinShards,
				DrainTimeout: cfg.QueueConfig.DrainTimeout,
			},
		}
		res = append(res, cc)
	}

	return res
}
