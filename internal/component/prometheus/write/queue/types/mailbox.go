package types

import (
	"context"
	"github.com/vladopajic/go-actor/actor"
	"go.uber.org/atomic"
)

// Mailbox wraps a standard mailbox with an atomic bool to prevent reads while closed.
type Mailbox[T any] struct {
	mbx     actor.Mailbox[T]
	stopped atomic.Bool
}

func NewMailbox[T any](capacity int, asChan bool) *Mailbox[T] {
	options := make([]actor.MailboxOption, 0)
	if capacity > 0 {
		options = append(options, actor.OptCapacity(capacity))
	}
	if asChan {
		options = append(options, actor.OptAsChan())
	}
	mbx := actor.NewMailbox[T](options...)
	return &Mailbox[T]{mbx: mbx}
}

func (m *Mailbox[T]) Start() {
	m.mbx.Start()
}

func (m *Mailbox[T]) Stop() {
	m.stopped.Store(true)
	// Note we are explicitly NOT calling stop here.
	// Closing the channel can cause panics.
	// Since multiple goroutines can write its really hard to know when you can safely close.
	// Either way it will be garbage collected normally.
	// m.mbx.Stop()
}

func (m *Mailbox[T]) ReceiveC() <-chan T {
	return m.mbx.ReceiveC()
}

func (m *Mailbox[T]) Send(ctx context.Context, value T) error {
	if m.stopped.Load() {
		return nil
	}
	return m.mbx.Send(ctx, value)
}

// SyncMailbox is used to synchronously send data, and wait for it to process before returning.
type SyncMailbox[T any] struct {
	mbx     actor.Mailbox[Callback[T]]
	stopped *atomic.Bool
}

func NewSyncMailbox[T any]() *SyncMailbox[T] {
	return &SyncMailbox[T]{
		mbx:     actor.NewMailbox[Callback[T]](),
		stopped: atomic.NewBool(true),
	}
}

func (sm *SyncMailbox[T]) Start() {
	sm.mbx.Start()
	sm.stopped.Store(false)
}

func (sm *SyncMailbox[T]) Stop() {
	sm.stopped.Store(true)
	//sm.mbx.Stop()
}

func (sm *SyncMailbox[T]) ReceiveC() <-chan Callback[T] {
	return sm.mbx.ReceiveC()
}

func (sm *SyncMailbox[T]) Send(ctx context.Context, value T) error {
	if sm.stopped.Load() {
		return nil
	}
	done := make(chan struct{})
	defer close(done)
	err := sm.mbx.Send(ctx, Callback[T]{
		Value: value,
		done:  done,
	})
	if err != nil {
		return err
	}
	<-done
	return nil
}

type Callback[T any] struct {
	Value T
	done  chan struct{}
}

// Notify must be called to return the synchronous call.
func (c *Callback[T]) Notify() {
	c.done <- struct{}{}
}
