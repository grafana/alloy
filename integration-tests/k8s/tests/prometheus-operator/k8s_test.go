package prometheusoperator

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestPrometheusOperator(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-prometheus-operator",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	promOp := deps.NewPrometheusOperator(deps.PrometheusOperatorOptions{})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	promGen := deps.NewPromGen(deps.PromGenOptions{Namespace: ns.Name()})
	blackbox := deps.NewBlackboxExporter(deps.BlackboxExporterOptions{Namespace: ns.Name()})
	monitoringCRDs := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Path: "./config/workloads.yaml",
		Vars: map[string]string{"NAMESPACE": ns.Name()},
	})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-prometheus-operator",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, promOp, promGen, blackbox, monitoringCRDs, mimir, alloy},
	})

	t.Run("ServiceMonitors", func(t *testing.T) {
		t.Parallel()
		// Check that Mimir received metrics from the ServiceMonitor target.
		// All metrics are prefixed with test_servicemonitors_ via relabeling.
		mimir.QueryMetrics(t, "servicemonitor", []string{
			"test_servicemonitors_golang_counter",
			"test_servicemonitors_golang_gauge",
		})
		mimir.QueryMetadata(t, map[string]deps.ExpectedMetadata{
			"test_servicemonitors_golang_counter": {Type: "counter", Help: "The counter description string"},
			"test_servicemonitors_golang_gauge":   {Type: "gauge", Help: "The gauge description string"},
		})
	})

	t.Run("PodMonitors", func(t *testing.T) {
		t.Parallel()
		// Check that Mimir received metrics from the PodMonitor target.
		// All metrics are prefixed with test_podmonitors_ via relabeling.
		mimir.QueryMetrics(t, "podmonitor", []string{
			"test_podmonitors_golang_counter",
			"test_podmonitors_golang_gauge",
		})
		mimir.QueryMetadata(t, map[string]deps.ExpectedMetadata{
			"test_podmonitors_golang_counter": {Type: "counter", Help: "The counter description string"},
			"test_podmonitors_golang_gauge":   {Type: "gauge", Help: "The gauge description string"},
		})
	})

	t.Run("Probes", func(t *testing.T) {
		t.Parallel()
		// Check that Mimir received metrics from the Probe target.
		// All metrics are prefixed with test_probes_ via relabeling.
		mimir.QueryMetrics(t, "probe", []string{
			"test_probes_probe_success",
			"test_probes_probe_duration_seconds",
		})
		mimir.QueryMetadata(t, map[string]deps.ExpectedMetadata{
			"test_probes_probe_success":          {Type: "gauge", Help: "Displays whether or not the probe was a success"},
			"test_probes_probe_duration_seconds": {Type: "gauge", Help: "Returns how long the probe took to complete in seconds"},
		})
	})

	t.Run("ScrapeConfigs", func(t *testing.T) {
		t.Parallel()
		// Check that Mimir received metrics from the ScrapeConfig target.
		// All metrics are prefixed with test_scrapeconfigs_ via relabeling.
		mimir.QueryMetrics(t, "scrapeconfig", []string{
			"test_scrapeconfigs_golang_counter",
			"test_scrapeconfigs_golang_gauge",
		})
		mimir.QueryMetadata(t, map[string]deps.ExpectedMetadata{
			"test_scrapeconfigs_golang_counter": {Type: "counter", Help: "The counter description string"},
			"test_scrapeconfigs_golang_gauge":   {Type: "gauge", Help: "The gauge description string"},
		})
	})
}
