package metric

// NOTE: This code is copied from Promtail (07cbef92268aecc0f20d1791a6df390c2df5c072) with changes kept to the minimum.

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

// DefaultHistogramConfig sets the defaults for a Histogram.
var DefaultHistogramConfig = HistogramConfig{
	MaxIdle: 5 * time.Minute,
}

// HistogramConfig defines a histogram metric whose values are bucketed.
type HistogramConfig struct {
	// Shared fields
	Name        string        `alloy:"name,attr"`
	Description string        `alloy:"description,attr,optional"`
	Source      string        `alloy:"source,attr,optional"`
	Prefix      string        `alloy:"prefix,attr,optional"`
	MaxIdle     time.Duration `alloy:"max_idle_duration,attr,optional"`
	Value       string        `alloy:"value,attr,optional"`

	// Histogram-specific fields
	Buckets []float64 `alloy:"buckets,attr"`
}

// SetToDefault implements syntax.Defaulter.
func (h *HistogramConfig) SetToDefault() {
	*h = DefaultHistogramConfig
}

// Validate implements syntax.Validator.
func (h *HistogramConfig) Validate() error {
	if h.MaxIdle < 1*time.Second {
		return fmt.Errorf("max_idle_duration must be greater or equal than 1s")
	}

	if h.Source == "" {
		h.Source = h.Name
	}
	return nil
}

// Histograms is a vector of histograms for a log stream.
type Histograms struct {
	*metricVec
	Cfg *HistogramConfig
}

// NewHistograms creates a new histogram vec.
func NewHistograms(name string, config *HistogramConfig) (*Histograms, error) {
	return &Histograms{
		metricVec: newMetricVec(func(labels map[string]string) prometheus.Metric {
			return &expiringHistogram{prometheus.NewHistogram(prometheus.HistogramOpts{
				Help:        config.Description,
				Name:        name,
				ConstLabels: labels,
				Buckets:     config.Buckets,
			}),
				atomic.Int64{},
			}
		}, int64(config.MaxIdle.Seconds())),
		Cfg: config,
	}, nil
}

// With returns the histogram associated with a stream labelset.
func (h *Histograms) With(labels model.LabelSet) prometheus.Histogram {
	return h.metricVec.With(labels).(prometheus.Histogram)
}

type expiringHistogram struct {
	prometheus.Histogram
	lastModSec atomic.Int64
}

// Observe adds a single observation to the histogram.
func (h *expiringHistogram) Observe(val float64) {
	h.Histogram.Observe(val)
	h.lastModSec.Store(time.Now().Unix())
}

// HasExpired implements Expirable
func (h *expiringHistogram) HasExpired(currentTimeSec int64, maxAgeSec int64) bool {
	return currentTimeSec-h.lastModSec.Load() >= maxAgeSec
}
