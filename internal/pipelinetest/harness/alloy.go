package harness

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	alloyruntime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"
)

const (
	defaultTimeout  = 30 * time.Second
	defaultInterval = 50 * time.Millisecond
)

type Config struct {
	SinkID         string
	Source         string
	DataPath       string
	LokiPipelineV2 bool

	// AssertTimeout is how long Assert waits for its assertions to pass.
	// It defaults to 30 seconds.
	AssertTimeout time.Duration

	// StableFor is how long all assertions must keep passing before Assert
	// succeeds. A negative assertion, such as a count of zero, can pass before
	// the data it guards arrives. A stable window gives that data time to
	// arrive. The window counts against AssertTimeout.
	StableFor time.Duration
}

// assertTimeout returns the assertion timeout, or the default when unset.
func (c Config) assertTimeout() time.Duration {
	if c.AssertTimeout <= 0 {
		return defaultTimeout
	}
	return c.AssertTimeout
}

// NewAlloy creates and starts an in-process Alloy runtime for pipeline tests
// from the provided source.
func NewAlloy(cfg Config) (*Alloy, error) {
	if cfg.StableFor > 0 && cfg.StableFor >= cfg.assertTimeout() {
		return nil, fmt.Errorf("the stable window of %s does not fit in the assertion timeout of %s", cfg.StableFor, cfg.assertTimeout())
	}

	logs := &logBuffer{}
	logger, err := logging.New(logs, logging.DefaultOptions)
	if err != nil {
		return nil, err
	}

	assertTimeout := cfg.assertTimeout()

	ctrl, err := alloyruntime.New(alloyruntime.Options{
		Logger:   logger,
		DataPath: cfg.DataPath,
		// Using DefaultRegisterer prevents us from running pipeline tests in
		// parallel. It is required because prometheus.exporter.self only serves
		// the default registry.
		Reg:          prometheus.DefaultRegisterer,
		MinStability: featuregate.StabilityExperimental,
		Services:     defaultServices(logger, cfg),
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	a := &Alloy{
		ctrl:          ctrl,
		cancel:        cancel,
		logs:          logs,
		assertTimeout: assertTimeout,
		stableFor:     cfg.StableFor,
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

	logs          *logBuffer
	assertTimeout time.Duration
	stableFor     time.Duration
}

func (a *Alloy) Stop() {
	a.cancel()
	a.wg.Wait()
}

// Assert evaluates the provided assertions against the current snapshot
// until they all pass, and keep passing for the stable window, or the
// assertion timeout is reached. On failure it returns the latest snapshot
// alongside the assertion failures.
//
// It returns early when the stable window can no longer fit before the
// timeout. A passing streak ends at its start plus the window, and a failing
// poll can at best start a streak now, so waiting longer cannot help.
func (a *Alloy) Assert(assertions ...Assertion) error {
	deadline := time.Now().Add(a.assertTimeout)
	var passingSince time.Time

	// windowFits reports whether a streak that starts at start can finish.
	windowFits := func(start time.Time) bool {
		return a.stableFor <= 0 || !start.Add(a.stableFor).After(deadline)
	}

	return eventually(func() error {
		s := a.sink.snapshot()
		s.logs = a.logs.String()

		errs := make([]error, 0, len(assertions))
		for _, assertion := range assertions {
			if err := assertion(s); err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			passingSince = time.Time{}
			if !windowFits(time.Now()) {
				errs = append(errs, AssertionError{
					Kind:    "stable",
					Message: fmt.Sprintf("not enough time left for the stable window of %s", a.stableFor),
				})
				return fatalError{AssertionErrors{Errors: errs, Snapshot: s}}
			}
			return AssertionErrors{Errors: errs, Snapshot: s}
		}

		if a.stableFor <= 0 {
			return nil
		}
		if passingSince.IsZero() {
			passingSince = time.Now()
		}
		if !windowFits(passingSince) {
			return fatalError{AssertionErrors{
				Errors: []error{AssertionError{
					Kind:    "stable",
					Message: fmt.Sprintf("assertions pass, but the stable window of %s cannot finish before the timeout", a.stableFor),
				}},
				Snapshot: s,
			}}
		}
		if time.Since(passingSince) < a.stableFor {
			return AssertionErrors{
				Errors: []error{AssertionError{
					Kind:    "stable",
					Message: fmt.Sprintf("assertions pass but have not held for %s yet", a.stableFor),
				}},
				Snapshot: s,
			}
		}
		return nil
	}, a.assertTimeout, defaultInterval)
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

// fatalError makes eventually stop and return the wrapped error at once.
type fatalError struct{ err error }

func (e fatalError) Error() string { return e.err.Error() }

func eventually(fn func() error, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)

	var lastErr error
	for {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if fatal, ok := lastErr.(fatalError); ok {
			return fatal.err
		}

		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(interval)
	}
}
