package receiver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/sigil"
	"github.com/grafana/alloy/internal/runtime/logging"
	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
	"github.com/grafana/sigil-sdk/go/proto/sigil/wire"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

type mockReceiver struct {
	calls    atomic.Int32
	mu       atomicReq
	response *sigil.GenerationsResponse
	err      error
}

type atomicReq struct {
	last *sigil.GenerationsRequest
}

func (m *mockReceiver) ExportGenerations(_ context.Context, req *sigil.GenerationsRequest) (*sigil.GenerationsResponse, error) {
	m.calls.Add(1)
	m.mu.last = req
	if m.response != nil || m.err != nil {
		return m.response, m.err
	}
	return &sigil.GenerationsResponse{
		StatusCode: http.StatusAccepted,
		Response:   &sigilv1.ExportGenerationsResponse{},
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
		expectForwards int
		expectGenIDs   []string
	}{
		{
			name:           "forwards POST to receiver",
			method:         http.MethodPost,
			body:           `{"generations":[{"id":"gen-1"}]}`,
			contentType:    "application/json",
			orgID:          "tenant-1",
			receivers:      1,
			expectStatus:   http.StatusAccepted,
			expectForwards: 1,
			expectGenIDs:   []string{"gen-1"},
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
			body:           `{"generations":[{"id":"xxxxxxxxxxxxxxxxx"}]}`,
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
		{
			name:           "accepts content-type with charset parameter",
			method:         http.MethodPost,
			body:           `{"generations":[{"id":"gen-1"}]}`,
			contentType:    "application/json; charset=utf-8",
			receivers:      1,
			expectStatus:   http.StatusAccepted,
			expectForwards: 1,
			expectGenIDs:   []string{"gen-1"},
		},
		{
			name:           "rejects malformed JSON",
			method:         http.MethodPost,
			body:           `{not json`,
			contentType:    "application/json",
			receivers:      1,
			expectStatus:   http.StatusBadRequest,
			expectForwards: 0,
		},
		{
			name:           "rejects unsupported content type",
			method:         http.MethodPost,
			body:           `something`,
			contentType:    "text/plain",
			receivers:      1,
			expectStatus:   http.StatusUnsupportedMediaType,
			expectForwards: 0,
		},
		{
			name:           "defaults to JSON when content-type is empty",
			method:         http.MethodPost,
			body:           `{"generations":[{"id":"g1"}]}`,
			contentType:    "",
			receivers:      1,
			expectStatus:   http.StatusAccepted,
			expectForwards: 1,
			expectGenIDs:   []string{"g1"},
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
			h := newHandler(logging.NewSlogNop(), m, receivers, maxBody)

			var bodyReader *bytes.Reader
			if tc.body != "" {
				bodyReader = bytes.NewReader([]byte(tc.body))
			} else {
				bodyReader = bytes.NewReader(nil)
			}

			req := httptest.NewRequest(tc.method, wire.GenerationExportHTTPPath, bodyReader)
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			if tc.orgID != "" {
				req.Header.Set(wire.TenantHeaderName, tc.orgID)
			}
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			require.Equal(t, tc.expectStatus, rr.Code)

			totalForwards := 0
			for _, mock := range mocks {
				totalForwards += int(mock.calls.Load())
			}
			require.Equal(t, tc.expectForwards, totalForwards)

			if tc.expectForwards > 0 && tc.receivers > 0 {
				require.NotNil(t, mocks[0].mu.last)
				require.NotNil(t, mocks[0].mu.last.Request)
				if tc.orgID != "" {
					require.Equal(t, tc.orgID, mocks[0].mu.last.OrgID)
				}
				if tc.expectGenIDs != nil {
					gotIDs := make([]string, 0, len(mocks[0].mu.last.Request.Generations))
					for _, g := range mocks[0].mu.last.Request.Generations {
						gotIDs = append(gotIDs, g.Id)
					}
					require.Equal(t, tc.expectGenIDs, gotIDs)
				}
			}
		})
	}
}

