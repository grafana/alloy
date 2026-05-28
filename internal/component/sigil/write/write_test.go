package write

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/sigil"
	"github.com/grafana/alloy/internal/runtime/logging"
	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
	"github.com/grafana/sigil-sdk/go/proto/sigil/wire"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func testGenerationsRequest() *sigil.GenerationsRequest {
	return &sigil.GenerationsRequest{
		Request: &sigilv1.ExportGenerationsRequest{
			Generations: []*sigilv1.Generation{
				{Id: "g1", AgentName: "a"},
			},
		},
	}
}

func TestEndpointClient(t *testing.T) {
	tests := []struct {
		name              string
		tenantID          string
		reqOrgID          string
		serverStatus      int
		serverBody        string
		serverContentType string
		headers           map[string]string
		expectOrgID       string
		expectCustom      string
		expectContentType string
		expectStatus      int
		expectErr         bool
		expectErrMaxLen   int
	}{
		{
			name:              "forwards headers and JSON body",
			reqOrgID:          "tenant-1",
			headers:           map[string]string{"X-Custom": "hello"},
			serverStatus:      http.StatusAccepted,
			serverBody:        `{"results":[{"generation_id":"g1","accepted":true}]}`,
			serverContentType: "application/json",
			expectOrgID:       "tenant-1",
			expectCustom:      "hello",
			expectContentType: "application/json",
			expectStatus:      http.StatusAccepted,
		},
		{
			name:              "tenant_id config overrides request org ID",
			tenantID:          "config-tenant",
			reqOrgID:          "request-tenant",
			serverStatus:      http.StatusAccepted,
			expectOrgID:       "config-tenant",
			expectContentType: "application/json",
			expectStatus:      http.StatusAccepted,
		},
		{
			name:              "tenant_id config overrides endpoint header",
			tenantID:          "config-tenant",
			reqOrgID:          "request-tenant",
			headers:           map[string]string{wire.TenantHeaderName: "header-tenant"},
			serverStatus:      http.StatusAccepted,
			expectOrgID:       "config-tenant",
			expectContentType: "application/json",
			expectStatus:      http.StatusAccepted,
		},
		{
			name:              "endpoint tenant header overrides request org ID",
			reqOrgID:          "request-tenant",
			headers:           map[string]string{wire.TenantHeaderName: "header-tenant"},
			serverStatus:      http.StatusAccepted,
			expectOrgID:       "header-tenant",
			expectContentType: "application/json",
			expectStatus:      http.StatusAccepted,
		},
		{
			name:         "no retry on 4xx",
			serverStatus: http.StatusBadRequest,
			serverBody:   "bad request",
			expectErr:    true,
		},
		{
			name:         "redirect response is not success",
			serverStatus: http.StatusMultipleChoices,
			expectErr:    true,
		},
		{
			name:              "endpoint header cannot override Content-Type",
			headers:           map[string]string{"Content-Type": "text/plain"},
			serverStatus:      http.StatusAccepted,
			expectContentType: "application/json",
			expectStatus:      http.StatusAccepted,
		},
		{
			name:            "large error body is truncated",
			serverStatus:    http.StatusInternalServerError,
			serverBody:      strings.Repeat("x", 4096),
			expectErr:       true,
			expectErrMaxLen: 2300,
		},
		{
			name:         "oversized success response is rejected",
			serverStatus: http.StatusAccepted,
			serverBody:   strings.Repeat("a", maxResponseBodyOverhead+65536),
			expectErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				gotOrgID       string
				gotCustom      string
				gotContentType string
				gotPath        string
				gotBody        []byte
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotOrgID = r.Header.Get(wire.TenantHeaderName)
				gotCustom = r.Header.Get("X-Custom")
				gotContentType = r.Header.Get("Content-Type")
				gotPath = r.URL.Path
				gotBody, _ = io.ReadAll(r.Body)
				if tc.serverContentType != "" {
					w.Header().Set("Content-Type", tc.serverContentType)
				}
				w.WriteHeader(tc.serverStatus)
				if tc.serverBody != "" {
					_, _ = w.Write([]byte(tc.serverBody))
				}
			}))
			defer srv.Close()

			opts := GetDefaultEndpointOptions()
			opts.URL = srv.URL
			opts.TenantID = tc.tenantID
			opts.Headers = tc.headers
			opts.MinBackoff = 1 * time.Millisecond
			opts.MaxBackoff = 10 * time.Millisecond
			opts.MaxBackoffRetries = 1

			ec, err := newEndpointClient(logging.NewSlogNop(), &opts, newMetrics(prometheus.NewRegistry()))
			require.NoError(t, err)

			req := testGenerationsRequest()
			req.OrgID = tc.reqOrgID

			resp, err := ec.send(context.Background(), req)
			if tc.expectErr {
				require.Error(t, err)
				if tc.expectErrMaxLen > 0 {
					require.LessOrEqual(t, len(err.Error()), tc.expectErrMaxLen,
						"error message should be truncated")
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectStatus, resp.StatusCode)

			require.Equal(t, wire.GenerationExportHTTPPath, gotPath)
			if tc.expectOrgID != "" {
				require.Equal(t, tc.expectOrgID, gotOrgID)
			}
			if tc.expectCustom != "" {
				require.Equal(t, tc.expectCustom, gotCustom)
			}
			require.Equal(t, tc.expectContentType, gotContentType)

			// Verify the body is the marshaled request.
			expectBody, err := sigil.MarshalGenerationsRequest(req)
			require.NoError(t, err)
			require.Equal(t, expectBody, gotBody)

			// Round-trip the server body back into a parsed response.
			if tc.serverBody != "" {
				require.NotNil(t, resp.Response)
				require.Len(t, resp.Response.Results, 1)
			}
		})
	}
}

