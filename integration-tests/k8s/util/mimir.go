package util

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	intTestLabel  = "alloy_int_test"
	timeout       = 5 * time.Minute
	retryInterval = 500 * time.Millisecond
)

// MetricsResponse represents the response from Prometheus /api/v1/series endpoint.
type MetricsResponse struct {
	Status string `json:"status"`
	Data   []struct {
		Name string `json:"__name__"`
	} `json:"data"`
}

// MetadataResponse represents the response from Prometheus /api/v1/metadata endpoint.
type MetadataResponse struct {
	Status string                     `json:"status"`
	Data   map[string][]MetadataEntry `json:"data"`
}

// MetadataEntry represents a single metadata entry for a metric.
type MetadataEntry struct {
	Type string `json:"type"`
	Help string `json:"help"`
	Unit string `json:"unit"`
}

func (k *KubernetesTester) QueryMimirMetrics(t *testing.T, alloyIntTest, mimirPort string, expectedMetrics []string) {
	mimirURL := "http://localhost:" + mimirPort + "/prometheus/api/v1/"

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Query for all metrics scraped by the given alloy_int_test label value
		query := mimirURL + "series?match[]={" + intTestLabel + "=\"" + alloyIntTest + "\"}"
		resp := k.Curl(c, query)

		var metricsResponse MetricsResponse
		err := json.Unmarshal([]byte(resp), &metricsResponse)
		require.NoError(c, err, "Failed to parse Mimir response: %s", resp)

		require.Equal(c, "success", metricsResponse.Status, "Mimir query failed: %s", resp)

		// Collect actual metric names
		actualMetrics := make(map[string]struct{}, len(metricsResponse.Data))
		for _, metric := range metricsResponse.Data {
			actualMetrics[metric.Name] = struct{}{}
		}

		// Check that all expected metrics are present
		var missingMetrics []string
		for _, expectedMetric := range expectedMetrics {
			if _, exists := actualMetrics[expectedMetric]; !exists {
				missingMetrics = append(missingMetrics, expectedMetric)
			}
		}

		require.Emptyf(c, missingMetrics, "Missing expected metrics for %s=%s: %v. Found metrics: %v", intTestLabel, alloyIntTest, missingMetrics, actualMetrics)
	}, timeout, retryInterval)
}

// ExpectedMetadata defines the expected metadata for a metric.
type ExpectedMetadata struct {
	Type string
	Help string
}

func (k *KubernetesTester) QueryMimirMetadata(t *testing.T, mimirPort string, expectedMetadata map[string]ExpectedMetadata) {
	mimirURL := "http://localhost:" + mimirPort + "/prometheus/api/v1/"

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Query for all metadata
		query := mimirURL + "metadata"
		resp := k.Curl(c, query)

		var metadataResponse MetadataResponse
		err := json.Unmarshal([]byte(resp), &metadataResponse)
		require.NoError(c, err, "Failed to parse Mimir metadata response: %s", resp)

		require.Equal(c, "success", metadataResponse.Status, "Mimir metadata query failed: %s", resp)

		// Check that all expected metadata is present
		var missingMetrics []string
		var mismatchedMetrics []string
		for metricName, expected := range expectedMetadata {
			entries, exists := metadataResponse.Data[metricName]
			if !exists || len(entries) == 0 {
				missingMetrics = append(missingMetrics, metricName)
				continue
			}

			// Check the first entry matches expected type and help
			entry := entries[0]
			if expected.Type != "" && entry.Type != expected.Type {
				mismatchedMetrics = append(mismatchedMetrics, metricName+": expected type="+expected.Type+", got="+entry.Type)
			}
			if expected.Help != "" && entry.Help != expected.Help {
				mismatchedMetrics = append(mismatchedMetrics, metricName+": expected help="+expected.Help+", got="+entry.Help)
			}
		}

		require.Emptyf(c, missingMetrics, "Missing expected metadata for metrics: %v", missingMetrics)
		require.Emptyf(c, mismatchedMetrics, "Mismatched metadata: %v", mismatchedMetrics)
	}, timeout, retryInterval)
}

func (k *KubernetesTester) CheckMimirConfig(t *testing.T, testDataDir, mimirPort, expectedFile string) {
	expectedMimirConfigBytes, err := os.ReadFile(testDataDir + expectedFile)
	require.NoError(t, err)
	expectedMimirConfig := string(expectedMimirConfigBytes)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		actualMimirConfig := k.Curl(c, "http://localhost:"+mimirPort+"/api/v1/alerts")
		require.Equal(c, expectedMimirConfig, actualMimirConfig)
	}, timeout, retryInterval)
}
