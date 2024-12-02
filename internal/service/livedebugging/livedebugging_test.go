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
	callback := func(data FeedData) {}

	err := livedebugging.AddCallback(callbackID, "fake.liveDebugging", callback)
	require.ErrorContains(t, err, "the live debugging service is disabled. Check the documentation to find out how to enable it")

	livedebugging.SetEnabled(true)

	err = livedebugging.AddCallback(callbackID, "fake.liveDebugging", callback)
	require.ErrorContains(t, err, "the live debugging service is not ready yet")

	setupServiceHost(livedebugging)

	err = livedebugging.AddCallback(callbackID, "not found", callback)
	require.ErrorContains(t, err, "component not found")

	require.NoError(t, livedebugging.AddCallback(callbackID, "fake.liveDebugging", callback))

	component, _ := livedebugging.host.GetComponent(component.ParseID("fake.liveDebugging"), component.InfoOptions{})
	require.Equal(t, 1, component.Component.(*testlivedebugging.FakeComponentLiveDebugging).ConsumersCount)

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

	var receivedData FeedData
	callback := func(data FeedData) {
		receivedData = data
	}
	require.False(t, livedebugging.IsActive(componentID))
	livedebugging.AddCallback(callbackID, componentID, callback)
	require.True(t, livedebugging.IsActive(componentID))
	require.Len(t, livedebugging.callbacks[componentID], 1)

	livedebugging.Publish(componentID, FeedData{Data: "test data"})
	require.Equal(t, "test data", receivedData)

	livedebugging.SetEnabled(false)
	livedebugging.Publish(componentID, FeedData{Data: "new test data"})
	require.Equal(t, "test data", receivedData) // not updated because the feature is disabled
}

func TestStreamEmpty(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	require.NotPanics(t, func() { livedebugging.Publish(componentID, FeedData{Data: "test data"}) })
}

func TestMultipleStreams(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	callbackID1 := CallbackID("callback1")
	callbackID2 := CallbackID("callback2")

	var receivedData1 FeedData
	callback1 := func(data FeedData) {
		receivedData1 = data
	}

	var receivedData2 FeedData
	callback2 := func(data FeedData) {
		receivedData2 = data
	}

	require.NoError(t, livedebugging.AddCallback(callbackID1, componentID, callback1))
	require.NoError(t, livedebugging.AddCallback(callbackID2, componentID, callback2))
	require.Len(t, livedebugging.callbacks[componentID], 2)

	livedebugging.Publish(componentID, FeedData{Data: "test data"})
	require.Equal(t, "test data", receivedData1)
	require.Equal(t, "test data", receivedData2)
}

func TestDeleteCallback(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	callbackID1 := CallbackID("callback1")
	callbackID2 := CallbackID("callback2")

	callback1 := func(data FeedData) {}
	callback2 := func(data FeedData) {}

	component, _ := livedebugging.host.GetComponent(component.ParseID("fake.liveDebugging"), component.InfoOptions{})

	require.NoError(t, livedebugging.AddCallback(callbackID1, componentID, callback1))
	require.Equal(t, 1, component.Component.(*testlivedebugging.FakeComponentLiveDebugging).ConsumersCount)
	require.NoError(t, livedebugging.AddCallback(callbackID2, componentID, callback2))
	require.Equal(t, 2, component.Component.(*testlivedebugging.FakeComponentLiveDebugging).ConsumersCount)
	require.Len(t, livedebugging.callbacks[componentID], 2)

	// Deleting callbacks that don't exist should not panic
	require.NotPanics(t, func() { livedebugging.DeleteCallback(callbackID1, "fakeComponentID") })
	require.NotPanics(t, func() { livedebugging.DeleteCallback("fakeCallbackID", componentID) })

	livedebugging.DeleteCallback(callbackID1, componentID)
	require.Len(t, livedebugging.callbacks[componentID], 1)
	require.Equal(t, 1, component.Component.(*testlivedebugging.FakeComponentLiveDebugging).ConsumersCount)

	livedebugging.DeleteCallback(callbackID2, componentID)
	require.Empty(t, livedebugging.callbacks[componentID])
	require.Equal(t, 0, component.Component.(*testlivedebugging.FakeComponentLiveDebugging).ConsumersCount)

	require.False(t, livedebugging.IsActive(ComponentID("fake.liveDebugging")))
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