func TestEndpointClient_NormalizesEndpointURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		insecure   bool
		wantScheme string
		wantPath   string
	}{
		{name: "https with no path", url: "https://sigil.grafana.net", wantScheme: "https", wantPath: wire.GenerationExportHTTPPath},
		{name: "host only defaults to https", url: "sigil.grafana.net", wantScheme: "https", wantPath: wire.GenerationExportHTTPPath},
		{name: "host only insecure uses http", url: "sigil.local:4317", insecure: true, wantScheme: "http", wantPath: wire.GenerationExportHTTPPath},
		{name: "explicit path is preserved", url: "https://sigil.example.com/custom/path", wantScheme: "https", wantPath: "/custom/path"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := GetDefaultEndpointOptions()
			opts.URL = tc.url
			opts.Insecure = tc.insecure
			ec, err := newEndpointClient(logging.NewSlogNop(), &opts, newMetrics(prometheus.NewRegistry()))
			require.NoError(t, err)

			parsed, err := url.Parse(ec.endpoint)
			require.NoError(t, err)
			require.Equal(t, tc.wantScheme, parsed.Scheme)
			require.Equal(t, tc.wantPath, parsed.Path)
		})
	}
}

func TestEndpointClient_AppendsGenerationExportPath(t *testing.T) {
	// When configured with only a base URL, the writer should POST to
	// /api/v1/generations:export on that host.
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	opts := GetDefaultEndpointOptions()
	opts.URL = srv.URL
	opts.MinBackoff = 1 * time.Millisecond
	opts.MaxBackoff = 10 * time.Millisecond

	ec, err := newEndpointClient(logging.NewSlogNop(), &opts, newMetrics(prometheus.NewRegistry()))
	require.NoError(t, err)

	_, err = ec.send(context.Background(), testGenerationsRequest())
	require.NoError(t, err)
	require.Equal(t, wire.GenerationExportHTTPPath, gotPath)
}

func TestEndpointClient_ParsesResponse(t *testing.T) {
	expectResp := &sigilv1.ExportGenerationsResponse{
		Results: []*sigilv1.ExportGenerationResult{
			{GenerationId: "g1", Accepted: true},
		},
	}
	body, err := wire.MarshalExportGenerationsResponseJSON(expectResp)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	opts := GetDefaultEndpointOptions()
	opts.URL = srv.URL
	opts.MinBackoff = 1 * time.Millisecond
	opts.MaxBackoff = 10 * time.Millisecond

	ec, err := newEndpointClient(logging.NewSlogNop(), &opts, newMetrics(prometheus.NewRegistry()))
	require.NoError(t, err)

	resp, err := ec.send(context.Background(), testGenerationsRequest())
	require.NoError(t, err)
	require.NotNil(t, resp.Response)
	require.True(t, proto.Equal(expectResp, resp.Response))
}

