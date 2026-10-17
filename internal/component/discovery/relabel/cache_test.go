package relabel

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	commonlabels "github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/syntax"
)

// fakePublisher records the live debugging messages that were published, and can
// pretend to be inactive so that the message functions are never evaluated.
type fakePublisher struct {
	active   bool
	messages []string
	count    int
}

func (p *fakePublisher) PublishIfActive(data livedebugging.Data) {
	p.count++
	if p.active {
		p.messages = append(p.messages, data.DataFunc())
	}
}

// testComponent builds a Component directly so that tests can inspect the cache.
func testComponent(t *testing.T, reg prometheus.Registerer, pub *fakePublisher) *Component {
	t.Helper()
	return &Component{
		opts: component.Options{
			ID:            "discovery.relabel.test",
			OnStateChange: func(component.Exports) {},
		},
		cache:              newTargetCache(),
		metrics:            newMetrics(reg),
		debugDataPublisher: pub,
	}
}

func mustArgs(t *testing.T, cfg string) Arguments {
	t.Helper()
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	return args
}

// counterValue reads a counter out of a registry by metric name.
func counterValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			switch {
			case m.GetCounter() != nil:
				return m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				return m.GetGauge().GetValue()
			}
		}
	}
	return 0
}

const keepBackendRules = `
targets = []

rule {
	source_labels = ["app"]
	action        = "keep"
	regex         = "backend"
}

rule {
	source_labels = ["instance"]
	target_label  = "name"
}
`

func targetsFromMaps(ms ...map[string]string) []discovery.Target {
	out := make([]discovery.Target, 0, len(ms))
	for _, m := range ms {
		out = append(out, discovery.NewTargetFromMap(m))
	}
	return out
}

func TestCacheReusesUnchangedTargets(t *testing.T) {
	reg := prometheus.NewRegistry()
	pub := &fakePublisher{}
	c := testComponent(t, reg, pub)

	args := mustArgs(t, keepBackendRules)
	args.Targets = targetsFromMaps(
		map[string]string{"app": "backend", "instance": "one"},
		map[string]string{"app": "frontend", "instance": "two"},
		map[string]string{"app": "backend", "instance": "three"},
	)

	require.NoError(t, c.Update(args))
	require.Equal(t, float64(0), counterValue(t, reg, "alloy_discovery_relabel_cache_hits_total"))
	require.Equal(t, float64(3), counterValue(t, reg, "alloy_discovery_relabel_cache_misses_total"))
	// Dropped targets are cached too, so all three are held.
	require.Equal(t, float64(3), counterValue(t, reg, "alloy_discovery_relabel_cache_size"))
	require.Equal(t, 3, c.cache.len())

	// Feeding the very same targets again must be served entirely from the cache.
	require.NoError(t, c.Update(args))
	require.Equal(t, float64(3), counterValue(t, reg, "alloy_discovery_relabel_cache_hits_total"))
	require.Equal(t, float64(3), counterValue(t, reg, "alloy_discovery_relabel_cache_misses_total"))
	require.Equal(t, 3, c.cache.len())
}

func TestCacheHitsForDuplicateTargetsInOneUpdate(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := testComponent(t, reg, &fakePublisher{})

	same := map[string]string{"app": "backend", "instance": "one"}
	args := mustArgs(t, keepBackendRules)
	args.Targets = targetsFromMaps(same, same, same)

	require.NoError(t, c.Update(args))
	require.Equal(t, float64(1), counterValue(t, reg, "alloy_discovery_relabel_cache_misses_total"))
	require.Equal(t, float64(2), counterValue(t, reg, "alloy_discovery_relabel_cache_hits_total"))
	require.Equal(t, 1, c.cache.len(), "identical targets share a cache entry")

	// All three are still emitted.
	var got []discovery.Target
	c.opts.OnStateChange = func(e component.Exports) { got = e.(Exports).Output }
	require.NoError(t, c.Update(args))
	require.Len(t, got, 3)
}

func TestCacheMissesWhenLabelsChange(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := testComponent(t, reg, &fakePublisher{})

	args := mustArgs(t, keepBackendRules)
	args.Targets = targetsFromMaps(map[string]string{"app": "backend", "instance": "one"})
	require.NoError(t, c.Update(args))
	require.Equal(t, float64(1), counterValue(t, reg, "alloy_discovery_relabel_cache_misses_total"))

	// A changed label value must not be served from the cache.
	args.Targets = targetsFromMaps(map[string]string{"app": "backend", "instance": "CHANGED"})
	var got []discovery.Target
	c.opts.OnStateChange = func(e component.Exports) { got = e.(Exports).Output }
	require.NoError(t, c.Update(args))
	require.Equal(t, float64(2), counterValue(t, reg, "alloy_discovery_relabel_cache_misses_total"))
	require.Equal(t, float64(0), counterValue(t, reg, "alloy_discovery_relabel_cache_hits_total"))

	require.Len(t, got, 1)
	name, ok := got[0].Get("name")
	require.True(t, ok)
	require.Equal(t, "CHANGED", name, "relabeling must have been redone")
}

