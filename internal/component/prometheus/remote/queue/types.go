package queue

import (
	"fmt"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/prometheus/prometheus/storage"
)

func defaultArgs() Arguments {
	return Arguments{
		TTL:               2 * time.Hour,
		MaxFlushSize:      10_000,
		FlushDuration:     5 * time.Second,
		AppenderBatchSize: 1_000,
	}
}

type Arguments struct {
	// TTL is how old a series can be.
	TTL time.Duration `alloy:"ttl,attr,optional"`
	// The batch size to persist to the file queue.
	MaxFlushSize int `alloy:"max_flush_size,attr,optional"`
	// How often to flush to the file queue if BatchSizeBytes isn't met.
	FlushDuration time.Duration            `alloy:"flush_duration,attr,optional"`
	Connections   []types.ConnectionConfig `alloy:"endpoint,block"`
	// AppenderBatchSize determines how often to flush the appender batch size.
	AppenderBatchSize int               `alloy:"appender_batch_size,attr,optional"`
	ExternalLabels    map[string]string `alloy:"external_labels,attr,optional"`
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
		Timeout:                 15 * time.Second,
		RetryBackoff:            1 * time.Second,
		MaxRetryBackoffAttempts: 0,
		BatchCount:              1_000,
		FlushFrequency:          1 * time.Second,
		Connections:             4,
	}
}
func (cc *ConnectionConfig) SetToDefault() {
	*cc = defaultCC()
}

func (r *Arguments) Validate() error {
	if r.AppenderBatchSize == 0 {
		return fmt.Errorf("appender_batch_size must be greater than zero")
	}
	for _, conn := range r.Connections {
		if conn.BatchCount <= 0 {
			return fmt.Errorf("batch_count must be greater than 0")
		}
	}
	return nil
}

type ConnectionConfig struct {
	Name      string        `alloy:",label"`
	URL       string        `alloy:"url,attr"`
	BasicAuth *BasicAuth    `alloy:"basic_auth,block,optional"`
	Timeout   time.Duration `alloy:"write_timeout,attr,optional"`
	// How long to wait between retries.
	RetryBackoff time.Duration `alloy:"retry_backoff,attr,optional"`
	// Maximum number of retries.
	MaxRetryBackoffAttempts uint `alloy:"max_retry_backoff,attr,optional"`
	// How many series to write at a time.
	BatchCount int `alloy:"batch_count,attr,optional"`
	// How long to wait before sending regardless of batch count.
	FlushFrequency time.Duration `alloy:"flush_duration,attr,optional"`
	// How many concurrent queues to have.
	Connections uint `alloy:"queue_count,attr,optional"`

	ExternalLabels map[string]string `alloy:"external_labels,attr,optional"`
}

func (cc ConnectionConfig) ToTypesCC() types.ConnectionConfig {
	tcc := types.ConnectionConfig{
		URL:                     cc.URL,
		UserAgent:               "alloy",
		Timeout:                 cc.Timeout,
		RetryBackoff:            cc.RetryBackoff,
		MaxRetryBackoffAttempts: cc.MaxRetryBackoffAttempts,
		BatchCount:              cc.BatchCount,
		FlushFrequency:          cc.FlushFrequency,
		ExternalLabels:          cc.ExternalLabels,
		Connections:             cc.Connections,
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
