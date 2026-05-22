package prometheusexportermssql

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestPrometheusExporterMssql(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-prometheus-exporter-mssql",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mssql := deps.NewMSSQL(deps.MSSQLOptions{Namespace: ns.Name()})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-prometheus-exporter-mssql",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mssql, mimir, alloy},
	})

	mimir.QueryMetrics(t, "mssql", []string{
		"mssql_local_time_seconds",
		"mssql_connections",
		"mssql_batch_requests_total",
		"mssql_page_life_expectancy_seconds",
	})
	mimir.QueryMetadata(t, map[string]deps.ExpectedMetadata{
		"mssql_local_time_seconds":           {Type: "gauge", Help: "Local time in seconds since epoch (Unix time)."},
		"mssql_connections":                  {Type: "gauge", Help: "Number of active connections."},
		"mssql_batch_requests_total":         {Type: "counter", Help: "Number of command batches received."},
		"mssql_page_life_expectancy_seconds": {Type: "gauge", Help: "The minimum number of seconds a page will stay in the buffer pool on this node without references."},
	})
}
