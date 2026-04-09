package receiver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/atomic"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/sigil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

type mockReceiver struct {
	calls    atomic.Int32
	lastReq  *sigil.GenerationsRequest
	response *sigil.GenerationsResponse
	err      error
}

func (m *mockReceiver) ExportGenerations(_ context.Context, req *sigil.GenerationsRequest) (*sigil.GenerationsResponse, error) {
	m.calls.Add(1)
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}
	return &sigil.GenerationsResponse{
		StatusCode: http.StatusAccepted,
		Body:       []byte(`{"results":[]}`),
	}, nil
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           string
		contentType    string
		orgID          string
		auth           string
		receivers      int
		maxBodySize    int64
		expectStatus   int
		expectBody     string
		expectForwards int
	}{
		{
			name:           "forwards POST to receiver",
			method:         http.MethodPost,
			body:           `{"generations":[{"id":"gen-1"}]}`,
			contentType:    "application/json",
			orgID:          "tenant-1",
			receivers:      1,
			expectStatus:   http.StatusAccepted,
			expectBody:     `{"results":[]}`,
			expectForwards: 1,
		},
		{
			name:         "rejects non-POST",
			method:       http.MethodGet,
			receivers:    0,
			expectStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "rejects body exceeding max size",
			method:         http.MethodPost,
			body:           "x]x]x]x]x]x]", // 13 bytes, limit set to 10
			contentType:    "application/json",
			receivers:      1,
			maxBodySize:    10,
			expectStatus:   http.StatusRequestEntityTooLarge,
			expectForwards: 0,
		},
		{
			name:           "does not forward Authorization to receivers",
			method:         http.MethodPost,
			body:           `{"generations":[]}`,
			contentType:    "application/json",
			auth:           "Bearer secret-token",
			receivers:      1,
			expectStatus:   http.StatusAccepted,
			expectForwards: 1,
		},
		{
			name:           "fans out to multiple receivers",
			method:         http.MethodPost,
			body:           `{"generations":[]}`,
			contentType:    "application/json",
			receivers:      2,
			expectStatus:   http.StatusAccepted,
			expectForwards: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mocks := make([]*mockReceiver, tc.receivers)
			receivers := make([]sigil.GenerationsReceiver, tc.receivers)
			for i := range tc.receivers {
				mocks[i] = &mockReceiver{}
				receivers[i] = mocks[i]
			}

			maxBody := int64(50 * 1024 * 1024)
			if tc.maxBodySize > 0 {
				maxBody = tc.maxBodySize
			}
			m := newMetrics(prometheus.NewRegistry())
			h := newHandler(log.NewNopLogger(), m, receivers, maxBody)

			var bodyReader *bytes.Reader
			if tc.body != "" {
				bodyReader = bytes.NewReader([]byte(tc.body))
			} else {
				bodyReader = bytes.NewReader(nil)
			}

			req := httptest.NewRequest(tc.method, "/api/v1/generations:export", bodyReader)
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			if tc.orgID != "" {
				req.Header.Set("X-Scope-OrgID", tc.orgID)
			}
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			require.Equal(t, tc.expectStatus, rr.Code)
			if tc.expectBody != "" {
				require.Equal(t, tc.expectBody, rr.Body.String())
			}

			totalForwards := 0
			for _, mock := range mocks {
				totalForwards += int(mock.calls.Load())
			}
			require.Equal(t, tc.expectForwards, totalForwards)

			if tc.expectForwards > 0 && tc.receivers > 0 {
				require.NotNil(t, mocks[0].lastReq)
				require.Equal(t, []byte(tc.body), mocks[0].lastReq.Body)
				if tc.contentType != "" {
					require.Equal(t, tc.contentType, mocks[0].lastReq.ContentType)
				}
				if tc.orgID != "" {
					require.Equal(t, tc.orgID, mocks[0].lastReq.OrgID)
				}
				require.Nil(t, mocks[0].lastReq.Headers)
			}
		})
	}
}

func TestHandler_PartialSuccess(t *testing.T) {
	tests := []struct {
		name         string
		failFirst    bool
		failSecond   bool
		expectStatus int
	}{
		{
			name:         "one fails, one succeeds — returns success",
			failFirst:    true,
			failSecond:   false,
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "both succeed",
			failFirst:    false,
			failSecond:   false,
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "all fail — returns 502",
			failFirst:    true,
			failSecond:   true,
			expectStatus: http.StatusBadGateway,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock1 := &mockReceiver{}
			mock2 := &mockReceiver{}
			if tc.failFirst {
				mock1.err = fmt.Errorf("downstream error")
			}
			if tc.failSecond {
				mock2.err = fmt.Errorf("downstream error")
			}

			m := newMetrics(prometheus.NewRegistry())
			h := newHandler(log.NewNopLogger(), m, []sigil.GenerationsReceiver{mock1, mock2}, 50*1024*1024)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/generations:export", bytes.NewReader([]byte(`{}`)))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			require.Equal(t, tc.expectStatus, rr.Code)
		})
	}
}
