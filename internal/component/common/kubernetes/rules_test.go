package kubernetes

import (
	"testing"

	"k8s.io/client-go/util/workqueue"
)

func TestEventTypeIsHashable(t *testing.T) {
	// This test is here to ensure that the EventType type is hashable according to the workqueue implementation
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	queue.AddRateLimited(Event{})
}
