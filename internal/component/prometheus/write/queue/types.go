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
		Parallelism: ParralelismConfig{
			DriftScaleUpSeconds:        60,
			DriftScaleDownSeconds:      30,
			MaxConnections:             50,
			MinConnections:             2,
			NetworkFlushInterval:       1 * time.Minute,
			DesiredConnectionsLookback: 5 * time.Minute,
			DesiredCheckInterval:       5 * time.Second,
			AllowedNetworkErrorPercent: 0.50,
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
		if conn.Parallelism.DriftScaleUpSeconds <= conn.Parallelism.DriftScaleDownSeconds {
			return fmt.Errorf("drift_scale_up_seconds less than or equal drift_scale_down_seconds")
		}
		// Any lower than 1 second and you spend a fair amount of time churning on the draining and
		// refilling the write buffers.
		if conn.Parallelism.DesiredCheckInterval < 1*time.Second {
			return fmt.Errorf("desired_check_interval must be greater than or equal to 1 second")
		}
		if conn.Parallelism.AllowedNetworkErrorPercent < 0 || conn.Parallelism.AllowedNetworkErrorPercent > 1 {
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
	Parallelism    ParralelismConfig `alloy:"parallelism,block,optional"`
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

type ParralelismConfig struct {
	// DriftScaleUpSeconds is the maximum amount of seconds that is allowed for the Newest Timestamp Serializer - Newest Timestamp Sent via Network before the connections scales up.
	// Using non unix timestamp numbers. If Newest TS In Serializer sees 100s and Newest TS Out Network sees 20s then we have a drift of 80s. If AllowDriftSeconds is 60s that would
	// trigger a scaling up event.
	DriftScaleUpSeconds int64 `alloy:"drift_scale_up_seconds,attr,optional"`
	// DriftScaleDownSeconds is the amount if we go below that we can scale down. Using the above if In is 100s and Out is 70s and DriftScaleDownSeconds is 30 then we wont scale
	// down even though we are below the 60s. This is to keep the number of connections from flapping. In practice we should consider 30s DriftScaleDownSeconds and 60s DriftScaleUpSeconds to be a sweet spot
	// for general usage.
	DriftScaleDownSeconds int64 `alloy:"drift_scale_down_seconds,attr,optional"`
	// MaxConnections is the maximum number of concurrent connections to use.
	MaxConnections uint `alloy:"max_connections,attr,optional"`
	// MinConnections is the minimum number of concurrent connections to use.
	MinConnections uint `alloy:"min_connections,attr,optional"`
	// NetworkFlushInterval is how long to keep network successes and errors in memory for calculations.
	NetworkFlushInterval time.Duration `alloy:"network_flush_interval,attr,optional"`
	// DesiredConnectionsLookback is how far to lookback for previous desired values. This is to prevent flapping.
	// In a situation where in the past 5 minutes you have desired [1,2,1,1] and desired is 1 it will
	// choose 2 since that was the greatest. This determines how fast you can scale down.
	DesiredConnectionsLookback time.Duration `alloy:"desired_connections_lookback,attr,optional"`
	// DesiredCheckInterval is how long to check for desired values.
	DesiredCheckInterval time.Duration `alloy:"desired_check_interval,attr,optional"`
	// AllowedNetworkErrorPercent is the percentage of failed network requests that are allowable. This will
	// trigger a decrease in connections if exceeded.
	AllowedNetworkErrorPercent float64 `alloy:"allowed_network_error_percent,attr,optional"`
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
		Parralelism: types.ParralelismConfig{
			AllowedDriftSeconds:          cc.Parallelism.DriftScaleUpSeconds,
			MinimumScaleDownDriftSeconds: cc.Parallelism.DriftScaleDownSeconds,
			MaxConnections:               cc.Parallelism.MaxConnections,
			MinConnections:               cc.Parallelism.MinConnections,
			ResetInterval:                cc.Parallelism.NetworkFlushInterval,
			Lookback:                     cc.Parallelism.DesiredConnectionsLookback,
			CheckInterval:                cc.Parallelism.DesiredCheckInterval,
			AllowedNetworkErrorPercent:   cc.Parallelism.AllowedNetworkErrorPercent,
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
