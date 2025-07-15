package collector

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"

	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
)

func stringPtr(s string) *string {
	return &s
}

func floatPtr(f float64) *float64 {
	return &f
}

func explainPlanAccessTypePtr(s explainPlanAccessType) *explainPlanAccessType {
	return &s
}

func explainPlanJoinAlgorithmPtr(s explainPlanJoinAlgorithm) *explainPlanJoinAlgorithm {
	return &s
}

func TestExplainPlanRedactor(t *testing.T) {
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

func TestExplainPlanOutput(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		notJsonData := []byte("not json data")
		logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
		_, err := newExplainPlanOutput(logger, "", "", notJsonData, "")
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to get query block: Key path not found")
	})

	t.Run("unknown operation", func(t *testing.T) {
		logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
		explainPlanOutput, err := newExplainPlanOutput(logger, "", "", []byte("{\"query_block\": {\"operation\": \"some unknown thing we've never seen before.\"}}"), "")
		require.NoError(t, err)
		require.Equal(t, explainPlanOutputOperationUnknown, explainPlanOutput.Plan.Operation)
	})

	currentTime := time.Now().Format(time.RFC3339)
	tests := []struct {
		dbVersion string
		digest    string
		fname     string
		result    *explainPlanOutput
	}{
		{
			dbVersion: "8.0.32",
			digest:    "1234567890",
			fname:     "complex_aggregation_with_case",
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationGroupingOperation,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationNestedLoopJoin,
									Details: nodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
									},
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("d"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeIndex),
												EstimatedRows: 9,
												EstimatedCost: floatPtr(1.90),
												KeyUsed:       stringPtr("dept_name"),
											},
										},
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("de"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
												EstimatedRows: 37253,
												EstimatedCost: floatPtr(57154.49),
												KeyUsed:       stringPtr("dept_no"),
												Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
											},
										},
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationGroupingOperation,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationNestedLoopJoin,
									Details: nodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
									},
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("d"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeIndex),
												EstimatedRows: 9,
												EstimatedCost: floatPtr(1.90),
												KeyUsed:       stringPtr("dept_name"),
											},
										},
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("de"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
												EstimatedRows: 37253,
												EstimatedCost: floatPtr(57154.49),
												KeyUsed:       stringPtr("dept_no"),
												Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
											},
										},
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationOrderingOperation,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationGroupingOperation,
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationNestedLoopJoin,
									Details: nodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
									},
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationNestedLoopJoin,
											Details: nodeDetails{
												JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
											},
											Children: []planNode{
												{
													Operation: explainPlanOutputOperationNestedLoopJoin,
													Details: nodeDetails{
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
													},
													Children: []planNode{
														{
															Operation: explainPlanOutputOperationNestedLoopJoin,
															Details: nodeDetails{
																JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
															},
															Children: []planNode{
																{
																	Operation: explainPlanOutputOperationTableScan,
																	Details: nodeDetails{
																		Alias:         stringPtr("de"),
																		AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
																		EstimatedRows: 33114,
																		EstimatedCost: floatPtr(33851.30),
																		Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
																	},
																},
																{
																	Operation: explainPlanOutputOperationTableScan,
																	Details: nodeDetails{
																		Alias:         stringPtr("t"),
																		AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
																		EstimatedRows: 4920,
																		EstimatedCost: floatPtr(71886.48),
																		KeyUsed:       stringPtr("PRIMARY"),
																		Condition:     stringPtr("( `employees` . `t` . `to_date` = date ? )"),
																	},
																},
															},
														},
														{
															Operation: explainPlanOutputOperationTableScan,
															Details: nodeDetails{
																Alias:         stringPtr("d"),
																AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
																EstimatedRows: 4920,
																EstimatedCost: floatPtr(77299.46),
																KeyUsed:       stringPtr("PRIMARY"),
															},
														},
													},
												},
												{
													Operation: explainPlanOutputOperationTableScan,
													Details: nodeDetails{
														Alias:         stringPtr("e"),
														AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
														EstimatedRows: 1640,
														EstimatedCost: floatPtr(82708.26),
														KeyUsed:       stringPtr("PRIMARY"),
														Condition:     stringPtr("( `employees` . `e` . `hire_date` > date ? )"),
													},
												},
											},
										},
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("s"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationTableScan,
					Details: nodeDetails{
						Alias:         stringPtr("e"),
						AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRange),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationOrderingOperation,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationGroupingOperation,
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("employees"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationNestedLoopJoin,
					Details: nodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
					},
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
										EstimatedRows: 299556,
										EstimatedCost: floatPtr(30884.60),
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("t"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
										EstimatedRows: 44514,
										EstimatedCost: floatPtr(374955.51),
										KeyUsed:       stringPtr("PRIMARY"),
										Condition:     stringPtr("( `employees` . `t` . `to_date` = date ? )"),
									},
								},
							},
						},
						{
							Operation: explainPlanOutputOperationTableScan,
							Details: nodeDetails{
								Alias:      stringPtr("<subquery2>"),
								AccessType: explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
								KeyUsed:    stringPtr("<auto_distinct_key>"),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationMaterializedSubquery,
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationNestedLoopJoin,
											Details: nodeDetails{
												JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
											},
											Children: []planNode{
												{
													Operation: explainPlanOutputOperationTableScan,
													Details: nodeDetails{
														Alias:         stringPtr("salaries"),
														AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
														EstimatedRows: 94604,
														EstimatedCost: floatPtr(289962.60),
														Condition:     stringPtr("( ( `employees` . `salaries` . `to_date` = date ? ) and ( `employees` . `salaries` . `salary` > ? ) )"),
													},
												},
												{
													Operation: explainPlanOutputOperationTableScan,
													Details: nodeDetails{
														Alias:         stringPtr("titles"),
														AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationTableScan,
					Details: nodeDetails{
						Alias:         stringPtr("e"),
						AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationNestedLoopJoin,
					Details: nodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
					},
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationNestedLoopJoin,
									Details: nodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
									},
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("de"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
												EstimatedRows: 33114,
												EstimatedCost: floatPtr(33851.30),
												Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
											},
										},
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("s"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
												EstimatedRows: 31146,
												EstimatedCost: floatPtr(98405.51),
												KeyUsed:       stringPtr("PRIMARY"),
												Condition:     stringPtr("( `employees` . `s` . `to_date` = date ? )"),
											},
										},
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
										EstimatedRows: 31146,
										EstimatedCost: floatPtr(132640.69),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
							},
						},
						{
							Operation: explainPlanOutputOperationTableScan,
							Details: nodeDetails{
								Alias:         stringPtr("dept_salary_stats"),
								AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
								EstimatedRows: 138443,
								EstimatedCost: floatPtr(278020.70),
								KeyUsed:       stringPtr("<auto_key1>"),
								Condition:     stringPtr("( `employees` . `s` . `salary` > `dept_salary_stats` . `avg_salary` )"),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationMaterializedSubquery,
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationGroupingOperation,
											Children: []planNode{
												{
													Operation: explainPlanOutputOperationNestedLoopJoin,
													Details: nodeDetails{
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
													},
													Children: []planNode{
														{
															Operation: explainPlanOutputOperationNestedLoopJoin,
															Details: nodeDetails{
																JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
															},
															Children: []planNode{
																{
																	Operation: explainPlanOutputOperationTableScan,
																	Details: nodeDetails{
																		Alias:         stringPtr("de"),
																		AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
																		EstimatedRows: 33114,
																		EstimatedCost: floatPtr(33851.30),
																		Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
																	},
																},
																{
																	Operation: explainPlanOutputOperationTableScan,
																	Details: nodeDetails{
																		Alias:         stringPtr("s"),
																		AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
																		EstimatedRows: 31146,
																		EstimatedCost: floatPtr(98405.51),
																		KeyUsed:       stringPtr("PRIMARY"),
																		Condition:     stringPtr("( `employees` . `s` . `to_date` = date ? )"),
																	},
																},
															},
														},
														{
															Operation: explainPlanOutputOperationTableScan,
															Details: nodeDetails{
																Alias:         stringPtr("d"),
																AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationOrderingOperation,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationDuplicatesRemoval,
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationNestedLoopJoin,
									Details: nodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
									},
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationNestedLoopJoin,
											Details: nodeDetails{
												JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
											},
											Children: []planNode{
												{
													Operation: explainPlanOutputOperationTableScan,
													Details: nodeDetails{
														Alias:         stringPtr("de"),
														AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
														EstimatedRows: 33114,
														EstimatedCost: floatPtr(33851.30),
														Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
													},
												},
												{
													Operation: explainPlanOutputOperationTableScan,
													Details: nodeDetails{
														Alias:         stringPtr("t"),
														AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
														EstimatedRows: 4920,
														EstimatedCost: floatPtr(71886.48),
														KeyUsed:       stringPtr("PRIMARY"),
														Condition:     stringPtr("( `employees` . `t` . `to_date` = date ? )"),
													},
												},
											},
										},
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("d"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationGroupingOperation,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("d"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeIndex),
										EstimatedRows: 9,
										EstimatedCost: floatPtr(1.90),
										KeyUsed:       stringPtr("dept_name"),
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("de"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationOrderingOperation,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationNestedLoopJoin,
									Details: nodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
									},
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("d"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeIndex),
												EstimatedRows: 9,
												EstimatedCost: floatPtr(1.90),
												KeyUsed:       stringPtr("dept_name"),
											},
										},
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("de"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
												EstimatedRows: 37253,
												EstimatedCost: floatPtr(57154.49),
												KeyUsed:       stringPtr("dept_no"),
												Condition:     stringPtr("( `employees` . `de` . `to_date` = date ? )"),
											},
										},
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationGroupingOperation,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("t"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
										EstimatedRows: 44260,
										EstimatedCost: floatPtr(45512.50),
										Condition:     stringPtr("( `employees` . `t` . `to_date` = date ? )"),
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("s"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationNestedLoopJoin,
					Details: nodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
					},
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("d"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeIndex),
										EstimatedRows: 9,
										EstimatedCost: floatPtr(1.90),
										KeyUsed:       stringPtr("dept_name"),
										// JoinType is unknown, we could run the tree explain plan as well to determine this.
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("de"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
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
							Operation: explainPlanOutputOperationTableScan,
							Details: nodeDetails{
								Alias:         stringPtr("e"),
								AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationNestedLoopJoin,
					Details: nodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
					},
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("dm"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeIndex),
										EstimatedRows: 24,
										EstimatedCost: floatPtr(3.51),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("s"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeRef),
										EstimatedRows: 7,
										EstimatedCost: floatPtr(50.19),
										KeyUsed:       stringPtr("PRIMARY"),
										Condition:     stringPtr("( ( `employees` . `s` . `to_date` = date ? ) and ( `employees` . `s` . `salary` > ? ) )"),
									},
								},
							},
						},
						{
							Operation: explainPlanOutputOperationTableScan,
							Details: nodeDetails{
								Alias:         stringPtr("e"),
								AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationNestedLoopJoin,
					Details: nodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
					},
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationHashJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmHash),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationNestedLoopJoin,
									Details: nodeDetails{
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
									},
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("de1"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
												EstimatedRows: 33114,
												EstimatedCost: floatPtr(33851.30),
												Condition:     stringPtr("( `employees` . `de1` . `to_date` = date ? )"),
											},
										},
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("e1"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
												EstimatedRows: 33114,
												EstimatedCost: floatPtr(70249.00),
												KeyUsed:       stringPtr("PRIMARY"),
											},
										},
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("de2"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
										EstimatedRows: 137069612,
										EstimatedCost: floatPtr(110342868.42),
										Condition:     stringPtr("( ( `employees` . `de2` . `dept_no` = `employees` . `de1` . `dept_no` ) and ( `employees` . `de2` . `to_date` = date ? ) and ( `employees` . `de1` . `emp_no` < `employees` . `de2` . `emp_no` ) )"),
									},
								},
							},
						},
						{
							Operation: explainPlanOutputOperationTableScan,
							Details: nodeDetails{
								Alias:         stringPtr("e2"),
								AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationOrderingOperation,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationGroupingOperation,
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("employees"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationNestedLoopJoin,
					Details: nodeDetails{
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
					},
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationTableScan,
							Details: nodeDetails{
								Alias:         stringPtr("s"),
								AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
								EstimatedRows: 94604,
								EstimatedCost: floatPtr(289962.60),
								Condition:     stringPtr("( ( `employees` . `s` . `to_date` = date ? ) and ( `employees` . `s` . `salary` > ( select ( avg ( `employees` . `salaries` . `salary` ) * ? ) from `employees` . `salaries` ) ) )"),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationAttachedSubquery,
									Children: []planNode{
										{
											Operation: explainPlanOutputOperationTableScan,
											Details: nodeDetails{
												Alias:         stringPtr("salaries"),
												AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
												EstimatedRows: 2838426,
												EstimatedCost: floatPtr(289962.60),
											},
										},
									},
								},
							},
						},
						{
							Operation: explainPlanOutputOperationTableScan,
							Details: nodeDetails{
								Alias:         stringPtr("e"),
								AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			result: &explainPlanOutput{
				Metadata: metadataInfo{
					DatabaseEngine:  "MySQL",
					DatabaseVersion: "8.0.32",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: planNode{
					Operation: explainPlanOutputOperationUnion,
					Children: []planNode{
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("dm"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
										EstimatedRows: 2,
										EstimatedCost: floatPtr(3.40),
										Condition:     stringPtr("( `employees` . `dm` . `to_date` = date ? )"),
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
										EstimatedRows: 2,
										EstimatedCost: floatPtr(6.04),
										KeyUsed:       stringPtr("PRIMARY"),
									},
								},
							},
						},
						{
							Operation: explainPlanOutputOperationNestedLoopJoin,
							Details: nodeDetails{
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(explainPlanJoinAlgorithmNestedLoop),
							},
							Children: []planNode{
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("t"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeAll),
										EstimatedRows: 4426,
										EstimatedCost: floatPtr(45512.50),
										Condition:     stringPtr("( ( `employees` . `t` . `to_date` = date ? ) and ( `employees` . `t` . `title` = ? ) )"),
									},
								},
								{
									Operation: explainPlanOutputOperationTableScan,
									Details: nodeDetails{
										Alias:         stringPtr("e"),
										AccessType:    explainPlanAccessTypePtr(explainPlanAccessTypeEqRef),
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
			output, err := newExplainPlanOutput(logger, test.dbVersion, test.digest, jsonData, currentTime)
			require.NoError(t, err, "Failed generate explain plan output: %s", test.fname)
			// Override the generated at time to ensure the test is deterministic
			output.Metadata.GeneratedAt = currentTime
			require.Equal(t, test.result, output)
		})
	}
}

func TestExplainPlan(t *testing.T) {
	t.Run("last seen", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectDBSchemaVersion).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{
			"version",
		}).AddRow(
			"8.0.32",
		))

		lastSeen := time.Now().Add(-time.Hour)
		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		c, err := NewExplainPlan(ExplainPlanArguments{
			DB:              db,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout)),
			InstanceKey:     "mysql-db",
			ScrapeInterval:  time.Second,
			PerScrapeRatio:  1,
			EntryHandler:    lokiClient,
			InitialLookback: lastSeen,
		})
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		t.Run("uses argument value on first request", func(t *testing.T) {
			nextSeen := lastSeen.Add(time.Second * 45)
			mock.ExpectQuery(selectDigestsForExplainPlan).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
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
			mock.ExpectQuery(selectDigestsForExplainPlan).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
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

		mock.ExpectQuery(selectDBSchemaVersion).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{
			"version",
		}).AddRow(
			"8.0.32",
		))

		lastSeen := time.Now().Add(-time.Hour)
		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		logBuffer := bytes.NewBuffer(nil)

		c, err := NewExplainPlan(ExplainPlanArguments{
			DB:              db,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(logBuffer)),
			InstanceKey:     "mysql-db",
			ScrapeInterval:  time.Second,
			PerScrapeRatio:  1,
			EntryHandler:    lokiClient,
			InitialLookback: lastSeen,
		})
		require.NoError(t, err)

		t.Run("skips truncated queries", func(t *testing.T) {
			logBuffer.Reset()
			mock.ExpectQuery(selectDigestsForExplainPlan).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
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

			lokiEntries := lokiClient.Received()
			require.Equal(t, 0, len(lokiEntries))

			require.Contains(t, logBuffer.String(), "skipping truncated query")
			require.NotContains(t, logBuffer.String(), "error")
		})

		t.Run("skips non-select queries", func(t *testing.T) {
			lokiClient.Clear()
			logBuffer.Reset()
			mock.ExpectQuery(selectDigestsForExplainPlan).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
				"schema_name",
				"digest",
				"query_sample_text",
				"last_seen",
			}).AddRow(
				"some_schema",
				"some_digest",
				"update some_table set col = 1 where id = 1",
				lastSeen,
			).AddRow(
				"some_schema",
				"some_digest",
				"delete from some_table",
				lastSeen,
			).AddRow(
				"some_schema",
				"some_digest",
				"insert into some_table (col) values (1)",
				lastSeen,
			))

			err = c.fetchExplainPlans(t.Context())
			require.NoError(t, err)

			lokiEntries := lokiClient.Received()
			require.Equal(t, 0, len(lokiEntries))

			require.NotContains(t, logBuffer.String(), "error")
		})

		t.Run("passes queries beginning in select", func(t *testing.T) {
			lokiClient.Clear()
			logBuffer.Reset()
			mock.ExpectQuery(selectDigestsForExplainPlan).WithArgs(lastSeen).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})
}
