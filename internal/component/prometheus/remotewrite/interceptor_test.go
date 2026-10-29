package remotewrite

import (
	"context"
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

// debugCase is one scenario for the live debugging tests.
type debugCase struct {
	name   string
	v1     func(storage.Appender) error
	v1Want []string
}

func debugCases() []debugCase {
	metricLabels := labels.FromStrings("__name__", "test_metric")
	metricMetadata := metadata.Metadata{Type: "gauge", Unit: "items", Help: "test metric"}
	metricExemplar := exemplar.Exemplar{
		Labels: labels.FromStrings("trace_id", "abc"),
		Value:  12.5,
		Ts:     123,
	}
	h := &histogram.Histogram{Schema: 1}
	fh := &histogram.FloatHistogram{Schema: 1}

	const (
		wantSample   = `sample: ts=123, labels={__name__="test_metric"}, value=12.500000`
		wantMetadata = `metadata: labels={__name__="test_metric"}, type="gauge", unit="items", help="test metric"`
		wantExemplar = `exemplar: ts=123, labels={__name__="test_metric"}, exemplar_labels={trace_id="abc"}, value=12.500000`
		wantLabels   = `{__name__="test_metric"}`
	)
	return []debugCase{
		{
			name: "sample",
			v1: func(app storage.Appender) error {
				_, err := app.Append(0, metricLabels, 123, 12.5)
				return err
			},
			v1Want: []string{wantSample},
		},
		{
			name: "histogram",
			v1: func(app storage.Appender) error {
				_, err := app.AppendHistogram(0, metricLabels, 123, h, nil)
				return err
			},
			v1Want: []string{"histogram: ts=123, labels=" + wantLabels + ", value=" + h.String()},
		},
		{
			name: "float histogram",
			v1: func(app storage.Appender) error {
				_, err := app.AppendHistogram(0, metricLabels, 123, nil, fh)
				return err
			},
			v1Want: []string{"float_histogram: ts=123, labels=" + wantLabels + ", value=" + fh.String()},
		},
		{
			name: "histogram without a value",
			v1: func(app storage.Appender) error {
				_, err := app.AppendHistogram(0, metricLabels, 123, nil, nil)
				return err
			},
			v1Want: []string{`histogram_with_no_value: ts=123, labels={__name__="test_metric"}`},
		},
		{
			name: "exemplar",
			v1: func(app storage.Appender) error {
				_, err := app.AppendExemplar(0, metricLabels, metricExemplar)
				return err
			},
			v1Want: []string{wantExemplar},
		},
		{
			name: "metadata",
			v1: func(app storage.Appender) error {
				_, err := app.UpdateMetadata(0, metricLabels, metricMetadata)
				return err
			},
			v1Want: []string{wantMetadata},
		},
	}
}

func TestInterceptorDebuggingRespectsActivity(t *testing.T) {
	componentID := livedebugging.ComponentID("prometheus.remote_write.test")

	for _, tc := range debugCases() {
		t.Run(tc.name, func(t *testing.T) {
			// run builds an interceptor, calls the appender once, and checks that
			// the published data is want when active and that nothing is published
			// or allocated when not active.
			run := func(t *testing.T, want []string, newAppend func(*prometheus.Interceptor) func() error) {
				for _, active := range []bool{false, true} {
					t.Run(map[bool]string{false: "inactive", true: "active"}[active], func(t *testing.T) {
						publisher := &debugDataPublisherSpy{active: active}
						appendOnce := newAppend(NewInterceptor(
							string(componentID),
							&atomic.Bool{},
							publisher,
							labelstore.New(nil, promclient.NewRegistry()),
							interceptorTestStorage{appender: appenders.Noop{}},
						))

						require.NoError(t, appendOnce())
						require.Equal(t, 1, publisher.isActiveCalls)
						require.Equal(t, componentID, publisher.isActiveComponentID)

						if active {
							require.Len(t, publisher.publishedData, len(want))
							for i, data := range publisher.publishedData {
								require.Equal(t, componentID, data.ComponentID)
								require.Equal(t, livedebugging.PrometheusMetric, data.Type)
								require.Equal(t, uint64(1), data.Count)
								require.Equal(t, want[i], data.DataFunc())
							}
							return
						}

						require.Zero(t, publisher.publishCalls)
						var allocErr error
						allocs := testing.AllocsPerRun(100, func() {
							allocErr = appendOnce()
						})
						require.NoError(t, allocErr)
						require.Zero(t, allocs)
					})
				}
			}

			if tc.v1 != nil {
				t.Run("V1", func(t *testing.T) {
					run(t, tc.v1Want, func(i *prometheus.Interceptor) func() error {
						app := i.Appender(t.Context())
						return func() error { return tc.v1(app) }
					})
				})
			}
		})
	}
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
				interceptor := NewInterceptor(componentID, &atomic.Bool{}, &debugDataPublisherSpy{}, ls, interceptorTestStorage{appender: child})

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
				interceptor := NewInterceptor(componentID, &atomic.Bool{}, &debugDataPublisherSpy{}, ls, interceptorTestStorage{appender: child})

				ref, err := hook.append(interceptor.Appender(t.Context()), storage.SeriesRef(globalRef))
				require.NoError(t, err)
				require.Zero(t, child.gotRef)
				require.Equal(t, storage.SeriesRef(globalRef), ref)
				require.Equal(t, uint64(7), ls.GetLocalRefID(componentID, globalRef))
			})
		})
	}
}

type debugDataPublisherSpy struct {
	active        bool
	isActiveCalls int
	publishCalls  int

	isActiveComponentID livedebugging.ComponentID
	publishedData       []livedebugging.Data
}

func (p *debugDataPublisherSpy) IsActive(componentID livedebugging.ComponentID) bool {
	p.isActiveCalls++
	p.isActiveComponentID = componentID
	return p.active
}

func (p *debugDataPublisherSpy) PublishIfActive(data livedebugging.Data) {
	p.publishCalls++
	p.publishedData = append(p.publishedData, data)
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
