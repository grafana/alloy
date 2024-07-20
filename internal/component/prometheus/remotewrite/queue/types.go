package queue

import (
	"time"

	"github.com/prometheus/prometheus/storage"
)

func defaultArgs() Arguments {
	return Arguments{
		TTL:            2 * time.Hour,
		Evict:          1 * time.Hour,
		BatchSizeBytes: 32 * 1024 * 1024,
		FlushTime:      5 * time.Second,
		Connection: ConnectionConfig{
			Timeout:                 15 * time.Second,
			RetryBackoff:            1 * time.Second,
			MaxRetryBackoffAttempts: 0,
			BatchCount:              1_000,
			FlushDuration:           1 * time.Second,
			QueueCount:              4,
		},
	}
}

type Arguments struct {
	TTL            time.Duration     `alloy:"ttl,attr,optional"`
	Evict          time.Duration     `alloy:"evict_interval,attr,optional"`
	BatchSizeBytes int               `alloy:"batch_size_bytes,attr,optional"`
	ExternalLabels map[string]string `alloy:"external_labels,attr,optional"`
	FlushTime      time.Duration     `alloy:"flush_time,attr,optional"`
	Connection     ConnectionConfig  `alloy:"endpoint,block"`
}

type ConnectionConfig struct {
	URL                     string        `alloy:"url,attr"`
	BasicAuth               BasicAuth     `alloy:"basic_auth,block,optional"`
	Timeout                 time.Duration `alloy:"write_timeout,attr,optional"`
	RetryBackoff            time.Duration `alloy:"retry_backoff,attr,optional"`
	MaxRetryBackoffAttempts time.Duration `alloy:"max_retry_backoff,attr,optional"`
	BatchCount              int           `alloy:"batch_count,attr,optional"`
	FlushDuration           time.Duration `alloy:"flush_duration,attr,optional"`
	QueueCount              uint          `alloy:"queue_count,attr,optional"`
}

type BasicAuth struct {
	Username string `alloy:"username,attr,optional"`
	Password string `alloy:"password,attr,optional"`
}

type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

// SetToDefault sets the default
func (rc *Arguments) SetToDefault() {
	*rc = defaultArgs()
}

func (r *Arguments) Validate() error {
	return nil
}
