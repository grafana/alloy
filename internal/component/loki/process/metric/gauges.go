package metric

// NOTE: This code is copied from Promtail (07cbef92268aecc0f20d1791a6df390c2df5c072) with changes kept to the minimum.

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

const (
	GaugeSet = "set"
	GaugeInc = "inc"
	GaugeDec = "dec"
	GaugeAdd = "add"
	GaugeSub = "sub"

	ErrGaugeActionRequired = "gauge action must be defined as `set`, `inc`, `dec`, `add`, or `sub`"
	ErrGaugeInvalidAction  = "action %s is not valid, action must be `set`, `inc`, `dec`, `add`, or `sub`"
)

// DefaultGaugeConfig sets the defaults for a Gauge.
var DefaultGaugeConfig = GaugeConfig{
	MaxIdle: 5 * time.Minute,
}

// GaugeConfig defines a gauge metric whose value can go up or down.
type GaugeConfig struct {
	// Shared fields
	Name        string        `river:"name,attr"`
	Description string        `river:"description,attr,optional"`
	Source      string        `river:"source,attr,optional"`
	Prefix      string        `river:"prefix,attr,optional"`
	MaxIdle     time.Duration `river:"max_idle_duration,attr,optional"`
	Value       string        `river:"value,attr,optional"`

	// Gauge-specific fields
	Action string `river:"action,attr"`
}

// SetToDefault implements river.Defaulter.
func (g *GaugeConfig) SetToDefault() {
	*g = DefaultGaugeConfig
}

// Validate implements river.Validator.
func (g *GaugeConfig) Validate() error {
	if g.MaxIdle < 1*time.Second {
		return fmt.Errorf("max_idle_duration must be greater or equal than 1s")
	}

	if g.Source == "" {
		g.Source = g.Name
	}

	// TODO (@tpaschalis) A better way to keep track of these?
	if g.Action != "set" && g.Action != "inc" && g.Action != "dec" && g.Action != "add" && g.Action != "sub" {
		return fmt.Errorf("the 'action' gauge field must be one of the following values: [set, inc, dec, add, sub]")
	}
	return nil
}

// Gauges is a vector of gauges for a log stream.
type Gauges struct {
	*metricVec
	Cfg *GaugeConfig
}

// NewGauges creates a new gauge vec.
func NewGauges(name string, config *GaugeConfig) (*Gauges, error) {
	return &Gauges{
		metricVec: newMetricVec(func(labels map[string]string) prometheus.Metric {
			return &expiringGauge{prometheus.NewGauge(prometheus.GaugeOpts{
				Help:        config.Description,
				Name:        name,
				ConstLabels: labels,
			}),
				0,
			}
		}, int64(config.MaxIdle.Seconds())),
		Cfg: config,
	}, nil
}

// With returns the gauge associated with a stream labelset.
func (g *Gauges) With(labels model.LabelSet) prometheus.Gauge {
	return g.metricVec.With(labels).(prometheus.Gauge)
}

type expiringGauge struct {
	prometheus.Gauge
	lastModSec int64
}

// Set sets the Gauge to an arbitrary value.
func (g *expiringGauge) Set(val float64) {
	g.Gauge.Set(val)
	g.lastModSec = time.Now().Unix()
}

// Inc increments the Gauge by 1. Use Add to increment it by arbitrary
// values.
func (g *expiringGauge) Inc() {
	g.Gauge.Inc()
	g.lastModSec = time.Now().Unix()
}

// Dec decrements the Gauge by 1. Use Sub to decrement it by arbitrary
// values.
func (g *expiringGauge) Dec() {
	g.Gauge.Dec()
	g.lastModSec = time.Now().Unix()
}

// Add adds the given value to the Gauge. (The value can be negative,
// resulting in a decrease of the Gauge.)
func (g *expiringGauge) Add(val float64) {
	g.Gauge.Add(val)
	g.lastModSec = time.Now().Unix()
}

// Sub subtracts the given value from the Gauge. (The value can be
// negative, resulting in an increase of the Gauge.)
func (g *expiringGauge) Sub(val float64) {
	g.Gauge.Sub(val)
	g.lastModSec = time.Now().Unix()
}

// SetToCurrentTime sets the Gauge to the current Unix time in seconds.
func (g *expiringGauge) SetToCurrentTime() {
	g.Gauge.SetToCurrentTime()
	g.lastModSec = time.Now().Unix()
}

// HasExpired implements Expirable
func (g *expiringGauge) HasExpired(currentTimeSec int64, maxAgeSec int64) bool {
	return currentTimeSec-g.lastModSec >= maxAgeSec
}
