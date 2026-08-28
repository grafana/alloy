//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestOtelMetricsBasic(t *testing.T) {
	common.MimirMetricsTest(t, common.OtelDefaultMetrics, common.OtelDefaultHistogramMetrics, "otel_metrics_basic")
}
