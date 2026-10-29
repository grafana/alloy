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

// debugCase is one scenario for the live debugging tests. A nil v1 or v2
// means that the appender version has no equivalent of the scenario.
//
// TODO: Remove the v1 fields and the V1 subtests when the V1 appenders are removed.
type debugCase struct {
	name   string
	v1     func(storage.Appender) error
	v1Want []string
	v2     func(storage.AppenderV2) error
	v2Want []string
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
	appendV2 := func(h *histogram.Histogram, fh *histogram.FloatHistogram, opts storage.AppendV2Options) func(storage.AppenderV2) error {
		return func(app storage.AppenderV2) error {
			_, err := app.Append(0, metricLabels, 0, 123, 12.5, h, fh, opts)
			return err
		}
	}

	return []debugCase{
		{
			name: "sample",
			v1: func(app storage.Appender) error {
				_, err := app.Append(0, metricLabels, 123, 12.5)
				return err
			},
			v1Want: []string{wantSample},
			v2:     appendV2(nil, nil, storage.AppendV2Options{}),
			v2Want: []string{wantSample},
		},
		{
			name: "histogram",
			v1: func(app storage.Appender) error {
				_, err := app.AppendHistogram(0, metricLabels, 123, h, nil)
				return err
			},
			v1Want: []string{"histogram: ts=123, labels=" + wantLabels + ", value=" + h.String()},
			v2:     appendV2(h, nil, storage.AppendV2Options{}),
			v2Want: []string{"histogram: ts=123, labels=" + wantLabels + ", value=" + h.String()},
		},
		{
			name: "float histogram",
			v1: func(app storage.Appender) error {
				_, err := app.AppendHistogram(0, metricLabels, 123, nil, fh)
				return err
			},
			v1Want: []string{"float_histogram: ts=123, labels=" + wantLabels + ", value=" + fh.String()},
			v2:     appendV2(nil, fh, storage.AppendV2Options{}),
			v2Want: []string{"float_histogram: ts=123, labels=" + wantLabels + ", value=" + fh.String()},
		},
		{
			// V2 can't express a histogram without a value: it's a plain sample.
			name: "histogram without a value",
			v1: func(app storage.Appender) error {
				_, err := app.AppendHistogram(0, metricLabels, 123, nil, nil)
				return err
			},
			v1Want: []string{`histogram_with_no_value: ts=123, labels={__name__="test_metric"}`},
		},
		{
			// In V2 the exemplar comes together with its sample.
			name: "exemplar",
			v1: func(app storage.Appender) error {
				_, err := app.AppendExemplar(0, metricLabels, metricExemplar)
				return err
			},
			v1Want: []string{wantExemplar},
			v2:     appendV2(nil, nil, storage.AppendV2Options{Exemplars: []exemplar.Exemplar{metricExemplar}}),
			v2Want: []string{wantSample, wantExemplar},
		},
		{
			// In V2 the metadata comes together with its sample.
			name: "metadata",
			v1: func(app storage.Appender) error {
				_, err := app.UpdateMetadata(0, metricLabels, metricMetadata)
				return err
			},
			v1Want: []string{wantMetadata},
			v2:     appendV2(nil, nil, storage.AppendV2Options{Metadata: metricMetadata}),
			v2Want: []string{wantSample, wantMetadata},
		},
		{
			name:   "sample with metadata and exemplars",
			v2:     appendV2(nil, nil, storage.AppendV2Options{Metadata: metricMetadata, Exemplars: []exemplar.Exemplar{metricExemplar, metricExemplar}}),
			v2Want: []string{wantSample, wantMetadata, wantExemplar, wantExemplar},
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
							interceptorTestStorage{appender: appenders.Noop{}, appenderV2: appenders.NoopV2{}},
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
			if tc.v2 != nil {
				t.Run("V2", func(t *testing.T) {
					run(t, tc.v2Want, func(i *prometheus.Interceptor) func() error {
						app := i.AppenderV2(t.Context())
						return func() error { return tc.v2(app) }
					})
				})
			}
		})
	}
}

// refRecordingAppenderV2 returns localRef for every append and records the
// refs it was called with.
type refRecordingAppenderV2 struct {
	appenders.NoopV2
	localRef storage.SeriesRef
	gotRefs  []storage.SeriesRef
}

func (a *refRecordingAppenderV2) Append(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ float64, _ *histogram.Histogram, _ *histogram.FloatHistogram, _ storage.AppendV2Options) (storage.SeriesRef, error) {
	a.gotRefs = append(a.gotRefs, ref)
	return a.localRef, nil
}

func TestInterceptorAppendV2TranslatesRefs(t *testing.T) {
	ls := labelstore.New(nil, promclient.NewRegistry(), true)
	metricLabels := labels.FromStrings("__name__", "test_metric")
	globalRef := ls.GetOrAddGlobalRefID(metricLabels)

	next := &refRecordingAppenderV2{localRef: 77}
	interceptor := NewInterceptor(
		"prometheus.remote_write.test",
		&atomic.Bool{},
		&debugDataPublisherSpy{},
		ls,
		interceptorTestStorage{appenderV2: next},
	)
	appender := interceptor.AppenderV2(t.Context())

	for range 2 {
		gotRef, err := appender.Append(storage.SeriesRef(globalRef), metricLabels, 0, 123, 1, nil, nil, storage.AppendV2Options{})
		require.NoError(t, err)
		// The caller always gets the global ref back, never the local one.
		require.Equal(t, storage.SeriesRef(globalRef), gotRef)
	}
	// The first append has no local link yet; the second reuses the one that was created.
	require.Equal(t, []storage.SeriesRef{0, 77}, next.gotRefs)
}

func TestInterceptorAppendV2ErrorsOnceExited(t *testing.T) {
	exited := &atomic.Bool{}
	exited.Store(true)
	interceptor := NewInterceptor(
		"prometheus.remote_write.test",
		exited,
		&debugDataPublisherSpy{},
		labelstore.New(nil, promclient.NewRegistry()),
		interceptorTestStorage{appenderV2: appenders.NoopV2{}},
	)
	_, err := interceptor.AppenderV2(t.Context()).Append(0, labels.FromStrings("a", "b"), 0, 1, 1, nil, nil, storage.AppendV2Options{})
	require.ErrorContains(t, err, "has exited")
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

type interceptorTestStorage struct {
	storage.Queryable
	storage.ChunkQueryable

	appender   storage.Appender
	appenderV2 storage.AppenderV2
}

func (s interceptorTestStorage) Appender(context.Context) storage.Appender {
	return s.appender
}

func (s interceptorTestStorage) AppenderV2(_ context.Context) storage.AppenderV2 {
	if s.appenderV2 == nil {
		panic("AppenderV2 not implemented")
	}
	return s.appenderV2
}

func (interceptorTestStorage) StartTime() (int64, error) {
	return 0, nil
}

func (interceptorTestStorage) Close() error {
	return nil
}
