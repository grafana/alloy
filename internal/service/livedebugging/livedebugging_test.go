package livedebugging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	livedebugging := NewLiveDebugging()
	require.False(t, livedebugging.IsRegistered("type1"))
	livedebugging.Register("type1")
	require.True(t, livedebugging.IsRegistered("type1"))
	// registering a component name that has already been registered does not do anything
	require.NotPanics(t, func() { livedebugging.Register("type1") })
}

func TestStream(t *testing.T) {
	livedebugging := NewLiveDebugging()
	componentID := ComponentID("component1")
	CallbackID := CallbackID("callback1")

	var receivedData string
	callback := func(data string) {
		receivedData = data
	}
	require.False(t, livedebugging.IsActive(componentID))
	livedebugging.AddCallback(CallbackID, componentID, callback)
	require.True(t, livedebugging.IsActive(componentID))
	require.Len(t, livedebugging.callbacks[componentID], 1)

	livedebugging.Publish(componentID, "test data")
	require.Equal(t, "test data", receivedData)
}

func TestStreamEmpty(t *testing.T) {
	livedebugging := NewLiveDebugging()
	componentID := ComponentID("component1")
	require.NotPanics(t, func() { livedebugging.Publish(componentID, "test data") })
}

func TestMultipleStreams(t *testing.T) {
	livedebugging := NewLiveDebugging()
	componentID := ComponentID("component1")
	callbackID1 := CallbackID("callback1")
	callbackID2 := CallbackID("callback2")

	var receivedData1 string
	callback1 := func(data string) {
		receivedData1 = data
	}

	var receivedData2 string
	callback2 := func(data string) {
		receivedData2 = data
	}

	livedebugging.AddCallback(callbackID1, componentID, callback1)
	livedebugging.AddCallback(callbackID2, componentID, callback2)
	require.Len(t, livedebugging.callbacks[componentID], 2)

	livedebugging.Publish(componentID, "test data")
	require.Equal(t, "test data", receivedData1)
	require.Equal(t, "test data", receivedData2)
}

func TestDeleteCallback(t *testing.T) {
	livedebugging := NewLiveDebugging()
	componentID := ComponentID("component1")
	callbackID1 := CallbackID("callback1")
	callbackID2 := CallbackID("callback2")

	callback1 := func(data string) {}
	callback2 := func(data string) {}

	livedebugging.AddCallback(callbackID1, componentID, callback1)
	livedebugging.AddCallback(callbackID2, componentID, callback2)
	require.Len(t, livedebugging.callbacks[componentID], 2)

	// Deleting callbacks that don't exist should not panic
	require.NotPanics(t, func() { livedebugging.DeleteCallback(callbackID1, "fakeComponentID") })
	require.NotPanics(t, func() { livedebugging.DeleteCallback("fakeCallbackID", componentID) })

	livedebugging.AddCallback(callbackID1, componentID, callback1)
	livedebugging.AddCallback(callbackID2, componentID, callback2)

	livedebugging.DeleteCallback(callbackID1, componentID)
	require.Len(t, livedebugging.callbacks[componentID], 1)

	livedebugging.DeleteCallback(callbackID2, componentID)
	require.Empty(t, livedebugging.callbacks[componentID])
}
