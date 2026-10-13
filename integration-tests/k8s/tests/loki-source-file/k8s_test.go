package lokisourcefile

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestLokiSourceFile(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-loki-source-file",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	target := deps.NewNamespace(deps.NamespaceOptions{Name: "file-log-producer"})
	loki := deps.NewLoki(deps.LokiOptions{Namespace: ns.Name()})
	gen := deps.NewLogGen(deps.LogGenOptions{
		Namespace: target.Name(),
		Replicas:  2,
		FilePath:  "./config/test.log",
	})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-loki-source-file",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, target, loki, gen, alloy},
	})

	loki.QueryLogs(t, "loki-source-file",
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": target.Name()},
			StructuredMetadata: map[string]string{"pod": "log-gen-0"},
		},
		deps.ExpectedLogResult{
			EntryCount:         10,
			Labels:             map[string]string{"namespace": target.Name()},
			StructuredMetadata: map[string]string{"pod": "log-gen-1"},
		},
	)
}
