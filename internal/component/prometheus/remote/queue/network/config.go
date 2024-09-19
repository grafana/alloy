package network

import (
	"reflect"
	"time"
)

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