func TestCacheCachesDropDecision(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := testComponent(t, reg, &fakePublisher{})

	args := mustArgs(t, keepBackendRules)
	args.Targets = targetsFromMaps(map[string]string{"app": "frontend", "instance": "one"})

	var got []discovery.Target
	c.opts.OnStateChange = func(e component.Exports) { got = e.(Exports).Output }

	require.NoError(t, c.Update(args))
	require.Empty(t, got, "target should be dropped")
	require.Equal(t, 1, c.cache.len(), "the drop decision must be cached")

	require.NoError(t, c.Update(args))
	require.Empty(t, got, "target must still be dropped when served from the cache")
	require.Equal(t, float64(1), counterValue(t, reg, "alloy_discovery_relabel_cache_hits_total"))
}

func TestCachePreservesInputOrder(t *testing.T) {
	c := testComponent(t, prometheus.NewRegistry(), &fakePublisher{})

	args := mustArgs(t, `
targets = []

rule {
	source_labels = ["instance"]
	target_label  = "name"
}
`)
	args.Targets = targetsFromMaps(
		map[string]string{"instance": "c"},
		map[string]string{"instance": "a"},
		map[string]string{"instance": "b"},
	)

	var got []discovery.Target
	c.opts.OnStateChange = func(e component.Exports) { got = e.(Exports).Output }

	// Run twice: the second pass is served from the cache and must still preserve
	// the order of the input rather than the order entries were cached in.
	for i := 0; i < 2; i++ {
		require.NoError(t, c.Update(args))
		require.Len(t, got, 3)
		var names []string
		for _, target := range got {
			name, ok := target.Get("name")
			require.True(t, ok)
			names = append(names, name)
		}
		require.Equal(t, []string{"c", "a", "b"}, names, "pass %d", i)
	}
}

func TestCacheEvictsRemovedTargets(t *testing.T) {
	c := testComponent(t, prometheus.NewRegistry(), &fakePublisher{})

	args := mustArgs(t, keepBackendRules)
	args.Targets = targetsFromMaps(
		map[string]string{"app": "backend", "instance": "one"},
		map[string]string{"app": "backend", "instance": "two"},
		map[string]string{"app": "backend", "instance": "three"},
	)
	require.NoError(t, c.Update(args))
	require.Equal(t, 3, c.cache.len())

	// Two targets go away.
	args.Targets = targetsFromMaps(map[string]string{"app": "backend", "instance": "two"})
	require.NoError(t, c.Update(args))
	require.Equal(t, 1, c.cache.len(), "entries for targets that went away must be dropped")

	// Everything goes away.
	args.Targets = nil
	require.NoError(t, c.Update(args))
	require.Equal(t, 0, c.cache.len())
}

func TestCacheClearedWhenRulesChange(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := testComponent(t, reg, &fakePublisher{})

	targets := targetsFromMaps(map[string]string{"app": "backend", "instance": "one"})

	args := mustArgs(t, keepBackendRules)
	args.Targets = targets
	require.NoError(t, c.Update(args))
	require.Equal(t, 1, c.cache.len())

	var got []discovery.Target
	c.opts.OnStateChange = func(e component.Exports) { got = e.(Exports).Output }

	// Same targets, different rules: the cached result must not be reused.
	changed := mustArgs(t, `
targets = []

rule {
	source_labels = ["app"]
	action        = "keep"
	regex         = "backend"
}

rule {
	source_labels = ["instance"]
	target_label  = "renamed"
}
`)
	changed.Targets = targets
	require.NoError(t, c.Update(changed))

	require.Equal(t, float64(0), counterValue(t, reg, "alloy_discovery_relabel_cache_hits_total"))
	require.Len(t, got, 1)
	_, ok := got[0].Get("name")
	require.False(t, ok, "result from the old rules must not leak through the cache")
	renamed, ok := got[0].Get("renamed")
	require.True(t, ok)
	require.Equal(t, "one", renamed)
}

