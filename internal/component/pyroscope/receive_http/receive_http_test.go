package receive_http

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/util"
)

// TestForwardsProfiles verifies the behavior of the pyroscope.receive_http component
// under various scenarios. It tests different profile sizes, HTTP methods, paths,
// query parameters, and error conditions to ensure correct forwarding behavior
// and proper error handling.
func TestForwardsProfiles(t *testing.T) {
	tests := []struct {
		name             string
		profileSize      int
		method           string
		path             string
		queryParams      string
		headers          map[string]string
		appendableErrors []error
		expectedStatus   int
		expectedForwards int
	}{
		{
			name:             "Small profile",
			profileSize:      1024, // 1KB
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test_app_1&from=1234567890&until=1234567900",
			headers:          map[string]string{"Content-Type": "application/octet-stream"},
			appendableErrors: []error{nil, nil},
			expectedStatus:   http.StatusOK,
			expectedForwards: 2,
		},
		{
			name:             "Large profile with custom headers",
			profileSize:      1024 * 1024, // 1MB
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test_app_2&from=1234567891&until=1234567901&custom=param1",
			headers:          map[string]string{"X-Scope-OrgID": "1234"},
			appendableErrors: []error{nil},
			expectedStatus:   http.StatusOK,
			expectedForwards: 1,
		},
		{
			name:             "Invalid method",
			profileSize:      1024,
			method:           "GET",
			path:             "/ingest",
			queryParams:      "name=test_app_3&from=1234567892&until=1234567902",
			headers:          map[string]string{},
			appendableErrors: []error{nil, nil},
			expectedStatus:   http.StatusMethodNotAllowed,
			expectedForwards: 0,
		},
		{
			name:             "Invalid query params",
			profileSize:      1024,
			method:           "GET",
			path:             "/ingest",
			queryParams:      "test=test_app",
			headers:          map[string]string{},
			appendableErrors: []error{nil, nil},
			expectedStatus:   http.StatusMethodNotAllowed,
			expectedForwards: 0,
		},
		{
			name:             "Invalid path",
			profileSize:      1024,
			method:           "POST",
			path:             "/invalid",
			queryParams:      "name=test_app_4&from=1234567893&until=1234567903",
			headers:          map[string]string{"Content-Type": "application/octet-stream"},
			appendableErrors: []error{nil, nil},
			expectedStatus:   http.StatusNotFound,
			expectedForwards: 0,
		},
		{
			name:             "All appendables fail",
			profileSize:      2048,
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test_app_5&from=1234567894&until=1234567904&scenario=all_fail",
			headers:          map[string]string{"Content-Type": "application/octet-stream", "X-Test": "fail-all"},
			appendableErrors: []error{fmt.Errorf("error1"), fmt.Errorf("error2")},
			expectedStatus:   http.StatusInternalServerError,
			expectedForwards: 2,
		},
		{
			name:             "One appendable fails, one succeeds",
			profileSize:      4096,
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test_app_6&from=1234567895&until=1234567905&scenario=partial_failure",
			headers:          map[string]string{"X-Custom-ID": "test-6"},
			appendableErrors: []error{fmt.Errorf("error"), nil},
			expectedStatus:   http.StatusInternalServerError,
			expectedForwards: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appendables := createTestAppendables(tt.appendableErrors)
			port := startComponent(t, appendables)

			testProfile, resp := sendCustomRequest(t, port, tt.method, tt.path, tt.queryParams, tt.headers, tt.profileSize)
			require.Equal(t, tt.expectedStatus, resp.StatusCode)

			forwardedCount := countForwardedProfiles(appendables)
			require.Equal(t, tt.expectedForwards, forwardedCount, "Unexpected number of forwards")

			if tt.expectedForwards > 0 {
				verifyForwardedProfiles(t, appendables, testProfile, tt.headers, tt.queryParams)
			}
		})
	}
}

