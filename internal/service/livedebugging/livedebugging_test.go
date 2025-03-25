package livedebugging

import (
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/util/testlivedebugging"
	"github.com/stretchr/testify/require"
)

func TestAddCallback(t *testing.T) {
	livedebugging := NewLiveDebugging()
	callbackID := CallbackID("callback1")
	callback := func(data Data) {}

	err := livedebugging.AddCallback(callbackID, "fake.liveDebugging", callback)
	require.ErrorContains(t, err, "the live debugging service is disabled. Check the documentation to find out how to enable it")

	livedebugging.SetEnabled(true)

	err = livedebugging.AddCallback(callbackID, "fake.liveDebugging", callback)
	require.ErrorContains(t, err, "the live debugging service is not ready yet")

	setupServiceHost(livedebugging)

	err = livedebugging.AddCallback(callbackID, "not found", callback)
	require.ErrorContains(t, err, "component not found")

	require.NoError(t, livedebugging.AddCallback(callbackID, "fake.liveDebugging", callback))

	err = livedebugging.AddCallback(callbackID, "fake.noLiveDebugging", callback)
	require.ErrorContains(t, err, "the component \"fake.noLiveDebugging\" does not support live debugging")

	require.NoError(t, livedebugging.AddCallback(callbackID, "declared.cmp/fake.liveDebugging", callback))

	err = livedebugging.AddCallback(callbackID, "declared.cmp/fake.noLiveDebugging", callback)
	require.ErrorContains(t, err, "the component \"fake.noLiveDebugging\" does not support live debugging")
}

func TestStream(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	callbackID := CallbackID("callback1")

	var receivedData Data
	callback := func(data Data) {
		receivedData = data
	}
	livedebugging.AddCallback(callbackID, componentID, callback)

	livedebugging.PublishIfActive(NewData(componentID, PrometheusMetric, 3, func() string { return "test data" }, WithTargetComponentIDs([]string{"component1"})))
	require.Equal(t, componentID, receivedData.ComponentID)
	require.Equal(t, []string{"component1"}, receivedData.TargetComponentIDs)
	require.Equal(t, uint64(3), receivedData.Count)
	require.Equal(t, "test data", receivedData.DataFunc())
	livedebugging.SetEnabled(false)
	livedebugging.PublishIfActive(NewData(componentID, PrometheusMetric, 3, func() string { return "new test data" }, WithTargetComponentIDs([]string{"component1"})))
	require.Equal(t, "test data", receivedData.DataFunc()) // not updated because the feature is disabled
}

func TestStreamEmpty(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	require.NotPanics(t, func() {
		livedebugging.PublishIfActive(NewData(componentID, PrometheusMetric, 3, func() string { return "test data" }, WithTargetComponentIDs([]string{"component1"})))
	})
}

func TestMultipleStreams(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	callbackID1 := CallbackID("callback1")
	callbackID2 := CallbackID("callback2")

	var receivedData1 Data
	callback1 := func(data Data) {
		receivedData1 = data
	}

	var receivedData2 Data
	callback2 := func(data Data) {
		receivedData2 = data
	}

	require.NoError(t, livedebugging.AddCallback(callbackID1, componentID, callback1))
	require.NoError(t, livedebugging.AddCallback(callbackID2, componentID, callback2))
	require.Len(t, livedebugging.callbacks[componentID], 2)

	livedebugging.PublishIfActive(NewData(componentID, PrometheusMetric, 3, func() string { return "test data" }))
	require.Equal(t, "test data", receivedData1.DataFunc())
	require.Equal(t, "test data", receivedData2.DataFunc())
}

func TestDeleteCallback(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	callbackID1 := CallbackID("callback1")
	callbackID2 := CallbackID("callback2")

	callback1 := func(data Data) {}
	callback2 := func(data Data) {}

	require.NoError(t, livedebugging.AddCallback(callbackID1, componentID, callback1))
	require.NoError(t, livedebugging.AddCallback(callbackID2, componentID, callback2))

	// Deleting callbacks that don't exist should not panic
	require.NotPanics(t, func() { livedebugging.DeleteCallback(callbackID1, "fakeComponentID") })
	require.NotPanics(t, func() { livedebugging.DeleteCallback("fakeCallbackID", componentID) })

	livedebugging.DeleteCallback(callbackID1, componentID)
	require.Len(t, livedebugging.callbacks[componentID], 1)

	livedebugging.DeleteCallback(callbackID2, componentID)
	require.Empty(t, livedebugging.callbacks[componentID])
}

