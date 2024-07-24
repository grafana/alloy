package cluster

import (
	"math/rand"
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func mockDiscoverPeers(peers []string, err error) func() ([]string, error) {
	return func() ([]string, error) {
		return peers, err
	}
}

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
			s := &Service{
				log:     log.NewLogfmtLogger(os.Stdout),
				opts:    test.opts,
				randGen: rand.New(rand.NewSource(1)), // Seeded random generator to have consistent results in tests.
			}

			peers, _ := s.getPeers()

			require.ElementsMatch(t, peers, test.expectedPeers)
		})
	}
}
