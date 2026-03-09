//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

// TestBeyla is the top-level test that runs all Beyla subtests in a guaranteed sequential order. 
// Capabilities must be checked last so that we are sure all capabilities used during testing have been recorded.
func TestBeyla(t *testing.T) {
	t.Run("Metrics", func(t *testing.T) {
		var beylaMetrics = []string{
			"beyla_internal_build_info",                // check that internal Beyla metrics are reported
			"http_server_request_duration_seconds_sum", // check that the target metrics are reported
		}
		common.MimirMetricsTest(t, beylaMetrics, []string{}, "beyla")
	})

	t.Run("Traces", func(t *testing.T) {
		// Test that traces are being generated and sent to Tempo
		tags := map[string]string{
			"service.name": "main", // This should match the instrumented app
		}
		common.TracesTest(t, tags, "beyla")
	})

	t.Run("Capabilities", func(t *testing.T) {
		common.AssertTetragonCapabilities(t, "alloy", nil)
	})
}
