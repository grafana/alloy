package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestBeylaMetrics(t *testing.T) {
	var beylaMetrics = []string{
		"beyla_internal_build_info",                // check that internal Beyla metrics are reported
		"http_server_request_duration_seconds_sum", // check that the target metrics are reported
	}
	config := common.Config{
		T:                t,
		TestName:         "beyla",
		Metrics:          beylaMetrics,
		HistogramMetrics: []string{},
		ExpectedMetadata: common.ExpectedMetadata,
	}
	common.MimirMetricsTest(config)
}
