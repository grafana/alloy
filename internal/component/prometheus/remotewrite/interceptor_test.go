package remotewrite

import (
	"context"
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/service/labelstore"
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
			componentID := livedebugging.ComponentID("prometheus.remote_write.test")
			publisher := &debugDataPublisherSpy{active: tc.active}
			interceptor := NewInterceptor(
				string(componentID),
				&atomic.Bool{},
				publisher,
				labelstore.New(nil, promclient.NewRegistry()),
				interceptorTestStorage{appender: appenders.Noop{}},
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

type interceptorTestStorage struct {
	storage.Queryable
	storage.ChunkQueryable

	appender storage.Appender
}

func (s interceptorTestStorage) Appender(context.Context) storage.Appender {
	return s.appender
}

func (interceptorTestStorage) StartTime() (int64, error) {
	return 0, nil
}

func (interceptorTestStorage) Close() error {
	return nil
}
