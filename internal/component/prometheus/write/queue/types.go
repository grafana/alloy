package queue

import (
	"fmt"
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
		Parallelism: ParallelismConfig{
			DriftScaleUp:                60 * time.Second,
			DriftScaleDown:              30 * time.Second,
			MaxConnections:              50,
			MinConnections:              2,
			NetworkFlushInterval:        1 * time.Minute,
			DesiredConnectionsLookback:  5 * time.Minute,
			DesiredCheckInterval:        5 * time.Second,
			AllowedNetworkErrorFraction: 0.50,
		},
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
		if conn.Parallelism.MaxConnections < conn.Parallelism.MinConnections {
			return fmt.Errorf("max_connections less than min_connections")
		}
		if conn.Parallelism.MinConnections == 0 {
			return fmt.Errorf("min_connections must be greater than 0")
		}
		if conn.Parallelism.DriftScaleUp <= conn.Parallelism.DriftScaleDown {
			return fmt.Errorf("drift_scale_up_seconds less than or equal drift_scale_down_seconds")
		}
		// Any lower than 1 second and you spend a fair amount of time churning on the draining and
		// refilling the write buffers.
		if conn.Parallelism.DesiredCheckInterval < 1*time.Second {
			return fmt.Errorf("desired_check_interval must be greater than or equal to 1 second")
		}
		if conn.Parallelism.AllowedNetworkErrorFraction < 0 || conn.Parallelism.AllowedNetworkErrorFraction > 1 {
			return fmt.Errorf("allowed_network_error_percent must be between 0.00 and 1.00")
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
	Parallelism    ParallelismConfig `alloy:"parallelism,block,optional"`
	ExternalLabels map[string]string `alloy:"external_labels,attr,optional"`
	TLSConfig      *TLSConfig        `alloy:"tls_config,block,optional"`
	RoundRobin     bool              `alloy:"enable_round_robin,attr,optional"`
}

type TLSConfig struct {
	CA                 string            `alloy:"ca_pem,attr,optional"`
	Cert               string            `alloy:"cert_pem,attr,optional"`
	Key                alloytypes.Secret `alloy:"key_pem,attr,optional"`
	InsecureSkipVerify bool              `alloy:"insecure_skip_verify,attr,optional"`
}

type ParallelismConfig struct {
	DriftScaleUp                time.Duration `alloy:"drift_scale_up,attr,optional"`
	DriftScaleDown              time.Duration `alloy:"drift_scale_down,attr,optional"`
	MaxConnections              uint          `alloy:"max_connections,attr,optional"`
	MinConnections              uint          `alloy:"min_connections,attr,optional"`
	NetworkFlushInterval        time.Duration `alloy:"network_flush_interval,attr,optional"`
	DesiredConnectionsLookback  time.Duration `alloy:"desired_connections_lookback,attr,optional"`
	DesiredCheckInterval        time.Duration `alloy:"desired_check_interval,attr,optional"`
	AllowedNetworkErrorFraction float64       `alloy:"allowed_network_error_fraction,attr,optional"`
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
		UseRoundRobin:    cc.RoundRobin,
		Parallelism: types.ParallelismConfig{
			AllowedDrift:                cc.Parallelism.DriftScaleUp,
			MinimumScaleDownDrift:       cc.Parallelism.DriftScaleDown,
			MaxConnections:              cc.Parallelism.MaxConnections,
			MinConnections:              cc.Parallelism.MinConnections,
			ResetInterval:               cc.Parallelism.NetworkFlushInterval,
			Lookback:                    cc.Parallelism.DesiredConnectionsLookback,
			CheckInterval:               cc.Parallelism.DesiredCheckInterval,
			AllowedNetworkErrorFraction: cc.Parallelism.AllowedNetworkErrorFraction,
		},
	}
	if cc.BasicAuth != nil {
		tcc.BasicAuth = &types.BasicAuth{
			Username: cc.BasicAuth.Username,
			Password: string(cc.BasicAuth.Password),
		}
	}
	if cc.TLSConfig != nil {
		tcc.InsecureSkipVerify = cc.TLSConfig.InsecureSkipVerify
		tcc.TLSCert = cc.TLSConfig.Cert
		tcc.TLSKey = string(cc.TLSConfig.Key)
		tcc.TLSCACert = cc.TLSConfig.CA
	}
	return tcc
}

type BasicAuth struct {
	Username string            `alloy:"username,attr,optional"`
	Password alloytypes.Secret `alloy:"password,attr,optional"`
}
