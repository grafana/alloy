package remotewrite

import (
	"context"
	"errors"
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
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
	h := &histogram.Histogram{}
	ex := exemplar.Exemplar{Labels: labels.FromStrings("trace_id", "abc"), Value: 1, Ts: 123}

	// Each case appends the same data through the V1 appender and through the
	// V2 appender.
	for _, hook := range []struct {
		name    string
		v1      func(storage.Appender, storage.SeriesRef) (storage.SeriesRef, error)
		v2      func(storage.AppenderV2, storage.SeriesRef) (storage.SeriesRef, error)
		wantErr string
	}{
		{
			name: "sample",
			v1: func(app storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return app.Append(ref, metricLabels, 123, 1)
			},
			v2: func(app storage.AppenderV2, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return app.Append(ref, metricLabels, 0, 123, 1, nil, nil, storage.AppendV2Options{})
			},
		},
		{
			name: "histogram",
			v1: func(app storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return app.AppendHistogram(ref, metricLabels, 123, h, nil)
			},
			v2: func(app storage.AppenderV2, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return app.Append(ref, metricLabels, 0, 123, 0, h, nil, storage.AppendV2Options{})
			},
		},
		{
			name: "start timestamp zero sample",
			v1: func(app storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return app.AppendSTZeroSample(ref, metricLabels, 123, 100)
			},
			v2: func(app storage.AppenderV2, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return app.Append(ref, metricLabels, 100, 123, 1, nil, nil, storage.AppendV2Options{})
			},
		},
		{
			name: "histogram start timestamp zero sample",
			v1: func(app storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return app.AppendHistogramSTZeroSample(ref, metricLabels, 123, 100, h, nil)
			},
			v2: func(app storage.AppenderV2, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return app.Append(ref, metricLabels, 100, 123, 0, h, nil, storage.AppendV2Options{})
			},
		},
		// The next appender rejects every exemplar, but the sample is still
		// appended, so the series is linked like for any other sample.
		{
			name: "sample with a rejected exemplar",
			v1: func(app storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
				ref, err := app.Append(ref, metricLabels, 123, 1)
				if err != nil {
					return ref, err
				}
				_, err = app.AppendExemplar(ref, metricLabels, ex)
				return ref, err
			},
			v2: func(app storage.AppenderV2, ref storage.SeriesRef) (storage.SeriesRef, error) {
				return app.Append(ref, metricLabels, 0, 123, 1, nil, nil, storage.AppendV2Options{Exemplars: []exemplar.Exemplar{ex}})
			},
			wantErr: errRejectedExemplar.Error(),
		},
	} {
		// TODO: Remove V1 when the V1 appenders are removed.
		for _, version := range []string{"V1", "V2"} {
			// appendTo makes the append of the case through the appender of
			// version.
			appendTo := func(i *prometheus.Interceptor, ref storage.SeriesRef) (storage.SeriesRef, error) {
				if version == "V1" {
					return hook.v1(i.Appender(t.Context()), ref)
				}
				return hook.v2(i.AppenderV2(t.Context()), ref)
			}

			t.Run(hook.name+"/"+version, func(t *testing.T) {
				requireAppendErr := func(t *testing.T, err error) {
					if hook.wantErr == "" {
						require.NoError(t, err)
					} else {
						require.ErrorContains(t, err, hook.wantErr)
					}
				}

				// newInterceptor returns an interceptor whose appends are
				// recorded by a refRecorder that returns localRef.
				newInterceptor := func(ls labelstore.LabelStore, localRef storage.SeriesRef) (*prometheus.Interceptor, *refRecorder) {
					rec := &refRecorder{returnRef: localRef}
					store := interceptorTestStorage{appender: refRecordingAppender{refRecorder: rec}, appenderV2: refRecordingAppenderV2{refRecorder: rec}}
					return NewInterceptor(componentID, &atomic.Bool{}, livedebugging.NewLiveDebugging(), ls, store), rec
				}

				// Global and local refs are counted separately, so passing the
				// global ref through would make the next appender resolve it
				// to a different series.
				t.Run("linked series", func(t *testing.T) {
					ls := labelstore.New(nil, promclient.NewRegistry())
					globalRef := ls.GetOrAddGlobalRefID(metricLabels)
					ls.AddLocalLink(componentID, globalRef, 42)
					interceptor, child := newInterceptor(ls, 42)

					ref, err := appendTo(interceptor, storage.SeriesRef(globalRef))
					requireAppendErr(t, err)
					require.Equal(t, storage.SeriesRef(42), child.gotRef)
					require.Equal(t, storage.SeriesRef(globalRef), ref)
				})

				// A series with no local link is passed on with ref 0, so the
				// next appender looks it up by labels, and the ref it returns
				// is linked.
				t.Run("unlinked series", func(t *testing.T) {
					ls := labelstore.New(nil, promclient.NewRegistry())
					globalRef := ls.GetOrAddGlobalRefID(metricLabels)
					interceptor, child := newInterceptor(ls, 7)

					ref, err := appendTo(interceptor, storage.SeriesRef(globalRef))
					requireAppendErr(t, err)
					require.Zero(t, child.gotRef)
					require.Equal(t, storage.SeriesRef(globalRef), ref)
					require.Equal(t, uint64(7), ls.GetLocalRefID(componentID, globalRef))
				})

				t.Run("errors once exited", func(t *testing.T) {
					store := interceptorTestStorage{appender: appenders.Noop{}, appenderV2: appenders.NoopV2{}}
					interceptor := NewInterceptor(componentID, atomic.NewBool(true), livedebugging.NewLiveDebugging(), labelstore.New(nil, promclient.NewRegistry()), store)

					_, err := appendTo(interceptor, 0)
					require.ErrorContains(t, err, "has exited")
				})
			})
		}
	}
}

