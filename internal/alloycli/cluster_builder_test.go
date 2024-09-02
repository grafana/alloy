package alloycli

import (
	"os"
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

func TestStaticDiscovery(t *testing.T) {
	t.Run("no addresses provided", func(t *testing.T) {
		logger := log.NewLogfmtLogger(os.Stdout)
		sd := newStaticDiscovery([]string{}, 12345, logger)
		actual, err := sd()
		require.NoError(t, err)
		require.Nil(t, actual)
	})
	t.Run("host and port provided", func(t *testing.T) {
		logger := log.NewLogfmtLogger(os.Stdout)
		sd := newStaticDiscovery([]string{"host:8080"}, 12345, logger)
		actual, err := sd()
		require.NoError(t, err)
		require.Equal(t, []string{"host:8080"}, actual)
	})
	t.Run("IP provided and default port added", func(t *testing.T) {
		logger := log.NewLogfmtLogger(os.Stdout)
		sd := newStaticDiscovery([]string{"192.168.0.1"}, 12345, logger)
		actual, err := sd()
		require.NoError(t, err)
		require.Equal(t, []string{"192.168.0.1:12345"}, actual)
	})
	t.Run("fallback to next host and port provided", func(t *testing.T) {
		logger := log.NewLogfmtLogger(os.Stdout)
		sd := newStaticDiscovery([]string{"this | wont | work", "host2:8080"}, 12345, logger)
		actual, err := sd()
		require.NoError(t, err)
		require.Equal(t, []string{"host2:8080"}, actual)
	})
	t.Run("fallback to next host and port provided", func(t *testing.T) {
		logger := log.NewLogfmtLogger(os.Stdout)
		sd := newStaticDiscovery([]string{"this | wont | work", "host2:8080"}, 12345, logger)
		actual, err := sd()
		require.NoError(t, err)
		require.Equal(t, []string{"host2:8080"}, actual)
	})
	t.Run("nothing found", func(t *testing.T) {
		logger := log.NewLogfmtLogger(os.Stdout)
		sd := newStaticDiscovery([]string{"this | wont | work"}, 12345, logger)
		actual, err := sd()
		require.Nil(t, actual)
		require.ErrorContains(t, err, "failed to find any valid join addresses")
		// We can only check the error messages in Alloy's code.
		// We cannot check for error messages from other Go libraries,
		// because they may differ based on operating system and
		// on env var settings such as GODEBUG=netdns.
		require.ErrorContains(t, err, "failed to extract host and port")
		require.ErrorContains(t, err, "failed to resolve SRV records")
	})
}
