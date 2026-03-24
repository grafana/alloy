package common

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

const promURL = "http://localhost:9009/prometheus/api/v1/"

// Default metrics list according to what the prom-gen app is generating.
var PromDefaultMetrics = []string{
	"golang_counter",
	"golang_gauge",
	"golang_histogram_bucket",
	"golang_histogram_count",
	"golang_histogram_sum",
	"golang_mixed_histogram_bucket",
	"golang_mixed_histogram_count",
	"golang_mixed_histogram_sum",
	"golang_summary",
}

// Default native histogram metrics list according to what the prom-gen app is generating.
var PromDefaultNativeHistogramMetrics = []string{
	"golang_native_histogram",
	"golang_mixed_histogram",
}

// Default metrics list according to what the otel-gen app is generating.
var OtelDefaultMetrics = []string{
	"example_counter",
	"example_float_counter",
	"example_updowncounter",
	"example_float_updowncounter",
	"example_histogram_bucket",
	"example_float_histogram_bucket",
}

// Default histogram metrics list according to what the otel-gen app is generating.
var OtelDefaultHistogramMetrics = []string{
	"example_exponential_histogram",
	"example_exponential_float_histogram",
}

// MetricQuery returns a formatted Prometheus metric query with a given metricName and the given test_name label.
func MetricQuery(metricName string, testName string) string {
	// https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries
	return fmt.Sprintf("%squery?query=%s{test_name='%s'}", promURL, metricName, testName)
}

// MetricsQuery returns the list of available metrics matching the given test_name label.
func MetricsQuery(testName string) string {
	// https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers
	query := fmt.Sprintf("%sseries?match[]={test_name='%s'}", promURL, testName)
	if startingAt := AlloyStartTimeUnix(); startingAt > 0 {
		query += fmt.Sprintf("&start=%d", startingAt)
	}
	return query
}

// MimirMetricsTest checks that all given metrics are stored in Mimir.
func MimirMetricsTest(t *testing.T, metrics []string, histogramMetrics []string, testName string) {
	AssertStatefulTestEnv(t)

	AssertMetricsAvailable(t, metrics, histogramMetrics, testName)
	for _, metric := range metrics {
		metric := metric
		t.Run(metric, func(t *testing.T) {
			t.Parallel()
			AssertMetricData(t, MetricQuery(metric, testName), metric, testName)
		})
	}
	for _, metric := range histogramMetrics {
		metric := metric
		t.Run(metric, func(t *testing.T) {
			t.Parallel()
			AssertHistogramData(t, MetricQuery(metric, testName), metric, testName)
		})
	}
}

// AssertMetricsAvailable performs a Prometheus query and expect the result to eventually contain the list of expected metrics.
func AssertMetricsAvailable(t *testing.T, metrics []string, histogramMetrics []string, testName string) {
	var missingMetrics []string
	expectedMetrics := append(metrics, histogramMetrics...)
	query := MetricsQuery(testName)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var metricsResponse MetricsResponse
		_, err := FetchDataFromURL(query, &metricsResponse)
		assert.NoError(c, err)

		actualMetrics := make(map[string]struct{}, len(metricsResponse.Data))
		for _, metric := range metricsResponse.Data {
			actualMetrics[metric.Name] = struct{}{}
		}

		missingMetrics = findMissingMetrics(expectedMetrics, actualMetrics)

		assert.Emptyf(c, missingMetrics, "Did not find %v in received metrics %v", missingMetrics, maps.Keys(actualMetrics))
	}, TestTimeoutEnv(t), DefaultRetryInterval)
}

// findMissingMetrics returns the expectedMetrics which are not contained in actualMetrics.
func findMissingMetrics(expectedMetrics []string, actualMetrics map[string]struct{}) []string {
	var missingMetrics []string
	for _, expectedMetric := range expectedMetrics {
		if _, exists := actualMetrics[expectedMetric]; !exists {
			missingMetrics = append(missingMetrics, expectedMetric)
		}
	}
	return missingMetrics
}

// AssertHistogramData performs a Prometheus query and expect the result to eventually contain the expected histogram.
// The count and sum metrics should be greater than 10 before the timeout triggers.
func AssertHistogramData(t *testing.T, query string, expectedMetric string, testName string) {
	var metricResponse MetricResponse
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		responseStr, err := FetchDataFromURL(query, &metricResponse)
		assert.NoError(c, err)
		if assert.NotEmpty(c, metricResponse.Data.Result) {
			assert.Equal(c, metricResponse.Data.Result[0].Metric.Name, expectedMetric)
			assert.Equal(c, metricResponse.Data.Result[0].Metric.TestName, testName)
			require.NotNil(c, metricResponse.Data.Result[0].Histogram, "Histogram data was not present in query %s for the metric response %v", query, responseStr)

			histogram := metricResponse.Data.Result[0].Histogram
			if assert.NotEmpty(c, histogram.Data.Count) {
				count, _ := strconv.Atoi(histogram.Data.Count)
				assert.Greater(c, count, 10, "Count should be at some point greater than 10.")
			}
			if assert.NotEmpty(c, histogram.Data.Sum) {
				sum, _ := strconv.ParseFloat(histogram.Data.Sum, 64)
				assert.Greater(c, sum, 10., "Sum should be at some point greater than 10.")
			}
			assert.NotEmpty(c, histogram.Data.Buckets)
			assert.Nil(c, metricResponse.Data.Result[0].Value)
		}
	}, TestTimeoutEnv(t), DefaultRetryInterval, "Histogram data did not satisfy the conditions within the time limit")
}

// AssertMetricData performs a Prometheus query and expect the result to eventually contain the expected metric.
func AssertMetricData(t *testing.T, query, expectedMetric string, testName string) {
	var metricResponse MetricResponse
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := FetchDataFromURL(query, &metricResponse)
		assert.NoError(c, err)
		if assert.NotEmpty(c, metricResponse.Data.Result) {
			assert.Equal(c, metricResponse.Data.Result[0].Metric.Name, expectedMetric)
			assert.Equal(c, metricResponse.Data.Result[0].Metric.TestName, testName)
			assert.NotEmpty(c, metricResponse.Data.Result[0].Value.Value)
			assert.Nil(c, metricResponse.Data.Result[0].Histogram)
		}
	}, TestTimeoutEnv(t), DefaultRetryInterval, "Data did not satisfy the conditions within the time limit")
}
