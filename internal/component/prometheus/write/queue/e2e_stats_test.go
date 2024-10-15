package queue

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

const remoteSamples = "prometheus_remote_storage_samples_total"
const remoteHistograms = "prometheus_remote_storage_histograms_total"
const remoteMetadata = "prometheus_remote_storage_metadata_total"

const sentBytes = "prometheus_remote_storage_sent_bytes_total"
const sentMetadataBytes = "prometheus_remote_storage_metadata_bytes_total"

const outTimestamp = "prometheus_remote_storage_queue_highest_sent_timestamp_seconds"
const inTimestamp = "prometheus_remote_storage_highest_timestamp_in_seconds"

const failedSample = "prometheus_remote_storage_samples_failed_total"
const failedHistogram = "prometheus_remote_storage_histograms_failed_total"
const failedMetadata = "prometheus_remote_storage_metadata_failed_total"

const retriedSamples = "prometheus_remote_storage_samples_retried_total"
const retriedHistogram = "prometheus_remote_storage_histograms_retried_total"
const retriedMetadata = "prometheus_remote_storage_metadata_retried_total"

const prometheusDuration = "prometheus_remote_storage_queue_duration_seconds"

const serializerIncoming = "alloy_queue_series_serializer_incoming_signals"
const alloySent = "alloy_queue_series_network_sent"
const alloySerializerIncoming = "alloy_queue_series_serializer_incoming_timestamp_seconds"
const alloyNetworkDuration = "alloy_queue_series_network_duration_seconds"
const alloyFailures = "alloy_queue_series_network_failed"
const alloyRetries = "alloy_queue_series_network_retried"
const alloy429 = "alloy_queue_series_network_retried_429"

const alloyMetadataDuration = "alloy_queue_metadata_network_duration_seconds"
const alloyMetadataSent = "alloy_queue_metadata_network_sent"
const alloyMetadataFailed = "alloy_queue_metadata_network_failed"
const alloyMetadataRetried429 = "alloy_queue_metadata_network_retried_429"
const alloyMetadataRetried = "alloy_queue_metadata_network_retried"

const alloyNetworkTimestamp = "alloy_queue_series_network_timestamp_seconds"

// TestMetadata is the large end to end testing for the queue based wal, specifically for metadata.
func TestMetadata(t *testing.T) {
	// Check assumes you are checking for any value that is not 0.
	// The test at the end will see if there are any values that were not 0.
	tests := []statsTest{
		// Metadata Tests
		{
			name:             "metadata success",
			returnStatusCode: http.StatusOK,
			dtype:            Metadata,
			checks: []check{
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name:  remoteMetadata,
					value: 10,
				},
				{
					name:      sentMetadataBytes,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyMetadataDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:  alloyMetadataSent,
					value: 10,
				},
			},
		},
		{
			name:             "metadata failure",
			returnStatusCode: http.StatusBadRequest,
			dtype:            Metadata,
			checks: []check{
				{
					name:  alloyMetadataFailed,
					value: 10,
				},
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name:  failedMetadata,
					value: 10,
				},
				{
					name:      alloyMetadataDuration,
					valueFunc: greaterThenZero,
				},
			},
		},
		{
			name:             "metadata retry",
			returnStatusCode: http.StatusTooManyRequests,
			dtype:            Metadata,
			checks: []check{
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name: retriedMetadata,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyMetadataDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyMetadataRetried,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyMetadataRetried429,
					valueFunc: greaterThenZero,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runE2eStats(t, test)
		})
	}

}

