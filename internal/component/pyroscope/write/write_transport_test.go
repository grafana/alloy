package write

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	alloyconfig "github.com/grafana/alloy/internal/component/common/config"
	pyrotestlogger "github.com/grafana/alloy/internal/component/pyroscope/util/testlog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestTransport(t *testing.T) {
	protoHandler := func(proto string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, proto, r.Proto)
			w.WriteHeader(http.StatusOK)
		})
	}

	newTestReceiver := func(url string, httpCfg *alloyconfig.HTTPClientConfig) *fanOutClient {
		opts := GetDefaultEndpointOptions()
		opts.URL = url
		opts.HTTPClientConfig = httpCfg
		args := DefaultArguments()
		args.Endpoints = []*EndpointOptions{&opts}
		var receiver *fanOutClient
		_, err := New(
			pyrotestlogger.TestLogger(t),
			noop.Tracer{},
			prometheus.NewRegistry(),
			func(exports Exports) {
				receiver = exports.Receiver.(*fanOutClient)
			},
			"test-agent", "",
			args,
		)
		require.NoError(t, err)
		return receiver
	}

	t.Run("http1", func(t *testing.T) {
		srv := httptest.NewUnstartedServer(protoHandler("HTTP/1.1"))
		srv.Start()
		defer srv.Close()

		cfg := alloyconfig.CloneDefaultHTTPClientConfig()
		cfg.EnableHTTP2 = false
		receiver := newTestReceiver(srv.URL, cfg)
		client := receiver.endpoints[0].ingestClient

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("http1_tls", func(t *testing.T) {
		srv := httptest.NewUnstartedServer(protoHandler("HTTP/1.1"))
		srv.StartTLS()
		defer srv.Close()

		cfg := alloyconfig.CloneDefaultHTTPClientConfig()
		cfg.EnableHTTP2 = false
		cfg.TLSConfig.InsecureSkipVerify = true
		receiver := newTestReceiver(srv.URL, cfg)
		client := receiver.endpoints[0].ingestClient

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestUnixSocketTransport(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "pyro-test.sock")

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ingest", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})}
	lis, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer lis.Close()
	go func() { _ = srv.Serve(lis) }()
	defer srv.Close()

	opts := GetDefaultEndpointOptions()
	opts.URL = "http://127.0.0.1:4040/ingest"
	opts.UnixSocketPath = sockPath
	args := DefaultArguments()
	args.Endpoints = []*EndpointOptions{&opts}
	var receiver *fanOutClient
	_, err = New(
		pyrotestlogger.TestLogger(t),
		noop.Tracer{},
		prometheus.NewRegistry(),
		func(exports Exports) {
			receiver = exports.Receiver.(*fanOutClient)
		},
		"test-agent", "",
		args,
	)
	require.NoError(t, err)

	// The URL host is never dialed; the transport must reach the stub over unix.
	resp, err := receiver.endpoints[0].ingestClient.Get("http://127.0.0.1:4040/ingest")
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
