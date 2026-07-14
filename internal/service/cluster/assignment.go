package cluster

import (
	"sort"

	"github.com/grafana/ckit/shard"
)

// This file implements the pure, deterministic core of series-aware (weighted)
// target distribution. It is intentionally free of any cluster/gossip
// dependencies so that the same inputs always produce the same output on every
// node — which is what guarantees that no target is owned by two nodes at once.
//
// The design in a nutshell:
//   - each target has a weight derived from its observed series count;
//   - targets are handed out one at a time, each to the least-loaded node that
//     the ring allows to own it, accumulating that node's load as we go;
//   - "t-shirt sizing" and hysteresis keep the assignment stable across the
//     small scrape-to-scrape jitter in series counts.

// tShirtSizes are coarse weight buckets, smallest to largest. A target's
// measured series count is rounded UP to the nearest bucket — its "t-shirt
// size". Ordinary scrape-to-scrape jitter (e.g. 1010 vs 1020 series) lands in
// the same bucket, so a target's weight stays put and doesn't trigger a
// reshuffle; only a real jump in magnitude moves it to a new size. The values
// are just a stability knob — coarser buckets mean fewer moves — not anything
// load-bearing, so they're easy to tune.
var tShirtSizes = []uint64{10, 50, 100, 500, 1_000, 5_000, 10_000, 50_000, 100_000}

// tShirtSize rounds series up to the nearest bucket in tShirtSizes.
// Examples: 0->10, 410->500, 1010->5000, 1020->5000, 31000->50000.
func tShirtSize(series uint64) uint64 {
	for _, size := range tShirtSizes {
		if series <= size {
			return size
		}
	}
	return series // larger than the biggest bucket: use the raw count.
}

// SeriesAssignment computes the weighted owner for every target key. It is the
// deterministic core of series-aware distribution, shared by the cluster
// service and end-to-end tests: candidates come from the ring sharder, weights
// are t-shirt-sized, and hysteresis (against prev) avoids churn. nodes is the
// full participant set, used only for the imbalance comparison in hysteresis.
func SeriesAssignment(sharder shard.Sharder, keys []shard.Key, weights map[uint64]uint64, prev map[shard.Key]string, nodes []string) map[shard.Key]string {
	// Probe at most as many ring candidates as there are participants; Lookup
	// errors if it can't return the requested count, which would otherwise drop
	// every target in clusters smaller than seriesProbeOwners.
	probe := seriesProbeOwners
	if len(nodes) < probe {
		probe = len(nodes)
	}
	weightOf := func(k shard.Key) uint64 { return tShirtSize(weights[uint64(k)]) }
	candidates := func(k shard.Key) []string {
		ps, err := sharder.Lookup(k, probe, shard.OpReadWrite)
		if err != nil {
			return nil
		}
		names := make([]string, len(ps))
		for i, p := range ps {
			names[i] = p.Name
		}
		return names
	}
	candidate := assignTargets(keys, weightOf, candidates, prev)
	return chooseAssignment(prev, candidate, weightOf, nodes, seriesImbalanceEpsilon)
}

// assignTargets computes a deterministic owner for every target key. It is the
// classic greedy "least-loaded bin" heuristic: walk the targets in a fixed
// order and hand each one to the least-loaded node the ring allows to hold it.
// Being a pure function, every node that calls it with the same inputs produces
// an identical map — which is what prevents two nodes scraping the same target.
//
//	keys:       all clustered target keys (any order; sorted internally).
//	weightOf:   the (t-shirt-sized) weight for a key; callers should apply
//	            weightFloor so every key is >= 1.
//	candidates: the ranked list of node names the ring permits to own a key
//	            (i.e. cluster.Lookup(key, K)). The chosen owner is always one of
//	            these, so a weight change can only move a target among its K ring
//	            neighbours — preserving consistent-hashing locality.
//	prevOwner:  the previous owner of a key ("" if none), used only to break
//	            ties so equal-load targets don't drift between nodes.
func assignTargets(
	keys []shard.Key,
	weightOf func(shard.Key) uint64,
	candidates func(shard.Key) []string,
	prevOwner map[shard.Key]string,
) map[shard.Key]string {
	sorted := make([]shard.Key, len(keys))
	copy(sorted, keys)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	load := map[string]uint64{}
	assignment := make(map[shard.Key]string, len(sorted))
	for _, k := range sorted {
		cands := candidates(k)
		if len(cands) == 0 {
			continue // not assignable right now (e.g. no eligible peers); skip.
		}
		owner := pickLeastLoaded(cands, load, prevOwner[k])
		assignment[k] = owner
		load[owner] += weightOf(k)
	}
	return assignment
}

// pickLeastLoaded returns the candidate with the smallest accumulated load.
// Ties are broken deterministically: first prefer the previous owner
// (stickiness), then the lexicographically smallest node name.
func pickLeastLoaded(cands []string, load map[string]uint64, prevOwner string) string {
	best := cands[0]
	for _, c := range cands[1:] {
		if preferred(c, best, load, prevOwner) {
			best = c
		}
	}
	return best
}

// preferred reports whether candidate c should win over the current best.
func preferred(c, best string, load map[string]uint64, prevOwner string) bool {
	switch {
	case load[c] != load[best]:
		return load[c] < load[best]
	case c == prevOwner:
		return true
	case best == prevOwner:
		return false
	default:
		return c < best
	}
}

// imbalance returns the peak-to-mean load ratio of an assignment (1.0 == perfect
// balance). nodes is the full participant set so empty nodes count as 0 load.
func imbalance(assignment map[shard.Key]string, weightOf func(shard.Key) uint64, nodes []string) float64 {
	if len(nodes) == 0 {
		return 1
	}
	load := make(map[string]uint64, len(nodes))
	for _, n := range nodes {
		load[n] = 0
	}
	var total uint64
	for k, owner := range assignment {
		w := weightOf(k)
		load[owner] += w
		total += w
	}
	mean := float64(total) / float64(len(nodes))
	if mean == 0 {
		return 1
	}
	var peak uint64
	for _, n := range nodes {
		if load[n] > peak {
			peak = load[n]
		}
	}
	return float64(peak) / mean
}

// chooseAssignment applies hysteresis: it keeps the current assignment unless
// the freshly computed candidate improves peak-to-mean imbalance by more than
// epsilon. This avoids reshuffling targets for marginal (e.g. ~1%) gains.
func chooseAssignment(
	current, candidate map[shard.Key]string,
	weightOf func(shard.Key) uint64,
	nodes []string,
	epsilon float64,
) map[shard.Key]string {
	if len(current) == 0 {
		return candidate
	}
	if imbalance(candidate, weightOf, nodes) < imbalance(current, weightOf, nodes)-epsilon {
		return candidate
	}
	return current
}
