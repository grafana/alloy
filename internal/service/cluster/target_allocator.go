package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
)

// participantNames returns the names of all peers in the participant state.
func participantNames(peers []peer.Peer) []string {
	var names []string
	for _, p := range peers {
		if p.State == peer.StateParticipant {
			names = append(names, p.Name)
		}
	}
	return names
}

// allocatorLeaderKey is the fixed ring key whose owner is the target-allocator
// leader. Every node derives the same owner from the same ring.
var allocatorLeaderKey = shard.StringKey("__alloy_target_allocator__")

// allocatorRequest is the body a follower POSTs to the leader to fetch its
// assigned slice for a component. It carries the follower's measured series
// weights so the leader can weight the assignment (replacing weight gossip).
type allocatorRequest struct {
	Component string            `json:"component"`
	Node      string            `json:"node"`
	Weights   map[uint64]uint64 `json:"weights,omitempty"`
}

// TargetEntry is a neutral, JSON-serializable representation of a discovered
// target, used so the cluster package needn't import the discovery package
// (which imports cluster). Key is the target's clustering key
// (discovery.Target.NonMetaLabelsHash); Labels is its full label set.
type TargetEntry struct {
	Key    uint64            `json:"key"`
	Labels map[string]string `json:"labels"`
}

// targetAllocator is the leader-side "target allocator": it holds the full set
// of discovered targets per discovery component (registered by the leader's
// discovery component), the series weights reported by the cluster, and the
// resulting per-node assignment computed with the same SeriesAssignment used by
// the legacy scrape-side path. Each node then takes only its slice — so a target
// is owned by exactly one node by construction (no per-node divergence).
//
// It is computed only on the leader; followers fetch their slice over HTTP.
type targetAllocator struct {
	mut sync.Mutex

	// full[componentID][key] = the discovered target. Set by the leader.
	full map[string]map[uint64]TargetEntry
	// weights[key] = latest reported series count for the target, GLOBAL across
	// all components. Target keys are globally unique, and the scrape component
	// that measures series doesn't know which discovery component produced a
	// target, so weights are keyed only by target — every component's assignment
	// looks up its keys from this one map.
	weights map[uint64]uint64
	// assignment[componentID][key] = owning node name (last computed).
	assignment map[string]map[uint64]string
}

func newTargetAllocator() *targetAllocator {
	return &targetAllocator{
		full:       make(map[string]map[uint64]TargetEntry),
		weights:    make(map[uint64]uint64),
		assignment: make(map[string]map[uint64]string),
	}
}

// setTargets replaces the full discovered target set for a component (leader
// only, from its discovery component).
func (a *targetAllocator) setTargets(componentID string, targets []TargetEntry) {
	byKey := make(map[uint64]TargetEntry, len(targets))
	for _, t := range targets {
		byKey[t.Key] = t
	}
	a.mut.Lock()
	a.full[componentID] = byKey
	a.mut.Unlock()
}

// reportWeights merges a node's measured per-target series counts into the
// global aggregate the leader weights with. Each target is scraped by exactly
// one node, so the union across nodes is the full per-target weight map.
func (a *targetAllocator) reportWeights(weights map[uint64]uint64) {
	a.mut.Lock()
	defer a.mut.Unlock()
	for k, w := range weights {
		a.weights[k] = w
	}
}

// computeAll recomputes the assignment for every component from the current
// targets + weights, using the ring sharder and participant set. Returns the
// set of component IDs whose assignment changed.
func (a *targetAllocator) computeAll(sharder shard.Sharder, nodes []string) {
	a.mut.Lock()
	defer a.mut.Unlock()

	for componentID, byKey := range a.full {
		keys := make([]shard.Key, 0, len(byKey))
		for k := range byKey {
			keys = append(keys, shard.Key(k))
		}
		prev := a.assignment[componentID]
		prevKeyed := toKeyed(prev)

		chosen := SeriesAssignment(sharder, keys, a.weights, prevKeyed, nodes)
		a.assignment[componentID] = fromKeyed(chosen)
	}
}

