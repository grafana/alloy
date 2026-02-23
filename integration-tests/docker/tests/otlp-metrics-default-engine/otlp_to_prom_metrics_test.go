//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestOTLPToPromMetrics(t *testing.T) {
	// Not using the default here because some metric names change during the conversion.
	metrics := []string{
		"example_counter_total",       // Change from example_counter to example_counter_total.
		"example_float_counter_total", // Change from example_float_counter to example_float_counter_total.
		"example_updowncounter",
		"example_float_updowncounter",
		"example_histogram_bucket",
		"example_float_histogram_bucket",
	}

	common.MimirMetricsTest(t, metrics, common.OtelDefaultHistogramMetrics, "otlp_to_prom_metrics")
}
