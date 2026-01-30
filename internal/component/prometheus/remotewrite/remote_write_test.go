package remotewrite_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

type RemoteWriteVersion int

const (
	RemoteWriteVersionV1 RemoteWriteVersion = iota
	RemoteWriteVersionV2
)

// TestSend is an integration-level test which ensures that metrics can get sent to
// a prometheus.remote_write component and forwarded to a
// remote_write-compatible server.
func TestSend(t *testing.T) {
	// We need to use a future timestamp since remote_write will ignore any
	// sample which is earlier than the time when it started. Adding a minute
	// ensures that our samples will never get ignored.
	sampleTimestamp := time.Now().Add(time.Hour).UnixMilli()

	tests := []struct {
		name            string
		rwVersion       RemoteWriteVersion
		metrics         []Appendable
		expectedSamples int
	}{
		{
			name:            "Remote write v1",
			rwVersion:       RemoteWriteVersionV1,
			expectedSamples: 2,
			metrics: []Appendable{
				&Sample{
					Labels: labels.FromStrings("foo", "bar"),
					Time:   sampleTimestamp,
					Value:  12,
				},
				&Sample{
					Labels: labels.FromStrings("fizz", "baz"),
					Time:   sampleTimestamp,
					Value:  34,
				},
			},
		},
		{
			name:            "Remote write v2 with metadata",
			expectedSamples: 6,
			rwVersion:       RemoteWriteVersionV2,
			metrics: []Appendable{
				// TODO: Add exemplars
				&Sample{
					Labels: labels.FromStrings("foo", "bar"),
					Time:   sampleTimestamp,
					Value:  12,
				},
				&Metadata{
					Labels: labels.FromStrings("foo", "bar"),
					Metadata: metadata.Metadata{
						Type: model.MetricTypeCounter,
						Help: "test metric foo",
					},
				},
				&Sample{
					Labels: labels.FromStrings("fizz", "buzz"),
					Time:   sampleTimestamp,
					Value:  34,
				},
				&Metadata{
					Labels: labels.FromStrings("fizz", "buzz"),
					Metadata: metadata.Metadata{
						Type: model.MetricTypeGauge,
						Help: "test metric fizz",
					},
				},
				&Histogram{
					Labels:    labels.FromStrings("histo", "int histogram"),
					Time:      sampleTimestamp,
					Histogram: tsdbutil.GenerateTestHistogram(12),
				},
				&Metadata{
					Labels: labels.FromStrings("histo", "int histogram"),
					Metadata: metadata.Metadata{
						Type: model.MetricTypeHistogram,
						Help: "test int histogram",
					},
				},
				&FloatHistogram{
					Labels:    labels.FromStrings("histo", "float histogram"),
					Time:      sampleTimestamp,
					Histogram: tsdbutil.GenerateTestFloatHistogram(12),
				},
				&Metadata{
					Labels: labels.FromStrings("histo", "float histogram"),
					Metadata: metadata.Metadata{
						Type: model.MetricTypeHistogram,
						Help: "test float histogram",
					},
				},
				&Histogram{
					Labels:    labels.FromStrings("histo", "int nhcb"),
					Time:      sampleTimestamp,
					Histogram: tsdbutil.GenerateTestCustomBucketsHistogram(12),
				},
				&Metadata{
					Labels: labels.FromStrings("histo", "int nhcb"),
					Metadata: metadata.Metadata{
						Type: model.MetricTypeHistogram,
						Help: "test int nhcb",
					},
				},
				&FloatHistogram{
					Labels:    labels.FromStrings("histo", "float nhcb"),
					Time:      sampleTimestamp,
					Histogram: tsdbutil.GenerateTestCustomBucketsFloatHistogram(12),
				},
				&Metadata{
					Labels: labels.FromStrings("histo", "float nhcb"),
					Metadata: metadata.Metadata{
						Type: model.MetricTypeHistogram,
						Help: "test float nhcb",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writeResult := make(chan string)

			// Create a remote_write server which forwards any received payloads to the
			// writeResult channel.
			responseStats := getResponseStats(test.metrics)
			srv := newTestServer(t, writeResult, test.rwVersion, responseStats)
			defer srv.Close()

			// Load expected response and metrics from testdata
			// Convert test name to directory name: replace spaces with underscores
			testDirName := strings.ReplaceAll(test.name, " ", "_")
			testdataDir := filepath.Join("testdata", "TestSend", testDirName)
			expectedResponseBytes, err := os.ReadFile(filepath.Join(testdataDir, "expected_response.json"))
			require.NoError(t, err)
			expectedResponse := strings.ReplaceAll(string(expectedResponseBytes), "\"__TIMESTAMP__\"", strconv.FormatInt(sampleTimestamp, 10))

			expectedMetricsBytes, err := os.ReadFile(filepath.Join(testdataDir, "expected_metrics.txt"))
			require.NoError(t, err)
			expectedMetricsBytes = normalizeLineEndings(expectedMetricsBytes)
			expectedMetrics := strings.ReplaceAll(string(expectedMetricsBytes), "__MIMIR_RW_URL__", srv.URL)
			expectedMetrics = strings.ReplaceAll(expectedMetrics, "__EXPECTED_SAMPLES__", strconv.Itoa(test.expectedSamples))

			cfgProtobufMessageAttr := ""
			if test.rwVersion == RemoteWriteVersionV2 {
				cfgProtobufMessageAttr = "protobuf_message = \"io.prometheus.write.v2.Request\""
			}

			cfg := fmt.Sprintf(`
					external_labels = {
						cluster = "local",
					}
					endpoint {
						name                   = "test-url"
						url                    = "%s/api/v1/write"
						send_native_histograms = true
						%s

						queue_config {
							// The WAL watcher should send the expected number of samples as soon as it gets them.
							// That way all samples are sent in one batch and the RW request can be tested for all samples.
							// Also, the test is not slowed down by the WAL watcher sending samples due to a timeout
							// caused by waiting for an unnecessarily large number of samples.
							max_samples_per_send = %d
							batch_send_deadline = "1m"
						}
					}
				`,
				srv.URL, cfgProtobufMessageAttr, test.expectedSamples)

			// Create our component and wait for it to start running, so we can write
			// metrics to the WAL.
			args := testArgs(t, cfg)
			tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "prometheus.remote_write")
			require.NoError(t, err)

			promRegistry := prometheus.NewRegistry()
			tc.PromRegistry = promRegistry

			go func() {
				err = tc.Run(componenttest.TestContext(t), args)
				require.NoError(t, err)
			}()
			require.NoError(t, tc.WaitRunning(5*time.Second))

			sendMetrics(t, tc, test.metrics)

			select {
			case <-time.After(120 * time.Second):
				require.FailNow(t, "timed out waiting for metrics")
			case actual := <-writeResult:
				require.JSONEq(t, expectedResponse, actual)
			}

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				// TODO: Check metrics such as these whether they are greater than 0:
				// * prometheus_remote_storage_metadata_bytes_total
				// * prometheus_remote_storage_bytes_total
				// * prometheus_remote_storage_highest_timestamp_in_seconds

				err = testutil.GatherAndCompare(
					promRegistry,
					strings.NewReader(expectedMetrics),
					"prometheus_remote_storage_histograms_failed_total",
					"prometheus_remote_storage_histograms_pending",
					"prometheus_remote_storage_histograms_total",
					"prometheus_remote_storage_metadata_failed_total",
					"prometheus_remote_storage_metadata_retried_total",
					"prometheus_remote_storage_metadata_total",
					"prometheus_remote_storage_samples_failed_total",
					"prometheus_remote_storage_samples_pending",
					"prometheus_remote_storage_samples_retried_total",
					"prometheus_remote_storage_samples_total",
					"prometheus_remote_write_wal_metadata_updates_total",
					"prometheus_remote_write_wal_samples_appended_total",
				)
				require.NoError(c, err)
			}, 10*time.Second, 100*time.Millisecond)
		})
	}
}

