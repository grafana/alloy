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
	pyroscopeURL     = "http://localhost:4040"
	ebpfServiceName = "java-fast-slow-ebpf"
)

type labelValuesRequest struct {
	Name  string `json:"name"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
}

type labelValuesResponse struct {
	Names []string `json:"names"`
}

// TestPyroscopeEbpf runs profile verification first, then checks Linux
// capabilities via Tetragon so the full syscall surface (including eBPF load
// and discovery.process /proc scans) is reflected in the capability report.
func TestPyroscopeEbpf(t *testing.T) {
	t.Run("Profiles", testProfiles)
	t.Run("Capabilities", func(t *testing.T) {
		// discovery.process scans /proc and checks CAP_SYS_PTRACE (same as pyroscope.java).
		required := []common.ExpectedCapabilityEvent{
			{
				Capability:    "CAP_SYS_PTRACE",
				StackContains: []string{"github.com/grafana/alloy/internal/component/discovery/process.discover"},
			},
			// eBPF profiling uses BPF maps/programs and perf events; the kernel
			// consults these capabilities (names are stable in Tetragon's enum).
			{Capability: "CAP_BPF", StackContains: nil},
			{Capability: "CAP_PERFMON", StackContains: nil},
		}
		common.AssertTetragonCapabilities(t, "alloy", required)
	})
}

// testProfiles checks that pyroscope.ebpf profiles the FastSlow Java workload
// and that Pyroscope ingests at least one chunk (service_name label present).
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

		assert.Containsf(c, labelResp.Names, ebpfServiceName,
			"service %q not yet visible in Pyroscope; current services: %v", ebpfServiceName, labelResp.Names)
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval,
		"profiles for service %q did not appear in Pyroscope within the timeout", ebpfServiceName)
}
