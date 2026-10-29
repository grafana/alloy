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
