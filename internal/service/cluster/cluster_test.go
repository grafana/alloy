package cluster

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
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
				log:     log.NewLogfmtLogger(os.Stdout),
				opts:    test.opts,
				randGen: rand.New(rand.NewSource(1)),
			}

			peers, _ := s.getPeers()

			require.ElementsMatch(t, peers, test.expectedPeers)
		})
	}
}

func TestReadyToAdmitTraffic(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name               string
		enableClustering   bool
		minimumClusterSize int
		waitTimeout        time.Duration
		deadline           time.Time
		peerCount          int
		expectedReady      bool
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
			name:               "deadline passed",
			enableClustering:   true,
			minimumClusterSize: 5,
			waitTimeout:        5 * time.Minute,
			deadline:           now.Add(-1 * time.Minute), // deadline in the past
			peerCount:          1,                         // less than minimum
			expectedReady:      true,
		},
		{
			name:               "enough peers",
			enableClustering:   true,
			minimumClusterSize: 3,
			waitTimeout:        5 * time.Minute,
			deadline:           now.Add(5 * time.Minute), // deadline in the future
			peerCount:          3,                        // equal to minimum
			expectedReady:      true,
		},
		{
			name:               "not enough peers, deadline not passed",
			enableClustering:   true,
			minimumClusterSize: 5,
			waitTimeout:        5 * time.Minute,
			deadline:           now.Add(5 * time.Minute), // deadline in the future
			peerCount:          2,                        // less than minimum
			expectedReady:      false,
		},
		{
			name:               "not enough peers, no deadline set",
			enableClustering:   true,
			minimumClusterSize: 5,
			waitTimeout:        0,           // no timeout
			deadline:           time.Time{}, // zero value
			peerCount:          2,           // less than minimum
			expectedReady:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			peers := buildPeers(tt.peerCount)

			s := newTestService(Options{
				EnableClustering:       tt.enableClustering,
				MinimumClusterSize:     tt.minimumClusterSize,
				MinimumSizeWaitTimeout: tt.waitTimeout,
			}, peers, tt.deadline)

			assert.Equal(t, tt.expectedReady, s.alloyCluster.Ready())
		})
	}
}

func TestAdmitTrafficSequence_WithDeadline(t *testing.T) {
	t.Parallel()
	minimumClusterSize := 10
	clusterSizeWaitTimeout := time.Second

	s := newTestService(Options{
		EnableClustering:       true,
		MinimumClusterSize:     minimumClusterSize,
		MinimumSizeWaitTimeout: clusterSizeWaitTimeout,
	}, buildPeers(1), time.Now().Add(1*time.Minute))
	s.alloyCluster.limiter = rate.NewLimiter(rate.Every(time.Millisecond), 1000) // effectively disable rate limiter for this test

	assert.False(t, s.alloyCluster.Ready()) // starts as not ready

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize)}) // we reach the minimum, should be ready now!
	assert.True(t, s.alloyCluster.Ready())

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize - 1)}) // we dip back under the minimum = not ready
	assert.False(t, s.alloyCluster.Ready())

	time.Sleep(time.Second) // deadline passes though, so we are ready to admit traffic again
	assert.True(t, s.alloyCluster.Ready())

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize + 1)}) // we reach the minimum, should continue to be ready
	assert.True(t, s.alloyCluster.Ready())

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize - 5)}) // we dip back under the minimum = not ready, deadline should have reset
	assert.False(t, s.alloyCluster.Ready())

	time.Sleep(time.Second) // deadline passes again, so we are ready to admit traffic again
	assert.True(t, s.alloyCluster.Ready())

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize)}) // we reach the minimum, should continue to be ready
	assert.True(t, s.alloyCluster.Ready())
}

func TestAdmitTrafficSequence_NoDeadline(t *testing.T) {
	t.Parallel()
	minimumClusterSize := 10

	s := newTestService(Options{
		EnableClustering:   true,
		MinimumClusterSize: minimumClusterSize,
	}, buildPeers(1), time.Now().Add(1*time.Minute))
	s.alloyCluster.limiter = rate.NewLimiter(rate.Every(time.Millisecond), 1000) // effectively disable rate limiter for this test

	assert.False(t, s.alloyCluster.Ready()) // starts as not ready

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize)}) // we reach the minimum, should be ready now!
	assert.True(t, s.alloyCluster.Ready())

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize - 1)}) // we dip back under the minimum = not ready
	assert.False(t, s.alloyCluster.Ready())

	time.Sleep(time.Second) // even though time passes by, there is no deadline, and we're still not ready
	assert.False(t, s.alloyCluster.Ready())

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize + 1)}) // we reach the minimum, should be ready
	assert.True(t, s.alloyCluster.Ready())

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize - 5)}) // we dip back under the minimum = not ready
	assert.False(t, s.alloyCluster.Ready())

	time.Sleep(time.Second) // time passes, but nothing will change
	assert.False(t, s.alloyCluster.Ready())

	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize)}) // we reach the minimum, should become ready
	assert.True(t, s.alloyCluster.Ready())
}

func TestAdmitTrafficSequence_RateLimited(t *testing.T) {
	t.Parallel()
	minimumClusterSize := 10
	limiterInterval := time.Second * 2 // makes a test a bit longer, but lower risk of flakes when GC happens

	s := newTestService(Options{
		EnableClustering:   true,
		MinimumClusterSize: minimumClusterSize,
	}, buildPeers(1), time.Time{})
	s.alloyCluster.limiter = rate.NewLimiter(rate.Every(limiterInterval), 1)

	// not enough peers - not ready
	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize - 1)})
	assert.False(t, s.alloyCluster.Ready())

	// enough peers, but rate limited
	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize)})
	for i := 0; i < 10; i++ {
		assert.False(t, s.alloyCluster.Ready())
	}

	// rate limit passed - ready
	time.Sleep(limiterInterval)
	for i := 0; i < 10; i++ {
		assert.True(t, s.alloyCluster.Ready())
	}

	// dip below required - but we are rate limited - still ready
	updateSharder(s, &mockSharder{peers: buildPeers(minimumClusterSize - 1)})
	for i := 0; i < 10; i++ {
		assert.True(t, s.alloyCluster.Ready())
	}

	// rate limit passed - we are not ready now
	time.Sleep(limiterInterval)
	for i := 0; i < 10; i++ {
		assert.False(t, s.alloyCluster.Ready())
	}
}

func updateSharder(service *Service, sharder *mockSharder) {
	service.sharder = sharder
	service.alloyCluster.sharder = sharder
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

func newTestService(opts Options, peers []peer.Peer, deadline time.Time) *Service {
	logger := log.NewLogfmtLogger(os.Stdout)
	sharder := &mockSharder{peers: peers}
	ac := newAlloyCluster(log.With(logger, "subcomponent", "alloy_cluster"), sharder, opts)
	ac.minimumSizeDeadline.Store(deadline)
	return &Service{
		log:          logger,
		opts:         opts,
		alloyCluster: ac,
		sharder:      sharder,
	}
}
