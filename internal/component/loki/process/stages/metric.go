package stages

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/loki/process/metric"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// Metric types.
const (
	defaultMetricsPrefix = "loki_process_custom_"
)

// MetricConfig is a single metrics configuration.
type MetricConfig struct {
	Counter   *metric.CounterConfig   `alloy:"counter,block,optional"`
	Gauge     *metric.GaugeConfig     `alloy:"gauge,block,optional"`
	Histogram *metric.HistogramConfig `alloy:"histogram,block,optional"`
}

// MetricsConfig is a set of configured metrics.
type MetricsConfig struct {
	Metrics []MetricConfig `alloy:"metric,enum,optional"`
}

type cfgCollector struct {
	cfg       MetricConfig
	collector prometheus.Collector
}

// newMetricStage creates a new set of metrics to process for each log entry
func newMetricStage(logger log.Logger, config MetricsConfig, registry prometheus.Registerer) (Stage, error) {
	metrics := map[string]cfgCollector{}
	for _, cfg := range config.Metrics {
		var collector prometheus.Collector
		var err error

		switch {
		case cfg.Counter != nil:
			customPrefix := ""
			if cfg.Counter.Prefix != "" {
				customPrefix = cfg.Counter.Prefix
			} else {
				customPrefix = defaultMetricsPrefix
			}
			collector, err = metric.NewCounters(customPrefix+cfg.Counter.Name, cfg.Counter)
			if err != nil {
				return nil, err
			}
			// It is safe to .MustRegister here because the metric created above is unchecked.
			registry.MustRegister(collector)
			metrics[cfg.Counter.Name] = cfgCollector{cfg: cfg, collector: collector}
		case cfg.Gauge != nil:
			customPrefix := ""
			if cfg.Gauge.Prefix != "" {
				customPrefix = cfg.Gauge.Prefix
			} else {
				customPrefix = defaultMetricsPrefix
			}
			collector, err = metric.NewGauges(customPrefix+cfg.Gauge.Name, cfg.Gauge)
			if err != nil {
				return nil, err
			}
			// It is safe to .MustRegister here because the metric created above is unchecked.
			registry.MustRegister(collector)
			metrics[cfg.Gauge.Name] = cfgCollector{cfg: cfg, collector: collector}
		case cfg.Histogram != nil:
			customPrefix := ""
			if cfg.Histogram.Prefix != "" {
				customPrefix = cfg.Histogram.Prefix
			} else {
				customPrefix = defaultMetricsPrefix
			}
			collector, err = metric.NewHistograms(customPrefix+cfg.Histogram.Name, cfg.Histogram)
			if err != nil {
				return nil, err
			}
			// It is safe to .MustRegister here because the metric created above is unchecked.
			registry.MustRegister(collector)
			metrics[cfg.Histogram.Name] = cfgCollector{cfg: cfg, collector: collector}
		default:
			return nil, fmt.Errorf("undefined stage type in '%v', exiting", cfg)
		}
	}
	return &metricStage{
		logger:  logger,
		metrics: metrics,
	}, nil
}

// metricStage creates and updates prometheus metrics based on extracted pipeline data
type metricStage struct {
	logger  log.Logger
	metrics map[string]cfgCollector
}

func (m *metricStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)

		for e := range in {
			m.Process(e.Labels, e.Extracted, &e.Timestamp, &e.Line)
			out <- e
		}
	}()
	return out
}

// Process implements Stage
func (m *metricStage) Process(labels model.LabelSet, extracted map[string]any, _ *time.Time, entry *string) {
	for name, cc := range m.metrics {
		// There is a special case for counters where we count even if there is no match in the extracted map.
		if c, ok := cc.collector.(*metric.Counters); ok {
			if c != nil && c.Cfg.MatchAll {
				if c.Cfg.CountEntryBytes {
					if entry != nil {
						m.recordCounter(name, c, labels, len(*entry))
					}
				} else {
					m.recordCounter(name, c, labels, nil)
				}
				continue
			}
		}
		switch {
		case cc.cfg.Counter != nil:
			if v, ok := extracted[cc.cfg.Counter.Source]; ok {
				m.recordCounter(name, cc.collector.(*metric.Counters), labels, v)
			} else {
				level.Debug(m.logger).Log("msg", "source does not exist", "err", fmt.Sprintf("source: %s, does not exist", cc.cfg.Counter.Source))
			}
		case cc.cfg.Gauge != nil:
			if v, ok := extracted[cc.cfg.Gauge.Source]; ok {
				m.recordGauge(name, cc.collector.(*metric.Gauges), labels, v)
			} else {
				level.Debug(m.logger).Log("msg", "source does not exist", "err", fmt.Sprintf("source: %s, does not exist", cc.cfg.Gauge.Source))
			}
		case cc.cfg.Histogram != nil:
			if v, ok := extracted[cc.cfg.Histogram.Source]; ok {
				m.recordHistogram(name, cc.collector.(*metric.Histograms), labels, v)
			} else {
				level.Debug(m.logger).Log("msg", "source does not exist", "err", fmt.Sprintf("source: %s, does not exist", cc.cfg.Histogram.Source))
			}
		}
	}
}

