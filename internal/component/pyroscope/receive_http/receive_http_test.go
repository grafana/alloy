package receive_http

import (
	"bytes"
	"context"
	"crypto/rand"
	crypto_tls "crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/net/http2"

	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
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
		name                string
		profileSize         int
		method              string
		path                string
		queryParams         string
		expectedContentType []string
		appendableErrors    []error
		expectedStatus      int
		expectedForwards    int
		expectedLabels      map[string]string
	}{
		{
			name:                "Small profile",
			profileSize:         1024, // 1KB
			method:              "POST",
			path:                "/ingest",
			queryParams:         "name=test_app&from=1234567890&until=1234567900",
			expectedContentType: []string{"application/octet-stream"},
			appendableErrors:    []error{nil, nil},
			expectedStatus:      http.StatusOK,
			expectedForwards:    2,
			expectedLabels: map[string]string{
				"__name__":     "test_app",
				"service_name": "test_app",
			},
		},
		{
			name:                "Large profile with two headers",
			profileSize:         1024 * 1024, // 1MB
			method:              "POST",
			path:                "/ingest",
			queryParams:         "name=test_app&from=1234567891&until=1234567901&custom=param1",
			appendableErrors:    []error{nil},
			expectedContentType: []string{"my-content", "is", "multiple"}, // not too sure when that happens
			expectedStatus:      http.StatusOK,
			expectedForwards:    1,
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
			appendableErrors: []error{nil, nil},
			expectedStatus:   http.StatusMethodNotAllowed,
			expectedForwards: 0,
		},
		{
			name:                "Invalid path",
			profileSize:         1024,
			method:              "POST",
			path:                "/invalid",
			queryParams:         "name=test_app&from=1234567893&until=1234567903",
			expectedContentType: []string{"application/octet-stream"},
			appendableErrors:    []error{nil, nil},
			expectedStatus:      http.StatusNotFound,
			expectedForwards:    0,
		},
		{
			name:                "All appendables fail",
			profileSize:         2048,
			method:              "POST",
			path:                "/ingest",
			queryParams:         "name=test_app&from=1234567894&until=1234567904&scenario=all_fail",
			expectedContentType: []string{"application/octet-stream"},
			appendableErrors:    []error{fmt.Errorf("error1"), fmt.Errorf("error2")},
			expectedStatus:      http.StatusInternalServerError,
			expectedForwards:    2,
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
			appendableErrors: []error{fmt.Errorf("error"), nil},
			expectedStatus:   http.StatusInternalServerError,
			expectedForwards: 2,
			expectedLabels: map[string]string{
				"__name__":     "test_app",
				"service_name": "test_app",
			},
		},
		{
			name:                "Valid labels are parsed and forwarded",
			profileSize:         1024,
			method:              "POST",
			path:                "/ingest",
			queryParams:         "name=test.app{env=prod,region=us-east}",
			expectedContentType: []string{"application/octet-stream"},
			appendableErrors:    []error{nil, nil},
			expectedStatus:      http.StatusOK,
			expectedForwards:    2,
			expectedLabels: map[string]string{
				"__name__":     "test.app",
				"service_name": "test.app",
				"env":          "prod",
				"region":       "us-east",
			},
		},
		{
			name:                "Invalid labels still forward profile",
			profileSize:         1024,
			method:              "POST",
			path:                "/ingest",
			queryParams:         "name=test.app{invalid-label-syntax}",
			expectedContentType: []string{"application/octet-stream"},
			appendableErrors:    []error{nil, nil},
			expectedStatus:      http.StatusOK,
			expectedForwards:    2,
			expectedLabels: map[string]string{
				"__name__":     "test.app",
				"service_name": "test.app",
			},
		},
		{
			name:                "existing service_name sets app_name from __name__",
			profileSize:         1024,
			method:              "POST",
			path:                "/ingest",
			queryParams:         "name=test.app{service_name=my-service}",
			expectedContentType: []string{"application/octet-stream"},
			appendableErrors:    []error{nil},
			expectedStatus:      http.StatusOK,
			expectedForwards:    1,
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

			testProfile, resp := sendCustomRequest(t, port, tt.method, tt.path, tt.queryParams, tt.expectedContentType, tt.profileSize)
			require.Equal(t, tt.expectedStatus, resp.StatusCode)

			forwardedCount := countForwardedProfiles(appendables)
			require.Equal(t, tt.expectedForwards, forwardedCount, "Unexpected number of forwards")

			if tt.expectedForwards > 0 {
				verifyForwardedProfiles(t, appendables, testProfile, tt.expectedContentType, tt.queryParams, tt.expectedLabels)
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

			ctx, cancel := context.WithCancel(t.Context())
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
						ID:         fmt.Sprintf("request-id-%d-%d", i, j),
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
				for seriesIdx, samples := range a.pushedSamples {
					for sampleIdx, sample := range samples {
						require.Len(t, sample.RawProfile, tc.SampleSize)
						// Verify that ID field is propagated correctly
						expectedID := fmt.Sprintf("request-id-%d-%d", seriesIdx, sampleIdx)
						require.Equal(t, expectedID, sample.ID, "ID field should be correctly propagated to sample")
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
	expectedContentType []string,
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

			// Verify content type
			require.Equal(
				t,
				expectedContentType,
				testApp.lastProfile.ContentType,
				"Content-Type mismatch in appendable %d",
				i,
			)

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

	comp, err := New(
		util.TestAlloyLogger(t),
		noop.Tracer{},
		prometheus.NewRegistry(),
		args,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
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
	contentType []string,
	profileSize int,
) ([]byte, *http.Response) {

	t.Helper()
	testProfile := make([]byte, profileSize)
	_, err := rand.Read(testProfile)
	require.NoError(t, err)

	testURL := fmt.Sprintf("http://localhost:%d%s?%s", port, path, queryParams)

	req, err := http.NewRequest(method, testURL, bytes.NewReader(testProfile))
	require.NoError(t, err)

	for idx := range contentType {
		if idx == 0 {
			req.Header.Set(pyroscope.HeaderContentType, contentType[idx])
			continue
		}
		req.Header.Add(pyroscope.HeaderContentType, contentType[idx])
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
	mu          sync.Mutex
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
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pushedLabels = append(a.pushedLabels, lbls)
	a.pushedSamples = append(a.pushedSamples, samples)
	return a.appendErr
}

func (a *testAppender) AppendIngest(_ context.Context, profile *pyroscope.IncomingProfile) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	newProfile := &pyroscope.IncomingProfile{
		RawBody:     profile.RawBody,
		ContentType: profile.ContentType,
		URL:         profile.URL,
		Labels:      profile.Labels,
	}
	a.lastProfile = newProfile
	return a.appendErr
}

func (a *testAppender) ConnectClients() []debuginfov1alpha1connect.DebuginfoServiceClient {
	return nil
}

func (a *testAppender) Upload(_ debuginfo.UploadJob) {
	// no-op for tests
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

	comp, err := New(
		util.TestAlloyLogger(t),
		noop.Tracer{},
		prometheus.NewRegistry(),
		args,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
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

	shutdown, err := comp.update(Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    ports[1],
			},
		},
		ForwardTo: forwardTo,
	})
	require.NoError(t, err)
	require.False(t, shutdown)
}

// TestAPIToAlloySamples verifies that the ID field is properly propagated
// from API samples to Alloy samples during conversion.
func TestAPIToAlloySamples(t *testing.T) {
	tests := []struct {
		name     string
		input    []*pushv1.RawSample
		expected []*pyroscope.RawSample
	}{
		{
			name: "single sample with ID",
			input: []*pushv1.RawSample{
				{
					ID:         "test-id-123",
					RawProfile: []byte("profile-data-1"),
				},
			},
			expected: []*pyroscope.RawSample{
				{
					ID:         "test-id-123",
					RawProfile: []byte("profile-data-1"),
				},
			},
		},
		{
			name: "multiple samples with different IDs",
			input: []*pushv1.RawSample{
				{
					ID:         "request-id-1",
					RawProfile: []byte("profile-1"),
				},
				{
					ID:         "request-id-2",
					RawProfile: []byte("profile-2"),
				},
			},
			expected: []*pyroscope.RawSample{
				{
					ID:         "request-id-1",
					RawProfile: []byte("profile-1"),
				},
				{
					ID:         "request-id-2",
					RawProfile: []byte("profile-2"),
				},
			},
		},
		{
			name: "sample with empty ID",
			input: []*pushv1.RawSample{
				{
					ID:         "",
					RawProfile: []byte("profile-data"),
				},
			},
			expected: []*pyroscope.RawSample{
				{
					ID:         "",
					RawProfile: []byte("profile-data"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := apiToAlloySamples(tt.input)

			require.Len(t, result, len(tt.expected), "Unexpected number of samples")

			for i, expected := range tt.expected {
				require.Equal(t, expected.ID, result[i].ID, "ID mismatch at index %d", i)
				require.Equal(t, expected.RawProfile, result[i].RawProfile, "RawProfile mismatch at index %d", i)
			}
		})
	}
}

// --- debuginfo proxy tests ---

// mockDebuginfoHandler implements debuginfov1alpha1connect.DebuginfoServiceHandler for testing.
type mockDebuginfoHandler struct {
	uploadFunc func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error
}

func (m *mockDebuginfoHandler) Upload(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
	return m.uploadFunc(ctx, stream)
}

// downstreamResult captures what a mock downstream server received.
type downstreamResult struct {
	initFile *debuginfov1alpha1.FileMetadata
	data     []byte
	chunks   int
}

// startMockDownstream creates a TLS httptest server (HTTP/2) running a mock debuginfo handler.
// shouldUpload controls whether the server accepts uploads.
// resultCh receives the captured result after the handler finishes.
func startMockDownstream(t *testing.T, shouldUpload bool, resultCh chan<- downstreamResult) debuginfov1alpha1connect.DebuginfoServiceClient {
	t.Helper()
	handler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			req, err := stream.Receive()
			if err != nil {
				return err
			}

			if err := stream.Send(&debuginfov1alpha1.UploadResponse{
				Data: &debuginfov1alpha1.UploadResponse_Init{
					Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
						ShouldInitiateUpload: shouldUpload,
					},
				},
			}); err != nil {
				return err
			}

			res := downstreamResult{
				initFile: req.GetInit().GetFile(),
			}

			if shouldUpload {
				for {
					chunkReq, err := stream.Receive()
					if err != nil {
						break
					}
					if chunk := chunkReq.GetChunk(); chunk != nil {
						res.chunks++
						res.data = append(res.data, chunk.GetChunk()...)
					}
				}
			}

			resultCh <- res
			return nil
		},
	}

	_, h := debuginfov1alpha1connect.NewDebuginfoServiceHandler(handler)
	server := httptest.NewUnstartedServer(h)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)
	return debuginfov1alpha1connect.NewDebuginfoServiceClient(server.Client(), server.URL)
}

// debuginfoAppendable is a test pyroscope.Appendable that returns Connect debuginfo clients.
type debuginfoAppendable struct {
	clients []debuginfov1alpha1connect.DebuginfoServiceClient
}

func (d *debuginfoAppendable) Appender() pyroscope.Appender { return d }
func (d *debuginfoAppendable) Append(_ context.Context, _ labels.Labels, _ []*pyroscope.RawSample) error {
	return nil
}
func (d *debuginfoAppendable) AppendIngest(_ context.Context, _ *pyroscope.IncomingProfile) error {
	return nil
}
func (d *debuginfoAppendable) Upload(_ debuginfo.UploadJob) {}
func (d *debuginfoAppendable) ConnectClients() []debuginfov1alpha1connect.DebuginfoServiceClient {
	return d.clients
}

// h2cClient returns an HTTP client that speaks h2c (HTTP/2 over cleartext).
func h2cClient() *http.Client {
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *crypto_tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}
}

