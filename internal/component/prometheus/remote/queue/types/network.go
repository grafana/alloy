package types

import (
	"context"
	"reflect"
	"time"
)

type NetworkClient interface {
	Start()
	Stop()
	SendSeries(ctx context.Context, d *TimeSeriesBinary) error
	SendMetadata(ctx context.Context, d *TimeSeriesBinary) error
	UpdateConfig(ctx context.Context, cfg ConnectionConfig) error
}
type ConnectionConfig struct {
	URL                     string
	BasicAuth               *BasicAuth
	UserAgent               string
	Timeout                 time.Duration
	RetryBackoff            time.Duration
	MaxRetryBackoffAttempts uint
	BatchCount              int
	FlushFrequency          time.Duration
	ExternalLabels          map[string]string
	Connections             uint
}

type BasicAuth struct {
	Username string
	Password string
}

func (cc ConnectionConfig) Equals(bb ConnectionConfig) bool {
	return reflect.DeepEqual(cc, bb)
}
