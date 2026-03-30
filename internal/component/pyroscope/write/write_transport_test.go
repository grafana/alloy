package write

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	alloyconfig "github.com/grafana/alloy/internal/component/common/config"
	pyrotestlogger "github.com/grafana/alloy/internal/component/pyroscope/util/testlog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func TestTransport(t *testing.T) {
	protoHandler := func(proto string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, proto, r.Proto)
			w.WriteHeader(http.StatusOK)
		})
	}

	newTestComponent := func(url string, httpCfg *alloyconfig.HTTPClientConfig) *Component {
		opts := GetDefaultEndpointOptions()
		opts.URL = url
		opts.HTTPClientConfig = httpCfg
		args := DefaultArguments()
		args.Endpoints = []*EndpointOptions{&opts}
		c, err := New(
			pyrotestlogger.TestLogger(t),
			noop.Tracer{},
			prometheus.NewRegistry(),
			func(Exports) {},
			"test-agent", "", t.TempDir(),
			args,
		)
		require.NoError(t, err)
		return c
	}

	t.Run("http1", func(t *testing.T) {
		srv := httptest.NewUnstartedServer(protoHandler("HTTP/1.1"))
		srv.Start()
		defer srv.Close()

		cfg := alloyconfig.CloneDefaultHTTPClientConfig()
		cfg.EnableHTTP2 = false
		c := newTestComponent(srv.URL, cfg)
		client := c.receiver.endpoints[0].ingestClient

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
		c := newTestComponent(srv.URL, cfg)
		client := c.receiver.endpoints[0].ingestClient

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("http2_tls", func(t *testing.T) {
		srv := httptest.NewUnstartedServer(protoHandler("HTTP/2.0"))
		srv.EnableHTTP2 = true
		srv.StartTLS()
		defer srv.Close()

		cfg := alloyconfig.CloneDefaultHTTPClientConfig()
		cfg.EnableHTTP2 = true
		cfg.TLSConfig.InsecureSkipVerify = true
		c := newTestComponent(srv.URL, cfg)
		client := c.receiver.endpoints[0].http2Client()

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("h2c", func(t *testing.T) {
		srv := httptest.NewUnstartedServer(h2c.NewHandler(protoHandler("HTTP/2.0"), &http2.Server{}))
		srv.Start()
		defer srv.Close()

		c := newTestComponent(srv.URL, alloyconfig.CloneDefaultHTTPClientConfig())
		client := c.receiver.endpoints[0].http2Client()

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
