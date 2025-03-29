package discovery

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/service/cluster"
)

var (
	target1        = mkTarget("instance", "1", "host", "pie")
	target2        = mkTarget("instance", "2", "host", "cake")
	target3        = mkTarget("instance", "3", "host", "muffin")
	allTestTargets = []Target{target1, target2, target3}

	peer1Self    = peer.Peer{Name: "peer1", Addr: "peer1", Self: true, State: peer.StateParticipant}
	peer2        = peer.Peer{Name: "peer2", Addr: "peer2", Self: false, State: peer.StateParticipant}
	peer3        = peer.Peer{Name: "peer3", Addr: "peer3", Self: false, State: peer.StateParticipant}
	allTestPeers = []peer.Peer{peer1Self, peer2, peer3}

	targetWithLookupError = mkTarget("instance", "-1", "host", "error")
	magicErrorKey         = keyFor(targetWithLookupError)
)

var localTargetsTestCases = []struct {
	name                 string
	clusteringDisabled   bool
	cluster              cluster.Cluster
	allTargets           []Target
	expectedLocalTargets []Target
}{
	{
		name:                 "all targets are local when clustering disabled",
		clusteringDisabled:   true,
		cluster:              &fakeCluster{},
		allTargets:           allTestTargets,
		expectedLocalTargets: allTestTargets,
	},
	{
		name:                 "all targets are local when cluster is nil",
		allTargets:           allTestTargets,
		expectedLocalTargets: allTestTargets,
	},
	{
		name:                 "all targets are local when no peers are returned from cluster",
		cluster:              &fakeCluster{peers: nil},
		allTargets:           allTestTargets,
		expectedLocalTargets: allTestTargets,
	},
	{
		name: "only targets assigned to local node are seen as local",
		cluster: &fakeCluster{
			peers: allTestPeers,
			lookupMap: map[shard.Key][]peer.Peer{
				keyFor(target1): {peer1Self},
				keyFor(target2): {peer2},
				keyFor(target3): {peer1Self},
			},
		},
		allTargets: allTestTargets,
		expectedLocalTargets: []Target{
			target1, target3,
		},
	},
	{
		name: "no targets assigned to local node if no keys match it",
		cluster: &fakeCluster{
			peers: allTestPeers,
			lookupMap: map[shard.Key][]peer.Peer{
				keyFor(target1): {peer2},
				keyFor(target2): {peer2},
				keyFor(target3): {peer3},
			},
		},
		allTargets:           allTestTargets,
		expectedLocalTargets: []Target{},
	},
	{
		name: "additional replica peers do not affect local targets assignment",
		cluster: &fakeCluster{
			peers: allTestPeers,
			lookupMap: map[shard.Key][]peer.Peer{
				keyFor(target1): {peer1Self, peer2},
				keyFor(target2): {peer1Self, peer2},
				keyFor(target3): {peer2, peer3},
			},
		},
		allTargets: allTestTargets,
		expectedLocalTargets: []Target{
			target1, target2,
		},
	},
	{
		name: "lookup errors fall back to local target assignment",
		cluster: &fakeCluster{
			peers: allTestPeers,
			lookupMap: map[shard.Key][]peer.Peer{
				magicErrorKey:   {peer2},
				keyFor(target1): {peer1Self},
			},
		},
		allTargets: []Target{target1, targetWithLookupError},
		expectedLocalTargets: []Target{
			target1, targetWithLookupError,
		},
	},
}

func TestDistributedTargets_LocalTargets(t *testing.T) {
	for _, tt := range localTargetsTestCases {
		t.Run(tt.name, func(t *testing.T) {
			dt := NewDistributedTargets(!tt.clusteringDisabled, tt.cluster, tt.allTargets)
			localTargets := dt.LocalTargets()
			require.Equal(t, tt.expectedLocalTargets, localTargets)
		})
	}
}

var movedToRemoteInstanceTestCases = []struct {
	name                 string
	previous             *DistributedTargets
	current              *DistributedTargets
	expectedMovedTargets []Target
}{
	{
		name:     "no previous targets distribution",
		previous: nil,
		current: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer1Self},
			keyFor(target2): {peer2},
			keyFor(target3): {peer1Self},
		}),
		expectedMovedTargets: nil,
	},
	{
		name: "nothing moved",
		previous: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer1Self},
			keyFor(target2): {peer2},
			keyFor(target3): {peer1Self},
		}),
		current: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer1Self},
			keyFor(target2): {peer2},
			keyFor(target3): {peer1Self},
		}),
		expectedMovedTargets: nil,
	},
	{
		name: "all moved",
		previous: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer3},
			keyFor(target2): {peer3},
			keyFor(target3): {peer2},
		}),
		current: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer1Self},
			keyFor(target2): {peer2},
			keyFor(target3): {peer1Self},
		}),
		expectedMovedTargets: nil,
	},
	{
		name: "all moved to local",
		previous: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer3},
			keyFor(target2): {peer2},
			keyFor(target3): {peer2},
		}),
		current: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer1Self},
			keyFor(target2): {peer1Self},
			keyFor(target3): {peer1Self},
		}),
		expectedMovedTargets: nil,
	},
	{
		name: "all moved to remote",
		previous: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer1Self},
			keyFor(target2): {peer1Self},
			keyFor(target3): {peer1Self},
		}),
		current: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer3},
			keyFor(target2): {peer2},
			keyFor(target3): {peer2},
		}),
		expectedMovedTargets: allTestTargets,
	},
	{
		name: "subset moved to remote",
		previous: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer1Self},
			keyFor(target2): {peer1Self},
			keyFor(target3): {peer1Self},
		}),
		current: testDistTargets(map[shard.Key][]peer.Peer{
			keyFor(target1): {peer3},
			keyFor(target2): {peer1Self},
			keyFor(target3): {peer2},
		}),
		expectedMovedTargets: []Target{target1, target3},
	},
}

