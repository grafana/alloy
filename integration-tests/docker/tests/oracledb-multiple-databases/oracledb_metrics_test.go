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
	// All other metrics carry the exporter's database label (e.g. default, oracledb_1, ...).
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

	// Multi-target: one set of exporter-internal series for the whole component.
	common.MimirMetricsTest(t, exporterBuiltin, []string{}, "oracledb_multi_metrics")
	common.MimirMetricsTest(t, metrics, []string{}, "oracledb_multi_metrics")

	multiDBs := []string{"oracledb_1", "oracledb_2"}
	for _, db := range multiDBs {
		db := db
		t.Run("multi_"+db, func(t *testing.T) {
			t.Parallel()
			assertSeriesLabelsForMetrics(t, "oracledb_multi_metrics", metrics, map[string]string{
				"test_name": "oracledb_multi_metrics",
				"database":  db,
			})
		})
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
