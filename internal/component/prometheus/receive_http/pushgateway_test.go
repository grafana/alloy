package receive_http

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloyprom "github.com/grafana/alloy/internal/component/prometheus"
)

const (
	textContentType  = "text/plain"
	protoContentType = "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited"
)

func TestParseGroupingLabels(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected labels.Labels
		errorMsg string
	}{
		{
			name:     "job only",
			path:     "job/demo_batch",
			expected: labels.FromStrings("job", "demo_batch"),
		},
		{
			name:     "job and grouping labels",
			path:     "job/demo_batch/instance/worker-1/zone/eu",
			expected: labels.FromStrings("instance", "worker-1", "job", "demo_batch", "zone", "eu"),
		},
		{
			name:     "trailing slash of a complete pair",
			path:     "job/demo_batch/",
			errorMsg: "expected /metrics/job/<JOB_NAME>",
		},
		{
			name:     "base64 encoded job",
			path:     "job@base64/ZGVtby9iYXRjaA",
			expected: labels.FromStrings("job", "demo/batch"),
		},
		{
			name:     "base64 encoded value with padding",
			path:     "job/demo_batch/instance@base64/d29ya2VyLzE=",
			expected: labels.FromStrings("instance", "worker/1", "job", "demo_batch"),
		},
		{
			name:     "base64 encoded empty value is dropped",
			path:     "job/demo_batch/instance@base64/=",
			expected: labels.FromStrings("job", "demo_batch"),
		},
		{
			name:     "empty value is dropped",
			path:     "job/demo_batch/instance/",
			expected: labels.FromStrings("job", "demo_batch"),
		},
		{
			name:     "last value of a repeated label wins",
			path:     "job/demo_batch/zone/eu/zone/us",
			expected: labels.FromStrings("job", "demo_batch", "zone", "us"),
		},
		{
			name:     "empty path",
			path:     "",
			errorMsg: "expected /metrics/job/<JOB_NAME>",
		},
		{
			name:     "odd number of components",
			path:     "job/demo_batch/instance",
			errorMsg: "expected /metrics/job/<JOB_NAME>",
		},
		{
			name:     "job is not the first label",
			path:     "instance/worker-1/job/demo_batch",
			errorMsg: `the first label must be "job"`,
		},
		{
			name:     "empty job name",
			path:     "job/",
			errorMsg: "the job name must not be empty",
		},
		{
			name:     "base64 encoded empty job name",
			path:     "job@base64/=",
			errorMsg: "the job name must not be empty",
		},
		{
			name:     "reserved label name",
			path:     "job/demo_batch/__name__/foo",
			errorMsg: `invalid label name "__name__"`,
		},
		{
			name:     "invalid base64 value",
			path:     "job/demo_batch/instance@base64/!!!",
			errorMsg: `invalid base64 value of label "instance"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := parseGroupingLabels(tt.path)
			if tt.errorMsg != "" {
				require.ErrorContains(t, err, tt.errorMsg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestIsSeriesOfFamily(t *testing.T) {
	tests := []struct {
		metricName string
		familyName string
		expected   bool
	}{
		{metricName: "request_duration_seconds", familyName: "request_duration_seconds", expected: true},
		{metricName: "request_duration_seconds_sum", familyName: "request_duration_seconds", expected: true},
		{metricName: "request_duration_seconds_bucket", familyName: "request_duration_seconds", expected: true},
		{metricName: "requests_total", familyName: "request_duration_seconds", expected: false},
		// A family whose name is a prefix of an unrelated metric name.
		{metricName: "requestsize", familyName: "requests", expected: false},
		{metricName: "requests", familyName: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.metricName+"/"+tt.familyName, func(t *testing.T) {
			lset := labels.FromStrings("__name__", tt.metricName)
			require.Equal(t, tt.expected, isSeriesOfFamily(lset, tt.familyName))
		})
	}
}

func TestPushTextFormat(t *testing.T) {
	samples := make(chan testSample, 10)
	addr := startPushComponent(t, Arguments{ForwardTo: testAppendable(samples)})

	body := `
# HELP job_last_success_timestamp Timestamp of the last successful run.
# TYPE job_last_success_timestamp gauge
job_last_success_timestamp 1700000000
requests_total{code="200"} 3
requests_total{code="500",job="overridden"} 1
`

	before := time.Now().UnixMilli()
	resp := doPush(t, http.MethodPost, addr+"/metrics/job/demo_batch/instance/worker-1", textContentType, body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	after := time.Now().UnixMilli()

	actual := collectSamples(t, samples, 3)
	require.Equal(t, []testSample{
		{ts: actual[0].ts, val: 1700000000, l: labels.FromStrings("__name__", "job_last_success_timestamp", "instance", "worker-1", "job", "demo_batch")},
		{ts: actual[1].ts, val: 3, l: labels.FromStrings("__name__", "requests_total", "code", "200", "instance", "worker-1", "job", "demo_batch")},
		{ts: actual[2].ts, val: 1, l: labels.FromStrings("__name__", "requests_total", "code", "500", "instance", "worker-1", "job", "demo_batch")},
	}, actual)

	// Samples without a timestamp of their own are stamped on arrival.
	for _, s := range actual {
		require.GreaterOrEqual(t, s.ts, before)
		require.LessOrEqual(t, s.ts, after)
	}
}

// TestPushFallsBackToTextFormat covers the Pushgateway behavior of parsing a
// body as the Prometheus text format when the Content-Type says nothing about
// it, which is what `curl --data-binary` sends.
func TestPushFallsBackToTextFormat(t *testing.T) {
	for _, contentType := range []string{"", "application/x-www-form-urlencoded", "application/json"} {
		t.Run(contentType, func(t *testing.T) {
			samples := make(chan testSample, 10)
			addr := startPushComponent(t, Arguments{ForwardTo: testAppendable(samples)})

			resp := doPush(t, http.MethodPost, addr+"/metrics/job/demo_batch", contentType, "my_metric 42\n")
			require.Equal(t, http.StatusOK, resp.StatusCode)

			actual := collectSamples(t, samples, 1)
			require.Equal(t, []testSample{
				{ts: actual[0].ts, val: 42, l: labels.FromStrings("__name__", "my_metric", "job", "demo_batch")},
			}, actual)
		})
	}
}

func TestPushHonorsTimestamps(t *testing.T) {
	samples := make(chan testSample, 10)
	addr := startPushComponent(t, Arguments{ForwardTo: testAppendable(samples)})

	resp := doPush(t, http.MethodPost, addr+"/metrics/job/demo_batch", textContentType, "my_metric 42 1700000000000\n")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, []testSample{
		{ts: 1700000000000, val: 42, l: labels.FromStrings("__name__", "my_metric", "job", "demo_batch")},
	}, collectSamples(t, samples, 1))
}

// TestPushFromClientLibrary covers a push from an unmodified Prometheus client
// library, which uses PUT, the protobuf exposition format, and base64 encoded
// path components.
func TestPushFromClientLibrary(t *testing.T) {
	samples := make(chan testSample, 10)
	addr := startPushComponent(t, Arguments{ForwardTo: testAppendable(samples)})

	registry := prometheus.NewRegistry()
	duration := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "batch_duration_seconds",
		Help: "Duration of the last batch run.",
	})
	duration.Set(1.5)
	registry.MustRegister(duration)

	// The job name contains a slash, so the client library base64 encodes it,
	// and the empty instance is encoded as a lone padding character.
	err := push.New("http://"+addr, "demo/batch").
		Grouping("instance", "").
		Gatherer(registry).
		Push()
	require.NoError(t, err)

	actual := collectSamples(t, samples, 1)
	require.Equal(t, []testSample{
		{ts: actual[0].ts, val: 1.5, l: labels.FromStrings("__name__", "batch_duration_seconds", "job", "demo/batch")},
	}, actual)
}

func TestPushNativeHistogram(t *testing.T) {
	histograms := make(chan testHistogram, 10)
	addr := startPushComponent(t, Arguments{ForwardTo: testHistogramAppendable(histograms)})

	registry := prometheus.NewRegistry()
	latency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                            "request_latency_seconds",
		Help:                            "Latency of the last requests.",
		NativeHistogramBucketFactor:     1.1,
		NativeHistogramMaxBucketNumber:  100,
		NativeHistogramMinResetDuration: time.Hour,
	})
	latency.Observe(0.5)
	latency.Observe(1.5)
	registry.MustRegister(latency)

	require.NoError(t, push.New("http://"+addr, "demo_batch").Gatherer(registry).Push())

	select {
	case actual := <-histograms:
		require.Equal(t, labels.FromStrings("__name__", "request_latency_seconds", "job", "demo_batch"), actual.l)
		require.NotNil(t, actual.h)
		require.NoError(t, actual.h.Validate())
		require.Equal(t, uint64(2), actual.h.Count)
		require.Equal(t, 2.0, actual.h.Sum)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a native histogram")
	}
}

func TestPushMetadata(t *testing.T) {
	samples := make(chan testSample, 10)
	metadata := make(chan testMetadata, 10)
	addr := startPushComponent(t, Arguments{
		ForwardTo:      testAppendableWithMetadata(samples, metadata),
		AppendMetadata: true,
	})

	body := `
# HELP request_duration_seconds Duration of the requests.
# TYPE request_duration_seconds summary
request_duration_seconds_sum 12
request_duration_seconds_count 3
`
	resp := doPush(t, http.MethodPost, addr+"/metrics/job/demo_batch", textContentType, body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	expected := testMetadata{metricName: "request_duration_seconds", metricType: "summary", help: "Duration of the requests."}
	for range 2 {
		select {
		case actual := <-metadata:
			// The suffixed series of a summary share the metadata of the family.
			require.Equal(t, expected.metricType, actual.metricType)
			require.Equal(t, expected.help, actual.help)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for metadata")
		}
	}
}

func TestPushErrors(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
		expected    int
	}{
		{
			name:     "invalid path",
			method:   http.MethodPost,
			path:     "/metrics/job/demo_batch/instance",
			body:     "my_metric 1\n",
			expected: http.StatusBadRequest,
		},
		{
			name:     "missing job name",
			method:   http.MethodPost,
			path:     "/metrics/job/",
			body:     "my_metric 1\n",
			expected: http.StatusBadRequest,
		},
		{
			name:     "unparsable body",
			method:   http.MethodPost,
			path:     "/metrics/job/demo_batch",
			body:     "this is not an exposition format\n",
			expected: http.StatusBadRequest,
		},
		{
			name:        "unparsable protobuf body",
			method:      http.MethodPost,
			path:        "/metrics/job/demo_batch",
			contentType: protoContentType,
			body:        "my_metric 1\n",
			expected:    http.StatusBadRequest,
		},
		{
			name:     "delete is not supported",
			method:   http.MethodDelete,
			path:     "/metrics/job/demo_batch",
			expected: http.StatusMethodNotAllowed,
		},
		{
			name:     "get is not supported",
			method:   http.MethodGet,
			path:     "/metrics/job/demo_batch",
			expected: http.StatusMethodNotAllowed,
		},
	}

	samples := make(chan testSample, 10)
	addr := startPushComponent(t, Arguments{ForwardTo: testAppendable(samples)})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := doPush(t, tt.method, addr+tt.path, tt.contentType, tt.body)
			require.Equal(t, tt.expected, resp.StatusCode)
			require.Empty(t, samples)
		})
	}
}

// TestPushDoesNotBreakRemoteWrite makes sure the push endpoint doesn't shadow
// the remote write one.
func TestPushDoesNotBreakRemoteWrite(t *testing.T) {
	samples := make(chan testSample, 10)
	addr := startPushComponent(t, Arguments{ForwardTo: testAppendable(samples)})

	resp := doPush(t, http.MethodPost, addr+"/metrics/job/demo_batch", textContentType, "my_metric 1\n")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doPush(t, http.MethodPost, addr+"/api/v1/metrics/write", "", "")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func startPushComponent(t *testing.T, args Arguments) string {
	t.Helper()

	args.Server = &fnet.ServerConfig{
		HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: 0},
		GRPC: testGRPCConfig(),
	}
	args.AcceptedRemoteWriteProtobufMessages = []string{string(remote.WriteV1MessageType)}

	comp, err := New(testOptions(t), args)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go func() {
		_ = comp.Run(ctx)
	}()

	addr := comp.server.HTTPListenAddr()
	waitForServerToBeReady(t, addr, nil)
	return addr
}

func doPush(t *testing.T, method, url, contentType, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, "http://"+url, strings.NewReader(body))
	require.NoError(t, err)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// collectSamples waits for count samples and returns them sorted by labels, so
// that assertions don't depend on the order the exposition was parsed in.
func collectSamples(t *testing.T, samples chan testSample, count int) []testSample {
	t.Helper()

	collected := make([]testSample, 0, count)
	for range count {
		select {
		case sample := <-samples:
			collected = append(collected, sample)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out after %d of %d samples", len(collected), count)
		}
	}
	sort.Slice(collected, func(i, j int) bool { return labels.Compare(collected[i].l, collected[j].l) < 0 })

	require.Empty(t, samples, "unexpected extra samples")
	return collected
}

type testHistogram struct {
	l labels.Labels
	h *histogram.Histogram
}

func testHistogramAppendable(actual chan testHistogram) []storage.Appendable {
	hookFn := func(
		ref storage.SeriesRef,
		l labels.Labels,
		ts int64,
		h *histogram.Histogram,
		fh *histogram.FloatHistogram,
		next storage.Appender,
	) (storage.SeriesRef, error) {

		actual <- testHistogram{l: l, h: h}
		return ref, nil
	}

	return []storage.Appendable{alloyprom.NewInterceptor(nil, alloyprom.WithHistogramHook(hookFn))}
}