// The metadata should be sent again on every remote write HTTP request, even if it hasn't changed.
func TestMetadataResend_V2(t *testing.T) {
	// We need to use a future timestamp since remote_write will ignore any
	// sample which is earlier than the time when it started. Adding a minute
	// ensures that our samples will never get ignored.
	startTimestamp := time.Now().Add(time.Hour).UnixMilli()

	// Load expected response and metrics templates from testdata
	testdataDir := filepath.Join("testdata", "TestMetadataResend_V2")
	expectedResponseBytes, err := os.ReadFile(filepath.Join(testdataDir, "expected_response.json"))
	require.NoError(t, err)
	expectedResponseTemplate := string(expectedResponseBytes)

	expectedMetricsBytes, err := os.ReadFile(filepath.Join(testdataDir, "expected_metrics.txt"))
	require.NoError(t, err)
	expectedMetricsBytes = normalizeLineEndings(expectedMetricsBytes)
	expectedMetricsTemplate := string(expectedMetricsBytes)

	writeResult := make(chan string)
	responseStats := remote.WriteResponseStats{
		Samples: 1,
	}

	// Create a remote_write server which forwards any received payloads to the
	// writeResult channel.
	srv := newTestServer(t, writeResult, RemoteWriteVersionV2, &responseStats)
	defer srv.Close()

	cfg := fmt.Sprintf(`
	external_labels = {
		cluster = "local",
	}
	endpoint {
		name           = "test-url"
		url            = "%s/api/v1/write"
		send_native_histograms = true
		protobuf_message = "io.prometheus.write.v2.Request"

		queue_config {
			// The WAL watcher should send the expected number of samples as soon as it gets them.
			// That way all samples are sent in one batch and the RW request can be tested for all samples.
			// Also, the test is not slowed down by the WAL watcher sending samples due to a timeout
			// caused by waiting for an unnecessarily large number of samples.
			max_samples_per_send = 1
			batch_send_deadline = "1m"
		}
}
`,
		srv.URL)

	// Create our component and wait for it to start running, so we can write
	// metrics to the WAL.
	args := testArgs(t, cfg)
	tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "prometheus.remote_write")
	require.NoError(t, err)

	promRegistry := prometheus.NewRegistry()
	tc.PromRegistry = promRegistry

	go func() {
		err = tc.Run(componenttest.TestContext(t), args)
		require.NoError(t, err)
	}()
	require.NoError(t, tc.WaitRunning(5*time.Second))

	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprintf("Send %d", i+1), func(t *testing.T) {
			currentTimestamp := startTimestamp + int64(i*1000)
			expectedResponse := strings.ReplaceAll(expectedResponseTemplate, "\"__TIMESTAMP__\"", strconv.FormatInt(currentTimestamp, 10))

			currentValue := float64(i + 1)
			expectedResponse = strings.ReplaceAll(expectedResponse, "\"__VALUE__\"", strconv.FormatFloat(currentValue, 'f', -1, 64))

			expectedMetrics := strings.ReplaceAll(expectedMetricsTemplate, "__MIMIR_RW_URL__", srv.URL)
			expectedMetrics = strings.ReplaceAll(expectedMetrics, "__ITERATION__", strconv.Itoa(i+1))

			sendMetrics(t, tc, []Appendable{
				&Sample{
					Labels: labels.FromStrings("foo", "bar"),
					Time:   currentTimestamp,
					Value:  currentValue,
				},
				&Metadata{
					Labels: labels.FromStrings("foo", "bar"),
					Metadata: metadata.Metadata{
						Type: model.MetricTypeCounter,
						Help: "test metric foo",
					},
				}},
			)

			select {
			case <-time.After(120 * time.Second):
				require.FailNow(t, "timed out waiting for metrics")
			case actual := <-writeResult:
				require.JSONEq(t, expectedResponse, actual)
			}

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				err = testutil.GatherAndCompare(
					promRegistry,
					strings.NewReader(expectedMetrics),
					"prometheus_remote_storage_metadata_failed_total",
					"prometheus_remote_storage_metadata_retried_total",
					"prometheus_remote_storage_metadata_total",
					"prometheus_remote_storage_samples_failed_total",
					"prometheus_remote_storage_samples_pending",
					"prometheus_remote_storage_samples_retried_total",
					"prometheus_remote_storage_samples_total",
					"prometheus_remote_write_wal_metadata_updates_total",
					"prometheus_remote_write_wal_samples_appended_total",
				)
				require.NoError(c, err)
			}, 10*time.Second, 100*time.Millisecond)
		})
	}
}

