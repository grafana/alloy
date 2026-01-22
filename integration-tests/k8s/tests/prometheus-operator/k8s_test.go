//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const mimirPort = "12347"

func TestPrometheusOperator(t *testing.T) {
	testDataDir := "./testdata/"

	cleanupFunc := util.BootstrapTest(testDataDir, "prometheus-operator")
	defer cleanupFunc()

	terminatePortFwd := util.ExecuteBackgroundCommand(
		"kubectl", []string{"port-forward", "service/mimir-nginx", mimirPort + ":80", "--namespace=mimir-test"},
		"Port forward Mimir")
	defer terminatePortFwd()

	kt := util.NewKubernetesTester(t)
	kt.WaitForPodRunning(t, "testing", "app=grafana-alloy")
	kt.WaitForPodRunning(t, "testing", "app=prom-gen")
	kt.WaitForPodRunning(t, "testing", "app=blackbox-exporter")
	kt.WaitForPodRunning(t, "mimir-test", "app.kubernetes.io/component=distributor")

	t.Run("ServiceMonitors", func(t *testing.T) {
		// Check that Mimir received metrics from the ServiceMonitor target.
		// The prom-gen pod exposes metrics like "golang_counter", "golang_gauge", etc.
		kt.QueryMimirMetrics(t, "servicemonitor", mimirPort, []string{
			"golang_counter",
			"golang_gauge",
		})
		// Check that Mimir received metadata for the metrics (honor_metadata = true)
		kt.QueryMimirMetadata(t, mimirPort, map[string]util.ExpectedMetadata{
			"golang_counter": {Type: "counter", Help: "The counter description string"},
			"golang_gauge":   {Type: "gauge", Help: "The gauge description string"},
		})
	})

	t.Run("PodMonitors", func(t *testing.T) {
		// Check that Mimir received metrics from the PodMonitor target.
		// Uses the same prom-gen pod but scraped via PodMonitor.
		kt.QueryMimirMetrics(t, "podmonitor", mimirPort, []string{
			"golang_counter",
			"golang_gauge",
		})
		// Check that Mimir received metadata for the metrics (honor_metadata = true)
		// Note: metadata from ServiceMonitors test should already be present, so we just verify it's still there
		kt.QueryMimirMetadata(t, mimirPort, map[string]util.ExpectedMetadata{
			"golang_counter": {Type: "counter", Help: "The counter description string"},
			"golang_gauge":   {Type: "gauge", Help: "The gauge description string"},
		})
	})

	t.Run("Probes", func(t *testing.T) {
		// Check that Mimir received metrics from the Probe target.
		// The blackbox exporter returns probe_success metric.
		kt.QueryMimirMetrics(t, "probe", mimirPort, []string{
			"probe_success",
			"probe_duration_seconds",
		})
		// Check that Mimir received metadata for the probe metrics (honor_metadata = true)
		kt.QueryMimirMetadata(t, mimirPort, map[string]util.ExpectedMetadata{
			"probe_success":          {Type: "gauge", Help: "Displays whether or not the probe was a success"},
			"probe_duration_seconds": {Type: "gauge", Help: "Returns how long the probe took to complete in seconds"},
		})
	})

	t.Run("ScrapeConfigs", func(t *testing.T) {
		// Check that Mimir received metrics from the ScrapeConfig target.
		// Uses the same prom-gen pod but scraped via ScrapeConfig with static targets.
		kt.QueryMimirMetrics(t, "scrapeconfig", mimirPort, []string{
			"golang_counter",
			"golang_gauge",
		})
		// Check that Mimir received metadata for the metrics (honor_metadata = true)
		kt.QueryMimirMetadata(t, mimirPort, map[string]util.ExpectedMetadata{
			"golang_counter": {Type: "counter", Help: "The counter description string"},
			"golang_gauge":   {Type: "gauge", Help: "The gauge description string"},
		})
	})
}
