//go:build !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestRedisMetrics(t *testing.T) {
	var redisMetrics = []string{
		"redis_up",
		"redis_memory_used_bytes",
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
		"redis_up",
	}
	common.MimirMetricsTest(t, redisMetrics, []string{}, "redis_metrics")
}
