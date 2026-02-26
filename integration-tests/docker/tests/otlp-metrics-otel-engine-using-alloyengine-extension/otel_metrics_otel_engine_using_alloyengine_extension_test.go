//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestOtelMetricsOtelEngineUsingAlloyengineExtension(t *testing.T) {
	common.MimirMetricsTest(
		t,
		common.OtelDefaultMetrics,
		common.OtelDefaultHistogramMetrics,
		"otel_metrics_otel_engine_using_alloyengine_extension",
	)
}
