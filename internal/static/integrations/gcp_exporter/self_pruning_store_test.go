package gcp_exporter_test

import (
	"testing"
	"time"

	"github.com/prometheus-community/stackdriver_exporter/collectors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tilinna/clock"
	"golang.org/x/exp/maps"
	"google.golang.org/api/monitoring/v3"

	"github.com/grafana/alloy/internal/static/integrations/gcp_exporter"
	"github.com/grafana/alloy/internal/util"
)

type testStore struct {
	incrementCounter               int
	descriptorToListMetricsCounter map[string]int
	state                          map[string][]*collectors.ConstMetric
}

func newTestStore(state ...map[string][]*collectors.ConstMetric) *testStore {
	internalState := map[string][]*collectors.ConstMetric{}
	if len(state) == 1 {
		internalState = state[0]
	}
	return &testStore{
		descriptorToListMetricsCounter: map[string]int{},
		state:                          internalState,
	}
}

func (t *testStore) Increment(_ *monitoring.MetricDescriptor, _ *collectors.ConstMetric) {
	t.incrementCounter++
}

func (t *testStore) ListMetrics(metricDescriptorName string) []*collectors.ConstMetric {
	t.descriptorToListMetricsCounter[metricDescriptorName]++
	return t.state[metricDescriptorName]
}

func TestSelfPruningDeltaStore_Increment_Delegates(t *testing.T) {
	counterStore := newTestStore()
	pruningStore := gcp_exporter.NewSelfPruningDeltaStore[collectors.ConstMetric](util.TestAlloyLogger(t), counterStore)
	descriptor := &monitoring.MetricDescriptor{Name: "test-descriptor"}
	currentValue := &collectors.ConstMetric{}
	pruningStore.Increment(descriptor, currentValue)
	assert.Equal(t, 1, counterStore.incrementCounter)
}

func TestSelfPruningDeltaStore_ListMetrics_Delegates(t *testing.T) {
	counterStore := newTestStore()
	pruningStore := gcp_exporter.NewSelfPruningDeltaStore[collectors.ConstMetric](util.TestAlloyLogger(t), counterStore)
	pruningStore.ListMetrics("test-descriptor")
	assert.Len(t, counterStore.descriptorToListMetricsCounter, 1)
	assert.Equal(t, 1, counterStore.descriptorToListMetricsCounter["test-descriptor"])
}

func TestSelfPruningDeltaStore_PruningWorkflow(t *testing.T) {
	sixMinutesAheadClock := clock.Context(t.Context(), clock.NewMock(time.Now().Add(6*time.Minute)))
	type testCase struct {
		name               string
		storeState         map[string][]*collectors.ConstMetric
		callsToMakeTo      func(store *gcp_exporter.SelfPruningDeltaStore[collectors.ConstMetric], ts *testStore)
		expectedCallCounts map[string]int
	}
	tests := []testCase{
		{
			name: "does nothing when last operation time does not require pruning",
			storeState: map[string][]*collectors.ConstMetric{
				"test-descriptor": {{FqName: "test-const-metric"}},
			},
			callsToMakeTo: func(store *gcp_exporter.SelfPruningDeltaStore[collectors.ConstMetric], ts *testStore) {
				// Initialize last operation time
				store.ListMetrics("test-descriptor")
				store.Prune(t.Context())
			},
			expectedCallCounts: map[string]int{"test-descriptor": 1}, // Once to init last operation time
		},
		{
			name:       "does nothing when no metric descriptors have been tracked",
			storeState: map[string][]*collectors.ConstMetric{},
			callsToMakeTo: func(store *gcp_exporter.SelfPruningDeltaStore[collectors.ConstMetric], ts *testStore) {
				// Initialize last operation time
				store.ListMetrics("test-descriptor")
				store.Prune(sixMinutesAheadClock)
			},
			expectedCallCounts: map[string]int{"test-descriptor": 1}, // Once to init last operation time
		},
		{
			name: "will prune outstanding descriptors",
			storeState: map[string][]*collectors.ConstMetric{
				"test-descriptor": {{FqName: "test-const-metric"}},
			},
			callsToMakeTo: func(store *gcp_exporter.SelfPruningDeltaStore[collectors.ConstMetric], ts *testStore) {
				store.ListMetrics("test-descriptor")
				store.Prune(sixMinutesAheadClock)
			},
			expectedCallCounts: map[string]int{
				"test-descriptor": 2, // Once to track it and once to prune it
			},
		},
		{
			name: "will stop pruning a descriptor with no results",
			storeState: map[string][]*collectors.ConstMetric{
				"test-descriptor": {{FqName: "test-const-metric"}},
			},
			callsToMakeTo: func(store *gcp_exporter.SelfPruningDeltaStore[collectors.ConstMetric], ts *testStore) {
				store.ListMetrics("test-descriptor")
				ts.state["test-descriptor"] = []*collectors.ConstMetric{}
				store.Prune(sixMinutesAheadClock)
				store.Prune(sixMinutesAheadClock)
			},
			expectedCallCounts: map[string]int{
				"test-descriptor": 2, // Once to track it and once to prune it
			},
		},
		{
			name: "stops tracking descriptors with no results",
			storeState: map[string][]*collectors.ConstMetric{
				"test-descriptor": {{FqName: "test-const-metric"}},
			},
			callsToMakeTo: func(store *gcp_exporter.SelfPruningDeltaStore[collectors.ConstMetric], ts *testStore) {
				// Track it
				store.ListMetrics("test-descriptor")
				// Make it empty
				ts.state["test-descriptor"] = []*collectors.ConstMetric{}
				// Try to untrack it
				store.ListMetrics("test-descriptor")
				store.Prune(sixMinutesAheadClock)
			},
			expectedCallCounts: map[string]int{
				"test-descriptor": 2, // Once to track it and once to untrack it
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestStore(tc.storeState)
			store := gcp_exporter.NewSelfPruningDeltaStore[collectors.ConstMetric](
				util.TestAlloyLogger(t),
				ts)
			tc.callsToMakeTo(store, ts)

			require.ElementsMatch(t, maps.Keys(tc.expectedCallCounts), maps.Keys(ts.descriptorToListMetricsCounter))
			for descriptor, callCount := range tc.expectedCallCounts {
				assert.Equal(t, callCount, ts.descriptorToListMetricsCounter[descriptor], "descriptor %s had an incorrect call count", descriptor)
			}
		})
	}
}
