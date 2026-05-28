//go:build alloyintegrationtests

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

type exportResult struct {
	GenerationID string `json:"generation_id"`
	Accepted     bool   `json:"accepted"`
}

type exportResponse struct {
	Results []exportResult `json:"results"`
}

const (
	receiveURL  = "http://localhost:1236/api/v1/generations:export"
	echoBaseURL = "http://localhost:8894"
)

// TestSigilConfigWiring checks that endpoint configuration on sigil.write is
// honored end-to-end: tenant_id overrides the inbound tenant header, custom
// headers reach the upstream, and the upstream status code and response body
// are propagated back through sigil.receive to the caller.
func TestSigilConfigWiring(t *testing.T) {
	common.WaitForReady(t, "http://localhost:12343/-/ready", 30*time.Second)
	common.WaitForReady(t, echoBaseURL+"/requests", 10*time.Second)

	common.ResetEcho(t, echoBaseURL)

	body := `{"generations":[{"id":"gen-1"}]}`
	req, err := http.NewRequest(http.MethodPost, receiveURL, bytes.NewReader([]byte(body)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scope-OrgID", "caller-tenant")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// The upstream returns 200 with a populated result list; both must round
	// trip back to the caller.
	require.Equal(t, http.StatusOK, resp.StatusCode)
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var decoded exportResponse
	require.NoError(t, json.Unmarshal(respBody, &decoded))
	require.Len(t, decoded.Results, 1)
	require.Equal(t, "gen-1", decoded.Results[0].GenerationID)
	require.True(t, decoded.Results[0].Accepted)

	var reqs []common.EchoRequest
	require.Eventually(t, func() bool {
		reqs = common.FetchEchoRequests(t, echoBaseURL)
		return len(reqs) == 1
	}, 5*time.Second, 100*time.Millisecond)

	require.Len(t, reqs, 1)
	require.Equal(t, "override-tenant", reqs[0].OrgID, "tenant_id should override the inbound X-Scope-OrgID")
	require.Equal(t, "header-value", reqs[0].Headers["X-Sigil-Test"], "configured endpoint headers should reach the upstream")
}
