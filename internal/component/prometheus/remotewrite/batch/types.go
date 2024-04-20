package batch

import (
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/prometheus/storage"

	types "github.com/grafana/alloy/internal/component/common/config"
)

// Defaults for config blocks.
var (
	DefaultQueueOptions = QueueOptions{
		Capacity:          10000,
		MaxShards:         50,
		MinShards:         1,
		MaxSamplesPerSend: 2000,
		BatchSendDeadline: 5 * time.Second,
		MinBackoff:        30 * time.Millisecond,
		MaxBackoff:        5 * time.Second,
		RetryOnHTTP429:    true,
	}

	DefaultMetadataOptions = MetadataOptions{
		Send:              true,
		SendInterval:      1 * time.Minute,
		MaxSamplesPerSend: 2000,
	}
)

func defaultArgs() Arguments {
	return Arguments{
		TTL:       2 * time.Hour,
		Evict:     1 * time.Hour,
		BatchSize: 128 * 1024 * 1024,
	}
}

type Arguments struct {
	TTL            time.Duration          `alloy:"ttl,attr,optional"`
	Evict          time.Duration          `alloy:"evict_interval,attr,optional"`
	BatchSize      int64                  `alloy:"batch_size,attr,optional"`
	Endpoints      []EndpointOptions      `alloy:"endpoint,block,optional"`
	WALOptions     remotewrite.WALOptions `alloy:"wal,block,optional"`
	ExternalLabels map[string]string      `alloy:"external_labels,attr,optional"`
}

type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

// SetToDefault sets the default
func (rc *Arguments) SetToDefault() {
	*rc = defaultArgs()
}

// EndpointOptions describes an individual location for where metrics in the WAL
// should be delivered to using the remote_write protocol.
type EndpointOptions struct {
	Name                 string                  `alloy:"name,attr,optional"`
	URL                  string                  `alloy:"url,attr"`
	RemoteTimeout        time.Duration           `alloy:"remote_timeout,attr,optional"`
	Headers              map[string]string       `alloy:"headers,attr,optional"`
	SendExemplars        bool                    `alloy:"send_exemplars,attr,optional"`
	SendNativeHistograms bool                    `alloy:"send_native_histograms,attr,optional"`
	HTTPClientConfig     *types.HTTPClientConfig `alloy:",squash"`
	QueueOptions         QueueOptions            `alloy:"queue_config,block,optional"`
	MetadataOptions      MetadataOptions         `alloy:"metadata_config,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (r *EndpointOptions) SetToDefault() {
	*r = EndpointOptions{
		RemoteTimeout:    30 * time.Second,
		SendExemplars:    true,
		HTTPClientConfig: types.CloneDefaultHTTPClientConfig(),
		QueueOptions:     DefaultQueueOptions,
		MetadataOptions:  DefaultMetadataOptions,
	}
}
func (r *EndpointOptions) Validate() error {
	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	if r.HTTPClientConfig != nil {
		return r.HTTPClientConfig.Validate()
	}
	return nil
}

// QueueOptions handles the low level queue config options for a remote_write
type QueueOptions struct {
	Capacity          int           `alloy:"capacity,attr,optional"`
	MaxShards         int           `alloy:"max_shards,attr,optional"`
	MinShards         int           `alloy:"min_shards,attr,optional"`
	MaxSamplesPerSend int           `alloy:"max_samples_per_send,attr,optional"`
	BatchSendDeadline time.Duration `alloy:"batch_send_deadline,attr,optional"`
	MinBackoff        time.Duration `alloy:"min_backoff,attr,optional"`
	MaxBackoff        time.Duration `alloy:"max_backoff,attr,optional"`
	RetryOnHTTP429    bool          `alloy:"retry_on_http_429,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (r *QueueOptions) SetToDefault() {
	*r = DefaultQueueOptions
}

// MetadataOptions configures how metadata gets sent over the remote_write
// protocol.
type MetadataOptions struct {
	Send              bool          `alloy:"send,attr,optional"`
	SendInterval      time.Duration `alloy:"send_interval,attr,optional"`
	MaxSamplesPerSend int           `alloy:"max_samples_per_send,attr,optional"`
}

// SetToDefault set to defaults.
func (o *MetadataOptions) SetToDefault() {
	*o = DefaultMetadataOptions
}

type maxTimestamp struct {
	mtx   sync.Mutex
	value float64
	prometheus.Gauge
}

func (m *maxTimestamp) Set(value float64) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if value > m.value {
		m.value = value
		m.Gauge.Set(value)
	}
}

func (m *maxTimestamp) Get() float64 {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.value
}

func (m *maxTimestamp) Collect(c chan<- prometheus.Metric) {
	if m.Get() > 0 {
		m.Gauge.Collect(c)
	}
}

type TTLError struct {
	error
}