// TestRuleSnapshotDetectsEveryField guards against a rule field being added
// without being included in the snapshot, which would let a config change go
// unnoticed and serve stale relabeling results.
func TestRuleSnapshotDetectsEveryField(t *testing.T) {
	base := alloy_relabel.DefaultRelabelConfig
	base.SourceLabels = []string{"a"}
	base.TargetLabel = "t"

	mutations := map[string]func(c *alloy_relabel.Config){
		"source_labels": func(c *alloy_relabel.Config) { c.SourceLabels = []string{"b"} },
		"separator":     func(c *alloy_relabel.Config) { c.Separator = "|" },
		"regex":         func(c *alloy_relabel.Config) { require.NoError(t, c.Regex.UnmarshalText([]byte("something"))) },
		"modulus":       func(c *alloy_relabel.Config) { c.Modulus = 7 },
		"target_label":  func(c *alloy_relabel.Config) { c.TargetLabel = "other" },
		"replacement":   func(c *alloy_relabel.Config) { c.Replacement = "other" },
		"action":        func(c *alloy_relabel.Config) { c.Action = alloy_relabel.Drop },
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			before := base
			after := base
			mutate(&after)
			require.True(t,
				rulesChanged(snapshotRules([]*alloy_relabel.Config{&before}), snapshotRules([]*alloy_relabel.Config{&after})),
				"changing %s must invalidate the cache", name)
		})
	}

	t.Run("identical", func(t *testing.T) {
		a, b := base, base
		require.False(t, rulesChanged(
			snapshotRules([]*alloy_relabel.Config{&a}),
			snapshotRules([]*alloy_relabel.Config{&b})))
	})

	t.Run("rule count", func(t *testing.T) {
		a := base
		require.True(t, rulesChanged(
			snapshotRules([]*alloy_relabel.Config{&a}),
			snapshotRules([]*alloy_relabel.Config{&a, &a})))
	})

	t.Run("source label boundaries", func(t *testing.T) {
		// ["a", "b"] must not snapshot the same as ["ab"].
		x, y := base, base
		x.SourceLabels = []string{"a", "b"}
		y.SourceLabels = []string{"ab"}
		require.True(t, rulesChanged(
			snapshotRules([]*alloy_relabel.Config{&x}),
			snapshotRules([]*alloy_relabel.Config{&y})))
	})
}

func TestNoRulesBypassesCache(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := testComponent(t, reg, &fakePublisher{})

	targets := targetsFromMaps(
		map[string]string{"app": "backend", "instance": "one"},
		map[string]string{"app": "frontend", "instance": "two"},
	)

	var got []discovery.Target
	c.opts.OnStateChange = func(e component.Exports) { got = e.(Exports).Output }

	args := Arguments{Targets: targets}
	require.NoError(t, c.Update(args))

	require.Len(t, got, 2, "with no rules every target passes through")
	require.Equal(t, 0, c.cache.len(), "nothing should be cached when there are no rules")
	require.Equal(t, float64(0), counterValue(t, reg, "alloy_discovery_relabel_cache_size"))
	for i := range targets {
		require.True(t, targets[i].EqualsTarget(&got[i]))
	}

	// The exported slice must not alias the caller's argument slice.
	require.NotSame(t, &args.Targets[0], &got[0])
}

func TestRemovingRulesReleasesCache(t *testing.T) {
	c := testComponent(t, prometheus.NewRegistry(), &fakePublisher{})

	targets := targetsFromMaps(map[string]string{"app": "backend", "instance": "one"})

	args := mustArgs(t, keepBackendRules)
	args.Targets = targets
	require.NoError(t, c.Update(args))
	require.Equal(t, 1, c.cache.len())

	require.NoError(t, c.Update(Arguments{Targets: targets}))
	require.Equal(t, 0, c.cache.len(), "dropping the rules must release the cache")
	require.Nil(t, c.rules)
}

func TestLiveDebuggingReportsCachedResults(t *testing.T) {
	pub := &fakePublisher{active: true}
	c := testComponent(t, prometheus.NewRegistry(), pub)

	args := mustArgs(t, keepBackendRules)
	args.Targets = targetsFromMaps(
		map[string]string{"app": "backend", "instance": "one"},
		map[string]string{"app": "frontend", "instance": "two"},
	)

	require.NoError(t, c.Update(args))
	first := append([]string(nil), pub.messages...)
	require.Len(t, first, 2)
	require.Contains(t, first[0], `"name"="one"`, "kept target should show its relabeled form")
	require.Contains(t, first[1], "=> {}", "dropped target should show an empty result")

	// The second pass is served from the cache and must publish the same thing.
	pub.messages = nil
	require.NoError(t, c.Update(args))
	require.Equal(t, first, pub.messages, "cached results must produce the same debug output")
}

