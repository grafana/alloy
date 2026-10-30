package collector

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
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
			got, err := fullScansFromExplainJSON([]byte(tc.plan), planTableNames{})
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestFullScansFromRealServers(t *testing.T) {
	for name, want := range map[string][]fullScan{
		// One alias, two tables: each is reported under its own name.
		"alias_reused_in_scalar_subquery": {{tableSchema: "shop", table: "orders", rows: 4}, {tableSchema: "shop", table: "customers", rows: 3}},
		// The derived table t is read in full, but no index could change that.
		"derived_table": {{tableSchema: "shop", table: "customers", rows: 3, hasPossibleKeys: true}},
		// Two tables named o in one block can't be told apart.
		"same_alias_merged_by_semijoin": {{table: "o", rows: 2}},
		"cross_schema":                  {{tableSchema: "archive", table: "events", rows: 2}},
	} {
		for version, fixture := range loadTableNameFixtures(t, name) {
			t.Run(name+"/mysql-"+version, func(t *testing.T) {
				got, err := fullScansFromExplainJSON(fixture.plan, parseRewrittenQueryTables(fixture.note))
				require.NoError(t, err)
				require.ElementsMatch(t, want, got)
			})
		}
	}
}

func fullScanMetric(schema, digest, tableSchema, table, hasKeys string, rows float64) string {
	return fmt.Sprintf(`
	# HELP database_observability_mysql_explain_plan_full_scan_rows %s
	# TYPE database_observability_mysql_explain_plan_full_scan_rows gauge
	database_observability_mysql_explain_plan_full_scan_rows{digest=%q,has_possible_keys=%q,schema=%q,table=%q,table_schema=%q} %v
`, "Estimated rows the optimizer reads per scan of a table it reads in full (access type ALL), from the latest explain plan of the query. "+
		"schema is the schema the query runs in; table_schema and table are the table's own schema and name. "+
		"When the table's real name can't be resolved, table is the name the plan uses, which is the alias when the query gives one, and table_schema is empty. "+
		"has_possible_keys is \"false\" when MySQL found no index it could use for the query's conditions on that table.",
		digest, hasKeys, schema, table, tableSchema, rows)
}

// processPlan runs one query through processExplainPlan with the given explain
// plan JSON, as the collector would after reading it from the digest table.
// When the collector exposes its metric, SHOW WARNINGS returns no notes. It
// returns whether the query would be denylisted.
func processPlan(t *testing.T, c *ExplainPlans, mock sqlmock.Sqlmock, schema, digest, planJSON string) bool {
	t.Helper()
	return processPlanWithNote(t, c, mock, schema, digest, planJSON, "")
}

// processPlanWithNote is processPlan with the rewritten query note that SHOW
// WARNINGS returns after the EXPLAIN.
func processPlanWithNote(t *testing.T, c *ExplainPlans, mock sqlmock.Sqlmock, schema, digest, planJSON, note string) bool {
	t.Helper()
	mock.ExpectExec("USE `" + schema + "`").WithoutArgs().WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(selectExplainPlanPrefix + "select * from some_table where id = 1").
		WillReturnRows(sqlmock.NewRows([]string{"json"}).AddRow([]byte(planJSON)))
	if c.registry != nil {
		warnings := sqlmock.NewRows([]string{"Level", "Code", "Message"})
		if note != "" {
			warnings.AddRow("Note", rewrittenQueryNoteCode, note)
		}
		mock.ExpectQuery("SHOW WARNINGS").WillReturnRows(warnings)
	}
	return c.processExplainPlan(t.Context(), newQueryInfo(schema, digest, "select * from some_table where id = 1"))
}

// explainPlanOutputs decodes the explain plan outputs sent to Loki so far.
func explainPlanOutputs(t *testing.T, lokiClient *loki.CollectingHandler) []database_observability.ExplainPlanOutput {
	t.Helper()
	var outputs []database_observability.ExplainPlanOutput
	for _, entry := range lokiClient.Received() {
		_, encoded, found := strings.Cut(entry.Line, `explain_plan_output="`)
		require.True(t, found, entry.Line)
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(encoded, `"`))
		require.NoError(t, err)
		var output database_observability.ExplainPlanOutput
		require.NoError(t, json.Unmarshal(decoded, &output))
		outputs = append(outputs, output)
	}
	return outputs
}

