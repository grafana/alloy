//go:build alloyintegrationtests

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	// All other metrics carry the exporter's database label (e.g. default, oracledb_c1, ...).
	metrics := []string{
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

	// Each component publishes under its own test_name so we can assert that
	// every exporter instance produced metrics independently. Components a and
	// b use the top-level connection_string; c and d use database blocks.
	componentTestNames := []string{
		"oracledb_multi_component_a",
		"oracledb_multi_component_b",
		"oracledb_multi_component_c",
		"oracledb_multi_component_d",
	}
	for _, testName := range componentTestNames {
		testName := testName
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			common.MimirMetricsTest(t, exporterBuiltin, []string{}, testName)
			common.MimirMetricsTest(t, metrics, []string{}, testName)
		})
	}

	// Multi-target components must emit metrics with the per-database label set
	// to each `database` block's name.
	multiTargets := map[string][]string{
		"oracledb_multi_component_c": {"oracledb_c1", "oracledb_c2"},
		"oracledb_multi_component_d": {"oracledb_d1", "oracledb_d2"},
	}
	for testName, dbs := range multiTargets {
		testName, dbs := testName, dbs
		for _, db := range dbs {
			db := db
			t.Run(testName+"/"+db, func(t *testing.T) {
				t.Parallel()
				assertSeriesLabelsForMetrics(t, testName, metrics, map[string]string{
					"test_name": testName,
					"database":  db,
				})
			})
		}
	}
}

type SeriesResponse struct {
	Status string              `json:"status"`
	Data   []map[string]string `json:"data"`
}

func (m *SeriesResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func seriesLabelsMatch(actualLabels, wantedLabels map[string]string) bool {
	for k, v := range wantedLabels {
		if actualLabels[k] != v {
			return false
		}
	}
	return true
}

// assertSeriesLabelsForMetrics fetches series for testName, then for each metric name in metrics
// expects at least one series whose label set matches labels.
func assertSeriesLabelsForMetrics(t *testing.T, testName string, metrics []string, wantedLabels map[string]string) {
	metricNames := make(map[string]struct{}, len(metrics))
	for _, n := range metrics {
		metricNames[n] = struct{}{}
	}

	var resp SeriesResponse
	_, err := common.FetchDataFromURL(common.MetricsQuery(testName), &resp)
	require.NoError(t, err)

	found := make(map[string]struct{})
	for _, actualLabels := range resp.Data {
		name := actualLabels["__name__"]
		if _, ok := metricNames[name]; !ok {
			continue
		}
		if !seriesLabelsMatch(actualLabels, wantedLabels) {
			continue
		}
		found[name] = struct{}{}
	}

	for _, name := range metrics {
		assert.Contains(t, found, name, "expected a series for metric %s with labels %v", name, wantedLabels)
	}
}
