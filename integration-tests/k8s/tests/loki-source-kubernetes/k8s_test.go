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

	targetNamespace := deps.NewNamespace(deps.NamespaceOptions{
		Name: "log-producer",
	})

	loki := deps.NewLoki(deps.LokiOptions{Namespace: namespace.Name()})

	logProducerImage := deps.NewCustomImage(deps.CustomImageOptions{
		Tag:            "log-producer:test",
		ContextPath:    "./config",
		DockerfilePath: "./config/Dockerfile.logproducer",
	})

	workloads := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Path: "./config/workloads.yaml",
		Vars: map[string]string{
			"NAMESPACE": targetNamespace.Name(),
		},
	})

	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  namespace.Name(),
		Release:    "alloy-test-loki-source-kubernetes",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})

	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{namespace, targetNamespace, logProducerImage, workloads, loki, alloy},
	})

	loki.QueryLogs(t, "log-producer",
		deps.ExpectedLogResult{
			EntryCount:         3,
			Labels:             map[string]string{"namespace": targetNamespace.Name()},
			StructuredMetadata: map[string]string{"pod": "log-producer"},
		},
		deps.ExpectedLogResult{
			EntryCount:         3,
			Labels:             map[string]string{"namespace": targetNamespace.Name()},
			StructuredMetadata: map[string]string{"pod": "log-producer-2"},
		},
	)
}
