package collector

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/util/syncbuffer"
)

func stringPtr(s string) *string {
	return &s
}

func floatPtr(f float64) *float64 {
	return &f
}

func explainPlanAccessTypePtr(s database_observability.ExplainPlanAccessType) *database_observability.ExplainPlanAccessType {
	return &s
}

func explainPlanJoinAlgorithmPtr(s database_observability.ExplainPlanJoinAlgorithm) *database_observability.ExplainPlanJoinAlgorithm {
	return &s
}

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
		notJsonData := []byte("not json data")
		logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
		_, err := newExplainPlansOutput(logger, notJsonData)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to get query block: Key path not found")
	})

	t.Run("unknown operation", func(t *testing.T) {
		logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
		explainPlanOutput, err := newExplainPlansOutput(logger, []byte("{\"query_block\": {\"operation\": \"some unknown thing we've never seen before.\"}}"))
		require.NoError(t, err)
		require.Equal(t, database_observability.ExplainPlanOutputOperationUnknown, explainPlanOutput.Operation)
	})

	t.Run("zero rows", func(t *testing.T) {
		logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
		_, err := newExplainPlansOutput(logger, []byte("{\"query_block\": {\"message\": \"no matching row in const table\"}}"))
		require.NoError(t, err)
	})

	currentTime := time.Now().Format(time.RFC3339)
	tests := []struct {
		dbVersion string
		digest    string
		fname     string
		result    *database_observability.ExplainPlanOutput
	}{
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "complex_aggregation_with_case",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
									Details: database_observability.ExplainPlanNodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("d"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeIndex),
												EstimatedRows: 9,
												EstimatedCost: floatPtr(1.90),
												KeyUsed:       stringPtr("dept_name"),
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("de"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
												EstimatedRows: 37253,
												EstimatedCost: floatPtr(57154.49),
												KeyUsed:       stringPtr("dept_no"),
												Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
											},
										},
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
										EstimatedRows: 37253,
										EstimatedCost: floatPtr(98133.43),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "complex_join_with_aggregate_subquery",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
									Details: database_observability.ExplainPlanNodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("d"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeIndex),
												EstimatedRows: 9,
												EstimatedCost: floatPtr(1.90),
												KeyUsed:       stringPtr("dept_name"),
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("de"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
												EstimatedRows: 37253,
												EstimatedCost: floatPtr(57154.49),
												KeyUsed:       stringPtr("dept_no"),
												Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
											},
										},
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
										EstimatedRows: 37253,
										EstimatedCost: floatPtr(98133.43),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "complex_query_with_multiple_conditions_and_functions",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationOrderingOperation,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
									Details: database_observability.ExplainPlanNodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
											Details: database_observability.ExplainPlanNodeDetails{
												JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
													Details: database_observability.ExplainPlanNodeDetails{
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
															Details: database_observability.ExplainPlanNodeDetails{
																JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperationTableScan,
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("de"),
																		AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
																		EstimatedRows: 33114,
																		EstimatedCost: floatPtr(33851.30),
																		Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
																	},
																},
																{
																	Operation: database_observability.ExplainPlanOutputOperationTableScan,
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("t"),
																		AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
																		EstimatedRows: 4920,
																		EstimatedCost: floatPtr(71886.48),
																		KeyUsed:       stringPtr("PRIMARY"),
																		Condition:     stringPtr("( `employees` . `t` . `to_date` = date ? )"),
																	},
																},
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperationTableScan,
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("d"),
																AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
																EstimatedRows: 4920,
																EstimatedCost: floatPtr(77299.46),
																KeyUsed:       stringPtr("PRIMARY"),
															},
														},
													},
												},
												{
													Operation: database_observability.ExplainPlanOutputOperationTableScan,
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("e"),
														AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
														EstimatedRows: 1640,
														EstimatedCost: floatPtr(82708.26),
														KeyUsed:       stringPtr("PRIMARY"),
														Condition:     stringPtr("( `employees` . `e` . `hire_date` > date ? )"),
													},
												},
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("s"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
												EstimatedRows: 1542,
												EstimatedCost: floatPtr(85905.59),
												KeyUsed:       stringPtr("PRIMARY"),
												Condition:     stringPtr("( `employees` . `s` . `to_date` = date ? )"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "complex_subquery_in_select_clause",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationTableScan,
					Details: database_observability.ExplainPlanNodeDetails{
						Alias:         stringPtr("e"),
						AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRange),
						EstimatedRows: 49,
						EstimatedCost: floatPtr(10.86),
						KeyUsed:       stringPtr("PRIMARY"),
						Condition:     stringPtr("( `employees` . `e` . `emp_no` < ? )"),
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "conditional_aggregation_with_case",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationOrderingOperation,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("employees"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
										EstimatedRows: 299556,
										EstimatedCost: floatPtr(30884.60),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "correlated_subquery",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
					Details: database_observability.ExplainPlanNodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
										EstimatedRows: 299556,
										EstimatedCost: floatPtr(30884.60),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("t"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
										EstimatedRows: 44514,
										EstimatedCost: floatPtr(374955.51),
										KeyUsed:       stringPtr("PRIMARY"),
										Condition:     stringPtr("( `employees` . `t` . `to_date` = date ? )"),
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperationTableScan,
							Details: database_observability.ExplainPlanNodeDetails{
								Alias:      stringPtr("<subquery2>"),
								AccessType: explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
								KeyUsed:    stringPtr("<auto_distinct_key>"),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationMaterializedSubquery,
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
											Details: database_observability.ExplainPlanNodeDetails{
												JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperationTableScan,
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("salaries"),
														AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
														EstimatedRows: 94604,
														EstimatedCost: floatPtr(289962.60),
														Condition:     stringPtr("( ( `employees` . `salaries` . `to_date` = date ? ) and ( `employees` . `salaries` . `salary` > ? ) )"),
													},
												},
												{
													Operation: database_observability.ExplainPlanOutputOperationTableScan,
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("titles"),
														AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
														EstimatedRows: 140585,
														EstimatedCost: floatPtr(400924.92),
														KeyUsed:       stringPtr("PRIMARY"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "date_manipulation_with_conditions",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationTableScan,
					Details: database_observability.ExplainPlanNodeDetails{
						Alias:         stringPtr("e"),
						AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
						EstimatedRows: 99842,
						EstimatedCost: floatPtr(30884.60),
						Condition:     stringPtr("( ( month ( `employees` . `e` . `hire_date` ) = < cache > ( month ( curdate ( ) ) ) ) and ( `employees` . `e` . `hire_date` < date ? ) )"),
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "derived_table_with_aggregates",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
					Details: database_observability.ExplainPlanNodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
									Details: database_observability.ExplainPlanNodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("de"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
												EstimatedRows: 33114,
												EstimatedCost: floatPtr(33851.30),
												Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("s"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
												EstimatedRows: 31146,
												EstimatedCost: floatPtr(98405.51),
												KeyUsed:       stringPtr("PRIMARY"),
												Condition:     stringPtr("( `employees` . `s` . `to_date` = date ? )"),
											},
										},
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
										EstimatedRows: 31146,
										EstimatedCost: floatPtr(132640.69),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperationTableScan,
							Details: database_observability.ExplainPlanNodeDetails{
								Alias:         stringPtr("dept_salary_stats"),
								AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
								EstimatedRows: 138443,
								EstimatedCost: floatPtr(278020.70),
								KeyUsed:       stringPtr("<auto_key1>"),
								Condition:     stringPtr("( `employees` . `s` . `salary` > `dept_salary_stats` . `avg_salary` )"),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationMaterializedSubquery,
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
													Details: database_observability.ExplainPlanNodeDetails{
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
															Details: database_observability.ExplainPlanNodeDetails{
																JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperationTableScan,
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("de"),
																		AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
																		EstimatedRows: 33114,
																		EstimatedCost: floatPtr(33851.30),
																		Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
																	},
																},
																{
																	Operation: database_observability.ExplainPlanOutputOperationTableScan,
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("s"),
																		AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
																		EstimatedRows: 31146,
																		EstimatedCost: floatPtr(98405.51),
																		KeyUsed:       stringPtr("PRIMARY"),
																		Condition:     stringPtr("( `employees` . `s` . `to_date` = date ? )"),
																	},
																},
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperationTableScan,
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("d"),
																AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
																EstimatedRows: 31146,
																EstimatedCost: floatPtr(132667.05),
																KeyUsed:       stringPtr("PRIMARY"),
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "distinct_with_multiple_joins",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationOrderingOperation,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationDuplicatesRemoval,
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
									Details: database_observability.ExplainPlanNodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
											Details: database_observability.ExplainPlanNodeDetails{
												JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperationTableScan,
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("de"),
														AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
														EstimatedRows: 33114,
														EstimatedCost: floatPtr(33851.30),
														Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
													},
												},
												{
													Operation: database_observability.ExplainPlanOutputOperationTableScan,
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("t"),
														AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
														EstimatedRows: 4920,
														EstimatedCost: floatPtr(71886.48),
														KeyUsed:       stringPtr("PRIMARY"),
														Condition:     stringPtr("( `employees` . `t` . `to_date` = date ? )"),
													},
												},
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("d"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
												EstimatedRows: 4920,
												EstimatedCost: floatPtr(77299.46),
												KeyUsed:       stringPtr("PRIMARY"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "group_by_with_having",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("d"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeIndex),
										EstimatedRows: 9,
										EstimatedCost: floatPtr(1.90),
										KeyUsed:       stringPtr("dept_name"),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("de"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
										EstimatedRows: 37253,
										EstimatedCost: floatPtr(57154.49),
										KeyUsed:       stringPtr("dept_no"),
										Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "join_and_order",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationOrderingOperation,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
									Details: database_observability.ExplainPlanNodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("d"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeIndex),
												EstimatedRows: 9,
												EstimatedCost: floatPtr(1.90),
												KeyUsed:       stringPtr("dept_name"),
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("de"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
												EstimatedRows: 37253,
												EstimatedCost: floatPtr(57154.49),
												KeyUsed:       stringPtr("dept_no"),
												Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
											},
										},
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
										EstimatedRows: 37253,
										EstimatedCost: floatPtr(98133.43),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "multiple_aggregate_functions_with_having",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("t"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
										EstimatedRows: 44260,
										EstimatedCost: floatPtr(45512.50),
										Condition:     stringPtr("( `employees` . `t` . `to_date` = date ? )"),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("s"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
										EstimatedRows: 41630,
										EstimatedCost: floatPtr(131795.52),
										KeyUsed:       stringPtr("PRIMARY"),
										Condition:     stringPtr("( `employees` . `s` . `to_date` = date ? )"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "multiple_joins_with_date_functions",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
					Details: database_observability.ExplainPlanNodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("d"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeIndex),
										EstimatedRows: 9,
										EstimatedCost: floatPtr(1.90),
										KeyUsed:       stringPtr("dept_name"),
										// JoinType is unknown, we could run the tree explain plan as well to determine this.
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("de"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
										EstimatedRows: 37253,
										EstimatedCost: floatPtr(57154.49),
										KeyUsed:       stringPtr("dept_no"),
										Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
										// JoinType is unknown, we could run the tree explain plan as well to determine this.
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperationTableScan,
							Details: database_observability.ExplainPlanNodeDetails{
								Alias:         stringPtr("e"),
								AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
								EstimatedRows: 37253,
								EstimatedCost: floatPtr(98133.43),
								KeyUsed:       stringPtr("PRIMARY"),
								Condition:     stringPtr("( year ( `employees` . `e` . `hire_date` ) = ? )"),
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "nested_subqueries_with_exists",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
					Details: database_observability.ExplainPlanNodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("dm"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeIndex),
										EstimatedRows: 24,
										EstimatedCost: floatPtr(3.51),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("s"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeRef),
										EstimatedRows: 7,
										EstimatedCost: floatPtr(50.19),
										KeyUsed:       stringPtr("PRIMARY"),
										Condition:     stringPtr("( ( `employees` . `s` . `to_date` = date ? ) and ( `employees` . `s` . `salary` > ? ) )"),
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperationTableScan,
							Details: database_observability.ExplainPlanNodeDetails{
								Alias:         stringPtr("e"),
								AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
								EstimatedRows: 1,
								EstimatedCost: floatPtr(58.57),
								KeyUsed:       stringPtr("PRIMARY"),
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "self_join_with_date_comparison",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
					Details: database_observability.ExplainPlanNodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationHashJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
									Details: database_observability.ExplainPlanNodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("de1"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
												EstimatedRows: 33114,
												EstimatedCost: floatPtr(33851.30),
												Condition:     stringPtr("( `employees` . `de1` . `to_date` = date ? )"),
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("e1"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
												EstimatedRows: 33114,
												EstimatedCost: floatPtr(70249.00),
												KeyUsed:       stringPtr("PRIMARY"),
											},
										},
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("de2"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
										EstimatedRows: 137069612,
										EstimatedCost: floatPtr(110342868.42),
										Condition:     stringPtr("( ( `employees` . `de2` . `dept_no` = `employees` . `de1` . `dept_no` ) and ( `employees` . `de2` . `to_date` = date ? ) and ( `employees` . `de1` . `emp_no` < `employees` . `de2` . `emp_no` ) )"),
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperationTableScan,
							Details: database_observability.ExplainPlanNodeDetails{
								Alias:         stringPtr("e2"),
								AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
								EstimatedRows: 13706961,
								EstimatedCost: floatPtr(124053965.42),
								KeyUsed:       stringPtr("PRIMARY"),
								Condition:     stringPtr("( `employees` . `e2` . `hire_date` = `employees` . `e1` . `hire_date` )"),
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "string_functions_with_grouping",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationOrderingOperation,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("employees"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
										EstimatedRows: 299556,
										EstimatedCost: floatPtr(30884.60),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "subquery_with_aggregate",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
					Details: database_observability.ExplainPlanNodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationTableScan,
							Details: database_observability.ExplainPlanNodeDetails{
								Alias:         stringPtr("s"),
								AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
								EstimatedRows: 94604,
								EstimatedCost: floatPtr(289962.60),
								Condition:     stringPtr("( ( `employees` . `s` . `to_date` = date ? ) and ( `employees` . `s` . `salary` > ( select ( avg ( `employees` . `salaries` . `salary` ) * ? ) from `employees` . `salaries` ) ) )"),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationAttachedSubquery,
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperationTableScan,
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("salaries"),
												AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
												EstimatedRows: 2838426,
												EstimatedCost: floatPtr(289962.60),
											},
										},
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperationTableScan,
							Details: database_observability.ExplainPlanNodeDetails{
								Alias:         stringPtr("e"),
								AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
								EstimatedRows: 94604,
								EstimatedCost: floatPtr(394027.81),
								KeyUsed:       stringPtr("PRIMARY"),
							},
						},
					},
				},
			},
		},
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "union_with_different_conditions",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:   "MySQL",
					DatabaseVersion:  "8.0.32",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperationUnion,
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("dm"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
										EstimatedRows: 2,
										EstimatedCost: floatPtr(3.40),
										Condition:     stringPtr("( `employees` . `dm` . `to_date` = date ? )"),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
										EstimatedRows: 2,
										EstimatedCost: floatPtr(6.04),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
							Details: database_observability.ExplainPlanNodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("t"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeAll),
										EstimatedRows: 4426,
										EstimatedCost: floatPtr(45512.50),
										Condition:     stringPtr("( ( `employees` . `t` . `to_date` = date ? ) and ( `employees` . `t` . `title` = ? ) )"),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperationTableScan,
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
										EstimatedRows: 4426,
										EstimatedCost: floatPtr(50381.16),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
							},
						},
					},
				},
			},
		},
		// TODO: Window functions yet. However, mysql workbench also does not visualize them. Maybe we can add support for them in the future.
	}

	for _, test := range tests {
		t.Run(test.fname, func(t *testing.T) {
			archive, err := txtar.ParseFile(fmt.Sprintf("./testdata/explain_plan/%s.txtar", test.fname))
			require.NoError(t, err)
			require.Equal(t, 1, len(archive.Files))
			jsonFile := archive.Files[0]
			require.Equal(t, fmt.Sprintf("%s.json", test.fname), jsonFile.Name)
			jsonData := jsonFile.Data
			logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
			output, err := newExplainPlansOutput(logger, jsonData)
			require.NoError(t, err, "Failed generate explain plan output: %s", test.fname)
			require.Equal(t, test.result.Plan, *output)
		})
	}
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
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout)),
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

	t.Run("query validation", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lastSeen := time.Now().Add(-time.Hour)
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		logBuffer := syncbuffer.Buffer{}

		c, err := NewExplainPlans(ExplainPlansArguments{
			DB:              db,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			ScrapeInterval:  time.Second,
			PerScrapeRatio:  1,
			EntryHandler:    lokiClient,
			DBVersion:       "8.0.32",
			InitialLookback: lastSeen,
		})
		require.NoError(t, err)

		t.Run("skips truncated queries", func(t *testing.T) {
			logBuffer.Reset()
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

			err = c.fetchExplainPlans(t.Context())
			require.NoError(t, err)

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
			lokiClient.Clear()
		})

		t.Run("skips non-select queries", func(t *testing.T) {
			lokiClient.Clear()
			logBuffer.Reset()
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
			))

			err = c.fetchExplainPlans(t.Context())
			require.NoError(t, err)

			require.Eventually(
				t,
				func() bool { return len(lokiClient.Received()) == 3 },
				5*time.Second,
				10*time.Millisecond,
				"did not receive the explain plan output log messages within the timeout",
			)

			lokiEntries := lokiClient.Received()
			require.Equal(t, 3, len(lokiEntries))

			for _, lokiEntry := range lokiEntries {
				ep, err := database_observability.ExtractExplainPlanOutputFromLogMsg(lokiEntry)
				require.NoError(t, err)
				require.Equal(t, database_observability.ExplainProcessingResultSkipped, ep.Metadata.ProcessingResult)
				require.Equal(t, "query contains reserved word", ep.Metadata.ProcessingResultReason)
			}

			require.NotContains(t, logBuffer.String(), "error")
			lokiClient.Clear()
		})

		t.Run("skips no row result", func(t *testing.T) {
			logBuffer.Reset()
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

			err = c.fetchExplainPlans(t.Context())
			require.NoError(t, err)

			lokiClient.Clear()

			require.NotContains(t, logBuffer.String(), "error")
			require.Contains(t, logBuffer.String(), "no matching row in const table")
		})

		t.Run("passes queries beginning in select", func(t *testing.T) {
			lokiClient.Clear()
			logBuffer.Reset()
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

			err = c.fetchExplainPlans(t.Context())
			require.NoError(t, err)

			require.NotContains(t, logBuffer.String(), "error")

			require.Eventually(
				t,
				func() bool { return len(lokiClient.Received()) == 1 },
				5*time.Second,
				10*time.Millisecond,
				"did not receive the explain plan output log message within the timeout",
			)
		})

		t.Run("passes queries beginning in with", func(t *testing.T) {
			lokiClient.Clear()
			logBuffer.Reset()
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

			err = c.fetchExplainPlans(t.Context())
			require.NoError(t, err)

			require.NotContains(t, logBuffer.String(), "error")

			require.Eventually(
				t,
				func() bool { return len(lokiClient.Received()) == 1 },
				5*time.Second,
				10*time.Millisecond,
				"did not receive the explain plan output log message within the timeout",
			)
		})

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
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

	queryUnderTestHash := "some_schemasome_digest1"

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
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
		require.Equal(t, 1, c.queryDenylist[queryUnderTestHash].failureCount)
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

func TestSchemaDenylist(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lastSeen := time.Now().Add(-time.Hour)
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	logBuffer := syncbuffer.Buffer{}

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
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
