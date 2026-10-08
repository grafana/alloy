package harness

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
)

func TestMockHTTP(t *testing.T) {
	var exports MockHTTPExports
	m, err := NewMockHTTP(component.Options{
		OnStateChange: func(e component.Exports) { exports = e.(MockHTTPExports) },
	}, MockHTTPArguments{Routes: []MockHTTPRoute{
		{Path: "/any", Body: "any method"},
		{Path: "/any", Method: "post", Status: http.StatusCreated, Body: "post only"},
		{Path: "/headers", Headers: map[string]string{"Content-Type": "application/json"}, Body: "{}"},
		{Path: "/fail", Status: http.StatusServiceUnavailable},
	}})
	require.NoError(t, err)
	t.Cleanup(m.server.Close)
	require.Equal(t, m.server.URL, exports.URL)

	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantBody    string
		wantHeaders map[string]string
	}{
		{name: "status defaults to 200", method: http.MethodGet, path: "/any", wantStatus: http.StatusOK, wantBody: "any method"},
		{name: "method route wins, case-insensitive", method: http.MethodPost, path: "/any", wantStatus: http.StatusCreated, wantBody: "post only"},
		{name: "query string is ignored", method: http.MethodGet, path: "/any?x=1", wantStatus: http.StatusOK, wantBody: "any method"},
		{name: "headers are set", method: http.MethodGet, path: "/headers", wantStatus: http.StatusOK, wantBody: "{}", wantHeaders: map[string]string{"Content-Type": "application/json"}},
		{name: "status without body", method: http.MethodGet, path: "/fail", wantStatus: http.StatusServiceUnavailable},
		{name: "unmatched path is 404", method: http.MethodGet, path: "/missing", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), tc.method, exports.URL+tc.path, nil)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.Equal(t, tc.wantBody, string(body))
			for k, v := range tc.wantHeaders {
				require.Equal(t, v, resp.Header.Get(k))
			}
		})
	}
}

func TestMockHTTPUpdate(t *testing.T) {
	var exports MockHTTPExports
	m, err := NewMockHTTP(component.Options{
		OnStateChange: func(e component.Exports) { exports = e.(MockHTTPExports) },
	}, MockHTTPArguments{Routes: []MockHTTPRoute{{Path: "/old"}}})
	require.NoError(t, err)
	t.Cleanup(m.server.Close)

	require.NoError(t, m.Update(MockHTTPArguments{Routes: []MockHTTPRoute{{Path: "/new"}}}))

	for path, want := range map[string]int{"/old": http.StatusNotFound, "/new": http.StatusOK} {
		resp, err := http.Get(exports.URL + path)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, want, resp.StatusCode, path)
	}
}

func TestMockHTTPValidate(t *testing.T) {
	tests := []struct {
		name    string
		routes  []MockHTTPRoute
		wantErr string
	}{
		{name: "valid", routes: []MockHTTPRoute{{Path: "/a"}, {Path: "/a", Method: "GET"}}},
		{name: "relative path", routes: []MockHTTPRoute{{Path: "a"}}, wantErr: "must start with /"},
		{name: "invalid status", routes: []MockHTTPRoute{{Path: "/a", Status: 42}}, wantErr: "not a valid HTTP status code"},
		{name: "duplicate route", routes: []MockHTTPRoute{{Path: "/a", Method: "get"}, {Path: "/a", Method: "GET"}}, wantErr: "duplicate route"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := MockHTTPArguments{Routes: tc.routes}
			err := args.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