// slice returns the targets assigned to node for a component (after computeAll).
func (a *targetAllocator) slice(componentID, node string) []TargetEntry {
	a.mut.Lock()
	defer a.mut.Unlock()

	owners := a.assignment[componentID]
	byKey := a.full[componentID]
	if owners == nil || byKey == nil {
		return nil
	}

	out := make([]TargetEntry, 0, len(owners))
	for key, owner := range owners {
		if owner != node {
			continue
		}
		if t, ok := byKey[key]; ok {
			out = append(out, t)
		}
	}
	// Stable order so equal assignments produce identical output.
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// stats returns the number of distinct registered targets and how many of them
// have a measured series weight. Used for diagnostic metrics.
func (a *targetAllocator) stats() (registered, weighted int) {
	a.mut.Lock()
	defer a.mut.Unlock()
	seen := make(map[uint64]struct{})
	for _, byKey := range a.full {
		for k := range byKey {
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			if a.weights[k] > 0 {
				weighted++
			}
		}
	}
	return len(seen), weighted
}

// dropNode removes a node's reported weights and ownership across all components
// (called when a peer leaves) so stale state doesn't linger.
func (a *targetAllocator) dropNode(node string) {
	a.mut.Lock()
	defer a.mut.Unlock()
	for _, owners := range a.assignment {
		for key, owner := range owners {
			if owner == node {
				delete(owners, key)
			}
		}
	}
}

// --- alloyCluster methods (the Cluster interface surface used by components) ---

// AllocatorEnabled implements Cluster.
func (c *alloyCluster) AllocatorEnabled() bool {
	return c.opts.EnableTargetAllocator
}

// IsAllocatorLeader implements Cluster.
func (c *alloyCluster) IsAllocatorLeader() bool {
	ps, err := c.sharder.Lookup(allocatorLeaderKey, 1, shard.OpReadWrite)
	return err == nil && len(ps) > 0 && ps[0].Self
}

// RegisterDiscoveredTargets implements Cluster. It stores the leader's freshly
// discovered set and immediately recomputes the assignment, so newly discovered
// targets are assigned (and become visible to AssignedTargets and to followers'
// pulls) without waiting for the next membership change.
func (c *alloyCluster) RegisterDiscoveredTargets(componentID string, targets []TargetEntry) {
	c.allocator.setTargets(componentID, targets)
	c.allocator.computeAll(c.sharder, participantNames(c.sharder.Peers()))
}

// ReportWeights implements Cluster. weights is a scrape component's measured
// series count per target key. It is MERGED into the node's local weight view
// (not replaced): a node runs several scrape components, each measuring a
// disjoint set of target keys, so merging unions them. The merged view is sent
// to the leader on the next allocator pull. Stale keys for targets no longer
// scraped linger harmlessly — SeriesAssignment only looks up keys that are still
// in a component's current target set.
func (c *alloyCluster) ReportWeights(weights map[uint64]uint64) {
	c.localWeightsMut.Lock()
	defer c.localWeightsMut.Unlock()
	for k, w := range weights {
		c.localWeights[k] = w
	}
}

// localWeightsSnapshot returns a copy of the node's merged weight view, safe to
// read/serialize without holding the lock.
func (c *alloyCluster) localWeightsSnapshot() map[uint64]uint64 {
	c.localWeightsMut.Lock()
	defer c.localWeightsMut.Unlock()
	out := make(map[uint64]uint64, len(c.localWeights))
	for k, w := range c.localWeights {
		out[k] = w
	}
	return out
}

// localSeriesSum returns the total series this node has measured and reported.
func (c *alloyCluster) localSeriesSum() uint64 {
	c.localWeightsMut.Lock()
	defer c.localWeightsMut.Unlock()
	var sum uint64
	for _, w := range c.localWeights {
		sum += w
	}
	return sum
}

// AssignedTargets implements Cluster. On the leader it returns the locally
// computed slice; on a follower it fetches the slice from the leader over HTTP,
// sending this node's measured weights along.
func (c *alloyCluster) AssignedTargets(componentID string) ([]TargetEntry, error) {
	weights := c.localWeightsSnapshot()

	if c.IsAllocatorLeader() {
		c.allocator.reportWeights(weights)
		return c.allocator.slice(componentID, c.opts.NodeName), nil
	}
	return c.fetchSliceFromLeader(componentID, weights)
}

func (c *alloyCluster) fetchSliceFromLeader(componentID string, weights map[uint64]uint64) ([]TargetEntry, error) {
	ps, err := c.sharder.Lookup(allocatorLeaderKey, 1, shard.OpReadWrite)
	if err != nil || len(ps) == 0 {
		return nil, fmt.Errorf("no target-allocator leader available: %w", err)
	}
	leader := ps[0]

	scheme := "http"
	if c.useTLS {
		scheme = "https"
	}
	url := scheme + "://" + leader.Addr + c.allocatorPath

	body, err := json.Marshal(allocatorRequest{Component: componentID, Node: c.opts.NodeName, Weights: weights})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching assigned targets from leader %s: %w", leader.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("leader %s returned %d: %s", leader.Name, resp.StatusCode, msg)
	}

	var out []TargetEntry
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding assigned targets: %w", err)
	}
	return out, nil
}

// serveAllocatorTargets is the leader's HTTP handler: it records the caller's
// reported weights and returns the caller's assigned slice for the component.
func (s *Service) serveAllocatorTargets(w http.ResponseWriter, r *http.Request) {
	if !s.alloyCluster.IsAllocatorLeader() {
		// This node isn't the leader (stale view on the caller's side); tell it to retry.
		http.Error(w, "not the target-allocator leader", http.StatusMisdirectedRequest)
		return
	}

	var req allocatorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Component == "" || req.Node == "" {
		http.Error(w, "component and node are required", http.StatusBadRequest)
		return
	}

	if req.Weights != nil {
		s.allocator.reportWeights(req.Weights)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.allocator.slice(req.Component, req.Node))
}

// recomputeAllocator recomputes every component's assignment. Only the leader
// computes; followers are no-ops (they pull their slice from the leader).
func (s *Service) recomputeAllocator() {
	if !s.alloyCluster.IsAllocatorLeader() {
		return
	}
	s.allocator.computeAll(s.sharder, participantNames(s.node.Peers()))
}

func toKeyed(m map[uint64]string) map[shard.Key]string {
	if m == nil {
		return nil
	}
	out := make(map[shard.Key]string, len(m))
	for k, v := range m {
		out[shard.Key(k)] = v
	}
	return out
}

func fromKeyed(m map[shard.Key]string) map[uint64]string {
	out := make(map[uint64]string, len(m))
	for k, v := range m {
		out[uint64(k)] = v
	}
	return out
}
