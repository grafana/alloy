package collector

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/syncbuffer"
)

func TestExplainPlansRedactor(t *testing.T) {
	tests := []struct {
		txtarfile string
		file      string
		original  [][]byte
		redacted  [][]byte
	}{
		{
			txtarfile: "./testdata/explain_plan/join_and_order.txtar",
			file:      "join_and_order.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/subquery_with_aggregate.txtar",
			file:      "subquery_with_aggregate.json",
			original: [][]byte{
				[]byte("((`employees`.`s`.`to_date` = DATE'9999-01-01') and (`employees`.`s`.`salary` > (/* select#2 */ select (avg(`employees`.`salaries`.`salary`) * 1.5) from `employees`.`salaries`)))"),
			},
			redacted: [][]byte{
				[]byte("( ( `employees` . `s` . `to_date` = date ? ) and ( `employees` . `s` . `salary` > ( select ( avg ( `employees` . `salaries` . `salary` ) * ? ) from `employees` . `salaries` ) ) )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/group_by_with_having.txtar",
			file:      "group_by_with_having.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/multiple_joins_with_date_functions.txtar",
			file:      "multiple_joins_with_date_functions.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
				[]byte("(year(`employees`.`e`.`hire_date`) = 1985)"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
				[]byte("( year ( `employees` . `e` . `hire_date` ) = ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/window_functions.txtar",
			file:      "window_functions.json",
			original: [][]byte{
				[]byte("(`employees`.`s`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `s` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/correlated_subquery.txtar",
			file:      "correlated_subquery.json",
			original: [][]byte{
				[]byte("(`employees`.`t`.`to_date` = DATE'9999-01-01')"),
				[]byte("((`employees`.`salaries`.`to_date` = DATE'9999-01-01') and (`employees`.`salaries`.`salary` > 100000))"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `t` . `to_date` = date ? )"),
				[]byte("( ( `employees` . `salaries` . `to_date` = date ? ) and ( `employees` . `salaries` . `salary` > ? ) )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/distinct_with_multiple_joins.txtar",
			file:      "distinct_with_multiple_joins.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`t`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
				[]byte("( `employees` . `t` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/complex_aggregation_with_case.txtar",
			file:      "complex_aggregation_with_case.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/string_functions_with_grouping.txtar",
			file:      "string_functions_with_grouping.json",
			original:  [][]byte{},
			redacted:  [][]byte{},
		},
		{
			txtarfile: "./testdata/explain_plan/nested_subqueries_with_exists.txtar",
			file:      "nested_subqueries_with_exists.json",
			original: [][]byte{
				[]byte("((`employees`.`s`.`to_date` = DATE'9999-01-01') and (`employees`.`s`.`salary` > 100000))"),
			},
			redacted: [][]byte{
				[]byte("( ( `employees` . `s` . `to_date` = date ? ) and ( `employees` . `s` . `salary` > ? ) )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/union_with_different_conditions.txtar",
			file:      "union_with_different_conditions.json",
			original: [][]byte{
				[]byte("(`employees`.`dm`.`to_date` = DATE'9999-01-01')"),
				[]byte("((`employees`.`t`.`to_date` = DATE'9999-01-01') and (`employees`.`t`.`title` = 'Senior Engineer'))"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `dm` . `to_date` = date ? )"),
				[]byte("( ( `employees` . `t` . `to_date` = date ? ) and ( `employees` . `t` . `title` = ? ) )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/date_manipulation_with_conditions.txtar",
			file:      "date_manipulation_with_conditions.json",
			original: [][]byte{
				[]byte("((month(`employees`.`e`.`hire_date`) = <cache>(month(curdate()))) and (`employees`.`e`.`hire_date` < DATE'1990-01-01'))"),
			},
			redacted: [][]byte{
				[]byte("( ( month ( `employees` . `e` . `hire_date` ) = < cache > ( month ( curdate ( ) ) ) ) and ( `employees` . `e` . `hire_date` < date ? ) )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/complex_join_with_aggregate_subquery.txtar",
			file:      "complex_join_with_aggregate_subquery.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`de2`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`s2`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
				[]byte("( `employees` . `de2` . `to_date` = date ? )"),
				[]byte("( `employees` . `s2` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/multiple_aggregate_functions_with_having.txtar",
			file:      "multiple_aggregate_functions_with_having.json",
			original: [][]byte{
				[]byte("(`employees`.`t`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`s`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `t` . `to_date` = date ? )"),
				[]byte("( `employees` . `s` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/conditional_aggregation_with_case.txtar",
			file:      "conditional_aggregation_with_case.json",
			original:  [][]byte{},
			redacted:  [][]byte{},
		},
		{
			txtarfile: "./testdata/explain_plan/complex_subquery_in_select_clause.txtar",
			file:      "complex_subquery_in_select_clause.json",
			original: [][]byte{
				[]byte("(`employees`.`e`.`emp_no` < 10050)"),
				[]byte("(`employees`.`t`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `e` . `emp_no` < ? )"),
				[]byte("( `employees` . `t` . `to_date` = date ? )"),
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/window_functions_with_partitioning.txtar",
			file:      "window_functions_with_partitioning.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`s`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
				[]byte("( `employees` . `s` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/self_join_with_date_comparison.txtar",
			file:      "self_join_with_date_comparison.json",
			original: [][]byte{
				[]byte("(`employees`.`de1`.`to_date` = DATE'9999-01-01')"),
				[]byte("((`employees`.`de2`.`dept_no` = `employees`.`de1`.`dept_no`) and (`employees`.`de2`.`to_date` = DATE'9999-01-01') and (`employees`.`de1`.`emp_no` < `employees`.`de2`.`emp_no`))"),
				[]byte("(`employees`.`e2`.`hire_date` = `employees`.`e1`.`hire_date`)"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de1` . `to_date` = date ? )"),
				[]byte("( ( `employees` . `de2` . `dept_no` = `employees` . `de1` . `dept_no` ) and ( `employees` . `de2` . `to_date` = date ? ) and ( `employees` . `de1` . `emp_no` < `employees` . `de2` . `emp_no` ) )"),
				[]byte("( `employees` . `e2` . `hire_date` = `employees` . `e1` . `hire_date` )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/derived_table_with_aggregates.txtar",
			file:      "derived_table_with_aggregates.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`s`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`s`.`salary` > `dept_salary_stats`.`avg_salary`)"),
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`s`.`to_date` = DATE'9999-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
				[]byte("( `employees` . `s` . `to_date` = date ? )"),
				[]byte("( `employees` . `s` . `salary` > `dept_salary_stats` . `avg_salary` )"),
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
				[]byte("( `employees` . `s` . `to_date` = date ? )"),
			},
		},
		{
			txtarfile: "./testdata/explain_plan/complex_query_with_multiple_conditions_and_functions.txtar",
			file:      "complex_query_with_multiple_conditions_and_functions.json",
			original: [][]byte{
				[]byte("(`employees`.`de`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`s`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`t`.`to_date` = DATE'9999-01-01')"),
				[]byte("(`employees`.`e`.`hire_date` > DATE'1985-01-01')"),
			},
			redacted: [][]byte{
				[]byte("( `employees` . `de` . `to_date` = date ? )"),
				[]byte("( `employees` . `s` . `to_date` = date ? )"),
				[]byte("( `employees` . `t` . `to_date` = date ? )"),
				[]byte("( `employees` . `e` . `hire_date` > date ? )"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			archive, err := txtar.ParseFile(test.txtarfile)
			require.NoError(t, err)
			require.Equal(t, 1, len(archive.Files))
			jsonFile := archive.Files[0]
			require.Equal(t, test.file, jsonFile.Name)
			jsonData := jsonFile.Data
			for _, original := range test.original {
				require.Contains(t, string(jsonData), string(original))
				// Comparing the byte arrays directly fails, even though the contents are identical?
				// require.Contains(t, data, original)
			}

			redactedExplainPlanJSON, redactedAttachedConditionsCount, err := redactAttachedConditions(jsonData)
			require.NoError(t, err, "Failed to redact file: %s", test.file)

			for _, tRedacted := range test.redacted {
				require.Contains(t, string(redactedExplainPlanJSON), string(tRedacted))
			}

			require.Equal(t, len(test.original), redactedAttachedConditionsCount)
		})
	}
}

func TestExplainPlansOutput(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		_, err := newExplainPlansOutput(util.TestAlloyLogger(t).Slog(), []byte("not json data"))
		require.ErrorContains(t, err, "failed to get query block: Key path not found")
	})

	t.Run("unknown operation", func(t *testing.T) {
		output, err := newExplainPlansOutput(util.TestAlloyLogger(t).Slog(), []byte(`{"query_block":{"operation":"unknown"}}`))
		require.NoError(t, err)
		require.Equal(t, database_observability.ExplainPlanOutputOperationUnknown, output.Operation)
	})

	t.Run("zero rows", func(t *testing.T) {
		_, err := newExplainPlansOutput(util.TestAlloyLogger(t).Slog(), []byte(`{"query_block":{"message":"no matching row in const table"}}`))
		require.NoError(t, err)
	})
}

func TestMySQLAccessTypeOperation(t *testing.T) {
	tests := map[string]database_observability.ExplainPlanOutputOperation{
		"system":          "Single Row (system constant)",
		"const":           "Single Row (constant)",
		"eq_ref":          "Unique Key Lookup",
		"ref":             "Non-Unique Key Lookup",
		"fulltext":        "Fulltext Index Search",
		"ref_or_null":     "Key Lookup + Fetch NULL Values",
		"index_merge":     "Index Merge",
		"unique_subquery": "Unique Key Lookup into table of subquery",
		"index_subquery":  "Non-Unique Key Lookup into table of subquery",
		"range":           "Index Range Scan",
		"index":           "Full Index Scan",
		"ALL":             "Full Table Scan",
		"unexpected":      database_observability.ExplainPlanOutputOperationUnknown,
	}

	for accessType, expectedOperation := range tests {
		t.Run(accessType, func(t *testing.T) {
			tableJSON := fmt.Sprintf(`{"table_name":"t","access_type":%q}`, accessType)
			node, err := parseTableNode(util.TestAlloyLogger(t).Slog(), []byte(tableJSON))
			require.NoError(t, err)
			require.Equal(t, expectedOperation, node.Operation)
			require.Equal(t, strings.ToLower(accessType), string(*node.Details.AccessType))
		})
	}
}

func TestMySQLNestedLoopNormalization(t *testing.T) {
	output := loadMySQLExplainPlanFixture(t, "join_and_order")

	require.Equal(t, database_observability.ExplainPlanOutputOperationOrderingOperation, output.Operation)
	require.InDelta(t, 37253.59, *output.Details.EstimatedCost, 0.001)
	require.Len(t, output.Children, 1)

	e := output.Children[0]
	require.Equal(t, database_observability.ExplainPlanOutputOperation("Unique Key Lookup"), e.Operation)
	require.Equal(t, "e", *e.Details.Alias)
	require.Equal(t, database_observability.ExplainPlanJoinAlgorithmNestedLoop, *e.Details.JoinAlgorithm)
	require.InDelta(t, 40978.94, *e.Details.EstimatedCost, 0.001)
	require.Len(t, e.Children, 1)

	de := e.Children[0]
	require.Equal(t, database_observability.ExplainPlanOutputOperation("Non-Unique Key Lookup"), de.Operation)
	require.Equal(t, "de", *de.Details.Alias)
	require.Equal(t, database_observability.ExplainPlanJoinAlgorithmNestedLoop, *de.Details.JoinAlgorithm)
	require.InDelta(t, 57152.59, *de.Details.EstimatedCost, 0.001)
	require.Len(t, de.Children, 1)

	d := de.Children[0]
	require.Equal(t, database_observability.ExplainPlanOutputOperation("Full Index Scan"), d.Operation)
	require.Equal(t, "d", *d.Details.Alias)
	require.Nil(t, d.Details.JoinAlgorithm)
	require.InDelta(t, 1.9, *d.Details.EstimatedCost, 0.001)
	require.Empty(t, d.Children)

	require.InDelta(t, 135387.02, sumExplainPlanCosts(output), 0.001)
}

func TestMySQLExplicitHashJoinIsPreserved(t *testing.T) {
	output := loadMySQLExplainPlanFixture(t, "self_join_with_date_comparison")

	require.Equal(t, 1, countExplainPlanOperations(output, database_observability.ExplainPlanOutputOperationHashJoin))
	require.Zero(t, countExplainPlanOperations(output, database_observability.ExplainPlanOutputOperationNestedLoopJoin))
}

func TestMySQLIndependentlyCostedMaterializedSubquery(t *testing.T) {
	output := loadMySQLExplainPlanFixture(t, "materialized_subquery_with_duplicates_removal")

	require.Equal(t, database_observability.ExplainPlanOutputOperationOrderingOperation, output.Operation)
	require.Nil(t, output.Details.EstimatedCost)
	require.Len(t, output.Children, 1)

	outerLookup := output.Children[0]
	require.Equal(t, database_observability.ExplainPlanOutputOperation("Non-Unique Key Lookup"), outerLookup.Operation)
	require.Equal(t, "rl", *outerLookup.Details.Alias)
	require.InDelta(t, 81784.59, *outerLookup.Details.EstimatedCost, 0.001)
	require.Len(t, outerLookup.Children, 2)

	outerScan := outerLookup.Children[0]
	require.Equal(t, database_observability.ExplainPlanOutputOperation("Index Range Scan"), outerScan.Operation)
	require.InDelta(t, 14051.77, *outerScan.Details.EstimatedCost, 0.001)

	materializedSubquery := outerLookup.Children[1]
	require.Equal(t, database_observability.ExplainPlanOutputOperationMaterializedSubquery, materializedSubquery.Operation)
	require.Len(t, materializedSubquery.Children, 1)

	subqueryOrdering := materializedSubquery.Children[0]
	require.Equal(t, database_observability.ExplainPlanOutputOperationOrderingOperation, subqueryOrdering.Operation)
	require.Nil(t, subqueryOrdering.Details.EstimatedCost)
	require.Len(t, subqueryOrdering.Children, 1)

	duplicatesRemoval := subqueryOrdering.Children[0]
	require.Equal(t, database_observability.ExplainPlanOutputOperationDuplicatesRemoval, duplicatesRemoval.Operation)
	require.InDelta(t, 2304463.25, *duplicatesRemoval.Details.EstimatedCost, 0.001)

	require.InDelta(t, 2634900.96, sumExplainPlanCosts(output), 0.02)
}

func TestMySQLCostsReconcileWithQueryCost(t *testing.T) {
	tests := map[string]float64{
		"conditional_aggregation_with_case": 330440.60,
		"correlated_subquery":               868450.61,
		"derived_table_with_aggregates":     278020.69,
		"join_and_order":                    135387.02,
		"subquery_with_aggregate":           394027.81,
	}

	for fixture, queryCost := range tests {
		t.Run(fixture, func(t *testing.T) {
			output := loadMySQLExplainPlanFixture(t, fixture)
			require.InDelta(t, queryCost, sumExplainPlanCosts(output), 0.02)
		})
	}
}

func TestMySQLExplainPlanFixtures(t *testing.T) {
	fixtures := []string{
		"complex_aggregation_with_case",
		"complex_join_with_aggregate_subquery",
		"complex_query_with_multiple_conditions_and_functions",
		"complex_subquery_in_select_clause",
		"conditional_aggregation_with_case",
		"correlated_subquery",
		"date_manipulation_with_conditions",
		"derived_table_with_aggregates",
		"distinct_with_multiple_joins",
		"group_by_with_having",
		"join_and_order",
		"materialized_subquery_with_duplicates_removal",
		"multiple_aggregate_functions_with_having",
		"multiple_joins_with_date_functions",
		"nested_subqueries_with_exists",
		"self_join_with_date_comparison",
		"string_functions_with_grouping",
		"subquery_with_aggregate",
		"union_with_different_conditions",
	}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			output := loadMySQLExplainPlanFixture(t, fixture)
			actual, err := json.Marshal(output)
			require.NoError(t, err)
			expected, err := os.ReadFile(fmt.Sprintf("./testdata/explain_plan_expected/%s.json", fixture))
			require.NoError(t, err)
			require.JSONEq(t, string(expected), string(actual))
			require.Zero(t, countExplainPlanOperations(output, database_observability.ExplainPlanOutputOperationNestedLoopJoin))
		})
	}
}

func TestMySQLTableCostValidation(t *testing.T) {
	logger := util.TestAlloyLogger(t).Slog()

	withoutCost, err := parseTableNode(logger, []byte(`{"table_name":"t","access_type":"ALL"}`))
	require.NoError(t, err)
	require.Nil(t, withoutCost.Details.EstimatedCost)

	_, err = parseTableNode(logger, []byte(`{"table_name":"t","access_type":"ALL","cost_info":{"prefix_cost":"invalid"}}`))
	require.ErrorContains(t, err, "failed to parse estimated cost as float")
}

func loadMySQLExplainPlanFixture(t *testing.T, name string) database_observability.ExplainPlanNode {
	t.Helper()
	archive, err := txtar.ParseFile(fmt.Sprintf("./testdata/explain_plan/%s.txtar", name))
	require.NoError(t, err)
	require.Len(t, archive.Files, 1)

	output, err := newExplainPlansOutput(util.TestAlloyLogger(t).Slog(), archive.Files[0].Data)
	require.NoError(t, err)
	return *output
}

func countExplainPlanOperations(node database_observability.ExplainPlanNode, operation database_observability.ExplainPlanOutputOperation) int {
	count := 0
	if node.Operation == operation {
		count++
	}
	for _, child := range node.Children {
		count += countExplainPlanOperations(child, operation)
	}
	return count
}

func sumExplainPlanCosts(node database_observability.ExplainPlanNode) float64 {
	var total float64
	if node.Details.EstimatedCost != nil {
		total = *node.Details.EstimatedCost
	}
	for _, child := range node.Children {
		total += sumExplainPlanCosts(child)
	}
	return total
}

func TestExplainPlans(t *testing.T) {
	t.Run("last seen", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lastSeen := time.Now().Add(-time.Hour)
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		c, err := NewExplainPlans(ExplainPlansArguments{
			DB:              db,
			Logger:          util.TestAlloyLogger(t).Slog(),
			ScrapeInterval:  time.Second,
			PerScrapeRatio:  1,
			EntryHandler:    lokiClient,
			DBVersion:       "8.0.32",
			InitialLookback: lastSeen,
		})
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		t.Run("uses argument value on first request", func(t *testing.T) {
			nextSeen := lastSeen.Add(time.Second * 45)
			mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
				"schema_name",
				"digest",
				"query_text",
				"last_seen",
			}).AddRow(
				"some_schema",
				"some_digest",
				"some_query_text",
				lastSeen.Add(time.Second*5),
			).AddRow(
				"some_schema",
				"some_digest",
				"some_query_text",
				nextSeen,
			))
			lastSeen = nextSeen
			err := c.populateQueryCache(t.Context())
			require.NoError(t, err)
		})

		t.Run("uses oldest last seen value on subsequent requests", func(t *testing.T) {
			mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
				"schema_name",
				"digest",
				"query_text",
				"last_seen",
			}).AddRow(
				"some_schema",
				"some_digest",
				"some_query_text",
				lastSeen.Add(time.Second*5),
			))
			err := c.populateQueryCache(t.Context())
			require.NoError(t, err)
		})

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})
}

func TestExplainPlansSkipsTruncatedQueries(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lastSeen := time.Now().Add(-time.Hour)
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	logBuffer := syncbuffer.Buffer{}
	logger, err := logging.New(&logBuffer, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          logger.Slog(),
		ScrapeInterval:  time.Second,
		PerScrapeRatio:  1,
		EntryHandler:    lokiClient,
		DBVersion:       "8.0.32",
		InitialLookback: lastSeen,
	})
	require.NoError(t, err)

	mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"schema_name",
		"digest",
		"query_sample_text",
		"last_seen",
	}).AddRow(
		"some_schema",
		"some_digest",
		"select * from some_table where ...",
		lastSeen,
	))

	require.NoError(t, c.fetchExplainPlans(t.Context()))

	require.Eventually(
		t,
		func() bool { return len(lokiClient.Received()) == 1 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the explain plan output log message within the timeout",
	)
	require.NotContains(t, logBuffer.String(), "error")
	lokiEntries := lokiClient.Received()
	require.Equal(t, 1, len(lokiEntries))
	epo, err := database_observability.ExtractExplainPlanOutputFromLogMsg(lokiEntries[0])
	require.NoError(t, err)
	require.Equal(t, database_observability.ExplainProcessingResultSkipped, epo.Metadata.ProcessingResult)
	require.Equal(t, "query is truncated", epo.Metadata.ProcessingResultReason)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExplainPlansSkipsNonSelectQueries(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lastSeen := time.Now().Add(-time.Hour)
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	logBuffer := syncbuffer.Buffer{}
	logger, err := logging.New(&logBuffer, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          logger.Slog(),
		ScrapeInterval:  time.Second,
		PerScrapeRatio:  1,
		EntryHandler:    lokiClient,
		DBVersion:       "8.0.32",
		InitialLookback: lastSeen,
	})
	require.NoError(t, err)

	mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"schema_name",
		"digest",
		"query_sample_text",
		"last_seen",
	}).AddRow(
		"some_schema",
		"update_digest",
		"update some_table set col = 1 where id = 1",
		lastSeen,
	).AddRow(
		"some_schema",
		"delete_digest",
		"delete from some_table",
		lastSeen,
	).AddRow(
		"some_schema",
		"insert_digest",
		"insert into some_table (col) values (1)",
		lastSeen,
	).AddRow(
		"some_schema",
		"show_digest",
		"show global status like 'Uptime'",
		lastSeen,
	).AddRow(
		"some_schema",
		"kill_digest",
		"kill query 123",
		lastSeen,
	).AddRow(
		"some_schema",
		"call_digest",
		"call refresh_summary()",
		lastSeen,
	).AddRow(
		"some_schema",
		"do_digest",
		"do release_lock('foo')",
		lastSeen,
	))

	require.NoError(t, c.fetchExplainPlans(t.Context()))

	require.Eventually(
		t,
		func() bool { return len(lokiClient.Received()) == 7 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the explain plan output log messages within the timeout",
	)

	lokiEntries := lokiClient.Received()
	require.Equal(t, 7, len(lokiEntries))

	for _, lokiEntry := range lokiEntries {
		ep, err := database_observability.ExtractExplainPlanOutputFromLogMsg(lokiEntry)
		require.NoError(t, err)
		require.Equal(t, database_observability.ExplainProcessingResultSkipped, ep.Metadata.ProcessingResult)
		require.Equal(t, "query contains reserved word", ep.Metadata.ProcessingResultReason)
	}

	require.NotContains(t, logBuffer.String(), "error")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExplainPlansSkipsNoRowResult(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lastSeen := time.Now().Add(-time.Hour)
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	logBuffer := syncbuffer.Buffer{}
	logger, err := logging.New(&logBuffer, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          logger.Slog(),
		ScrapeInterval:  time.Second,
		PerScrapeRatio:  1,
		EntryHandler:    lokiClient,
		DBVersion:       "8.0.32",
		InitialLookback: lastSeen,
	})
	require.NoError(t, err)

	mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"schema_name",
		"digest",
		"query_sample_text",
		"last_seen",
	}).AddRow(
		"some_schema",
		"some_digest",
		"select * from some_table where id = 1",
		lastSeen,
	))

	mock.ExpectExec("USE `some_schema`").WithoutArgs().WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(selectExplainPlanPrefix + "select * from some_table where id = 1").WillReturnRows(sqlmock.NewRows([]string{
		"json",
	}).AddRow(
		[]byte(`{"query_block": {"message": "no matching row in const table"}}`),
	))

	require.NoError(t, c.fetchExplainPlans(t.Context()))

	require.NotContains(t, logBuffer.String(), "error")
	require.Contains(t, logBuffer.String(), "no matching row in const table")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExplainPlansPassesQueriesBeginningInSelect(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lastSeen := time.Now().Add(-time.Hour)
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	logBuffer := syncbuffer.Buffer{}
	logger, err := logging.New(&logBuffer, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          logger.Slog(),
		ScrapeInterval:  time.Second,
		PerScrapeRatio:  1,
		EntryHandler:    lokiClient,
		DBVersion:       "8.0.32",
		InitialLookback: lastSeen,
	})
	require.NoError(t, err)

	mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"schema_name",
		"digest",
		"query_sample_text",
		"last_seen",
	}).AddRow(
		"some_schema",
		"some_digest",
		"select * from some_table where id = 1",
		lastSeen,
	))

	mock.ExpectExec("USE `some_schema`").WithoutArgs().WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(selectExplainPlanPrefix + "select * from some_table where id = 1").WillReturnRows(sqlmock.NewRows([]string{
		"json",
	}).AddRow(
		[]byte(`{"query_block": {"select_id": 1}}`),
	))

	require.NoError(t, c.fetchExplainPlans(t.Context()))

	require.NotContains(t, logBuffer.String(), "error")

	require.Eventually(
		t,
		func() bool { return len(lokiClient.Received()) == 1 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the explain plan output log message within the timeout",
	)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExplainPlansPassesQueriesBeginningInWith(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lastSeen := time.Now().Add(-time.Hour)
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	logBuffer := syncbuffer.Buffer{}
	logger, err := logging.New(&logBuffer, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          logger.Slog(),
		ScrapeInterval:  time.Second,
		PerScrapeRatio:  1,
		EntryHandler:    lokiClient,
		DBVersion:       "8.0.32",
		InitialLookback: lastSeen,
	})
	require.NoError(t, err)

	mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"schema_name",
		"digest",
		"query_sample_text",
		"last_seen",
	}).AddRow(
		"some_schema",
		"some_digest",
		"with cte as (select * from some_table where id = 1) select * from cte",
		lastSeen,
	))

	mock.ExpectExec("USE `some_schema`").WithoutArgs().WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(selectExplainPlanPrefix + "with cte as (select * from some_table where id = 1) select * from cte").WillReturnRows(sqlmock.NewRows([]string{
		"json",
	}).AddRow(
		[]byte(`{"query_block": {"select_id": 1}}`),
	))

	require.NoError(t, c.fetchExplainPlans(t.Context()))

	require.NotContains(t, logBuffer.String(), "error")

	require.Eventually(
		t,
		func() bool { return len(lokiClient.Received()) == 1 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the explain plan output log message within the timeout",
	)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExplainPlansThrottling(t *testing.T) {
	base := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)

	t.Run("all emitted results start the throttle", func(t *testing.T) {
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		c, err := NewExplainPlans(ExplainPlansArguments{
			Logger:       util.TestAlloyLogger(t).Slog(),
			EntryHandler: lokiClient,
			DBVersion:    "8.0.32",
		})
		require.NoError(t, err)
		c.now = func() time.Time { return base }

		results := []database_observability.ExplainProcessingResult{
			database_observability.ExplainProcessingResultSuccess,
			database_observability.ExplainProcessingResultSkipped,
			database_observability.ExplainProcessingResultError,
		}
		for i, result := range results {
			digest := fmt.Sprintf("digest_%d", i)
			require.NoError(t, c.sendExplainPlansOutput(
				"some_schema",
				digest,
				base.Format(time.RFC3339),
				result,
				"",
				nil,
			))
			require.Equal(t, base, c.lastEmittedAt[explainPlanQueryKey("some_schema", digest)])
		}

		lokiClient.Stop()
		require.Len(t, lokiClient.Received(), len(results))
	})

	t.Run("skips explain within interval and retries after interval", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		c, err := NewExplainPlans(ExplainPlansArguments{
			DB:             db,
			Logger:         util.TestAlloyLogger(t).Slog(),
			PerScrapeRatio: 1,
			EntryHandler:   lokiClient,
			DBVersion:      "8.0.32",
		})
		require.NoError(t, err)

		fakeNow := base
		c.now = func() time.Time { return fakeNow }
		qi := newQueryInfo("some_schema", "some_digest", "select * from some_table")
		explainJSON := []byte(`{"query_block":{"table":{"table_name":"some_table","access_type":"ALL"}}}`)

		expectExplain := func() {
			mock.ExpectExec("USE `some_schema`").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery(selectExplainPlanPrefix + qi.queryText).
				WillReturnRows(sqlmock.NewRows([]string{"json"}).AddRow(explainJSON))
		}
		queueQuery := func() {
			c.queryCache[qi.uniqueKey] = qi
			c.currentBatchSize = 1
		}

		expectExplain()
		queueQuery()
		require.NoError(t, c.fetchExplainPlans(t.Context()))
		require.Equal(t, base, c.lastEmittedAt[qi.uniqueKey])
		require.NoError(t, mock.ExpectationsWereMet())

		fakeNow = base.Add(time.Minute)
		queueQuery()
		require.NoError(t, c.fetchExplainPlans(t.Context()))
		require.Empty(t, c.queryCache)
		require.Equal(t, base, c.lastEmittedAt[qi.uniqueKey])
		require.NoError(t, mock.ExpectationsWereMet())

		fakeNow = base.Add(database_observability.EmitInterval + time.Minute)
		expectExplain()
		queueQuery()
		require.NoError(t, c.fetchExplainPlans(t.Context()))
		require.Equal(t, fakeNow, c.lastEmittedAt[qi.uniqueKey])
		require.NoError(t, mock.ExpectationsWereMet())
		lokiClient.Stop()
		require.Len(t, lokiClient.Received(), 2)
	})

	t.Run("filters throttled discoveries without consuming batch capacity", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		c, err := NewExplainPlans(ExplainPlansArguments{
			DB:              db,
			Logger:          util.TestAlloyLogger(t).Slog(),
			PerScrapeRatio:  1,
			InitialLookback: base.Add(-time.Hour),
		})
		require.NoError(t, err)

		fakeNow := base.Add(time.Minute)
		c.now = func() time.Time { return fakeNow }
		throttledKey := explainPlanQueryKey("some_schema", "throttled_digest")
		c.lastEmittedAt[throttledKey] = base

		nextSeen := base.Add(2 * time.Minute)
		mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).
			WithArgs(base.Add(-time.Hour)).
			WillReturnRows(sqlmock.NewRows([]string{"schema_name", "digest", "query_sample_text", "last_seen"}).
				AddRow("some_schema", "throttled_digest", "select * from throttled_table", base).
				AddRow("some_schema", "due_digest", "select * from due_table", nextSeen))

		require.NoError(t, c.populateQueryCache(t.Context()))
		require.NotContains(t, c.queryCache, throttledKey)
		require.Contains(t, c.queryCache, explainPlanQueryKey("some_schema", "due_digest"))
		require.Equal(t, 1, c.currentBatchSize)
		require.Equal(t, nextSeen, c.lastSeen)
		require.NoError(t, mock.ExpectationsWereMet())

		fakeNow = base.Add(database_observability.EmitInterval)
		c.pruneExpiredThrottle()
		require.NotContains(t, c.lastEmittedAt, throttledKey)
	})

	t.Run("throttles denylist output", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		c, err := NewExplainPlans(ExplainPlansArguments{
			DB:              db,
			Logger:          util.TestAlloyLogger(t).Slog(),
			PerScrapeRatio:  1,
			InitialLookback: base.Add(-time.Hour),
			EntryHandler:    lokiClient,
			DBVersion:       "8.0.32",
		})
		require.NoError(t, err)
		c.now = func() time.Time { return base }

		key := explainPlanQueryKey("some_schema", "some_digest")
		c.queryDenylist[key] = struct{}{}
		require.NoError(t, c.sendExplainPlansOutput(
			"some_schema",
			"some_digest",
			base.Format(time.RFC3339),
			database_observability.ExplainProcessingResultSkipped,
			"query denylisted",
			nil,
		))

		mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).
			WithArgs(base.Add(-time.Hour)).
			WillReturnRows(sqlmock.NewRows([]string{"schema_name", "digest", "query_sample_text", "last_seen"}).
				AddRow("some_schema", "some_digest", "select * from some_table", base))

		require.NoError(t, c.populateQueryCache(t.Context()))
		require.Empty(t, c.queryCache)
		require.Equal(t, base, c.lastSeen)
		require.Equal(t, base, c.lastEmittedAt[key])
		require.NoError(t, mock.ExpectationsWereMet())
		lokiClient.Stop()
		require.Len(t, lokiClient.Received(), 1)
	})
}

func TestQueryFailureDenylist(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lastSeen := time.Now().Add(-time.Hour)
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	logBuffer := syncbuffer.Buffer{}
	logger, err := logging.New(&logBuffer, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	queryUnderTestHash := explainPlanQueryKey("some_schema", "some_digest1")

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          logger.Slog(),
		ScrapeInterval:  time.Second,
		PerScrapeRatio:  1,
		EntryHandler:    lokiClient,
		InitialLookback: lastSeen,
	})
	require.NoError(t, err)

	mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"schema_name",
		"digest",
		"query_sample_text",
		"last_seen",
	}).AddRow(
		"some_schema",
		"some_digest1",
		"select * from some_table where id = 1",
		lastSeen,
	))

	c.populateQueryCache(t.Context())

	t.Run("non-recoverable sql error denylists query", func(t *testing.T) {
		lokiClient.Clear()
		logBuffer.Reset()

		mock.ExpectExec("USE `some_schema`").WithoutArgs().WillReturnResult(sqlmock.NewResult(0, 0))

		mock.ExpectQuery(selectExplainPlanPrefix + "select * from some_table where id = 1").WillReturnError(fmt.Errorf("Error 1044: Access denied for user 'some_user'@'some_host' to database 'some_schema'"))

		err = c.fetchExplainPlans(t.Context())
		require.NoError(t, err)
		require.Equal(t, 0, len(c.queryCache))
		require.Equal(t, 1, len(c.queryDenylist))
		require.Contains(t, c.queryDenylist, queryUnderTestHash)
		require.Empty(t, c.lastEmittedAt, "a failure without explain_plan_output must remain unthrottled")
	})

	t.Run("denylisted queries are not added to query cache", func(t *testing.T) {
		lokiClient.Clear()
		logBuffer.Reset()

		mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, exclusionClause)).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
			"schema_name",
			"digest",
			"query_sample_text",
			"last_seen",
		}).AddRow(
			"some_schema",
			"some_digest1",
			"select * from some_table where id = 1",
			lastSeen,
		).AddRow(
			"some_schema",
			"some_digest2",
			"select * from some_table where id = 2",
			lastSeen,
		))

		err = c.populateQueryCache(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, len(c.queryCache))
		require.Equal(t, 1, len(c.queryDenylist))

		mock.ExpectExec("USE `some_schema`").WithoutArgs().WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(selectExplainPlanPrefix + "select * from some_table where id = 2").WillReturnRows(sqlmock.NewRows([]string{
			"json",
		}).AddRow(
			[]byte(`{"query_block": {"select_id": 1}}`),
		))

		err = c.fetchExplainPlans(t.Context())
		require.NoError(t, err)
		require.Equal(t, 0, len(c.queryCache))
		require.Equal(t, 1, len(c.queryDenylist))
	})
}

func TestBatchSizeLimitsProcessing(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:             db,
		Logger:         util.TestAlloyLogger(t).Slog(),
		ScrapeInterval: time.Second,
		PerScrapeRatio: 1,
		EntryHandler:   lokiClient,
		DBVersion:      "8.0.32",
	})
	require.NoError(t, err)

	c.queryCache = map[string]*queryInfo{
		explainPlanQueryKey("s1", "d1"): newQueryInfo("s1", "d1", "select * from t1 where ..."),
		explainPlanQueryKey("s1", "d2"): newQueryInfo("s1", "d2", "select * from t2 where ..."),
		explainPlanQueryKey("s1", "d3"): newQueryInfo("s1", "d3", "select * from t3 where ..."),
		explainPlanQueryKey("s1", "d4"): newQueryInfo("s1", "d4", "select * from t4 where ..."),
	}
	c.currentBatchSize = 2

	err = c.fetchExplainPlans(t.Context())
	require.NoError(t, err)

	require.Equal(t, 2, len(c.queryCache), "batch size limit should leave unprocessed items in cache")

	require.Eventually(
		t,
		func() bool { return len(lokiClient.Received()) == 2 },
		5*time.Second, 10*time.Millisecond,
		"expected exactly 2 loki entries (one per processed item), got %d", len(lokiClient.Received()),
	)

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestSchemaDenylist(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lastSeen := time.Now().Add(-time.Hour)
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	logBuffer := syncbuffer.Buffer{}
	logger, err := logging.New(&logBuffer, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          logger.Slog(),
		ScrapeInterval:  time.Second,
		PerScrapeRatio:  1,
		ExcludeSchemas:  []string{"some_schema"},
		EntryHandler:    lokiClient,
		InitialLookback: lastSeen,
	})
	require.NoError(t, err)

	mock.ExpectQuery(fmt.Sprintf(selectDigestsForExplainPlan, buildExcludedSchemasClause([]string{"some_schema"}))).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"schema_name",
		"digest",
		"query_sample_text",
		"last_seen",
	}).AddRow(
		"different_schema",
		"some_digest2",
		"select * from some_table where id = 2",
		lastSeen,
	))

	c.populateQueryCache(t.Context())
	require.Equal(t, 1, len(c.queryCache))
	require.Equal(t, 0, len(c.queryDenylist))
}
