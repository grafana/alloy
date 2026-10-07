package collector

import (
	"fmt"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
)

func TestFullScansFromExplainJSON(t *testing.T) {
	for name, tc := range map[string]struct {
		plan    string
		want    []fullScan
		wantErr bool
	}{
		"single table read in full, no usable index": {
			plan: `{"query_block": {"select_id": 1, "table": {"table_name": "invites", "access_type": "ALL", "rows_examined_per_scan": 369777, "rows_produced_per_join": 362}}}`,
			want: []fullScan{{table: "invites", rows: 369777}},
		},
		"read in full although an index could be used": {
			plan: `{"query_block": {"select_id": 1, "table": {"table_name": "o", "access_type": "ALL", "possible_keys": ["organizations_slug_index"], "rows_examined_per_scan": 833423}}}`,
			want: []fullScan{{table: "o", rows: 833423, hasPossibleKeys: true}},
		},
		"empty possible_keys counts as none": {
			plan: `{"query_block": {"table": {"table_name": "t", "access_type": "ALL", "possible_keys": [], "rows_examined_per_scan": 10}}}`,
			want: []fullScan{{table: "t", rows: 10}},
		},
		"join: only the table read in full is reported": {
			plan: `{"query_block": {"select_id": 1, "nested_loop": [
				{"table": {"table_name": "o", "access_type": "ALL", "rows_examined_per_scan": 833423}},
				{"table": {"table_name": "hpi", "access_type": "ref", "possible_keys": ["hp_instances_org_id_index"], "key": "hp_instances_org_id_index", "rows_examined_per_scan": 3}},
				{"table": {"table_name": "c", "access_type": "eq_ref", "key": "PRIMARY", "rows_examined_per_scan": 1}}]}}`,
			want: []fullScan{{table: "o", rows: 833423}},
		},
		"several tables read in full": {
			plan: `{"query_block": {"nested_loop": [
				{"table": {"table_name": "a", "access_type": "ALL", "rows_examined_per_scan": 5}},
				{"table": {"table_name": "b", "access_type": "ALL", "possible_keys": ["k"], "rows_examined_per_scan": 7}}]}}`,
			want: []fullScan{{table: "a", rows: 5}, {table: "b", rows: 7, hasPossibleKeys: true}},
		},
		"full index scan, range and lookups are not full table scans": {
			plan: `{"query_block": {"nested_loop": [
				{"table": {"table_name": "a", "access_type": "index", "key": "k", "rows_examined_per_scan": 5}},
				{"table": {"table_name": "b", "access_type": "range", "key": "k", "rows_examined_per_scan": 7}},
				{"table": {"table_name": "c", "access_type": "const", "key": "PRIMARY"}}]}}`,
			want: []fullScan{},
		},
		"derived tables and union results are not reported": {
			plan: `{"query_block": {"nested_loop": [
				{"table": {"table_name": "<derived2>", "access_type": "ALL", "rows_examined_per_scan": 100, "materialized_from_subquery": {"query_block": {"table": {"table_name": "real", "access_type": "ALL", "rows_examined_per_scan": 9}}}}},
				{"table": {"table_name": "<union1,2>", "access_type": "ALL", "rows_examined_per_scan": 3}}]}}`,
			want: []fullScan{{table: "real", rows: 9}},
		},
		"the same table in a subquery keeps the larger estimate and any usable index": {
			plan: `{"query_block": {"table": {"table_name": "t", "access_type": "ALL", "rows_examined_per_scan": 10,
				"attached_subqueries": [{"query_block": {"table": {"table_name": "t", "access_type": "ALL", "possible_keys": ["k"], "rows_examined_per_scan": 50}}}]}}}`,
			want: []fullScan{{table: "t", rows: 50, hasPossibleKeys: true}},
		},
		"access type is matched case-insensitively": {
			plan: `{"query_block": {"table": {"table_name": "t", "access_type": "all", "rows_examined_per_scan": 4}}}`,
			want: []fullScan{{table: "t", rows: 4}},
		},
		"missing row estimate is zero": {
			plan: `{"query_block": {"table": {"table_name": "t", "access_type": "ALL"}}}`,
			want: []fullScan{{table: "t"}},
		},
		"plan without tables": {
			plan: `{"query_block": {"select_id": 1, "message": "no matching row in const table"}}`,
			want: []fullScan{},
		},
		"not json": {plan: `not json`, wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := fullScansFromExplainJSON([]byte(tc.plan))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func fullScanMetric(schema, digest, table, hasKeys string, rows float64) string {
	return fmt.Sprintf(`
	# HELP database_observability_mysql_explain_plan_full_scan_rows %s
	# TYPE database_observability_mysql_explain_plan_full_scan_rows gauge
	database_observability_mysql_explain_plan_full_scan_rows{digest=%q,has_possible_keys=%q,schema=%q,table=%q} %v
`, "Estimated rows the optimizer reads per scan of a table it reads in full (access type ALL), from the latest explain plan of the query. "+
		"has_possible_keys is \"false\" when MySQL found no index it could use for the query's conditions on that table. "+
		"The table label is the name the plan uses, which is the alias when the query gives one.", digest, hasKeys, schema, table, rows)
}

// processPlan runs one query through processExplainPlan with the given explain
// plan JSON, as the collector would after reading it from the digest table.
func processPlan(t *testing.T, c *ExplainPlans, mock sqlmock.Sqlmock, schema, digest, planJSON string) {
	t.Helper()
	mock.ExpectExec("USE `" + schema + "`").WithoutArgs().WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(selectExplainPlanPrefix + "select * from some_table where id = 1").
		WillReturnRows(sqlmock.NewRows([]string{"json"}).AddRow([]byte(planJSON)))
	c.processExplainPlan(t.Context(), newQueryInfo(schema, digest, "select * from some_table where id = 1"))
}

func newFullScanTestCollector(t *testing.T, registry *prometheus.Registry) (*ExplainPlans, sqlmock.Sqlmock, *time.Time) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	lokiClient := loki.NewCollectingHandler()
	t.Cleanup(lokiClient.Stop)

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          util.TestAlloyLogger(t).Slog(),
		ScrapeInterval:  time.Hour,
		PerScrapeRatio:  1,
		EntryHandler:    lokiClient,
		InitialLookback: time.Now().Add(-time.Hour),
		Registry:        registry,
	})
	require.NoError(t, err)

	clock := time.Now()
	c.now = func() time.Time { return clock }
	c.fullScans = newFullScanStore(c.now)
	return c, mock, &clock
}

