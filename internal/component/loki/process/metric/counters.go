package metric

// NOTE: This code is copied from Promtail (07cbef92268aecc0f20d1791a6df390c2df5c072) with changes kept to the minimum.

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

const (
	CounterInc = "inc"
	CounterAdd = "add"
)

// CounterConfig defines a counter metric whose value only goes up.
type CounterConfig struct {
	// Shared fields
	Name        string        `river:"name,attr"`
	Description string        `river:"description,attr,optional"`
	Source      string        `river:"source,attr,optional"`
	Prefix      string        `river:"prefix,attr,optional"`
	MaxIdle     time.Duration `river:"max_idle_duration,attr,optional"`
	Value       string        `river:"value,attr,optional"`

	// Counter-specific fields
	Action          string `river:"action,attr"`
	MatchAll        bool   `river:"match_all,attr,optional"`
	CountEntryBytes bool   `river:"count_entry_bytes,attr,optional"`
}

// DefaultCounterConfig sets the default for a Counter.
var DefaultCounterConfig = CounterConfig{
	MaxIdle: 5 * time.Minute,
}

// SetToDefault implements river.Defaulter.
func (c *CounterConfig) SetToDefault() {
	*c = DefaultCounterConfig
}

// Validate implements river.Validator.
func (c *CounterConfig) Validate() error {
	if c.MaxIdle < 1*time.Second {
		return fmt.Errorf("max_idle_duration must be greater or equal than 1s")
	}

	if c.Source == "" {
		c.Source = c.Name
	}
	if c.Action != CounterInc && c.Action != CounterAdd {
		return fmt.Errorf("the 'action' counter field must be either 'inc' or 'add'")
	}

	if c.MatchAll && c.Value != "" {
		return fmt.Errorf("a 'counter' metric supports either 'match_all' or a 'value', but not both")
	}
	if c.CountEntryBytes && (!c.MatchAll || c.Action != "add") {
		return fmt.Errorf("the 'count_entry_bytes' counter field must be specified along with match_all set to true or action set to 'add'")
	}
	return nil
}

// Counters is a vector of counters for a log stream.
type Counters struct {
	*metricVec
	Cfg *CounterConfig
}

// NewCounters creates a new counter vec.
func NewCounters(name string, config *CounterConfig) (*Counters, error) {
	return &Counters{
		metricVec: newMetricVec(func(labels map[string]string) prometheus.Metric {
			return &expiringCounter{prometheus.NewCounter(prometheus.CounterOpts{
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

// With returns the counter associated with a stream labelset.
func (c *Counters) With(labels model.LabelSet) prometheus.Counter {
	return c.metricVec.With(labels).(prometheus.Counter)
}

type expiringCounter struct {
	prometheus.Counter
	lastModSec int64
}

// Inc increments the counter by 1. Use Add to increment it by arbitrary
// non-negative values.
func (e *expiringCounter) Inc() {
	e.Counter.Inc()
	e.lastModSec = time.Now().Unix()
}

// Add adds the given value to the counter. It panics if the value is <
// 0.
func (e *expiringCounter) Add(val float64) {
	e.Counter.Add(val)
	e.lastModSec = time.Now().Unix()
}

// HasExpired implements Expirable
func (e *expiringCounter) HasExpired(currentTimeSec int64, maxAgeSec int64) bool {
	return currentTimeSec-e.lastModSec >= maxAgeSec
}
