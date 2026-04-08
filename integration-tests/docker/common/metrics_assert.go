package common

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
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

// TestNameLabel is the Prometheus label used by integration tests to scope metrics to a test case.
const TestNameLabel = "test_name"

// TestNameSelector returns label matchers for PromQL containing only the test_name label.
func TestNameSelector(testName string) map[string]string {
	return map[string]string{TestNameLabel: testName}
}

// promLabelSelectorString builds a PromQL label matcher set for use in instant queries and series
// match[]. Values use equality (=) unless they have the prefix "regex:", in which case =~ is used
// and the rest of the string is the raw regex (single quotes escaped).
func promLabelSelectorString(labelSelectors map[string]string) string {
	if len(labelSelectors) == 0 {
		return "{}"
	}
	keys := slices.Sorted(maps.Keys(labelSelectors))
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		v := labelSelectors[k]
		b.WriteString(k)
		if rest, ok := strings.CutPrefix(v, "regex:"); ok {
			b.WriteString("=~'")
			b.WriteString(strings.ReplaceAll(rest, "'", `\'`))
			b.WriteByte('\'')
		} else {
			b.WriteString("='")
			b.WriteString(strings.ReplaceAll(v, "'", `\'`))
			b.WriteByte('\'')
		}
	}
	b.WriteByte('}')
	return b.String()
}

func seriesURLWithOptionalStart(pathQuery string) string {
	if startingAt := AlloyStartTimeUnix(); startingAt > 0 {
		return pathQuery + fmt.Sprintf("&start=%d", startingAt)
	}
	return pathQuery
}

// MetricQuery returns a formatted Prometheus instant query for metricName with the given label matchers.
func MetricQuery(metricName string, labelSelectors map[string]string) string {
	promql := metricName + promLabelSelectorString(labelSelectors)
	return fmt.Sprintf("%squery?query=%s", promURL, url.QueryEscape(promql))
}

// MetricsQuery returns the list of available series matching labelSelectors (e.g. from [TestNameSelector]).
func MetricsQuery(labelSelectors map[string]string) string {
	// https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers
	q := fmt.Sprintf("%sseries?match[]=%s", promURL, url.QueryEscape(promLabelSelectorString(labelSelectors)))
	return seriesURLWithOptionalStart(q)
}

// MimirMetricsTestWithLabels checks metrics in Mimir for labelSelectors, then runs per-metric subtests.
func MimirMetricsTestWithLabels(t *testing.T, metrics []string, histogramMetrics []string, labelSelectors map[string]string) {
	AssertStatefulTestEnv(t)

	AssertMetricsAvailable(t, metrics, histogramMetrics, labelSelectors)

	for _, metric := range metrics {
		metric := metric
		t.Run(metric, func(t *testing.T) {
			t.Parallel()
			query := MetricQuery(metric, labelSelectors)
			wantTestName := labelSelectors[TestNameLabel]
			waitForScalarMetric(t, query, metric, wantTestName)
		})
	}
	for _, metric := range histogramMetrics {
		metric := metric
		t.Run(metric, func(t *testing.T) {
			t.Parallel()
			query := MetricQuery(metric, labelSelectors)
			wantTestName := labelSelectors[TestNameLabel]
			waitForHistogramMetric(t, query, metric, wantTestName)
		})
	}
}

// MimirMetricsTest checks that all given metrics are stored in Mimir (no extra label assertions).
func MimirMetricsTest(t *testing.T, metrics []string, histogramMetrics []string, testName string) {
	MimirMetricsTestWithLabels(t, metrics, histogramMetrics, TestNameSelector(testName))
}

// AssertMetricsAvailable performs a Prometheus series query and expects the given metric names to
// appear for labelSelectors.
func AssertMetricsAvailable(t *testing.T, metrics []string, histogramMetrics []string, labelSelectors map[string]string) {
	var missingMetrics []string
	expectedMetrics := append(metrics, histogramMetrics...)
	query := MetricsQuery(labelSelectors)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var metricsResponse MetricsResponse
		_, err := FetchDataFromURL(query, &metricsResponse)
		assert.NoError(c, err)

		actualMetrics := make(map[string]struct{}, len(metricsResponse.Data))
		for _, metric := range metricsResponse.Data {
			actualMetrics[metric.Name] = struct{}{}
		}

		missingMetrics = findMissingMetrics(expectedMetrics, actualMetrics)

		assert.Emptyf(c, missingMetrics, "Did not find %v in received metrics %v", missingMetrics, slices.Sorted(maps.Keys(actualMetrics)))
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

// waitForHistogramMetric polls query until a histogram sample satisfies count/sum thresholds.
// wantTestName, if non-empty, is asserted on the first returned series (caller's responsibility).
func waitForHistogramMetric(t *testing.T, query, expectedMetric, wantTestName string) {
	var metricResponse MetricResponse
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		responseStr, err := FetchDataFromURL(query, &metricResponse)
		assert.NoError(c, err)
		if assert.NotEmpty(c, metricResponse.Data.Result) {
			r0 := metricResponse.Data.Result[0]
			assert.Equal(c, r0.Metric.Name, expectedMetric)
			if wantTestName != "" {
				assert.Equal(c, r0.Metric.TestName, wantTestName)
			}
			require.NotNil(c, r0.Histogram, "Histogram data was not present in query %s for the metric response %v", query, responseStr)

			histogram := r0.Histogram
			if assert.NotEmpty(c, histogram.Data.Count) {
				count, _ := strconv.Atoi(histogram.Data.Count)
				assert.Greater(c, count, 10, "Count should be at some point greater than 10.")
			}
			if assert.NotEmpty(c, histogram.Data.Sum) {
				sum, _ := strconv.ParseFloat(histogram.Data.Sum, 64)
				assert.Greater(c, sum, 10., "Sum should be at some point greater than 10.")
			}
			assert.NotEmpty(c, histogram.Data.Buckets)
			assert.Nil(c, r0.Value)
		}
	}, TestTimeoutEnv(t), DefaultRetryInterval, "Histogram data did not satisfy the conditions within the time limit")
}

// waitForScalarMetric polls query until a non-empty scalar sample exists for expectedMetric.
// wantTestName, if non-empty, is asserted on the first returned series (caller's responsibility).
func waitForScalarMetric(t *testing.T, query, expectedMetric, wantTestName string) {
	var metricResponse MetricResponse
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := FetchDataFromURL(query, &metricResponse)
		assert.NoError(c, err)
		if assert.NotEmpty(c, metricResponse.Data.Result) {
			r0 := metricResponse.Data.Result[0]
			assert.Equal(c, r0.Metric.Name, expectedMetric)
			if wantTestName != "" {
				assert.Equal(c, r0.Metric.TestName, wantTestName)
			}
			assert.NotEmpty(c, r0.Value.Value)
			assert.Nil(c, r0.Histogram)
		}
	}, TestTimeoutEnv(t), DefaultRetryInterval, "Data did not satisfy the conditions within the time limit")
}
