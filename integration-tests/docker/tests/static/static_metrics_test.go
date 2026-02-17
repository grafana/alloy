//go:build alloyintegrationtests && !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestStaticExporter(t *testing.T) {
	expectedMetrics := []string{"http_requests_total",
		"http_request_duration_seconds_bucket",
		"http_request_duration_seconds_count",
		"http_request_duration_seconds_sum",
	}

	common.MimirMetricsTest(t, expectedMetrics, nil, "static_exporter_test")
}
