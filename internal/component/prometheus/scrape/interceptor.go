package scrape

import (
	"fmt"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

// NewInterceptor creates a new Prometheus storage.Appendable interceptor proxies calls to the provided appendable publishing
// live debugging data using the provided debugDataPublisher if live debugging is active.
func NewInterceptor(componentID livedebugging.ComponentID, ls labelstore.LabelStore, debugDataPublisher livedebugging.DebugDataPublisher, appendable storage.Appendable) *prometheus.Interceptor {
	return prometheus.NewInterceptor(appendable, ls,
		prometheus.WithAppendHook(func(globalRef storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error) {
			_, nextErr := next.Append(globalRef, l, t, v)
			debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.PrometheusMetric,
				1,
				func() string {
					return fmt.Sprintf("sample: ts=%d, labels=%s, value=%f", t, l, v)
				},
			))
			return globalRef, nextErr
		}),
		prometheus.WithHistogramHook(func(globalRef storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, next storage.Appender) (storage.SeriesRef, error) {
			_, nextErr := next.AppendHistogram(globalRef, l, t, h, fh)
			debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.PrometheusMetric,
				1,
				func() string {
					var data string
					if h != nil {
						data = fmt.Sprintf("histogram: ts=%d, labels=%s, value=%s", t, l, h.String())
					} else if fh != nil {
						data = fmt.Sprintf("float_histogram: ts=%d, labels=%s, value=%s", t, l, fh.String())
					} else {
						data = fmt.Sprintf("histogram_with_no_value: ts=%d, labels=%s", t, l)
					}
					return data
				},
			))
			return globalRef, nextErr
		}),
		prometheus.WithMetadataHook(func(globalRef storage.SeriesRef, l labels.Labels, m metadata.Metadata, next storage.Appender) (storage.SeriesRef, error) {
			_, nextErr := next.UpdateMetadata(globalRef, l, m)
			debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.PrometheusMetric,
				1,
				func() string {
					return fmt.Sprintf("metadata: labels=%s, type=%q, unit=%q, help=%q", l, m.Type, m.Unit, m.Help)
				},
			))
			return globalRef, nextErr
		}),
		prometheus.WithExemplarHook(func(globalRef storage.SeriesRef, l labels.Labels, e exemplar.Exemplar, next storage.Appender) (storage.SeriesRef, error) {
			_, nextErr := next.AppendExemplar(globalRef, l, e)
			debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.PrometheusMetric,
				1,
				func() string {
					return fmt.Sprintf("exemplar: ts=%d, labels=%s, exemplar_labels=%s, value=%f", e.Ts, l, e.Labels, e.Value)
				},
			))
			return globalRef, nextErr
		}),
	)
}
