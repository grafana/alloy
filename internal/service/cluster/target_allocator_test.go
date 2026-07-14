package cluster

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/stretchr/testify/require"
)

func ringFor(nodes ...string) shard.Sharder {
	s := shard.Ring(512)
	peers := make([]peer.Peer, len(nodes))
	for i, n := range nodes {
		peers[i] = peer.Peer{Name: n, State: peer.StateParticipant}
	}
	s.SetPeers(peers)
	return s
}

func entries(n int) []TargetEntry {
	out := make([]TargetEntry, n)
	for i := range out {
		out[i] = TargetEntry{
			Key:    uint64(i+1) * 2654435761,
			Labels: map[string]string{"instance": fmt.Sprintf("t-%d", i)},
		}
	}
	return out
}

// The leader's per-node slices must partition the full target set exactly: every
// target assigned to exactly one node (no double-scraping, no gaps).
func TestTargetAllocator_PartitionsExactly(t *testing.T) {
	nodes := []string{"alloy-0", "alloy-1", "alloy-2", "alloy-3"}
	sharder := ringFor(nodes...)

	const comp = "discovery.kubernetes.pods"
	all := entries(200)

	a := newTargetAllocator()
	a.setTargets(comp, all)
	a.computeAll(sharder, nodes)

	seen := map[uint64]int{}
	for _, node := range nodes {
		for _, e := range a.slice(comp, node) {
			seen[e.Key]++
		}
	}
	require.Len(t, seen, len(all), "every target must be assigned to exactly one node")
	for k, n := range seen {
		require.Equalf(t, 1, n, "target %d assigned to %d nodes", k, n)
	}
}

// Recomputing with identical inputs yields identical slices (the single-authority
// determinism the per-node path lacked).
func TestTargetAllocator_Deterministic(t *testing.T) {
	nodes := []string{"alloy-0", "alloy-1", "alloy-2"}
	sharder := ringFor(nodes...)
	const comp = "c"
	all := entries(150)

	a := newTargetAllocator()
	a.setTargets(comp, all)
	a.computeAll(sharder, nodes)
	first := a.slice(comp, "alloy-1")

	a.computeAll(sharder, nodes)
	second := a.slice(comp, "alloy-1")

	require.Equal(t, first, second)
}

// Weighted targets are offset: a heavy target's owner gets fewer light ones.
func TestTargetAllocator_Weighted(t *testing.T) {
	nodes := []string{"alloy-0", "alloy-1", "alloy-2", "alloy-3"}
	sharder := ringFor(nodes...)
	const comp = "c"

	all := entries(400)
	a := newTargetAllocator()
	a.setTargets(comp, all)
	// Make the first target a whale; the rest light.
	whale := all[0].Key
	w := map[uint64]uint64{whale: 50000}
	for _, e := range all[1:] {
		w[e.Key] = 100
	}
	a.reportWeights(w)
	a.computeAll(sharder, nodes)

	// Find the whale's owner and assert it carries few targets.
	var whaleOwner string
	counts := map[string]int{}
	for _, node := range nodes {
		for _, e := range a.slice(comp, node) {
			counts[node]++
			if e.Key == whale {
				whaleOwner = node
			}
		}
	}
	require.NotEmpty(t, whaleOwner)
	maxc := 0
	for _, c := range counts {
		if c > maxc {
			maxc = c
		}
	}
	require.Less(t, counts[whaleOwner], maxc, "whale owner should hold fewer targets than the busiest node")
}

// Mirrors the live convergence sequence: the leader first computes a count-based
// assignment (no weights reported yet), then weights arrive and a later recompute
// must SWITCH to the weighted assignment despite hysteresis. This is the path the
// deployment actually takes; a single weighted compute (TestTargetAllocator_Weighted)
// doesn't exercise the count->weighted transition.
func TestTargetAllocator_WeightedAfterCountBased(t *testing.T) {
	nodes := []string{"alloy-0", "alloy-1", "alloy-2", "alloy-3"}
	sharder := ringFor(nodes...)
	const comp = "c"

	all := entries(400)
	whale := all[0].Key

	a := newTargetAllocator()
	a.setTargets(comp, all)

	// 1. First compute with NO weights -> count-based assignment.
	a.computeAll(sharder, nodes)
	countBased := map[string]int{}
	var whaleOwner string
	for _, node := range nodes {
		for _, e := range a.slice(comp, node) {
			countBased[node]++
			if e.Key == whale {
				whaleOwner = node
			}
		}
	}
	require.NotEmpty(t, whaleOwner)

	// 2. Weights arrive: the whale is huge, the rest light.
	w := map[uint64]uint64{whale: 50000}
	for _, e := range all[1:] {
		w[e.Key] = 100
	}
	a.reportWeights(w)

	// 3. Recompute -> must switch to weighted: the whale owner sheds most targets.
	a.computeAll(sharder, nodes)
	weighted := map[string]int{}
	for _, node := range nodes {
		weighted[node] = len(a.slice(comp, node))
	}

	t.Logf("whale owner %s: count-based=%d targets, weighted=%d targets", whaleOwner, countBased[whaleOwner], weighted[whaleOwner])
	require.Less(t, weighted[whaleOwner], countBased[whaleOwner],
		"after weights arrive, the whale owner must shed targets (count->weighted switch rejected by hysteresis?)")
}

