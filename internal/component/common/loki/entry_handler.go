package loki

// This code is copied from Promtail. The loki package contains the definitions
// that allow log entries to flow from one subsystem to another, from scrapes,
// to relabeling, stages and finally batched in a client to be written to Loki.

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/common/model"
)

// finalEntryTimeout is how long NewEntryMutatorHandler will wait before giving
// up on sending the final log entry. If this timeout is reached, the final log
// entry is permanently lost.
//
// This timeout can only be reached if the loki.write client is backlogged due
// to an outage or erroring (such as limits being hit).
const finalEntryTimeout = 5 * time.Second

// EntryHandler is something that can "handle" entries via a channel.
// Stop must be called to gracefully shut down the EntryHandler
type EntryHandler interface {
	Chan() chan<- Entry
	Stop()
}

// EntryMiddleware takes an EntryHandler and returns another one that will intercept and forward entries.
// The newly created EntryHandler should be Stopped independently of the original one.
type EntryMiddleware interface {
	Wrap(EntryHandler) EntryHandler
}

// EntryMiddlewareFunc allows to create EntryMiddleware via a function.
type EntryMiddlewareFunc func(EntryHandler) EntryHandler

// Wrap uses an EntryMiddlewareFunc to wrap around an EntryHandler and return
// a new one that applies that func.
func (e EntryMiddlewareFunc) Wrap(next EntryHandler) EntryHandler {
	return e(next)
}

// EntryMutatorFunc is a function that can mutate an entry
type EntryMutatorFunc func(Entry) Entry

type entryHandler struct {
	stop    func()
	entries chan<- Entry
}

func (e entryHandler) Chan() chan<- Entry {
	return e.entries
}

func (e entryHandler) Stop() {
	e.stop()
}

// NewEntryHandler creates a new EntryHandler using an input channel and a stop function.
func NewEntryHandler(entries chan<- Entry, stop func()) EntryHandler {
	return entryHandler{
		stop:    stop,
		entries: entries,
	}
}

// NewEntryMutatorHandler creates a EntryHandler that mutates incoming entries from another EntryHandler.
func NewEntryMutatorHandler(next EntryHandler, f EntryMutatorFunc) EntryHandler {
	var (
		ctx, cancel = context.WithCancel(context.Background())

		in       = make(chan Entry)
		nextChan = next.Chan()
	)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer cancel()

		for e := range in {
			select {
			case <-ctx.Done():
				// This is a hard stop to the reading goroutine. Anything not forwarded
				// to nextChan at this point will probably be permanently lost, since
				// the positions file has likely already updated to a byte offset past
				// the read entry.
				//
				// TODO(rfratto): revisit whether this logic is necessary after we have
				// a WAL for logs.
				return
			case nextChan <- f(e):
				// no-op; log entry has been queued for sending.
			}
		}
	}()

	var closeOnce sync.Once
	return NewEntryHandler(in, func() {
		closeOnce.Do(func() {
			close(in)

			select {
			case <-ctx.Done():
				// The goroutine above exited on its own, so we don't have to wait for
				// the timeout.
			case <-time.After(finalEntryTimeout):
				// We reached the timeout for sending the final entry to nextChan;
				// request a hard stop from the reading goroutine.
				cancel()
			}
		})

		wg.Wait()
	})
}

// AddLabelsMiddleware is an EntryMiddleware that adds some labels.
func AddLabelsMiddleware(additionalLabels model.LabelSet) EntryMiddleware {
	return EntryMiddlewareFunc(func(eh EntryHandler) EntryHandler {
		return NewEntryMutatorHandler(eh, func(e Entry) Entry {
			if len(additionalLabels) == 0 {
				return e
			}

			if e.Labels == nil {
				e.Labels = make(model.LabelSet, len(additionalLabels))
			}

			// Iterate and mutate the labels in place to avoid allocations.
			for k, v := range additionalLabels {
				if _, ok := e.Labels[k]; !ok {
					e.Labels[k] = v
				}
			}
			return e
		})
	})
}