func TestArguments_Validate(t *testing.T) {
	validArgs := func() Arguments {
		args := Arguments{}
		args.SetToDefault()
		args.ForwardTo = []sigil.GenerationsReceiver{&mockReceiver{}}
		return args
	}

	tests := []struct {
		name    string
		args    Arguments
		wantErr bool
	}{
		{
			name: "valid",
			args: validArgs(),
		},
		{
			name: "no receivers",
			args: func() Arguments {
				args := validArgs()
				args.ForwardTo = nil
				return args
			}(),
			wantErr: true,
		},
		{
			name: "nil receiver",
			args: func() Arguments {
				args := validArgs()
				args.ForwardTo = []sigil.GenerationsReceiver{nil}
				return args
			}(),
			wantErr: true,
		},
		{
			name: "invalid max request body size",
			args: func() Arguments {
				args := validArgs()
				args.MaxRequestBodySize = 0
				return args
			}(),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.args.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestComponent_UpdateRestartsOnServerConfigChange(t *testing.T) {
	args := Arguments{}
	args.SetToDefault()
	args.Server.HTTP.ListenAddress = "127.0.0.1"
	args.Server.HTTP.ListenPort = 0
	args.Server.GRPC.ListenAddress = "127.0.0.1"
	args.Server.GRPC.ListenPort = 0
	args.ForwardTo = []sigil.GenerationsReceiver{&mockReceiver{}}

	comp, err := New(component.Options{
		SLogger:    logging.NewSlogNop(),
		Registerer: prometheus.NewRegistry(),
	}, args)
	require.NoError(t, err)
	defer comp.shutdownServer()

	initialServer := comp.server
	require.NotNil(t, initialServer)

	updated := args
	updated.Server = fnet.DefaultServerConfig()
	updated.Server.HTTP.ListenAddress = "127.0.0.1"
	updated.Server.HTTP.ListenPort = 0
	updated.Server.GRPC.ListenAddress = "127.0.0.1"
	updated.Server.GRPC.ListenPort = 0
	updated.Server.GracefulShutdownTimeout = args.Server.GracefulShutdownTimeout + time.Second

	require.NoError(t, comp.Update(updated))
	require.NotNil(t, comp.server)
	require.NotSame(t, initialServer, comp.server)
}

func TestHandler_FanoutClonesRequest(t *testing.T) {
	// Each downstream receiver must observe an independently mutable request.
	var (
		mocks     = []*mockReceiver{{}, {}}
		receivers = []sigil.GenerationsReceiver{mocks[0], mocks[1]}
	)
	m := newMetrics(prometheus.NewRegistry())
	h := newHandler(logging.NewSlogNop(), m, receivers, 50*1024*1024)

	body := `{"generations":[{"id":"g1","tags":{"env":"prod"}}]}`
	req := httptest.NewRequest(http.MethodPost, wire.GenerationExportHTTPPath, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Code)

	require.NotSame(t, mocks[0].mu.last.Request, mocks[1].mu.last.Request)
	require.NotSame(t, mocks[0].mu.last.Request.Generations[0], mocks[1].mu.last.Request.Generations[0])

	// Mutate the first receiver's copy and confirm the second remains intact.
	mocks[0].mu.last.Request.Generations[0].Tags["env"] = "staging"
	require.Equal(t, "prod", mocks[1].mu.last.Request.Generations[0].Tags["env"])
}

func TestHandler_EncodesJSONResponse(t *testing.T) {
	mock := &mockReceiver{
		response: &sigil.GenerationsResponse{
			StatusCode: http.StatusAccepted,
			Response: &sigilv1.ExportGenerationsResponse{
				Results: []*sigilv1.ExportGenerationResult{
					{GenerationId: "g1", Accepted: true},
				},
			},
		},
	}
	m := newMetrics(prometheus.NewRegistry())
	h := newHandler(logging.NewSlogNop(), m, []sigil.GenerationsReceiver{mock}, 50*1024*1024)

	req := httptest.NewRequest(http.MethodPost, wire.GenerationExportHTTPPath, bytes.NewReader([]byte(`{"generations":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	parsed, err := sigil.ParseGenerationsResponse(rr.Body.Bytes())
	require.NoError(t, err)
	require.Len(t, parsed.Results, 1)
	require.Equal(t, "g1", parsed.Results[0].GenerationId)
	require.True(t, parsed.Results[0].Accepted)
}

func TestHandler_DefaultsInvalidResponseStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "zero status", statusCode: 0},
		{name: "too low", statusCode: 99},
		{name: "too high", statusCode: 1000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockReceiver{
				response: &sigil.GenerationsResponse{
					StatusCode: tc.statusCode,
					Response:   &sigilv1.ExportGenerationsResponse{},
				},
			}

			m := newMetrics(prometheus.NewRegistry())
			h := newHandler(logging.NewSlogNop(), m, []sigil.GenerationsReceiver{mock}, 50*1024*1024)

			req := httptest.NewRequest(http.MethodPost, wire.GenerationExportHTTPPath, bytes.NewReader([]byte(`{}`)))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			require.NotPanics(t, func() {
				h.ServeHTTP(rr, req)
			})

			require.Equal(t, http.StatusAccepted, rr.Code)
		})
	}
}

func TestHandler_PartialSuccess(t *testing.T) {
	tests := []struct {
		name         string
		receivers    []*mockReceiver
		expectStatus int
	}{
		{
			name: "one fails, one succeeds — returns success",
			receivers: []*mockReceiver{
				{err: fmt.Errorf("downstream error")},
				{},
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name: "both succeed",
			receivers: []*mockReceiver{
				{},
				{},
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name: "all fail — returns 502",
			receivers: []*mockReceiver{
				{err: fmt.Errorf("downstream error")},
				{err: fmt.Errorf("downstream error")},
			},
			expectStatus: http.StatusBadGateway,
		},
		{
			// A branch that returns both response and error is treated as a
			// failure — the error takes precedence so that sibling fanouts
			// stay consistent and a broken branch is never silently treated
			// as a success.
			name: "branch returning response and error counts as failure",
			receivers: []*mockReceiver{
				{
					response: &sigil.GenerationsResponse{
						StatusCode: http.StatusAccepted,
						Response:   &sigilv1.ExportGenerationsResponse{},
					},
					err: fmt.Errorf("downstream error"),
				},
			},
			expectStatus: http.StatusBadGateway,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			receivers := make([]sigil.GenerationsReceiver, len(tc.receivers))
			for i, receiver := range tc.receivers {
				receivers[i] = receiver
			}

			m := newMetrics(prometheus.NewRegistry())
			h := newHandler(logging.NewSlogNop(), m, receivers, 50*1024*1024)

			req := httptest.NewRequest(http.MethodPost, wire.GenerationExportHTTPPath, bytes.NewReader([]byte(`{}`)))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			require.Equal(t, tc.expectStatus, rr.Code)
		})
	}
}
