package beylaebpf

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const namespace = "test-beyla-ebpf"

// TestBeylaEBPF runs beyla.ebpf in a privileged, host PID Alloy pod and checks
// that it instruments the HTTP server of the Mimir pod in the same namespace.
func TestBeylaEBPF(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   namespace,
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-beyla-ebpf",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, alloy},
	})

	// The pod's Kubernetes metadata shows Beyla instrumented the right process.
	mimirPod := map[string]string{
		"service_name":        "mimir",
		"k8s_namespace_name":  namespace,
		"k8s_deployment_name": "mimir",
	}

	t.Run("Metrics", func(t *testing.T) {
		mimir.QueryMetrics(t, "beyla", []string{"beyla_internal_build_info"})
		mimir.QueryMetricsWithLabelMatch(t, "beyla", []string{
			"http_server_request_duration_seconds_count",
			"http_server_request_duration_seconds_sum",
		}, mimirPod)
	})

	t.Run("Traces", func(t *testing.T) {
		mimir.QueryMetricsWithLabelMatch(t, "beyla-traces", []string{
			"traces_span_metrics_calls_total",
		}, map[string]string{"service_name": "mimir"})
	})

	// The build injects the Beyla version. Without it the label is "unset".
	t.Run("Version", func(t *testing.T) {
		mimir.QuerySampleTimestamps(t, "beyla", "beyla_internal_build_info", "1m", nil,
			func(c *assert.CollectT, labels map[string]string, _ []float64) {
				assert.NotEmpty(c, labels["version"], "version label")
				assert.NotEqual(c, "unset", labels["version"], "version label")
			})
	})
}
