package harness

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	alloyruntime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"
)

const (
	defaultTimeout  = 10 * time.Second
	defaultInterval = 50 * time.Millisecond
)

type Config struct {
	SinkID   string
	Source   string
	DataPath string
}

// NewAlloy creates and starts an in-process Alloy runtime for pipeline tests
// from the provided source.
func NewAlloy(cfg Config) (*Alloy, error) {
	logger, err := logging.New(io.Discard, logging.DefaultOptions)
	if err != nil {
		return nil, err
	}

	ctrl, err := alloyruntime.New(alloyruntime.Options{
		Logger:       logger,
		DataPath:     cfg.DataPath,
		MinStability: featuregate.StabilityExperimental,
		Services:     defaultServices(logger),
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	a := &Alloy{
		ctrl:   ctrl,
		cancel: cancel,
	}

	a.wg.Go(func() {
		ctrl.Run(ctx)
	})

	source, err := alloyruntime.ParseSource("", []byte(cfg.Source))
	if err != nil {
		a.Stop()
		return nil, err
	}

	err = ctrl.LoadSource(source, nil, "")
	if err != nil {
		a.Stop()
		return nil, err
	}

	if err := eventually(func() error {
		if ctrl.LoadComplete() {
			return nil
		}
		return fmt.Errorf("runtime has not finished loading")
	}, defaultTimeout, defaultInterval); err != nil {
		a.Stop()
		return nil, fmt.Errorf("timed out waiting for runtime to finish loading: %w", err)
	}

	a.sink = MustComponent[*Sink](a, cfg.SinkID)
	return a, nil
}

type Alloy struct {
	ctrl   *alloyruntime.Runtime
	cancel func()
	wg     sync.WaitGroup

	sink *Sink
}

func (a *Alloy) Stop() {
	a.cancel()
	a.wg.Wait()
}

// Assert evaluates the provided assertions against the current snapshot
// until they all pass or the assertion timeout is reached. On failure it
// returns the latest snapshot alongside the assertion failures.
func (a *Alloy) Assert(assertions ...Assertion) error {
	return eventually(func() error {
		s := a.sink.snapshot()

		errs := make([]error, 0, len(assertions))
		for _, assertion := range assertions {
			if err := assertion(s); err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) == 0 {
			return nil
		}

		return AssertionErrors{
			Errors:   errs,
			Snapshot: s,
		}
	}, defaultTimeout, defaultInterval)
}

func MustComponent[T any](a *Alloy, id string) T {
	info, err := a.ctrl.GetComponent(component.ParseID(id), component.InfoOptions{})
	if err != nil {
		panic(fmt.Sprintf("failed to get component %q: %v", id, err))
	}

	typed, ok := info.Component.(T)
	if !ok {
		panic(fmt.Sprintf("component %q has type %T, want %T", id, info.Component, *new(T)))
	}
	return typed
}

func eventually(fn func() error, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)

	var lastErr error
	for {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(interval)
	}
}