// errRejectedExemplar is returned by the refRecorder appenders for every
// exemplar.
var errRejectedExemplar = errors.New("rejected exemplar")

// refRecorder records the ref of the first append it gets, and returns
// returnRef from every append.
//
// Only the first ref is recorded, because a V2 append with a start timestamp
// that reaches the V1 hooks through the adapter becomes two appends. The first
// one creates the local link, so the second one already gets the local ref.
type refRecorder struct {
	returnRef storage.SeriesRef
	gotRef    storage.SeriesRef
	recorded  bool
}

func (r *refRecorder) record(ref storage.SeriesRef) (storage.SeriesRef, error) {
	if !r.recorded {
		r.gotRef, r.recorded = ref, true
	}
	return r.returnRef, nil
}

// refRecordingAppender passes the refs of its sample appends to a refRecorder,
// and rejects every exemplar.
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

func (a refRecordingAppender) AppendExemplar(storage.SeriesRef, labels.Labels, exemplar.Exemplar) (storage.SeriesRef, error) {
	return 0, errRejectedExemplar
}

func (a refRecordingAppender) AppendSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	return a.record(ref)
}

func (a refRecordingAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return a.record(ref)
}

// refRecordingAppenderV2 passes the refs of its appends to a refRecorder, and
// rejects every exemplar with a partial error, like the V1 adapter does.
type refRecordingAppenderV2 struct {
	appenders.NoopV2
	*refRecorder
}

func (a refRecordingAppenderV2) Append(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ float64, _ *histogram.Histogram, _ *histogram.FloatHistogram, opts storage.AppendV2Options) (storage.SeriesRef, error) {
	newRef, _ := a.record(ref)
	if len(opts.Exemplars) == 0 {
		return newRef, nil
	}
	partialErr := &storage.AppendPartialError{}
	for range opts.Exemplars {
		partialErr.ExemplarErrors = append(partialErr.ExemplarErrors, errRejectedExemplar)
	}
	return newRef, partialErr
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
