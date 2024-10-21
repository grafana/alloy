package types

import (
	"context"
	"github.com/grafana/alloy/syntax/alloytypes"
	"reflect"
	"time"
)

type NetworkClient interface {
	Start()
	Stop()
	SendSeries(ctx context.Context, d *TimeSeriesBinary) error
	SendMetadata(ctx context.Context, d *TimeSeriesBinary) error
	// UpdateConfig is a synchronous call and will only return once the config
	// is applied or an error occurs.
	UpdateConfig(ctx context.Context, cfg ConnectionConfig) error
}
type ConnectionConfig struct {
	URL              string
	BasicAuth        *BasicAuth
	BearerToken      alloytypes.Secret
	UserAgent        string
	Timeout          time.Duration
	RetryBackoff     time.Duration
	MaxRetryAttempts uint
	BatchCount       int
	FlushInterval    time.Duration
	ExternalLabels   map[string]string
	Connections      uint
}

type BasicAuth struct {
	Username string
	Password string
}

func (cc ConnectionConfig) Equals(bb ConnectionConfig) bool {
	return reflect.DeepEqual(cc, bb)
}
