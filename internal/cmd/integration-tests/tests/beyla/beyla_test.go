package main

import (
	"testing"
)

func TestBeylaMetrics(t *testing.T) {
	// TODO this test is broken and needs deeper investigation
	//var beylaMetrics = []string{
	//	"beyla_internal_build_info",                // check that internal Beyla metrics are reported
	//	"http_server_request_duration_seconds_sum", // check that the target metrics are reported
	//}
	//common.MimirMetricsTest(t, beylaMetrics, []string{}, "beyla")
}

func TestBeylaTraces(t *testing.T) {
	// TODO this test is broken and needs deeper investigation
	// Test that traces are being generated and sent to Tempo
	//tags := map[string]string{
	//	"service.name": "main", // This should match the instrumented app
	//}
	// common.TracesTest(t, tags, "beyla")
}
