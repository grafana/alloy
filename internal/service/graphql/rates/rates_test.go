package rates_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
	"github.com/grafana/alloy/internal/service/graphql/rates"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

const (
	scrapeDefaultID      = "prometheus.scrape.default"
	remoteWriteDefaultID = "prometheus.remote_write.default"
)

func TestDataFlowEdgesAggregateRatesForMatchingDestination(t *testing.T) {
	snapshot, err := rates.BuildSnapshot(5, []livedebugging.Data{
		livedebugging.NewData(scrapeDefaultID, livedebugging.PrometheusMetric, 10, nil, livedebugging.WithTargetComponentIDs([]string{remoteWriteDefaultID})),
		livedebugging.NewData(scrapeDefaultID, livedebugging.PrometheusMetric, 5, nil, livedebugging.WithTargetComponentIDs([]string{"prometheus.relabel.default"})),
	})
	require.NoError(t, err)

	got := snapshot.DataRatesForEdge(model.ComponentEdge{
		SourceID:    scrapeDefaultID,
		ComponentID: remoteWriteDefaultID,
	})

	require.Equal(t, []model.ComponentDataRate{{Type: "prometheus_metric", Rate: 2}}, got)
}

func TestDataFlowEdgesApplyUndirectedEventsToAllOutgoingEdges(t *testing.T) {
	snapshot, err := rates.BuildSnapshot(10, []livedebugging.Data{
		livedebugging.NewData(scrapeDefaultID, livedebugging.Target, 30, nil),
	})
	require.NoError(t, err)

	got := snapshot.DataRatesForEdge(model.ComponentEdge{
		SourceID:    scrapeDefaultID,
		ComponentID: remoteWriteDefaultID,
	})

	require.Equal(t, []model.ComponentDataRate{{Type: "target", Rate: 3}}, got)
}

func TestDataRatesRejectWindowOutsideOneToSixtySeconds(t *testing.T) {
	_, err := rates.BuildSnapshot(0, nil)
	require.Error(t, err)

	_, err = rates.BuildSnapshot(61, nil)
	require.Error(t, err)
}

func TestCollectSnapshotReadsContinuousRateSnapshot(t *testing.T) {
	provider := &snapshotProvider{
		data: []livedebugging.Data{
			livedebugging.NewData(scrapeDefaultID, livedebugging.PrometheusMetric, 20, nil),
		},
	}

	snapshot, err := rates.CollectSnapshot(provider, 5)

	require.NoError(t, err)
	require.Equal(t, 1, provider.snapshotCalls)
	require.Equal(t, []model.ComponentDataRate{{
		Type: "prometheus_metric",
		Rate: 4,
	}}, snapshot.DataRatesForEdge(model.ComponentEdge{
		SourceID:    scrapeDefaultID,
		ComponentID: remoteWriteDefaultID,
	}))
}

type snapshotProvider struct {
	data          []livedebugging.Data
	snapshotCalls int
}

func (p *snapshotProvider) RateSnapshot(int) ([]livedebugging.Data, error) {
	p.snapshotCalls++
	return p.data, nil
}

func (p *snapshotProvider) AddCallback(service.Host, livedebugging.CallbackID, livedebugging.ComponentID, func(livedebugging.Data)) error {
	return errors.New("callbacks must not be used for rate snapshots")
}

func (p *snapshotProvider) DeleteCallback(livedebugging.CallbackID, livedebugging.ComponentID) {}

func (p *snapshotProvider) AddCallbackMulti(service.Host, livedebugging.CallbackID, livedebugging.ModuleID, func(livedebugging.Data)) error {
	return errors.New("callbacks must not be used for rate snapshots")
}

func (p *snapshotProvider) DeleteCallbackMulti(service.Host, livedebugging.CallbackID, livedebugging.ModuleID) {
}
