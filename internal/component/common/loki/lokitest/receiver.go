package lokitest

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

var (
	_ loki.Consumer     = (*Receiver)(nil)
	_ loki.LogsReceiver = (*Receiver)(nil)
)

// Receiver is a receiver for a component to forward to. It collects every
// entry it receives so tests can assert on them.
type Receiver struct {
	recv     loki.LogsReceiver
	consumer *CollectingConsumer
	entries  func() []loki.Entry

	// consumed is the number of entries returned by Next so far.
	consumed int

	// stopFn stops the receiver from accepting entries.
	stopFn func()

	// pauseFn and resumeFn pause and resume the receiver.
	pauseFn  func()
	resumeFn func()
}

func (r *Receiver) Chan() chan loki.Entry {
	return r.recv.Chan()
}

// Consume implements loki.Consumer. It returns an error with Loki pipeline V1.
func (r *Receiver) Consume(ctx context.Context, batch loki.Batch) error {
	if r.consumer == nil {
		return errors.New("lokitest: receiver does not support Loki pipeline V2")
	}
	return r.consumer.Consume(ctx, batch)
}

// ConsumeCalls returns how many times Consume has been called, It returns 0 with Loki pipeline V1.
func (r *Receiver) ConsumeCalls() int {
	if r.consumer == nil {
		return 0
	}
	return r.consumer.ConsumeCalls()
}

// Next waits until n entries arrive after the ones previous calls returned.
func (r *Receiver) Next(t *testing.T, n int) []loki.Entry {
	t.Helper()

	return r.NextWithin(t, n, 5*time.Second)
}

// NextWithin is like Next but waits up to timeout for the entries.
func (r *Receiver) NextWithin(t *testing.T, n int, timeout time.Duration) []loki.Entry {
	t.Helper()

	end := r.consumed + n
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.GreaterOrEqual(c, len(r.entries()), end)
	}, timeout, 10*time.Millisecond, "waiting for %d entries", n)

	entries := r.entries()[r.consumed:end]
	r.consumed = end
	return entries
}

// FailNext makes the next Consume call return err without collecting its batch.
// It does nothing with Loki pipeline V1.
func (r *Receiver) FailNext(err error) {
	if r.consumer == nil {
		return
	}
	r.consumer.FailNext(err)
}

// Entries returns every entry received so far.
func (r *Receiver) Entries() []loki.Entry {
	return r.entries()
}

// Stop makes the receiver stop accepting entries.
func (r *Receiver) Stop() {
	r.stopFn()
}

// Pause makes the receiver hold back entries until Resume is called.
// A sender blocks until then, or until its context is canceled.
func (r *Receiver) Pause() {
	r.pauseFn()
}

// Resume makes a paused receiver accept entries again.
func (r *Receiver) Resume() {
	r.resumeFn()
}

var _ loki.LogsReceiver = (*CollectingReceiver)(nil)

// NewCollectingReceiver returns a CollectingReceiver that is already collecting.
// It must be stopped to release its goroutine.
func NewCollectingReceiver() *CollectingReceiver {
	r := &CollectingReceiver{
		ch:      make(chan loki.Entry),
		stopped: make(chan struct{}),
		wake:    make(chan struct{}, 1),
	}

	r.wg.Go(func() {
		for {
			// A nil channel is never ready, so while paused nothing is received.
			in := r.ch
			r.mut.Lock()
			if r.paused {
				in = nil
			}
			r.mut.Unlock()

			select {
			case <-r.stopped:
				return
			case <-r.wake:
			case e := <-in:
				r.mut.Lock()
				r.entries = append(r.entries, e)
				r.mut.Unlock()
			}
		}
	})

	return r
}

// CollectingReceiver is a loki.LogsReceiver that collects all entries sent to
// its channel until it is stopped. Once stopped it no longer reads from the
// channel, so senders block until their context ends.
type CollectingReceiver struct {
	ch      chan loki.Entry
	stopped chan struct{}
	// wake makes the collecting goroutine check if it is paused.
	wake chan struct{}
	once sync.Once
	wg   sync.WaitGroup

	mut     sync.Mutex
	paused  bool
	entries []loki.Entry
}

// Pause stops reading from the channel until Resume is called, so senders block.
func (r *CollectingReceiver) Pause() {
	r.mut.Lock()
	r.paused = true
	r.mut.Unlock()
	r.wakeUp()
}

// Resume makes a paused receiver read from the channel again.
func (r *CollectingReceiver) Resume() {
	r.mut.Lock()
	r.paused = false
	r.mut.Unlock()
	r.wakeUp()
}

func (r *CollectingReceiver) wakeUp() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *CollectingReceiver) Chan() chan loki.Entry {
	return r.ch
}

// Stop stops reading from the channel and waits for the collecting goroutine to
// exit. It can be called more than once.
func (r *CollectingReceiver) Stop() {
	r.once.Do(func() { close(r.stopped) })
	r.wg.Wait()
}

func (r *CollectingReceiver) Entries() []loki.Entry {
	r.mut.Lock()
	defer r.mut.Unlock()

	return slices.Clone(r.entries)
}

var _ loki.Consumer = (*CollectingConsumer)(nil)

func NewCollectingConsumer() *CollectingConsumer {
	return &CollectingConsumer{}
}

// CollectingConsumer is a Consumer that will collect all received batches
// so it can be inspected later.
type CollectingConsumer struct {
	mut      sync.Mutex
	stopped  bool
	calls    int
	failNext error
	batches  []loki.Batch
	// paused is non-nil while paused and is closed on Resume.
	paused chan struct{}
}

// Pause makes Consume block until Resume is called.
func (c *CollectingConsumer) Pause() {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.paused == nil {
		c.paused = make(chan struct{})
	}
}

// Resume unblocks Consume calls waiting on Pause.
func (c *CollectingConsumer) Resume() {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.paused != nil {
		close(c.paused)
		c.paused = nil
	}
}

func (c *CollectingConsumer) Consume(ctx context.Context, batch loki.Batch) error {
	c.mut.Lock()
	paused := c.paused
	c.mut.Unlock()

	if paused != nil {
		select {
		case <-paused:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	c.mut.Lock()
	defer c.mut.Unlock()

	c.calls++
	if c.stopped {
		return loki.ErrConsumerStopped
	}

	if c.failNext != nil {
		err := c.failNext
		c.failNext = nil
		return err
	}

	c.batches = append(c.batches, batch.Clone())
	return nil
}

// FailNext makes the next Consume call return err without collecting its batch.
func (c *CollectingConsumer) FailNext(err error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.failNext = err
}

// ConsumeCalls returns how many times Consume has been called, including the
// calls that returned loki.ErrConsumerStopped.
func (c *CollectingConsumer) ConsumeCalls() int {
	c.mut.Lock()
	defer c.mut.Unlock()

	return c.calls
}

func (c *CollectingConsumer) Batches() []loki.Batch {
	c.mut.Lock()
	defer c.mut.Unlock()

	return slices.Clone(c.batches)
}

func (c *CollectingConsumer) Entries() []loki.Entry {
	c.mut.Lock()
	defer c.mut.Unlock()

	var entries []loki.Entry
	for _, b := range c.batches {
		for _, s := range b.Streams() {
			for _, e := range s.Entries {
				entries = append(entries, loki.NewEntryWithCreatedUnixMicro(s.Labels, s.Created(), e))
			}
		}
	}
	return entries
}

// Stop makes Consume return loki.ErrConsumerStopped
func (c *CollectingConsumer) Stop() {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.stopped = true
}