// Name implements Stage
func (m *metricStage) Name() string {
	return StageTypeMetric
}

// Cleanup implements Stage.
func (m *metricStage) Cleanup() {
	for _, cfgCollector := range m.metrics {
		switch vec := cfgCollector.collector.(type) {
		case *metric.Counters:
			vec.DeleteAll()
		case *metric.Gauges:
			vec.DeleteAll()
		case *metric.Histograms:
			vec.DeleteAll()
		}
	}
}

// recordCounter will update a counter metric
func (m *metricStage) recordCounter(name string, counter *metric.Counters, labels model.LabelSet, v any) {
	// If value matching is defined, make sure value matches.
	if counter.Cfg.Value != "" {
		stringVal, err := getString(v)
		if err != nil {
			if Debug {
				level.Debug(m.logger).Log("msg", "failed to convert extracted value to string, "+
					"can't perform value comparison", "metric", name, "err",
					fmt.Sprintf("can't convert %v to string", reflect.TypeOf(v)))
			}
			return
		}
		if counter.Cfg.Value != stringVal {
			return
		}
	}

	switch counter.Cfg.Action {
	case metric.CounterInc:
		counter.With(labels).Inc()
	case metric.CounterAdd:
		f, err := getFloat(v)
		if err != nil {
			if Debug {
				level.Debug(m.logger).Log("msg", "failed to convert extracted value to positive float", "metric", name, "err", err)
			}
			return
		}
		counter.With(labels).Add(f)
	}
}

// recordGauge will update a gauge metric
func (m *metricStage) recordGauge(name string, gauge *metric.Gauges, labels model.LabelSet, v any) {
	// If value matching is defined, make sure value matches.
	if gauge.Cfg.Value != "" {
		stringVal, err := getString(v)
		if err != nil {
			if Debug {
				level.Debug(m.logger).Log("msg", "failed to convert extracted value to string, "+
					"can't perform value comparison", "metric", name, "err",
					fmt.Sprintf("can't convert %v to string", reflect.TypeOf(v)))
			}
			return
		}
		if gauge.Cfg.Value != stringVal {
			return
		}
	}

	switch gauge.Cfg.Action {
	case metric.GaugeSet:
		f, err := getFloat(v)
		if err != nil {
			if Debug {
				level.Debug(m.logger).Log("msg", "failed to convert extracted value to positive float", "metric", name, "err", err)
			}
			return
		}
		gauge.With(labels).Set(f)
	case metric.GaugeInc:
		gauge.With(labels).Inc()
	case metric.GaugeDec:
		gauge.With(labels).Dec()
	case metric.GaugeAdd:
		f, err := getFloat(v)
		if err != nil {
			if Debug {
				level.Debug(m.logger).Log("msg", "failed to convert extracted value to positive float", "metric", name, "err", err)
			}
			return
		}
		gauge.With(labels).Add(f)
	case metric.GaugeSub:
		f, err := getFloat(v)
		if err != nil {
			if Debug {
				level.Debug(m.logger).Log("msg", "failed to convert extracted value to positive float", "metric", name, "err", err)
			}
			return
		}
		gauge.With(labels).Sub(f)
	}
}

// recordHistogram will update a Histogram metric
func (m *metricStage) recordHistogram(name string, histogram *metric.Histograms, labels model.LabelSet, v any) {
	// If value matching is defined, make sure value matches.
	if histogram.Cfg.Value != "" {
		stringVal, err := getString(v)
		if err != nil {
			if Debug {
				level.Debug(m.logger).Log("msg", "failed to convert extracted value to string, "+
					"can't perform value comparison", "metric", name, "err",
					fmt.Sprintf("can't convert %v to string", reflect.TypeOf(v)))
			}
			return
		}
		if histogram.Cfg.Value != stringVal {
			return
		}
	}
	f, err := getFloat(v)
	if err != nil {
		if Debug {
			level.Debug(m.logger).Log("msg", "failed to convert extracted value to float", "metric", name, "err", err)
		}
		return
	}
	histogram.With(labels).Observe(f)
}

// getFloat will take the provided value and return a float64 if possible
func getFloat(unk any) (float64, error) {
	switch i := unk.(type) {
	case float64:
		return i, nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case string:
		return getFloatFromString(i)
	case bool:
		if i {
			return float64(1), nil
		}
		return float64(0), nil
	default:
		return math.NaN(), fmt.Errorf("can't convert %v to float64", unk)
	}
}

// getFloatFromString converts string into float64
// Two types of string formats are supported:
//   - strings that represent floating point numbers, e.g., "0.804"
//   - duration format strings, e.g., "0.5ms", "10h".
//     Valid time units are "ns", "us", "ms", "s", "m", "h".
//     Values in this format are converted as a floating point number of seconds.
//     E.g., "0.5ms" is converted to 0.0005
func getFloatFromString(str string) (float64, error) {
	dur, err := strconv.ParseFloat(str, 64)
	if err != nil {
		dur, err := time.ParseDuration(str)
		if err != nil {
			return 0, err
		}
		return dur.Seconds(), nil
	}
	return dur, nil
}