func TestUpdate(t *testing.T) {
	writeResult := make(chan string)

	// Create a remote_write server which forwards any received payloads to the
	// writeResult channel.
	srv := newTestServer(t, writeResult, RemoteWriteVersionV1, nil)

	// Create the component under test and start it.
	args := testArgs(t, fmt.Sprintf(`
	external_labels = {
		cluster = "local",
	}
	endpoint {
		name           = "test-url"
		url            = "%s/api/v1/write"
		remote_timeout = "100ms"

		queue_config {
			// The WAL watcher should send the expected number of samples as soon as it gets them.
			// That way all samples are sent in one batch and the RW request can be tested for all samples.
			// Also, the test is not slowed down by the WAL watcher sending samples due to a timeout
			// caused by waiting for an unnecessarily large number of samples.
			max_samples_per_send = 1
			batch_send_deadline = "1m"
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
	sendMetrics(t, tc, []Appendable{&Sample{
		Labels: labels.FromStrings("foo", "bar"),
		Time:   sample1Time,
		Value:  12,
	}})
	assertReceived(t, writeResult, `{
    "timeseries": [
        {
            "labels": [
                {
                    "name": "cluster",
                    "value": "local"
                },
                {
                    "name": "foo",
                    "value": "bar"
                }
            ],
            "samples": [
                {
                    "value": 12,
                    "timestamp": `+strconv.FormatInt(sample1Time, 10)+`
                }
            ],
            "exemplars": null,
            "histograms": null
        }
    ],
    "metadata": null
}`)

	// To test the update - close the current server and create a new one
	srv.Close()
	srv = newTestServer(t, writeResult, RemoteWriteVersionV1, nil)

	// Update the component with the new server URL
	args = testArgs(t, fmt.Sprintf(`
	external_labels = {
		cluster = "another-local",
		source = "test",
	}
	endpoint {
		name           = "second-test-url"
		url            = "%s/api/v1/write"
		remote_timeout = "100ms"

		queue_config {
			max_samples_per_send = 2
			batch_send_deadline = "1m"
		}
	}
`, srv.URL))
	require.NoError(t, tc.Update(args))

	// Send another metric after update
	sample2Time := time.Now().Add(2 * time.Minute).UnixMilli()
	sendMetrics(t, tc, []Appendable{&Sample{
		Labels: labels.FromStrings("fizz", "buzz"),
		Time:   sample2Time,
		Value:  34,
	}})

	expected := `{
    "timeseries": [
        {
            "labels": [
                {
                    "name": "cluster",
                    "value": "another-local"
                },
                {
                    "name": "foo",
                    "value": "bar"
                },
                {
                    "name": "source",
                    "value": "test"
                }
            ],
            "samples": [
                {
                    "value": 12,
                    "timestamp": ` + strconv.FormatInt(sample1Time, 10) + `
                }
            ],
            "exemplars": null,
            "histograms": null
        },
        {
            "labels": [
                {
                    "name": "cluster",
                    "value": "another-local"
                },
                {
                    "name": "fizz",
                    "value": "buzz"
                },
                {
                    "name": "source",
                    "value": "test"
                }
            ],
            "samples": [
                {
                    "value": 34,
                    "timestamp": ` + strconv.FormatInt(sample2Time, 10) + `
                }
            ],
            "exemplars": null,
            "histograms": null
        }
    ],
    "metadata": null
}`

	assertReceived(t, writeResult, expected)
}

func assertReceived(t *testing.T, writeResult chan string, expect string) {
	select {
	case <-time.After(time.Minute):
		require.FailNow(t, "timed out waiting for metrics")
	case actual := <-writeResult:
		require.JSONEq(t, expect, actual)
	}
}

func newTestServer(t *testing.T, writeResult chan string, rwVersion RemoteWriteVersion, responseStats *remote.WriteResponseStats) *httptest.Server {
	switch rwVersion {
	case RemoteWriteVersionV1:
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req, err := remote.DecodeWriteRequest(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			reqJson, err := json.Marshal(req)
			require.NoError(t, err)

			select {
			case writeResult <- string(reqJson):
			default:
				require.Fail(t, "failed to send remote_write result over channel")
			}
		}))
	case RemoteWriteVersionV2:
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req, err := remote.DecodeWriteV2Request(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			reqJson, err := json.Marshal(req)
			require.NoError(t, err)

			select {
			case writeResult <- string(reqJson):
			default:
				require.Fail(t, "failed to send remote_write result over channel")
			}

			// If we don't set these headers, then the client will think that the write failed.
			// Then metrics such as prometheus_remote_storage_histograms_failed_total will be incremented.
			responseStats.SetHeaders(w)
		}))
	default:
		require.FailNow(t, "invalid remote write version")
		return nil
	}
}

type Appendable interface {
	Append(appender storage.Appender) error
}

type Sample struct {
	Labels labels.Labels
	Time   int64
	Value  float64
}

func (s *Sample) Append(appender storage.Appender) error {
	_, err := appender.Append(0, s.Labels, s.Time, s.Value)
	return err
}

type Metadata struct {
	Labels   labels.Labels
	Metadata metadata.Metadata
}

func (m *Metadata) Append(appender storage.Appender) error {
	_, err := appender.UpdateMetadata(0, m.Labels, m.Metadata)
	return err
}

type Histogram struct {
	Labels    labels.Labels
	Time      int64
	Histogram *histogram.Histogram
}

func (h *Histogram) Append(appender storage.Appender) error {
	_, err := appender.AppendHistogram(0, h.Labels, h.Time, h.Histogram, nil)
	return err
}

type FloatHistogram struct {
	Labels    labels.Labels
	Time      int64
	Histogram *histogram.FloatHistogram
}

func (fh *FloatHistogram) Append(appender storage.Appender) error {
	_, err := appender.AppendHistogram(0, fh.Labels, fh.Time, nil, fh.Histogram)
	return err
}

func getResponseStats(metrics []Appendable) *remote.WriteResponseStats {
	responseStats := remote.WriteResponseStats{}
	for _, metric := range metrics {
		switch metric.(type) {
		case *Sample:
			responseStats.Samples++
		case *Histogram:
			responseStats.Histograms++
		case *FloatHistogram:
			responseStats.Histograms++
		}
	}
	return &responseStats
}

func sendMetrics(
	t *testing.T,
	tc *componenttest.Controller,
	metrics []Appendable,
) {

	rwExports := tc.Exports().(remotewrite.Exports)
	appender := rwExports.Receiver.Appender(t.Context())

	for _, metric := range metrics {
		require.NoError(t, metric.Append(appender))
	}

	require.NoError(t, appender.Commit())
}

func testArgs(t *testing.T, cfg string) remotewrite.Arguments {
	var args remotewrite.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	return args
}

// normalizeLineEndings will replace '\r\n' with '\n'.
func normalizeLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte{'\r', '\n'}, []byte{'\n'})
}
