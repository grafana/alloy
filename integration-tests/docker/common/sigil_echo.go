package common

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// EchoRequest is a single request captured by the sigil integration test echo
// server (see integration-tests/docker/tests/sigil/echo-server). Each test
// reads only the fields relevant to its scenario; unused fields stay zero.
type EchoRequest struct {
	Body        string            `json:"body"`
	ContentType string            `json:"content_type"`
	OrgID       string            `json:"org_id"`
	Auth        string            `json:"auth"`
	Headers     map[string]string `json:"headers"`
}

// WaitForReady polls url until it responds with a status below 500 or timeout
// elapses.
func WaitForReady(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode < 500
	}, timeout, 500*time.Millisecond, "waiting for %s", url)
}

// ResetEcho clears the requests recorded by the echo server at base.
func ResetEcho(t *testing.T, base string) {
	t.Helper()
	resp, err := http.Post(base+"/reset", "", nil)
	require.NoError(t, err)
	resp.Body.Close()
}

// FetchEchoRequests returns the requests recorded by the echo server at base.
// It returns nil if the server can't be reached so callers can poll with
// require.Eventually.
func FetchEchoRequests(t *testing.T, base string) []EchoRequest {
	t.Helper()
	r, err := http.Get(base + "/requests")
	if err != nil {
		return nil
	}
	defer r.Body.Close()
	var reqs []EchoRequest
	_ = json.NewDecoder(r.Body).Decode(&reqs)
	return reqs
}
