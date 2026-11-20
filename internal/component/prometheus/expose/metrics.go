package expose

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"google.golang.org/protobuf/proto"
)

// MetricSample holds a single metric sample
type MetricSample struct {
	Labels    labels.Labels
	Timestamp int64
	Value     float64
}

// MetricsCollector implements prometheus.Collector and stores metrics in memory
type MetricsCollector struct {
	mut          sync.RWMutex
	namespace    string
	subsystem    string
	globalLabels labels.Labels

	// Store metrics by metric name for efficient collection
	metrics map[string]*MetricSample
}

// NewMetricsCollector creates a new MetricsCollector
func NewMetricsCollector(namespace, subsystem string, globalLabels map[string]string) *MetricsCollector {
	builder := labels.NewBuilder(labels.EmptyLabels())
	for k, v := range globalLabels {
		builder.Set(k, v)
	}
	lbls := builder.Labels()

	return &MetricsCollector{
		namespace:    namespace,
		subsystem:    subsystem,
		globalLabels: lbls,
		metrics:      make(map[string]*MetricSample),
	}
}

// AppendMetric adds or updates a metric sample
func (mc *MetricsCollector) AppendMetric(lbls labels.Labels, timestamp int64, value float64) {
	mc.mut.Lock()
	defer mc.mut.Unlock()

	// Get metric name
	metricName := lbls.Get(model.MetricNameLabel)
	if metricName == "" {
		return
	}

	// Combine with global labels
	builder := labels.NewBuilder(lbls)
	mc.globalLabels.Range(func(lbl labels.Label) {
		if lbls.Get(lbl.Name) == "" {
			builder.Set(lbl.Name, lbl.Value)
		}
	})

	// Apply namespace and subsystem prefixes if specified
	if mc.namespace != "" || mc.subsystem != "" {
		builder.Set(model.MetricNameLabel, prometheus.BuildFQName(mc.namespace, mc.subsystem, metricName))
	}

	finalLabels := builder.Labels()
	key := finalLabels.String()

	mc.metrics[key] = &MetricSample{
		Labels:    finalLabels,
		Timestamp: timestamp,
		Value:     value,
	}
}

// Collect implements prometheus.Collector
func (mc *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	mc.mut.RLock()
	defer mc.mut.RUnlock()

	for _, sample := range mc.metrics {
		metric := newMetricFromSample(sample)
		if metric != nil {
			ch <- metric
		}
	}
}

// Describe implements prometheus.Collector.
// This is an unchecked collector — metric names are determined dynamically at
// runtime from the incoming time series, so no descriptors are sent.
func (mc *MetricsCollector) Describe(_ chan<- *prometheus.Desc) {
	// Intentionally empty: unchecked collector with dynamic metric names.
}

// newMetricFromSample converts a MetricSample to a prometheus.Metric
func newMetricFromSample(sample *MetricSample) prometheus.Metric {
	metricName := sample.Labels.Get(model.MetricNameLabel)
	labelNames := make([]string, 0)
	labelValues := make([]string, 0)

	sample.Labels.Range(func(lbl labels.Label) {
		if lbl.Name != model.MetricNameLabel {
			labelNames = append(labelNames, lbl.Name)
			labelValues = append(labelValues, lbl.Value)
		}
	})

	desc := prometheus.NewDesc(metricName, "Metric from prometheus.expose", labelNames, nil)

	// Create a gauge metric value
	value := &dto.Metric{
		Gauge: &dto.Gauge{
			Value: proto.Float64(sample.Value),
		},
		Label: labelPairsFromLabels(labelNames, labelValues),
	}
	if sample.Timestamp > 0 {
		value.TimestampMs = proto.Int64(sample.Timestamp)
	}

	return newCustomMetric(desc, value)
}

// customMetric implements prometheus.Metric
type customMetric struct {
	desc   *prometheus.Desc
	metric *dto.Metric
}

func newCustomMetric(desc *prometheus.Desc, metric *dto.Metric) prometheus.Metric {
	return &customMetric{
		desc:   desc,
		metric: metric,
	}
}

func (m *customMetric) Desc() *prometheus.Desc {
	return m.desc
}

func (m *customMetric) Write(out *dto.Metric) error {
	out.Label = m.metric.Label
	out.Gauge = m.metric.Gauge
	out.TimestampMs = m.metric.TimestampMs
	return nil
}

// labelPairsFromLabels creates label pairs for the metric
func labelPairsFromLabels(names, values []string) []*dto.LabelPair {
	pairs := make([]*dto.LabelPair, len(names))
	for i := range names {
		pairs[i] = &dto.LabelPair{
			Name:  proto.String(names[i]),
			Value: proto.String(values[i]),
		}
	}
	return pairs
}
