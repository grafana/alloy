package loki

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
)

// testProducer sends entries to out until stopped. If blocking is set, it
// finishes handing off its current entry before it stops.
type testProducer struct {
	stop, done chan struct{}
	stopped    atomic.Bool
}

func startTestProducer(out chan<- Entry, blocking bool) *testProducer {
	p := &testProducer{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(p.done)
		for {
			if blocking {
				out <- Entry{}
				select {
				case <-p.stop:
					return
				default:
				}
				continue
			}
			select {
			case out <- Entry{}:
			case <-p.stop:
				return
			}
		}
	}()
	return p
}

func (p *testProducer) Stop() {
	close(p.stop)
	<-p.done
	p.stopped.Store(true)
}

func (p *testProducer) producer() Producer[testConfig] {
	return Producer[testConfig]{Stop: p.Stop}
}

type testConfig struct{ generation int }

// consumeAll reads recv until the test ends.
func consumeAll(t *testing.T, recv LogsReceiver) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-recv.Chan():
			case <-done:
				return
			}
		}
	})
	t.Cleanup(func() {
		close(done)
		wg.Wait()
	})
}

func runRunner(t *testing.T, r *SourceRunner[testConfig]) (stop func()) {
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("Run did not return")
		}
	}
}

func TestSourceRunner_UpdateDoesNotBlockOnUnstartedDownstream(t *testing.T) {
	var starts atomic.Int32
	var r *SourceRunner[testConfig]
	r = NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(_ context.Context, _ testConfig) (Producer[testConfig], error) {
			starts.Add(1)
			return startTestProducer(r.Entries().Chan(), true).producer(), nil
		},
	})
	running, notStarted := NewLogsReceiver(), NewLogsReceiver()
	consumeAll(t, running)

	r.Update([]LogsReceiver{running}, testConfig{generation: 0})
	stop := runRunner(t, r)
	defer stop()
	require.Eventually(t, func() bool { return starts.Load() == 1 }, 5*time.Second, time.Millisecond)

	// Switch to a receiver nobody reads yet and force a restart, like a reload
	// that adds a component. Update must return right away.
	updated := make(chan struct{})
	go func() {
		r.Update([]LogsReceiver{notStarted}, testConfig{generation: 1})
		close(updated)
	}()
	select {
	case <-updated:
	case <-time.After(5 * time.Second):
		t.Fatal("Update blocked")
	}

	// Once the new receiver is read, the restart completes.
	consumeAll(t, notStarted)
	require.Eventually(t, func() bool { return starts.Load() == 2 }, 5*time.Second, time.Millisecond)
}

func TestSourceRunner_RetriesFailedStartAndReportsHealth(t *testing.T) {
	var attempts atomic.Int32
	var r *SourceRunner[testConfig]
	r = NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(_ context.Context, _ testConfig) (Producer[testConfig], error) {
			if attempts.Add(1) <= 3 {
				return Producer[testConfig]{}, errors.New("port in use")
			}
			return startTestProducer(r.Entries().Chan(), false).producer(), nil
		},
		MinBackoff:   time.Millisecond,
		MaxBackoff:   10 * time.Millisecond,
		DrainTimeout: 10 * time.Millisecond,
	})
	recv := NewLogsReceiver()
	r.Update([]LogsReceiver{recv}, testConfig{})
	require.Equal(t, component.HealthTypeUnknown, r.CurrentHealth().Health)

	stop := runRunner(t, r)
	defer stop()

	require.Eventually(t, func() bool {
		return r.CurrentHealth().Health == component.HealthTypeUnhealthy
	}, 5*time.Second, time.Millisecond)
	require.Contains(t, r.CurrentHealth().Message, "port in use")

	<-recv.Chan()
	require.Eventually(t, func() bool {
		return r.CurrentHealth().Health == component.HealthTypeHealthy
	}, 5*time.Second, time.Millisecond)
	require.EqualValues(t, 4, attempts.Load())
}

