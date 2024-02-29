package flowmode

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildClusterService(t *testing.T) {
	opts := clusterOptions{
		JoinPeers:     []string{"foo", "bar"},
		DiscoverPeers: "provider=aws key1=val1 key2=val2",
	}

	cs, err := buildClusterService(opts)
	require.Nil(t, cs)
	require.EqualError(t, err, "at most one of join peers and discover peers may be set")
}
