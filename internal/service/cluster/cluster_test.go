package cluster

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/util"
)

func TestGetPeers(t *testing.T) {
	tests := []struct {
		name              string
		opts              Options
		expectedPeers     []string
		expectedError     error
		discoverPeersMock func() ([]string, error)
	}{
		{
			name:          "Test clustering disabled",
			opts:          Options{EnableClustering: false},
			expectedPeers: nil,
		},
		{
			name:          "Test no max peers limit",
			opts:          Options{EnableClustering: true, ClusterMaxJoinPeers: 0, DiscoverPeers: mockDiscoverPeers([]string{"A", "B"}, nil)},
			expectedPeers: []string{"A", "B"},
		},
		{
			name:          "Test max higher than number of peers",
			opts:          Options{EnableClustering: true, ClusterMaxJoinPeers: 5, DiscoverPeers: mockDiscoverPeers([]string{"A", "B", "C"}, nil)},
			expectedPeers: []string{"A", "B", "C"},
		},
		{
			name:          "Test max peers limit with shuffling",
			opts:          Options{EnableClustering: true, ClusterMaxJoinPeers: 2, DiscoverPeers: mockDiscoverPeers([]string{"A", "B", "C"}, nil)},
			expectedPeers: []string{"A", "C"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s := &Service{
				log:     util.TestAlloyLogger(t).Slog(),
				opts:    test.opts,
				randGen: rand.New(rand.NewSource(1)),
			}

			peers, _ := s.getRandomPeers()

			require.ElementsMatch(t, peers, test.expectedPeers)
		})
	}
}

func TestReadyToAdmitTraffic(t *testing.T) {
	tests := []struct {
		name                 string
		enableClustering     bool
		minimumClusterSize   int
		waitTimeout          time.Duration
		peerCount            int
		expectedReady        bool
		expectNotifyCallback bool
		sleepFor             time.Duration
	}{
		{
			name:          "defaults",
			peerCount:     1,
			expectedReady: true,
		},
		{
			name:               "clustering disabled",
			enableClustering:   false,
			minimumClusterSize: 5,
			peerCount:          1, // less than minimum but clustering disabled
			expectedReady:      true,
		},
		{
			name:               "clustering disabled and zero peers",
			enableClustering:   false,
			minimumClusterSize: 5,
			peerCount:          0, // no peers but clustering disabled
			expectedReady:      true,
		},
		{
			name:               "no minimum size requirement",
			enableClustering:   true,
			minimumClusterSize: 0,
			waitTimeout:        5 * time.Minute,
			peerCount:          1,
			expectedReady:      true,
		},
		{
			name:               "no minimum size requirement zero peers",
			enableClustering:   true,
			minimumClusterSize: 0,
			waitTimeout:        5 * time.Minute,
			peerCount:          0,
			expectedReady:      true,
		},
		{
			name:                 "deadline passed",
			enableClustering:     true,
			minimumClusterSize:   5,
			waitTimeout:          150 * time.Millisecond,
			sleepFor:             200 * time.Millisecond,
			peerCount:            1, // less than minimum
			expectedReady:        true,
			expectNotifyCallback: true,
		},
		{
			name:                 "enough peers",
			enableClustering:     true,
			minimumClusterSize:   3,
			waitTimeout:          5 * time.Minute,
			peerCount:            3, // equal to minimum
			expectedReady:        true,
			expectNotifyCallback: false,
		},
		{
			name:               "not enough peers, deadline not passed",
			enableClustering:   true,
			minimumClusterSize: 5,
			waitTimeout:        5 * time.Minute,
			peerCount:          2, // less than minimum
			expectedReady:      false,
		},
		{
			name:               "not enough peers, no deadline set",
			enableClustering:   true,
			minimumClusterSize: 5,
			waitTimeout:        0, // no timeout
			peerCount:          2, // less than minimum
			expectedReady:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			peers := buildPeers(tt.peerCount)

			notifyChangeCallbackCalled := atomic.NewBool(false)
			s := newTestService(t, Options{
				EnableClustering:       tt.enableClustering,
				MinimumClusterSize:     tt.minimumClusterSize,
				MinimumSizeWaitTimeout: tt.waitTimeout,
			}, peers, func() {
				notifyChangeCallbackCalled.Store(true)
			})
			defer s.alloyCluster.shutdown()

			if tt.sleepFor > 0 {
				time.Sleep(tt.sleepFor)
			}

			s.alloyCluster.updateReadyState()
			assert.Equal(t, tt.expectedReady, s.alloyCluster.Ready())
			assert.Equal(t, tt.expectNotifyCallback, notifyChangeCallbackCalled.Load())
		})
	}
}