// TestMetrics is the large end to end testing for the queue based wal.
func TestMetrics(t *testing.T) {
	// Check assumes you are checking for any value that is not 0.
	// The test at the end will see if there are any values that were not 0.
	tests := []statsTest{
		// Sample Tests
		{
			name:             "sample success",
			returnStatusCode: http.StatusOK,
			dtype:            Sample,
			checks: []check{
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name:  remoteSamples,
					value: 10,
				},
				{
					name:  alloySent,
					value: 10,
				},
				{
					name:      prometheusDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyNetworkDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloySerializerIncoming,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      sentBytes,
					valueFunc: greaterThenZero,
				},
				{
					name:      outTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      inTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      alloyNetworkTimestamp,
					valueFunc: greaterThenZero,
				},
			},
		},
		{
			name:             "sample failure",
			returnStatusCode: http.StatusBadRequest,
			dtype:            Sample,
			checks: []check{
				{
					name:  alloyFailures,
					value: 10,
				},
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name:  failedSample,
					value: 10,
				},
				{
					name:      prometheusDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyNetworkDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloySerializerIncoming,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      inTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
			},
		},
		{
			name:             "sample retry",
			returnStatusCode: http.StatusTooManyRequests,
			dtype:            Sample,
			checks: []check{
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name: retriedSamples,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name: alloyRetries,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name: alloy429,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name:      prometheusDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyNetworkDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloySerializerIncoming,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      inTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
			},
		},
		// histograms
		{
			name:             "histogram success",
			returnStatusCode: http.StatusOK,
			dtype:            Histogram,
			checks: []check{
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name:  remoteHistograms,
					value: 10,
				},
				{
					name:  alloySent,
					value: 10,
				},
				{
					name:      prometheusDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyNetworkDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloySerializerIncoming,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      sentBytes,
					valueFunc: greaterThenZero,
				},
				{
					name:      outTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      inTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      alloyNetworkTimestamp,
					valueFunc: greaterThenZero,
				},
			},
		},
		{
			name:             "histogram failure",
			returnStatusCode: http.StatusBadRequest,
			dtype:            Histogram,
			checks: []check{
				{
					name:  alloyFailures,
					value: 10,
				},
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name:  failedHistogram,
					value: 10,
				},
				{
					name:      prometheusDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyNetworkDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloySerializerIncoming,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      inTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
			},
		},
		{
			name:             "histogram retry",
			returnStatusCode: http.StatusTooManyRequests,
			dtype:            Histogram,
			checks: []check{
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name: retriedHistogram,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name: alloyRetries,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name: alloy429,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name:      prometheusDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyNetworkDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloySerializerIncoming,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      inTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
			},
		},
		// exemplar, note that once it hits the appender exemplars are treated the same as series.
		{
			name:             "exemplar success",
			returnStatusCode: http.StatusOK,
			dtype:            Exemplar,
			checks: []check{
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name:  remoteSamples,
					value: 10,
				},
				{
					name:  alloySent,
					value: 10,
				},
				{
					name:      prometheusDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyNetworkDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloySerializerIncoming,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      sentBytes,
					valueFunc: greaterThenZero,
				},
				{
					name:      outTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      inTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      alloyNetworkTimestamp,
					valueFunc: greaterThenZero,
				},
			},
		},
		{
			name:             "exemplar failure",
			returnStatusCode: http.StatusBadRequest,
			dtype:            Exemplar,
			checks: []check{
				{
					name:  alloyFailures,
					value: 10,
				},
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name:  failedSample,
					value: 10,
				},
				{
					name:      prometheusDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyNetworkDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloySerializerIncoming,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      inTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
			},
		},
		{
			name:             "exemplar retry",
			returnStatusCode: http.StatusTooManyRequests,
			dtype:            Exemplar,
			checks: []check{
				{
					name:  serializerIncoming,
					value: 10,
				},
				{
					name: retriedSamples,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name: alloyRetries,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name: alloy429,
					// This will be more than 10 since it retries in a loop.
					valueFunc: greaterThenZero,
				},
				{
					name:      prometheusDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloyNetworkDuration,
					valueFunc: greaterThenZero,
				},
				{
					name:      alloySerializerIncoming,
					valueFunc: isReasonableTimeStamp,
				},
				{
					name:      inTimestamp,
					valueFunc: isReasonableTimeStamp,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runE2eStats(t, test)
		})
	}

}

func greaterThenZero(v float64) bool {
	return v > 0
}

func isReasonableTimeStamp(v float64) bool {
	if v < 0 {
		return false
	}
	unixTime := time.Unix(int64(v), 0)

	return time.Since(unixTime) < 10*time.Second
}

type dataType int

const (
	Sample dataType = iota
	Histogram
	Exemplar
	Metadata
)

type check struct {
	name      string
	value     float64
	valueFunc func(v float64) bool
}
type statsTest struct {
	name             string
	returnStatusCode int
	// Only check for non zero values, once all checks are ran it will automatically ensure all remaining metrics are 0.
	checks []check
	dtype  dataType
}

