package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestOTLPMetrics(t *testing.T) {
	config := common.Config{
		T:                t,
		TestName:         "otlp_metrics",
		Metrics:          common.OtelDefaultMetrics,
		HistogramMetrics: common.OtelDefaultHistogramMetrics,
		ExpectedMetadata: common.ExpectedMetadata,
	}
	common.MimirMetricsTest(config)
}
