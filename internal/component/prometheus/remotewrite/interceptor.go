package remotewrite

import (
	"fmt"
	"sync/atomic"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

func NewInterceptor(componentID string, exited *atomic.Bool, debugDataPublisher livedebugging.DebugDataPublisher, ls labelstore.LabelStore, store storage.Storage) *prometheus.Interceptor {
	liveDebuggingComponentID := livedebugging.ComponentID(componentID)

	handleLocalLink := func(globalRef uint64, l labels.Labels, cachedLocalRef uint64, newLocalRef uint64) {
		// We had a local ref that was still valid nothing to do
		if cachedLocalRef != 0 && cachedLocalRef == newLocalRef {
			return
		}

		// There are some unique scenarios that can have an append end with no error but the returned localRef is zero (duplicate exemplars).
		// We don't want to update a valid link to an invalid link
		if cachedLocalRef != 0 && newLocalRef == 0 {
			return
		}

		// This should never happen in a proper appender chain. Since we cannot enforce it, we are extra defensive.
		if globalRef == 0 {
			globalRef = ls.GetOrAddGlobalRefID(l)
		}

		if cachedLocalRef == 0 {
			ls.AddLocalLink(componentID, globalRef, newLocalRef)
		} else {
			ls.ReplaceLocalLink(componentID, globalRef, cachedLocalRef, newLocalRef)
		}
	}

	return prometheus.NewInterceptor(
		store, ls,
		prometheus.WithComponentID(componentID),
		// In the methods below, conversion is needed because remote_writes assume
		// they are responsible for generating ref IDs. This means two
		// remote_writes may return the same ref ID for two different series. We
		// treat the remote_write ID as a "local ID" and translate it to a "global
		// ID" to ensure Alloy compatibility.

		prometheus.WithAppendHook(func(globalRef storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error) {
			if exited.Load() {
				return 0, fmt.Errorf("%s has exited", componentID)
			}

			localRef := ls.GetLocalRefID(componentID, uint64(globalRef))
			newLocalRef, nextErr := next.Append(storage.SeriesRef(localRef), l, t, v)
			if nextErr == nil {
				handleLocalLink(uint64(globalRef), l, localRef, uint64(newLocalRef))
			}

			debugDataPublisher.PublishIfActive(livedebugging.NewData(
				liveDebuggingComponentID,
				livedebugging.PrometheusMetric,
				1,
				func() string {
					return fmt.Sprintf("sample: ts=%d, labels=%s, value=%f", t, l, v)
				},
			))
			return globalRef, nextErr
		}),
		prometheus.WithHistogramHook(func(globalRef storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, next storage.Appender) (storage.SeriesRef, error) {
			if exited.Load() {
				return 0, fmt.Errorf("%s has exited", componentID)
			}

			localRef := ls.GetLocalRefID(componentID, uint64(globalRef))
			newLocalRef, nextErr := next.AppendHistogram(storage.SeriesRef(localRef), l, t, h, fh)
			if nextErr == nil {
				handleLocalLink(uint64(globalRef), l, localRef, uint64(newLocalRef))
			}

			debugDataPublisher.PublishIfActive(livedebugging.NewData(
				liveDebuggingComponentID,
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
			if exited.Load() {
				return 0, fmt.Errorf("%s has exited", componentID)
			}

			localRef := ls.GetLocalRefID(componentID, uint64(globalRef))
			newLocalRef, nextErr := next.UpdateMetadata(storage.SeriesRef(localRef), l, m)
			if nextErr == nil {
				handleLocalLink(uint64(globalRef), l, localRef, uint64(newLocalRef))
			}

			debugDataPublisher.PublishIfActive(livedebugging.NewData(
				liveDebuggingComponentID,
				livedebugging.PrometheusMetric,
				1,
				func() string {
					return fmt.Sprintf("metadata: labels=%s, type=%q, unit=%q, help=%q", l, m.Type, m.Unit, m.Help)
				},
			))
			return globalRef, nextErr
		}),
		prometheus.WithExemplarHook(func(globalRef storage.SeriesRef, l labels.Labels, e exemplar.Exemplar, next storage.Appender) (storage.SeriesRef, error) {
			if exited.Load() {
				return 0, fmt.Errorf("%s has exited", componentID)
			}

			localRef := ls.GetLocalRefID(componentID, uint64(globalRef))
			newLocalRef, nextErr := next.AppendExemplar(storage.SeriesRef(localRef), l, e)
			if nextErr == nil {
				handleLocalLink(uint64(globalRef), l, localRef, uint64(newLocalRef))
			}

			debugDataPublisher.PublishIfActive(livedebugging.NewData(
				liveDebuggingComponentID,
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
