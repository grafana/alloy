package alloycli

import (
	"testing"

	"github.com/go-kit/log"
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

func TestGetAdvertiseAddress(t *testing.T) {
	// This tests that an IPv4 advertise address is properly joined to it's port.
	t.Run("IPv4", func(t *testing.T) {
		opts := clusterOptions{
			AdvertiseAddress: "1.1.1.1",
		}

		addr, err := getAdvertiseAddress(opts, 80)
		require.Nil(t, err)
		require.Equal(t, "1.1.1.1:80", addr)
	})

	// This tests that an IPv6 advertise address is properly joined to it's port.
	t.Run("IPv6", func(t *testing.T) {
		opts := clusterOptions{
			AdvertiseAddress: "2606:4700:4700::1111",
		}

		addr, err := getAdvertiseAddress(opts, 80)
		require.Nil(t, err)
		require.Equal(t, "[2606:4700:4700::1111]:80", addr)
	})

	// This tests the loopback fallback.
	t.Run("loopback Fallback", func(t *testing.T) {
		opts := clusterOptions{
			Log:                 log.NewNopLogger(),
			EnableClustering:    true,
			AdvertiseInterfaces: []string{"lo"},
		}

		addr, err := getAdvertiseAddress(opts, 80)
		require.Nil(t, err)
		require.Equal(t, "127.0.0.1:80", addr)
	})
}