func TestAdmitTrafficSequence_WithDeadline(t *testing.T) {
	t.Parallel()
	minimumClusterSize := 10
	clusterSizeWaitTimeout := time.Second

	notifyChangeCallsCount := atomic.NewInt32(0)
	s := newTestService(t, Options{
		EnableClustering:       true,
		MinimumClusterSize:     minimumClusterSize,
		MinimumSizeWaitTimeout: clusterSizeWaitTimeout,
	}, buildPeers(1), func() {
		notifyChangeCallsCount.Inc()
	})
	defer s.alloyCluster.shutdown()

	assert.False(t, s.alloyCluster.Ready()) // starts as not ready
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())

	updatePeers(s, &mockSharder{peers: buildPeers(minimumClusterSize)}) // we reach the minimum, should be ready now!
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())

	updatePeers(s, &mockSharder{peers: buildPeers(minimumClusterSize - 1)}) // we dip back under the minimum = still ready
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())

	time.Sleep(clusterSizeWaitTimeout + 200*time.Millisecond) // the deadline timer must not fire once ready
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())

	updatePeers(s, &mockSharder{peers: buildPeers(1)}) // even a drastic drop doesn't make us not ready
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())
}

func TestAdmitTrafficSequence_DeadlinePassedBeforeMinimumReached(t *testing.T) {
	t.Parallel()
	minimumClusterSize := 10
	clusterSizeWaitTimeout := 500 * time.Millisecond

	notifyChangeCallsCount := atomic.NewInt32(0)
	s := newTestService(t, Options{
		EnableClustering:       true,
		MinimumClusterSize:     minimumClusterSize,
		MinimumSizeWaitTimeout: clusterSizeWaitTimeout,
	}, buildPeers(1), func() {
		notifyChangeCallsCount.Inc()
	})
	defer s.alloyCluster.shutdown()

	assert.False(t, s.alloyCluster.Ready()) // starts as not ready

	require.Eventually(t, func() bool {
		return s.alloyCluster.Ready()
	}, 3*time.Second, time.Millisecond) // deadline passes, so we are ready to admit traffic
	assert.Equal(t, int32(1), notifyChangeCallsCount.Load()) // deadline pass must trigger notify change callback

	updatePeers(s, &mockSharder{peers: buildPeers(minimumClusterSize)}) // we reach the minimum, should continue to be ready
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(1), notifyChangeCallsCount.Load())

	updatePeers(s, &mockSharder{peers: buildPeers(minimumClusterSize - 5)}) // we dip back under the minimum = still ready
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(1), notifyChangeCallsCount.Load())

	time.Sleep(clusterSizeWaitTimeout + 200*time.Millisecond) // no new deadline is started, nothing changes
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(1), notifyChangeCallsCount.Load())
}

func TestAdmitTrafficSequence_NoDeadline(t *testing.T) {
	t.Parallel()
	minimumClusterSize := 10

	notifyChangeCallsCount := atomic.NewInt32(0)
	s := newTestService(t, Options{
		EnableClustering:   true,
		MinimumClusterSize: minimumClusterSize,
	}, buildPeers(1), func() {
		notifyChangeCallsCount.Inc()
	})
	defer s.alloyCluster.shutdown()

	assert.False(t, s.alloyCluster.Ready()) // starts as not ready
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())

	updatePeers(s, &mockSharder{peers: buildPeers(minimumClusterSize - 1)}) // growing, but not yet at the minimum
	assert.False(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())

	updatePeers(s, &mockSharder{peers: buildPeers(minimumClusterSize)}) // we reach the minimum, should be ready now!
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())

	updatePeers(s, &mockSharder{peers: buildPeers(minimumClusterSize - 1)}) // we dip back under the minimum = still ready
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())

	updatePeers(s, &mockSharder{peers: buildPeers(minimumClusterSize - 5)}) // dip further = still ready
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())

	updatePeers(s, &mockSharder{peers: buildPeers(minimumClusterSize + 1)}) // grow again = still ready
	assert.True(t, s.alloyCluster.Ready())
	assert.Equal(t, int32(0), notifyChangeCallsCount.Load())
}

func updatePeers(service *Service, sharder *mockSharder) {
	service.sharder = sharder
	service.alloyCluster.sharder = sharder
	service.alloyCluster.updateReadyState()
}

type mockSharder struct {
	peers []peer.Peer
}

func (m *mockSharder) Lookup(_ shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	return m.peers, nil
}

func (m *mockSharder) Peers() []peer.Peer {
	return m.peers
}

func (m *mockSharder) SetPeers(_ []peer.Peer) {}

func mockDiscoverPeers(peers []string, err error) func() ([]string, error) {
	return func() ([]string, error) {
		return peers, err
	}
}

func buildPeers(count int) []peer.Peer {
	var peers []peer.Peer
	for i := 0; i < count; i++ {
		peers = append(peers, peer.Peer{
			Name: fmt.Sprintf("peer_%d", i),
		})
	}
	return peers
}

func newTestService(t *testing.T, opts Options, peers []peer.Peer, callback func()) *Service {
	logger := util.TestAlloyLogger(t).Slog()
	sharder := &mockSharder{peers: peers}
	ac := newAlloyCluster(sharder, callback, opts, logger.With("subcomponent", "alloy_cluster"))
	return &Service{
		log:          logger,
		opts:         opts,
		alloyCluster: ac,
		sharder:      sharder,
	}
}
