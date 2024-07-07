package queue

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite/queue/networkqueue"

	"github.com/prometheus/prometheus/storage"

	types "github.com/grafana/alloy/internal/component/common/config"
)

// Defaults for config blocks.
var (
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
		BatchSize: 32 * 1024 * 1024,
		FlushTime: 30 * time.Second,
	}
}

type Arguments struct {
	TTL            time.Duration          `alloy:"ttl,attr,optional"`
	Evict          time.Duration          `alloy:"evict_interval,attr,optional"`
	BatchSize      int64                  `alloy:"batch_size,attr,optional"`
	Endpoints      []EndpointOptions      `alloy:"endpoint,block,optional"`
	WALOptions     remotewrite.WALOptions `alloy:"wal,block,optional"`
	ExternalLabels map[string]string      `alloy:"external_labels,attr,optional"`
	FlushTime      time.Duration          `alloy:"flush_time,attr,optional"`
}

type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

// SetToDefault sets the default
func (rc *Arguments) SetToDefault() {
	*rc = defaultArgs()
}

func (r *Arguments) Validate() error {
	names := make(map[string]struct{})
	for _, e := range r.Endpoints {
		name := e.UniqueName()
		_, found := names[name]
		if found {
			return fmt.Errorf("non-unique name found %s", name)
		}
		names[name] = struct{}{}
	}
	return nil
}

// EndpointOptions describes an individual location for where metrics in the WAL
// should be delivered to using the remote_write protocol.
type EndpointOptions struct {
	Name                 string                    `alloy:"name,attr,optional"`
	URL                  string                    `alloy:"url,attr"`
	RemoteTimeout        time.Duration             `alloy:"remote_timeout,attr,optional"`
	Headers              map[string]string         `alloy:"headers,attr,optional"`
	SendExemplars        bool                      `alloy:"send_exemplars,attr,optional"`
	SendNativeHistograms bool                      `alloy:"send_native_histograms,attr,optional"`
	HTTPClientConfig     *types.HTTPClientConfig   `alloy:",squash"`
	QueueOptions         networkqueue.QueueOptions `alloy:"queue_config,block,optional"`
	MetadataOptions      MetadataOptions           `alloy:"metadata_config,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (r *EndpointOptions) SetToDefault() {
	*r = EndpointOptions{
		RemoteTimeout:    30 * time.Second,
		SendExemplars:    true,
		HTTPClientConfig: types.CloneDefaultHTTPClientConfig(),
		QueueOptions:     networkqueue.DefaultQueueOptions,
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

func (r *EndpointOptions) UniqueName() string {
	if r.Name != "" {
		return r.Name
	}
	return base64.RawURLEncoding.EncodeToString([]byte(r.URL))
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

type TTLError struct {
	error
}
