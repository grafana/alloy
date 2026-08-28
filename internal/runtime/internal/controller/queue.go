package controller

import (
	"sync"
	"time"
)

// Queue is a thread-safe, insertion-ordered set of nodes.
//
// Queue is intended for tracking nodes that have been updated for later reevaluation.
type Queue struct {
	mut         sync.Mutex
	queuedSet   map[*QueuedNode]struct{}
	queuedOrder []*QueuedNode

	updateCh chan struct{}
}

type QueuedNode struct {
	Node            BlockNode
	LastUpdatedTime time.Time
}

// NewQueue returns a new queue.
func NewQueue() *Queue {
	return &Queue{
		updateCh:    make(chan struct{}, 1),
		queuedSet:   make(map[*QueuedNode]struct{}),
		queuedOrder: make([]*QueuedNode, 0),
	}
}

// Enqueue inserts a new BlockNode into the Queue. Enqueue is a no-op if the
// BlockNode is already in the Queue.
func (q *Queue) Enqueue(c *QueuedNode) {
	q.mut.Lock()
	defer q.mut.Unlock()

	// Skip if already queued.
	if _, ok := q.queuedSet[c]; ok {
		return
	}

	q.queuedOrder = append(q.queuedOrder, c)
	q.queuedSet[c] = struct{}{}
	select {
	case q.updateCh <- struct{}{}:
	default:
	}
}

// Chan returns a channel which is written to when the queue is non-empty.
func (q *Queue) Chan() <-chan struct{} { return q.updateCh }

// DequeueAll removes all BlockNode from the queue and returns them.
func (q *Queue) DequeueAll() []*QueuedNode {
	q.mut.Lock()
	defer q.mut.Unlock()

	all := q.queuedOrder
	q.queuedOrder = make([]*QueuedNode, 0)
	q.queuedSet = make(map[*QueuedNode]struct{})

	return all
}