// sendUploadViaProxy opens a bidi stream to the proxy, sends an init request, reads the
// init response, and if accepted, streams fileData as chunks.
func sendUploadViaProxy(t *testing.T, port int, fileData []byte) (bool, error) {
	t.Helper()
	proxyURL := fmt.Sprintf("http://localhost:%d", port)
	client := debuginfov1alpha1connect.NewDebuginfoServiceClient(h2cClient(), proxyURL)

	stream := client.Upload(context.Background())

	// Send init.
	err := stream.Send(&debuginfov1alpha1.UploadRequest{
		Data: &debuginfov1alpha1.UploadRequest_Init{
			Init: &debuginfov1alpha1.ShouldInitiateUploadRequest{
				File: &debuginfov1alpha1.FileMetadata{
					GnuBuildId: "test-build-id",
					Name:       "test.so",
					Type:       debuginfov1alpha1.FileMetadata_TYPE_EXECUTABLE_FULL,
				},
			},
		},
	})
	if err != nil {
		return false, fmt.Errorf("send init: %w", err)
	}

	// Receive init response.
	resp, err := stream.Receive()
	if err != nil {
		return false, fmt.Errorf("receive init response: %w", err)
	}

	initResp := resp.GetInit()
	if initResp == nil {
		return false, fmt.Errorf("expected init response")
	}

	if !initResp.ShouldInitiateUpload {
		_ = stream.CloseRequest()
		return false, nil
	}

	// Stream chunks.
	chunkSize := 1024
	for offset := 0; offset < len(fileData); offset += chunkSize {
		end := offset + chunkSize
		if end > len(fileData) {
			end = len(fileData)
		}
		if err := stream.Send(&debuginfov1alpha1.UploadRequest{
			Data: &debuginfov1alpha1.UploadRequest_Chunk{
				Chunk: &debuginfov1alpha1.UploadChunk{
					Chunk: fileData[offset:end],
				},
			},
		}); err != nil {
			return true, fmt.Errorf("send chunk: %w", err)
		}
	}

	_ = stream.CloseRequest()
	return true, nil
}

