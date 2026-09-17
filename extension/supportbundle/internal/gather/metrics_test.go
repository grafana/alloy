package gather

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestMetricsEndpoint(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		require.Empty(t, metricsEndpoint(nil))
	})

	t.Run("no telemetry metrics", func(t *testing.T) {
		require.Empty(t, metricsEndpoint(confmap.NewFromStringMap(map[string]any{})))
	})

	t.Run("legacy address", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"service": map[string]any{"telemetry": map[string]any{"metrics": map[string]any{"address": "localhost:8888"}}},
		})
		require.Equal(t, "localhost:8888", metricsEndpoint(conf))
	})

	t.Run("ipv6 host is not double-bracketed", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"service": map[string]any{"telemetry": map[string]any{"metrics": map[string]any{
				"readers": []any{
					map[string]any{"pull": map[string]any{"exporter": map[string]any{"prometheus": map[string]any{
						"host": "[::1]", "port": 9464,
					}}}},
				},
			}}},
		})
		require.Equal(t, "[::1]:9464", metricsEndpoint(conf))
	})

	t.Run("pull prometheus reader", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"service": map[string]any{"telemetry": map[string]any{"metrics": map[string]any{
				"readers": []any{
					map[string]any{"pull": map[string]any{"exporter": map[string]any{"prometheus": map[string]any{
						"host": "127.0.0.1", "port": 9464,
					}}}},
				},
			}}},
		})
		require.Equal(t, "127.0.0.1:9464", metricsEndpoint(conf))
	})
}

// metricsServer starts a test /metrics endpoint and returns a gatherer for it.
func metricsServer(t *testing.T) *Metrics {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			_, _ = io.WriteString(w, "otelcol_process_uptime 42\n")
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	addr := strings.TrimPrefix(srv.URL, "http://")
	conf := confmap.NewFromStringMap(map[string]any{
		"service": map[string]any{"telemetry": map[string]any{"metrics": map[string]any{"address": addr}}},
	})
	return &Metrics{Conf: func() *confmap.Conf { return conf }, Client: srv.Client()}
}

// runMetrics runs the async gatherer start-to-finish.
func runMetrics(t *testing.T, g *Metrics, opts Options) ([]File, error) {
	t.Helper()
	finish, err := g.Start(context.Background(), opts)
	require.NoError(t, err)
	if finish == nil {
		return nil, nil
	}
	return finish(context.Background())
}

func TestMetricsGathererSingleSample(t *testing.T) {
	// A zero window yields one sample in metrics.txt.
	files, err := runMetrics(t, metricsServer(t), Options{Duration: 0})
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "metrics.txt")
	require.Contains(t, string(m["metrics.txt"]), "otelcol_process_uptime 42")
}

func TestMetricsGathererStartEndSamples(t *testing.T) {
	// A non-zero window yields a start and an end sample.
	files, err := runMetrics(t, metricsServer(t), Options{Duration: 50 * time.Millisecond})
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "metrics-start.txt")
	require.Contains(t, m, "metrics-end.txt")
	require.NotContains(t, m, "metrics.txt")
}

func TestMetricsGathererSkipsUnresolvedEndpoint(t *testing.T) {
	// An unexpanded config reference must not be scraped as a literal address.
	conf := confmap.NewFromStringMap(map[string]any{
		"service": map[string]any{"telemetry": map[string]any{"metrics": map[string]any{"address": "${env:METRICS_ADDR}"}}},
	})
	g := &Metrics{Conf: func() *confmap.Conf { return conf }, Client: http.DefaultClient}
	finish, err := g.Start(context.Background(), Options{})
	require.NoError(t, err)
	require.Nil(t, finish)
}

func TestMetricsGathererNoEndpoint(t *testing.T) {
	g := &Metrics{Conf: func() *confmap.Conf { return nil }, Client: http.DefaultClient}
	finish, err := g.Start(context.Background(), Options{})
	require.NoError(t, err)
	require.Nil(t, finish)
}

func TestMetricsGathererUnreachableEndpointErrors(t *testing.T) {
	// An address that is configured but not reachable is reported as an error.
	conf := confmap.NewFromStringMap(map[string]any{
		"service": map[string]any{"telemetry": map[string]any{"metrics": map[string]any{"address": "127.0.0.1:1"}}},
	})
	g := &Metrics{Conf: func() *confmap.Conf { return conf }, Client: &http.Client{}}
	_, err := runMetrics(t, g, Options{Duration: 0})
	require.Error(t, err)
}
