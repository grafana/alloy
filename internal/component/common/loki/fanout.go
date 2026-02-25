package loki

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// NewFanout creates a new Fanout that will send log entries to the provided
// list of LogsReceivers.
func NewFanout(children []LogsReceiver, register prometheus.Registerer) *Fanout {
	wl := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "loki_fanout_latency",
		Help:    "Write latency for sending to components",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
	})
	_ = register.Register(wl)

	return &Fanout{
		wl:       wl,
		children: children,
	}
}

// Fanout distributes log entries to multiple LogsReceivers.
// It is thread-safe and allows the list of receivers to be updated dynamically
type Fanout struct {
	mut      sync.RWMutex
	wl       prometheus.Histogram
	children []LogsReceiver
}

// Send forwards a log entry to all registered receivers. It returns an error
// if the context is cancelled while sending.
func (f *Fanout) Send(ctx context.Context, entry Entry) error {
	start := time.Now()
	defer func() { f.wl.Observe(float64(time.Since(start))) }()

	// NOTE: It's important that we hold a read lock for the duration of Send
	// rather than making a copy of children and releasing the lock early.
	//
	// When config is updated, the loader evaluates all components and updates
	// them while they continue running. The scheduler only stops removed components
	// after all updates complete. During this window, Send may execute concurrently
	// with receiver list updates. By holding the read lock for the entire Send
	// operation, receiver list updates (which require a write lock) will block
	// until all in-flight Send calls complete. This prevents sending entries to
	// receivers that have been removed by the scheduler.

	f.mut.RLock()
	defer f.mut.RUnlock()
	for _, recv := range f.children {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case recv.Chan() <- entry:
		}
	}
	return nil
}

// SendBatch forwards a batch of entires to all registered receivers. It returns an error
// if the context is cancelled while sending.
func (f *Fanout) SendBatch(ctx context.Context, batch []Entry) error {
	start := time.Now()
	defer func() { f.wl.Observe(float64(time.Since(start))) }()

	// NOTE: It's important that we hold a read lock for the duration of SendBatch
	// rather than making a copy of children and releasing the lock early.
	//
	// When config is updated, the loader evaluates all components and updates
	// them while they continue running. The scheduler only stops removed components
	// after all updates complete. During this window, Send may execute concurrently
	// with receiver list updates. By holding the read lock for the entire Send
	// operation, receiver list updates (which require a write lock) will block
	// until all in-flight Send calls complete. This prevents sending entries to
	// receivers that have been removed by the scheduler.

	f.mut.RLock()
	defer f.mut.RUnlock()
	for _, e := range batch {
		for _, recv := range f.children {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case recv.Chan() <- e:
			}
		}
	}
	return nil
}

// UpdateChildren updates the list of receivers that will receive log entries.
func (f *Fanout) UpdateChildren(children []LogsReceiver) {
	f.mut.RLock()
	if receiversChanged(f.children, children) {
		// Upgrade lock to write.
		f.mut.RUnlock()
		f.mut.Lock()
		f.children = children
		f.mut.Unlock()
	} else {
		f.mut.RUnlock()
	}
}

func receiversChanged(prev, next []LogsReceiver) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], next[i]) {
			return true
		}
	}
	return false
}
