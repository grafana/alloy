//go:build alloyintegrationtests

package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const (
	receiveURL = "http://localhost:1235/api/v1/generations:export"
	echoA      = "http://localhost:8891"
	echoB      = "http://localhost:8892"
	echoC      = "http://localhost:8893"
)

// TestSigilFanout verifies that a single inbound request fans out to every
// configured branch: sigil.receive forwards to two sigil.write components, and
// one of those writers forwards to two endpoints. All three upstreams must see
// the same payload, proving each branch gets an independent copy rather than
// sharing (and draining) one request body.
func TestSigilFanout(t *testing.T) {
	common.WaitForReady(t, "http://localhost:12342/-/ready", 30*time.Second)
	for _, echo := range []string{echoA, echoB, echoC} {
		common.WaitForReady(t, echo+"/requests", 10*time.Second)
		common.ResetEcho(t, echo)
	}

	body := `{"generations":[{"id":"gen-1"},{"id":"gen-2"}]}`

	req, err := http.NewRequest(http.MethodPost, receiveURL, bytes.NewReader([]byte(body)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scope-OrgID", "tenant-1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	for _, echo := range []string{echoA, echoB, echoC} {
		var reqs []common.EchoRequest
		require.Eventually(t, func() bool {
			reqs = common.FetchEchoRequests(t, echo)
			return len(reqs) == 1
		}, 5*time.Second, 100*time.Millisecond, "expected exactly one request at %s", echo)

		require.Len(t, reqs, 1)
		require.JSONEq(t, body, reqs[0].Body, "payload at %s must match the inbound body", echo)
		require.Equal(t, "tenant-1", reqs[0].OrgID, "tenant header should pass through to %s", echo)
	}
}
