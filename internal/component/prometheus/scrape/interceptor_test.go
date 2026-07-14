package scrape

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

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
