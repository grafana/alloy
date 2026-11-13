package infinitysource

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// fixturesImage must match the image in config/fixtures.yaml.
const fixturesImage = "nginx:1.27-alpine"

func TestInfinitySource(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-infinity-source",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	loki := deps.NewLoki(deps.LokiOptions{Namespace: ns.Name()})
	// nginx serves static copies of the jsonplaceholder.typicode.com responses,
	// so the test does not depend on the public API.
	fixtures := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Namespace: ns.Name(),
		Path:      "./config/fixtures.yaml",
		Images:    []string{fixturesImage},
	})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-infinity-source",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, loki, fixtures, alloy},
	})

	const testName = "infinity-source"

	// The todos query filters and summarizes rows into one gauge per user.
	mimir.QueryMetrics(t, testName, []string{
		"jsonplaceholder_completed_todos",
		"up",
	})
	mimir.QueryMetricWithLabelsPresent(t, testName, "jsonplaceholder_completed_todos", "user", "job", "instance")
	mimir.QueryPositive(t, testName, []string{
		"jsonplaceholder_completed_todos",
		"up",
	})

	// Each poll sends every user again, so the entry count is not fixed.
	loki.QueryLogsPresent(t, testName,
		deps.LogMatcher{
			Labels: map[string]string{
				"job":      "infinity.source.jsonplaceholder",
				"instance": "users",
				"city":     "Gwenborough",
			},
			StructuredMetadata: map[string]string{"email": "Sincere@april.biz"},
		},
		deps.LogMatcher{
			Labels: map[string]string{
				"job":      "infinity.source.jsonplaceholder",
				"instance": "users",
				"city":     "Lebsackbury",
			},
			StructuredMetadata: map[string]string{"email": "Rey.Padberg@karina.biz"},
		},
	)
}
