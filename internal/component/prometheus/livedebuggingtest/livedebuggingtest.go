// Package livedebuggingtest tests the live debugging data that the
// interceptors of Prometheus components publish.
package livedebuggingtest

import (
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
// newInterceptor returns, once through the V1 appender and once through the
// V2 appender. It checks that the interceptor publishes the expected live
// debugging data when live debugging is active for componentID, and that it
// publishes and allocates nothing when it isn't.
func RunInterceptorTests(t *testing.T, componentID livedebugging.ComponentID, newInterceptor func(livedebugging.DebugDataPublisher) *prometheus.Interceptor) {
	// TODO: Remove V1 when the V1 appenders are removed.
	for _, version := range []string{"V1", "V2"} {
		for _, tc := range cases() {
			for _, active := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/active=%t", version, tc.name, active), func(t *testing.T) {
					publisher := &publisherSpy{componentID: componentID, active: active}
					interceptor := newInterceptor(publisher)

					// The appender is created up front, so that only the
					// append counts towards allocations.
					var appendOnce func() error
					if version == "V1" {
						app := interceptor.Appender(t.Context())
						appendOnce = func() error { return tc.v1(app) }
					} else {
						app := interceptor.AppenderV2(t.Context())
						appendOnce = func() error { return tc.v2(app) }
					}

					require.NoError(t, appendOnce())
					require.NotZero(t, publisher.isActiveCalls)
					require.Zero(t, publisher.otherComponentIDs, "IsActive was called for another component")

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
}

// testCase is the same data appended through the V1 appender and through the
// V2 appender, and the live debugging data that both publish.
type testCase struct {
	name string
	v1   func(storage.Appender) error
	v2   func(storage.AppenderV2) error
	want []string
}

func cases() []testCase {
	lbls := labels.FromStrings("__name__", "test_metric")
	h := &histogram.Histogram{Schema: 1}
	fh := &histogram.FloatHistogram{Schema: 1}
	meta := metadata.Metadata{Type: "gauge", Unit: "items", Help: "test metric"}
	exemplars := []exemplar.Exemplar{
		{Labels: labels.FromStrings("trace_id", "abc"), Value: 12.5, Ts: 123},
		{Labels: labels.FromStrings("trace_id", "def"), Value: 13.5, Ts: 124},
	}

	const (
		wantSample    = `sample: ts=123, labels={__name__="test_metric"}, value=12.500000`
		wantMetadata  = `metadata: labels={__name__="test_metric"}, type="gauge", unit="items", help="test metric"`
		wantExemplar1 = `exemplar: ts=123, labels={__name__="test_metric"}, exemplar_labels={trace_id="abc"}, value=12.500000`
		wantExemplar2 = `exemplar: ts=124, labels={__name__="test_metric"}, exemplar_labels={trace_id="def"}, value=13.500000`
	)
	wantHistogram := `histogram: ts=123, labels={__name__="test_metric"}, value=` + h.String()
	wantFloatHistogram := `float_histogram: ts=123, labels={__name__="test_metric"}, value=` + fh.String()

	// appendV1 makes the V1 calls for a sample with value 12.5 at ts 123 and
	// the given exemplars and metadata, in the order that the Prometheus
	// scrape loop makes them.
	appendV1 := func(app storage.Appender, exemplars []exemplar.Exemplar, meta metadata.Metadata) error {
		ref, err := app.Append(0, lbls, 123, 12.5)
		if err != nil {
			return err
		}
		for _, e := range exemplars {
			if _, err := app.AppendExemplar(ref, lbls, e); err != nil {
				return err
			}
		}
		if !meta.IsEmpty() {
			_, err = app.UpdateMetadata(ref, lbls, meta)
		}
		return err
	}
	// appendV2 makes the V2 call for the same sample. The options are built
	// before the test runs, so that building them doesn't count towards
	// allocations.
	appendV2 := func(opts storage.AppendV2Options) func(storage.AppenderV2) error {
		return func(app storage.AppenderV2) error {
			_, err := app.Append(0, lbls, 0, 123, 12.5, nil, nil, opts)
			return err
		}
	}

	return []testCase{
		{
			name: "sample",
			v1:   func(app storage.Appender) error { return appendV1(app, nil, metadata.Metadata{}) },
			v2:   appendV2(storage.AppendV2Options{}),
			want: []string{wantSample},
		},
		{
			name: "histogram",
			v1: func(app storage.Appender) error {
				_, err := app.AppendHistogram(0, lbls, 123, h, nil)
				return err
			},
			v2: func(app storage.AppenderV2) error {
				_, err := app.Append(0, lbls, 0, 123, 0, h, nil, storage.AppendV2Options{})
				return err
			},
			want: []string{wantHistogram},
		},
		{
			name: "float histogram",
			v1: func(app storage.Appender) error {
				_, err := app.AppendHistogram(0, lbls, 123, nil, fh)
				return err
			},
			v2: func(app storage.AppenderV2) error {
				_, err := app.Append(0, lbls, 0, 123, 0, nil, fh, storage.AppendV2Options{})
				return err
			},
			want: []string{wantFloatHistogram},
		},
		{
			name: "sample with exemplars",
			v1:   func(app storage.Appender) error { return appendV1(app, exemplars, metadata.Metadata{}) },
			v2:   appendV2(storage.AppendV2Options{Exemplars: exemplars}),
			want: []string{wantSample, wantExemplar1, wantExemplar2},
		},
		{
			name: "sample with metadata",
			v1:   func(app storage.Appender) error { return appendV1(app, nil, meta) },
			v2:   appendV2(storage.AppendV2Options{Metadata: meta}),
			want: []string{wantSample, wantMetadata},
		},
		{
			name: "sample with exemplars and metadata",
			v1:   func(app storage.Appender) error { return appendV1(app, exemplars, meta) },
			v2:   appendV2(storage.AppendV2Options{Exemplars: exemplars, Metadata: meta}),
			want: []string{wantSample, wantExemplar1, wantExemplar2, wantMetadata},
		},
	}
}

// publisherSpy is a livedebugging.DebugDataPublisher that records how it's
// called. It counts the IsActive calls for components other than componentID
// instead of recording their IDs, so that IsActive doesn't allocate.
type publisherSpy struct {
	componentID       livedebugging.ComponentID
	active            bool
	isActiveCalls     int
	otherComponentIDs int
	published         []livedebugging.Data
}

func (p *publisherSpy) IsActive(componentID livedebugging.ComponentID) bool {
	p.isActiveCalls++
	if componentID != p.componentID {
		p.otherComponentIDs++
	}
	return p.active
}

func (p *publisherSpy) PublishIfActive(data livedebugging.Data) {
	p.published = append(p.published, data)
}
