package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestOTLPMetrics(t *testing.T) {
	common.MimirMetricsTest(t, common.OtelDefaultMetrics, common.OtelDefaultHistogramMetrics, "otlp_metrics")
}
