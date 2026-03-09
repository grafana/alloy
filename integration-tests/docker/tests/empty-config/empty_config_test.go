//go:build alloyintegrationtests

package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/docker/common"
	"github.com/stretchr/testify/require"
)

const alloyMetricsURL = "http://localhost:12345/metrics"

// TestCapabilities asserts that Alloy with an empty configuration requests no
// Linux capabilities at all. The test is expected to fail on first run — use
// the output to understand which capabilities are requested and why, then
// decide whether each one is necessary.
func TestCapabilities(t *testing.T) {
	containerID := os.Getenv(common.TetragonContainerIDEnv)
	if containerID == "" {
		t.Skip("Tetragon container not configured (tetragon_image not set in test.yaml)")
	}

	// Wait for Alloy to be fully started before collecting capability events.
	// Polling the /metrics endpoint ensures the HTTP server is up and Alloy
	// has completed its initialisation sequence.
	t.Log("waiting for Alloy metrics endpoint to become ready...")
	require.Eventually(t, func() bool {
		resp, err := http.Get(alloyMetricsURL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		// TODO: Also check for an actual metric
		return resp.StatusCode == http.StatusOK
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval,
		"Alloy metrics endpoint at %s did not become ready within the timeout", alloyMetricsURL)
	t.Log("Alloy is ready")

	t.Log("waiting 30s for any late capability syscalls to be recorded by Tetragon...")
	time.Sleep(30 * time.Second)

	// With an empty config Alloy requires no feature-specific capabilities.
	common.AssertTetragonCapabilities(t, "alloy", nil)
}
