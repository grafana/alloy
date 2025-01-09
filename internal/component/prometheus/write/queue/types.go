package queue

import (
	"fmt"
	common "github.com/prometheus/common/config"
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/walqueue/types"
	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/storage"
)

func defaultArgs() Arguments {
	return Arguments{
		TTL: 2 * time.Hour,
		Persistence: Persistence{
			MaxSignalsToBatch: 10_000,
			BatchInterval:     5 * time.Second,
		},
	}
}

type Arguments struct {
	// TTL is how old a series can be.
	TTL         time.Duration    `alloy:"ttl,attr,optional"`
	Persistence Persistence      `alloy:"persistence,block,optional"`
	Endpoints   []EndpointConfig `alloy:"endpoint,block"`
}

type Persistence struct {
	// The batch size to persist to the file queue.
	MaxSignalsToBatch int `alloy:"max_signals_to_batch,attr,optional"`
	// How often to flush to the file queue if BatchSize isn't met.
	BatchInterval time.Duration `alloy:"batch_interval,attr,optional"`
}

type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

// SetToDefault sets the default
func (rc *Arguments) SetToDefault() {
	*rc = defaultArgs()
}

func defaultEndpointConfig() EndpointConfig {
	return EndpointConfig{
		Timeout:          30 * time.Second,
		RetryBackoff:     1 * time.Second,
		MaxRetryAttempts: 0,
		BatchCount:       1_000,
		FlushInterval:    1 * time.Second,
		Parallelism:      4,
	}
}

func (cc *EndpointConfig) SetToDefault() {
	*cc = defaultEndpointConfig()
}

func (r *Arguments) Validate() error {
	for _, conn := range r.Endpoints {
		if conn.BatchCount <= 0 {
			return fmt.Errorf("batch_count must be greater than 0")
		}
		if conn.FlushInterval < 1*time.Second {
			return fmt.Errorf("flush_interval must be greater or equal to 1s, the internal timers resolution is 1s")
		}
	}

	return nil
}

// EndpointConfig is the alloy specific version of ConnectionConfig.
type EndpointConfig struct {
	Name        string            `alloy:",label"`
	URL         string            `alloy:"url,attr"`
	BasicAuth   *BasicAuth        `alloy:"basic_auth,block,optional"`
	BearerToken alloytypes.Secret `alloy:"bearer_token,attr,optional"`
	Timeout     time.Duration     `alloy:"write_timeout,attr,optional"`
	// How long to wait between retries.
	RetryBackoff time.Duration `alloy:"retry_backoff,attr,optional"`
	// Maximum number of retries.
	MaxRetryAttempts uint `alloy:"max_retry_attempts,attr,optional"`
	// How many series to write at a time.
	BatchCount int `alloy:"batch_count,attr,optional"`
	// How long to wait before sending regardless of batch count.
	FlushInterval time.Duration `alloy:"flush_interval,attr,optional"`
	// How many concurrent queues to have.
	Parallelism    uint              `alloy:"parallelism,attr,optional"`
	ExternalLabels map[string]string `alloy:"external_labels,attr,optional"`
	TLSConfig      *common.TLSConfig `alloy:"tls_config,block,optional"`
}

var UserAgent = fmt.Sprintf("Alloy/%s", version.Version)

func (cc EndpointConfig) ToNativeType() types.ConnectionConfig {
	tcc := types.ConnectionConfig{
		URL:              cc.URL,
		BearerToken:      string(cc.BearerToken),
		UserAgent:        UserAgent,
		Timeout:          cc.Timeout,
		RetryBackoff:     cc.RetryBackoff,
		MaxRetryAttempts: cc.MaxRetryAttempts,
		BatchCount:       cc.BatchCount,
		FlushInterval:    cc.FlushInterval,
		ExternalLabels:   cc.ExternalLabels,
		Connections:      cc.Parallelism,
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
