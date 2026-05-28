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

func TestSigilPipeline(t *testing.T) {
	common.WaitForReady(t, "http://localhost:12341/-/ready", 30*time.Second)
	common.WaitForReady(t, "http://localhost:8888/requests", 10*time.Second)

	tests := []struct {
		name         string
		body         string
		contentType  string
		orgID        string
		auth         string
		expectStatus int
		expectOrgID  string
		expectCT     string
		expectNoAuth bool
	}{
		{
			name:         "generation payload flows end-to-end",
			body:         `{"generations":[{"id":"gen-1"}]}`,
			contentType:  "application/json",
			orgID:        "tenant-1",
			expectStatus: http.StatusAccepted,
			expectOrgID:  "tenant-1",
			expectCT:     "application/json",
		},
		{
			name:         "Authorization not forwarded to upstream",
			body:         `{"generations":[{"id":"gen-2"}]}`,
			contentType:  "application/json",
			auth:         "Bearer caller-secret",
			expectStatus: http.StatusAccepted,
			expectNoAuth: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset echo server state.
			common.ResetEcho(t, "http://localhost:8888")

			// Send request to sigil.receiver.
			req, err := http.NewRequest(http.MethodPost, "http://localhost:1234/api/v1/generations:export", bytes.NewReader([]byte(tc.body)))
			require.NoError(t, err)
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			if tc.orgID != "" {
				req.Header.Set("X-Scope-OrgID", tc.orgID)
			}
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, tc.expectStatus, resp.StatusCode)

			// Verify what the upstream echo server received.
			var reqs []common.EchoRequest
			require.Eventually(t, func() bool {
				reqs = common.FetchEchoRequests(t, "http://localhost:8888")
				return len(reqs) == 1
			}, 5*time.Second, 100*time.Millisecond)

			require.Len(t, reqs, 1)
			require.JSONEq(t, tc.body, reqs[0].Body)
			if tc.expectOrgID != "" {
				require.Equal(t, tc.expectOrgID, reqs[0].OrgID)
			}
			if tc.expectCT != "" {
				require.Equal(t, tc.expectCT, reqs[0].ContentType)
			}
			if tc.expectNoAuth {
				require.Empty(t, reqs[0].Auth, "Authorization should not reach upstream")
			}
		})
	}
}
