package write

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
)

// Arguments holds values which are used to configure the loki.write component.
type Arguments struct {
	Endpoints      []EndpointArguments `alloy:"endpoint,block,optional"`
	ExternalLabels map[string]string   `alloy:"external_labels,attr,optional"`
	MaxStreams     int                 `alloy:"max_streams,attr,optional"`
	WAL            WalArguments        `alloy:"wal,block,optional"`
}

func (a *Arguments) Validate() error {
	if len(a.Endpoints) == 0 {
		return errors.New("at least one endpoint must be configured")
	}

	names := make(map[string]struct{}, len(a.Endpoints))
	for _, e := range a.Endpoints {
		if e.Name == "" {
			continue
		}
		if _, ok := names[e.Name]; ok {
			return fmt.Errorf("duplicate endpoint name %q", e.Name)
		}
		names[e.Name] = struct{}{}
	}

	return nil
}

// WalArguments holds the settings for configuring the Write-Ahead Log (WAL) used
// by the underlying remote write client.
type WalArguments struct {
	Enabled          bool          `alloy:"enabled,attr,optional"`
	MaxSegmentAge    time.Duration `alloy:"max_segment_age,attr,optional"`
	MinReadFrequency time.Duration `alloy:"min_read_frequency,attr,optional"`
	MaxReadFrequency time.Duration `alloy:"max_read_frequency,attr,optional"`
	DrainTimeout     time.Duration `alloy:"drain_timeout,attr,optional"`
}

func (wa *WalArguments) Validate() error {
	if wa.MinReadFrequency >= wa.MaxReadFrequency {
		return fmt.Errorf("WAL min read frequency should be lower than max read frequency")
	}
	return nil
}

func (wa *WalArguments) SetToDefault() {
	// todo(thepalbi): Once we are in a good state: replay implemented, and a better cleanup mechanism
	// make WAL enabled the default
	*wa = WalArguments{
		Enabled:          false,
		MaxSegmentAge:    wal.DefaultMaxSegmentAge,
		MinReadFrequency: wal.DefaultWatchConfig.MinReadFrequency,
		MaxReadFrequency: wal.DefaultWatchConfig.MaxReadFrequency,
		DrainTimeout:     wal.DefaultWatchConfig.DrainTimeout,
	}
}

// EndpointArguments describes an individual location to send logs to.
type EndpointArguments struct {
	Name              string                   `alloy:"name,attr,optional"`
	URL               string                   `alloy:"url,attr"`
	BatchWait         time.Duration            `alloy:"batch_wait,attr,optional"`
	BatchSize         units.Base2Bytes         `alloy:"batch_size,attr,optional"`
	RemoteTimeout     time.Duration            `alloy:"remote_timeout,attr,optional"`
	Headers           map[string]string        `alloy:"headers,attr,optional"`
	MinBackoff        time.Duration            `alloy:"min_backoff_period,attr,optional"`  // start backoff at this level
	MaxBackoff        time.Duration            `alloy:"max_backoff_period,attr,optional"`  // increase exponentially to this level
	MaxBackoffRetries int                      `alloy:"max_backoff_retries,attr,optional"` // give up after this many; zero means infinite retries
	TenantID          string                   `alloy:"tenant_id,attr,optional"`
	RetryOnHTTP429    bool                     `alloy:"retry_on_http_429,attr,optional"`
	HTTPClientConfig  *config.HTTPClientConfig `alloy:",squash"`
	QueueConfig       QueueConfigArguments     `alloy:"queue_config,block,optional"`
}

// GetDefaultEndpointArguments defines the default settings for sending logs to a
// remote endpoint.
// The backoff schedule with the default parameters:
// 0.5s, 1s, 2s, 4s, 8s, 16s, 32s, 64s, 128s, 256s(4.267m)
// For a total time of 511.5s (8.5m) before logs are lost.
func GetDefaultEndpointArguments() EndpointArguments {
	var defaultEndpointOptions = EndpointArguments{
		BatchWait:         1 * time.Second,
		BatchSize:         1 * units.MiB,
		RemoteTimeout:     10 * time.Second,
		MinBackoff:        500 * time.Millisecond,
		MaxBackoff:        5 * time.Minute,
		MaxBackoffRetries: 10,
		HTTPClientConfig:  config.CloneDefaultHTTPClientConfig(),
		RetryOnHTTP429:    true,
		QueueConfig:       defaultQueueConfigArguments,
	}

	return defaultEndpointOptions
}

// SetToDefault implements syntax.Defaulter.
func (r *EndpointArguments) SetToDefault() {
	*r = GetDefaultEndpointArguments()
}

// Validate implements syntax.Validator.
func (r *EndpointArguments) Validate() error {
	if _, err := url.Parse(r.URL); err != nil {
		return fmt.Errorf("failed to parse remote url %q: %w", r.URL, err)
	}

	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	if r.HTTPClientConfig != nil {
		return r.HTTPClientConfig.Validate()
	}

	return nil
}

// QueueConfigArguments controls how shards and queue are configured for endpoint.
type QueueConfigArguments struct {
	Capacity        units.Base2Bytes `alloy:"capacity,attr,optional"`
	MinShards       int              `alloy:"min_shards,attr,optional"`
	DrainTimeout    time.Duration    `alloy:"drain_timeout,attr,optional"`
	BlockOnOverflow bool             `alloy:"block_on_overflow,attr,optional"`
}

var defaultQueueConfigArguments = QueueConfigArguments{
	Capacity:        10 * units.MiB, // considering the default BatchSize of 1MiB, this gives us a default buffered channel of size 10
	MinShards:       1,
	DrainTimeout:    15 * time.Second,
	BlockOnOverflow: true,
}

// SetToDefault implements syntax.Defaulter.
func (q *QueueConfigArguments) SetToDefault() {
	*q = defaultQueueConfigArguments
}

func (args Arguments) convertEndpointConfigs() []client.Config {
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
			QueueConfig: client.QueueConfig{
				Capacity:        int(cfg.QueueConfig.Capacity),
				MinShards:       cfg.QueueConfig.MinShards,
				DrainTimeout:    cfg.QueueConfig.DrainTimeout,
				BlockOnOverflow: cfg.QueueConfig.BlockOnOverflow,
			},
		}
		res = append(res, cc)
	}

	return res
}
