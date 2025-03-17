package receive_http

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/util"
	pushv1 "github.com/grafana/pyroscope/api/gen/proto/go/push/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/push/v1/pushv1connect"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
	"github.com/grafana/pyroscope/api/model/labelset"
)

// TestForwardsProfilesIngest verifies the behavior of the
// pyroscope.receive_http component under various scenarios. It tests different
// profile sizes, HTTP methods, paths, query parameters, and error conditions
// to ensure correct forwarding behavior and proper error handling, when
// clients use the legacy OG Pyroscope /ingest API, which is predominentaly
// used by the SDKs.
func TestForwardsProfilesIngest(t *testing.T) {
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
		expectedLabels   map[string]string
	}{
		{
			name:             "Small profile",
			profileSize:      1024, // 1KB
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test_app&from=1234567890&until=1234567900",
			headers:          map[string]string{"Content-Type": "application/octet-stream"},
			appendableErrors: []error{nil, nil},
			expectedStatus:   http.StatusOK,
			expectedForwards: 2,
			expectedLabels: map[string]string{
				"__name__":     "test_app",
				"service_name": "test_app",
			},
		},
		{
			name:             "Large profile with custom headers",
			profileSize:      1024 * 1024, // 1MB
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test_app&from=1234567891&until=1234567901&custom=param1",
			headers:          map[string]string{"X-Scope-OrgID": "1234"},
			appendableErrors: []error{nil},
			expectedStatus:   http.StatusOK,
			expectedForwards: 1,
			expectedLabels: map[string]string{
				"__name__":     "test_app",
				"service_name": "test_app",
			},
		},
		{
			name:             "Invalid method",
			profileSize:      1024,
			method:           "GET",
			path:             "/ingest",
			queryParams:      "name=test_app&from=1234567892&until=1234567902",
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
			queryParams:      "name=test_app&from=1234567893&until=1234567903",
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
			queryParams:      "name=test_app&from=1234567894&until=1234567904&scenario=all_fail",
			headers:          map[string]string{"Content-Type": "application/octet-stream", "X-Test": "fail-all"},
			appendableErrors: []error{fmt.Errorf("error1"), fmt.Errorf("error2")},
			expectedStatus:   http.StatusInternalServerError,
			expectedForwards: 2,
			expectedLabels: map[string]string{
				"__name__":     "test_app",
				"service_name": "test_app",
			},
		},
		{
			name:             "One appendable fails, one succeeds",
			profileSize:      4096,
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test_app&from=1234567895&until=1234567905&scenario=partial_failure",
			headers:          map[string]string{"X-Custom-ID": "test-6"},
			appendableErrors: []error{fmt.Errorf("error"), nil},
			expectedStatus:   http.StatusInternalServerError,
			expectedForwards: 2,
			expectedLabels: map[string]string{
				"__name__":     "test_app",
				"service_name": "test_app",
			},
		},
		{
			name:             "Valid labels are parsed and forwarded",
			profileSize:      1024,
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test.app{env=prod,region=us-east}",
			headers:          map[string]string{"Content-Type": "application/octet-stream"},
			appendableErrors: []error{nil, nil},
			expectedStatus:   http.StatusOK,
			expectedForwards: 2,
			expectedLabels: map[string]string{
				"__name__":     "test.app",
				"service_name": "test.app",
				"env":          "prod",
				"region":       "us-east",
			},
		},
		{
			name:             "Invalid labels still forward profile",
			profileSize:      1024,
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test.app{invalid-label-syntax}",
			headers:          map[string]string{"Content-Type": "application/octet-stream"},
			appendableErrors: []error{nil, nil},
			expectedStatus:   http.StatusOK,
			expectedForwards: 2,
			expectedLabels: map[string]string{
				"__name__":     "test.app",
				"service_name": "test.app",
			},
		},
		{
			name:             "existing service_name sets app_name from __name__",
			profileSize:      1024,
			method:           "POST",
			path:             "/ingest",
			queryParams:      "name=test.app{service_name=my-service}",
			headers:          map[string]string{"Content-Type": "application/octet-stream"},
			appendableErrors: []error{nil},
			expectedStatus:   http.StatusOK,
			expectedForwards: 1,
			expectedLabels: map[string]string{
				"__name__":     "test.app",
				"service_name": "my-service",
				"app_name":     "test.app",
			},
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
				verifyForwardedProfiles(t, appendables, testProfile, tt.headers, tt.queryParams, tt.expectedLabels)
			}
		})
	}
}

