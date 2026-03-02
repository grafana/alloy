//go:build alloyintegrationtests

package main

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestPromMetadataWriteQueue(t *testing.T) {
	testName := "prom_metadata_writequeue"
	toTestMetric := func(metricName string) string {
		return fmt.Sprintf("%s_%s", testName, metricName)
	}

	// Metadata queries cannot use a series matcher so the metrics all need to be unqiue to the test
	metadataTestMetrics := make([]string, 0, len(common.PromDefaultMetrics))
	for _, metricName := range common.PromDefaultMetrics {
		metadataTestMetrics = append(metadataTestMetrics, toTestMetric(metricName))
	}

	metadataTestHistogram := make([]string, 0, len(common.PromDefaultNativeHistogramMetrics))
	for _, metricName := range common.PromDefaultNativeHistogramMetrics {
		metadataTestHistogram = append(metadataTestHistogram, toTestMetric(metricName))
	}

	// Make sure we got the expected metrics before checking metadata
	common.MimirMetricsTest(t, metadataTestMetrics, metadataTestHistogram, testName)
	expectedMetadata := make(map[string]common.Metadata, len(common.WriteQueueDefaultMetricMetadata)+len(common.PromDefaultNativeHistogramMetadata))
	for k, v := range common.WriteQueueDefaultMetricMetadata {
		expectedMetadata[toTestMetric(k)] = v
	}
	for k, v := range common.PromDefaultNativeHistogramMetadata {
		expectedMetadata[toTestMetric(k)] = v
	}
	common.MimirMetadataTest(t, expectedMetadata)
}
