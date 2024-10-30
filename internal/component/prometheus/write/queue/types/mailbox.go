package types

import (
	"context"
	"github.com/vladopajic/go-actor/actor"
)

// Mailbox wraps a standard mailbox. The reason we want to wrap is to not close the mailbox implicitly.
// A mailbox is a channel and will get garbage collected underneath so no worries about leaks, but since it can have multiple
// writers closing the channel can lead to panics and/or leaks of the TimeSeries not being added to the pool.
type Mailbox[T any] struct {
	mbx actor.Mailbox[T]
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
	return m.mbx.Send(ctx, value)
}

// SyncMailbox is used to synchronously send data, and wait for it to process before returning.
type SyncMailbox[T any] struct {
	mbx actor.Mailbox[Callback[T]]
}

func NewSyncMailbox[T any]() *SyncMailbox[T] {
	return &SyncMailbox[T]{
		mbx: actor.NewMailbox[Callback[T]](),
	}
}

func (sm *SyncMailbox[T]) Start() {
	sm.mbx.Start()
}

func (sm *SyncMailbox[T]) Stop() {
	// Note we are explicitly NOT calling stop here.
	// Closing the channel can cause panics.
	// Since multiple goroutines can write its really hard to know when you can safely close.
	// Either way it will be garbage collected normally.
	//sm.mbx.Stop()
}

func (sm *SyncMailbox[T]) ReceiveC() <-chan Callback[T] {
	return sm.mbx.ReceiveC()
}

func (sm *SyncMailbox[T]) Send(ctx context.Context, value T) error {
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
