package scrape

import (
	"testing"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

func TestInterceptorDataHooksDebuggingRespectsActivity(t *testing.T) {
	componentID := livedebugging.ComponentID("prometheus.scrape.test")
	metricLabels := labels.FromStrings("__name__", "test_metric")
	metricExemplar := exemplar.Exemplar{
		Labels: labels.FromStrings("trace_id", "abc"),
		Value:  12.5,
		Ts:     123,
	}

	for _, hook := range []struct {
		name     string
		append   func(storage.Appender) (storage.SeriesRef, error)
		wantData string
	}{
		{
			name: "append",
			append: func(appender storage.Appender) (storage.SeriesRef, error) {
				return appender.Append(0, metricLabels, 123, 12.5)
			},
			wantData: `sample: ts=123, labels={__name__="test_metric"}, value=12.500000`,
		},
		{
			name: "histogram",
			append: func(appender storage.Appender) (storage.SeriesRef, error) {
				return appender.AppendHistogram(0, metricLabels, 123, nil, nil)
			},
			wantData: `histogram_with_no_value: ts=123, labels={__name__="test_metric"}`,
		},
		{
			name: "exemplar",
			append: func(appender storage.Appender) (storage.SeriesRef, error) {
				return appender.AppendExemplar(0, metricLabels, metricExemplar)
			},
			wantData: `exemplar: ts=123, labels={__name__="test_metric"}, exemplar_labels={trace_id="abc"}, value=12.500000`,
		},
	} {
		t.Run(hook.name, func(t *testing.T) {
			for _, active := range []bool{false, true} {
				t.Run(map[bool]string{false: "inactive", true: "active"}[active], func(t *testing.T) {
					publisher := &debugDataPublisherSpy{active: active}
					interceptor := NewInterceptor(componentID, publisher, appenders.Noop{})
					appender := interceptor.Appender(t.Context())

					_, err := hook.append(appender)
					require.NoError(t, err)
					require.Equal(t, 1, publisher.isActiveCalls)
					require.Equal(t, componentID, publisher.isActiveComponentID)

					if active {
						require.Equal(t, 1, publisher.publishCalls)
						require.Equal(t, componentID, publisher.publishedData.ComponentID)
						require.Equal(t, livedebugging.PrometheusMetric, publisher.publishedData.Type)
						require.Equal(t, uint64(1), publisher.publishedData.Count)
						require.Equal(t, hook.wantData, publisher.publishedData.DataFunc())
						return
					}

					require.Zero(t, publisher.publishCalls)
					var allocErr error
					allocs := testing.AllocsPerRun(100, func() {
						_, allocErr = hook.append(appender)
					})
					require.NoError(t, allocErr)
					require.Zero(t, allocs)
				})
			}
		})
	}
}

func TestInterceptorMetadataDebuggingRespectsActivity(t *testing.T) {
	for _, tc := range []struct {
		name             string
		active           bool
		wantPublishCalls int
	}{
		{name: "inactive", active: false, wantPublishCalls: 0},
		{name: "active", active: true, wantPublishCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			componentID := livedebugging.ComponentID("prometheus.scrape.test")
			publisher := &debugDataPublisherSpy{active: tc.active}
			interceptor := NewInterceptor(
				componentID,
				publisher,
				appenders.Noop{},
			)
			appender := interceptor.Appender(t.Context())
			metricLabels := labels.FromStrings("__name__", "test_metric")
			metricMetadata := metadata.Metadata{Type: "gauge", Unit: "items", Help: "test metric"}

			_, err := appender.UpdateMetadata(0, metricLabels, metricMetadata)
			require.NoError(t, err)
			require.Equal(t, 1, publisher.isActiveCalls)
			require.Equal(t, componentID, publisher.isActiveComponentID)
			require.Equal(t, tc.wantPublishCalls, publisher.publishCalls)

			if tc.active {
				require.Equal(t, componentID, publisher.publishedData.ComponentID)
				require.Equal(t, livedebugging.PrometheusMetric, publisher.publishedData.Type)
				require.Equal(t, uint64(1), publisher.publishedData.Count)
				wantData := `metadata: labels={__name__="test_metric"}, type="gauge", unit="items", help="test metric"`
				require.Equal(t, wantData, publisher.publishedData.DataFunc())
				return
			}

			var allocErr error
			allocs := testing.AllocsPerRun(100, func() {
				_, allocErr = appender.UpdateMetadata(0, metricLabels, metricMetadata)
			})
			require.NoError(t, allocErr)
			require.Zero(t, allocs)
		})
	}
}

type debugDataPublisherSpy struct {
	active        bool
	isActiveCalls int
	publishCalls  int

	isActiveComponentID livedebugging.ComponentID
	publishedData       livedebugging.Data
}

func (p *debugDataPublisherSpy) IsActive(componentID livedebugging.ComponentID) bool {
	p.isActiveCalls++
	p.isActiveComponentID = componentID
	return p.active
}

func (p *debugDataPublisherSpy) PublishIfActive(data livedebugging.Data) {
	p.publishCalls++
	p.publishedData = data
}
