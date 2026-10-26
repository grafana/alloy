package simulation

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
)

// A scenario drives a pipeline through reloads and returns an error if the
// lifecycle under test broke: a reload deadlocked or failed, traffic stopped,
// or entries were lost. It uses s.t only for failures of the harness itself.
type scenario struct {
	name string
	run  func(s *sim, mode string) error
}

var scenarios = []scenario{
	{
		// The deadlock from grafana/alloy#7306: a reload adds a component and
		// points a restarting source at it.
		name: "reroute_to_new_sink_with_restart",
		run: func(s *sim, mode string) error {
			tp := newTopology()
			tp.sinks["a"] = sinkSpec{}
			tp.sources["s"] = sourceSpec{to: []string{"sink.a"}, mode: mode, blockingSends: true}
			if err := s.load(tp); err != nil {
				return err
			}
			if err := flowing(s); err != nil {
				return err
			}

			tp = tp.clone()
			delete(tp.sinks, "a")
			tp.sinks["b"] = sinkSpec{}
			src := tp.sources["s"]
			src.to, src.generation = []string{"sink.b"}, 1
			tp.sources["s"] = src
			if err := s.load(tp); err != nil {
				return err
			}
			return delivered(s, tp)
		},
	},
	{
		// The case from database_observability.mysql: the new config can't
		// start, while the old downstream is removed.
		name: "reroute_when_start_fails",
		run: func(s *sim, mode string) error {
			tp := newTopology()
			tp.sinks["a"] = sinkSpec{}
			tp.sources["s"] = sourceSpec{to: []string{"sink.a"}, mode: mode}
			if err := s.load(tp); err != nil {
				return err
			}
			if err := flowing(s); err != nil {
				return err
			}

			tp = tp.clone()
			delete(tp.sinks, "a")
			tp.sinks["b"] = sinkSpec{}
			src := tp.sources["s"]
			src.to, src.generation, src.startFailures = []string{"sink.b"}, 1, 3
			tp.sources["s"] = src
			if err := s.load(tp); err != nil {
				return err
			}
			return delivered(s, tp)
		},
	},
	{
		// A processor and its upstream source are both rerouted to new
		// components in one reload, while the source restarts.
		name: "reroute_chain",
		run: func(s *sim, mode string) error {
			tp := newTopology()
			tp.sinks["a"] = sinkSpec{}
			tp.relays["r"] = relaySpec{to: []string{"sink.a"}}
			tp.sources["s"] = sourceSpec{to: []string{"relay.r"}, mode: mode, blockingSends: true}
			if err := s.load(tp); err != nil {
				return err
			}
			if err := flowing(s); err != nil {
				return err
			}

			tp = tp.clone()
			delete(tp.sinks, "a")
			tp.sinks["b"] = sinkSpec{}
			tp.sinks["c"] = sinkSpec{}
			tp.relays["r"] = relaySpec{to: []string{"sink.b"}}
			src := tp.sources["s"]
			src.to, src.generation = []string{"relay.r", "sink.c"}, 1
			tp.sources["s"] = src
			if err := s.load(tp); err != nil {
				return err
			}
			return delivered(s, tp)
		},
	},
	{
		// A component is renamed and keeps its port. The new one can only bind
		// once the old one has stopped.
		name: "rename_with_exclusive_resource",
		run: func(s *sim, mode string) error {
			tp := newTopology()
			tp.sinks["a"] = sinkSpec{}
			tp.sources["old"] = sourceSpec{to: []string{"sink.a"}, mode: mode, resource: "port"}
			if err := s.load(tp); err != nil {
				return err
			}
			if err := flowing(s); err != nil {
				return err
			}

			tp = tp.clone()
			tp.sources["new"] = tp.sources["old"]
			delete(tp.sources, "old")
			if err := s.load(tp); err != nil {
				return err
			}
			return delivered(s, tp)
		},
	},
	{
		// The port is taken by something else for a while. The source must
		// report it and recover once the port is free.
		name: "resource_held_elsewhere",
		run: func(s *sim, mode string) error {
			require.NoError(s.t, s.rec.acquire("port", "external"))
			tp := newTopology()
			tp.sinks["a"] = sinkSpec{}
			tp.sources["s"] = sourceSpec{to: []string{"sink.a"}, mode: mode, resource: "port"}
			if err := s.load(tp); err != nil {
				return err
			}
			if mode == modeRunner {
				s.requireEventuallyHealth("lokisim.source.s", component.HealthTypeUnhealthy)
			}
			s.rec.release("port", "external")
			return delivered(s, tp)
		},
	},
	{
		// Many reloads in a row, each adding a sink that starts late, moving
		// the source to it and restarting the source.
		name: "reload_storm",
		run: func(s *sim, mode string) error {
			tp := newTopology()
			tp.sinks["s0"] = sinkSpec{}
			tp.sources["s"] = sourceSpec{to: []string{"sink.s0"}, mode: mode, blockingSends: true}
			if err := s.load(tp); err != nil {
				return err
			}
			for i := 1; i <= 30; i++ {
				tp = tp.clone()
				delete(tp.sinks, fmt.Sprintf("s%d", i-1))
				tp.sinks[fmt.Sprintf("s%d", i)] = sinkSpec{startDelay: time.Duration(i%3) * time.Millisecond}
				src := tp.sources["s"]
				src.to, src.generation = []string{fmt.Sprintf("sink.s%d", i)}, i
				tp.sources["s"] = src
				if err := s.load(tp); err != nil {
					return fmt.Errorf("reload %d: %w", i, err)
				}
			}
			return delivered(s, tp)
		},
	},
	{
		// A removed source drains into a processor that the same reload
		// points at a new sink. The scheduler only starts the new sink once
		// the removed source has exited, so the source can only wait out its
		// drain timeout and drop what it holds.
		name: "remove_source_draining_into_rerouted_processor",
		run: func(s *sim, mode string) error {
			tp := newTopology()
			tp.sinks["a"] = sinkSpec{}
			tp.relays["r"] = relaySpec{to: []string{"sink.a"}}
			tp.sources["removed"] = sourceSpec{to: []string{"relay.r"}, mode: mode, drainTimeout: 200 * time.Millisecond}
			tp.sources["kept"] = sourceSpec{to: []string{"relay.r"}, mode: mode}
			if err := s.load(tp); err != nil {
				return err
			}
			if err := flowing(s); err != nil {
				return err
			}

			tp = tp.clone()
			delete(tp.sources, "removed")
			delete(tp.sinks, "a")
			tp.sinks["b"] = sinkSpec{}
			tp.relays["r"] = relaySpec{to: []string{"sink.b"}}
			if err := s.load(tp); err != nil {
				return err
			}
			return s.verify(tp, func(by string) bool { return by == "lokisim.source.removed" })
		},
	},
	{
		// Like the scenario above, with several removed sources. The scheduler
		// stops removed components of a pipeline one at a time, so the reload
		// takes up to one drain timeout per removed source. This is a limit of
		// the runtime, not of the component: the scenario checks that the
		// stall stays bounded.
		name: "remove_many_sources_draining_into_rerouted_processor",
		run: func(s *sim, mode string) error {
			const removed, drain = 3, 300 * time.Millisecond
			tp := newTopology()
			tp.sinks["a"] = sinkSpec{}
			tp.relays["r"] = relaySpec{to: []string{"sink.a"}}
			tp.sources["kept"] = sourceSpec{to: []string{"relay.r"}, mode: mode}
			for i := range removed {
				tp.sources[fmt.Sprintf("removed%d", i)] = sourceSpec{to: []string{"relay.r"}, mode: mode, drainTimeout: drain}
			}
			if err := s.load(tp); err != nil {
				return err
			}
			if err := flowing(s); err != nil {
				return err
			}

			tp = tp.clone()
			for i := range removed {
				delete(tp.sources, fmt.Sprintf("removed%d", i))
			}
			delete(tp.sinks, "a")
			tp.sinks["b"] = sinkSpec{}
			tp.relays["r"] = relaySpec{to: []string{"sink.b"}}
			start := time.Now()
			if err := s.load(tp); err != nil {
				return err
			}
			if took, limit := time.Since(start), removed*drain+2*time.Second; took > limit {
				return fmt.Errorf("reload took %s, want at most %s", took, limit)
			}
			return s.verify(tp, func(by string) bool { return strings.HasPrefix(by, "lokisim.source.removed") })
		},
	},
}

