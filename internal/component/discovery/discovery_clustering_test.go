package discovery

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
)

// allocFakeCluster is a minimal cluster.Cluster for exercising the discovery
// component's allocator-mode routing. It simulates a single leader that owns
// every target it discovers, or a follower that is served a fixed slice.
type allocFakeCluster struct {
	mut sync.Mutex

	enabled bool
	leader  bool

	registered map[string][]cluster.TargetEntry // captured from RegisterDiscoveredTargets
	assigned   map[string][]cluster.TargetEntry  // returned by AssignedTargets
}

func newAllocFakeCluster(enabled, leader bool) *allocFakeCluster {
	return &allocFakeCluster{
		enabled:    enabled,
		leader:     leader,
		registered: map[string][]cluster.TargetEntry{},
		assigned:   map[string][]cluster.TargetEntry{},
	}
}

func (f *allocFakeCluster) AllocatorEnabled() bool { return f.enabled }
func (f *allocFakeCluster) IsAllocatorLeader() bool { return f.leader }

func (f *allocFakeCluster) RegisterDiscoveredTargets(componentID string, targets []cluster.TargetEntry) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.registered[componentID] = targets
	// A single-node leader owns everything it discovers.
	f.assigned[componentID] = targets
}

func (f *allocFakeCluster) AssignedTargets(componentID string) ([]cluster.TargetEntry, error) {
	f.mut.Lock()
	defer f.mut.Unlock()
	return f.assigned[componentID], nil
}

func (f *allocFakeCluster) setAssigned(componentID string, targets []cluster.TargetEntry) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.assigned[componentID] = targets
}

func (f *allocFakeCluster) registeredFor(componentID string) []cluster.TargetEntry {
	f.mut.Lock()
	defer f.mut.Unlock()
	return f.registered[componentID]
}

// The rest of the Cluster interface is unused by these tests.
func (f *allocFakeCluster) Lookup(shard.Key, int, shard.Op) ([]peer.Peer, error) { return nil, nil }
func (f *allocFakeCluster) Peers() []peer.Peer                                   { return nil }
func (f *allocFakeCluster) Ready() bool                                          { return true }
func (f *allocFakeCluster) ReportWeights(map[uint64]uint64)                      {}

// recordingDiscoverer is a fakeDiscoverer that records whether Run was ever
// called, so a test can assert a follower never starts the discoverer.
type recordingDiscoverer struct {
	*fakeDiscoverer
	started *atomicBool
}

func newRecordingDiscoverer() *recordingDiscoverer {
	return &recordingDiscoverer{fakeDiscoverer: newFakeDiscoverer(), started: &atomicBool{}}
}

func (r *recordingDiscoverer) Run(ctx context.Context, ch chan<- []*targetgroup.Group) {
	r.started.Store(true)
	r.fakeDiscoverer.Run(ctx, ch)
}

type atomicBool struct {
	mu sync.Mutex
	v  bool
}

func (a *atomicBool) Store(v bool) { a.mu.Lock(); a.v = v; a.mu.Unlock() }
func (a *atomicBool) Load() bool   { a.mu.Lock(); defer a.mu.Unlock(); return a.v }

func newClusteringTestComponent(t *testing.T, clustered bool, clstr cluster.Cluster) (*Component, *exportRecorder) {
	rec := &exportRecorder{}
	opts := component.Options{
		ID:            "discovery.test",
		Logger:        util.TestAlloyLogger(t).Slog(),
		OnStateChange: rec.record,
		GetServiceData: func(name string) (any, error) {
			if name == livedebugging.ServiceName {
				return livedebugging.NewLiveDebugging(), nil
			}
			return nil, assert.AnError
		},
	}
	dbg, _ := opts.GetServiceData(livedebugging.ServiceName)
	return &Component{
		opts:               opts,
		newDiscoverer:      make(chan struct{}, 1),
		clusterChange:      make(chan struct{}, 1),
		cluster:            clstr,
		clustered:          clustered,
		debugDataPublisher: dbg.(livedebugging.DebugDataPublisher),
	}, rec
}

type exportRecorder struct {
	mu   sync.Mutex
	last []Target
	n    int
}

func (r *exportRecorder) record(e component.Exports) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last = e.(Exports).Targets
	r.n++
}

func (r *exportRecorder) latest() []Target {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last
}

func tgGroup() []*targetgroup.Group {
	return []*targetgroup.Group{
		{Source: "s", Labels: model.LabelSet{"job": "j"}, Targets: []model.LabelSet{
			{"__address__": "10.0.0.1:80"},
			{"__address__": "10.0.0.2:80"},
		}},
	}
}

// On the leader, the discoverer runs, the full discovered set is registered with
// the allocator, and the node exports its assigned slice.
func TestDiscovery_AllocatorLeader_RegistersAndExportsSlice(t *testing.T) {
	prev := MaxUpdateFrequency
	MaxUpdateFrequency = 50 * time.Millisecond
	defer func() { MaxUpdateFrequency = prev }()

	fc := newAllocFakeCluster(true /*enabled*/, true /*leader*/)
	comp, rec := newClusteringTestComponent(t, true, fc)

	disc := newFakeDiscoverer()
	updateDiscoverer(comp, disc)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _ = comp.Run(ctx) }()

	disc.Publish(tgGroup())

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Len(t, fc.registeredFor("discovery.test"), 2, "leader must register the full discovered set")
		assert.Len(t, rec.latest(), 2, "leader must export its assigned slice")
	}, 3*time.Second, 5*time.Millisecond)
}

// A follower must NOT run the discoverer; it exports the slice the leader serves.
func TestDiscovery_AllocatorFollower_PullsSliceWithoutDiscovering(t *testing.T) {
	prevRefresh := ClusteredRefreshInterval
	ClusteredRefreshInterval = 50 * time.Millisecond
	defer func() { ClusteredRefreshInterval = prevRefresh }()

	fc := newAllocFakeCluster(true /*enabled*/, false /*leader=follower*/)
	// The leader has assigned this follower one target.
	fc.setAssigned("discovery.test", []cluster.TargetEntry{
		{Key: 123, Labels: map[string]string{"__address__": "10.0.0.9:80", "job": "j"}},
	})

	comp, rec := newClusteringTestComponent(t, true, fc)
	disc := newRecordingDiscoverer()
	updateDiscoverer(comp, disc.fakeDiscoverer)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _ = comp.Run(ctx) }()

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Len(t, rec.latest(), 1, "follower must export the leader's slice")
	}, 3*time.Second, 5*time.Millisecond)

	require.False(t, disc.started.Load(), "follower must not run the discoverer")
	require.Empty(t, fc.registeredFor("discovery.test"), "follower must not register targets")
}

// With the feature flag off, a clustering-enabled component behaves exactly as
// the default: it runs the discoverer and exports the full set, never touching
// the allocator.
func TestDiscovery_AllocatorDisabled_ExportsFullSet(t *testing.T) {
	prev := MaxUpdateFrequency
	MaxUpdateFrequency = 50 * time.Millisecond
	defer func() { MaxUpdateFrequency = prev }()

	fc := newAllocFakeCluster(false /*flag off*/, true /*even if "leader"*/)
	comp, rec := newClusteringTestComponent(t, true, fc)

	disc := newFakeDiscoverer()
	updateDiscoverer(comp, disc)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _ = comp.Run(ctx) }()

	disc.Publish(tgGroup())

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Len(t, rec.latest(), 2, "unclustered node exports the full discovered set")
	}, 3*time.Second, 5*time.Millisecond)
	require.Empty(t, fc.registeredFor("discovery.test"), "must not register with the allocator when the flag is off")
}