func TestEndpointClient_RetriesOn5xx(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	opts := GetDefaultEndpointOptions()
	opts.URL = srv.URL
	opts.MinBackoff = 1 * time.Millisecond
	opts.MaxBackoff = 10 * time.Millisecond
	opts.MaxBackoffRetries = 5

	ec, err := newEndpointClient(logging.NewSlogNop(), &opts, newMetrics(prometheus.NewRegistry()))
	require.NoError(t, err)

	resp, err := ec.send(context.Background(), testGenerationsRequest())
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.Equal(t, int32(3), attempts.Load())
}

func TestEndpointClient_RetriesOn429And408(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "429 Too Many Requests", status: http.StatusTooManyRequests},
		{name: "408 Request Timeout", status: http.StatusRequestTimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var attempts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if attempts.Add(1) <= 1 {
					w.WriteHeader(tc.status)
					return
				}
				w.WriteHeader(http.StatusAccepted)
			}))
			defer srv.Close()

			opts := GetDefaultEndpointOptions()
			opts.URL = srv.URL
			opts.MinBackoff = 1 * time.Millisecond
			opts.MaxBackoff = 10 * time.Millisecond
			opts.MaxBackoffRetries = 5

			ec, err := newEndpointClient(logging.NewSlogNop(), &opts, newMetrics(prometheus.NewRegistry()))
			require.NoError(t, err)
			resp, err := ec.send(context.Background(), testGenerationsRequest())
			require.NoError(t, err)
			require.Equal(t, http.StatusAccepted, resp.StatusCode)
			require.Equal(t, int32(2), attempts.Load())
		})
	}
}

func TestEndpointClient_RetriesRemoteTimeout(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			time.Sleep(50 * time.Millisecond)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	opts := GetDefaultEndpointOptions()
	opts.URL = srv.URL
	opts.RemoteTimeout = 10 * time.Millisecond
	opts.MinBackoff = 1 * time.Millisecond
	opts.MaxBackoff = 1 * time.Millisecond
	opts.MaxBackoffRetries = 2

	ec, err := newEndpointClient(logging.NewSlogNop(), &opts, newMetrics(prometheus.NewRegistry()))
	require.NoError(t, err)

	resp, err := ec.send(context.Background(), testGenerationsRequest())
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.Equal(t, int32(2), attempts.Load())
}

func TestFanOutClient(t *testing.T) {
	tests := []struct {
		name         string
		statuses     []int
		expectStatus int
		expectErr    bool
	}{
		{
			name:         "sends to multiple endpoints",
			statuses:     []int{http.StatusAccepted, http.StatusAccepted},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "partial success returns success",
			statuses:     []int{http.StatusAccepted, http.StatusBadRequest},
			expectStatus: http.StatusAccepted,
		},
		{
			name:      "all fail returns error",
			statuses:  []int{http.StatusBadRequest, http.StatusBadRequest},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			counts := make([]*atomic.Int32, len(tc.statuses))
			endpoints := make([]*EndpointOptions, 0, len(tc.statuses))

			for i, status := range tc.statuses {
				count := &atomic.Int32{}
				counts[i] = count

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					count.Add(1)
					w.WriteHeader(status)
				}))
				t.Cleanup(srv.Close)

				ep := GetDefaultEndpointOptions()
				ep.URL = srv.URL
				endpoints = append(endpoints, &ep)
			}

			fc, err := newFanOutClient(logging.NewSlogNop(), Arguments{
				Endpoints: endpoints,
			}, newMetrics(prometheus.NewRegistry()))
			require.NoError(t, err)

			resp, err := fc.ExportGenerations(context.Background(), testGenerationsRequest())
			if tc.expectErr {
				require.Error(t, err)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectStatus, resp.StatusCode)
			}

			for _, count := range counts {
				require.Equal(t, int32(1), count.Load())
			}
		})
	}
}

