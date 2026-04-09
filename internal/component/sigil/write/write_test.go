package write

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/sigil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestEndpointClient(t *testing.T) {
	tests := []struct {
		name              string
		tenantID          string
		reqOrgID          string
		contentType       string
		serverStatus      int
		serverBody        string
		headers           map[string]string
		expectOrgID       string
		expectCustom      string
		expectContentType string
		expectStatus      int
		expectErr         bool
		expectErrMaxLen   int
	}{
		{
			name:         "forwards headers and body",
			contentType:  "application/json",
			reqOrgID:     "tenant-1",
			headers:      map[string]string{"X-Custom": "hello"},
			serverStatus: http.StatusAccepted,
			serverBody:   `{"results":[]}`,
			expectOrgID:  "tenant-1",
			expectCustom: "hello",
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "tenant_id config overrides request org ID",
			contentType:  "application/json",
			tenantID:     "config-tenant",
			reqOrgID:     "request-tenant",
			serverStatus: http.StatusAccepted,
			expectOrgID:  "config-tenant",
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "no retry on 4xx",
			contentType:  "application/json",
			serverStatus: http.StatusBadRequest,
			serverBody:   "bad request",
			expectErr:    true,
		},
		{
			name:              "request content-type overwrites endpoint header",
			contentType:       "application/json",
			headers:           map[string]string{"Content-Type": "text/plain"},
			serverStatus:      http.StatusAccepted,
			expectContentType: "application/json",
			expectStatus:      http.StatusAccepted,
		},
		{
			name:              "empty content-type preserves endpoint header",
			contentType:       "",
			headers:           map[string]string{"Content-Type": "text/plain"},
			serverStatus:      http.StatusAccepted,
			expectContentType: "text/plain",
			expectStatus:      http.StatusAccepted,
		},
		{
			name:            "large error body is truncated",
			contentType:     "application/json",
			serverStatus:    http.StatusInternalServerError,
			serverBody:      strings.Repeat("x", 4096),
			expectErr:       true,
			expectErrMaxLen: 2300,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				gotOrgID       string
				gotCustom      string
				gotContentType string
				gotBody        []byte
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotOrgID = r.Header.Get("X-Scope-OrgID")
				gotCustom = r.Header.Get("X-Custom")
				gotContentType = r.Header.Get("Content-Type")
				gotBody, _ = io.ReadAll(r.Body)
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

			ec, err := newEndpointClient(log.NewNopLogger(), &opts, newMetrics(prometheus.NewRegistry()))
			require.NoError(t, err)

			req := &sigil.GenerationsRequest{
				Body:        []byte(`{"generations":[]}`),
				ContentType: tc.contentType,
				OrgID:       tc.reqOrgID,
			}

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
			if tc.serverBody != "" {
				require.Equal(t, tc.serverBody, string(resp.Body))
			}
			if tc.expectOrgID != "" {
				require.Equal(t, tc.expectOrgID, gotOrgID)
			}
			if tc.expectCustom != "" {
				require.Equal(t, tc.expectCustom, gotCustom)
			}
			if tc.expectContentType != "" {
				require.Equal(t, tc.expectContentType, gotContentType)
			}
			require.Equal(t, `{"generations":[]}`, string(gotBody))
		})
	}
}

func TestEndpointClient_RetriesOn5xx(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	opts := GetDefaultEndpointOptions()
	opts.URL = srv.URL
	opts.MinBackoff = 1 * time.Millisecond
	opts.MaxBackoff = 10 * time.Millisecond
	opts.MaxBackoffRetries = 5

	ec, err := newEndpointClient(log.NewNopLogger(), &opts, newMetrics(prometheus.NewRegistry()))
	require.NoError(t, err)

	resp, err := ec.send(context.Background(), &sigil.GenerationsRequest{
		Body:        []byte(`{}`),
		ContentType: "application/json",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.Equal(t, int32(3), attempts.Load())
}

func TestFanOutClient_SendsToMultipleEndpoints(t *testing.T) {
	var count1, count2 atomic.Int32

	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count1.Add(1)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count2.Add(1)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv2.Close()

	ep1 := GetDefaultEndpointOptions()
	ep1.URL = srv1.URL
	ep2 := GetDefaultEndpointOptions()
	ep2.URL = srv2.URL

	fc, err := newFanOutClient(log.NewNopLogger(), Arguments{
		Endpoints: []*EndpointOptions{&ep1, &ep2},
	}, newMetrics(prometheus.NewRegistry()))
	require.NoError(t, err)

	resp, err := fc.ExportGenerations(context.Background(), &sigil.GenerationsRequest{
		Body:        []byte(`{"generations":[]}`),
		ContentType: "application/json",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	require.Equal(t, int32(1), count1.Load())
	require.Equal(t, int32(1), count2.Load())
}

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
			name:    "empty URL",
			modify:  func(e *EndpointOptions) { e.URL = "" },
			wantErr: true,
		},
		{
			name:    "invalid URL",
			modify:  func(e *EndpointOptions) { e.URL = "://bad" },
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
