//go:build alloyintegrationtests

package prometheusoperator

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestPrometheusOperator(t *testing.T) {
	harness.SkipShard(t, "prometheus-operator")
	kt := harness.Setup(t, harness.Options{
		Name:       "prometheus-operator",
		ConfigPath: "./config/config.alloy",
		Workloads:  "./config/workloads.yaml",
		Backends:   []harness.Backend{harness.BackendMimir},
		Controller: "daemonset",
	})
	defer kt.Cleanup(t)

	kt.WaitForPodRunning(t, kt.Namespace, "app.kubernetes.io/name=alloy")
	kt.WaitForPodRunning(t, kt.Namespace, "app=prom-gen")
	kt.WaitForPodRunning(t, kt.Namespace, "app=blackbox-exporter")
	kt.WaitForPodRunning(t, kt.Namespace, "app.kubernetes.io/component=distributor")

	t.Run("ServiceMonitors", func(t *testing.T) {
		// Check that Mimir received metrics from the ServiceMonitor target.
		// All metrics are prefixed with test_servicemonitors_ via relabeling.
		kt.QueryMimirMetrics(t, "servicemonitor", []string{
			"test_servicemonitors_golang_counter",
			"test_servicemonitors_golang_gauge",
		})
	})

	t.Run("PodMonitors", func(t *testing.T) {
		// Check that Mimir received metrics from the PodMonitor target.
		// All metrics are prefixed with test_podmonitors_ via relabeling.
		kt.QueryMimirMetrics(t, "podmonitor", []string{
			"test_podmonitors_golang_counter",
			"test_podmonitors_golang_gauge",
		})
	})

	t.Run("Probes", func(t *testing.T) {
		// Check that Mimir received metrics from the Probe target.
		// All metrics are prefixed with test_probes_ via relabeling.
		kt.QueryMimirMetrics(t, "probe", []string{
			"test_probes_probe_success",
			"test_probes_probe_duration_seconds",
		})
	})

	t.Run("ScrapeConfigs", func(t *testing.T) {
		// Check that Mimir received metrics from the ScrapeConfig target.
		// All metrics are prefixed with test_scrapeconfigs_ via relabeling.
		kt.QueryMimirMetrics(t, "scrapeconfig", []string{
			"test_scrapeconfigs_golang_counter",
			"test_scrapeconfigs_golang_gauge",
		})
	})
}
