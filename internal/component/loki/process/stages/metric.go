package stages

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/loki/process/metric"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/labelstore"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
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
	Metrics              []MetricConfig       `alloy:"metric,enum,optional"`
	ForwardTo            []storage.Appendable `alloy:"forward_to,attr,optional"`
	MetricsFlushInterval time.Duration        `alloy:"metrics_flush_interval,attr,optional"`
}

func (m *MetricsConfig) SetToDefault() {
	m.MetricsFlushInterval = 60 * time.Second
}

type cfgCollector struct {
	cfg       MetricConfig
	collector prometheus_client.Collector
	fullName  string
}

// newMetricStage creates a new set of metrics to process for each log entry
func newMetricStage(logger log.Logger, config MetricsConfig, registry prometheus_client.Registerer, componentID string, ls labelstore.LabelStore) (Stage, error) {
	metrics := map[string]cfgCollector{}
	var fanout *prometheus.Fanout
	if len(config.ForwardTo) > 0 {
		fanout = prometheus.NewFanout(config.ForwardTo, componentID, registry, ls)
	}

	for _, cfg := range config.Metrics {
		var collector prometheus_client.Collector
		var err error
		var fullName string
		switch {
		case cfg.Counter != nil:
			customPrefix := ""
			if cfg.Counter.Prefix != "" {
				customPrefix = cfg.Counter.Prefix
			} else {
				customPrefix = defaultMetricsPrefix
			}
			fullName = customPrefix + cfg.Counter.Name
			collector, err = metric.NewCounters(fullName, cfg.Counter)
			if err != nil {
				return nil, err
			}
			// Register the collector with the registry when not forwarding to another component.
			if fanout == nil {
				// It is safe to .MustRegister here because the metric created above is unchecked.
				registry.MustRegister(collector)
			}
			metrics[cfg.Counter.Name] = cfgCollector{cfg: cfg, collector: collector, fullName: fullName}
		case cfg.Gauge != nil:
			customPrefix := ""
			if cfg.Gauge.Prefix != "" {
				customPrefix = cfg.Gauge.Prefix
			} else {
				customPrefix = defaultMetricsPrefix
			}
			fullName = customPrefix + cfg.Gauge.Name
			collector, err = metric.NewGauges(fullName, cfg.Gauge)
			if err != nil {
				return nil, err
			}
			// Register the collector with the registry when not forwarding to another component.
			if fanout == nil {
				// It is safe to .MustRegister here because the metric created above is unchecked.
				registry.MustRegister(collector)
			}
			metrics[cfg.Gauge.Name] = cfgCollector{cfg: cfg, collector: collector, fullName: fullName}
		case cfg.Histogram != nil:
			customPrefix := ""
			if cfg.Histogram.Prefix != "" {
				customPrefix = cfg.Histogram.Prefix
			} else {
				customPrefix = defaultMetricsPrefix
			}
			fullName = customPrefix + cfg.Histogram.Name
			collector, err = metric.NewHistograms(fullName, cfg.Histogram)
			if err != nil {
				return nil, err
			}
			// Register the collector with the registry when not forwarding to another component.
			if fanout == nil {
				// It is safe to .MustRegister here because the metric created above is unchecked.
				registry.MustRegister(collector)
			}
			metrics[cfg.Histogram.Name] = cfgCollector{cfg: cfg, collector: collector, fullName: fullName}
		default:
			return nil, fmt.Errorf("undefined stage type in '%v', exiting", cfg)
		}
	}
	return &metricStage{
		logger:        logger,
		metrics:       metrics,
		forwardTo:     config.ForwardTo,
		fanout:        fanout,
		flushInterval: config.MetricsFlushInterval,
		quit:          make(chan struct{}),
	}, nil
}

// metricStage creates and updates prometheus metrics based on extracted pipeline data
type metricStage struct {
	logger        log.Logger
	metrics       map[string]cfgCollector
	forwardTo     []storage.Appendable
	fanout        *prometheus.Fanout
	flushInterval time.Duration
	quit          chan struct{}
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

	if len(m.forwardTo) > 0 && m.fanout != nil {
		go m.runFlushLoop()
	}

	return out
}

func (m *metricStage) runFlushLoop() {
	ticker := time.NewTicker(m.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.flushMetrics()
		case <-m.quit:
			return
		}
	}
}

