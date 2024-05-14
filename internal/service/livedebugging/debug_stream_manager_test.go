package livedebugging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
	streamID := "stream1"

	callback := func(data string) {}

	// Deleting wrong stream should not panic
	manager.DeleteStream(streamID, "fakeComponentID")
	manager.DeleteStream("fakeStreamID", componentID)

	manager.SetStream(streamID, componentID, callback)
	manager.DeleteStream(streamID, componentID)
	require.Empty(t, manager.streams[componentID])
}