func TestCacheRebuildsAfterCardinalityCollapse(t *testing.T) {
	require.False(t, shouldRebuild(rebuildMinSize-1, 0), "small caches are not worth rebuilding")
	require.False(t, shouldRebuild(rebuildMinSize, rebuildMinSize), "no collapse, no rebuild")
	require.False(t, shouldRebuild(rebuildMinSize, rebuildMinSize/2+1), "less than half is not a collapse")
	require.True(t, shouldRebuild(rebuildMinSize, rebuildMinSize/2), "half or less is a collapse")
	require.True(t, shouldRebuild(rebuildMinSize, 0))

	c := testComponent(t, prometheus.NewRegistry(), &fakePublisher{})
	args := mustArgs(t, keepBackendRules)

	// Grow past the rebuild threshold.
	big := make([]map[string]string, 0, rebuildMinSize+10)
	for i := 0; i < rebuildMinSize+10; i++ {
		big = append(big, map[string]string{"app": "backend", "instance": fmt.Sprintf("i%d", i)})
	}
	args.Targets = targetsFromMaps(big...)
	require.NoError(t, c.Update(args))
	require.Equal(t, rebuildMinSize+10, c.cache.len())

	// Maps are not comparable, so identify the backing map by its address.
	mapIdentity := func(m map[discovery.TargetCacheKey]*cacheEntry) uintptr {
		return reflect.ValueOf(m).Pointer()
	}
	before := mapIdentity(c.cache.entries)

	// Collapse to a single target.
	args.Targets = targetsFromMaps(big[0])
	require.NoError(t, c.Update(args))
	require.Equal(t, 1, c.cache.len())
	require.NotEqual(t, before, mapIdentity(c.cache.entries), "the backing map should have been rebuilt")
	require.Equal(t, 1, c.cache.peak, "peak resets to the live size after a rebuild")

	// The surviving entry must still be usable.
	require.NoError(t, c.Update(args))
	require.Equal(t, 1, c.cache.len())
}

// TestCacheKeyDistinguishesGroupSplit documents that the cache key is based on
// the packed representation, so two targets with the same labels but a different
// group/own split are different keys. That is a safe miss, never a wrong hit.
func TestCacheKeyDistinguishesGroupSplit(t *testing.T) {
	labels := map[string]string{"job": "j", "instance": "i"}

	flat := discovery.NewTargetFromMap(labels)
	split := discovery.NewTargetFromSpecificAndBaseLabelSet(
		commonlabels.LabelSet{"instance": "i"},
		commonlabels.LabelSet{"job": "j"},
	)

	require.True(t, flat.EqualsTarget(&split), "the targets have the same labels")
	require.NotEqual(t, flat.CacheKey(), split.CacheKey(), "different splits are different keys")

	// Same split, same labels: keys must match.
	again := discovery.NewTargetFromMap(labels)
	require.Equal(t, flat.CacheKey(), again.CacheKey())
}

// TestCacheDoesNotRebuildOnFullChurn guards the peak accounting. While an update
// is in flight the map holds both the previous and the current generation, so a
// component whose targets all change every update momentarily has twice as many
// entries as are live. Treating that as the high-water mark made every single
// update look like a collapse and rebuild the map, which cost ~13% more memory
// for no benefit.
func TestCacheDoesNotRebuildOnFullChurn(t *testing.T) {
	c := testComponent(t, prometheus.NewRegistry(), &fakePublisher{})
	args := mustArgs(t, keepBackendRules)

	build := func(generation int) []discovery.Target {
		ms := make([]map[string]string, 0, rebuildMinSize)
		for i := 0; i < rebuildMinSize; i++ {
			ms = append(ms, map[string]string{
				"app":      "backend",
				"instance": fmt.Sprintf("i%d-gen%d", i, generation),
			})
		}
		return targetsFromMaps(ms...)
	}

	mapIdentity := func(m map[discovery.TargetCacheKey]*cacheEntry) uintptr {
		return reflect.ValueOf(m).Pointer()
	}

	args.Targets = build(0)
	require.NoError(t, c.Update(args))
	require.Equal(t, rebuildMinSize, c.cache.len())
	require.Equal(t, rebuildMinSize, c.cache.peak)

	// Replace every target, repeatedly. The live size never changes, so the map
	// must not be rebuilt.
	identity := mapIdentity(c.cache.entries)
	for generation := 1; generation <= 3; generation++ {
		args.Targets = build(generation)
		require.NoError(t, c.Update(args))
		require.Equal(t, rebuildMinSize, c.cache.len(), "generation %d", generation)
		require.Equal(t, rebuildMinSize, c.cache.peak, "generation %d: peak must track live size", generation)
		require.Equal(t, identity, mapIdentity(c.cache.entries),
			"generation %d: full churn must not rebuild the map", generation)
	}

	// Only the current generation is retained.
	require.Equal(t, rebuildMinSize, c.cache.len())
}