func (m *metricStage) flushMetrics() {
	if m.fanout == nil {
		level.Error(m.logger).Log("msg", "fanout is not set, skipping flush")
		return
	}

	ctx := context.Background()
	app := m.fanout.Appender(ctx)
	timestamp := time.Now().UnixMilli()

	// For each metric block
	for _, cc := range m.metrics {
		ch := make(chan prometheus_client.Metric)
		go func() {
			cc.collector.Collect(ch)
			close(ch)
		}()

		for metric := range ch {
			var d dto.Metric
			if err := metric.Write(&d); err != nil {
				level.Error(m.logger).Log("msg", "failed to write metric to dto", "err", err)
				continue
			}

			var ls []labels.Label
			ls = append(ls, labels.Label{Name: labels.MetricName, Value: cc.fullName})
			for _, lp := range d.Label {
				ls = append(ls, labels.Label{Name: lp.GetName(), Value: lp.GetValue()})
			}
			// Get sorted labels
			lbls := labels.New(ls...)

			if d.Counter != nil {
				if _, err := app.UpdateMetadata(0, lbls, metadata.Metadata{
					Type: model.MetricTypeCounter,
					Help: cc.cfg.Counter.Description,
				}); err != nil {
					level.Error(m.logger).Log("msg", "failed to update metadata", "err", err)
				}

				if _, err := app.Append(0, lbls, timestamp, d.Counter.GetValue()); err != nil {
					level.Error(m.logger).Log("msg", "failed to append counter", "err", err)
				}
			} else if d.Gauge != nil {
				if _, err := app.UpdateMetadata(0, lbls, metadata.Metadata{
					Type: model.MetricTypeGauge,
					Help: cc.cfg.Gauge.Description,
				}); err != nil {
					level.Error(m.logger).Log("msg", "failed to update metadata", "err", err)
				}

				if _, err := app.Append(0, lbls, timestamp, d.Gauge.GetValue()); err != nil {
					level.Error(m.logger).Log("msg", "failed to append gauge", "err", err)
				}
			} else if d.Histogram != nil {
				h := d.Histogram

				if _, err := app.UpdateMetadata(0, lbls, metadata.Metadata{
					Type: model.MetricTypeHistogram,
					Help: cc.cfg.Histogram.Description,
				}); err != nil {
					level.Error(m.logger).Log("msg", "failed to update metadata", "err", err)
				}

				// Buckets
				for _, b := range h.Bucket {
					bucketLs := make([]labels.Label, 0, len(ls)+1)
					bucketLs = append(bucketLs, ls...)

					for i, l := range bucketLs {
						if l.Name == labels.MetricName {
							bucketLs[i].Value = cc.fullName + "_bucket"
							break
						}
					}
					bucketLs = append(bucketLs, labels.Label{Name: "le", Value: fmt.Sprintf("%g", b.GetUpperBound())})
					bucketLbls := labels.New(bucketLs...)

					if _, err := app.Append(0, bucketLbls, timestamp, float64(b.GetCumulativeCount())); err != nil {
						level.Error(m.logger).Log("msg", "failed to append histogram bucket", "err", err)
					}
				}

				// Sum
				sumLs := make([]labels.Label, len(ls))
				copy(sumLs, ls)
				for i, l := range sumLs {
					if l.Name == labels.MetricName {
						sumLs[i].Value = cc.fullName + "_sum"
						break
					}
				}
				sumLbls := labels.New(sumLs...)

				if _, err := app.Append(0, sumLbls, timestamp, h.GetSampleSum()); err != nil {
					level.Error(m.logger).Log("msg", "failed to append histogram sum", "err", err)
				}

				// Count
				countLs := make([]labels.Label, len(ls))
				copy(countLs, ls)
				for i, l := range countLs {
					if l.Name == labels.MetricName {
						countLs[i].Value = cc.fullName + "_count"
						break
					}
				}
				countLbls := labels.New(countLs...)

				if _, err := app.Append(0, countLbls, timestamp, float64(h.GetSampleCount())); err != nil {
					level.Error(m.logger).Log("msg", "failed to append histogram count", "err", err)
				}
			}
		}
	}

	if err := app.Commit(); err != nil {
		level.Error(m.logger).Log("msg", "failed to commit metrics", "err", err)
	}
}

// Process implements Stage
func (m *metricStage) Process(labels model.LabelSet, extracted map[string]interface{}, _ *time.Time, entry *string) {
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
	close(m.quit)
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
func (m *metricStage) recordCounter(name string, counter *metric.Counters, labels model.LabelSet, v interface{}) {
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
func (m *metricStage) recordGauge(name string, gauge *metric.Gauges, labels model.LabelSet, v interface{}) {
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
func (m *metricStage) recordHistogram(name string, histogram *metric.Histograms, labels model.LabelSet, v interface{}) {
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
func getFloat(unk interface{}) (float64, error) {
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
