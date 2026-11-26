package remotewrite

import (
	"context"
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/component/prometheus/livedebuggingtest"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

func TestInterceptorDebuggingRespectsActivity(t *testing.T) {
	const componentID = "prometheus.remote_write.test"
	livedebuggingtest.RunInterceptorTests(t, componentID, func(publisher livedebugging.DebugDataPublisher) *prometheus.Interceptor {
		store := interceptorTestStorage{appender: appenders.Noop{}, appenderV2: appenders.NoopV2{}}
		return NewInterceptor(componentID, &atomic.Bool{}, publisher, labelstore.New(nil, promclient.NewRegistry()), store)
	})
}

// TestInterceptorAppendHooks checks each hook that appends a sample.
func TestInterceptorAppendHooks(t *testing.T) {
	const componentID = "prometheus.remote_write.test"
	metricLabels := labels.FromStrings("__name__", "test_metric")

	for _, hook := range []struct {
		name   string
		append func(*prometheus.Interceptor, storage.SeriesRef) (storage.SeriesRef, error)
	}{
		{
			name: "V1 sample",
			append: func(i *prometheus.Interceptor, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return i.Appender(t.Context()).Append(ref, metricLabels, 123, 1)
			},
		},
		{
			name: "V1 histogram",
			append: func(i *prometheus.Interceptor, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return i.Appender(t.Context()).AppendHistogram(ref, metricLabels, 123, &histogram.Histogram{}, nil)
			},
		},
		{
			name: "V1 start timestamp zero sample",
			append: func(i *prometheus.Interceptor, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return i.Appender(t.Context()).AppendSTZeroSample(ref, metricLabels, 123, 100)
			},
		},
		{
			name: "V1 histogram start timestamp zero sample",
			append: func(i *prometheus.Interceptor, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return i.Appender(t.Context()).AppendHistogramSTZeroSample(ref, metricLabels, 123, 100, &histogram.Histogram{}, nil)
			},
		},
		{
			name: "V2 sample",
			append: func(i *prometheus.Interceptor, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return i.AppenderV2(t.Context()).Append(ref, metricLabels, 0, 123, 1, nil, nil, storage.AppendV2Options{})
			},
		},
	} {
		t.Run(hook.name, func(t *testing.T) {
			// newInterceptor returns an interceptor whose appends are recorded
			// by a refRecorder that returns localRef.
			newInterceptor := func(ls labelstore.LabelStore, localRef storage.SeriesRef) (*prometheus.Interceptor, *refRecorder) {
				rec := &refRecorder{returnRef: localRef}
				store := interceptorTestStorage{appender: refRecordingAppender{refRecorder: rec}, appenderV2: refRecordingAppenderV2{refRecorder: rec}}
				return NewInterceptor(componentID, &atomic.Bool{}, livedebugging.NewLiveDebugging(), ls, store), rec
			}

			// Global and local refs are counted separately, so passing the
			// global ref through would make the next appender resolve it to
			// a different series.
			t.Run("linked series", func(t *testing.T) {
				ls := labelstore.New(nil, promclient.NewRegistry())
				globalRef := ls.GetOrAddGlobalRefID(metricLabels)
				ls.AddLocalLink(componentID, globalRef, 42)
				interceptor, child := newInterceptor(ls, 42)

				ref, err := hook.append(interceptor, storage.SeriesRef(globalRef))
				require.NoError(t, err)
				require.Equal(t, storage.SeriesRef(42), child.gotRef)
				require.Equal(t, storage.SeriesRef(globalRef), ref)
			})

			// A series with no local link is passed on with ref 0, so the
			// next appender looks it up by labels, and the ref it returns is
			// linked.
			t.Run("unlinked series", func(t *testing.T) {
				ls := labelstore.New(nil, promclient.NewRegistry())
				globalRef := ls.GetOrAddGlobalRefID(metricLabels)
				interceptor, child := newInterceptor(ls, 7)

				ref, err := hook.append(interceptor, storage.SeriesRef(globalRef))
				require.NoError(t, err)
				require.Zero(t, child.gotRef)
				require.Equal(t, storage.SeriesRef(globalRef), ref)
				require.Equal(t, uint64(7), ls.GetLocalRefID(componentID, globalRef))
			})

			t.Run("errors once exited", func(t *testing.T) {
				store := interceptorTestStorage{appender: appenders.Noop{}, appenderV2: appenders.NoopV2{}}
				interceptor := NewInterceptor(componentID, atomic.NewBool(true), livedebugging.NewLiveDebugging(), labelstore.New(nil, promclient.NewRegistry()), store)

				_, err := hook.append(interceptor, 0)
				require.ErrorContains(t, err, "has exited")
			})
		})
	}
}

// refRecorder records the ref of the last append it gets, and returns
// returnRef from it.
type refRecorder struct {
	returnRef storage.SeriesRef
	gotRef    storage.SeriesRef
}

func (r *refRecorder) record(ref storage.SeriesRef) (storage.SeriesRef, error) {
	r.gotRef = ref
	return r.returnRef, nil
}

// refRecordingAppender passes the refs of its sample appends to a refRecorder.
type refRecordingAppender struct {
	appenders.Noop
	*refRecorder
}

func (a refRecordingAppender) Append(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
	return a.record(ref)
}

func (a refRecordingAppender) AppendHistogram(ref storage.SeriesRef, _ labels.Labels, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return a.record(ref)
}

func (a refRecordingAppender) AppendSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	return a.record(ref)
}

func (a refRecordingAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return a.record(ref)
}

// refRecordingAppenderV2 passes the refs of its appends to a refRecorder.
type refRecordingAppenderV2 struct {
	appenders.NoopV2
	*refRecorder
}

func (a refRecordingAppenderV2) Append(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ float64, _ *histogram.Histogram, _ *histogram.FloatHistogram, _ storage.AppendV2Options) (storage.SeriesRef, error) {
	return a.record(ref)
}

type interceptorTestStorage struct {
	storage.Queryable
	storage.ChunkQueryable

	appender   storage.Appender
	appenderV2 storage.AppenderV2
}

func (s interceptorTestStorage) Appender(context.Context) storage.Appender {
	return s.appender
}

func (s interceptorTestStorage) AppenderV2(context.Context) storage.AppenderV2 {
	return s.appenderV2
}

func (interceptorTestStorage) StartTime() (int64, error) {
	return 0, nil
}

func (interceptorTestStorage) Close() error {
	return nil
}
