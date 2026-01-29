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
		// All metrics are prefixed with test_servicemonitors_ via relabeling.
		kt.QueryMimirMetrics(t, "servicemonitor", mimirPort, []string{
			"test_servicemonitors_golang_counter",
			"test_servicemonitors_golang_gauge",
		})
		// Check that Mimir received metadata for the metrics (honor_metadata = true)
		// Each component has unique metric names to verify metadata is sent independently.
		kt.QueryMimirMetadata(t, mimirPort, map[string]util.ExpectedMetadata{
			"test_servicemonitors_golang_counter": {Type: "counter", Help: "The counter description string"},
			"test_servicemonitors_golang_gauge":   {Type: "gauge", Help: "The gauge description string"},
		})
	})

	t.Run("PodMonitors", func(t *testing.T) {
		// Check that Mimir received metrics from the PodMonitor target.
		// All metrics are prefixed with test_podmonitors_ via relabeling.
		kt.QueryMimirMetrics(t, "podmonitor", mimirPort, []string{
			"test_podmonitors_golang_counter",
			"test_podmonitors_golang_gauge",
		})
		// Check that Mimir received metadata for the metrics (honor_metadata = true)
		// Each component has unique metric names to verify metadata is sent independently.
		kt.QueryMimirMetadata(t, mimirPort, map[string]util.ExpectedMetadata{
			"test_podmonitors_golang_counter": {Type: "counter", Help: "The counter description string"},
			"test_podmonitors_golang_gauge":   {Type: "gauge", Help: "The gauge description string"},
		})
	})

	t.Run("Probes", func(t *testing.T) {
		// Check that Mimir received metrics from the Probe target.
		// All metrics are prefixed with test_probes_ via relabeling.
		kt.QueryMimirMetrics(t, "probe", mimirPort, []string{
			"test_probes_probe_success",
			"test_probes_probe_duration_seconds",
		})
		// Check that Mimir received metadata for the probe metrics (honor_metadata = true)
		// Each component has unique metric names to verify metadata is sent independently.
		kt.QueryMimirMetadata(t, mimirPort, map[string]util.ExpectedMetadata{
			"test_probes_probe_success":          {Type: "gauge", Help: "Displays whether or not the probe was a success"},
			"test_probes_probe_duration_seconds": {Type: "gauge", Help: "Returns how long the probe took to complete in seconds"},
		})
	})

	t.Run("ScrapeConfigs", func(t *testing.T) {
		// Check that Mimir received metrics from the ScrapeConfig target.
		// All metrics are prefixed with test_scrapeconfigs_ via relabeling.
		kt.QueryMimirMetrics(t, "scrapeconfig", mimirPort, []string{
			"test_scrapeconfigs_golang_counter",
			"test_scrapeconfigs_golang_gauge",
		})
		// Check that Mimir received metadata for the metrics (honor_metadata = true)
		// Each component has unique metric names to verify metadata is sent independently.
		kt.QueryMimirMetadata(t, mimirPort, map[string]util.ExpectedMetadata{
			"test_scrapeconfigs_golang_counter": {Type: "counter", Help: "The counter description string"},
			"test_scrapeconfigs_golang_gauge":   {Type: "gauge", Help: "The gauge description string"},
		})
	})
}
