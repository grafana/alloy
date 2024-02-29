//go:build !race && !windows

package node_exporter //nolint:golint

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/stretchr/testify/require"
)

// TestNodeExporter runs an integration test for node_exporter, doing the
// following:
//
// 1. Enabling all collectors (minus some that cause issues in cross-platform testing)
// 2. Creating the integration
// 3. Scrape the integration once
// 4. Parse the result of the scrape
//
// This ensures that the flag parsing is correct and that the handler is
// set up properly. We do not test the contents of the scrape, just that it
// was parsable by Prometheus.
func TestNodeExporter(t *testing.T) {
	cfg := DefaultConfig

	// Enable all collectors except perf
	cfg.SetCollectors = make([]string, 0, len(Collectors))
	for c := range Collectors {
		cfg.SetCollectors = append(cfg.SetCollectors, c)
	}
	cfg.DisableCollectors = []string{CollectorPerf, CollectorBuddyInfo}

	// Check that the flags convert and the integration initializes
	logger := log.NewNopLogger()
	integration, err := New(logger, &cfg)
	require.NoError(t, err, "failed to setup node_exporter")

	r := mux.NewRouter()
	handler, err := integration.MetricsHandler()
	require.NoError(t, err)
	r.Handle("/metrics", handler)

	// Invoke /metrics and parse the response
	srv := httptest.NewServer(r)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/metrics")
	require.NoError(t, err)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	p := textparse.NewPromParser(body)
	for {
		_, err := p.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}
}
