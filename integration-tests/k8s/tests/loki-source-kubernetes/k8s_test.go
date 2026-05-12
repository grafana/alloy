package lokisourcekubernetes

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestLokiSourceKubernetes(t *testing.T) {
	namespace := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-loki-source-kubernetes",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})

	targetA := deps.NewNamespace(deps.NamespaceOptions{Name: "log-producer-a"})
	targetB := deps.NewNamespace(deps.NamespaceOptions{Name: "log-producer-b"})
	targetC := deps.NewNamespace(deps.NamespaceOptions{Name: "log-producer-c"})

	loki := deps.NewLoki(deps.LokiOptions{Namespace: namespace.Name()})

	logProducerImage := deps.NewCustomImage(deps.CustomImageOptions{
		Tag:            "log-producer:test",
		ContextPath:    "./config",
		DockerfilePath: "./config/Dockerfile.logproducer",
	})

	workloads := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Path: "./config/workloads.yaml",
		Vars: map[string]string{
			"NAMESPACE_A": targetA.Name(),
			"NAMESPACE_B": targetB.Name(),
			"NAMESPACE_C": targetC.Name(),
		},
	})

	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  namespace.Name(),
		Release:    "alloy-test-loki-source-kubernetes",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})

	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{namespace, targetA, targetB, targetC, logProducerImage, workloads, loki, alloy},
	})

	loki.QueryLogs(t, "loki-source-kubernetes",
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetA.Name()},
			StructuredMetadata: map[string]string{"pod": "log-producer-0"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetA.Name()},
			StructuredMetadata: map[string]string{"pod": "log-producer-1"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetB.Name()},
			StructuredMetadata: map[string]string{"pod": "log-producer-0"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetB.Name()},
			StructuredMetadata: map[string]string{"pod": "log-producer-1"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetC.Name()},
			StructuredMetadata: map[string]string{"pod": "log-producer-0"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetC.Name()},
			StructuredMetadata: map[string]string{"pod": "log-producer-1"},
		},
	)
}
