package lokisourcekubernetesevents

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestLokiSourceKubernetesEvents(t *testing.T) {
	namespace := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-loki-k8s-events",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})

	eventsNS := deps.NewNamespace(deps.NamespaceOptions{Name: "k8sevents-test"})

	loki := deps.NewLoki(deps.LokiOptions{Namespace: namespace.Name()})

	gen := deps.NewLogGen(deps.LogGenOptions{
		Namespace: eventsNS.Name(),
		Replicas:  1,
		FilePath:  "./config/test.log",
	})

	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  namespace.Name(),
		Release:    "alloy-test-loki-k8s-events",
		ConfigPath: "./config/config.alloy",
	})

	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{namespace, eventsNS, loki, gen, alloy},
	})

	loki.QueryLogs(t, "loki-source-kubernetes_events",
		deps.ExpectedLogResult{
			EntryCount:         1,
			Labels:             map[string]string{"reason": "Started", "level": "Info"},
			StructuredMetadata: map[string]string{"name": "log-gen-0"},
		},
		deps.ExpectedLogResult{
			EntryCount:         1,
			Labels:             map[string]string{"reason": "Created", "level": "Info"},
			StructuredMetadata: map[string]string{"name": "log-gen-0"},
		},
		deps.ExpectedLogResult{
			EntryCount:         1,
			Labels:             map[string]string{"reason": "Scheduled", "level": "Info"},
			StructuredMetadata: map[string]string{"name": "log-gen-0"},
		},
	)
}
