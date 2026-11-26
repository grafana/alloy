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
		store := interceptorTestStorage{appender: appenders.Noop{}}
		return NewInterceptor(componentID, &atomic.Bool{}, publisher, labelstore.New(nil, promclient.NewRegistry()), store)
	})
}

func TestInterceptorSTZeroSampleHooksUseLocalRef(t *testing.T) {
	const componentID = "prometheus.remote_write.test"
	metricLabels := labels.FromStrings("__name__", "test_metric")

	for _, hook := range []struct {
		name   string
		append func(storage.Appender, storage.SeriesRef) (storage.SeriesRef, error)
	}{
		{
			name: "st_zero_sample",
			append: func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return appender.AppendSTZeroSample(ref, metricLabels, 123, 100)
			},
		},
		{
			name: "histogram_st_zero_sample",
			append: func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return appender.AppendHistogramSTZeroSample(ref, metricLabels, 123, 100, &histogram.Histogram{}, nil)
			},
		},
	} {
		t.Run(hook.name, func(t *testing.T) {
			// Global and local refs are counted separately, so passing the
			// global ref through would make the next appender resolve it to
			// a different series.
			t.Run("linked series", func(t *testing.T) {
				ls := labelstore.New(nil, promclient.NewRegistry())
				globalRef := ls.GetOrAddGlobalRefID(metricLabels)
				ls.AddLocalLink(componentID, globalRef, 42)
				child := &refRecordingAppender{returnRef: 42}
				interceptor := NewInterceptor(componentID, &atomic.Bool{}, livedebugging.NewLiveDebugging(), ls, interceptorTestStorage{appender: child})

				ref, err := hook.append(interceptor.Appender(t.Context()), storage.SeriesRef(globalRef))
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
				child := &refRecordingAppender{returnRef: 7}
				interceptor := NewInterceptor(componentID, &atomic.Bool{}, livedebugging.NewLiveDebugging(), ls, interceptorTestStorage{appender: child})

				ref, err := hook.append(interceptor.Appender(t.Context()), storage.SeriesRef(globalRef))
				require.NoError(t, err)
				require.Zero(t, child.gotRef)
				require.Equal(t, storage.SeriesRef(globalRef), ref)
				require.Equal(t, uint64(7), ls.GetLocalRefID(componentID, globalRef))
			})
		})
	}
}

// refRecordingAppender records the ref passed to the ST zero sample methods
// and returns returnRef from them.
type refRecordingAppender struct {
	appenders.Noop

	returnRef storage.SeriesRef
	gotRef    storage.SeriesRef
}

func (a *refRecordingAppender) AppendSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	a.gotRef = ref
	return a.returnRef, nil
}

func (a *refRecordingAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	a.gotRef = ref
	return a.returnRef, nil
}

type interceptorTestStorage struct {
	storage.Queryable
	storage.ChunkQueryable

	appender storage.Appender
}

func (s interceptorTestStorage) Appender(context.Context) storage.Appender {
	return s.appender
}

func (interceptorTestStorage) AppenderV2(_ context.Context) storage.AppenderV2 {
	panic("AppenderV2 not implemented")
}

func (interceptorTestStorage) StartTime() (int64, error) {
	return 0, nil
}

func (interceptorTestStorage) Close() error {
	return nil
}
