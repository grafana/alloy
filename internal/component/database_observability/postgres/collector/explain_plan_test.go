package collector

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

func stringPtr(s string) *string {
	return &s
}

func floatPtr(f float64) *float64 {
	return &f
}

func explainPlanJoinAlgorithmPtr(s database_observability.ExplainPlanJoinAlgorithm) *database_observability.ExplainPlanJoinAlgorithm {
	return &s
}

func validatePlan(t *testing.T, expect, actual database_observability.ExplainPlanNode) {
	t.Run(string(expect.Operation), func(t *testing.T) {
		require.Equal(t, expect.Operation, actual.Operation)
		require.Equal(t, expect.Details, actual.Details)
		require.Equal(t, len(expect.Children), len(actual.Children))
		for i := range expect.Children {
			validatePlan(t, expect.Children[i], actual.Children[i])
		}
	})
}

func TestExplainPlanOutput(t *testing.T) {
	currentTime := time.Now().Format(time.RFC3339)
	tests := []struct {
		engineVersion string
		queryid       string
		fname         string
		result        *database_observability.ExplainPlanOutput
	}{
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "complex_aggregation_with_case",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Finalize Group Aggregate"),
					Details: database_observability.ExplainPlanNodeDetails{
						GroupByKeys:   []string{"d.dept_name"},
						EstimatedRows: 9,
						EstimatedCost: floatPtr(.18),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 9,
								EstimatedCost: floatPtr(1001.03),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Sort"),
									Details: database_observability.ExplainPlanNodeDetails{
										SortKeys:      []string{"d.dept_name"},
										EstimatedRows: 9,
										EstimatedCost: floatPtr(.16),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Partial Hashed Aggregate"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 9,
												EstimatedCost: floatPtr(2117.76),
												GroupByKeys:   []string{"d.dept_name"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
													Details: database_observability.ExplainPlanNodeDetails{
														JoinType:      stringPtr("Inner"),
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
														EstimatedRows: 141178,
														EstimatedCost: floatPtr(545.21),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 141178,
																EstimatedCost: floatPtr(3257),
																JoinType:      stringPtr("Inner"),
																JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("e"),
																		EstimatedRows: 176485,
																		EstimatedCost: floatPtr(4268.85),
																	},
																},
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		EstimatedRows: 141178,
																		EstimatedCost: floatPtr(0),
																	},
																	Children: []database_observability.ExplainPlanNode{
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				EstimatedRows: 141178,
																				EstimatedCost: floatPtr(4598.26),
																				Condition:     stringPtr("(to_date = ?::date)"),
																				Alias:         stringPtr("de"),
																			},
																		},
																	},
																},
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperation("Hash"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 9,
																EstimatedCost: floatPtr(0),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("d"),
																		EstimatedRows: 9,
																		EstimatedCost: floatPtr(1.09),
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
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "complex_join_with_aggregate_subquery",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Simple Hashed Aggregate"),
					Details: database_observability.ExplainPlanNodeDetails{
						GroupByKeys:   []string{"d.id"},
						EstimatedRows: 9,
						EstimatedCost: floatPtr(464092.26),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
							Details: database_observability.ExplainPlanNodeDetails{
								JoinType:      stringPtr("Inner"),
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
								EstimatedRows: 240003,
								EstimatedCost: floatPtr(926.79),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
									Details: database_observability.ExplainPlanNodeDetails{
										JoinType:      stringPtr("Inner"),
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
										EstimatedRows: 240003,
										EstimatedCost: floatPtr(9068.32),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("de"),
												EstimatedRows: 240003,
												EstimatedCost: floatPtr(6305.04),
												Condition:     stringPtr("(to_date = ?::date)"),
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperation("Hash"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 300024,
												EstimatedCost: floatPtr(0),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("e"),
														EstimatedRows: 300024,
														EstimatedCost: floatPtr(5504.24),
													},
												},
											},
										},
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperation("Hash"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 9,
										EstimatedCost: floatPtr(0),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("d"),
												EstimatedRows: 9,
												EstimatedCost: floatPtr(1.09),
											},
										},
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperation("Simple Aggregate"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 1,
								EstimatedCost: floatPtr(57.66),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
									Details: database_observability.ExplainPlanNodeDetails{
										JoinType:      stringPtr("Inner"),
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
										EstimatedRows: 23059,
										EstimatedCost: floatPtr(969.17),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("s2"),
												EstimatedRows: 242218,
												EstimatedCost: floatPtr(53710.59),
												Condition:     stringPtr("(to_date = ?::date)"),
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperation("Hash"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 26667,
												EstimatedCost: floatPtr(0),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Bitmap Heap Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("de2"),
														EstimatedRows: 26667,
														EstimatedCost: floatPtr(2719.34),
														Condition:     stringPtr("(to_date = ?::date)"),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Bitmap Index Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 36845,
																EstimatedCost: floatPtr(404.76),
																KeyUsed:       stringPtr("idx_16982_dept_no"),
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
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "complex_query_with_multiple_conditions",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Incremental Sort"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 21,
						EstimatedCost: floatPtr(0.57),
						SortKeys:      []string{"d.dept_name", "(avg(s.amount)) DESC"},
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Finalize Group Aggregate"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 21,
								EstimatedCost: floatPtr(2.05),
								Condition:     stringPtr("(avg(s.amount) > ?::numeric)"),
								GroupByKeys:   []string{"d.dept_name", "t.title"},
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 63,
										EstimatedCost: floatPtr(1007.1),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Sort"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 63,
												EstimatedCost: floatPtr(2.04),
												SortKeys:      []string{"d.dept_name", "t.title"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Partial Hashed Aggregate"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 63,
														EstimatedCost: floatPtr(2275.93),
														GroupByKeys:   []string{"d.dept_name", "t.title"},
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 91006,
																EstimatedCost: floatPtr(351.5),
																JoinType:      stringPtr("Inner"),
																JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		EstimatedRows: 91006,
																		EstimatedCost: floatPtr(2021.24),
																		JoinType:      stringPtr("Inner"),
																		JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
																	},
																	Children: []database_observability.ExplainPlanNode{
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				Alias:         stringPtr("e"),
																				Condition:     stringPtr("(hire_date > ?::date)"),
																				EstimatedRows: 176468,
																				EstimatedCost: floatPtr(4710.06),
																			},
																		},
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				EstimatedRows: 73958,
																				EstimatedCost: floatPtr(0),
																			},
																			Children: []database_observability.ExplainPlanNode{
																				{
																					Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
																					Details: database_observability.ExplainPlanNodeDetails{
																						JoinType:      stringPtr("Inner"),
																						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
																						EstimatedRows: 73958,
																						EstimatedCost: floatPtr(1763.4),
																					},
																					Children: []database_observability.ExplainPlanNode{
																						{
																							Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																							Details: database_observability.ExplainPlanNodeDetails{
																								Alias:         stringPtr("t"),
																								Condition:     stringPtr("(to_date = ?::date)"),
																								EstimatedRows: 99824,
																								EstimatedCost: floatPtr(5508.9),
																							},
																						},
																						{
																							Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
																							Details: database_observability.ExplainPlanNodeDetails{
																								EstimatedRows: 86472,
																								EstimatedCost: floatPtr(0),
																							},
																							Children: []database_observability.ExplainPlanNode{
																								{
																									Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
																									Details: database_observability.ExplainPlanNodeDetails{
																										JoinType:      stringPtr("Inner"),
																										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
																										EstimatedRows: 86472,
																										EstimatedCost: floatPtr(2651.85),
																									},
																									Children: []database_observability.ExplainPlanNode{
																										{
																											Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																											Details: database_observability.ExplainPlanNodeDetails{
																												Alias:         stringPtr("s"),
																												Condition:     stringPtr("(to_date = ?::date)"),
																												EstimatedRows: 100924,
																												EstimatedCost: floatPtr(32972.74),
																											},
																										},
																										{
																											Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
																											Details: database_observability.ExplainPlanNodeDetails{
																												EstimatedRows: 141178,
																												EstimatedCost: floatPtr(0),
																											},
																											Children: []database_observability.ExplainPlanNode{
																												{
																													Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																													Details: database_observability.ExplainPlanNodeDetails{
																														Alias:         stringPtr("de"),
																														Condition:     stringPtr("(to_date = ?::date)"),
																														EstimatedRows: 141178,
																														EstimatedCost: floatPtr(4598.26),
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
																	Operation: database_observability.ExplainPlanOutputOperation("Hash"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		EstimatedRows: 9,
																		EstimatedCost: floatPtr(0),
																	},
																	Children: []database_observability.ExplainPlanNode{
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				Alias:         stringPtr("d"),
																				EstimatedRows: 9,
																				EstimatedCost: floatPtr(1.09),
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
					},
				},
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "complex_subquery_in_select_clause",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Index Scan"),
					Details: database_observability.ExplainPlanNodeDetails{
						Alias:         stringPtr("e"),
						EstimatedRows: 49,
						EstimatedCost: floatPtr(962.45),
						KeyUsed:       stringPtr("idx_16988_primary"),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Nested Loop"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 1,
								EstimatedCost: floatPtr(0.11),
								JoinType:      stringPtr("Inner"),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Index Scan"),
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("de"),
										Condition:     stringPtr("(to_date = ?::date)"),
										EstimatedRows: 1,
										EstimatedCost: floatPtr(8.44),
										KeyUsed:       stringPtr("idx_16982_primary"),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("d"),
										EstimatedRows: 9,
										EstimatedCost: floatPtr(1.09),
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperation("Index Scan"),
							Details: database_observability.ExplainPlanNodeDetails{
								Alias:         stringPtr("t"),
								Condition:     stringPtr("(to_date = ?::date)"),
								EstimatedRows: 1,
								EstimatedCost: floatPtr(10.21),
								KeyUsed:       stringPtr("idx_16994_primary"),
							},
						},
					},
				},
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "conditional_aggregation_with_case",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Finalize Group Aggregate"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 5065,
						EstimatedCost: floatPtr(113.96),
						GroupByKeys:   []string{"(EXTRACT(year FROM hire_date))"},
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 5065,
								EstimatedCost: floatPtr(1569.82),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Sort"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 5065,
										EstimatedCost: floatPtr(324.32),
										SortKeys:      []string{"(EXTRACT(year FROM hire_date))"},
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Partial Hashed Aggregate"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 5065,
												EstimatedCost: floatPtr(2710.59),
												GroupByKeys:   []string{"EXTRACT(year FROM hire_date)"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("employee"),
														EstimatedRows: 176485,
														EstimatedCost: floatPtr(4710.06),
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
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "correlated_subquery",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 239578,
						EstimatedCost: floatPtr(10703.2),
						JoinType:      stringPtr("Inner"),
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 239578,
								EstimatedCost: floatPtr(3294.29),
								JoinType:      stringPtr("Inner"),
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("t"),
										Condition:     stringPtr("(to_date = ?::date)"),
										EstimatedRows: 239578,
										EstimatedCost: floatPtr(8741.35),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperation("Hash"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 7,
										EstimatedCost: floatPtr(0),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Simple Hashed Aggregate"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 7,
												EstimatedCost: floatPtr(34.77),
												GroupByKeys:   []string{"(title.title)::text"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Gather"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 13883,
														EstimatedCost: floatPtr(2388.3),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 5785,
																EstimatedCost: floatPtr(622.29),
																JoinType:      stringPtr("Semi"),
																JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("title"),
																		EstimatedRows: 184712,
																		EstimatedCost: floatPtr(5047.12),
																	},
																},
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		EstimatedRows: 3532,
																		EstimatedCost: floatPtr(0),
																	},
																	Children: []database_observability.ExplainPlanNode{
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				Alias:         stringPtr("salary"),
																				Condition:     stringPtr("((amount > ?) AND (to_date = ?::date))"),
																				EstimatedRows: 3532,
																				EstimatedCost: floatPtr(35935.29),
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
							Operation: database_observability.ExplainPlanOutputOperation("Hash"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 300024,
								EstimatedCost: floatPtr(0),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										EstimatedRows: 300024,
										EstimatedCost: floatPtr(5504.24),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "date_manipulation_with_conditions",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Gather"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 827,
						EstimatedCost: floatPtr(1082.7),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
							Details: database_observability.ExplainPlanNodeDetails{
								Alias:         stringPtr("e"),
								Condition:     stringPtr("((hire_date < ?::date) AND (EXTRACT(month FROM hire_date) = EXTRACT(month FROM CURRENT_DATE)))"),
								EstimatedRows: 486,
								EstimatedCost: floatPtr(6476.12),
							},
						},
					},
				},
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "derived_table_with_aggregates",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
					Details: database_observability.ExplainPlanNodeDetails{
						JoinType:      stringPtr("Inner"),
						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
						EstimatedRows: 64587,
						EstimatedCost: floatPtr(3633.13),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Gather"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 193761,
								EstimatedCost: floatPtr(20376.1),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 113977,
										EstimatedCost: floatPtr(2251.38),
										JoinType:      stringPtr("Inner"),
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("e"),
												EstimatedRows: 176485,
												EstimatedCost: floatPtr(4268.85),
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 86472,
												EstimatedCost: floatPtr(0),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 86472,
														EstimatedCost: floatPtr(2651.85),
														JoinType:      stringPtr("Inner"),
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("s"),
																Condition:     stringPtr("(to_date = ?::date)"),
																EstimatedRows: 100924,
																EstimatedCost: floatPtr(32972.74),
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 141178,
																EstimatedCost: floatPtr(0),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("de"),
																		Condition:     stringPtr("(to_date = ?::date)"),
																		EstimatedRows: 141178,
																		EstimatedCost: floatPtr(4598.26),
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
							Operation: database_observability.ExplainPlanOutputOperation("Hash"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 9,
								EstimatedCost: floatPtr(0),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Subquery Scan"),
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("dept_salary_stats"),
										EstimatedRows: 9,
										EstimatedCost: floatPtr(0.09),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Finalize Group Aggregate"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 9,
												EstimatedCost: floatPtr(0.25),
												GroupByKeys:   []string{"d.id"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 18,
														EstimatedCost: floatPtr(1002.1),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Sort"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 9,
																EstimatedCost: floatPtr(0.16),
																SortKeys:      []string{"d.id"},
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Partial Hashed Aggregate"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		EstimatedRows: 9,
																		EstimatedCost: floatPtr(432.48),
																		GroupByKeys:   []string{"d.id"},
																	},
																	Children: []database_observability.ExplainPlanNode{
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				EstimatedRows: 86472,
																				EstimatedCost: floatPtr(333.99),
																				JoinType:      stringPtr("Inner"),
																				JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
																			},
																			Children: []database_observability.ExplainPlanNode{
																				{
																					Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
																					Details: database_observability.ExplainPlanNodeDetails{
																						EstimatedRows: 86472,
																						EstimatedCost: floatPtr(2651.85),
																						JoinType:      stringPtr("Inner"),
																						JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
																					},
																					Children: []database_observability.ExplainPlanNode{
																						{
																							Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																							Details: database_observability.ExplainPlanNodeDetails{
																								Alias:         stringPtr("s_1"),
																								Condition:     stringPtr("(to_date = ?::date)"),
																								EstimatedRows: 100924,
																								EstimatedCost: floatPtr(32972.74),
																							},
																						},
																						{
																							Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
																							Details: database_observability.ExplainPlanNodeDetails{
																								EstimatedRows: 141178,
																								EstimatedCost: floatPtr(0),
																							},
																							Children: []database_observability.ExplainPlanNode{
																								{
																									Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																									Details: database_observability.ExplainPlanNodeDetails{
																										Alias:         stringPtr("de_1"),
																										Condition:     stringPtr("(to_date = ?::date)"),
																										EstimatedRows: 141178,
																										EstimatedCost: floatPtr(4598.26),
																									},
																								},
																							},
																						},
																					},
																				},
																				{
																					Operation: database_observability.ExplainPlanOutputOperation("Hash"),
																					Details: database_observability.ExplainPlanNodeDetails{
																						EstimatedRows: 9,
																						EstimatedCost: floatPtr(0),
																					},
																					Children: []database_observability.ExplainPlanNode{
																						{
																							Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
																							Details: database_observability.ExplainPlanNodeDetails{
																								Alias:         stringPtr("d"),
																								EstimatedRows: 9,
																								EstimatedCost: floatPtr(1.09),
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
									},
								},
							},
						},
					},
				},
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "distinct_with_multiple_joins",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Unique"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 63,
						EstimatedCost: floatPtr(0.31),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 63,
								EstimatedCost: floatPtr(1007.1),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Sort"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 63,
										EstimatedCost: floatPtr(2.04),
										SortKeys:      []string{"d.dept_name", "t.title"},
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Simple Hashed Aggregate"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 63,
												EstimatedCost: floatPtr(604.37),
												GroupByKeys:   []string{"d.dept_name", "t.title"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 120748,
														EstimatedCost: floatPtr(466.33),
														JoinType:      stringPtr("Inner"),
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 120748,
																EstimatedCost: floatPtr(2280.32),
																JoinType:      stringPtr("Inner"),
																JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("de"),
																		Condition:     stringPtr("(to_date = ?::date)"),
																		EstimatedRows: 141178,
																		EstimatedCost: floatPtr(4598.26),
																	},
																},
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		EstimatedRows: 99824,
																		EstimatedCost: floatPtr(0),
																	},
																	Children: []database_observability.ExplainPlanNode{
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				Alias:         stringPtr("t"),
																				Condition:     stringPtr("(to_date = ?::date)"),
																				EstimatedRows: 99824,
																				EstimatedCost: floatPtr(5508.9),
																			},
																		},
																	},
																},
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperation("Hash"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 9,
																EstimatedCost: floatPtr(0),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("d"),
																		EstimatedRows: 9,
																		EstimatedCost: floatPtr(1.09),
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
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "group_by_with_having",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Finalize Group Aggregate"),
					Details: database_observability.ExplainPlanNodeDetails{
						Condition:     stringPtr("(count(de.employee_id) > ?)"),
						EstimatedRows: 3,
						EstimatedCost: floatPtr(0.15),
						GroupByKeys:   []string{"d.dept_name"},
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 9,
								EstimatedCost: floatPtr(1001.03),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Sort"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 9,
										EstimatedCost: floatPtr(0.16),
										SortKeys:      []string{"d.dept_name"},
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Partial Hashed Aggregate"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 9,
												EstimatedCost: floatPtr(705.98),
												GroupByKeys:   []string{"d.dept_name"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 141178,
														EstimatedCost: floatPtr(545.21),
														JoinType:      stringPtr("Inner"),
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("de"),
																Condition:     stringPtr("(to_date = ?::date)"),
																EstimatedRows: 141178,
																EstimatedCost: floatPtr(4598.26),
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperation("Hash"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 9,
																EstimatedCost: floatPtr(0),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("d"),
																		EstimatedRows: 9,
																		EstimatedCost: floatPtr(1.09),
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
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "join_and_order",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 141178,
						EstimatedCost: floatPtr(16882.54),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Sort"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 141178,
								EstimatedCost: floatPtr(21115.71),
								SortKeys:      []string{"e.last_name", "e.first_name"},
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 141178,
										EstimatedCost: floatPtr(545.21),
										JoinType:      stringPtr("Inner"),
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 141178,
												EstimatedCost: floatPtr(3257),
												JoinType:      stringPtr("Inner"),
												JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("e"),
														EstimatedRows: 176485,
														EstimatedCost: floatPtr(4268.85),
													},
												},
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 141178,
														EstimatedCost: floatPtr(0),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("de"),
																Condition:     stringPtr("(to_date = ?::date)"),
																EstimatedRows: 141178,
																EstimatedCost: floatPtr(4598.26),
															},
														},
													},
												},
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperation("Hash"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 9,
												EstimatedCost: floatPtr(0),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("d"),
														EstimatedRows: 9,
														EstimatedCost: floatPtr(1.09),
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
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "multiple_aggregate_functions_with_having",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Finalize Group Aggregate"),
					Details: database_observability.ExplainPlanNodeDetails{
						Condition:     stringPtr("(avg(s.amount) > ?::numeric)"),
						EstimatedRows: 2,
						EstimatedCost: floatPtr(0.32),
						GroupByKeys:   []string{"t.title"},
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 14,
								EstimatedCost: floatPtr(1001.64),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Sort"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 7,
										EstimatedCost: floatPtr(0.11),
										SortKeys:      []string{"t.title"},
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Partial Hashed Aggregate"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 7,
												EstimatedCost: floatPtr(1116.76),
												GroupByKeys:   []string{"t.title"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 89334,
														EstimatedCost: floatPtr(1998.49),
														JoinType:      stringPtr("Inner"),
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("s"),
																EstimatedRows: 100924,
																EstimatedCost: floatPtr(32972.74),
																Condition:     stringPtr("(to_date = ?::date)"),
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 99824,
																EstimatedCost: floatPtr(0),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("t"),
																		EstimatedRows: 99824,
																		EstimatedCost: floatPtr(5508.90),
																		Condition:     stringPtr("(to_date = ?::date)"),
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
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "multiple_joins_with_date_functions",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Gather"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 1200,
						EstimatedCost: floatPtr(1120),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Hash Join"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 706,
								EstimatedCost: floatPtr(11.65),
								JoinType:      stringPtr("Inner"),
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 706,
										EstimatedCost: floatPtr(381.62),
										JoinType:      stringPtr("Inner"),
										JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("de"),
												EstimatedRows: 141178,
												EstimatedCost: floatPtr(4598.26),
												Condition:     stringPtr("(to_date = ?::date)"),
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 882,
												EstimatedCost: floatPtr(0),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("e"),
														EstimatedRows: 882,
														EstimatedCost: floatPtr(5151.27),
														Condition:     stringPtr("(EXTRACT(year FROM hire_date) = ?::numeric)"),
													},
												},
											},
										},
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperation("Hash"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 9,
										EstimatedCost: floatPtr(0),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("d"),
												EstimatedRows: 9,
												EstimatedCost: floatPtr(1.09),
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
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "nested_subqueries_with_exists",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Gather"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 8476,
						EstimatedCost: floatPtr(1847.6),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 4986,
								EstimatedCost: floatPtr(629.91),
								JoinType:      stringPtr("Semi"),
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										EstimatedRows: 176485,
										EstimatedCost: floatPtr(4268.85),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 5902,
										EstimatedCost: floatPtr(0),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 5902,
												EstimatedCost: floatPtr(651.35),
												JoinType:      stringPtr("Semi"),
												JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("dm"),
														EstimatedRows: 195061,
														EstimatedCost: floatPtr(4110.61),
													},
												},
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 3532,
														EstimatedCost: floatPtr(0),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("s"),
																Condition:     stringPtr("((amount > ?) AND (to_date = ?::date))"),
																EstimatedRows: 3532,
																EstimatedCost: floatPtr(35935.29),
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
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "self_join_with_date_comparison",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Limit"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 100,
						EstimatedCost: floatPtr(0),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Nested Loop"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 655628,
								EstimatedCost: floatPtr(2513164.11),
								JoinType:      stringPtr("Inner"),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Nested Loop"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 5011391,
										EstimatedCost: floatPtr(2536922.07),
										JoinType:      stringPtr("Inner"),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Gather"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 6264662,
												EstimatedCost: floatPtr(627466.2),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 3685095,
														EstimatedCost: floatPtr(114795.94),
														JoinType:      stringPtr("Inner"),
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("e1"),
																EstimatedRows: 176485,
																EstimatedCost: floatPtr(4268.85),
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 176485,
																EstimatedCost: floatPtr(0),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("e2"),
																		EstimatedRows: 176485,
																		EstimatedCost: floatPtr(4268.85),
																	},
																},
															},
														},
													},
												},
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperation("Memoize"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 1,
												EstimatedCost: floatPtr(0.01),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Index Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("de1"),
														Condition:     stringPtr("(to_date = ?::date)"),
														EstimatedRows: 1,
														EstimatedCost: floatPtr(0.49),
														KeyUsed:       stringPtr("idx_16982_primary"),
													},
												},
											},
										},
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperation("Index Scan"),
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("de2"),
										Condition:     stringPtr("(to_date = ?::date)"),
										EstimatedRows: 1,
										EstimatedCost: floatPtr(0.49),
										KeyUsed:       stringPtr("idx_16982_primary"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "string_functions_with_grouping",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Sort"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 1637,
						EstimatedCost: floatPtr(91.49),
						SortKeys:      []string{"(count(*)) DESC"},
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Finalize Hashed Aggregate"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 1637,
								EstimatedCost: floatPtr(28.64),
								GroupByKeys:   []string{"(\"left\"((last_name)::text, 1))"},
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Gather"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 1637,
										EstimatedCost: floatPtr(1163.7),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Partial Hashed Aggregate"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 1637,
												EstimatedCost: floatPtr(902.89),
												GroupByKeys:   []string{"\"left\"((last_name)::text, 1)"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("employee"),
														EstimatedRows: 176485,
														EstimatedCost: floatPtr(4710.06),
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
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "subquery_with_aggregate",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Gather"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 80739,
						EstimatedCost: floatPtr(9073.9),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Finalize Aggregate"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 1,
								EstimatedCost: floatPtr(0.02),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Gather"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 2,
										EstimatedCost: floatPtr(1000.2),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Partial Aggregate"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 1,
												EstimatedCost: floatPtr(2962.56),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("salary"),
														EstimatedRows: 1185020,
														EstimatedCost: floatPtr(30010.20),
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 47494,
								EstimatedCost: floatPtr(2162.65),
								JoinType:      stringPtr("Inner"),
								JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
									Details: database_observability.ExplainPlanNodeDetails{
										Alias:         stringPtr("e"),
										EstimatedRows: 176485,
										EstimatedCost: floatPtr(4268.85),
									},
								},
								{
									Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 33641,
										EstimatedCost: floatPtr(0),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("s"),
												Condition:     stringPtr("((to_date = ?::date) AND ((amount)::numeric > (InitPlan ?).col1))"),
												EstimatedRows: 33641,
												EstimatedCost: floatPtr(38897.84),
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
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "union_with_different_conditions",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Sort"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 52748,
						EstimatedCost: floatPtr(7695.61),
						SortKeys:      []string{"e.last_name"},
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Unique"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 52748,
								EstimatedCost: floatPtr(395.61),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Sort"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 52748,
										EstimatedCost: floatPtr(7695.61),
										SortKeys:      []string{"e.first_name", "e.last_name", "('Manager'::text)"},
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Append"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 52748,
												EstimatedCost: floatPtr(263.74),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Nested Loop"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 1,
														EstimatedCost: floatPtr(0),
														JoinType:      stringPtr("Inner"),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("dm"),
																EstimatedRows: 1,
																EstimatedCost: floatPtr(1.30),
																Condition:     stringPtr("(to_date = ?::date)"),
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperation("Index Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("e"),
																EstimatedRows: 1,
																EstimatedCost: floatPtr(8.44),
																KeyUsed:       stringPtr("idx_16988_primary"),
															},
														},
													},
												},
												{
													Operation: database_observability.ExplainPlanOutputOperation("Gather"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 52747,
														EstimatedCost: floatPtr(6274.7),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 31028,
																EstimatedCost: floatPtr(1065.82),
																JoinType:      stringPtr("Inner"),
																JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		Alias:         stringPtr("e_1"),
																		EstimatedRows: 176485,
																		EstimatedCost: floatPtr(4268.85),
																	},
																},
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		EstimatedRows: 21978,
																		EstimatedCost: floatPtr(0),
																	},
																	Children: []database_observability.ExplainPlanNode{
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				Alias:         stringPtr("t"),
																				EstimatedRows: 21978,
																				EstimatedCost: floatPtr(5970.68),
																				Condition:     stringPtr("(((title)::text = ?::text) AND (to_date = ?::date))"),
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
					},
				},
			},
		},
		{
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "window_functions_with_partitioning",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("WindowAgg"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 193761,
						EstimatedCost: floatPtr(3390.82),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("Merge Join"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 193761,
								EstimatedCost: floatPtr(2422.03),
								JoinType:      stringPtr("Inner"),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 193761,
										EstimatedCost: floatPtr(22798.12),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Sort"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 113977,
												EstimatedCost: floatPtr(12588.09),
												SortKeys:      []string{"de.department_id", "s.amount DESC"},
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 113977,
														EstimatedCost: floatPtr(2251.38),
														JoinType:      stringPtr("Inner"),
														JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("e"),
																EstimatedRows: 176485,
																EstimatedCost: floatPtr(4268.85),
															},
														},
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 86472,
																EstimatedCost: floatPtr(0),
															},
															Children: []database_observability.ExplainPlanNode{
																{
																	Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
																	Details: database_observability.ExplainPlanNodeDetails{
																		EstimatedRows: 86472,
																		EstimatedCost: floatPtr(2651.85),
																		JoinType:      stringPtr("Inner"),
																		JoinAlgorithm: explainPlanJoinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
																	},
																	Children: []database_observability.ExplainPlanNode{
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				Alias:         stringPtr("s"),
																				Condition:     stringPtr("(to_date = ?::date)"),
																				EstimatedRows: 100924,
																				EstimatedCost: floatPtr(32972.74),
																			},
																		},
																		{
																			Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash"),
																			Details: database_observability.ExplainPlanNodeDetails{
																				EstimatedRows: 141178,
																				EstimatedCost: floatPtr(0),
																			},
																			Children: []database_observability.ExplainPlanNode{
																				{
																					Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
																					Details: database_observability.ExplainPlanNodeDetails{
																						Alias:         stringPtr("de"),
																						Condition:     stringPtr("(to_date = ?::date)"),
																						EstimatedRows: 141178,
																						EstimatedCost: floatPtr(4598.26),
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
									Operation: database_observability.ExplainPlanOutputOperation("Sort"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 9,
										EstimatedCost: floatPtr(0.17),
										SortKeys:      []string{"d.id"},
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Seq Scan"),
											Details: database_observability.ExplainPlanNodeDetails{
												Alias:         stringPtr("d"),
												EstimatedRows: 9,
												EstimatedCost: floatPtr(1.09),
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
			engineVersion: "14.1",
			queryid:       "1234567890",
			fname:         "window_functions",
			result: &database_observability.ExplainPlanOutput{
				Metadata: database_observability.ExplainPlanMetadataInfo{
					DatabaseEngine:  "PostgreSQL",
					DatabaseVersion: "14.1",
					QueryIdentifier: "1234567890",
					GeneratedAt:     currentTime,
				},
				Plan: database_observability.ExplainPlanNode{
					Operation: database_observability.ExplainPlanOutputOperation("Limit"),
					Details: database_observability.ExplainPlanNodeDetails{
						EstimatedRows: 100,
						EstimatedCost: floatPtr(0),
					},
					Children: []database_observability.ExplainPlanNode{
						{
							Operation: database_observability.ExplainPlanOutputOperation("WindowAgg"),
							Details: database_observability.ExplainPlanNodeDetails{
								EstimatedRows: 242218,
								EstimatedCost: floatPtr(3633.27),
							},
							Children: []database_observability.ExplainPlanNode{
								{
									Operation: database_observability.ExplainPlanOutputOperation("Nested Loop"),
									Details: database_observability.ExplainPlanNodeDetails{
										EstimatedRows: 242218,
										EstimatedCost: floatPtr(108876.83),
										JoinType:      stringPtr("Inner"),
									},
									Children: []database_observability.ExplainPlanNode{
										{
											Operation: database_observability.ExplainPlanOutputOperation("Gather Merge"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 242218,
												EstimatedCost: floatPtr(28957.99),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Sort"),
													Details: database_observability.ExplainPlanNodeDetails{
														EstimatedRows: 100924,
														EstimatedCost: floatPtr(8640.57),
														SortKeys:      []string{"s.amount DESC"},
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Seq Scan"),
															Details: database_observability.ExplainPlanNodeDetails{
																Alias:         stringPtr("s"),
																Condition:     stringPtr("(to_date = ?::date)"),
																EstimatedRows: 100924,
																EstimatedCost: floatPtr(32972.74),
															},
														},
													},
												},
											},
										},
										{
											Operation: database_observability.ExplainPlanOutputOperation("Memoize"),
											Details: database_observability.ExplainPlanNodeDetails{
												EstimatedRows: 1,
												EstimatedCost: floatPtr(.01),
											},
											Children: []database_observability.ExplainPlanNode{
												{
													Operation: database_observability.ExplainPlanOutputOperation("Index Scan"),
													Details: database_observability.ExplainPlanNodeDetails{
														Alias:         stringPtr("e"),
														KeyUsed:       stringPtr("idx_16988_primary"),
														EstimatedRows: 1,
														EstimatedCost: floatPtr(.49),
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
	}

	for _, tt := range tests {
		t.Run(tt.fname, func(t *testing.T) {
			archive, err := txtar.ParseFile(fmt.Sprintf("./testdata/explain_plan/%s.txtar", tt.fname))
			require.NoError(t, err)
			require.Equal(t, 1, len(archive.Files))
			jsonFile := archive.Files[0]
			require.Equal(t, fmt.Sprintf("%s.json", tt.fname), jsonFile.Name)
			jsonData := jsonFile.Data
			output, err := newExplainPlanOutput(tt.engineVersion, tt.queryid, jsonData, currentTime)
			require.NoError(t, err, "Failed generate explain plan output: %s", tt.fname)
			// Override the generated at time to ensure the test is deterministic
			output.Metadata.GeneratedAt = currentTime
			require.Equal(t, tt.result.Metadata, output.Metadata)
			validatePlan(t, tt.result.Plan, output.Plan)
		})
	}
}

func TestReplaceDatabaseNameInDSN(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		newDBName   string
		expected    string
		expectError bool
	}{
		{
			name:      "basic postgres DSN",
			dsn:       "postgres://user:pass@localhost:5432/mydb",
			newDBName: "newdb",
			expected:  "postgres://user:pass@localhost:5432/newdb",
		},
		{
			name:      "postgres DSN with query parameters",
			dsn:       "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
			newDBName: "newdb",
			expected:  "postgres://user:pass@localhost:5432/newdb?sslmode=disable",
		},
		{
			name:      "postgres DSN with multiple query parameters",
			dsn:       "postgres://user:pass@localhost:5432/mydb?sslmode=disable&connect_timeout=10",
			newDBName: "newdb",
			expected:  "postgres://user:pass@localhost:5432/newdb?sslmode=disable&connect_timeout=10",
		},
		{
			name:      "problematic case - database name is 'postgres'",
			dsn:       "postgres://postgres:password@localhost:5432/postgres",
			newDBName: "testdb",
			expected:  "postgres://postgres:password@localhost:5432/testdb",
		},
		{
			name:      "database name appears in password",
			dsn:       "postgres://user:mydb123@localhost:5432/mydb",
			newDBName: "newdb",
			expected:  "postgres://user:mydb123@localhost:5432/newdb",
		},
		{
			name:      "database name with special characters",
			dsn:       "postgres://user:pass@localhost:5432/my-db_test$1",
			newDBName: "new_db",
			expected:  "postgres://user:pass@localhost:5432/new_db",
		},
		{
			name:        "invalid DSN format",
			dsn:         "invalid-dsn-format",
			newDBName:   "newdb",
			expectError: true,
		},
		{
			name:        "DSN without database name",
			dsn:         "postgres://user:pass@localhost:5432/",
			newDBName:   "newdb",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal ExplainPlan instance for testing
			ep := &ExplainPlan{}

			result, err := ep.replaceDatabaseNameInDSN(tt.dsn, tt.newDBName)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}
