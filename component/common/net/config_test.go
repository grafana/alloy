package net

import (
	"testing"
	"time"

	dskit "github.com/grafana/dskit/server"
	"github.com/stretchr/testify/require"

	"github.com/grafana/river"
)

// testArguments mimics an arguments type used by a component, applying the defaults to ServerConfig
// from it's UnmarshalRiver implementation, since the block is squashed.
type testArguments struct {
	Server *ServerConfig `river:",squash"`
}

func (t *testArguments) UnmarshalRiver(f func(v interface{}) error) error {
	// apply server defaults from here since the fields are squashed
	*t = testArguments{
		Server: DefaultServerConfig(),
	}

	type args testArguments
	err := f((*args)(t))
	if err != nil {
		return err
	}
	return nil
}

func TestConfig(t *testing.T) {
	type testcase struct {
		raw         string
		errExpected bool
		assert      func(t *testing.T, config dskit.Config)
	}
	var cases = map[string]testcase{
		"empty config applies defaults": {
			raw: ``,
			assert: func(t *testing.T, config dskit.Config) {
				// custom defaults
				require.Equal(t, DefaultHTTPPort, config.HTTPListenPort)
				require.Equal(t, DefaultGRPCPort, config.GRPCListenPort)
				// defaults inherited from dskit
				require.Equal(t, "", config.HTTPListenAddress)
				require.Equal(t, "", config.GRPCListenAddress)
				require.False(t, config.RegisterInstrumentation)
				require.Equal(t, time.Second*30, config.ServerGracefulShutdownTimeout)

				require.Equal(t, size4MB, config.GRPCServerMaxSendMsgSize)
				require.Equal(t, size4MB, config.GRPCServerMaxRecvMsgSize)
			},
		},
		"overriding defaults": {
			raw: `
			graceful_shutdown_timeout = "1m"
			http {
				listen_port = 8080
				listen_address = "0.0.0.0"
				conn_limit = 10
				server_write_timeout = "10s"
			}`,
			assert: func(t *testing.T, config dskit.Config) {
				require.Equal(t, 8080, config.HTTPListenPort)
				require.Equal(t, "0.0.0.0", config.HTTPListenAddress)
				require.Equal(t, 10, config.HTTPConnLimit)
				require.Equal(t, time.Second*10, config.HTTPServerWriteTimeout)

				require.Equal(t, time.Minute, config.ServerGracefulShutdownTimeout)
			},
		},
		"overriding just some defaults": {
			raw: `
			graceful_shutdown_timeout = "1m"
			http {
				listen_port = 8080
				listen_address = "0.0.0.0"
				conn_limit = 10
			}
			grpc {
				listen_port = 8080
				listen_address = "0.0.0.0"
				server_max_send_msg_size = 10
			}`,
			assert: func(t *testing.T, config dskit.Config) {
				// these should be overridden
				require.Equal(t, 8080, config.HTTPListenPort)
				require.Equal(t, "0.0.0.0", config.HTTPListenAddress)
				require.Equal(t, 10, config.HTTPConnLimit)
				// this should have the default applied
				require.Equal(t, 30*time.Second, config.HTTPServerReadTimeout)

				// these should be overridden
				require.Equal(t, 8080, config.GRPCListenPort)
				require.Equal(t, "0.0.0.0", config.GRPCListenAddress)
				require.Equal(t, 10, config.GRPCServerMaxSendMsgSize)
				// this should have the default applied
				require.Equal(t, size4MB, config.GRPCServerMaxRecvMsgSize)

				require.Equal(t, time.Minute, config.ServerGracefulShutdownTimeout)
			},
		},
		"all params": {
			raw: `
			graceful_shutdown_timeout = "1m"
			http {
				listen_address = "0.0.0.0"
				listen_port = 1
				conn_limit = 2
				server_read_timeout = "2m"
				server_write_timeout = "3m"
				server_idle_timeout = "4m"
			}

			grpc {
				listen_address = "0.0.0.1"
				listen_port = 3
				conn_limit = 4
				max_connection_age = "5m"
				max_connection_age_grace = "6m"
				max_connection_idle = "7m"
				server_max_recv_msg_size = 5
				server_max_send_msg_size = 6
				server_max_concurrent_streams = 7
			}`,
			assert: func(t *testing.T, config dskit.Config) {
				// general
				require.Equal(t, time.Minute, config.ServerGracefulShutdownTimeout)
				// http
				require.Equal(t, "0.0.0.0", config.HTTPListenAddress)
				require.Equal(t, 1, config.HTTPListenPort)
				require.Equal(t, 2, config.HTTPConnLimit)
				require.Equal(t, time.Minute*2, config.HTTPServerReadTimeout)
				require.Equal(t, time.Minute*3, config.HTTPServerWriteTimeout)
				require.Equal(t, time.Minute*4, config.HTTPServerIdleTimeout)
				// grpc
				require.Equal(t, "0.0.0.1", config.GRPCListenAddress)
				require.Equal(t, 3, config.GRPCListenPort)
				require.Equal(t, 5*time.Minute, config.GRPCServerMaxConnectionAge)
				require.Equal(t, 6*time.Minute, config.GRPCServerMaxConnectionAgeGrace)
				require.Equal(t, 7*time.Minute, config.GRPCServerMaxConnectionIdle)
				require.Equal(t, 5, config.GRPCServerMaxRecvMsgSize)
				require.Equal(t, 6, config.GRPCServerMaxSendMsgSize)
				require.Equal(t, uint(7), config.GRPCServerMaxConcurrentStreams)
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			args := testArguments{}
			err := river.Unmarshal([]byte(tc.raw), &args)
			require.Equal(t, tc.errExpected, err != nil)
			wConfig := args.Server.convert()
			tc.assert(t, wConfig)
		})
	}
}
