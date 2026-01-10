//go:build !race && !windows

package node_exporter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/prometheus/prometheus/model/labels"
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

	p := textparse.NewPromParser(body, nil, false)
	foundBuildInfo := false
	for {
		et, err := p.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		// Check for node_exporter_build_info metric
		if et == textparse.EntrySeries {
			series, _, _ := p.Series()
			var lbls labels.Labels
			p.Labels(&lbls)
			metricName := lbls.Get("__name__")
			if metricName == "node_exporter_build_info" {
				foundBuildInfo = true
				// Verify the version label contains the correct node_exporter version
				version := lbls.Get("version")
				require.Equal(t, nodeExporterVersion, version, "node_exporter_build_info should have correct version, series: %s", string(series))
			}
		}
	}
	require.True(t, foundBuildInfo, "node_exporter_build_info metric should be present")
}