func TestSourceRunner_StopsOldProducerBeforeStartingNew(t *testing.T) {
	var (
		mut     sync.Mutex
		current *testProducer
		lastGen int
		overlap atomic.Bool
		starts  atomic.Int32
	)
	var r *SourceRunner[testConfig]
	r = NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(_ context.Context, cfg testConfig) (Producer[testConfig], error) {
			mut.Lock()
			defer mut.Unlock()
			if current != nil && !current.stopped.Load() {
				overlap.Store(true)
			}
			current, lastGen = startTestProducer(r.Entries().Chan(), true), cfg.generation
			starts.Add(1)
			return current.producer(), nil
		},
	})
	recv := NewLogsReceiver()
	consumeAll(t, recv)
	r.Update([]LogsReceiver{recv}, testConfig{})
	stop := runRunner(t, r)
	defer stop()
	require.Eventually(t, func() bool { return starts.Load() == 1 }, 5*time.Second, time.Millisecond)

	for i := 1; i <= 50; i++ {
		r.Update([]LogsReceiver{recv}, testConfig{generation: i})
		if i%10 == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	// Updates coalesce, but the last config always ends up running.
	require.Eventually(t, func() bool {
		mut.Lock()
		defer mut.Unlock()
		return lastGen == 50
	}, 5*time.Second, time.Millisecond)
	require.False(t, overlap.Load(), "a producer started while the previous one was running")
}

func TestSourceRunner_UpdatesDoNotSkipBackoff(t *testing.T) {
	var starts atomic.Int32
	r := NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(context.Context, testConfig) (Producer[testConfig], error) {
			starts.Add(1)
			return Producer[testConfig]{}, errors.New("port in use")
		},
		MinBackoff: time.Hour,
		MaxBackoff: time.Hour,
	})
	r.Update(nil, testConfig{})
	stop := runRunner(t, r)
	defer stop()
	require.Eventually(t, func() bool { return starts.Load() == 1 }, 5*time.Second, time.Millisecond)

	// Updates that only change the receivers wait for the scheduled retry.
	for range 20 {
		r.Update([]LogsReceiver{NewLogsReceiver()}, testConfig{})
		time.Sleep(time.Millisecond)
	}
	require.EqualValues(t, 1, starts.Load())

	// A different config is tried right away.
	r.Update(nil, testConfig{generation: 1})
	require.Eventually(t, func() bool { return starts.Load() == 2 }, 5*time.Second, time.Millisecond)
}

func TestSourceRunner_RestartsProducerThatExited(t *testing.T) {
	// Each producer exits with an error when its channel is closed.
	producers := make(chan chan struct{}, 10)
	r := NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(context.Context, testConfig) (Producer[testConfig], error) {
			exit, exited, done := make(chan struct{}), make(chan error, 1), make(chan struct{})
			go func() {
				defer close(done)
				<-exit
				exited <- errors.New("lost connection")
			}()
			producers <- exit
			return Producer[testConfig]{Stop: func() { <-done }, Exited: exited}, nil
		},
		MinBackoff: 10 * time.Millisecond,
		MaxBackoff: 10 * time.Millisecond,
	})
	r.Update(nil, testConfig{})
	stop := runRunner(t, r)
	defer stop()

	first := <-producers
	require.Eventually(t, func() bool {
		return r.CurrentHealth().Health == component.HealthTypeHealthy
	}, 5*time.Second, time.Millisecond)

	close(first)
	require.Eventually(t, func() bool {
		return r.CurrentHealth().Health == component.HealthTypeUnhealthy
	}, 5*time.Second, time.Millisecond)
	require.Contains(t, r.CurrentHealth().Message, "lost connection")

	second := <-producers
	require.Eventually(t, func() bool {
		return r.CurrentHealth().Health == component.HealthTypeHealthy
	}, 5*time.Second, time.Millisecond)
	close(second)
}

func TestSourceRunner_RunAgainStartsProducer(t *testing.T) {
	var starts, shutdowns atomic.Int32
	var r *SourceRunner[testConfig]
	r = NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(context.Context, testConfig) (Producer[testConfig], error) {
			starts.Add(1)
			return startTestProducer(r.Entries().Chan(), false).producer(), nil
		},
		OnShutdown: func() { shutdowns.Add(1) },
	})
	recv := NewLogsReceiver()
	consumeAll(t, recv)
	r.Update([]LogsReceiver{recv}, testConfig{})

	for i := 1; i <= 2; i++ {
		stop := runRunner(t, r)
		require.Eventually(t, func() bool { return starts.Load() == int32(i) }, 5*time.Second, time.Millisecond)
		stop()
	}
	require.EqualValues(t, 1, shutdowns.Load(), "OnShutdown must run once")
}

