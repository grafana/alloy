package remotecfg

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterMetrics(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	// Register metrics
	m := registerMetrics(reg)

	// Verify the metrics struct is not nil
	require.NotNil(t, m)

	// Verify all metric fields are initialized
	assert.NotNil(t, m.lastLoadSuccess)
	assert.NotNil(t, m.lastFetchNotModified)
	assert.NotNil(t, m.totalFailures)
	assert.NotNil(t, m.configHash)
	assert.NotNil(t, m.lastFetchSuccessTime)
	assert.NotNil(t, m.totalAttempts)
	assert.NotNil(t, m.getConfigTime)
}

func TestMetricsRegistration(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	// Register metrics
	m := registerMetrics(reg)

	// Set some values to ensure metrics appear in registry
	m.configHash.WithLabelValues("test_hash").Set(1)

	// Gather metrics from registry
	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	// Expected metric names
	expectedMetrics := []string{
		"remotecfg_hash",
		"remotecfg_last_load_successful",
		"remotecfg_last_load_not_modified",
		"remotecfg_load_failures_total",
		"remotecfg_load_attempts_total",
		"remotecfg_last_load_success_timestamp_seconds",
		"remotecfg_request_duration_seconds",
	}

	// Check that all expected metrics are registered
	foundMetrics := make(map[string]bool)
	for _, mf := range metricFamilies {
		foundMetrics[mf.GetName()] = true
	}

	for _, expectedMetric := range expectedMetrics {
		assert.True(t, foundMetrics[expectedMetric], "Metric %s should be registered", expectedMetric)
	}

	// Verify we have the expected number of metrics
	assert.Len(t, foundMetrics, len(expectedMetrics), "Should have exactly %d metrics registered", len(expectedMetrics))

	// Verify metrics are functional by setting some values
	m.lastLoadSuccess.Set(1)
	m.totalFailures.Inc()
	m.configHash.WithLabelValues("test_hash").Set(1)
}

func TestMetricNames(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := registerMetrics(reg)

	// Set config hash value to ensure it appears in registry
	m.configHash.WithLabelValues("test_hash").Set(1)

	// Test individual metric names by gathering and checking
	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	metricNames := make(map[string]string) // name -> help text
	for _, mf := range metricFamilies {
		metricNames[mf.GetName()] = mf.GetHelp()
	}

	// Verify specific metrics exist with correct help text
	expectedMetrics := map[string]string{
		"remotecfg_hash":                                "Hash of the currently active remote configuration.",
		"remotecfg_last_load_successful":                "Remote config loaded successfully",
		"remotecfg_last_load_not_modified":              "Remote config not modified since last fetch",
		"remotecfg_load_failures_total":                 "Remote configuration load failures",
		"remotecfg_load_attempts_total":                 "Attempts to load remote configuration",
		"remotecfg_last_load_success_timestamp_seconds": "Timestamp of the last successful remote configuration load",
		"remotecfg_request_duration_seconds":            "Duration of remote configuration requests.",
	}

	for expectedName, expectedHelp := range expectedMetrics {
		actualHelp, found := metricNames[expectedName]
		assert.True(t, found, "Metric %s should exist", expectedName)
		assert.Equal(t, expectedHelp, actualHelp, "Help text for %s should match", expectedName)
	}

	// Test that config hash has the expected label
	m.configHash.WithLabelValues("test_hash").Set(1)

	// Verify the config hash metric has the right labels
	configHashFound := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "remotecfg_hash" {
			configHashFound = true
			metrics := mf.GetMetric()
			require.Len(t, metrics, 1, "Should have one config hash metric instance")

			labels := metrics[0].GetLabel()
			require.Len(t, labels, 1, "Config hash should have one label")
			assert.Equal(t, "hash", labels[0].GetName())
			assert.Equal(t, "test_hash", labels[0].GetValue())
		}
	}
	assert.True(t, configHashFound, "Config hash metric should be found")
}

func TestMetricTypes(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := registerMetrics(reg)

	// Set some values to ensure metrics are created
	m.lastLoadSuccess.Set(1)
	m.lastFetchNotModified.Set(0)
	m.totalFailures.Inc()
	m.totalAttempts.Add(5)
	m.configHash.WithLabelValues("abc123").Set(1)
	m.lastFetchSuccessTime.SetToCurrentTime()
	m.getConfigTime.Observe(0.5)

	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	metricTypes := make(map[string]string)
	for _, mf := range metricFamilies {
		metricTypes[mf.GetName()] = mf.GetType().String()
	}

	// Verify metric types
	expectedTypes := map[string]string{
		"remotecfg_hash":                                "GAUGE",
		"remotecfg_last_load_successful":                "GAUGE",
		"remotecfg_last_load_not_modified":              "GAUGE",
		"remotecfg_load_failures_total":                 "COUNTER",
		"remotecfg_load_attempts_total":                 "COUNTER",
		"remotecfg_last_load_success_timestamp_seconds": "GAUGE",
		"remotecfg_request_duration_seconds":            "HISTOGRAM",
	}

	for metricName, expectedType := range expectedTypes {
		actualType, found := metricTypes[metricName]
		assert.True(t, found, "Metric %s should exist", metricName)
		assert.Equal(t, expectedType, actualType, "Metric %s should be of type %s", metricName, expectedType)
	}
}

