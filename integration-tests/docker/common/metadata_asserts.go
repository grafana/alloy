package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

// Default metrics metadata that the prometheus.write_queue component creates in Mimir.
// For some reason it causes _bucket, _count, and _sum metrics to be visible on Mimir's /metadata endpoint.
// This is not normal. When Prometheus or Alloy's prometheus.remote_write components send metrics to Mimir,
// only the metric family is visible on Mimir's /metadata endpoint.
// TODO: Look into why the write_queue components causes Mimir to behave this way.
var WriteQueueDefaultMetricMetadata = map[string]Metadata{
	"golang_counter":                {Type: "counter", Help: "The counter description string"},
	"golang_gauge":                  {Type: "gauge", Help: "The gauge description string"},
	"golang_histogram_bucket":       {Type: "histogram", Help: "The histogram description string"},
	"golang_histogram_count":        {Type: "histogram", Help: "The histogram description string"},
	"golang_histogram_sum":          {Type: "histogram", Help: "The histogram description string"},
	"golang_mixed_histogram_bucket": {Type: "histogram", Help: "The mixed_histogram description string"},
	"golang_mixed_histogram_count":  {Type: "histogram", Help: "The mixed_histogram description string"},
	"golang_mixed_histogram_sum":    {Type: "histogram", Help: "The mixed_histogram description string"},
	"golang_summary":                {Type: "summary", Help: "The summary description string"},
}

// Default metrics metadata that the prometheus.remote_write and otelcol.exporter.otlphttp components create in Mimir.
var PromDefaultMetricMetadata = map[string]Metadata{
	"golang_counter":         {Type: "counter", Help: "The counter description string"},
	"golang_gauge":           {Type: "gauge", Help: "The gauge description string"},
	"golang_histogram":       {Type: "histogram", Help: "The histogram description string"},
	"golang_mixed_histogram": {Type: "histogram", Help: "The mixed_histogram description string"},
	"golang_summary":         {Type: "summary", Help: "The summary description string"},
}

// Default native histogram metadata
var PromDefaultNativeHistogramMetadata = map[string]Metadata{
	"golang_native_histogram": {Type: "histogram", Help: "The native_histogram description string"},
	"golang_mixed_histogram":  {Type: "histogram", Help: "The mixed_histogram description string"},
}

func MimirMetadataTest(t *testing.T, expectedMetadata map[string]Metadata) {
	AssertStatefulTestEnv(t)

	expectedMetricsWithMetadata := make([]string, 0, len(expectedMetadata))
	for metricName := range expectedMetadata {
		expectedMetricsWithMetadata = append(expectedMetricsWithMetadata, metricName)
	}

	var metricMetadata MetadataResponse
	var err error
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		metricMetadata, err = GetMetadata()
		assert.NoError(c, err)
		assert.Subset(c, maps.Keys(metricMetadata.Data), expectedMetricsWithMetadata, "did not find metadata for the expected metrics")
	}, TestTimeoutEnv(t), DefaultRetryInterval)

	for metricName, expectedMeta := range expectedMetadata {
		actualMetas := metricMetadata.Data[metricName]
		if assert.Len(t, actualMetas, 1, "expected exactly one metadata entry for metric %s but found %d", metricName, len(actualMetas)) {
			actualMeta := actualMetas[0]
			assert.Equal(t, expectedMeta, actualMeta, "metadata for metric %s did not match the expected metadata", metricName)
		}
	}

	if IsStatefulTest() {
		assert.Fail(t, "Metadata queries cannot be done with a timestamp so if we found data it's possible it's from a previous test run. This test fails so you can consider if this is a problem for you or not")
	}
}

type Metadata struct {
	Type string `json:"type"`
	Help string `json:"help"`
	Unit string `json:"unit"`
}

type MetadataResponse struct {
	Status string                `json:"status"`
	Data   map[string][]Metadata `json:"data"`
}

func (m *MetadataResponse) Unmarshal(bytes []byte) error {
	return json.Unmarshal(bytes, &m)
}

func MetadataQuery() string {
	// https://prometheus.io/docs/prometheus/latest/querying/api/#querying-metric-metadata
	return fmt.Sprintf("%smetadata", promURL)
}

// GetMetadata returns all available metric metadata
func GetMetadata() (MetadataResponse, error) {
	var metadataResponse MetadataResponse
	query := MetadataQuery()
	_, err := FetchDataFromURL(query, &metadataResponse)
	return metadataResponse, err
}
