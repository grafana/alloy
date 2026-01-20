//go:build alloyintegrationtests

package main

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestPromMetadataRemoteWriteV2(t *testing.T) {
	testName := "prom_metadata_remote_write_v2"
	toTestMetric := func(metricName string) string {
		return fmt.Sprintf("%s_%s", testName, metricName)
	}

	// Metadata queries cannot use a series matcher so the metrics all need to be unqiue to the test
	metadataTestMetrics := make([]string, 0, len(common.PromDefaultMetrics))
	for _, metricName := range common.PromDefaultMetrics {
		metadataTestMetrics = append(metadataTestMetrics, toTestMetric(metricName))
	}

	// Make sure we got the expected metrics before checking metadata
	common.MimirMetricsTest(t, metadataTestMetrics, nil, testName)
	expectedMetadata := make(map[string]common.Metadata, len(common.PromDefaultMetricMetadata)+len(common.PromDefaultNativeHistogramMetadata))
	for k, v := range common.PromDefaultMetricMetadata {
		expectedMetadata[toTestMetric(k)] = v
	}
	common.MimirMetadataTest(t, expectedMetadata)
}
