package loki

import (
	"context"
	"slices"
	"sync"
)

// NewFanout creates a new Fanout that will send log entries to the provided
// list of LogsReceivers.
func NewFanout(children []LogsReceiver) *Fanout {
	return &Fanout{
		children: children,
		changed:  make(chan struct{}),
	}
}

// Fanout distributes log entries to multiple LogsReceivers.
// It is thread-safe and allows the list of receivers to be updated dynamically.
//
// UpdateChildren never blocks. This matters during a config reload: receivers
// that are new in the reload don't read entries until every component has
// been updated, so an UpdateChildren that waited for in-flight sends could
// deadlock the reload.
//
// A Send that is blocked when the receivers change stops waiting on receivers
// that were removed and delivers the entry to the new receivers instead. A
// receiver that is in both lists doesn't get the entry again. If the new list
// is empty, the entry goes nowhere, as it would for an entry sent after the
// change.
type Fanout struct {
	mut      sync.RWMutex
	children []LogsReceiver
	changed  chan struct{} // Closed and replaced every time children change.
}

// Send forwards a log entry to all registered receivers. It returns an error
// if the context is cancelled while sending.
func (f *Fanout) Send(ctx context.Context, entry Entry) error {
	// Receivers that already got the entry, so they can be skipped if the
	// receivers change mid-Send. The array keeps the common case free of
	// allocations.
	var buf [8]LogsReceiver
	sent := buf[:0]
	restarted := false

	children, changed := f.load()
outer:
	for {
		for _, recv := range children {
			if restarted && slices.Contains(sent, recv) {
				continue
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case recv.Chan() <- entry:
				sent = append(sent, recv)
			case <-changed:
				children, changed = f.load()
				restarted = true
				continue outer
			}
		}
		return nil
	}
}

// SendBatch forwards a batch of entries to all registered receivers. It
// returns an error if the context is cancelled while sending.
func (f *Fanout) SendBatch(ctx context.Context, batch []Entry) error {
	for _, e := range batch {
		if err := f.Send(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// UpdateChildren updates the list of receivers that will receive log entries.
// It never blocks on in-flight sends.
func (f *Fanout) UpdateChildren(children []LogsReceiver) {
	f.mut.Lock()
	defer f.mut.Unlock()

	if slices.Equal(f.children, children) {
		return
	}
	f.children = children
	close(f.changed)
	f.changed = make(chan struct{})
}

func (f *Fanout) load() ([]LogsReceiver, chan struct{}) {
	f.mut.RLock()
	defer f.mut.RUnlock()
	return f.children, f.changed
}
