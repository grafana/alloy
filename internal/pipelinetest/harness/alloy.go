package harness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
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
	// SinkID is the default sink, which inline configs send to.
	SinkID string
	// Sinks maps the name a test uses for a sink, such as the exit component
	// it replaces, to the sink's component ID.
	Sinks  map[string]string
	Source string
	// SourceName is the file name used in parse and evaluation errors.
	SourceName string
	// ConfigPath is the config file, or "." for an inline config. Alloy sets
	// module_path from it.
	ConfigPath     string
	DataPath       string
	LokiPipelineV2 bool

	// AssertTimeout bounds how long assertions are retried. Defaults to 30s.
	AssertTimeout time.Duration
	// LogWriter receives the runtime's logs. Logs are discarded when nil.
	LogWriter io.Writer
}

var registerOnce sync.Once

// Register adds the pipelinetest.* components to the registry, once.
// NewAlloy calls it, so only a process that runs tests has them.
func Register() {
	registerOnce.Do(func() {
		registerSink()
		registerSource()
		registerMockHTTP()
	})
}

// NewAlloy creates and starts an in-process Alloy runtime for pipeline tests
// from the provided source.
func NewAlloy(cfg Config) (*Alloy, error) {
	Register()

	logWriter := cfg.LogWriter
	if logWriter == nil {
		logWriter = io.Discard
	}
	problems := newProblemLog(maxProblemLogs)
	logger, err := logging.New(io.MultiWriter(logWriter, problems), logging.DefaultOptions)
	if err != nil {
		return nil, err
	}

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

	assertTimeout := cfg.AssertTimeout
	if assertTimeout <= 0 {
		assertTimeout = defaultTimeout
	}

	a := &Alloy{
		ctrl:          ctrl,
		cancel:        cancel,
		assertTimeout: assertTimeout,
		problems:      problems,
	}

	a.wg.Go(func() {
		ctrl.Run(ctx)
	})

	source, err := alloyruntime.ParseSource(cfg.SourceName, []byte(cfg.Source))
	if err != nil {
		a.Stop()
		return nil, err
	}

	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = "."
	}
	err = ctrl.LoadSource(source, nil, configPath)
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

	a.sinks = []namedSink{{name: "", sink: MustComponent[*Sink](a, cfg.SinkID)}}
	for _, name := range slices.Sorted(maps.Keys(cfg.Sinks)) {
		a.sinks = append(a.sinks, namedSink{name: name, sink: MustComponent[*Sink](a, cfg.Sinks[name])})
	}
	return a, nil
}

type Alloy struct {
	ctrl          *alloyruntime.Runtime
	cancel        func()
	wg            sync.WaitGroup
	assertTimeout time.Duration
	targetRefs    []string
	problems      *problemLog

	sinks []namedSink
}

// ProblemLogs returns Alloy's first distinct warning and error log lines.
func (a *Alloy) ProblemLogs() []string {
	return a.problems.lines()
}

type namedSink struct {
	name string
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
		s := a.snapshot()

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
	}, a.assertTimeout, defaultInterval)
}

// Check is an Assertion with a human-readable name, used for reporting.
type Check struct {
	Name      string
	Assertion Assertion
}

// CheckResult is the outcome of one Check. Err is nil when the check passed.
type CheckResult struct {
	Name string
	Err  error
}

// RunChecks evaluates checks against the current snapshot until they all pass
// or the assertion timeout is reached. Unlike Assert, it reports the outcome
// of every check, and returns the snapshot of the final attempt.
func (a *Alloy) RunChecks(checks []Check) ([]CheckResult, Snapshot) {
	var (
		results []CheckResult
		s       snapshot
	)
	_ = eventually(func() error {
		s = a.snapshot()
		results = make([]CheckResult, len(checks))
		failed := false
		for i, check := range checks {
			results[i] = CheckResult{Name: check.Name, Err: check.Assertion(s)}
			failed = failed || results[i].Err != nil
		}
		if failed {
			return errors.New("checks failed")
		}
		return nil
	}, a.assertTimeout, defaultInterval)

	return results, exportSnapshot(s)
}

// ObserveTargets adds component exports, such as
// discovery.relabel.pods.output, whose targets every snapshot reads so
// assertions can check them.
func (a *Alloy) ObserveTargets(refs ...string) {
	a.targetRefs = append(a.targetRefs, refs...)
}

func (a *Alloy) snapshot() snapshot {
	s := snapshot{bySink: make(map[string]snapshot, len(a.sinks))}
	for _, ns := range a.sinks {
		sub := ns.sink.snapshot()
		s.bySink[ns.name] = sub
		s.loki = append(s.loki, sub.loki...)
		s.prometheus = append(s.prometheus, sub.prometheus...)
		s.spans = append(s.spans, sub.spans...)
		s.lokiSinks = appendName(s.lokiSinks, ns.name, len(sub.loki))
		s.promSinks = appendName(s.promSinks, ns.name, len(sub.prometheus))
		s.spanSinks = appendName(s.spanSinks, ns.name, len(sub.spans))
		s.otlpDataPoints += sub.otlpDataPoints
		s.otlpLogRecords += sub.otlpLogRecords
	}
	if len(a.targetRefs) > 0 {
		s.targets = make(map[string]targetsSnapshot, len(a.targetRefs))
		for _, ref := range a.targetRefs {
			targets, err := a.targetsOf(ref)
			s.targets[ref] = targetsSnapshot{targets: targets, err: err}
		}
	}
	return s
}

func appendName(names []string, name string, n int) []string {
	for range n {
		names = append(names, name)
	}
	return names
}

// UnhealthyComponents lists the components that do not report healthy, with
// their health message. A component that cannot start, for example because its
// port is taken, otherwise only shows up as a failed assertion.
func (a *Alloy) UnhealthyComponents() []ComponentHealth {
	infos, err := a.ctrl.ListComponents("", component.InfoOptions{GetHealth: true})
	if err != nil {
		return nil
	}

	var out []ComponentHealth
	for _, info := range infos {
		if info.Health.Health == component.HealthTypeHealthy {
			continue
		}
		out = append(out, ComponentHealth{
			ID:      info.ID.String(),
			Health:  info.Health.Health.String(),
			Message: info.Health.Message,
		})
	}
	return out
}

// ComponentHealth is the health of one component.
type ComponentHealth struct {
	ID      string `json:"id"`
	Health  string `json:"health"`
	Message string `json:"message,omitempty"`
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