// End-to-end of the HTTP path: the leader serves each node's slice; a follower
// pulls its slice over a real HTTP server. Leader-local + follower-pull together
// cover the full set with no overlap.
func TestAllocator_HTTPRoundTrip(t *testing.T) {
	nodes := []string{"leader", "follower"}
	const comp = "discovery.kubernetes.pods"
	const path = "/api/v1/ckit/transport/targets"

	alloc := newTargetAllocator()
	alloc.setTargets(comp, entries(20))
	alloc.computeAll(ringFor(nodes...), nodes)

	// Leader: a Service whose alloyCluster reports itself as the leader.
	leaderAC := &alloyCluster{
		sharder:      &mockSharder{peers: []peer.Peer{{Name: "leader", Self: true, State: peer.StateParticipant}}},
		opts:         Options{NodeName: "leader"},
		allocator:    alloc,
		localWeights: map[uint64]uint64{},
	}
	leaderSvc := &Service{alloyCluster: leaderAC, allocator: alloc}

	mux := http.NewServeMux()
	mux.HandleFunc(path, leaderSvc.serveAllocatorTargets)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Follower: points its allocator client at the leader's server address.
	followerAC := &alloyCluster{
		sharder: &mockSharder{peers: []peer.Peer{
			{Name: "leader", Addr: strings.TrimPrefix(srv.URL, "http://"), Self: false, State: peer.StateParticipant},
		}},
		opts:          Options{NodeName: "follower"},
		allocator:     newTargetAllocator(),
		httpClient:    srv.Client(),
		allocatorPath: path,
		localWeights:  map[uint64]uint64{},
	}

	leaderGot, err := leaderAC.AssignedTargets(comp)
	require.NoError(t, err)
	require.Equal(t, alloc.slice(comp, "leader"), leaderGot)

	followerGot, err := followerAC.AssignedTargets(comp)
	require.NoError(t, err)
	require.Equal(t, alloc.slice(comp, "follower"), followerGot)

	// Union of both nodes' slices = the full set, each target exactly once.
	seen := map[uint64]int{}
	for _, e := range append(leaderGot, followerGot...) {
		seen[e.Key]++
	}
	require.Len(t, seen, 20, "leader + follower slices must cover the full set")
	for k, n := range seen {
		require.Equalf(t, 1, n, "target %d served to both nodes", k)
	}
}

// A non-leader rejects allocator requests so a follower with a stale view retries.
func TestAllocator_ServeRejectsNonLeader(t *testing.T) {
	ac := &alloyCluster{
		sharder: &mockSharder{peers: []peer.Peer{{Name: "other", Self: false, State: peer.StateParticipant}}},
		opts:    Options{NodeName: "self"},
	}
	svc := &Service{alloyCluster: ac, allocator: newTargetAllocator()}

	srv := httptest.NewServer(http.HandlerFunc(svc.serveAllocatorTargets))
	defer srv.Close()

	resp, err := srv.Client().Post(srv.URL, "application/json", strings.NewReader(`{"component":"c","node":"n"}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusMisdirectedRequest, resp.StatusCode)
}

func TestTargetAllocator_DropNode(t *testing.T) {
	nodes := []string{"alloy-0", "alloy-1"}
	sharder := ringFor(nodes...)
	const comp = "c"
	a := newTargetAllocator()
	a.setTargets(comp, entries(50))
	a.computeAll(sharder, nodes)

	a.dropNode("alloy-1")
	// alloy-1 no longer owns anything after being dropped.
	require.Empty(t, a.slice(comp, "alloy-1"))
}

// ReportWeights merges across calls (a node runs several scrape components, each
// measuring disjoint target keys) — a later call must not wipe earlier keys.
func TestAlloyCluster_ReportWeightsMerge(t *testing.T) {
	c := &alloyCluster{localWeights: map[uint64]uint64{}}

	c.ReportWeights(map[uint64]uint64{1: 100, 2: 200}) // scrape component A
	c.ReportWeights(map[uint64]uint64{3: 300})         // scrape component B
	c.ReportWeights(map[uint64]uint64{2: 250})         // A re-reports, updates key 2

	got := c.localWeightsSnapshot()
	require.Equal(t, map[uint64]uint64{1: 100, 2: 250, 3: 300}, got)

	// Snapshot is a copy: mutating it doesn't affect the node's view.
	got[1] = 999
	require.Equal(t, uint64(100), c.localWeightsSnapshot()[1])
}
