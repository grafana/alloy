package cluster

import (
	"net/http"

	"github.com/grafana/ckit"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
)

// Mock returns a mock implementation of the Cluster interface.
func Mock() Cluster { return mockCluster{} }

type mockCluster struct{}

func (mockCluster) Lookup(key shard.Key, replicationFactor int, op shard.Op) ([]peer.Peer, error) {
	return []peer.Peer{{
		Name:  "self",
		Addr:  "127.0.0.1",
		Self:  true,
		State: peer.StateParticipant,
	}}, nil
}

func (mockCluster) Peers() []peer.Peer {
	return []peer.Peer{{
		Name:  "self",
		Addr:  "127.0.0.1",
		Self:  true,
		State: peer.StateParticipant,
	}}
}

func (mockCluster) Observe(ckit.Observer) {
	// no-op
}

func (mockCluster) Handler() (string, http.Handler) {
	return "/not/a/valid/path", http.NotFoundHandler()
}
