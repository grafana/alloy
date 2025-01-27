package otelcol

import (
	"time"

	"github.com/grafana/alloy/syntax"
)

// ConsumerRetryArguments holds shared settings for stanza receivers which can retry
// requests. There is no Convert functionality as the consumerretry package is stanza internal
type ConsumerRetryArguments struct {
	Enabled         bool          `alloy:"enabled,attr,optional"`
	InitialInterval time.Duration `alloy:"initial_interval,attr,optional"`
	MaxInterval     time.Duration `alloy:"max_interval,attr,optional"`
	MaxElapsedTime  time.Duration `alloy:"max_elapsed_time,attr,optional"`
}

var (
	_ syntax.Defaulter = (*ConsumerRetryArguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *ConsumerRetryArguments) SetToDefault() {
	*args = ConsumerRetryArguments{
		Enabled:         false,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		MaxElapsedTime:  5 * time.Minute,
	}
}
