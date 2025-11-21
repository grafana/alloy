package remotewrite_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

// Test is an integration-level test which ensures that metrics can get sent to
// a prometheus.remote_write component and forwarded to a
// remote_write-compatible server.
func Test(t *testing.T) {
	writeResult := make(chan *prompb.WriteRequest)

	// Create a remote_write server which forwards any received payloads to the
	// writeResult channel.
	srv := newTestServer(t, writeResult)
	defer srv.Close()

	// Create our component and wait for it to start running, so we can write
	// metrics to the WAL.
	args := testArgsForConfig(t, fmt.Sprintf(`
		external_labels = {
			cluster = "local",
		}
		endpoint {
			name           = "test-url"
			url            = "%s/api/v1/write"
			remote_timeout = "100ms"

			queue_config {
				batch_send_deadline = "100ms"
			}
		}
	`, srv.URL))
	tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "prometheus.remote_write")
	require.NoError(t, err)
	go func() {
		err = tc.Run(componenttest.TestContext(t), args)
		require.NoError(t, err)
	}()
	require.NoError(t, tc.WaitRunning(5*time.Second))

	// We need to use a future timestamp since remote_write will ignore any
	// sample which is earlier than the time when it started. Adding a minute
	// ensures that our samples will never get ignored.
	sampleTimestamp := time.Now().Add(time.Minute).UnixMilli()

	// Send metrics to our component. These will be written to the WAL and
	// subsequently written to our HTTP server.
	sendMetric(t, tc, labels.FromStrings("foo", "bar"), sampleTimestamp, 12)
	sendMetric(t, tc, labels.FromStrings("fizz", "buzz"), sampleTimestamp, 34)

	expect := []prompb.TimeSeries{{
		Labels: []prompb.Label{
			{Name: "cluster", Value: "local"},
			{Name: "foo", Value: "bar"},
		},
		Samples: []prompb.Sample{
			{Timestamp: sampleTimestamp, Value: 12},
		},
	}, {
		Labels: []prompb.Label{
			{Name: "cluster", Value: "local"},
			{Name: "fizz", Value: "buzz"},
		},
		Samples: []prompb.Sample{
			{Timestamp: sampleTimestamp, Value: 34},
		},
	}}

	var results []prompb.TimeSeries
	select {
	case <-time.After(time.Minute):
		require.FailNow(t, "timed out waiting for metrics")
	case res := <-writeResult:
		if len(res.Timeseries) == 1 {
			results = append(results, res.Timeseries[0])
		} else if len(res.Timeseries) == 2 {
			results = res.Timeseries
		}
		// When we have two results make sure they match what we expect
		if len(results) == 2 {
			require.Equal(t, expect, results)
		}
	}
}

func TestUpdate(t *testing.T) {
	writeResult := make(chan *prompb.WriteRequest)

	// Create a remote_write server which forwards any received payloads to the
	// writeResult channel.
	srv := newTestServer(t, writeResult)

	// Create the component under test and start it.
	args := testArgsForConfig(t, fmt.Sprintf(`
		external_labels = {
			cluster = "local",
		}
		endpoint {
			name           = "test-url"
			url            = "%s/api/v1/write"
			remote_timeout = "100ms"

			queue_config {
				batch_send_deadline = "100ms"
			}
		}
	`, srv.URL))
	tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "prometheus.remote_write")
	require.NoError(t, err)
	go func() {
		err = tc.Run(componenttest.TestContext(t), args)
		require.NoError(t, err)
	}()
	require.NoError(t, tc.WaitRunning(5*time.Second))

	// Use a future timestamp since remote_write will ignore any
	// sample which is earlier than the time when it started.
	sample1Time := time.Now().Add(time.Minute).UnixMilli()

	// Send a metric and assert its received
	sendMetric(t, tc, labels.FromStrings("foo", "bar"), sample1Time, 12)
	assertReceived(t, writeResult, []prompb.TimeSeries{{
		Labels: []prompb.Label{
			{Name: "cluster", Value: "local"},
			{Name: "foo", Value: "bar"},
		},
		Samples: []prompb.Sample{
			{Timestamp: sample1Time, Value: 12},
		},
	}})

	// To test the update - close the current server and create a new one
	srv.Close()
	srv = newTestServer(t, writeResult)

	// Update the component with the new server URL
	args = testArgsForConfig(t, fmt.Sprintf(`
		external_labels = {
			cluster = "another-local",
			source = "test",
		}
		endpoint {
			name           = "second-test-url"
			url            = "%s/api/v1/write"
			remote_timeout = "100ms"

			queue_config {
				batch_send_deadline = "100ms"
			}
		}
	`, srv.URL))

	require.NoError(t, tc.Update(args))

	// Send another metric after update
	sample2Time := time.Now().Add(2 * time.Minute).UnixMilli()
	sendMetric(t, tc, labels.FromStrings("fizz", "buzz"), sample2Time, 34)
	assertReceived(t, writeResult, []prompb.TimeSeries{{
		Labels: []prompb.Label{
			{Name: "cluster", Value: "another-local"},
			{Name: "foo", Value: "bar"},
			{Name: "source", Value: "test"},
		},
		Samples: []prompb.Sample{
			{Timestamp: sample1Time, Value: 12},
		}}, {
		Labels: []prompb.Label{
			{Name: "cluster", Value: "another-local"},
			{Name: "fizz", Value: "buzz"},
			{Name: "source", Value: "test"},
		},
		Samples: []prompb.Sample{
			{Timestamp: sample2Time, Value: 34},
		},
	}})
}

func assertReceived(t *testing.T, writeResult chan *prompb.WriteRequest, expect []prompb.TimeSeries) {
	select {
	case <-time.After(time.Minute):
		require.FailNow(t, "timed out waiting for metrics")
	case res := <-writeResult:
		require.Equal(t, expect, res.Timeseries)
	}
}

func newTestServer(t *testing.T, writeResult chan *prompb.WriteRequest) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := remote.DecodeWriteRequest(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		select {
		case writeResult <- req:
		default:
			require.Fail(t, "failed to send remote_write result over channel")
		}
	}))
}

func sendMetric(
	t *testing.T,
	tc *componenttest.Controller,
	labels labels.Labels,
	time int64,
	value float64,
) {

	rwExports := tc.Exports().(remotewrite.Exports)
	appender := rwExports.Receiver.Appender(t.Context())
	_, err := appender.Append(0, labels, time, value)
	require.NoError(t, err)
	require.NoError(t, appender.Commit())
}

func testArgsForConfig(t *testing.T, cfg string) remotewrite.Arguments {
	var args remotewrite.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	return args
}
