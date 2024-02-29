package spanmetrics

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/river"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
)

// Dimension defines the dimension name and optional default value if the Dimension is missing from a span attribute.
type Dimension struct {
	Name    string  `river:"name,attr"`
	Default *string `river:"default,attr,optional"`
}

func (d Dimension) Convert() spanmetricsconnector.Dimension {
	res := spanmetricsconnector.Dimension{
		Name: d.Name,
	}

	if d.Default != nil {
		str := strings.Clone(*d.Default)
		res.Default = &str
	}

	return res
}

const (
	MetricsUnitMilliseconds string = "ms"
	MetricsUnitSeconds      string = "s"
)

// The unit is a private type in an internal Otel package,
// so we need to convert it to a map and then back to the internal type.
// ConvertMetricUnit matches the Unit type in this internal package:
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.87.0/connector/spanmetricsconnector/internal/metrics/unit.go
func ConvertMetricUnit(unit string) (map[string]interface{}, error) {
	switch unit {
	case MetricsUnitMilliseconds:
		return map[string]interface{}{
			"unit": 0,
		}, nil
	case MetricsUnitSeconds:
		return map[string]interface{}{
			"unit": 1,
		}, nil
	default:
		return nil, fmt.Errorf(
			"unknown unit %q, allowed units are %q and %q",
			unit, MetricsUnitMilliseconds, MetricsUnitSeconds)
	}
}

type HistogramConfig struct {
	Disable     bool                        `river:"disable,attr,optional"`
	Unit        string                      `river:"unit,attr,optional"`
	Exponential *ExponentialHistogramConfig `river:"exponential,block,optional"`
	Explicit    *ExplicitHistogramConfig    `river:"explicit,block,optional"`
}

var (
	_ river.Defaulter = (*HistogramConfig)(nil)
	_ river.Validator = (*HistogramConfig)(nil)
)

var DefaultHistogramConfig = HistogramConfig{
	Unit:        MetricsUnitMilliseconds,
	Exponential: nil,
	Explicit:    nil,
}

func (hc *HistogramConfig) SetToDefault() {
	*hc = DefaultHistogramConfig
}

func (hc *HistogramConfig) Validate() error {
	switch hc.Unit {
	case MetricsUnitMilliseconds, MetricsUnitSeconds:
		// Valid
	default:
		return fmt.Errorf(
			"unknown unit %q, allowed units are %q and %q",
			hc.Unit, MetricsUnitMilliseconds, MetricsUnitSeconds)
	}

	if hc.Exponential != nil && hc.Explicit != nil {
		return fmt.Errorf("only one of exponential or explicit histogram configuration can be specified")
	}

	if hc.Exponential == nil && hc.Explicit == nil {
		return fmt.Errorf("either exponential or explicit histogram configuration must be specified")
	}

	return nil
}

func (hc HistogramConfig) Convert() (*spanmetricsconnector.HistogramConfig, error) {
	input, err := ConvertMetricUnit(hc.Unit)
	if err != nil {
		return nil, err
	}

	var result spanmetricsconnector.HistogramConfig
	err = mapstructure.Decode(input, &result)
	if err != nil {
		return nil, err
	}

	if hc.Exponential != nil {
		result.Exponential = hc.Exponential.Convert()
	}

	if hc.Explicit != nil {
		result.Explicit = hc.Explicit.Convert()
	}

	result.Disable = hc.Disable
	return &result, nil
}

type ExemplarsConfig struct {
	Enabled bool `river:"enabled,attr,optional"`
}

func (ec ExemplarsConfig) Convert() *spanmetricsconnector.ExemplarsConfig {
	return &spanmetricsconnector.ExemplarsConfig{
		Enabled: ec.Enabled,
	}
}

type ExponentialHistogramConfig struct {
	MaxSize int32 `river:"max_size,attr,optional"`
}

var (
	_ river.Defaulter = (*ExponentialHistogramConfig)(nil)
	_ river.Validator = (*ExponentialHistogramConfig)(nil)
)

// SetToDefault implements river.Defaulter.
func (ehc *ExponentialHistogramConfig) SetToDefault() {
	ehc.MaxSize = 160
}

// Validate implements river.Validator.
func (ehc *ExponentialHistogramConfig) Validate() error {
	if ehc.MaxSize <= 0 {
		return fmt.Errorf("max_size must be greater than 0")
	}

	return nil
}

func (ehc ExponentialHistogramConfig) Convert() *spanmetricsconnector.ExponentialHistogramConfig {
	return &spanmetricsconnector.ExponentialHistogramConfig{
		MaxSize: ehc.MaxSize,
	}
}

type ExplicitHistogramConfig struct {
	// Buckets is the list of durations representing explicit histogram buckets.
	Buckets []time.Duration `river:"buckets,attr,optional"`
}

var (
	_ river.Defaulter = (*ExplicitHistogramConfig)(nil)
)

func (hc *ExplicitHistogramConfig) SetToDefault() {
	hc.Buckets = []time.Duration{
		2 * time.Millisecond,
		4 * time.Millisecond,
		6 * time.Millisecond,
		8 * time.Millisecond,
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1 * time.Second,
		1400 * time.Millisecond,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		15 * time.Second,
	}
}

func (hc ExplicitHistogramConfig) Convert() *spanmetricsconnector.ExplicitHistogramConfig {
	// Copy the values in the buckets slice so that we don't mutate the original.
	return &spanmetricsconnector.ExplicitHistogramConfig{
		Buckets: append([]time.Duration{}, hc.Buckets...),
	}
}
