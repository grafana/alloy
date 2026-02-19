package spanmetrics

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
	"go.opentelemetry.io/collector/config/configoptional"
)

// Dimension defines the dimension name and optional default value if the Dimension is missing from a span attribute.
type Dimension struct {
	Name    string  `alloy:"name,attr"`
	Default *string `alloy:"default,attr,optional"`
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
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.96.0/connector/spanmetricsconnector/internal/metrics/unit.go
func ConvertMetricUnit(unit string) (map[string]any, error) {
	switch unit {
	case MetricsUnitMilliseconds:
		return map[string]any{
			"unit": 0,
		}, nil
	case MetricsUnitSeconds:
		return map[string]any{
			"unit": 1,
		}, nil
	default:
		return nil, fmt.Errorf(
			"unknown unit %q, allowed units are %q and %q",
			unit, MetricsUnitMilliseconds, MetricsUnitSeconds)
	}
}

type HistogramConfig struct {
	Disable     bool                        `alloy:"disable,attr,optional"`
	Dimensions  []Dimension                 `alloy:"dimension,block,optional"`
	Unit        string                      `alloy:"unit,attr,optional"`
	Exponential *ExponentialHistogramConfig `alloy:"exponential,block,optional"`
	Explicit    *ExplicitHistogramConfig    `alloy:"explicit,block,optional"`
}

var (
	_ syntax.Defaulter = (*HistogramConfig)(nil)
	_ syntax.Validator = (*HistogramConfig)(nil)
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

	result.Exponential = hc.Exponential.Convert()

	result.Explicit = hc.Explicit.Convert()

	dimensions := make([]spanmetricsconnector.Dimension, 0, len(hc.Dimensions))
	for _, d := range hc.Dimensions {
		dimensions = append(dimensions, d.Convert())
	}
	result.Dimensions = dimensions

	result.Disable = hc.Disable
	return &result, nil
}

type ExemplarsConfig struct {
	Enabled         bool `alloy:"enabled,attr,optional"`
	MaxPerDataPoint int  `alloy:"max_per_data_point,attr,optional"`
}

func (ec ExemplarsConfig) Convert() *spanmetricsconnector.ExemplarsConfig {
	return &spanmetricsconnector.ExemplarsConfig{
		Enabled:         ec.Enabled,
		MaxPerDataPoint: ec.MaxPerDataPoint,
	}
}

type ExponentialHistogramConfig struct {
	MaxSize int32 `alloy:"max_size,attr,optional"`
}

var (
	_ syntax.Defaulter = (*ExponentialHistogramConfig)(nil)
	_ syntax.Validator = (*ExponentialHistogramConfig)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (ehc *ExponentialHistogramConfig) SetToDefault() {
	ehc.MaxSize = 160
}

// Validate implements syntax.Validator.
func (ehc *ExponentialHistogramConfig) Validate() error {
	if ehc.MaxSize <= 0 {
		return fmt.Errorf("max_size must be greater than 0")
	}

	return nil
}

func (ehc *ExponentialHistogramConfig) Convert() configoptional.Optional[spanmetricsconnector.ExponentialHistogramConfig] {
	if ehc == nil {
		return configoptional.None[spanmetricsconnector.ExponentialHistogramConfig]()
	}

	return configoptional.Some(spanmetricsconnector.ExponentialHistogramConfig{
		MaxSize: ehc.MaxSize,
	})
}

type ExplicitHistogramConfig struct {
	// Buckets is the list of durations representing explicit histogram buckets.
	Buckets []time.Duration `alloy:"buckets,attr,optional"`
}

var (
	_ syntax.Defaulter = (*ExplicitHistogramConfig)(nil)
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

func (hc *ExplicitHistogramConfig) Convert() configoptional.Optional[spanmetricsconnector.ExplicitHistogramConfig] {
	if hc == nil {
		return configoptional.None[spanmetricsconnector.ExplicitHistogramConfig]()
	}

	// Copy the values in the buckets slice so that we don't mutate the original.
	return configoptional.Some(spanmetricsconnector.ExplicitHistogramConfig{
		Buckets: append([]time.Duration{}, hc.Buckets...),
	})
}

type EventsConfig struct {
	// Enabled is a flag to enable events.
	Enabled bool `alloy:"enabled,attr,optional"`
	// Dimensions defines the list of dimensions to add to the events metric.
	Dimensions []Dimension `alloy:"dimension,block,optional"`
}

func (ec EventsConfig) Convert() spanmetricsconnector.EventsConfig {
	dimensions := make([]spanmetricsconnector.Dimension, 0, len(ec.Dimensions))
	for _, d := range ec.Dimensions {
		dimensions = append(dimensions, d.Convert())
	}

	return spanmetricsconnector.EventsConfig{
		Enabled:    ec.Enabled,
		Dimensions: dimensions,
	}
}
