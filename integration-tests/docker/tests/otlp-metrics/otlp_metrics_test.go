//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestOTLPMetrics(t *testing.T) {
	common.MimirMetricsTest(t, common.OtelDefaultMetrics, common.OtelDefaultHistogramMetrics, "otlp_metrics")
}
