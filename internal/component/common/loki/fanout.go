package loki

import (
	"context"
	"reflect"
	"sync"
)

// NewFanout creates a new Fanout that will send log entries to the provided
// list of LogsReceivers.
func NewFanout(children []LogsReceiver) *Fanout {
	return &Fanout{
		children: children,
	}
}

// Fanout distributes log entries to multiple LogsReceivers.
// It is thread-safe and allows the list of receivers to be updated dynamically
type Fanout struct {
	mut      sync.RWMutex
	children []LogsReceiver
}

// Send forwards a log entry to all registered receivers. It returns an error
// if the context is cancelled while sending.
func (f *Fanout) Send(ctx context.Context, entry Entry) error {
	f.mut.RLock()
	defer f.mut.Unlock()
	for _, recv := range f.children {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case recv.Chan() <- entry:
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
