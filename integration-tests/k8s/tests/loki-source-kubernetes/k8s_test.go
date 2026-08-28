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

	genA := deps.NewLogGen(deps.LogGenOptions{
		Namespace: targetA.Name(),
		Replicas:  2,
		FilePath:  "./config/test.log",
	})

	genB := deps.NewLogGen(deps.LogGenOptions{
		Namespace: targetB.Name(),
		Replicas:  2,
		FilePath:  "./config/test.log",
	})

	genC := deps.NewLogGen(deps.LogGenOptions{
		Namespace: targetC.Name(),
		Replicas:  2,
		FilePath:  "./config/test.log",
	})

	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  namespace.Name(),
		Release:    "alloy-test-loki-source-kubernetes",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})

	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{namespace, targetA, targetB, targetC, genA, genB, genC, loki, alloy},
	})

	loki.QueryLogs(t, "loki-source-kubernetes",
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetA.Name()},
			StructuredMetadata: map[string]string{"pod": "log-gen-0"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetA.Name()},
			StructuredMetadata: map[string]string{"pod": "log-gen-1"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetB.Name()},
			StructuredMetadata: map[string]string{"pod": "log-gen-0"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetB.Name()},
			StructuredMetadata: map[string]string{"pod": "log-gen-1"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetC.Name()},
			StructuredMetadata: map[string]string{"pod": "log-gen-0"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": targetC.Name()},
			StructuredMetadata: map[string]string{"pod": "log-gen-1"},
		},
	)
}
