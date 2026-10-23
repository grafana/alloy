package simulation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	alloyruntime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// errDeadlock is returned when a reload doesn't finish in time.
var errDeadlock = errors.New("reload did not finish")

const (
	loadTimeout     = 5 * time.Second
	quiesceTimeout  = 15 * time.Second
	shutdownTimeout = 15 * time.Second
)

var runCounter atomic.Int64

// sim runs a real Alloy runtime with fixture components.
type sim struct {
	t    testing.TB
	run  string
	rec  *recorder
	ctrl *alloyruntime.Runtime

	cancel     context.CancelFunc
	done       chan struct{}
	deadlocked bool
	loadedOnce bool
}

func newSim(t testing.TB) *sim {
	t.Helper()
	run := fmt.Sprintf("run-%d", runCounter.Add(1))
	rec := newRecorder()
	recorders.Store(run, rec)

	logger, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)
	ctrl, err := alloyruntime.New(alloyruntime.Options{
		Logger:       logger,
		DataPath:     t.TempDir(),
		Reg:          prometheus.NewRegistry(),
		MinStability: featuregate.StabilityExperimental,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	s := &sim{t: t, run: run, rec: rec, ctrl: ctrl, cancel: cancel, done: make(chan struct{})}
	go func() {
		ctrl.Run(ctx)
		close(s.done)
	}()
	t.Cleanup(s.shutdown)
	return s
}

func (s *sim) shutdown() {
	defer recorders.Delete(s.run)
	if s.deadlocked {
		// Nothing will unblock on its own.
		s.abort()
	}
	s.cancel()
	select {
	case <-s.done:
		return
	case <-time.After(shutdownTimeout):
	}
	// Something is deadlocked. Unblock the fixtures so the test process can
	// carry on. If that isn't enough, the deadlock is outside the fixtures.
	s.abort()
	select {
	case <-s.done:
		if !s.deadlocked {
			s.t.Errorf("runtime did not shut down within %s", shutdownTimeout)
		}
	case <-time.After(shutdownTimeout):
		s.t.Errorf("runtime is deadlocked outside the fixtures; leaking it")
	}
}

// abort unblocks every fixture, to unwind a deadlocked runtime.
func (s *sim) abort() {
	select {
	case <-s.rec.abort:
	default:
		close(s.rec.abort)
	}
}

// load applies a config and waits until the runtime has stopped removed
// components and started new ones. It returns errDeadlock if that takes
// longer than loadTimeout.
func (s *sim) load(tp topology) error {
	src, err := alloyruntime.ParseSource(s.run, []byte(tp.render(s.run)))
	if err != nil {
		return err
	}
	applied := make(chan error, 1)
	go func() { applied <- s.ctrl.LoadSource(src, nil, "") }()

	deadline := time.After(loadTimeout)
	select {
	case err := <-applied:
		if err != nil {
			if s.loadedOnce {
				// The runtime still schedules the components that did load.
				_ = s.waitScheduled(deadline)
			}
			return err
		}
	case <-deadline:
		s.deadlocked = true
		return fmt.Errorf("%w: components were still being updated after %s", errDeadlock, loadTimeout)
	}
	if err := s.waitScheduled(deadline); err != nil {
		return err
	}
	s.loadedOnce = true
	return nil
}

func (s *sim) waitScheduled(deadline <-chan time.Time) error {
	for !s.ctrl.LoadComplete() {
		select {
		case <-deadline:
			s.deadlocked = true
			return fmt.Errorf("%w: components were still being stopped or started after %s", errDeadlock, loadTimeout)
		case <-time.After(time.Millisecond):
		}
	}
	return nil
}

func (s *sim) mustLoad(tp topology) {
	s.t.Helper()
	require.NoError(s.t, s.load(tp))
}

// verify checks the pipeline after the last reload of tp. It returns an
// error if:
//   - a source never runs a producer for its latest generation,
//   - traffic stops,
//   - an entry was lost, duplicated, or dropped by a component allowDrop
//     doesn't allow,
//   - a fixture saw a contract violation, or
//   - an entry produced after every source runs its latest config doesn't
//     reach exactly the sinks, and paths, that tp sends it to.
func (s *sim) verify(tp topology, allowDrop func(component string) bool) error {
	deadline := time.Now().Add(quiesceTimeout)
	for name, spec := range tp.sources {
		for s.rec.runningGeneration("lokisim.source."+name) != spec.generation {
			if time.Now().After(deadline) {
				return fmt.Errorf("source %s never ran generation %d (running: %d)",
					name, spec.generation, s.rec.runningGeneration("lokisim.source."+name))
			}
			time.Sleep(time.Millisecond)
		}
	}

	epoch := s.rec.nextID.Load()
	if err := flowing(s); err != nil {
		return err
	}

	// Stop producing, then wait until every entry has gone everywhere it
	// should and nothing changes any more.
	s.rec.paused.Store(true)
	defer s.rec.paused.Store(false)
	var (
		a     accounting
		wrong []string
		last  = -1
	)
	for {
		a, wrong = s.rec.account(), s.wrongPaths(tp, epoch)
		settled := a.produced == last && len(a.missing) == 0 && len(wrong) == 0
		if settled || time.Now().After(deadline.Add(quiesceTimeout)) {
			break
		}
		last = a.produced
		time.Sleep(50 * time.Millisecond)
	}

	var errs []string
	if a.produced == 0 {
		errs = append(errs, "nothing was produced")
	}
	if len(a.missing) > 0 {
		errs = append(errs, "entries lost")
	}
	if len(a.duplicates) > 0 {
		errs = append(errs, "entries duplicated")
	}
	if len(a.violations) > 0 {
		errs = append(errs, "contract violated")
	}
	for by := range a.droppedBy {
		if !allowDrop(by) {
			errs = append(errs, by+" dropped entries")
		}
	}

	if len(wrong) > 0 {
		slices.Sort(wrong)
		errs = append(errs, fmt.Sprintf("%d entries reached the wrong sinks, for example: %v", len(wrong), wrong[:min(3, len(wrong))]))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s: %s", strings.Join(errs, ", "), a)
	}
	return nil
}

func (s *sim) health(id string) component.Health {
	info, err := s.ctrl.GetComponent(component.ParseID(id), component.InfoOptions{GetHealth: true})
	if err != nil {
		return component.Health{Health: component.HealthTypeUnknown, Message: err.Error()}
	}
	return info.Health
}

func (s *sim) requireEventuallyHealth(id string, want component.HealthType) {
	s.t.Helper()
	require.Eventually(s.t, func() bool {
		return s.health(id).Health == want
	}, quiesceTimeout, 5*time.Millisecond, "%s never became %s, last health: %+v", id, want, s.health(id))
}

// topology is a pipeline of fixture components that renders to a config.
// Targets are written "sink.<name>" or "relay.<name>".
type topology struct {
	sinks   map[string]sinkSpec
	relays  map[string]relaySpec
	sources map[string]sourceSpec
}

type sinkSpec struct {
	delay, startDelay time.Duration
}

type relaySpec struct {
	to []string
}

type sourceSpec struct {
	to            []string
	mode          string
	generation    int
	resource      string
	startFailures int
	blockingSends bool
	batchSize     int
	interval      time.Duration
	drainTimeout  time.Duration
}

func newTopology() topology {
	return topology{sinks: map[string]sinkSpec{}, relays: map[string]relaySpec{}, sources: map[string]sourceSpec{}}
}

func (tp topology) clone() topology {
	c := newTopology()
	maps.Copy(c.sinks, tp.sinks)
	for k, v := range tp.relays {
		v.to = slices.Clone(v.to)
		c.relays[k] = v
	}
	for k, v := range tp.sources {
		v.to = slices.Clone(v.to)
		c.sources[k] = v
	}
	return c
}

func (tp topology) render(run string) string {
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(tp.sinks)) {
		s := tp.sinks[name]
		fmt.Fprintf(&b, "lokisim.sink %q {\n\trun = %q\n", name, run)
		writeDuration(&b, "delay", s.delay)
		writeDuration(&b, "start_delay", s.startDelay)
		b.WriteString("}\n")
	}
	for _, name := range slices.Sorted(maps.Keys(tp.relays)) {
		fmt.Fprintf(&b, "lokisim.relay %q {\n\trun = %q\n\tforward_to = %s\n}\n", name, run, receivers(tp.relays[name].to))
	}
	for _, name := range slices.Sorted(maps.Keys(tp.sources)) {
		s := tp.sources[name]
		fmt.Fprintf(&b, "lokisim.source %q {\n\trun = %q\n\tforward_to = %s\n", name, run, receivers(s.to))
		if s.mode != "" {
			fmt.Fprintf(&b, "\tmode = %q\n", s.mode)
		}
		fmt.Fprintf(&b, "\tgeneration = %d\n\tstart_failures = %d\n\tblocking_sends = %t\n\tbatch_size = %d\n",
			s.generation, s.startFailures, s.blockingSends, s.batchSize)
		if s.resource != "" {
			fmt.Fprintf(&b, "\tresource = %q\n", s.resource)
		}
		writeDuration(&b, "interval", s.interval)
		writeDuration(&b, "drain_timeout", s.drainTimeout)
		b.WriteString("}\n")
	}
	return b.String()
}

