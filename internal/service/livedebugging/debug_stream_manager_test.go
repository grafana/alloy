package livedebugging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	manager := NewDebugStreamManager()
	require.False(t, manager.IsRegistered("type1"))
	manager.Register("type1")
	require.True(t, manager.IsRegistered("type1"))
	// registering a component name that has already been registered does not do anything
	require.NotPanics(t, func() { manager.Register("type1") })
}

func TestStream(t *testing.T) {
	manager := NewDebugStreamManager()
	componentID := "component1"
	streamID := "stream1"

	var receivedData string
	callback := func(data string) {
		receivedData = data
	}

	manager.SetStream(streamID, componentID, callback)
	require.Len(t, manager.streams[componentID], 1)

	manager.Stream(componentID, "test data")
	require.Equal(t, "test data", receivedData)
}

func TestStreamEmpty(t *testing.T) {
	manager := NewDebugStreamManager()
	componentID := "component1"
	require.NotPanics(t, func() { manager.Stream(componentID, "test data") })
}

func TestMultipleStreams(t *testing.T) {
	manager := NewDebugStreamManager()
	componentID := "component1"
	streamID1 := "stream1"
	streamID2 := "stream2"

	var receivedData1 string
	callback1 := func(data string) {
		receivedData1 = data
	}

	var receivedData2 string
	callback2 := func(data string) {
		receivedData2 = data
	}

	manager.SetStream(streamID1, componentID, callback1)
	manager.SetStream(streamID2, componentID, callback2)
	require.Len(t, manager.streams[componentID], 2)

	manager.Stream(componentID, "test data")
	require.Equal(t, "test data", receivedData1)
	require.Equal(t, "test data", receivedData2)
}

func TestDeleteStream(t *testing.T) {
	manager := NewDebugStreamManager()
	componentID := "component1"
	streamID1 := "stream1"
	streamID2 := "stream2"

	callback1 := func(data string) {}
	callback2 := func(data string) {}

	manager.SetStream(streamID1, componentID, callback1)
	manager.SetStream(streamID2, componentID, callback2)
	require.Len(t, manager.streams[componentID], 2)

	// Deleting streams that don't exist should not panic
	require.NotPanics(t, func() { manager.DeleteStream(streamID1, "fakeComponentID") })
	require.NotPanics(t, func() { manager.DeleteStream("fakeStreamID", componentID) })

	manager.SetStream(streamID1, componentID, callback1)
	manager.SetStream(streamID2, componentID, callback2)

	manager.DeleteStream(streamID1, componentID)
	require.Len(t, manager.streams[componentID], 1)

	manager.DeleteStream(streamID2, componentID)
	require.Empty(t, manager.streams[componentID])
}
