package loki

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
)

// Producer is a running instance of whatever a source uses to produce log
// entries, such as a server, a tailer or a consumer group member, as returned
// by SourceRunnerOptions.Start. A producer sends entries to
// SourceRunner.Entries or SourceRunner.Batches. An entry counts as accepted
// once it's handed off there.
type Producer[C any] struct {
	// Stop stops the producer and is required. Once Stop returns, the producer
	// must not send any more entries. Stop may block while the producer hands
	// off entries it already holds: SourceRunner keeps forwarding entries
	// until Stop returns. SourceRunner calls Stop exactly once per producer,
	// also when the producer exited on its own.
	Stop func()

	// Reconfigure, if set, applies cfg to the running producer and returns
	// true, or returns false if cfg needs a restart. It runs concurrently with
	// the producer's own goroutines. If it's nil, every config change restarts
	// the producer.
	Reconfigure func(cfg C) bool

	// Exited, if set, receives an error when the producer stops on its own,
	// for example after an error it can't recover from. SourceRunner then
	// reports the error and starts a new producer with backoff. Leave it nil
	// for producers that only stop when Stop is called.
	Exited <-chan error
}

// SourceRunnerOptions configures a SourceRunner.
type SourceRunnerOptions[C any] struct {
	Logger *slog.Logger

	// Start starts a producer for cfg. Start may block, for example to bind a
	// port, and may fail; a failed Start must release anything it acquired.
	// A failed Start is retried with backoff until it succeeds or the config
	// changes. ctx is canceled when the component shuts down; it only bounds
	// Start, not the producer's lifetime.
	Start func(ctx context.Context, cfg C) (Producer[C], error)

	// Equal reports whether a running producer for a already matches b, so no
	// restart is needed. Defaults to reflect.DeepEqual.
	Equal func(a, b C) bool

	// DrainTimeout bounds how long Run keeps forwarding entries while the
	// producer stops during shutdown. Entries that can't be delivered in time
	// are dropped. Defaults to DefaultDrainTimeout.
	DrainTimeout time.Duration

	// MinBackoff and MaxBackoff bound the delay between retries of a failed
	// Start. They default to 1s and 1m.
	MinBackoff, MaxBackoff time.Duration

	// OnDrop, if set, is called for every entry dropped during shutdown.
	OnDrop func(Entry)

	// OnShutdown, if set, is called once shutdown starts, before the running
	// producer is stopped. For example, a source can stop its positions file
	// so entries that are still being drained don't advance it.
	OnShutdown func()
}

// SourceRunner runs the producer of a log source and forwards the entries it
// produces to the source's receivers. It implements the lifecycle that
// component.Component asks of components that send data:
//
//   - Update never blocks: it switches the receivers, records the desired
//     config and returns.
//   - Run applies the desired config: it stops the old producer before it
//     starts the new one, retries a failed start with backoff, restarts a
//     producer that exited on its own, and reports all of it as the
//     component's health.
//   - Entries are forwarded on their own goroutine, so stopping a producer
//     that waits for its entries to be handed off never deadlocks.
type SourceRunner[C any] struct {
	opts         SourceRunnerOptions[C]
	entries      LogsReceiver
	batches      LogsBatchReceiver
	fanout       *Fanout
	signal       chan struct{}
	shutdownOnce sync.Once

	mut     sync.Mutex
	desired *C
	health  component.Health
}

// NewSourceRunner creates a SourceRunner. Call Update with the initial config
// before or after Run starts.
func NewSourceRunner[C any](opts SourceRunnerOptions[C]) *SourceRunner[C] {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	if opts.Equal == nil {
		opts.Equal = func(a, b C) bool { return reflect.DeepEqual(a, b) }
	}
	if opts.DrainTimeout == 0 {
		opts.DrainTimeout = DefaultDrainTimeout
	}
	if opts.MinBackoff == 0 {
		opts.MinBackoff = time.Second
	}
	if opts.MaxBackoff == 0 {
		opts.MaxBackoff = time.Minute
	}
	return &SourceRunner[C]{
		opts:    opts,
		entries: NewLogsReceiver(),
		batches: NewLogsBatchReceiver(),
		fanout:  NewFanout(nil),
		signal:  make(chan struct{}, 1),
		health: component.Health{
			Health:     component.HealthTypeUnknown,
			Message:    "not started",
			UpdateTime: time.Now(),
		},
	}
}

// Entries is where producers send single entries.
func (r *SourceRunner[C]) Entries() LogsReceiver { return r.entries }

// Batches is where producers send batches of entries.
func (r *SourceRunner[C]) Batches() LogsBatchReceiver { return r.batches }

// Update switches the receivers entries are sent to and records cfg as the
// desired config. It never blocks. Pass cfg without the receivers in it, so
// a change of receivers alone doesn't restart the producer.
func (r *SourceRunner[C]) Update(forwardTo []LogsReceiver, cfg C) {
	r.fanout.UpdateChildren(forwardTo)

	r.mut.Lock()
	r.desired = &cfg
	r.mut.Unlock()

	r.notify()
}