// wrongPaths describes entries produced after epoch that didn't reach
// exactly the sinks, and paths, that tp sends them to.
func (s *sim) wrongPaths(tp topology, epoch int64) []string {
	var wrong []string
	for id, d := range s.rec.deliveries(epoch) {
		want := tp.paths(strings.TrimPrefix(d.source, "lokisim.source."))
		if !maps.Equal(want, d.sinks) {
			wrong = append(wrong, fmt.Sprintf("entry %d from %s reached %v, want %v", id, d.source, d.sinks, want))
		}
	}
	return wrong
}

// paths returns every sink, and the relays on the way, that an entry from
// source reaches, keyed like recorder.markDelivered.
func (tp topology) paths(source string) map[string]int {
	paths := map[string]int{}
	var walk func(targets []string, via string)
	walk = func(targets []string, via string) {
		for _, t := range targets {
			kind, name, _ := strings.Cut(t, ".")
			if kind == "relay" {
				walk(tp.relays[name].to, via+">lokisim.relay."+name)
				continue
			}
			key := "lokisim.sink." + name
			if via != "" {
				key += " via " + via
			}
			paths[key]++
		}
	}
	walk(tp.sources[source].to, "")
	return paths
}

func receivers(targets []string) string {
	refs := make([]string, len(targets))
	for i, t := range targets {
		refs[i] = "lokisim." + t + ".receiver"
	}
	return "[" + strings.Join(refs, ", ") + "]"
}

func writeDuration(b *strings.Builder, name string, d time.Duration) {
	if d > 0 {
		fmt.Fprintf(b, "\t%s = %q\n", name, d.String())
	}
}
