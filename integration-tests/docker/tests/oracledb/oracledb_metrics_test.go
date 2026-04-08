//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestOracleDBMetrics(t *testing.T) {
	// The Oracle collector registers exporter-internal metrics (duration, scrape count, last error)
	// without a per-database label. Query them with test_name only.
	exporterBuiltin := []string{
		"oracledb_exporter_last_scrape_duration_seconds",
		"oracledb_exporter_last_scrape_error",
		"oracledb_exporter_scrapes_total",
	}
	// All other metrics carry the exporter's database label (e.g. default, oracledb_1, ...).
	dbScoped := []string{
		"oracledb_up",

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

	// Single connection_string mode uses internal target name "default" as the exporter `database` label.
	common.MimirMetricsTestWithLabels(t, exporterBuiltin, []string{}, map[string]string{
		common.TestNameLabel: "oracledb_metrics",
	})
	common.MimirMetricsTestWithLabels(t, dbScoped, []string{}, map[string]string{
		common.TestNameLabel: "oracledb_metrics",
		"database":           "default",
	})

	// Multi-target: one set of exporter-internal series for the whole component.
	common.MimirMetricsTestWithLabels(t, exporterBuiltin, []string{}, map[string]string{
		common.TestNameLabel: "oracledb_multi_metrics",
	})
	multiDBs := []string{"oracledb_1", "oracledb_2"}
	for _, db := range multiDBs {
		db := db
		t.Run("multi_"+db, func(t *testing.T) {
			t.Parallel()
			common.MimirMetricsTestWithLabels(t, dbScoped, []string{}, map[string]string{
				common.TestNameLabel: "oracledb_multi_metrics",
				"database":           db,
			})
		})
	}
}