// flowing returns an error if entries stop reaching sinks.
func flowing(s *sim) error {
	start := s.rec.reached.Load()
	deadline := time.Now().Add(loadTimeout)
	for s.rec.reached.Load() < start+100 {
		if time.Now().After(deadline) {
			return fmt.Errorf("traffic stopped: %s", s.rec.account())
		}
		time.Sleep(time.Millisecond)
	}
	return nil
}

// delivered verifies tp and allows no drops.
func delivered(s *sim, tp topology) error {
	return s.verify(tp, func(string) bool { return false })
}

// TestScenarios runs every scenario with SourceRunner. All of them must pass.
func TestScenarios(t *testing.T) {
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			s := newSim(t)
			require.NoError(t, sc.run(s, modeRunner))
			for id := range sourcesIn(s) {
				s.requireEventuallyHealth(id, component.HealthTypeHealthy)
			}
		})
	}
}

// TestLegacyLifecycles runs the same scenarios with the lifecycles that
// components use today, and checks that the simulation catches how they
// break. If one of these starts passing, the fixture no longer models the
// legacy lifecycle.
func TestLegacyLifecycles(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for deadlocks to time out")
	}

	// Scenario -> mode -> text of the error the scenario must end with. A
	// missing entry means the scenario must pass.
	const (
		deadlock     = "reload did not finish"
		startFailed  = "injected start failure"
		resourceHeld = "is held by"
	)
	want := map[string]map[string]string{
		"reroute_to_new_sink_with_restart": {modeLegacy: deadlock},
		"reroute_when_start_fails":         {modeLegacy: startFailed, modeLegacyDeferred: startFailed},
		"reroute_chain":                    {modeLegacy: deadlock, modeLegacyDeferred: deadlock},
		"rename_with_exclusive_resource":   {modeLegacy: resourceHeld, modeLegacyDeferred: resourceHeld},
		"resource_held_elsewhere":          {modeLegacy: resourceHeld, modeLegacyDeferred: resourceHeld},
		"reload_storm":                     {modeLegacy: deadlock},
	}

	for _, sc := range scenarios {
		for _, mode := range []string{modeLegacy, modeLegacyDeferred} {
			t.Run(sc.name+"/"+mode, func(t *testing.T) {
				s := newSim(t)
				err := sc.run(s, mode)
				t.Logf("result: %v", err)
				if wantErr := want[sc.name][mode]; wantErr != "" {
					require.ErrorContains(t, err, wantErr)
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
}

func sourcesIn(s *sim) map[string]bool {
	ids := map[string]bool{}
	for _, info := range s.componentInfos() {
		if strings.HasPrefix(info, "lokisim.source.") {
			ids[info] = true
		}
	}
	return ids
}

func (s *sim) componentInfos() []string {
	infos, err := s.ctrl.ListComponents("", component.InfoOptions{})
	require.NoError(s.t, err)
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		ids = append(ids, info.ID.LocalID)
	}
	return ids
}
