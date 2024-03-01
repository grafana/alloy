package statsd

import (
	"testing"
	"time"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

var (
	exampleRiverConfig = `
		listen_udp = ":1010"
		listen_tcp = ":1011"
		listen_unixgram = "unix"
		unix_socket_mode = "prom"
		mapping_config_path = "./testdata/mapTest.yaml"
		read_buffer = 1
		cache_size = 2
		cache_type = "random"
		event_queue_size = 1000
		event_flush_interval = "1m"
		parse_dogstatsd_tags = true
		parse_influxdb_tags = false
		parse_librato_tags = false
		parse_signalfx_tags = false
		relay_addr = "localhost:7125"
		relay_packet_length = 2000
		`
	duration1m, _ = time.ParseDuration("1m")
)

func TestRiverUnmarshal(t *testing.T) {
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	require.Equal(t, ":1010", args.ListenUDP)
	require.Equal(t, ":1011", args.ListenTCP)
	require.Equal(t, "unix", args.ListenUnixgram)
	require.Equal(t, "prom", args.UnixSocketMode)
	require.Equal(t, 1, args.ReadBuffer)
	require.Equal(t, 2, args.CacheSize)
	require.Equal(t, "random", args.CacheType)
	require.Equal(t, 1000, args.EventQueueSize)
	require.Equal(t, duration1m, args.EventFlushInterval)
	require.Equal(t, true, args.ParseDogStatsd)
	require.Equal(t, false, args.ParseInfluxDB)
	require.Equal(t, false, args.ParseLibrato)
	require.Equal(t, false, args.ParseSignalFX)
	require.Equal(t, `./testdata/mapTest.yaml`, args.MappingConfig)
	require.Equal(t, "localhost:7125", args.RelayAddr)
	require.Equal(t, 2000, args.RelayPacketLength)
}

func TestConvert(t *testing.T) {
	t.Run("with valid config", func(t *testing.T) {
		var args Arguments
		err := river.Unmarshal([]byte(exampleRiverConfig), &args)
		require.NoError(t, err)

		configStatsd, err := args.Convert()
		require.NoError(t, err)

		require.Equal(t, ":1010", args.ListenUDP)
		require.Equal(t, ":1011", args.ListenTCP)
		require.Equal(t, "unix", args.ListenUnixgram)
		require.Equal(t, "prom", args.UnixSocketMode)
		require.Equal(t, 1, args.ReadBuffer)
		require.Equal(t, 2, args.CacheSize)
		require.Equal(t, "random", args.CacheType)
		require.Equal(t, 1000, args.EventQueueSize)
		require.Equal(t, duration1m, configStatsd.EventFlushInterval)
		require.Equal(t, true, configStatsd.ParseDogStatsd)
		require.Equal(t, false, configStatsd.ParseInfluxDB)
		require.Equal(t, false, configStatsd.ParseLibrato)
		require.Equal(t, false, configStatsd.ParseSignalFX)
		require.Equal(t, "localhost:7125", configStatsd.RelayAddr)
		require.Equal(t, 2000, configStatsd.RelayPacketLength)
		require.NotNil(t, configStatsd.MappingConfig)
	})

	t.Run("with empty config", func(t *testing.T) {
		var args Arguments
		err := river.Unmarshal([]byte(""), &args)
		require.NoError(t, err)

		configStatsd, err := args.Convert()
		require.NoError(t, err)

		require.Nil(t, configStatsd.MappingConfig)
	})
}
