package queue

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/prometheus/prometheus/storage"
)

func defaultArgs() Arguments {
	return Arguments{
		TTL:               2 * time.Hour,
		MaxSignalsToBatch: 10_000,
		BatchFrequency:    5 * time.Second,
	}
}

type Arguments struct {
	// TTL is how old a series can be.
	TTL time.Duration `alloy:"ttl,attr,optional"`
	// The batch size to persist to the file queue.
	MaxSignalsToBatch int `alloy:"max_signals_to_batch,attr,optional"`
	// How often to flush to the file queue if BatchSize isn't met.
	// TODO @mattdurham this may need to go into a specific block for the serializer.
	BatchFrequency time.Duration      `alloy:"batch_frequency,attr,optional"`
	Connections    []ConnectionConfig `alloy:"endpoint,block"`
}

type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

// SetToDefault sets the default
func (rc *Arguments) SetToDefault() {
	*rc = defaultArgs()
}

func defaultCC() ConnectionConfig {
	return ConnectionConfig{
		Timeout:                 30 * time.Second,
		RetryBackoff:            1 * time.Second,
		MaxRetryBackoffAttempts: 0,
		BatchCount:              1_000,
		FlushFrequency:          1 * time.Second,
		QueueCount:              4,
	}
}

func (cc *ConnectionConfig) SetToDefault() {
	*cc = defaultCC()
}

func (r *Arguments) Validate() error {
	for _, conn := range r.Connections {
		if conn.BatchCount <= 0 {
			return fmt.Errorf("batch_count must be greater than 0")
		}
		if conn.FlushFrequency < 1*time.Second {
			return fmt.Errorf("flush_frequency must be greater or equal to 1s, the internal timers resolution is 1s")
		}
	}

	return nil
}

// ConnectionConfig is the alloy specific version of ConnectionConfig. This looks odd, the idea
//
//	is that once this code is tested that the bulk of the underlying code will be used elsewhere.
//	this means we need a very generic interface for that code, and a specific alloy implementation here.
type ConnectionConfig struct {
	Name      string        `alloy:",label"`
	URL       string        `alloy:"url,attr"`
	BasicAuth *BasicAuth    `alloy:"basic_auth,block,optional"`
	Timeout   time.Duration `alloy:"write_timeout,attr,optional"`
	// How long to wait between retries.
	RetryBackoff time.Duration `alloy:"retry_backoff,attr,optional"`
	// Maximum number of retries.
	MaxRetryBackoffAttempts uint `alloy:"max_retry_backoff_attempts,attr,optional"`
	// How many series to write at a time.
	BatchCount int `alloy:"batch_count,attr,optional"`
	// How long to wait before sending regardless of batch count.
	FlushFrequency time.Duration `alloy:"flush_frequency,attr,optional"`
	// How many concurrent queues to have.
	QueueCount uint `alloy:"queue_count,attr,optional"`

	ExternalLabels map[string]string `alloy:"external_labels,attr,optional"`
}

func (cc ConnectionConfig) ToNativeType() types.ConnectionConfig {
	tcc := types.ConnectionConfig{
		URL: cc.URL,
		// TODO @mattdurham generate this with build information.
		UserAgent:               "alloy",
		Timeout:                 cc.Timeout,
		RetryBackoff:            cc.RetryBackoff,
		MaxRetryBackoffAttempts: cc.MaxRetryBackoffAttempts,
		BatchCount:              cc.BatchCount,
		FlushFrequency:          cc.FlushFrequency,
		ExternalLabels:          cc.ExternalLabels,
		Connections:             cc.QueueCount,
	}
	if cc.BasicAuth != nil {
		tcc.BasicAuth = &types.BasicAuth{
			Username: cc.BasicAuth.Username,
			Password: string(cc.BasicAuth.Password),
		}
	}
	return tcc
}

type BasicAuth struct {
	Username string            `alloy:"username,attr,optional"`
	Password alloytypes.Secret `alloy:"password,attr,optional"`
}
