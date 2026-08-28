package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestEnqueueDequeue(t *testing.T) {
	tn := &QueuedNode{}
	q := NewQueue()
	q.Enqueue(tn)
	require.Lenf(t, q.queuedSet, 1, "queue should be 1")
	all := q.DequeueAll()
	require.Len(t, all, 1)
	require.True(t, all[0] == tn)
	require.Len(t, q.queuedSet, 0)
}

func TestDequeue_Empty(t *testing.T) {
	q := NewQueue()
	require.Len(t, q.queuedSet, 0)
	require.Len(t, q.DequeueAll(), 0)
}

func TestDequeue_InOrder(t *testing.T) {
	c1, c2, c3 := &QueuedNode{}, &QueuedNode{}, &QueuedNode{}
	q := NewQueue()
	q.Enqueue(c1)
	q.Enqueue(c2)
	q.Enqueue(c3)
	require.Len(t, q.queuedSet, 3)
	all := q.DequeueAll()
	require.Len(t, all, 3)
	require.Len(t, q.queuedSet, 0)
	require.Same(t, c1, all[0])
	require.Same(t, c2, all[1])
	require.Same(t, c3, all[2])
}

func TestDequeue_NoDuplicates(t *testing.T) {
	c1, c2 := &QueuedNode{}, &QueuedNode{}
	q := NewQueue()
	q.Enqueue(c1)
	q.Enqueue(c1)
	q.Enqueue(c2)
	q.Enqueue(c1)
	q.Enqueue(c2)
	q.Enqueue(c1)
	require.Len(t, q.queuedSet, 2)
	all := q.DequeueAll()
	require.Len(t, all, 2)
	require.Len(t, q.queuedSet, 0)
	require.Same(t, c1, all[0])
	require.Same(t, c2, all[1])
}

func TestEnqueue_ChannelNotification(t *testing.T) {
	c1 := &QueuedNode{}
	q := NewQueue()

	notificationsCount := atomic.Int32{}
	waiting := make(chan struct{})
	testDone := make(chan struct{})
	defer close(testDone)
	go func() {
		waiting <- struct{}{}
		for {
			select {
			case <-testDone:
				return
			case <-q.Chan():
				all := q.DequeueAll()
				notificationsCount.Add(int32(len(all)))
			}
		}
	}()

	// Make sure the consumer is waiting
	<-waiting

	// Write 10 items to the queue and make sure we get notified
	for i := 1; i <= 10; i++ {
		q.Enqueue(c1)
		require.Eventually(t, func() bool {
			return notificationsCount.Load() == int32(i)
		}, 3*time.Second, 5*time.Millisecond)
	}
}