func TestDistributedTargets_MovedToRemoteInstance(t *testing.T) {
	for _, tt := range movedToRemoteInstanceTestCases {
		t.Run(tt.name, func(t *testing.T) {
			movedTargets := tt.current.MovedToRemoteInstance(tt.previous)
			require.Equal(t, tt.expectedMovedTargets, movedTargets)
		})
	}
}

/*
	 Recent run on M2 MacBook Air:

		$ go test -count=10 -benchmem ./internal/component/discovery -bench BenchmarkDistributedTargets | tee perf_new.txt
		goos: darwin
		goarch: arm64
		pkg: github.com/grafana/alloy/internal/component/discovery
		BenchmarkDistributedTargets-8   	      28	  42125823 ns/op	52016505 B/op	  501189 allocs/op
		...

Comparison to baseline before optimisations:

	$ benchstat perf_baseline.txt perf_new.txt
	goos: darwin
	goarch: arm64
	pkg: github.com/grafana/alloy/internal/component/discovery
						 │ perf_baseline.txt │            perf_new.txt             │
						 │      sec/op       │   sec/op     vs base                │
	DistributedTargets-8       108.42m ± 14%   41.21m ± 2%  -61.99% (p=0.000 n=10)

						 │ perf_baseline.txt │             perf_new.txt             │
						 │       B/op        │     B/op      vs base                │
	DistributedTargets-8        90.06Mi ± 0%   49.66Mi ± 0%  -44.86% (p=0.000 n=10)

						 │ perf_baseline.txt │            perf_new.txt             │
						 │     allocs/op     │  allocs/op   vs base                │
	DistributedTargets-8        1572.1k ± 0%   501.2k ± 0%  -68.12% (p=0.000 n=10)
*/
func BenchmarkDistributedTargets(b *testing.B) {
	const (
		numTargets = 100_000
		numPeers   = 20
	)

	targets := make([]Target, 0, numTargets)
	for i := 0; i < numTargets; i++ {
		targets = append(targets, mkTarget("instance", fmt.Sprintf("%d", i), "host", "pie", "location", "kitchen_counter", "flavour", "delicious", "size", "XXL"))
	}

	peers := make([]peer.Peer, 0, numPeers)
	for i := 0; i < numPeers; i++ {
		peerName := fmt.Sprintf("peer_%d", i)
		peers = append(peers, peer.Peer{Name: peerName, Addr: peerName, Self: i == 0, State: peer.StateParticipant})
	}

	randomLookupMap := make(map[shard.Key][]peer.Peer)
	for _, target := range targets {
		randomLookupMap[keyFor(target)] = []peer.Peer{peers[rand.Int()%numPeers]}
	}

	fakeCluster := &fakeCluster{
		peers:     peers,
		lookupMap: randomLookupMap,
	}

	b.ResetTimer()

	var prev *DistributedTargets
	for i := 0; i < b.N; i++ {
		dt := NewDistributedTargets(true, fakeCluster, targets)
		_ = dt.LocalTargets()
		_ = dt.MovedToRemoteInstance(prev)
		prev = dt
	}
}

func mkTarget(kv ...string) Target {
	target := make(map[string]string)
	for i := 0; i < len(kv); i += 2 {
		target[kv[i]] = kv[i+1]
	}
	return NewTargetFromMap(target)
}

func testDistTargets(lookupMap map[shard.Key][]peer.Peer) *DistributedTargets {
	return NewDistributedTargets(true, &fakeCluster{
		peers:     allTestPeers,
		lookupMap: lookupMap,
	}, allTestTargets)
}

type fakeCluster struct {
	lookupMap map[shard.Key][]peer.Peer
	peers     []peer.Peer
}

func (f *fakeCluster) Lookup(key shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	if key == magicErrorKey {
		return nil, fmt.Errorf("test error for magic error key")
	}
	return f.lookupMap[key], nil
}

func (f *fakeCluster) Peers() []peer.Peer {
	return f.peers
}

func (f *fakeCluster) Ready() bool {
	return true
}
