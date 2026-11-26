// Package livedebuggingtest tests the live debugging data that the
// interceptors of Prometheus components publish.
package livedebuggingtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

// RunInterceptorTests appends each kind of data through the interceptor that
// newInterceptor returns. It checks that the interceptor publishes the
// expected live debugging data when live debugging is active for
// componentID, and that it publishes and allocates nothing when it isn't.
func RunInterceptorTests(t *testing.T, componentID livedebugging.ComponentID, newInterceptor func(livedebugging.DebugDataPublisher) *prometheus.Interceptor) {
	for _, tc := range cases() {
		for _, active := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/active=%t", tc.name, active), func(t *testing.T) {
				publisher := &publisherSpy{active: active}
				appendOnce := tc.appendFunc(t.Context(), newInterceptor(publisher))

				require.NoError(t, appendOnce())
				require.Equal(t, 1, publisher.isActiveCalls)
				require.Equal(t, componentID, publisher.isActiveComponentID)

				if !active {
					require.Empty(t, publisher.published)
					var err error
					allocs := testing.AllocsPerRun(100, func() { err = appendOnce() })
					require.NoError(t, err)
					require.Zero(t, allocs)
					return
				}

				got := make([]string, 0, len(publisher.published))
				for _, data := range publisher.published {
					require.Equal(t, componentID, data.ComponentID)
					require.Equal(t, livedebugging.PrometheusMetric, data.Type)
					require.Equal(t, uint64(1), data.Count)
					got = append(got, data.DataFunc())
				}
				require.Equal(t, tc.want, got)
			})
		}
	}
}

// testCase is one append and the live debugging data it publishes. Exactly
// one of v1 and v2 is set.
//
// TODO: Remove the V1 cases when the V1 appenders are removed.
type testCase struct {
	name string
	v1   func(storage.Appender) (storage.SeriesRef, error)
	v2   func(storage.AppenderV2) (storage.SeriesRef, error)
	want []string
}

// appendFunc returns a function that runs the append of the test case. The
// appender is created up front, so that only the append counts towards
// allocations.
func (tc testCase) appendFunc(ctx context.Context, i *prometheus.Interceptor) func() error {
	if tc.v1 != nil {
		app := i.Appender(ctx)
		return func() error {
			_, err := tc.v1(app)
			return err
		}
	}
	app := i.AppenderV2(ctx)
	return func() error {
		_, err := tc.v2(app)
		return err
	}
}

func cases() []testCase {
	lbls := labels.FromStrings("__name__", "test_metric")
	meta := metadata.Metadata{Type: "gauge", Unit: "items", Help: "test metric"}
	exemplar1 := exemplar.Exemplar{Labels: labels.FromStrings("trace_id", "abc"), Value: 12.5, Ts: 123}
	exemplar2 := exemplar.Exemplar{Labels: labels.FromStrings("trace_id", "def"), Value: 13.5, Ts: 124}
	// Built once, so that building it doesn't count towards allocations.
	withExemplarsAndMetadata := storage.AppendV2Options{Metadata: meta, Exemplars: []exemplar.Exemplar{exemplar1, exemplar2}}
	h := &histogram.Histogram{Schema: 1}
	fh := &histogram.FloatHistogram{Schema: 1}

	const (
		wantSample    = `sample: ts=123, labels={__name__="test_metric"}, value=12.500000`
		wantMetadata  = `metadata: labels={__name__="test_metric"}, type="gauge", unit="items", help="test metric"`
		wantExemplar1 = `exemplar: ts=123, labels={__name__="test_metric"}, exemplar_labels={trace_id="abc"}, value=12.500000`
		wantExemplar2 = `exemplar: ts=124, labels={__name__="test_metric"}, exemplar_labels={trace_id="def"}, value=13.500000`
	)
	wantHistogram := `histogram: ts=123, labels={__name__="test_metric"}, value=` + h.String()
	wantFloatHistogram := `float_histogram: ts=123, labels={__name__="test_metric"}, value=` + fh.String()

	return []testCase{
		{
			name: "V1 sample",
			v1: func(app storage.Appender) (storage.SeriesRef, error) {
				return app.Append(0, lbls, 123, 12.5)
			},
			want: []string{wantSample},
		},
		{
			name: "V1 histogram",
			v1: func(app storage.Appender) (storage.SeriesRef, error) {
				return app.AppendHistogram(0, lbls, 123, h, nil)
			},
			want: []string{wantHistogram},
		},
		{
			name: "V1 float histogram",
			v1: func(app storage.Appender) (storage.SeriesRef, error) {
				return app.AppendHistogram(0, lbls, 123, nil, fh)
			},
			want: []string{wantFloatHistogram},
		},
		{
			name: "V1 histogram without a value",
			v1: func(app storage.Appender) (storage.SeriesRef, error) {
				return app.AppendHistogram(0, lbls, 123, nil, nil)
			},
			want: []string{`histogram_with_no_value: ts=123, labels={__name__="test_metric"}`},
		},
		{
			name: "V1 exemplar",
			v1: func(app storage.Appender) (storage.SeriesRef, error) {
				return app.AppendExemplar(0, lbls, exemplar1)
			},
			want: []string{wantExemplar1},
		},
		{
			name: "V1 metadata",
			v1: func(app storage.Appender) (storage.SeriesRef, error) {
				return app.UpdateMetadata(0, lbls, meta)
			},
			want: []string{wantMetadata},
		},
		{
			name: "V2 sample",
			v2: func(app storage.AppenderV2) (storage.SeriesRef, error) {
				return app.Append(0, lbls, 0, 123, 12.5, nil, nil, storage.AppendV2Options{})
			},
			want: []string{wantSample},
		},
		{
			name: "V2 histogram",
			v2: func(app storage.AppenderV2) (storage.SeriesRef, error) {
				return app.Append(0, lbls, 0, 123, 0, h, nil, storage.AppendV2Options{})
			},
			want: []string{wantHistogram},
		},
		{
			name: "V2 float histogram",
			v2: func(app storage.AppenderV2) (storage.SeriesRef, error) {
				return app.Append(0, lbls, 0, 123, 0, nil, fh, storage.AppendV2Options{})
			},
			want: []string{wantFloatHistogram},
		},
		{
			// V2 gets the exemplars and metadata of a sample in the same call.
			name: "V2 sample with exemplars and metadata",
			v2: func(app storage.AppenderV2) (storage.SeriesRef, error) {
				return app.Append(0, lbls, 0, 123, 12.5, nil, nil, withExemplarsAndMetadata)
			},
			want: []string{wantSample, wantMetadata, wantExemplar1, wantExemplar2},
		},
	}
}

// publisherSpy is a livedebugging.DebugDataPublisher that records how it's
// called.
type publisherSpy struct {
	active              bool
	isActiveCalls       int
	isActiveComponentID livedebugging.ComponentID
	published           []livedebugging.Data
}

func (p *publisherSpy) IsActive(componentID livedebugging.ComponentID) bool {
	p.isActiveCalls++
	p.isActiveComponentID = componentID
	return p.active
}

func (p *publisherSpy) PublishIfActive(data livedebugging.Data) {
	p.published = append(p.published, data)
}
