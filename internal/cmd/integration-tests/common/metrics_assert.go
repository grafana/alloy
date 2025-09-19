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

type Config struct {
	T                *testing.T
	TestName         string
	Metrics          []string
	HistogramMetrics []string
	ExpectedMetadata map[string]Metadata
}

// Default metrics list according to what the prom-gen app is generating.
var PromDefaultMetrics = []string{
	"golang_counter",
	"golang_gauge",
	"golang_histogram_bucket",
	"golang_histogram_count",
	"golang_histogram_sum",
	"golang_summary",
}

// Default native histogram metrics list according to what the prom-gen app is generating.
var PromDefaultNativeHistogramMetrics = []string{
	"golang_native_histogram",
	"golang_mixed_histogram",
	"golang_summary",
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

var ExpectedMetadata = map[string]Metadata{
	"golang_counter": {
		Type: "counter",
		Help: "The counter description string",
	},
	"golang_gauge": {
		Type: "gauge",
		Help: "The gauge description string",
	},
	"golang_histogram_bucket": {
		Type: "histogram",
		Help: "The histogram description string",
	},
	"golang_mixed_histogram": {
		// This is the native histogram -
		// it doesn't have a _bucket, _sum, or _count suffix.
		Type: "histogram",
		Help: "The mixed_histogram description string",
	},
	"golang_mixed_histogram_bucket": {
		Type: "histogram",
		Help: "The mixed_histogram description string",
	},
	"golang_mixed_histogram_count": {
		Type: "histogram",
		Help: "The mixed_histogram description string",
	},
	"golang_mixed_histogram_sum": {
		Type: "histogram",
		Help: "The mixed_histogram description string",
	},
	"golang_summary": {
		Type: "summary",
		Help: "The summary description string",
	},
	"golang_summary_count": {
		Type: "summary",
		Help: "The summary description string",
	},
	"golang_summary_sum": {
		Type: "summary",
		Help: "The summary description string",
	},
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

// MetadataQuery returns the list of available metadata matching the given test_name label.
func MetadataQuery(testName string) string {
	query := fmt.Sprintf("%smetadata", promURL)
	return query
}

// MimirMetricsTest checks that all given metrics are stored in Mimir.
func MimirMetricsTest(c Config) {
	AssertStatefulTestEnv(c.T)

	AssertMetadataAvailable(c.T, c.Metrics, c.HistogramMetrics, c.ExpectedMetadata, c.TestName)
	AssertMetricsAvailable(c.T, c.Metrics, c.HistogramMetrics, c.TestName)
	for _, metric := range c.Metrics {
		metric := metric
		c.T.Run(metric, func(t *testing.T) {
			t.Parallel()
			AssertMetricData(t, MetricQuery(metric, c.TestName), metric, c.TestName)
		})
	}
	for _, metric := range c.HistogramMetrics {
		metric := metric
		c.T.Run(metric, func(t *testing.T) {
			t.Parallel()
			AssertHistogramData(t, MetricQuery(metric, c.TestName), metric, c.TestName)
		})
	}
}

func AssertMetadataAvailable(t *testing.T, metrics []string, histogramMetrics []string, expectedMetadata map[string]Metadata, testName string) {
	query := MetadataQuery(testName)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var missingMetadata []string // Reset the slice on each retry
		var metadataResponse MetadataResponse
		err := FetchDataFromURL(query, &metadataResponse)
		assert.NoError(c, err)

		for metric, metadata := range expectedMetadata {
			metadataList, exists := metadataResponse.Data[metric]
			if !exists || len(metadataList) == 0 || !checkMetadata(metadata, metadataList[0]) {
				missingMetadata = append(missingMetadata, metric)
			}
		}
		assert.Empty(c, missingMetadata,
			"err", err,
			"missing metadata", missingMetadata,
			"received metadata", metadataResponse.String())
	}, DefaultTimeout, DefaultRetryInterval)
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

func checkMetadata(expectedMetadata Metadata, actualMetadata Metadata) bool {
	if expectedMetadata.Type != actualMetadata.Type {
		return false
	}
	if expectedMetadata.Help != actualMetadata.Help {
		return false
	}
	if expectedMetadata.Unit != actualMetadata.Unit {
		return false
	}
	return true
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
			require.NotNil(c, metricResponse.Data.Result[0].Histogram, "Histogram data was not present on the metric result %v", metricResponse.Data.Result[0])

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
		err := FetchDataFromURL(query, &metricResponse)
		assert.NoError(c, err)
		if assert.NotEmpty(c, metricResponse.Data.Result) {
			assert.Equal(c, metricResponse.Data.Result[0].Metric.Name, expectedMetric)
			assert.Equal(c, metricResponse.Data.Result[0].Metric.TestName, testName)
			assert.NotEmpty(c, metricResponse.Data.Result[0].Value.Value)
			assert.Nil(c, metricResponse.Data.Result[0].Histogram)
		}
	}, TestTimeoutEnv(t), DefaultRetryInterval, "Data did not satisfy the conditions within the time limit")
}
