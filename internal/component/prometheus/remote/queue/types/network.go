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
	Username                string
	Password                string
	UserAgent               string
	Timeout                 time.Duration
	RetryBackoff            time.Duration
	MaxRetryBackoffAttempts time.Duration
	BatchCount              int
	FlushFrequency          time.Duration
	ExternalLabels          map[string]string
	Connections             uint64
}

func (cc ConnectionConfig) Equals(bb ConnectionConfig) bool {
	return reflect.DeepEqual(cc, bb)
}