// TestForwardsProfilesPushV1 verifies the behavior of the
// pyroscope.receive_http using the connect pushv1 API. This is predominentaly
// used by other alloy components like pyrscope.ebpf.
func TestForwardsProfilesPushV1(t *testing.T) {
	for _, tc := range []struct {
		name             string
		clientOpts       []connect.ClientOption
		appendableErrors []error

		numSeries           int
		numSamplesPerSeries int
		SampleSize          int

		expectedSeries []string
		expectedError  error
	}{
		{
			name:           "One series, one small profile, one appendables",
			expectedSeries: []string{`{app="app-0"}`},
		},
		{
			name:           "One series, one small profile, one appendables using JSON",
			expectedSeries: []string{`{app="app-0"}`},
			clientOpts:     []connect.ClientOption{connect.WithProtoJSON()},
		},
		{
			name:           "One series, one small profile, one appendables using GRPC",
			expectedSeries: []string{`{app="app-0"}`},
			clientOpts:     []connect.ClientOption{connect.WithGRPC()},
		},
		{
			name:           "One series, one small profile, one appendables using GRPCWeb",
			expectedSeries: []string{`{app="app-0"}`},
			clientOpts:     []connect.ClientOption{connect.WithGRPCWeb()},
		},
		{
			name:      "Two series, one small profile, one appendables",
			numSeries: 2,
			expectedSeries: []string{
				`{app="app-0"}`,
				`{app="app-1"}`,
			},
		},
		{
			name:                "One series, two small profile, one appendable",
			numSamplesPerSeries: 2,
			expectedSeries:      []string{`{app="app-0"}`},
		},
		{
			name:                "One series, two small profile, two appendable",
			numSamplesPerSeries: 2,
			appendableErrors:    []error{nil, nil},
			expectedSeries:      []string{`{app="app-0"}`},
		},
		{
			name:                "One series, two small profile, two appendable one with errors",
			numSamplesPerSeries: 2,
			appendableErrors:    []error{nil, errors.New("wtf")},
			expectedSeries:      []string{`{app="app-0"}`},
			expectedError:       errors.New(`internal: unable to append series {app="app-0"} to appendable 1: wtf`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.SampleSize == 0 {
				tc.SampleSize = 1024
			}
			if tc.numSeries == 0 {
				tc.numSeries = 1
			}
			if len(tc.appendableErrors) == 0 {
				tc.appendableErrors = []error{nil}
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			appendables := createTestAppendables(tc.appendableErrors)
			port := startComponent(t, appendables)

			c := pushv1connect.NewPusherServiceClient(
				http.DefaultClient,
				fmt.Sprintf("http://127.0.0.1:%d", port),
				tc.clientOpts...)

			var series []*pushv1.RawProfileSeries
			for i := 0; i < tc.numSeries; i++ {
				var samples []*pushv1.RawSample
				for j := 0; j < tc.numSamplesPerSeries; j++ {
					samples = append(samples, &pushv1.RawSample{
						RawProfile: bytes.Repeat([]byte{0xde, 0xad}, tc.SampleSize/2),
					})
				}

				series = append(series, &pushv1.RawProfileSeries{
					Labels: []*typesv1.LabelPair{
						{Name: "app", Value: fmt.Sprintf("app-%d", i)},
					},
					Samples: samples,
				})
			}

			_, err := c.Push(ctx, connect.NewRequest(&pushv1.PushRequest{
				Series: series,
			}))
			if tc.expectedError != nil {
				require.ErrorContains(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}

			for idx := range appendables {
				a := appendables[idx].(*testAppender)

				// check series match
				require.Equal(t, a.series(), tc.expectedSeries)

				// check number of samples is correct
				require.Equal(t, tc.numSeries*tc.numSamplesPerSeries, a.samples())

				// check samples are received in full
				for _, samples := range a.pushedSamples {
					for _, sample := range samples {
						require.Len(t, sample.RawProfile, tc.SampleSize)
					}
				}
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

func verifyForwardedProfiles(
	t *testing.T,
	appendables []pyroscope.Appendable,
	expectedProfile []byte,
	expectedHeaders map[string]string,
	expectedQueryParams string,
	expectedLabels map[string]string,
) {

	for i, app := range appendables {
		testApp, ok := app.(*testAppender)
		require.True(t, ok, "Appendable is not a testAppender")

		// Skip name parameter label check if we're testing service_name behavior
		if expectedLabels == nil || expectedLabels["service_name"] == "" {
			if nameParam := testApp.lastProfile.URL.Query().Get("name"); nameParam != "" {
				ls, err := labelset.Parse(nameParam)
				if err == nil {
					require.Equal(t, ls.Labels(), testApp.lastProfile.Labels.Map(),
						"Labels mismatch for appendable %d", i)
				}
			}
		}

		if expectedLabels != nil {
			require.Equal(t, expectedLabels, testApp.lastProfile.Labels.Map(),
				"Labels mismatch for appendable %d", i)
		}

		if testApp.lastProfile != nil {
			// Verify profile body
			require.Equal(t, expectedProfile, testApp.lastProfile.RawBody, "Profile mismatch for appendable %d", i)

			// Verify headers
			for key, value := range expectedHeaders {
				require.Equal(
					t,
					value,
					testApp.lastProfile.Headers.Get(key),
					"Header mismatch for key %s in appendable %d",
					key,
					i,
				)
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

func sendCustomRequest(
	t *testing.T,
	port int,
	method, path, queryParams string,
	headers map[string]string,
	profileSize int,
) ([]byte, *http.Response) {

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

	pushedLabels  []labels.Labels
	pushedSamples [][]*pyroscope.RawSample
}

func (a *testAppender) samples() int {
	var c = 0
	for _, x := range a.pushedSamples {
		c += len(x)
	}
	return c
}

func (a *testAppender) series() []string {
	var series []string
	for _, labels := range a.pushedLabels {
		series = append(series, labels.String())
	}
	return series
}

func (a *testAppender) Appender() pyroscope.Appender {
	return a
}

func (a *testAppender) Append(_ context.Context, lbls labels.Labels, samples []*pyroscope.RawSample) error {
	a.pushedLabels = append(a.pushedLabels, lbls)
	a.pushedSamples = append(a.pushedSamples, samples)
	return a.appendErr
}

func (a *testAppender) AppendIngest(_ context.Context, profile *pyroscope.IncomingProfile) error {
	newProfile := &pyroscope.IncomingProfile{
		RawBody: profile.RawBody,
		Headers: profile.Headers,
		URL:     profile.URL,
		Labels:  profile.Labels,
	}
	a.lastProfile = newProfile
	return a.appendErr
}

func testOptions(t *testing.T) component.Options {
	return component.Options{
		ID:         "pyroscope.receive_http.test",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
	}
}

// TestUpdateArgs verifies that the component can be updated with new arguments. This explicitly also makes sure that the server is restarted when the server configuration changes. And there are no metric registration conflicts.
func TestUpdateArgs(t *testing.T) {
	ports, err := freeport.GetFreePorts(2)
	require.NoError(t, err)

	forwardTo := []pyroscope.Appendable{testAppendable(nil)}

	args := Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    ports[0],
			},
		},
		ForwardTo: forwardTo,
	}

	comp, err := New(testOptions(t), args)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		require.NoError(t, comp.Run(ctx))
	}()

	waitForServerReady(t, ports[0])

	comp.Update(Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    ports[1],
			},
		},
		ForwardTo: forwardTo,
	})

	waitForServerReady(t, ports[1])
}
