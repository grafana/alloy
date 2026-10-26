package simulation

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
)

// TestRandomReloads drives SourceRunner-based sources through random reloads
// while entries flow: sinks and processors come and go, receivers change,
// sources restart, fail to start, get renamed and removed. It checks that no
// reload deadlocks, that every entry is delivered exactly once except those
// dropped by removed components, and that every source ends up healthy.
//
// LOKISIM_SEEDS and LOKISIM_STEPS scale the run, for example
// LOKISIM_SEEDS=200 LOKISIM_STEPS=200 for a long soak.
func TestRandomReloads(t *testing.T) {
	seeds, steps := envInt("LOKISIM_SEEDS", 8), envInt("LOKISIM_STEPS", 40)
	if testing.Short() {
		seeds, steps = 2, 15
	}
	for seed := range seeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			t.Parallel()
			randomReloads(t, uint64(seed), steps)
		})
	}
}

func randomReloads(t *testing.T, seed uint64, steps int) {
	s := newSim(t)
	g := &generator{r: rand.New(rand.NewPCG(seed, 42)), tp: newTopology(), removed: map[string]bool{}}
	g.addSink()
	g.addSource()
	g.addSource()
	s.mustLoad(g.tp)

	for step := range steps {
		g.tp = g.tp.clone()
		for range 1 + g.r.IntN(3) {
			g.mutate()
		}
		if err := s.load(g.tp); err != nil {
			t.Fatalf("step %d (seed %d): %v\nlast ops: %v\nconfig:\n%s", step, seed, err, g.lastOps(), g.tp.render(s.run))
		}
		time.Sleep(time.Duration(g.r.IntN(15)) * time.Millisecond)
	}

	if err := s.verify(g.tp, func(by string) bool { return g.removed[by] }); err != nil {
		t.Fatalf("seed %d: %v\nops: %v\nconfig:\n%s", seed, err, opCounts(g.ops), g.tp.render(s.run))
	}
	t.Logf("%d reloads, ops: %v, entries: %s", steps, opCounts(g.ops), s.rec.account())
	for name := range g.tp.sources {
		s.requireEventuallyHealth("lokisim.source."+name, component.HealthTypeHealthy)
	}
}

type generator struct {
	r       *rand.Rand
	tp      topology
	next    int
	removed map[string]bool // Component IDs removed at some point.
	ops     []string
}

func (g *generator) name(prefix string) string {
	g.next++
	return prefix + strconv.Itoa(g.next)
}

func (g *generator) lastOps() []string {
	return g.ops[max(0, len(g.ops)-10):]
}

func (g *generator) mutate() {
	ops := []struct {
		name string
		fn   func() bool
	}{
		{"add_sink", func() bool { g.addSink(); return true }},
		{"remove_sink", g.removeSink},
		{"reroute_source", g.rerouteSource},
		{"reroute_relay", g.rerouteRelay},
		{"restart_source", g.restartSource},
		{"add_relay", g.addRelay},
		{"remove_relay", g.removeRelay},
		{"add_source", func() bool { g.addSource(); return true }},
		{"remove_source", g.removeSource},
		{"rename_source", g.renameSource},
		{"tune_source", g.tuneSource},
		{"remove_source_and_reroute_downstream", g.removeSourceAndRerouteDownstream},
	}
	for {
		op := ops[g.r.IntN(len(ops))]
		if op.fn() {
			g.ops = append(g.ops, op.name)
			return
		}
	}
}

func (g *generator) addSink() string {
	name := g.name("sink")
	spec := sinkSpec{
		startDelay: time.Duration(g.r.IntN(5)) * time.Millisecond,
		delay:      time.Duration(g.r.IntN(50)) * time.Microsecond,
	}
	if g.r.IntN(10) == 0 {
		// A slow downstream that backs everything up for a while.
		spec.startDelay = time.Duration(20+g.r.IntN(80)) * time.Millisecond
		spec.delay = time.Duration(g.r.IntN(2)) * time.Millisecond
	}
	g.tp.sinks[name] = spec
	return name
}

func (g *generator) removeSink() bool {
	if len(g.tp.sinks) < 2 {
		return false
	}
	name := g.pick(slices.Collect(maps.Keys(g.tp.sinks)))
	delete(g.tp.sinks, name)
	g.removed["lokisim.sink."+name] = true
	g.dropTarget("sink." + name)
	return true
}

func (g *generator) addRelay() bool {
	// Relays only send to sinks and to older relays, so there are no cycles.
	targets := g.relayTargets("")
	name := g.name("relay")
	g.tp.relays[name] = relaySpec{to: g.someOf(targets)}
	// Put it in front of a source so it carries traffic.
	src := g.pick(slices.Collect(maps.Keys(g.tp.sources)))
	spec := g.tp.sources[src]
	spec.to = append(spec.to, "relay."+name)
	g.tp.sources[src] = spec
	return true
}

func (g *generator) removeRelay() bool {
	if len(g.tp.relays) == 0 {
		return false
	}
	name := g.pick(slices.Collect(maps.Keys(g.tp.relays)))
	delete(g.tp.relays, name)
	g.removed["lokisim.relay."+name] = true
	g.dropTarget("relay." + name)
	return true
}