func TestExplainPlansReportFullScans(t *testing.T) {
	const scanPlan = `{"query_block": {"select_id": 1, "table": {"table_name": "invites", "access_type": "ALL", "rows_examined_per_scan": 369777, "attached_condition": "(` + "`db`.`invites`.`org_id` = 123)" + `"}}}`
	const indexedPlan = `{"query_block": {"select_id": 1, "table": {"table_name": "invites", "access_type": "ref", "key": "invites_org_id_idx", "rows_examined_per_scan": 3}}}`

	registry := prometheus.NewRegistry()
	c, mock, clock := newFullScanTestCollector(t, registry)
	registry.MustRegister(c)

	t.Run("a plan with a full scan is reported", func(t *testing.T) {
		processPlan(t, c, mock, "grafana_net", "d1", scanPlan)
		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(fullScanMetric("grafana_net", "d1", "invites", "false", 369777))))
	})

	t.Run("the literal values of the query never reach the labels", func(t *testing.T) {
		families, err := registry.Gather()
		require.NoError(t, err)
		for _, f := range families {
			for _, m := range f.GetMetric() {
				for _, l := range m.GetLabel() {
					require.NotContains(t, l.GetValue(), "123")
				}
			}
		}
	})

	t.Run("a newer plan replaces the older one, and a fixed query stops being reported", func(t *testing.T) {
		processPlan(t, c, mock, "grafana_net", "d1", indexedPlan)
		require.Equal(t, 0, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"))
	})

	t.Run("a scan that nothing refreshes expires", func(t *testing.T) {
		processPlan(t, c, mock, "grafana_net", "d1", scanPlan)
		require.Equal(t, 1, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"))

		*clock = clock.Add(fullScanTTL - time.Minute)
		require.Equal(t, 1, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"), "still within the time to live")

		*clock = clock.Add(2 * time.Minute)
		require.Equal(t, 0, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"))
	})

	t.Run("a plan that can't be read keeps what was reported before", func(t *testing.T) {
		processPlan(t, c, mock, "grafana_net", "d2", scanPlan)
		require.Equal(t, 1, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"))
		// A truncated plan is skipped before it reaches the store.
		processPlan(t, c, mock, "grafana_net", "d2", `{"query_block": `)
		require.Equal(t, 1, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"))
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExplainPlansRegistersAndUnregistersItsMetric(t *testing.T) {
	registry := prometheus.NewRegistry()

	start := func() (*ExplainPlans, sqlmock.Sqlmock) {
		c, mock, _ := newFullScanTestCollector(t, registry)
		// The collector's loop looks for digests as soon as it starts.
		mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(sqlmock.AnyArg()).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"schema_name", "digest", "query_sample_text", "last_seen"}))
		require.NoError(t, c.Start(t.Context()))
		return c, mock
	}

	c, _ := start()
	c.fullScans.set("s", "d", []fullScan{{table: "t", rows: 1}})
	require.Equal(t, 1, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"), "registered by Start")
	c.Stop()
	require.Equal(t, 0, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"), "unregistered by Stop")

	// The same registry can be used again after Stop.
	c2, _ := start()
	c2.Stop()
}

func TestExplainPlansWithoutRegistry(t *testing.T) {
	c, mock, _ := newFullScanTestCollector(t, nil)
	processPlan(t, c, mock, "s", "d", `{"query_block": {"table": {"table_name": "t", "access_type": "ALL", "rows_examined_per_scan": 5}}}`)
	// Nothing to register with: the state is still kept, and Stop is safe.
	require.NotPanics(t, c.Stop)
	require.NoError(t, mock.ExpectationsWereMet())
}
