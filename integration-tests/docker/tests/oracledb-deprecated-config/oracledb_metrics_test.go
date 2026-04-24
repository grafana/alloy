//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestOracleDBMetrics(t *testing.T) {
	// Exporter built-in metrics — always present when the DB is reachable.
	var oracledbMetrics = []string{
		"oracledb_up",
		"oracledb_exporter_last_scrape_duration_seconds",
		"oracledb_exporter_last_scrape_error",
		"oracledb_exporter_scrapes_total",

		// Session and process counts.
		"oracledb_sessions_value",
		"oracledb_process_count",

		// Activity counters from v$sysstat (fieldtoappend=name, cleaned).
		"oracledb_activity_execute_count",
		"oracledb_activity_user_commits",
		"oracledb_activity_user_rollbacks",
		"oracledb_activity_parse_count_total",

		// Tablespace metrics — SYSTEM/SYSAUX/USERS are always present in XE.
		"oracledb_tablespace_bytes",
		"oracledb_tablespace_max_bytes",
		"oracledb_tablespace_free",
		"oracledb_tablespace_used_percent",

		// DB-level metadata metrics.
		"oracledb_db_system_value",
		"oracledb_db_platform_value",

		// Custom metric from custom-metrics.toml (context startup_time → oracledb_startup_time_hours_up).
		"oracledb_startup_time_hours_up",
	}
	common.MimirMetricsTest(t, oracledbMetrics, []string{}, "oracledb_metrics")
}