func (g *generator) rerouteRelay() bool {
	if len(g.tp.relays) == 0 {
		return false
	}
	name := g.pick(slices.Collect(maps.Keys(g.tp.relays)))
	g.tp.relays[name] = relaySpec{to: g.someOf(g.relayTargets(name))}
	return true
}

// relayTargets returns what relay can send to: sinks and relays older than it.
// An empty relay means a new one.
func (g *generator) relayTargets(relay string) []string {
	targets := g.sinkTargets()
	for name := range g.tp.relays {
		if relay == "" || relayAge(name) < relayAge(relay) {
			targets = append(targets, "relay."+name)
		}
	}
	slices.Sort(targets)
	return targets
}

func relayAge(name string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(name, "relay"))
	return n
}

func (g *generator) addSource() {
	g.tp.sources[g.name("src")] = sourceSpec{
		to:            g.someOf(g.allTargets()),
		resource:      g.name("port"),
		blockingSends: g.r.IntN(2) == 0,
		batchSize:     []int{0, 0, 1, 5}[g.r.IntN(4)],
		interval:      time.Duration(g.r.IntN(3)) * 50 * time.Microsecond,
		drainTimeout:  200 * time.Millisecond,
	}
}

func (g *generator) removeSource() bool {
	if len(g.tp.sources) < 2 {
		return false
	}
	name := g.pick(slices.Collect(maps.Keys(g.tp.sources)))
	delete(g.tp.sources, name)
	g.removed["lokisim.source."+name] = true
	return true
}

// renameSource replaces a source with an identical one under a new name. The
// new one needs the old one's resource.
func (g *generator) renameSource() bool {
	name := g.pick(slices.Collect(maps.Keys(g.tp.sources)))
	g.tp.sources[g.name("src")] = g.tp.sources[name]
	delete(g.tp.sources, name)
	g.removed["lokisim.source."+name] = true
	return true
}

// tuneSource changes a setting the producer applies without a restart.
func (g *generator) tuneSource() bool {
	name := g.pick(slices.Collect(maps.Keys(g.tp.sources)))
	spec := g.tp.sources[name]
	spec.interval = time.Duration(g.r.IntN(3)) * 50 * time.Microsecond
	g.tp.sources[name] = spec
	return true
}

// removeSourceAndRerouteDownstream removes a source while the relay it sends
// to moves to a new sink, so the removed source can't drain until the reload
// is done.
func (g *generator) removeSourceAndRerouteDownstream() bool {
	if len(g.tp.sources) < 2 {
		return false
	}
	for _, name := range slices.Sorted(maps.Keys(g.tp.sources)) {
		for _, t := range g.tp.sources[name].to {
			relay, ok := strings.CutPrefix(t, "relay.")
			if !ok {
				continue
			}
			delete(g.tp.sources, name)
			g.removed["lokisim.source."+name] = true
			g.tp.relays[relay] = relaySpec{to: []string{"sink." + g.addSink()}}
			return true
		}
	}
	return false
}

func (g *generator) rerouteSource() bool {
	name := g.pick(slices.Collect(maps.Keys(g.tp.sources)))
	spec := g.tp.sources[name]
	spec.to = g.someOf(g.allTargets())
	g.tp.sources[name] = spec
	return true
}

func (g *generator) restartSource() bool {
	name := g.pick(slices.Collect(maps.Keys(g.tp.sources)))
	spec := g.tp.sources[name]
	spec.generation++
	spec.startFailures = g.r.IntN(3)
	g.tp.sources[name] = spec
	return true
}

// dropTarget removes target from every forward_to, keeping each non-empty.
func (g *generator) dropTarget(target string) {
	for _, name := range slices.Sorted(maps.Keys(g.tp.relays)) {
		spec := g.tp.relays[name]
		spec.to = slices.DeleteFunc(spec.to, func(t string) bool { return t == target })
		if len(spec.to) == 0 {
			spec.to = []string{g.pick(g.sinkTargets())}
		}
		g.tp.relays[name] = spec
	}
	for _, name := range slices.Sorted(maps.Keys(g.tp.sources)) {
		spec := g.tp.sources[name]
		spec.to = slices.DeleteFunc(spec.to, func(t string) bool { return t == target })
		if len(spec.to) == 0 {
			spec.to = []string{g.pick(g.allTargets())}
		}
		g.tp.sources[name] = spec
	}
}

func (g *generator) sinkTargets() []string {
	var targets []string
	for name := range g.tp.sinks {
		targets = append(targets, "sink."+name)
	}
	slices.Sort(targets)
	return targets
}

func (g *generator) allTargets() []string {
	targets := g.sinkTargets()
	for name := range g.tp.relays {
		targets = append(targets, "relay."+name)
	}
	slices.Sort(targets)
	return targets
}

// someOf returns a random non-empty subset of targets.
func (g *generator) someOf(targets []string) []string {
	var picked []string
	for _, t := range targets {
		if g.r.IntN(3) == 0 {
			picked = append(picked, t)
		}
	}
	if len(picked) == 0 {
		picked = append(picked, g.pick(targets))
	}
	return picked
}

func (g *generator) pick(items []string) string {
	slices.Sort(items)
	return items[g.r.IntN(len(items))]
}

func opCounts(ops []string) map[string]int {
	counts := map[string]int{}
	for _, op := range ops {
		counts[op]++
	}
	return counts
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}