func TestFanOutClient_ClonesPerEndpoint(t *testing.T) {
	// Each endpoint should receive a request the writer can mutate independently
	// of sibling branches.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	ep1 := GetDefaultEndpointOptions()
	ep1.URL = srv.URL
	ep2 := GetDefaultEndpointOptions()
	ep2.URL = srv.URL

	fc, err := newFanOutClient(logging.NewSlogNop(), Arguments{
		Endpoints: []*EndpointOptions{&ep1, &ep2},
	}, newMetrics(prometheus.NewRegistry()))
	require.NoError(t, err)

	req := testGenerationsRequest()
	_, err = fc.ExportGenerations(context.Background(), req)
	require.NoError(t, err)
	// The shared input request is still valid and unmutated.
	require.Equal(t, "g1", req.Request.Generations[0].Id)
}

func TestShouldRetry(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	deadlineCtx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	<-deadlineCtx.Done()

	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{name: "context canceled", ctx: context.Background(), err: context.Canceled, want: false},
		{name: "caller canceled", ctx: canceledCtx, err: &WriteError{StatusCode: 500}, want: false},
		{name: "caller deadline exceeded", ctx: deadlineCtx, err: context.DeadlineExceeded, want: false},
		{name: "remote timeout", ctx: context.Background(), err: context.DeadlineExceeded, want: true},
		{name: "wrapped context canceled", ctx: context.Background(), err: fmt.Errorf("wrap: %w", context.Canceled), want: false},
		{name: "3xx WriteError not retried", ctx: context.Background(), err: &WriteError{StatusCode: 300}, want: false},
		{name: "4xx WriteError not retried", ctx: context.Background(), err: &WriteError{StatusCode: 400}, want: false},
		{name: "401 WriteError not retried", ctx: context.Background(), err: &WriteError{StatusCode: 401}, want: false},
		{name: "408 WriteError retried", ctx: context.Background(), err: &WriteError{StatusCode: http.StatusRequestTimeout}, want: true},
		{name: "429 WriteError retried", ctx: context.Background(), err: &WriteError{StatusCode: http.StatusTooManyRequests}, want: true},
		{name: "500 WriteError retried", ctx: context.Background(), err: &WriteError{StatusCode: 500}, want: true},
		{name: "502 WriteError retried", ctx: context.Background(), err: &WriteError{StatusCode: 502}, want: true},
		{name: "plain error not retried", ctx: context.Background(), err: fmt.Errorf("decoding json response: bad input"), want: false},
		{name: "network error retried", ctx: context.Background(), err: &fakeNetErr{}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldRetry(tc.ctx, tc.err))
		})
	}
}

type fakeNetErr struct{}

func (fakeNetErr) Error() string   { return "fake net error" }
func (fakeNetErr) Timeout() bool   { return false }
func (fakeNetErr) Temporary() bool { return true }

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr bool
	}{
		{
			name:    "no endpoints",
			args:    Arguments{},
			wantErr: true,
		},
		{
			name: "with endpoint",
			args: func() Arguments {
				ep := GetDefaultEndpointOptions()
				ep.URL = "http://localhost:4320"
				return Arguments{Endpoints: []*EndpointOptions{&ep}}
			}(),
			wantErr: false,
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

func TestEndpointOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*EndpointOptions)
		wantErr bool
	}{
		{
			name:    "valid defaults",
			modify:  func(e *EndpointOptions) { e.URL = "http://localhost:4320" },
			wantErr: false,
		},
		{
			name:    "valid host only",
			modify:  func(e *EndpointOptions) { e.URL = "sigil.grafana.net" },
			wantErr: false,
		},
		{
			name:    "empty URL",
			modify:  func(e *EndpointOptions) { e.URL = "" },
			wantErr: true,
		},
		{
			name:    "invalid URL",
			modify:  func(e *EndpointOptions) { e.URL = "http://" },
			wantErr: true,
		},
		{
			name: "min backoff exceeds max",
			modify: func(e *EndpointOptions) {
				e.URL = "http://localhost"
				e.MinBackoff = 10 * time.Second
				e.MaxBackoff = 1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "negative retries",
			modify: func(e *EndpointOptions) {
				e.URL = "http://localhost"
				e.MaxBackoffRetries = -1
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := GetDefaultEndpointOptions()
			tc.modify(&opts)
			err := opts.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
