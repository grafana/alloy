package common

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const promURL = "http://localhost:9009/prometheus/api/v1/"

// Default metrics list according to what the prom-gen app is generating.
var PromDefaultMetrics = []string{
	"golang_counter",
	"golang_gauge",
	"golang_histogram_bucket",
	"golang_summary",
}

// Default histogram metrics list according to what the prom-gen app is generating.
var PromDefaultHistogramMetric = []string{
	"golang_native_histogram",
}

// Default metrics list according to what the otel-metrics-gen app is generating.
var OtelDefaultMetrics = []string{
	"example_counter",
	"example_float_counter",
	"example_updowncounter",
	"example_float_updowncounter",
	"example_histogram_bucket",
	"example_float_histogram_bucket",
}

// Default histogram metrics list according to what the otel-metrics-gen app is generating.
var OtelDefaultHistogramMetrics = []string{
	"example_exponential_histogram",
	"example_exponential_float_histogram",
}

// MetricQuery returns a formatted Prometheus metric query with a given metricName and the given test_name label.
func MetricQuery(metricName string, testName string) string {
	return fmt.Sprintf("%squery?query=%s{test_name='%s'}", promURL, metricName, testName)
}

// MetricsQuery returns the list of available metrics matching the given test_name label.
func MetricsQuery(testName string) string {
	return fmt.Sprintf("%sseries?match[]={test_name='%s'}", promURL, testName)
}

// MimirMetricsTest checks that all given metrics are stored in Mimir.
func MimirMetricsTest(t *testing.T, metrics []string, histogramMetrics []string, testName string) {
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
		err := FetchDataFromURL(query, &metricsResponse)
		assert.NoError(c, err)
		missingMetrics = checkMissingMetrics(expectedMetrics, metricsResponse.Data)
		msg := fmt.Sprintf("Some metrics are missing: %v", missingMetrics)
		if len(missingMetrics) == len(expectedMetrics) {
			msg = "All metrics are missing"
		}
		assert.Empty(c, missingMetrics, msg)
	}, DefaultTimeout, DefaultRetryInterval)
}

// checkMissingMetrics returns the expectedMetrics which are not contained in actualMetrics.
func checkMissingMetrics(expectedMetrics []string, actualMetrics []Metric) []string {
	metricSet := make(map[string]struct{}, len(actualMetrics))
	for _, metric := range actualMetrics {
		metricSet[metric.Name] = struct{}{}
	}

	var missingMetrics []string
	for _, expectedMetric := range expectedMetrics {
		if _, exists := metricSet[expectedMetric]; !exists {
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
		err := FetchDataFromURL(query, &metricResponse)
		assert.NoError(c, err)
		if assert.NotEmpty(c, metricResponse.Data.Result) {
			assert.Equal(c, metricResponse.Data.Result[0].Metric.Name, expectedMetric)
			assert.Equal(c, metricResponse.Data.Result[0].Metric.TestName, testName)
			if assert.NotNil(c, metricResponse.Data.Result[0].Histogram) {
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
		}
	}, DefaultTimeout, DefaultRetryInterval, "Histogram data did not satisfy the conditions within the time limit")
}

// AssertMetricData performs a Prometheus query and expect the result to eventually contain the expected metric.
func AssertMetricData(t *testing.T, query, expectedMetric string, testName string) {
	var metricResponse MetricResponse
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		err := FetchDataFromURL(query, &metricResponse)
		assert.NoError(c, err)
		if assert.NotEmpty(c, metricResponse.Data.Result) {
			assert.Equal(c, metricResponse.Data.Result[0].Metric.Name, expectedMetric)
			assert.Equal(c, metricResponse.Data.Result[0].Metric.TestName, testName)
			assert.NotEmpty(c, metricResponse.Data.Result[0].Value.Value)
			assert.Nil(c, metricResponse.Data.Result[0].Histogram)
		}
	}, DefaultTimeout, DefaultRetryInterval, "Data did not satisfy the conditions within the time limit")
}
