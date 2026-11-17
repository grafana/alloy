package prometheusexporterredis

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestPrometheusExporterRedis(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-prometheus-exporter-redis",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	redis := deps.NewRedis(deps.RedisOptions{Namespace: ns.Name()})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-prometheus-exporter-redis",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, redis, mimir, alloy},
	})

	const testName = "prometheus-exporter-redis"

	mimir.QueryMetrics(t, testName, []string{
		"redis_up",
		"redis_blocked_clients",
		"redis_commands_duration_seconds_total",
		"redis_commands_total",
		"redis_connected_clients",
		"redis_connected_slaves",
		"redis_db_keys",
		"redis_db_keys_expiring",
		"redis_evicted_keys_total",
		"redis_keyspace_hits_total",
		"redis_keyspace_misses_total",
		"redis_memory_max_bytes",
		"redis_memory_used_bytes",
		"redis_memory_used_rss_bytes",
	})

	mimir.QueryPositive(t, testName, []string{
		"redis_up",
		"redis_commands_total",
		"redis_connected_clients",
		"redis_db_keys",
		"redis_db_keys_expiring",
		"redis_memory_used_bytes",
		"redis_memory_used_rss_bytes",
	})
}