func TestMetricFunctionality(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := registerMetrics(reg)

	// Test Gauge functionality
	m.lastLoadSuccess.Set(1)
	value := testutil.ToFloat64(m.lastLoadSuccess)
	assert.Equal(t, float64(1), value)

	m.lastLoadSuccess.Set(0)
	value = testutil.ToFloat64(m.lastLoadSuccess)
	assert.Equal(t, float64(0), value)

	// Test Counter functionality
	initialValue := testutil.ToFloat64(m.totalFailures)
	m.totalFailures.Inc()
	newValue := testutil.ToFloat64(m.totalFailures)
	assert.Equal(t, initialValue+1, newValue)

	m.totalAttempts.Add(5)
	attemptsValue := testutil.ToFloat64(m.totalAttempts)
	assert.Equal(t, float64(5), attemptsValue)

	// Test GaugeVec functionality
	m.configHash.WithLabelValues("hash1").Set(1)
	m.configHash.WithLabelValues("hash2").Set(1)

	// Reset all config hash metrics
	m.configHash.Reset()

	// After reset, add a new one
	m.configHash.WithLabelValues("hash3").Set(1)

	// Test Histogram functionality
	m.getConfigTime.Observe(0.1)
	m.getConfigTime.Observe(0.5)
	m.getConfigTime.Observe(1.0)

	// Verify histogram functionality by ensuring no panics occur
	// Detailed histogram testing is done in TestHistogramBuckets
	assert.NotNil(t, m.getConfigTime, "Histogram should not be nil")
}

func TestMetricsWithNilRegistry(t *testing.T) {
	// Test that registerMetrics handles nil registry gracefully
	// Note: promauto.With(nil) should work and use default registry
	m := registerMetrics(nil)

	// Verify the metrics struct is created
	require.NotNil(t, m)
	assert.NotNil(t, m.lastLoadSuccess)
	assert.NotNil(t, m.totalFailures)

	// These should not panic
	m.lastLoadSuccess.Set(1)
	m.totalFailures.Inc()
}

func TestMetricsOutput(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := registerMetrics(reg)

	// Set up some metric values
	m.lastLoadSuccess.Set(1)
	m.lastFetchNotModified.Set(0)
	m.totalFailures.Add(3)
	m.totalAttempts.Add(10)
	m.configHash.WithLabelValues("abc123").Set(1)
	m.getConfigTime.Observe(0.25)

	// Get the metrics output
	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	// Verify we can gather metrics without errors
	assert.NotEmpty(t, metricFamilies)

	// Test that we can get string representation of metrics
	for _, mf := range metricFamilies {
		assert.NotEmpty(t, mf.GetName())
		assert.NotEmpty(t, mf.GetHelp())
		assert.NotEmpty(t, mf.GetMetric())
	}
}

func TestConfigHashLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := registerMetrics(reg)

	// Test multiple hash values
	hashes := []string{"hash1", "hash2", "hash3"}
	for _, hash := range hashes {
		m.configHash.WithLabelValues(hash).Set(1)
	}

	// Verify all hashes are tracked
	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	var configHashFamily *dto.MetricFamily
	for _, mf := range metricFamilies {
		if mf.GetName() == "remotecfg_hash" {
			configHashFamily = mf
			break
		}
	}

	require.NotNil(t, configHashFamily)
	assert.Len(t, configHashFamily.GetMetric(), len(hashes))

	// Verify each hash is present
	foundHashes := make(map[string]bool)
	for _, metric := range configHashFamily.GetMetric() {
		labels := metric.GetLabel()
		require.Len(t, labels, 1)
		foundHashes[labels[0].GetValue()] = true
	}

	for _, expectedHash := range hashes {
		assert.True(t, foundHashes[expectedHash], "Hash %s should be present", expectedHash)
	}
}

func TestHistogramBuckets(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := registerMetrics(reg)

	// Record some observations
	observations := []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0}
	for _, obs := range observations {
		m.getConfigTime.Observe(obs)
	}

	// Gather metrics
	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	// Find the histogram metric
	var histogramFamily *dto.MetricFamily
	for _, mf := range metricFamilies {
		if mf.GetName() == "remotecfg_request_duration_seconds" {
			histogramFamily = mf
			break
		}
	}

	require.NotNil(t, histogramFamily)
	require.Len(t, histogramFamily.GetMetric(), 1)

	histogram := histogramFamily.GetMetric()[0].GetHistogram()
	require.NotNil(t, histogram)

	// Verify we have buckets
	assert.NotEmpty(t, histogram.GetBucket())

	// Verify total count matches our observations
	assert.Equal(t, uint64(len(observations)), histogram.GetSampleCount())
}
