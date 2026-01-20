//go:build alloyintegrationtests && !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestAlloyIntegrationMetrics(t *testing.T) {
	// These otel metrics are needed in the k8s-monitoring helm chart (https://github.com/grafana/k8s-monitoring-helm/blob/main/charts/k8s-monitoring-v1/default_allow_lists/alloy_integration.yaml)
	var OTLPMetrics = []string{
		"otelcol_exporter_send_failed_spans_total",
		"otelcol_exporter_sent_spans_total",
		"otelcol_processor_batch_batch_send_size_bucket",
		"otelcol_processor_batch_metadata_cardinality",
		"otelcol_processor_batch_timeout_trigger_send_total",
		"otelcol_receiver_accepted_spans_total",
		"otelcol_receiver_refused_spans_total",
	}
	common.MimirMetricsTest(t, OTLPMetrics, []string{}, "otlp_integration")
}