// TestCacheHitsWhenGroupIsRebuilt is the regression test for the cache keying on
// group contents rather than on the group pointer.
//
// Most service discovery mechanisms are built on refresh: they rebuild every
// target group on every refresh interval, so a group whose contents did not change
// still arrives as a fresh allocation. toAlloyTargets then packs a new
// *groupLabels for it, and a cache keyed on that pointer missed for every target
// of every refresh-based mechanism, which is nearly all of them. Verified against
// a running discovery.file: changing one address in one of three groups produced
// 0 hits and 12 misses.
func TestCacheHitsWhenGroupIsRebuilt(t *testing.T) {
	group := commonlabels.LabelSet{"job": "alpha", "env": "prod"}

	// Two independently packed groups with identical contents, as consecutive
	// refreshes of the same service discovery mechanism produce.
	build := func(addr string) discovery.Target {
		return discovery.NewTargetFromSpecificAndBaseLabelSet(
			commonlabels.LabelSet{"__address__": commonlabels.LabelValue(addr), "app": "backend"},
			commonlabels.LabelSet{"job": group["job"], "env": group["env"]},
		)
	}

	first := build("10.0.0.1:80")
	rebuilt := build("10.0.0.1:80")

	require.True(t, first.EqualsTarget(&rebuilt), "the two targets have identical labels")
	require.Equal(t, first.CacheKey(), rebuilt.CacheKey(),
		"a rebuilt group with identical contents must produce the same cache key")

	reg := prometheus.NewRegistry()
	c := testComponent(t, reg, &fakePublisher{})
	args := mustArgs(t, keepBackendRules)

	args.Targets = []discovery.Target{first}
	require.NoError(t, c.Update(args))
	require.Equal(t, float64(1), counterValue(t, reg, "alloy_discovery_relabel_cache_misses_total"))
	require.Equal(t, float64(0), counterValue(t, reg, "alloy_discovery_relabel_cache_hits_total"))

	// The same target arriving from a rebuilt group must be served from the cache.
	args.Targets = []discovery.Target{rebuilt}
	require.NoError(t, c.Update(args))
	require.Equal(t, float64(1), counterValue(t, reg, "alloy_discovery_relabel_cache_misses_total"),
		"a rebuilt group must not cause a miss")
	require.Equal(t, float64(1), counterValue(t, reg, "alloy_discovery_relabel_cache_hits_total"))
	require.Equal(t, 1, c.cache.len(), "the rebuilt target must reuse the existing entry, not add one")

	// A partially changed group: only the target that actually changed may miss.
	changed := build("10.0.0.99:80")
	args.Targets = []discovery.Target{rebuilt, changed}
	require.NoError(t, c.Update(args))
	require.Equal(t, float64(2), counterValue(t, reg, "alloy_discovery_relabel_cache_hits_total"),
		"the unchanged target must still hit")
	require.Equal(t, float64(2), counterValue(t, reg, "alloy_discovery_relabel_cache_misses_total"),
		"only the changed target may miss")
}

// TestCacheKeyIgnoresGroupIdentityNotContents checks the key distinguishes group
// contents, so that a target cannot be served a result computed for different
// group labels.
func TestCacheKeyIgnoresGroupIdentityNotContents(t *testing.T) {
	own := commonlabels.LabelSet{"__address__": "10.0.0.1:80"}

	a := discovery.NewTargetFromSpecificAndBaseLabelSet(own, commonlabels.LabelSet{"job": "alpha"})
	sameContents := discovery.NewTargetFromSpecificAndBaseLabelSet(own, commonlabels.LabelSet{"job": "alpha"})
	differentContents := discovery.NewTargetFromSpecificAndBaseLabelSet(own, commonlabels.LabelSet{"job": "beta"})

	require.Equal(t, a.CacheKey(), sameContents.CacheKey())
	require.NotEqual(t, a.CacheKey(), differentContents.CacheKey(),
		"different group labels must not share a cache entry")
}
