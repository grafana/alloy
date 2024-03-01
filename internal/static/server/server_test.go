package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const anyLocalhost = "127.0.0.1:0"

func TestServer(t *testing.T) {
	cfg := newTestConfig()
	flags := newTestFlags()
	srv := runExampleServer(t, cfg, flags)

	// Validate HTTP
	resp, err := http.Get(fmt.Sprintf("http://%s/testing", srv.HTTPAddress()))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	// Validate gRPC
	creds := grpc.WithTransportCredentials(insecure.NewCredentials())
	cc, err := grpc.Dial(srv.GRPCAddress().String(), creds)
	require.NoError(t, err)
	_, err = grpc_health_v1.NewHealthClient(cc).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
}

func TestServer_InMemory(t *testing.T) {
	cfg := newTestConfig()
	flags := newTestFlags()
	srv := runExampleServer(t, cfg, flags)

	// Validate HTTP
	var httpClient http.Client
	httpClient.Transport = &http.Transport{DialContext: srv.DialContext}
	resp, err := httpClient.Get(fmt.Sprintf("http://%s/testing", flags.HTTP.InMemoryAddr))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	// Validate gRPC
	grpcDialer := grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return srv.DialContext(ctx, "", s)
	})
	cc, err := grpc.Dial(flags.GRPC.InMemoryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpcDialer)
	require.NoError(t, err)
	_, err = grpc_health_v1.NewHealthClient(cc).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
}

func newTestConfig() Config {
	cfg := DefaultConfig()
	return cfg
}

func newTestFlags() Flags {
	flags := DefaultFlags
	flags.HTTP.ListenAddress = anyLocalhost
	flags.GRPC.ListenAddress = anyLocalhost
	return flags
}

func runExampleServer(t *testing.T, cfg Config, flags Flags) *Server {
	t.Helper()

	srv, err := New(log.NewNopLogger(), nil, nil, cfg, flags)
	require.NoError(t, err)

	// Set up some expected services for us to test against.
	srv.HTTP.HandleFunc("/testing", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	grpc_health_v1.RegisterHealthServer(srv.GRPC, health.NewServer())

	// Run our server.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		require.NoError(t, srv.Run(ctx))
	}()

	return srv
}

func TestServer_TLS(t *testing.T) {
	cfg := newTestConfig()
	flags := newTestFlags()

	flags.HTTP.UseTLS = true
	flags.GRPC.UseTLS = true

	tlsConfig := TLSConfig{
		TLSCertPath: "testdata/example-cert.pem",
		TLSKeyPath:  "testdata/example-key.pem",
	}
	cfg.HTTP.TLSConfig = tlsConfig
	cfg.GRPC.TLSConfig = tlsConfig

	srv := runExampleServer(t, cfg, flags)

	// Validate HTTPS
	cli := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := cli.Get(fmt.Sprintf("https://%s/testing", srv.HTTPAddress()))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	// Validate gRPC TLS
	creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	cc, err := grpc.Dial(srv.GRPCAddress().String(), grpc.WithTransportCredentials(creds))
	require.NoError(t, err)
	_, err = grpc_health_v1.NewHealthClient(cc).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
}

// TestRunReturnsError validates that Run exits with an error when the
// HTTP/GRPC servers stop unexpectedly.
func TestRunReturnsError(t *testing.T) {
	cfg := newTestConfig()
	flags := newTestFlags()

	t.Run("http", func(t *testing.T) {
		srv, err := New(nil, nil, nil, cfg, flags)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errChan := make(chan error, 1)
		go func() {
			errChan <- srv.Run(ctx)
		}()

		require.NoError(t, srv.httpListener.Close())
		require.NotNil(t, <-errChan)
	})

	t.Run("grpc", func(t *testing.T) {
		srv, err := New(nil, nil, nil, cfg, flags)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errChan := make(chan error, 1)
		go func() {
			errChan <- srv.Run(ctx)
		}()

		require.NoError(t, srv.grpcListener.Close())
		require.NotNil(t, <-errChan)
	})
}

func TestServer_ApplyConfig(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		cfg := newTestConfig()
		flags := newTestFlags()

		srv, err := New(nil, nil, nil, cfg, flags)
		require.NoError(t, err)

		require.NoError(t, srv.ApplyConfig(cfg))
	})

	t.Run("valid changes", func(t *testing.T) {
		cfg := newTestConfig()
		flags := newTestFlags()

		srv, err := New(nil, nil, nil, cfg, flags)
		require.NoError(t, err)

		cfg.LogLevel.Set("debug")
		require.NoError(t, srv.ApplyConfig(cfg))
	})
}