func TestDebugInfoProxy_SingleEndpoint_AcceptsUpload(t *testing.T) {
	resultCh := make(chan downstreamResult, 1)
	dsClient := startMockDownstream(t, true, resultCh)

	appendable := &debuginfoAppendable{clients: []debuginfov1alpha1connect.DebuginfoServiceClient{dsClient}}
	port := startComponent(t, []pyroscope.Appendable{appendable})

	fileData := []byte("hello proxy debuginfo upload test data")
	accepted, err := sendUploadViaProxy(t, port, fileData)
	require.NoError(t, err)
	require.True(t, accepted)

	select {
	case res := <-resultCh:
		require.Equal(t, "test-build-id", res.initFile.GetGnuBuildId())
		require.Equal(t, "test.so", res.initFile.GetName())
		require.Equal(t, fileData, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for downstream to receive upload")
	}
}

func TestDebugInfoProxy_MultipleEndpoints_AllAccept(t *testing.T) {
	const numEndpoints = 3
	resultChs := make([]chan downstreamResult, numEndpoints)
	clients := make([]debuginfov1alpha1connect.DebuginfoServiceClient, numEndpoints)

	for i := 0; i < numEndpoints; i++ {
		resultChs[i] = make(chan downstreamResult, 1)
		clients[i] = startMockDownstream(t, true, resultChs[i])
	}

	appendable := &debuginfoAppendable{clients: clients}
	port := startComponent(t, []pyroscope.Appendable{appendable})

	fileData := []byte("multi-endpoint-test-data-for-all-accepting-servers")
	accepted, err := sendUploadViaProxy(t, port, fileData)
	require.NoError(t, err)
	require.True(t, accepted)

	for i := 0; i < numEndpoints; i++ {
		select {
		case res := <-resultChs[i]:
			require.Equal(t, "test-build-id", res.initFile.GetGnuBuildId(), "endpoint %d", i)
			require.Equal(t, fileData, res.data, "endpoint %d data mismatch", i)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for downstream %d", i)
		}
	}
}

func TestDebugInfoProxy_MultipleEndpoints_AllDecline(t *testing.T) {
	const numEndpoints = 3
	resultChs := make([]chan downstreamResult, numEndpoints)
	clients := make([]debuginfov1alpha1connect.DebuginfoServiceClient, numEndpoints)

	for i := 0; i < numEndpoints; i++ {
		resultChs[i] = make(chan downstreamResult, 1)
		clients[i] = startMockDownstream(t, false, resultChs[i])
	}

	appendable := &debuginfoAppendable{clients: clients}
	port := startComponent(t, []pyroscope.Appendable{appendable})

	fileData := []byte("should-not-be-sent")
	accepted, err := sendUploadViaProxy(t, port, fileData)
	require.NoError(t, err)
	require.False(t, accepted, "proxy should decline when all endpoints decline")

	// Verify all downstreams received the init but no chunks.
	for i := 0; i < numEndpoints; i++ {
		select {
		case res := <-resultChs[i]:
			require.Equal(t, "test-build-id", res.initFile.GetGnuBuildId(), "endpoint %d", i)
			require.Nil(t, res.data, "endpoint %d should not have received chunks", i)
			require.Equal(t, 0, res.chunks, "endpoint %d should not have received chunks", i)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for downstream %d", i)
		}
	}
}

func TestDebugInfoProxy_MultipleEndpoints_SomeAccept(t *testing.T) {
	// 3 endpoints: [0]=decline, [1]=accept, [2]=accept
	accepts := []bool{false, true, true}
	resultChs := make([]chan downstreamResult, len(accepts))
	clients := make([]debuginfov1alpha1connect.DebuginfoServiceClient, len(accepts))

	for i, shouldAccept := range accepts {
		resultChs[i] = make(chan downstreamResult, 1)
		clients[i] = startMockDownstream(t, shouldAccept, resultChs[i])
	}

	appendable := &debuginfoAppendable{clients: clients}
	port := startComponent(t, []pyroscope.Appendable{appendable})

	fileData := []byte("partial-accept-test-data")
	accepted, err := sendUploadViaProxy(t, port, fileData)
	require.NoError(t, err)
	require.True(t, accepted, "proxy should accept when at least one endpoint accepts")

	for i, shouldAccept := range accepts {
		select {
		case res := <-resultChs[i]:
			require.Equal(t, "test-build-id", res.initFile.GetGnuBuildId(), "endpoint %d", i)
			if shouldAccept {
				require.Equal(t, fileData, res.data, "accepting endpoint %d data mismatch", i)
			} else {
				require.Nil(t, res.data, "declining endpoint %d should not receive chunks", i)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for downstream %d", i)
		}
	}
}

func TestDebugInfoProxy_NoEndpoints(t *testing.T) {
	// No downstream clients at all.
	appendable := &debuginfoAppendable{clients: nil}
	port := startComponent(t, []pyroscope.Appendable{appendable})

	proxyURL := fmt.Sprintf("http://localhost:%d", port)
	client := debuginfov1alpha1connect.NewDebuginfoServiceClient(h2cClient(), proxyURL)
	stream := client.Upload(context.Background())

	err := stream.Send(&debuginfov1alpha1.UploadRequest{
		Data: &debuginfov1alpha1.UploadRequest_Init{
			Init: &debuginfov1alpha1.ShouldInitiateUploadRequest{
				File: &debuginfov1alpha1.FileMetadata{
					GnuBuildId: "test",
					Name:       "test.so",
				},
			},
		},
	})
	if err != nil {
		// Error on send is acceptable — server may reject immediately.
		return
	}

	_, err = stream.Receive()
	require.Error(t, err)
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}
