package otelcol

import (
	"fmt"
	"time"

	"github.com/grafana/river"
	otelexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
)

// RetryArguments holds shared settings for components which can retry
// requests.
type RetryArguments struct {
	Enabled             bool          `river:"enabled,attr,optional"`
	InitialInterval     time.Duration `river:"initial_interval,attr,optional"`
	RandomizationFactor float64       `river:"randomization_factor,attr,optional"`
	Multiplier          float64       `river:"multiplier,attr,optional"`
	MaxInterval         time.Duration `river:"max_interval,attr,optional"`
	MaxElapsedTime      time.Duration `river:"max_elapsed_time,attr,optional"`
}

var (
	_ river.Defaulter = (*RetryArguments)(nil)
	_ river.Validator = (*RetryArguments)(nil)
)

// DefaultRetryArguments holds default settings for RetryArguments.
var DefaultRetryArguments = RetryArguments{
	Enabled:             true,
	InitialInterval:     5 * time.Second,
	RandomizationFactor: 0.5,
	Multiplier:          1.5,
	MaxInterval:         30 * time.Second,
	MaxElapsedTime:      5 * time.Minute,
}

// SetToDefault implements river.Defaulter.
func (args *RetryArguments) SetToDefault() {
	*args = DefaultRetryArguments
}

// Validate returns an error if args is invalid.
func (args *RetryArguments) Validate() error {
	if args.Multiplier <= 1 {
		return fmt.Errorf("multiplier must be greater than 1.0")
	}

	if args.RandomizationFactor < 0 {
		return fmt.Errorf("randomization_factor must be greater or equal to 0")
	}

	return nil
}

// Convert converts args into the upstream type.
func (args *RetryArguments) Convert() *otelexporterhelper.RetrySettings {
	if args == nil {
		return nil
	}

	return &otelexporterhelper.RetrySettings{
		Enabled:             args.Enabled,
		InitialInterval:     args.InitialInterval,
		RandomizationFactor: args.RandomizationFactor,
		Multiplier:          args.Multiplier,
		MaxInterval:         args.MaxInterval,
		MaxElapsedTime:      args.MaxElapsedTime,
	}
}
