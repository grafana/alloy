package livedebugging

import (
	"context"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAddCallback(t *testing.T) {
	livedebugging := NewLiveDebugging()
	callbackID := CallbackID("callback1")
	callback := func(data string) {}

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

	var receivedData string
	callback := func(data string) {
		receivedData = data
	}
	require.False(t, livedebugging.IsActive(componentID))
	livedebugging.AddCallback(callbackID, componentID, callback)
	require.True(t, livedebugging.IsActive(componentID))
	require.Len(t, livedebugging.callbacks[componentID], 1)

	livedebugging.Publish(componentID, "test data")
	require.Equal(t, "test data", receivedData)

	livedebugging.SetEnabled(false)
	livedebugging.Publish(componentID, "new test data")
	require.Equal(t, "test data", receivedData) // not updated because the feature is disabled
}

func TestStreamEmpty(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	require.NotPanics(t, func() { livedebugging.Publish(componentID, "test data") })
}

func TestMultipleStreams(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
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

	require.NoError(t, livedebugging.AddCallback(callbackID1, componentID, callback1))
	require.NoError(t, livedebugging.AddCallback(callbackID2, componentID, callback2))
	require.Len(t, livedebugging.callbacks[componentID], 2)

	livedebugging.Publish(componentID, "test data")
	require.Equal(t, "test data", receivedData1)
	require.Equal(t, "test data", receivedData2)
}

func TestDeleteCallback(t *testing.T) {
	livedebugging := NewLiveDebugging()
	setupServiceHost(livedebugging)
	componentID := ComponentID("fake.liveDebugging")
	callbackID1 := CallbackID("callback1")
	callbackID2 := CallbackID("callback2")

	callback1 := func(data string) {}
	callback2 := func(data string) {}

	require.NoError(t, livedebugging.AddCallback(callbackID1, componentID, callback1))
	require.NoError(t, livedebugging.AddCallback(callbackID2, componentID, callback2))
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

func setupServiceHost(liveDebugging *liveDebugging) {
	host := &fakeServiceHost{
		componentsInfo: map[component.ID]fakeInfo{
			component.ParseID("fake.liveDebugging"):                {ComponentName: "fake.liveDebugging", Component: &fakeComponentLiveDebugging{}},
			component.ParseID("declared.cmp/fake.liveDebugging"):   {ComponentName: "fake.liveDebugging", Component: &fakeComponentLiveDebugging{}},
			component.ParseID("fake.noLiveDebugging"):              {ComponentName: "fake.noLiveDebugging", Component: &fakeComponentNoLiveDebugging{}},
			component.ParseID("declared.cmp/fake.noLiveDebugging"): {ComponentName: "fake.noLiveDebugging", Component: &fakeComponentNoLiveDebugging{}},
		},
	}
	liveDebugging.SetServiceHost(host)
	liveDebugging.SetEnabled(true)
}

type fakeInfo struct {
	ComponentName string
	Component     component.Component
}

type fakeServiceHost struct {
	service.Host
	componentsInfo map[component.ID]fakeInfo
}

func (h *fakeServiceHost) GetComponent(id component.ID, opts component.InfoOptions) (*component.Info, error) {
	info, exist := h.componentsInfo[id]
	if exist {
		return &component.Info{ID: id, ComponentName: info.ComponentName, Component: info.Component}, nil
	}

	return nil, component.ErrComponentNotFound
}

type fakeComponentLiveDebugging struct {
}

func (f *fakeComponentLiveDebugging) LiveDebugging() {}

func (f *fakeComponentLiveDebugging) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (f *fakeComponentLiveDebugging) Update(_ component.Arguments) error {
	return nil
}

type fakeComponentNoLiveDebugging struct {
}

func (f *fakeComponentNoLiveDebugging) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (f *fakeComponentNoLiveDebugging) Update(_ component.Arguments) error {
	return nil
}