func createTestAppendables(errors []error) []pyroscope.Appendable {
	var appendables []pyroscope.Appendable
	for _, err := range errors {
		appendables = append(appendables, testAppendable(err))
	}
	return appendables
}

func countForwardedProfiles(appendables []pyroscope.Appendable) int {
	count := 0
	for _, app := range appendables {
		if testApp, ok := app.(*testAppender); ok && testApp.lastProfile != nil {
			count++
		}
	}
	return count
}

func verifyForwardedProfiles(t *testing.T, appendables []pyroscope.Appendable, expectedProfile []byte, expectedHeaders map[string]string, expectedQueryParams string) {
	for i, app := range appendables {
		testApp, ok := app.(*testAppender)
		require.True(t, ok, "Appendable is not a testAppender")

		if testApp.lastProfile != nil {
			// Verify profile body
			body, err := io.ReadAll(testApp.lastProfile.Body)
			require.NoError(t, err, "Failed to read profile body for appendable %d", i)
			require.Equal(t, expectedProfile, body, "Profile mismatch for appendable %d", i)

			// Verify headers
			for key, value := range expectedHeaders {
				require.Equal(t, value, testApp.lastProfile.Headers.Get(key), "Header mismatch for key %s in appendable %d", key, i)
			}

			// Verify query parameters
			expectedParams, err := url.ParseQuery(expectedQueryParams)
			require.NoError(t, err, "Failed to parse expected query parameters")
			actualParams := testApp.lastProfile.URL.Query()
			for key, values := range expectedParams {
				require.Equal(t, values, actualParams[key], "Query parameter mismatch for key %s in appendable %d", key, i)
			}
		}
	}
}

func startComponent(t *testing.T, appendables []pyroscope.Appendable) int {
	port, err := freeport.GetFreePort()
	require.NoError(t, err)

	args := Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    port,
			},
		},
		ForwardTo: appendables,
	}

	comp, err := New(testOptions(t), args)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		require.NoError(t, comp.Run(ctx))
	}()

	waitForServerReady(t, port)
	return port
}

func sendCustomRequest(t *testing.T, port int, method, path, queryParams string, headers map[string]string, profileSize int) ([]byte, *http.Response) {
	t.Helper()
	testProfile := make([]byte, profileSize)
	_, err := rand.Read(testProfile)
	require.NoError(t, err)

	testURL := fmt.Sprintf("http://localhost:%d%s?%s", port, path, queryParams)

	req, err := http.NewRequest(method, testURL, bytes.NewReader(testProfile))
	require.NoError(t, err)

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return testProfile, resp
}

func waitForServerReady(t *testing.T, port int) {
	t.Helper()
	require.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", port))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusNotFound
	}, 2*time.Second, 100*time.Millisecond, "server did not start in time")
}

func testAppendable(appendErr error) pyroscope.Appendable {
	return &testAppender{appendErr: appendErr}
}

type testAppender struct {
	appendErr   error
	lastProfile *pyroscope.IncomingProfile
}

func (a *testAppender) Appender() pyroscope.Appender {
	return a
}

func (a *testAppender) Append(_ context.Context, _ labels.Labels, _ []*pyroscope.RawSample) error {
	return fmt.Errorf("Append method not implemented for test")
}

func (a *testAppender) AppendIngest(_ context.Context, profile *pyroscope.IncomingProfile) error {
	var buf bytes.Buffer
	tee := io.TeeReader(profile.Body, &buf)

	newProfile := &pyroscope.IncomingProfile{
		Body:    io.NopCloser(&buf),
		Headers: profile.Headers,
		URL:     profile.URL,
	}
	a.lastProfile = newProfile

	_, err := io.Copy(io.Discard, tee)
	if err != nil {
		return err
	}

	return a.appendErr
}

func testOptions(t *testing.T) component.Options {
	return component.Options{
		ID:         "pyroscope.receive_http.test",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
	}
}
