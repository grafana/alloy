package databaseobservabilitypostgres

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestDatabaseObservabilityPostgres(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-database-observability-postgres",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	postgres := deps.NewPostgres(deps.PostgresOptions{Namespace: ns.Name()})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	loki := deps.NewLoki(deps.LokiOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-dbo-postgres",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, postgres, mimir, loki, alloy},
	})

	const testName = "database-observability-postgres"

	metrics := []string{"database_observability_connection_info"}
	mimir.QueryMetrics(t, testName, metrics)
	mimir.QueryPositive(t, testName, metrics)

	loki.QueryLogsPresent(t, testName,
		deps.LogMatcher{Labels: map[string]string{"op": "health_status"}},
		deps.LogMatcher{Labels: map[string]string{"op": "query_association"}},
		deps.LogMatcher{Labels: map[string]string{"op": "query_parsed_table_name"}},
		deps.LogMatcher{Labels: map[string]string{"op": "table_detection"}},
		deps.LogMatcher{Labels: map[string]string{"op": "create_statement"}},
	)
}
