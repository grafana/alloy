package write

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	alloyconfig "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/pyroscope/write/promhttp2"
	commonconfig "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func TestSmokeTransport(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("http1", func(t *testing.T) {
		srv := httptest.NewServer(handler)
		defer srv.Close()

		client, err := promhttp2.NewClientFromConfig(commonconfig.HTTPClientConfig{
			FollowRedirects: true,
			EnableHTTP2:     false,
		}, "test-http1")
		require.NoError(t, err)

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("http1_tls", func(t *testing.T) {
		srv := httptest.NewTLSServer(handler)
		defer srv.Close()

		client, err := promhttp2.NewClientFromConfig(commonconfig.HTTPClientConfig{
			FollowRedirects: true,
			EnableHTTP2:     false,
			TLSConfig:       commonconfig.TLSConfig{InsecureSkipVerify: true},
		}, "test-http1-tls")
		require.NoError(t, err)

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("http2_tls", func(t *testing.T) {
		srv := httptest.NewUnstartedServer(handler)
		require.NoError(t, http2.ConfigureServer(srv.Config, &http2.Server{}))
		srv.StartTLS()
		defer srv.Close()

		httpCfg := alloyconfig.CloneDefaultHTTPClientConfig()
		httpCfg.EnableHTTP2 = true
		httpCfg.TLSConfig.InsecureSkipVerify = true

		ec := &endpointClient{
			options: &EndpointOptions{
				URL:              srv.URL,
				Name:             "test-http2-tls",
				HTTPClientConfig: httpCfg,
			},
		}

		client, err := ec.http2Client()
		require.NoError(t, err)

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("h2c", func(t *testing.T) {
		srv := httptest.NewServer(h2c.NewHandler(handler, &http2.Server{}))
		defer srv.Close()

		ec := &endpointClient{
			options: &EndpointOptions{
				URL:              srv.URL,
				Name:             "test-h2c",
				HTTPClientConfig: alloyconfig.CloneDefaultHTTPClientConfig(),
			},
		}

		client, err := ec.http2Client()
		require.NoError(t, err)

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
