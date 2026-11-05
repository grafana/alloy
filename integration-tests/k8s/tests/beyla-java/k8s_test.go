package beylajava

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const (
	namespace = "test-beyla-java"
	testName  = "beyla-java"
)

// TestBeylaJava runs beyla.ebpf in a privileged, host PID Alloy pod and checks
// that it instruments a JVM in another pod, including attaching its Java agent
// to that JVM.
func TestBeylaJava(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   namespace,
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	javaApp := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Namespace: ns.Name(),
		Path:      "./config/java-app.yaml",
	})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-beyla-java",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, javaApp, alloy},
	})

	javaAppPod := map[string]string{
		"service_name":        "java-app",
		"k8s_namespace_name":  namespace,
		"k8s_deployment_name": "java-app",
	}

	t.Run("Metrics", func(t *testing.T) {
		mimir.QueryMetricsWithLabelMatch(t, testName, []string{
			"http_server_request_duration_seconds_count",
			"http_server_request_duration_seconds_sum",
		}, javaAppPod)
	})

	t.Run("Traces", func(t *testing.T) {
		mimir.QueryMetricsWithLabelMatch(t, testName, []string{
			"traces_span_metrics_calls_total",
		}, map[string]string{"service_name": "java-app", "telemetry_sdk_language": "java"})
	})

	// Only the Java agent that Beyla attaches to the JVM reports these. They
	// are missing with OTEL_EBPF_JAVAAGENT_ENABLED=false.
	t.Run("JavaAgent", func(t *testing.T) {
		mimir.QueryMetricsWithLabelMatch(t, testName, []string{
			"jvm_class_count",
			"jvm_gc_duration_seconds_count",
			"jvm_thread_count",
		}, map[string]string{"service_name": "java-app"})
	})
}
