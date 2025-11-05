package metric

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

// DefaultSummaryConfig sets the defaults for a Summary.
var DefaultSummaryConfig = SummaryConfig{
	MaxIdle: 5 * time.Minute,
}

type Objective struct {
	Quantile float64 `alloy:"quantile,attr"`
	Error    float64 `alloy:"error,attr"`
}

// SummaryConfig defines a summary metric whose values produce quantiles.
type SummaryConfig struct {
	// Shared fields
	Name        string        `alloy:"name,attr"`
	Description string        `alloy:"description,attr,optional"`
	Source      string        `alloy:"source,attr,optional"`
	Prefix      string        `alloy:"prefix,attr,optional"`
	MaxIdle     time.Duration `alloy:"max_idle_duration,attr,optional"`
	Value       string        `alloy:"value,attr,optional"`

	// Summary-specific fields
	Objectives *[]Objective `alloy:"objective,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (s *SummaryConfig) SetToDefault() {
	*s = DefaultSummaryConfig
}

// Validate implements syntax.Validator.
func (s *SummaryConfig) Validate() error {
	if s.MaxIdle < 1*time.Second {
		return fmt.Errorf("max_idle_duration must be greater or equal than 1s")
	}

	if s.Source == "" {
		s.Source = s.Name
	}

	return nil
}

// Summaries is a vector of summaries for a log stream.
type Summaries struct {
	*metricVec
	Cfg *SummaryConfig
}

// NewSummaries creates a new summary vec.
func NewSummaries(name string, config *SummaryConfig) (*Summaries, error) {
	// convert []Objective → map[float64]float64
	objectives := map[float64]float64{}
	if config.Objectives != nil {
		for _, obj := range *config.Objectives {
			objectives[obj.Quantile] = obj.Error
		}
	}

	return &Summaries{
		metricVec: newMetricVec(func(labels map[string]string) prometheus.Metric {
			return &expiringSummary{prometheus.NewSummary(prometheus.SummaryOpts{
				Help:        config.Description,
				Name:        name,
				ConstLabels: labels,
				Objectives:  objectives,
			}),
				0,
			}
		}, int64(config.MaxIdle.Seconds())),
		Cfg: config,
	}, nil
}

// With returns the summary associated with a stream labelset.
func (s *Summaries) With(labels model.LabelSet) prometheus.Summary {
	return s.metricVec.With(labels).(prometheus.Summary)
}

type expiringSummary struct {
	prometheus.Summary
	lastModSec int64
}

// Observe adds a single observation to the summary.
func (s *expiringSummary) Observe(val float64) {
	s.Summary.Observe(val)
	s.lastModSec = time.Now().Unix()
}

// HasExpired implements Expirable
func (s *expiringSummary) HasExpired(currentTimeSec int64, maxAgeSec int64) bool {
	return currentTimeSec-s.lastModSec >= maxAgeSec
}
