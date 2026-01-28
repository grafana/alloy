package net

import (
	"testing"
	"time"

	dskit "github.com/grafana/dskit/server"
	promCfg "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

// testArguments mimics an arguments type used by a component, applying the defaults to ServerConfig
// from it's UnmarshalAlloy implementation, since the block is squashed.
type testArguments struct {
	Server *ServerConfig `alloy:",squash"`
}

func (t *testArguments) UnmarshalAlloy(f func(v any) error) error {
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
		"http with tls config": {
			raw: `
			http {
				listen_port = 8443
				tls {
					cert_pem = "-----BEGIN CERTIFICATE-----\ntest cert\n-----END CERTIFICATE-----"
					key_pem = "-----BEGIN PRIVATE KEY-----\ntest key\n-----END PRIVATE KEY-----"
					cert_file = "/path/to/cert.pem"
					key_file = "/path/to/key.pem"
					client_auth_type = "RequireAndVerifyClientCert"
					client_ca_file = "/path/to/ca.pem"
					client_ca = "-----BEGIN CERTIFICATE-----\nCA cert\n-----END CERTIFICATE-----"
				}
			}`,
			assert: func(t *testing.T, config dskit.Config) {
				require.Equal(t, 8443, config.HTTPListenPort)

				// TLS Config assertions
				require.Equal(t, "-----BEGIN CERTIFICATE-----\ntest cert\n-----END CERTIFICATE-----", config.HTTPTLSConfig.TLSCert)
				require.Equal(t, promCfg.Secret("-----BEGIN PRIVATE KEY-----\ntest key\n-----END PRIVATE KEY-----"), config.HTTPTLSConfig.TLSKey)
				require.Equal(t, "/path/to/cert.pem", config.HTTPTLSConfig.TLSCertPath)
				require.Equal(t, "/path/to/key.pem", config.HTTPTLSConfig.TLSKeyPath)
				require.Equal(t, "RequireAndVerifyClientCert", config.HTTPTLSConfig.ClientAuth)
				require.Equal(t, "/path/to/ca.pem", config.HTTPTLSConfig.ClientCAs)
				require.Equal(t, "-----BEGIN CERTIFICATE-----\nCA cert\n-----END CERTIFICATE-----", config.HTTPTLSConfig.ClientCAsText)
			},
		},
		"grpc with tls config": {
			raw: `
			grpc {
				listen_port = 9443
				tls {
					cert_pem = "-----BEGIN CERTIFICATE-----\ngrpc cert\n-----END CERTIFICATE-----"
					key_pem = "-----BEGIN PRIVATE KEY-----\ngrpc key\n-----END PRIVATE KEY-----"
					cert_file = "/path/to/grpc-cert.pem"
					key_file = "/path/to/grpc-key.pem"
					client_auth_type = "RequestClientCert"
					client_ca_file = "/path/to/grpc-ca.pem"
					client_ca = "-----BEGIN CERTIFICATE-----\ngRPC CA cert\n-----END CERTIFICATE-----"
				}
			}`,
			assert: func(t *testing.T, config dskit.Config) {
				require.Equal(t, 9443, config.GRPCListenPort)

				// gRPC TLS Config assertions
				require.Equal(t, "-----BEGIN CERTIFICATE-----\ngrpc cert\n-----END CERTIFICATE-----", config.GRPCTLSConfig.TLSCert)
				require.Equal(t, promCfg.Secret("-----BEGIN PRIVATE KEY-----\ngrpc key\n-----END PRIVATE KEY-----"), config.GRPCTLSConfig.TLSKey)
				require.Equal(t, "/path/to/grpc-cert.pem", config.GRPCTLSConfig.TLSCertPath)
				require.Equal(t, "/path/to/grpc-key.pem", config.GRPCTLSConfig.TLSKeyPath)
				require.Equal(t, "RequestClientCert", config.GRPCTLSConfig.ClientAuth)
				require.Equal(t, "/path/to/grpc-ca.pem", config.GRPCTLSConfig.ClientCAs)
				require.Equal(t, "-----BEGIN CERTIFICATE-----\ngRPC CA cert\n-----END CERTIFICATE-----", config.GRPCTLSConfig.ClientCAsText)
			},
		},
		"both http and grpc with tls config": {
			raw: `
			http {
				listen_port = 8443
				tls {
					cert_file = "/path/to/http-cert.pem"
					key_file = "/path/to/http-key.pem"
					client_auth_type = "NoClientCert"
				}
			}
			grpc {
				listen_port = 9443
				tls {
					cert_file = "/path/to/grpc-cert.pem"
					key_file = "/path/to/grpc-key.pem"
					client_auth_type = "RequireAndVerifyClientCert"
					client_ca_file = "/path/to/grpc-ca.pem"
				}
			}`,
			assert: func(t *testing.T, config dskit.Config) {
				// HTTP TLS
				require.Equal(t, 8443, config.HTTPListenPort)
				require.Equal(t, "/path/to/http-cert.pem", config.HTTPTLSConfig.TLSCertPath)
				require.Equal(t, "/path/to/http-key.pem", config.HTTPTLSConfig.TLSKeyPath)
				require.Equal(t, "NoClientCert", config.HTTPTLSConfig.ClientAuth)

				// gRPC TLS
				require.Equal(t, 9443, config.GRPCListenPort)
				require.Equal(t, "/path/to/grpc-cert.pem", config.GRPCTLSConfig.TLSCertPath)
				require.Equal(t, "/path/to/grpc-key.pem", config.GRPCTLSConfig.TLSKeyPath)
				require.Equal(t, "RequireAndVerifyClientCert", config.GRPCTLSConfig.ClientAuth)
				require.Equal(t, "/path/to/grpc-ca.pem", config.GRPCTLSConfig.ClientCAs)
			},
		},
		"tls config with minimal settings": {
			raw: `
			http {
				tls {
					cert_file = "/minimal/cert.pem"
					key_file = "/minimal/key.pem"
				}
			}`,
			assert: func(t *testing.T, config dskit.Config) {
				require.Equal(t, "/minimal/cert.pem", config.HTTPTLSConfig.TLSCertPath)
				require.Equal(t, "/minimal/key.pem", config.HTTPTLSConfig.TLSKeyPath)
				// Other fields should be empty/default
				require.Equal(t, "", config.HTTPTLSConfig.TLSCert)
				require.Equal(t, "", config.HTTPTLSConfig.ClientAuth)
				require.Equal(t, "", config.HTTPTLSConfig.ClientCAs)
				require.Equal(t, "", config.HTTPTLSConfig.ClientCAsText)
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			args := testArguments{}
			err := syntax.Unmarshal([]byte(tc.raw), &args)
			require.Equal(t, tc.errExpected, err != nil)
			wConfig := args.Server.convert()
			tc.assert(t, wConfig)
		})
	}
}
