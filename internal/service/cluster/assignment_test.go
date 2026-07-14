package cluster

import (
	"testing"

	"github.com/grafana/ckit/shard"
	"github.com/stretchr/testify/require"
)

// allNodesCandidates returns a candidates func that lets any of nodes own any
// key, so tests exercise the load-balancing logic in isolation from the ring.
func allNodesCandidates(nodes ...string) func(shard.Key) []string {
	return func(shard.Key) []string { return nodes }
}

// weightMap turns an explicit per-key weight table into a weightOf func.
// Unknown keys (and anything below 1) default to weight 1 so the greedy logic
// is exercised with raw weights, independent of t-shirt bucketing.
func weightMap(weights map[shard.Key]uint64) func(shard.Key) uint64 {
	return func(k shard.Key) uint64 {
		if w, ok := weights[k]; ok && w > 1 {
			return w
		}
		return 1
	}
}

func TestTShirtSize(t *testing.T) {
	cases := []struct {
		series uint64
		want   uint64
	}{
		{0, 10},
		{7, 10},
		{410, 500},       // a node_exporter-ish target
		{1010, 5000},     // jitter ...
		{1020, 5000},     // ... maps to the same bucket, so no move
		{31000, 50000},   // the apiserver whale
		{250000, 250000}, // bigger than the largest bucket -> raw count
	}
	for _, tc := range cases {
		require.Equalf(t, tc.want, tShirtSize(tc.series), "tShirtSize(%d)", tc.series)
	}
}

// Two independent computations with identical inputs must yield byte-for-byte
// identical ownership. This is the property that prevents a target being scraped
// by two nodes at once.
func TestAssignTargets_Deterministic(t *testing.T) {
	nodes := []string{"alloy-2", "alloy-0", "alloy-1"} // deliberately unsorted
	keys := make([]shard.Key, 200)
	weights := map[shard.Key]uint64{}
	for i := range keys {
		keys[i] = shard.Key(i * 2654435761) // spread keys around
		weights[keys[i]] = uint64((i%5)*100 + 1)
	}
	cands := allNodesCandidates(nodes...)
	wOf := weightMap(weights)

	a := assignTargets(keys, wOf, cands, nil)
	b := assignTargets(keys, wOf, cands, nil)
	require.Equal(t, a, b, "same inputs must produce the same assignment")

	// Every key is assigned to exactly one node, and only to an eligible candidate.
	require.Len(t, a, len(keys))
	for k, owner := range a {
		require.Contains(t, nodes, owner, "owner of %d must be a candidate", k)
	}
}

// A single indivisible "whale" cannot be split, so peak/mean is bounded below by
// whale/mean no matter what. The property weighting must guarantee is that the
// whale's owner receives NO extra load and every other node shares the light
// targets evenly — i.e. nothing piles on top of the whale.
func TestAssignTargets_WhaleOffset(t *testing.T) {
	nodes := []string{"alloy-0", "alloy-1", "alloy-2", "alloy-3"}

	const whale = shard.Key(1)
	const whaleWeight = uint64(16384) // ~ the apiserver
	weights := map[shard.Key]uint64{whale: whaleWeight}
	keys := []shard.Key{whale}
	for i := 2; i <= 400; i++ { // 399 light targets, weight 1 each
		keys = append(keys, shard.Key(i))
	}
	wOf := weightMap(weights)

	assignment := assignTargets(keys, wOf, allNodesCandidates(nodes...), nil)

	// Per-node totals and light-target counts.
	load := map[string]uint64{}
	lightCount := map[string]int{}
	for k, owner := range assignment {
		load[owner] += wOf(k)
		if k != whale {
			lightCount[owner]++
		}
	}

	whaleOwner := assignment[whale]
	require.Equal(t, whaleWeight, load[whaleOwner], "whale owner must carry only the whale, no extra targets piled on")
	require.Zero(t, lightCount[whaleOwner], "no light targets should land on the already-full whale owner")

	// The other three nodes split the 399 light targets ~evenly (133 each).
	min, max := 1<<30, 0
	for _, n := range nodes {
		if n == whaleOwner {
			continue
		}
		if lightCount[n] < min {
			min = lightCount[n]
		}
		if lightCount[n] > max {
			max = lightCount[n]
		}
	}
	require.LessOrEqual(t, max-min, 1, "light targets must be balanced across the non-whale nodes (min=%d max=%d)", min, max)
}

