package types

import (
	"context"
	"fmt"

	"github.com/vladopajic/go-actor/actor"
	"go.uber.org/atomic"
)

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
	m.mbx.Stop()
}

func (m *Mailbox[T]) ReceiveC() <-chan T {
	return m.mbx.ReceiveC()
}

func (m *Mailbox[T]) Send(ctx context.Context, value T) error {
	if m.stopped.Load() {
		return fmt.Errorf("mailbox is stopped")
	}
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
	sm.mbx.Stop()
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