func TestExplainPlansConversionFailuresStillReportFullScans(t *testing.T) {
	// MySQL returned this plan, but its cost can't be read, so converting it
	// for the log output fails.
	const unconvertiblePlan = `{"query_block": {"select_id": 1, "table": {"table_name": "invites", "access_type": "ALL", "rows_examined_per_scan": 369777, "cost_info": {"prefix_cost": "not a number"}}}}`

	registry := prometheus.NewRegistry()
	c, mock, clock := newFullScanTestCollector(t, registry)
	registry.MustRegister(c)
	lokiClient := c.entryHandler.(*loki.CollectingHandler)

	t.Run("the query is not denylisted and its full scan is reported", func(t *testing.T) {
		require.False(t, processPlan(t, c, mock, "grafana_net", "d1", unconvertiblePlan))
		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(fullScanMetric("grafana_net", "d1", "", "invites", "false", 369777))))
	})

	t.Run("an error with a fixed reason is sent, which throttles the query", func(t *testing.T) {
		require.Eventually(t, func() bool { return len(lokiClient.Received()) == 1 }, 5*time.Second, 10*time.Millisecond)
		output := explainPlanOutputs(t, lokiClient)[0]
		require.Equal(t, database_observability.ExplainProcessingResultError, output.Metadata.ProcessingResult)
		require.Equal(t, "failed to create explain plan output", output.Metadata.ProcessingResultReason)
		require.True(t, c.isThrottled(explainPlanQueryKey("grafana_net", "d1")))
	})

	t.Run("explaining it again keeps the full scan from expiring", func(t *testing.T) {
		*clock = clock.Add(fullScanIdleTimeout - time.Minute)
		require.False(t, processPlan(t, c, mock, "grafana_net", "d1", unconvertiblePlan))
		*clock = clock.Add(2 * time.Minute)
		require.Equal(t, 1, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"))
	})

	t.Run("a plan that isn't valid UTF-8 is still read for full scans", func(t *testing.T) {
		require.False(t, processPlan(t, c, mock, "grafana_net", "d2", "{\"query_block\": {\"table\": {\"table_name\": \"inv\xffites\", \"access_type\": \"ALL\", \"rows_examined_per_scan\": 7}}}"))
		require.Equal(t, 2, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"))
	})

	t.Run("a plan that can't be redacted is not denylisted", func(t *testing.T) {
		require.False(t, processPlan(t, c, mock, "grafana_net", "d3", `{"query_block": `))
	})

	t.Run("a query that MySQL returns no plan for is still denylisted", func(t *testing.T) {
		require.True(t, processPlan(t, c, mock, "grafana_net", "d4", ""))
	})

	require.NoError(t, mock.ExpectationsWereMet())
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
		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(fullScanMetric("grafana_net", "d1", "", "invites", "false", 369777))))
	})

	t.Run("the rewritten query gives the table's real schema and name", func(t *testing.T) {
		const aliasedPlan = `{"query_block": {"select_id": 1, "table": {"table_name": "i", "access_type": "ALL", "rows_examined_per_scan": 369777}}}`
		processPlanWithNote(t, c, mock, "grafana_net", "d1", aliasedPlan,
			"/* select#1 */ select `shared`.`i`.`id` AS `id` from `shared`.`invites` `i` where (`shared`.`i`.`org_id` = 123)")
		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(fullScanMetric("grafana_net", "d1", "shared", "invites", "false", 369777))))
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

	t.Run("a scan stays reported while the query keeps running, however long until its next plan", func(t *testing.T) {
		processPlan(t, c, mock, "grafana_net", "d1", scanPlan)
		require.Equal(t, 1, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"))

		for range 3 {
			*clock = clock.Add(fullScanIdleTimeout - time.Minute)
			pollDigests(t, c, mock, "d1")
		}
		require.Equal(t, 1, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"))
	})

	t.Run("a scan stops being reported once the query stops running", func(t *testing.T) {
		*clock = clock.Add(fullScanIdleTimeout - time.Minute)
		pollDigests(t, c, mock, "another digest")
		require.Equal(t, 1, testutil.CollectAndCount(registry, "database_observability_mysql_explain_plan_full_scan_rows"), "seen running recently")

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

// pollDigests runs the collector's poll of the digest table, which returns the
// given digests as having run since the last poll.
func pollDigests(t *testing.T, c *ExplainPlans, mock sqlmock.Sqlmock, digests ...string) {
	t.Helper()
	rows := sqlmock.NewRows([]string{"schema_name", "digest", "query_sample_text", "last_seen"})
	for _, digest := range digests {
		rows.AddRow("grafana_net", digest, "select * from some_table where id = 1", time.Now())
	}
	mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(sqlmock.AnyArg()).RowsWillBeClosed().WillReturnRows(rows)
	require.NoError(t, c.populateQueryCache(t.Context()))
	// Only the poll is under test: leave explaining the queries out.
	clear(c.queryCache)
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