func runE2eStats(t *testing.T, test statsTest) {
	l := util.TestAlloyLogger(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(test.returnStatusCode)
	}))
	expCh := make(chan Exports, 1)

	reg := prometheus.NewRegistry()
	c, err := newComponent(t, l, srv.URL, expCh, reg)
	require.NoError(t, err)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		runErr := c.Run(ctx)
		require.NoError(t, runErr)
	}()
	// Wait for export to spin up.
	exp := <-expCh

	index := 0

	go func() {
		app := exp.Receiver.Appender(ctx)
		for j := 0; j < 10; j++ {
			index++
			switch test.dtype {
			case Sample:
				ts, v, lbls := makeSeries(index)
				_, errApp := app.Append(0, lbls, ts, v)
				require.NoError(t, errApp)
			case Histogram:
				ts, lbls, h := makeHistogram(index)
				_, errApp := app.AppendHistogram(0, lbls, ts, h, nil)
				require.NoError(t, errApp)
			case Exemplar:
				ex := makeExemplar(index)
				_, errApp := app.AppendExemplar(0, nil, ex)
				require.NoError(t, errApp)
			case Metadata:
				md, lbls := makeMetadata(index)
				_, errApp := app.UpdateMetadata(0, lbls, md)
				require.NoError(t, errApp)
			default:
				require.True(t, false)
			}
		}
		require.NoError(t, app.Commit())
	}()
	tm := time.NewTimer(8 * time.Second)
	<-tm.C
	cancel()

	require.Eventually(t, func() bool {
		dtos, gatherErr := reg.Gather()
		require.NoError(t, gatherErr)
		// Check if we have some valid metrics.
		found := 0
		for _, d := range dtos {
			if getValue(d) > 0 {
				found++
			}
		}
		// Make sure we have a few metrics.
		return found > 1
	}, 10*time.Second, 1*time.Second)
	metrics := make(map[string]float64)
	dtos, err := reg.Gather()
	require.NoError(t, err)
	// Get the value of metrics.
	for _, d := range dtos {
		metrics[*d.Name] = getValue(d)
	}

	// Check for the metrics that matter.
	for _, valChk := range test.checks {
		// These check functions will return the list of metrics with the one checked for deleted.
		// Ideally at the end we should only be left with metrics with a value of zero.s
		if valChk.valueFunc != nil {
			metrics = checkValueCondition(t, valChk.name, valChk.valueFunc, metrics)
		} else {
			metrics = checkValue(t, valChk.name, valChk.value, metrics)
		}
	}
	// all other metrics should be zero.
	for k, v := range metrics {
		require.Zerof(t, v, "%s should be zero", k)
	}
}

func getValue(d *dto.MetricFamily) float64 {
	switch *d.Type {
	case dto.MetricType_COUNTER:
		return d.Metric[0].Counter.GetValue()
	case dto.MetricType_GAUGE:
		return d.Metric[0].Gauge.GetValue()
	case dto.MetricType_SUMMARY:
		return d.Metric[0].Summary.GetSampleSum()
	case dto.MetricType_UNTYPED:
		return d.Metric[0].Untyped.GetValue()
	case dto.MetricType_HISTOGRAM:
		return d.Metric[0].Histogram.GetSampleSum()
	case dto.MetricType_GAUGE_HISTOGRAM:
		return d.Metric[0].Histogram.GetSampleSum()
	default:
		panic("unknown type " + d.Type.String())
	}
}

func checkValue(t *testing.T, name string, value float64, metrics map[string]float64) map[string]float64 {
	v, ok := metrics[name]
	require.Truef(t, ok, "invalid metric name %s", name)
	require.Equalf(t, value, v, "%s should be %f", name, value)
	delete(metrics, name)
	return metrics
}

func checkValueCondition(t *testing.T, name string, chk func(float64) bool, metrics map[string]float64) map[string]float64 {
	v, ok := metrics[name]
	require.Truef(t, ok, "invalid metric name %s", name)
	require.Truef(t, chk(v), "false test for metric name %s", name)
	delete(metrics, name)
	return metrics
}