func (r *SourceRunner[C]) notify() {
	select {
	case r.signal <- struct{}{}:
	default:
	}
}

// CurrentHealth reports whether a producer for the desired config is
// running.
func (r *SourceRunner[C]) CurrentHealth() component.Health {
	r.mut.Lock()
	defer r.mut.Unlock()
	return r.health
}

// Run runs producers for the desired config until ctx is canceled. On
// shutdown it stops the running producer and returns within DrainTimeout
// plus the time the producer takes to stop once its sends no longer block.
func (r *SourceRunner[C]) Run(ctx context.Context) {
	sendCtx, cancelSend := context.WithCancel(context.Background())
	defer cancelSend()

	var (
		wg        sync.WaitGroup
		stopped   = make(chan struct{}) // Closed once no producer is left.
		forwarded = make(chan struct{}) // Closed once forwarding has finished.
		dropped   int
	)

	forward := func(entry Entry) {
		if err := r.fanout.Send(sendCtx, entry); err != nil {
			dropped++
			if r.opts.OnDrop != nil {
				r.opts.OnDrop(entry)
			}
		}
	}
	wg.Go(func() {
		defer close(forwarded)
		for {
			select {
			case entry := <-r.entries.Chan():
				forward(entry)
			case batch := <-r.batches.Chan():
				for _, entry := range batch {
					forward(entry)
				}
			case <-stopped:
				return
			}
		}
	})

	// Once ctx is canceled, give up on entries that can't be delivered within
	// DrainTimeout, so a blocked downstream can't keep a producer from
	// stopping or Run from returning.
	wg.Go(func() {
		select {
		case <-ctx.Done():
		case <-forwarded:
			return
		}
		t := time.NewTimer(r.opts.DrainTimeout)
		defer t.Stop()
		select {
		case <-t.C:
			cancelSend()
		case <-forwarded:
		}
	})

	// Apply the desired config even if an earlier Run consumed the signal.
	r.notify()
	p := r.reconcile(ctx)
	if r.opts.OnShutdown != nil {
		r.shutdownOnce.Do(r.opts.OnShutdown)
	}
	if p != nil {
		p.Stop()
	}
	close(stopped)
	wg.Wait()

	if dropped > 0 {
		r.opts.Logger.Warn("dropped log entries on shutdown because they could not be delivered in time",
			"count", dropped, "drain_timeout", r.opts.DrainTimeout)
	}
}

// reconcile starts, restarts and retries producers until ctx is canceled,
// and returns the producer that is running at that point.
func (r *SourceRunner[C]) reconcile(ctx context.Context) *Producer[C] {
	var (
		running    *Producer[C]
		runningCfg C
		exited     <-chan error // running.Exited, nil if nothing is running.

		failed  *C // Config whose producer last failed; retried by retry.
		backoff time.Duration
		retry   = time.NewTimer(0)
		retryC  <-chan time.Time
	)
	retry.Stop()
	defer retry.Stop()

	fail := func(cfg C, err error) {
		if failed == nil || !r.opts.Equal(*failed, cfg) {
			backoff = 0
		}
		failed = &cfg
		backoff = min(max(2*backoff, r.opts.MinBackoff), r.opts.MaxBackoff)
		r.setHealth(component.HealthTypeUnhealthy, fmt.Sprintf("%s, retrying in %s", err, backoff))
		r.opts.Logger.Warn("log source is not running, will retry", "err", err, "backoff", backoff)
		retry.Reset(backoff)
		retryC = retry.C
	}

	for {
		retrying := false
		select {
		case <-ctx.Done():
			return running
		case <-r.signal:
		case <-retryC:
			retrying, retryC = true, nil
		case err := <-exited:
			running.Stop()
			running, exited = nil, nil
			fail(runningCfg, fmt.Errorf("stopped unexpectedly: %w", err))
			continue
		}

		r.mut.Lock()
		desired := r.desired
		r.mut.Unlock()
		if desired == nil {
			continue
		}
		cfg := *desired

		switch {
		case running != nil:
			if r.opts.Equal(runningCfg, cfg) {
				continue
			}
			if running.Reconfigure != nil && running.Reconfigure(cfg) {
				runningCfg = cfg
				continue
			}
			running.Stop()
			running, exited = nil, nil
		case retryC != nil && !retrying && r.opts.Equal(*failed, cfg):
			// A retry of this config is already scheduled; don't retry
			// early just because Update was called again.
			continue
		}

		p, err := r.opts.Start(ctx, cfg)
		if err == nil && p.Stop == nil {
			err = errors.New("start returned a producer without a Stop function")
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			fail(cfg, fmt.Errorf("failed to start: %w", err))
			continue
		}

		running, runningCfg, exited = &p, cfg, p.Exited
		failed, backoff, retryC = nil, 0, nil
		retry.Stop()
		r.setHealth(component.HealthTypeHealthy, "started")
	}
}

func (r *SourceRunner[C]) setHealth(h component.HealthType, msg string) {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.health = component.Health{Health: h, Message: msg, UpdateTime: time.Now()}
}
