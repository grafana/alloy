package eventhandler

import (
	"os"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestCacheLoad(t *testing.T) {
	l := log.NewNopLogger()
	testTime, _ := time.Parse(time.RFC3339, "2022-01-26T13:39:40-05:00")
	expectedEvents := &ShippedEvents{
		Timestamp: testTime,
		RvMap:     map[string]struct{}{"58588": {}},
	}
	cacheFile, err := os.OpenFile("testdata/eventhandler.cache", os.O_RDWR|os.O_CREATE, cacheFileMode)
	require.NoError(t, err, "Failed to open test eventhandler cache file")
	actualEvents, err := readInitEvent(cacheFile, l)
	require.NoError(t, err, "Failed to parse last event from eventhandler cache file")
	require.Equal(t, expectedEvents, actualEvents)
}

func TestExtractEventJson(t *testing.T) {
	var eh = new(EventHandler)
	eh.logFormat = logFormatJson
	var event = new(v1.Event)
	event.InvolvedObject = v1.ObjectReference{
		Name: "test-object",
	}
	event.Message = "Event Message"

	_, msg, err := eh.extractEvent(event)
	require.NoError(t, err, "Failed to extract test event")
	require.Equal(t, "{\"msg\":\"Event Message\",\"name\":\"test-object\"}", msg)
}

func TestExtractEventText(t *testing.T) {
	var eh = new(EventHandler)
	eh.logFormat = "logfmt"
	var event = new(v1.Event)
	event.InvolvedObject = v1.ObjectReference{
		Name: "test-object",
	}
	event.Message = "Event Message"

	_, msg, err := eh.extractEvent(event)
	require.NoError(t, err, "Failed to extract test event")
	require.Equal(t, "name=test-object msg=\"Event Message\"", msg)
}