func setupServiceHost(liveDebugging *liveDebugging) {
	host := &testlivedebugging.FakeServiceHost{
		ComponentsInfo: map[component.ID]testlivedebugging.FakeInfo{
			component.ParseID("fake.liveDebugging"):                {ComponentName: "fake.liveDebugging", Component: &testlivedebugging.FakeComponentLiveDebugging{}},
			component.ParseID("declared.cmp/fake.liveDebugging"):   {ComponentName: "fake.liveDebugging", Component: &testlivedebugging.FakeComponentLiveDebugging{}},
			component.ParseID("fake.noLiveDebugging"):              {ComponentName: "fake.noLiveDebugging", Component: &testlivedebugging.FakeComponentNoLiveDebugging{}},
			component.ParseID("declared.cmp/fake.noLiveDebugging"): {ComponentName: "fake.noLiveDebugging", Component: &testlivedebugging.FakeComponentNoLiveDebugging{}},
		},
	}
	liveDebugging.SetServiceHost(host)
	liveDebugging.SetEnabled(true)
}

func TestAddCallbackMulti(t *testing.T) {
	livedebugging := NewLiveDebugging()
	callbackID := CallbackID("callback1")
	callback := func(data Data) {}

	err := livedebugging.AddCallbackMulti(callbackID, "", callback)
	require.ErrorContains(t, err, "the live debugging service is not ready yet")

	setupServiceHost(livedebugging)

	err = livedebugging.AddCallbackMulti(callbackID, "not found", callback)
	require.ErrorContains(t, err, "module not found")

	require.NoError(t, livedebugging.AddCallbackMulti(callbackID, "", callback))

	require.NoError(t, livedebugging.AddCallbackMulti(callbackID, "declared.cmp", callback))
}

func TestDeleteCallbackMulti(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	callbackID1 := CallbackID("callback1")
	callbackID2 := CallbackID("callback2")

	callback1 := func(data Data) {}
	callback2 := func(data Data) {}

	require.NoError(t, livedebugging.AddCallbackMulti(callbackID1, "", callback1))
	require.NoError(t, livedebugging.AddCallbackMulti(callbackID2, "", callback2))
	require.Len(t, livedebugging.callbacks[componentID], 2)

	// Deleting callbacks that don't exist should not panic
	require.NotPanics(t, func() { livedebugging.DeleteCallbackMulti(callbackID1, "fakeComponentID") })
	require.NotPanics(t, func() { livedebugging.DeleteCallbackMulti("fakeCallbackID", "") })

	livedebugging.DeleteCallbackMulti(callbackID1, "")
	require.Len(t, livedebugging.callbacks[componentID], 1)

	livedebugging.DeleteCallbackMulti(callbackID2, "")
	require.Empty(t, livedebugging.callbacks[componentID])
}

func TestMultiCallbacksMultipleStreams(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	callbackID1 := CallbackID("callback1")
	callbackID2 := CallbackID("callback2")

	var receivedData1 Data
	callback1 := func(data Data) {
		receivedData1 = data
	}

	var receivedData2 Data
	callback2 := func(data Data) {
		receivedData2 = data
	}

	require.NoError(t, livedebugging.AddCallbackMulti(callbackID1, "", callback1))
	require.NoError(t, livedebugging.AddCallbackMulti(callbackID2, "", callback2))
	require.Len(t, livedebugging.callbacks[componentID], 2)

	livedebugging.PublishIfActive(NewData(componentID, PrometheusMetric, 3, func() string { return "test data" }))
	require.Equal(t, "test data", receivedData1.DataFunc())
	require.Equal(t, "test data", receivedData2.DataFunc())
}