// With multiple splittable heavy targets, weighting should flatten peak-to-mean
// load close to balanced, whereas a weight-blind (count-based) assignment that
// happens to clump the heavies is badly imbalanced.
func TestAssignTargets_BalancesHeaviesBetterThanCount(t *testing.T) {
	nodes := []string{"alloy-0", "alloy-1", "alloy-2", "alloy-3"}

	weights := map[shard.Key]uint64{}
	var keys []shard.Key
	for i := 1; i <= 8; i++ { // 8 heavy targets, weight 1000
		k := shard.Key(i)
		keys = append(keys, k)
		weights[k] = 1000
	}
	for i := 9; i <= 208; i++ { // 200 light targets, weight 1
		keys = append(keys, shard.Key(i))
	}
	wOf := weightMap(weights)

	weighted := assignTargets(keys, wOf, allNodesCandidates(nodes...), nil)
	require.Less(t, imbalance(weighted, wOf, nodes), 1.15, "weighting should spread the 8 heavies ~2 per node")

	// A weight-blind assignment that clumps the heavies onto one node (what a
	// count-based ring can do) is badly imbalanced — this is the motivation.
	countBlind := map[shard.Key]string{}
	for _, k := range keys {
		if w := weights[k]; w == 1000 {
			countBlind[k] = "alloy-0" // all heavies clump here
		} else {
			countBlind[k] = nodes[int(k)%len(nodes)]
		}
	}
	require.Greater(t, imbalance(countBlind, wOf, nodes), 2.5, "weight-blind clumping is badly imbalanced")
}

// The chosen owner must always be one of the ring-provided candidates, even when
// a less-loaded node exists outside the candidate set.
func TestAssignTargets_RespectsCandidates(t *testing.T) {
	// key 1 may only live on alloy-3; everything else may live on alloy-0/1.
	candidates := func(k shard.Key) []string {
		if k == 1 {
			return []string{"alloy-3"}
		}
		return []string{"alloy-0", "alloy-1"}
	}
	keys := []shard.Key{1, 2, 3, 4, 5}
	assignment := assignTargets(keys, weightMap(nil), candidates, nil)

	require.Equal(t, "alloy-3", assignment[1])
	for _, k := range []shard.Key{2, 3, 4, 5} {
		require.Contains(t, []string{"alloy-0", "alloy-1"}, assignment[k])
	}
}

// On an exact load tie, the current owner keeps the target (stickiness) so equal
// targets don't drift between nodes on every recompute.
func TestPickLeastLoaded_StickyTieBreak(t *testing.T) {
	load := map[string]uint64{"alloy-0": 5, "alloy-1": 5}
	cands := []string{"alloy-0", "alloy-1"}

	require.Equal(t, "alloy-1", pickLeastLoaded(cands, load, "alloy-1"), "tie should stick to previous owner")
	require.Equal(t, "alloy-0", pickLeastLoaded(cands, load, "alloy-0"), "tie should stick to previous owner")
	// No previous owner -> deterministic lexicographic choice.
	require.Equal(t, "alloy-0", pickLeastLoaded(cands, load, ""))
}

func TestPickLeastLoaded_PrefersLighter(t *testing.T) {
	load := map[string]uint64{"alloy-0": 10, "alloy-1": 3}
	// Even though alloy-0 is the previous owner, the much lighter alloy-1 wins.
	require.Equal(t, "alloy-1", pickLeastLoaded([]string{"alloy-0", "alloy-1"}, load, "alloy-0"))
}

// Hysteresis: a marginal improvement is rejected (keep current); a large
// improvement is adopted.
func TestChooseAssignment_Hysteresis(t *testing.T) {
	nodes := []string{"alloy-0", "alloy-1"}
	wOf := weightMap(nil) // all weight 1

	// current: lopsided 3 vs 1.
	current := map[shard.Key]string{1: "alloy-0", 2: "alloy-0", 3: "alloy-0", 4: "alloy-1"}
	// big improvement: perfectly balanced 2 vs 2.
	balanced := map[shard.Key]string{1: "alloy-0", 2: "alloy-0", 3: "alloy-1", 4: "alloy-1"}

	got := chooseAssignment(current, balanced, wOf, nodes, 0.10)
	require.Equal(t, balanced, got, "a large imbalance improvement should be adopted")

	// marginal: moving one target when already balanced barely changes imbalance.
	alreadyBalanced := map[shard.Key]string{1: "alloy-0", 2: "alloy-0", 3: "alloy-1", 4: "alloy-1"}
	marginal := map[shard.Key]string{1: "alloy-0", 2: "alloy-1", 3: "alloy-1", 4: "alloy-1"} // 1 vs 3, worse
	got = chooseAssignment(alreadyBalanced, marginal, wOf, nodes, 0.10)
	require.Equal(t, alreadyBalanced, got, "a non-improving candidate must be rejected")
}

func TestImbalance(t *testing.T) {
	nodes := []string{"alloy-0", "alloy-1"}
	wOf := weightMap(nil)
	require.InDelta(t, 1.0, imbalance(map[shard.Key]string{1: "alloy-0", 2: "alloy-1"}, wOf, nodes), 1e-9)
	require.InDelta(t, 2.0, imbalance(map[shard.Key]string{1: "alloy-0", 2: "alloy-0"}, wOf, nodes), 1e-9)
}