func TestSourceRunner_SameConfigDoesNotRestart(t *testing.T) {
	var starts atomic.Int32
	var r *SourceRunner[testConfig]
	r = NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(_ context.Context, _ testConfig) (Producer[testConfig], error) {
			starts.Add(1)
			return startTestProducer(r.Entries().Chan(), false).producer(), nil
		},
		DrainTimeout: 10 * time.Millisecond,
	})
	a, b := NewLogsReceiver(), NewLogsReceiver()
	r.Update([]LogsReceiver{a}, testConfig{})
	stop := runRunner(t, r)
	defer stop()

	<-a.Chan()
	// Only the receivers change: the producer keeps running.
	r.Update([]LogsReceiver{b}, testConfig{})
	<-b.Chan()
	<-b.Chan()
	require.EqualValues(t, 1, starts.Load())
}

func TestSourceRunner_Reconfigure(t *testing.T) {
	var starts, applied atomic.Int32
	var r *SourceRunner[testConfig]
	r = NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(_ context.Context, _ testConfig) (Producer[testConfig], error) {
			starts.Add(1)
			p := startTestProducer(r.Entries().Chan(), false).producer()
			// Odd generations are applied in place, even ones need a restart.
			p.Reconfigure = func(cfg testConfig) bool {
				if cfg.generation%2 == 0 {
					return false
				}
				applied.Add(1)
				return true
			}
			return p, nil
		},
	})
	recv := NewLogsReceiver()
	consumeAll(t, recv)
	r.Update([]LogsReceiver{recv}, testConfig{generation: 0})
	stop := runRunner(t, r)
	defer stop()
	require.Eventually(t, func() bool { return starts.Load() == 1 }, 5*time.Second, time.Millisecond)

	r.Update([]LogsReceiver{recv}, testConfig{generation: 1})
	require.Eventually(t, func() bool { return applied.Load() == 1 }, 5*time.Second, time.Millisecond)
	require.EqualValues(t, 1, starts.Load())

	r.Update([]LogsReceiver{recv}, testConfig{generation: 2})
	require.Eventually(t, func() bool { return starts.Load() == 2 }, 5*time.Second, time.Millisecond)
}

func TestSourceRunner_ShutdownIsBoundedWhenDownstreamBlocks(t *testing.T) {
	var dropped atomic.Int32
	var r *SourceRunner[testConfig]
	r = NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(_ context.Context, _ testConfig) (Producer[testConfig], error) {
			return startTestProducer(r.Entries().Chan(), true).producer(), nil
		},
		DrainTimeout: 50 * time.Millisecond,
		OnDrop:       func(Entry) { dropped.Add(1) },
	})
	// Nobody ever reads this receiver.
	r.Update([]LogsReceiver{NewLogsReceiver()}, testConfig{})
	stop := runRunner(t, r)
	time.Sleep(20 * time.Millisecond)

	start := time.Now()
	stop()
	require.Less(t, time.Since(start), 5*time.Second)
	require.Positive(t, dropped.Load())
}

func TestSourceRunner_ProducerWithoutStopIsAnError(t *testing.T) {
	r := NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(context.Context, testConfig) (Producer[testConfig], error) {
			return Producer[testConfig]{}, nil
		},
	})
	r.Update(nil, testConfig{})
	stop := runRunner(t, r)
	defer stop()
	require.Eventually(t, func() bool {
		return r.CurrentHealth().Health == component.HealthTypeUnhealthy
	}, 5*time.Second, time.Millisecond)
	require.Contains(t, r.CurrentHealth().Message, "without a Stop function")
}

func TestSourceRunner_ShutdownWhileStartBlocks(t *testing.T) {
	started := make(chan struct{})
	r := NewSourceRunner(SourceRunnerOptions[testConfig]{
		Start: func(ctx context.Context, _ testConfig) (Producer[testConfig], error) {
			close(started)
			<-ctx.Done()
			return Producer[testConfig]{}, ctx.Err()
		},
	})
	r.Update(nil, testConfig{})
	stop := runRunner(t, r)
	<-started
	stop()
}
