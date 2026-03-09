//go:build alloyintegrationtests

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/docker/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pyroscopeURL    = "http://localhost:4040"
	javaServiceName = "java-fast-slow-fibonacci"
)

type labelValuesRequest struct {
	Name  string `json:"name"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
}

type labelValuesResponse struct {
	Names []string `json:"names"`
}

// TestPyroscopeJava is the top-level test that runs all pyroscope.java subtests
// in a guaranteed sequential order. Capabilities are checked last so that
// Tetragon has observed every syscall made during the full test run.
func TestPyroscopeJava(t *testing.T) {
	t.Run("Profiles", testProfiles)
	t.Run("Capabilities", func(t *testing.T) {
		// discovery.process scans /proc and requires CAP_SYS_PTRACE to read
		// other processes' memory maps. This must always be observed so that
		// any refactor removing the capability usage is caught immediately.
		// TODO: Document this capability requirement in the discovery.process docs.
		required := []common.ExpectedCapabilityEvent{
			{
				Capability:    "CAP_SYS_PTRACE",
				StackContains: []string{"github.com/grafana/alloy/internal/component/discovery/process.discover"},
			},
		}
		common.AssertTetragonCapabilities(t, "alloy", required)
	})
}

// testProfiles verifies that the pyroscope.java component successfully profiles
// the FastSlow Java application and sends profiles to Pyroscope.
// It polls the Pyroscope QuerierService/LabelValues API until the
// java-fast-slow-fibonacci service appears, confirming that at least one
// profile chunk has been ingested.
func testProfiles(t *testing.T) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		now := time.Now()
		reqBody := labelValuesRequest{
			Name:  "service_name",
			Start: now.Add(-1 * time.Hour).UnixMilli(),
			End:   now.UnixMilli(),
		}
		reqBytes, err := json.Marshal(reqBody)
		if !assert.NoError(c, err, "failed to marshal request body") {
			return
		}

		resp, err := http.Post(
			fmt.Sprintf("%s/querier.v1.QuerierService/LabelValues", pyroscopeURL),
			"application/json",
			bytes.NewReader(reqBytes),
		)
		if !assert.NoError(c, err, "failed to query Pyroscope LabelValues API") {
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if !assert.NoError(c, err, "failed to read Pyroscope response body") {
			return
		}

		if !assert.Equalf(c, http.StatusOK, resp.StatusCode,
			"Pyroscope returned non-OK status: %s, body: %s", resp.Status, string(body)) {
			return
		}

		var labelResp labelValuesResponse
		if !assert.NoError(c, json.Unmarshal(body, &labelResp), "failed to decode Pyroscope response") {
			return
		}

		assert.Containsf(c, labelResp.Names, javaServiceName,
			"service %q not yet visible in Pyroscope; current services: %v", javaServiceName, labelResp.Names)
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval,
		"profiles for service %q did not appear in Pyroscope within the timeout", javaServiceName)
}
