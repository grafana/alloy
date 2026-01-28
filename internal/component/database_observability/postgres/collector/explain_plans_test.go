package collector

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/goleak"
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

type mockDbConnectionFactory struct {
	Mock               *sqlmock.Sqlmock
	InstantiationCount int
	db                 *sql.DB
}

func (m *mockDbConnectionFactory) NewDBConnection(dsn string) (*sql.DB, error) {
	if m.db == nil {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			return nil, err
		}
		m.db = db
		m.Mock = &mock
	}

	m.InstantiationCount++
	return m.db, nil
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
					DatabaseEngine:   "PostgreSQL",
					DatabaseVersion:  "14.1",
					QueryIdentifier:  "1234567890",
					GeneratedAt:      currentTime,
					ProcessingResult: database_observability.ExplainProcessingResultSuccess,
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
			output, err := newExplainPlanOutput(jsonData)
			require.NoError(t, err, "Failed generate explain plan output: %s", tt.fname)
			validatePlan(t, tt.result.Plan, *output)
		})
	}
}

func TestNewExplainPlan(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := log.NewNopLogger()
	entryHandler := loki.NewCollectingHandler()
	defer entryHandler.Stop()

	t.Run("pre17 version", func(t *testing.T) {
		pre17ver := "14.1"
		pre17semver, err := semver.ParseTolerant(pre17ver)
		require.NoError(t, err)

		args := ExplainPlansArguments{
			DB:               db,
			DSN:              "postgres://user:pass@localhost:5432/testdb",
			ScrapeInterval:   time.Minute,
			PerScrapeRatio:   0.1,
			ExcludeDatabases: []string{"information_schema", "pg_catalog"},
			EntryHandler:     entryHandler,
			DBVersion:        pre17ver,
			Logger:           logger,
		}

		explainPlan, err := NewExplainPlan(args)

		require.NoError(t, err)
		require.NotNil(t, explainPlan)
		assert.Equal(t, db, explainPlan.dbConnection)
		assert.Equal(t, args.DSN, explainPlan.dbDSN)
		assert.Equal(t, pre17semver, explainPlan.dbVersion)
		assert.Equal(t, args.ScrapeInterval, explainPlan.scrapeInterval)
		assert.Equal(t, args.PerScrapeRatio, explainPlan.perScrapeRatio)
		assert.Equal(t, args.ExcludeDatabases, explainPlan.excludeDatabases)
		assert.Equal(t, entryHandler, explainPlan.entryHandler)
		assert.NotNil(t, explainPlan.queryCache)
		assert.NotNil(t, explainPlan.queryDenylist)
		assert.NotNil(t, explainPlan.finishedQueryCache)
		assert.NotNil(t, explainPlan.running)
		assert.False(t, explainPlan.running.Load())
	})

	t.Run("version with trailing characters from docker image", func(t *testing.T) {
		args := ExplainPlansArguments{
			DBVersion: "16.10 (Debian 16.10-1.pgdg13+1)",
		}

		ep, err := NewExplainPlan(args)
		require.NoError(t, err)

		assert.Equal(t, ep.dbVersion.Major, uint64(16))
		assert.Equal(t, ep.dbVersion.Minor, uint64(10))
		assert.Equal(t, ep.dbVersion.Patch, uint64(0))
	})

	t.Run("version with trailing characters from percona helm", func(t *testing.T) {
		args := ExplainPlansArguments{
			DBVersion: "17.7 - Percona Server for PostgreSQL 17.7.1",
		}

		ep, err := NewExplainPlan(args)
		require.NoError(t, err)

		assert.Equal(t, ep.dbVersion.Major, uint64(17))
		assert.Equal(t, ep.dbVersion.Minor, uint64(7))
		assert.Equal(t, ep.dbVersion.Patch, uint64(0))
	})
}

func TestExplainPlan_Name(t *testing.T) {
	explainPlan := &ExplainPlans{}
	assert.Equal(t, ExplainPlanCollector, explainPlan.Name())
}

func TestExplainPlan_Stopped(t *testing.T) {
	explainPlan := &ExplainPlans{
		running: atomic.NewBool(false),
	}
	assert.True(t, explainPlan.Stopped())

	explainPlan.running.Store(true)
	assert.False(t, explainPlan.Stopped())
}

func TestNewQueryInfo(t *testing.T) {
	datname := "testdb"
	queryId := "123456789"
	queryText := "SELECT * FROM users WHERE id = $1"
	calls := int64(100)
	callsReset := time.Now()

	qi := newQueryInfo(datname, queryId, queryText, calls, callsReset)

	assert.Equal(t, datname, qi.datname)
	assert.Equal(t, queryId, qi.queryId)
	assert.Equal(t, queryText, qi.queryText)
	assert.Equal(t, calls, qi.calls)
	assert.Equal(t, callsReset, qi.callsReset)
	assert.Equal(t, datname+queryId, qi.uniqueKey)
	assert.Equal(t, 0, qi.failureCount)
}

func TestPlanNode_TotalCost(t *testing.T) {
	tests := []struct {
		name     string
		planNode PlanNode
		expected float64
	}{
		{
			name: "single node with no children",
			planNode: PlanNode{
				TotalCost: 100.5,
				Plans:     []PlanNode{},
			},
			expected: 100.5,
		},
		{
			name: "node with children",
			planNode: PlanNode{
				TotalCost: 200.75,
				Plans: []PlanNode{
					{TotalCost: 50.25},
					{TotalCost: 30.0},
				},
			},
			expected: 120.5, // 200.75 - 50.25 - 30.0
		},
		{
			name: "negative result becomes zero",
			planNode: PlanNode{
				TotalCost: 50.0,
				Plans: []PlanNode{
					{TotalCost: 60.0},
				},
			},
			expected: 0.0, // 50.0 - 60.0 = -10.0, but clamped to 0
		},
		{
			name: "rounding test",
			planNode: PlanNode{
				TotalCost: 100.123456,
				Plans:     []PlanNode{},
			},
			expected: 100.12, // Rounded to 2 decimal places
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.planNode.totalCost()
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, *result)
		})
	}
}

func TestPlanNode_ExplainPlanNodeOperation(t *testing.T) {
	tests := []struct {
		name     string
		planNode PlanNode
		expected database_observability.ExplainPlanOutputOperation
	}{
		{
			name: "simple node type",
			planNode: PlanNode{
				NodeType: "Seq Scan",
			},
			expected: "Seq Scan",
		},
		{
			name: "node with partial mode",
			planNode: PlanNode{
				NodeType:    "Aggregate",
				PartialMode: "Partial",
			},
			expected: "Partial Aggregate",
		},
		{
			name: "node with strategy - Sorted",
			planNode: PlanNode{
				NodeType: "Aggregate",
				Strategy: "Sorted",
			},
			expected: "Group Aggregate",
		},
		{
			name: "node with strategy - Plain (ignored)",
			planNode: PlanNode{
				NodeType: "Aggregate",
				Strategy: "Plain",
			},
			expected: "Aggregate",
		},
		{
			name: "node with custom strategy",
			planNode: PlanNode{
				NodeType: "Aggregate",
				Strategy: "Hashed",
			},
			expected: "Hashed Aggregate",
		},
		{
			name: "parallel aware node",
			planNode: PlanNode{
				NodeType:      "Seq Scan",
				ParallelAware: true,
			},
			expected: "Parallel Seq Scan",
		},
		{
			name: "complex combination",
			planNode: PlanNode{
				NodeType:      "Aggregate",
				PartialMode:   "Finalize",
				Strategy:      "Sorted",
				ParallelAware: true,
			},
			expected: "Finalize Group Parallel Aggregate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.planNode.explainPlanNodeOperation()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewExplainPlanOutput(t *testing.T) {
	explainJSON := []byte(`[{"Plan": {"Node Type": "Seq Scan", "Total Cost": 100.5, "Plan Rows": 1000, "Plan Width": 50}}]`)

	output, err := newExplainPlanOutput(explainJSON)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, database_observability.ExplainPlanOutputOperation("Seq Scan"), output.Operation)
}

func TestNewExplainPlanOutput_InvalidJSON(t *testing.T) {
	explainJSON := []byte(`invalid json`)

	output, err := newExplainPlanOutput(explainJSON)

	require.Error(t, err)
	assert.Nil(t, output)
}

func TestExplainPlan_PopulateQueryCache(t *testing.T) {
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	logger := log.NewNopLogger()

	pre17ver, err := semver.ParseTolerant("14.1")
	require.NoError(t, err)

	post17ver, err := semver.ParseTolerant("17.0")
	require.NoError(t, err)

	t.Run("populate query cache", func(t *testing.T) {
		t.Run("PostgreSQL < 17", func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			explainPlan := &ExplainPlans{
				dbConnection:       db,
				dbVersion:          pre17ver,
				queryCache:         make(map[string]*queryInfo),
				queryDenylist:      make(map[string]*queryInfo),
				finishedQueryCache: make(map[string]*queryInfo),
				excludeDatabases:   []string{},
				perScrapeRatio:     1.0,
				logger:             logger,
				entryHandler:       lokiClient,
			}

			resetTime := time.Now().Add(-time.Hour)

			resetRows := sqlmock.NewRows([]string{"stats_reset"}).AddRow(resetTime)
			mock.ExpectQuery("SELECT stats_reset FROM pg_stat_statements_info").WillReturnRows(resetRows)

			rows := sqlmock.NewRows([]string{"datname", "queryid", "query", "calls", "stats_since"}).
				AddRow("testdb", "123456", "SELECT * FROM users WHERE id = $1", int64(10), time.Now())

			mock.ExpectQuery(fmt.Sprintf(selectQueriesForExplainPlanTemplate, "NOW() AT TIME ZONE 'UTC' AS stats_since", exclusionClause)).WillReturnRows(rows)

			ctx := context.Background()
			err = explainPlan.populateQueryCache(ctx)

			require.NoError(t, err)
			assert.Len(t, explainPlan.queryCache, 1)
			assert.Equal(t, 1, explainPlan.currentBatchSize)

			qi := explainPlan.queryCache["testdb123456"]
			assert.Equal(t, resetTime, qi.callsReset)

			assert.NoError(t, mock.ExpectationsWereMet())
		})

		t.Run("Postgres 17+", func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			explainPlan := &ExplainPlans{
				dbConnection:       db,
				dbVersion:          post17ver,
				queryCache:         make(map[string]*queryInfo),
				queryDenylist:      make(map[string]*queryInfo),
				finishedQueryCache: make(map[string]*queryInfo),
				excludeDatabases:   []string{"information_schema"},
				perScrapeRatio:     0.5,
				logger:             logger,
				entryHandler:       lokiClient,
			}

			rows := sqlmock.NewRows([]string{"datname", "queryid", "query", "calls", "stats_since"}).
				AddRow("testdb", "123456", "SELECT * FROM users WHERE id = $1", int64(10), time.Now()).
				AddRow("testdb2", "345678", "SELECT * FROM orders", int64(20), time.Now())

			expectedQuery := fmt.Sprintf(selectQueriesForExplainPlanTemplate, "s.stats_since", buildExcludedDatabasesClause([]string{"information_schema"}))
			mock.ExpectQuery(expectedQuery).WillReturnRows(rows)

			ctx := context.Background()
			err = explainPlan.populateQueryCache(ctx)

			require.NoError(t, err)
			assert.Len(t, explainPlan.queryCache, 2)
			assert.Equal(t, 1, explainPlan.currentBatchSize)

			assert.Contains(t, explainPlan.queryCache, "testdb123456")
			assert.Contains(t, explainPlan.queryCache, "testdb2345678")

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	})

	t.Run("call count tracking", func(t *testing.T) {
		t.Run("skips unchanged call count", func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			explainPlan := &ExplainPlans{
				dbConnection: db,
				dbVersion:    post17ver,
				logger:       logger,
				entryHandler: lokiClient,
				queryCache:   make(map[string]*queryInfo),
				finishedQueryCache: map[string]*queryInfo{
					"testdb123456": newQueryInfo("testdb", "123456", "SELECT * FROM users WHERE id = $1", int64(10), time.Now()),
				},
			}

			mock.ExpectQuery(fmt.Sprintf(selectQueriesForExplainPlanTemplate, "s.stats_since", exclusionClause)).
				WillReturnRows(sqlmock.NewRows([]string{"datname", "queryid", "query", "calls", "stats_since"}).
					AddRow("testdb", "123456", "SELECT * FROM users WHERE id = $1", int64(10), time.Now()))

			explainPlan.populateQueryCache(t.Context())

			assert.Len(t, explainPlan.queryCache, 0)
			assert.NoError(t, mock.ExpectationsWereMet())
		})

		t.Run("adds on reset call count and stats_since", func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			explainPlan := &ExplainPlans{
				dbConnection: db,
				dbVersion:    post17ver,
				logger:       logger,
				entryHandler: lokiClient,
				queryCache:   make(map[string]*queryInfo),
				finishedQueryCache: map[string]*queryInfo{
					"testdb123456": newQueryInfo("testdb", "123456", "SELECT * FROM users WHERE id = $1", int64(10), time.Now().Add(-time.Hour)),
				},
			}

			mock.ExpectQuery(fmt.Sprintf(selectQueriesForExplainPlanTemplate, "s.stats_since", exclusionClause)).
				WillReturnRows(sqlmock.NewRows([]string{"datname", "queryid", "query", "calls", "stats_since"}).
					AddRow("testdb", "123456", "SELECT * FROM users WHERE id = $1", int64(1), time.Now()))

			explainPlan.populateQueryCache(t.Context())

			assert.Len(t, explainPlan.queryCache, 1)
			assert.NoError(t, mock.ExpectationsWereMet())
		})

		t.Run("skips on reset call count and unchanged stats_since", func(t *testing.T) {
			// Corner case, but here for completeness.
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			explainPlan := &ExplainPlans{
				dbConnection: db,
				dbVersion:    post17ver,
				logger:       logger,
				entryHandler: lokiClient,
				queryCache:   make(map[string]*queryInfo),
				finishedQueryCache: map[string]*queryInfo{
					"testdb123456": newQueryInfo("testdb", "123456", "SELECT * FROM users WHERE id = $1", int64(10), time.Now()),
				},
			}

			mock.ExpectQuery(fmt.Sprintf(selectQueriesForExplainPlanTemplate, "s.stats_since", exclusionClause)).
				WillReturnRows(sqlmock.NewRows([]string{"datname", "queryid", "query", "calls", "stats_since"}).
					AddRow("testdb", "123456", "SELECT * FROM users WHERE id = $1", int64(1), time.Now().Add(-time.Minute)))

			explainPlan.populateQueryCache(t.Context())

			assert.Len(t, explainPlan.queryCache, 0)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	})

	t.Run("returns error on database connection failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		explainPlan := &ExplainPlans{
			dbConnection: db,
			dbVersion:    pre17ver,
			logger:       logger,
			entryHandler: lokiClient,
		}

		mock.ExpectQuery("SELECT stats_reset FROM pg_stat_statements_info").
			WillReturnError(fmt.Errorf("database connection failed"))

		ctx := context.Background()
		err = explainPlan.populateQueryCache(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "database connection failed")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPlanNode_ToExplainPlanOutputNode(t *testing.T) {
	planNode := PlanNode{
		NodeType:  "Hash Join",
		TotalCost: 150.5,
		PlanRows:  1000,
		PlanWidth: 50,
		JoinType:  "Inner",
		Filter:    "users.id = orders.user_id",
		Alias:     "u",
		IndexName: "idx_user_id",
		GroupKey:  []string{"department"},
		SortKey:   []string{"name", "created_at"},
	}

	result, err := planNode.ToExplainPlanOutputNode()

	require.NoError(t, err)
	assert.Equal(t, database_observability.ExplainPlanOutputOperation("Hash Join"), result.Operation)
	assert.Equal(t, int64(1000), result.Details.EstimatedRows)
	assert.NotNil(t, result.Details.EstimatedCost)
	assert.Equal(t, 150.5, *result.Details.EstimatedCost)
	assert.Equal(t, []string{"department"}, result.Details.GroupByKeys)
	assert.Equal(t, []string{"name", "created_at"}, result.Details.SortKeys)
	assert.NotNil(t, result.Details.JoinType)
	assert.Equal(t, "Inner", *result.Details.JoinType)
	assert.NotNil(t, result.Details.Condition)
	// The redact function should obfuscate the condition, but let's just check it's not empty
	assert.NotEmpty(t, *result.Details.Condition)
	assert.NotNil(t, result.Details.Alias)
	assert.Equal(t, "u", *result.Details.Alias)
	assert.NotNil(t, result.Details.KeyUsed)
	assert.Equal(t, "idx_user_id", *result.Details.KeyUsed)
	assert.NotNil(t, result.Details.JoinAlgorithm)
	assert.Equal(t, database_observability.ExplainPlanJoinAlgorithmHash, *result.Details.JoinAlgorithm)
}

func TestExplainPlanFetchExplainPlans(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	logger := log.NewNopLogger()

	post17ver, err := semver.ParseTolerant("17.0")
	require.NoError(t, err)

	explainPlan := &ExplainPlans{
		dbConnection:        db,
		dbConnectionFactory: defaultDbConnectionFactory,
		dbDSN:               "postgres://user:pass@host:1234/database",
		dbVersion:           post17ver,
		queryCache:          make(map[string]*queryInfo),
		queryDenylist:       make(map[string]*queryInfo),
		finishedQueryCache:  make(map[string]*queryInfo),
		excludeDatabases:    []string{},
		perScrapeRatio:      1.0,
		logger:              logger,
	}

	t.Run("populates query cache when empty", func(t *testing.T) {
		// Mock the populateQueryCache call
		rows := sqlmock.NewRows([]string{"datname", "queryid", "query", "calls", "stats_since"}).
			AddRow("testdb", "123456", "SELECT * FROM users WHERE id = $1", int64(10), time.Now())

		mock.ExpectQuery(fmt.Sprintf(selectQueriesForExplainPlanTemplate, "s.stats_since", exclusionClause)).WillReturnRows(rows)

		ctx := context.Background()
		err = explainPlan.fetchExplainPlans(ctx)

		// Should succeed but not process any queries since they require actual DB connections
		require.NoError(t, err)
		require.Equal(t, 1, len(explainPlan.finishedQueryCache))

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query validation", func(t *testing.T) {
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		logBuffer := syncbuffer.Buffer{}

		t.Run("skips truncated queries", func(t *testing.T) {
			lokiClient.Clear()
			logBuffer.Reset()
			explainPlan = &ExplainPlans{
				dbConnection:        db,
				dbConnectionFactory: defaultDbConnectionFactory,
				dbDSN:               "postgres://user:pass@host:1234/database",
				dbVersion:           post17ver,
				queryCache: map[string]*queryInfo{
					"testdb123456": {
						queryId:    "123456",
						queryText:  "SELECT * FROM users WHERE ...",
						calls:      int64(10),
						callsReset: time.Now(),
					},
				},
				queryDenylist:      map[string]*queryInfo{},
				finishedQueryCache: map[string]*queryInfo{},
				excludeDatabases:   []string{},
				perScrapeRatio:     1.0,
				logger:             log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
				entryHandler:       lokiClient,
				currentBatchSize:   1,
			}

			assert.NoError(t, explainPlan.fetchExplainPlans(t.Context()))
			require.Eventually(
				t,
				func() bool { return len(lokiClient.Received()) == 1 },
				5*time.Second,
				10*time.Millisecond,
				"did not receive the explain plan output log message within the timeout",
			)
			lokiEntries := lokiClient.Received()
			require.Equal(t, 1, len(lokiEntries))
			ep, err := database_observability.ExtractExplainPlanOutputFromLogMsg(lokiEntries[0])
			require.NoError(t, err)
			require.Equal(t, database_observability.ExplainProcessingResultSkipped, ep.Metadata.ProcessingResult)
			require.Equal(t, "query is truncated", ep.Metadata.ProcessingResultReason)
			require.NotContains(t, logBuffer.String(), "error")
			assert.NoError(t, mock.ExpectationsWereMet())
		})

		t.Run("skips non-select queries", func(t *testing.T) {
			lokiClient.Clear()
			logBuffer.Reset()
			explainPlan = &ExplainPlans{
				dbConnection:        db,
				dbConnectionFactory: defaultDbConnectionFactory,
				dbDSN:               "postgres://user:pass@host:1234/database",
				dbVersion:           post17ver,
				queryCache: map[string]*queryInfo{
					"testdb123456": {
						queryId:    "123456",
						queryText:  "update some_table set col = 1 where id = 1",
						calls:      int64(10),
						callsReset: time.Now(),
					},
					"testdb123457": {
						queryId:    "123457",
						queryText:  "delete from some_table",
						calls:      int64(10),
						callsReset: time.Now(),
					},
					"testdb123458": {
						queryId:    "123458",
						queryText:  "insert into some_table (col) values (1)",
						calls:      int64(10),
						callsReset: time.Now(),
					},
				},
				queryDenylist:      map[string]*queryInfo{},
				finishedQueryCache: map[string]*queryInfo{},
				excludeDatabases:   []string{},
				perScrapeRatio:     1.0,
				logger:             log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
				entryHandler:       lokiClient,
				currentBatchSize:   3,
			}

			err = explainPlan.fetchExplainPlans(t.Context())

			require.NoError(t, err)
			require.Eventually(
				t,
				func() bool { return len(lokiClient.Received()) == 3 },
				5*time.Second,
				10*time.Millisecond,
				"did not receive the explain plan output log message within the timeout",
			)

			lokiEntries := lokiClient.Received()
			require.Equal(t, 3, len(lokiEntries))

			require.NotContains(t, logBuffer.String(), "error")
			assert.NoError(t, mock.ExpectationsWereMet())
		})

		t.Run("passes queries beginning in select", func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			lokiClient.Clear()
			logBuffer.Reset()
			dbConnFactory := &mockDbConnectionFactory{
				db:                 db,
				Mock:               &mock,
				InstantiationCount: 0,
			}
			explainPlan = &ExplainPlans{
				dbConnection:        db,
				dbDSN:               "postgres://user:pass@host:1234/database",
				dbConnectionFactory: dbConnFactory.NewDBConnection,
				dbVersion:           post17ver,
				queryCache: map[string]*queryInfo{
					"testdb123456": {
						datname:    "testdb",
						queryId:    "123456",
						queryText:  "select * from some_table where id = $1",
						calls:      int64(10),
						callsReset: time.Now(),
					},
				},
				queryDenylist:      map[string]*queryInfo{},
				finishedQueryCache: map[string]*queryInfo{},
				excludeDatabases:   []string{},
				perScrapeRatio:     1.0,
				logger:             log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
				currentBatchSize:   1,
				entryHandler:       lokiClient,
			}

			archive, err := txtar.ParseFile("./testdata/explain_plan/complex_aggregation_with_case.txtar")
			require.NoError(t, err)
			require.Equal(t, 1, len(archive.Files))
			jsonFile := archive.Files[0]
			require.Equal(t, "complex_aggregation_with_case.json", jsonFile.Name)
			jsonData := jsonFile.Data

			mock.ExpectExec("PREPARE explain_plan_123456 AS select * from some_table where id = $1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("SET search_path TO testdb, public").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("SET plan_cache_mode = force_generic_plan").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("EXPLAIN (FORMAT JSON) EXECUTE explain_plan_123456(null)").WillReturnRows(sqlmock.NewRows([]string{"json"}).AddRow(jsonData))
			mock.ExpectExec("DEALLOCATE explain_plan_123456").WillReturnResult(sqlmock.NewResult(0, 1))

			err = explainPlan.fetchExplainPlans(t.Context())
			require.NoError(t, err)

			assert.NoError(t, mock.ExpectationsWereMet())
			assert.Equal(t, 1, dbConnFactory.InstantiationCount)

			require.Eventually(
				t,
				func() bool {
					return len(lokiClient.Received()) == 1
				},
				5*time.Second,
				10*time.Millisecond,
				"did not receive the explain plan output log message within the timeout",
			)

			require.NotContains(t, logBuffer.String(), "error")
		})

		t.Run("passes queries beginning in with", func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			lokiClient.Clear()
			logBuffer.Reset()
			dbConnFactory := &mockDbConnectionFactory{
				db:                 db,
				Mock:               &mock,
				InstantiationCount: 0,
			}
			explainPlan = &ExplainPlans{
				dbConnection:        db,
				dbDSN:               "postgres://user:pass@host:1234/database",
				dbConnectionFactory: dbConnFactory.NewDBConnection,
				dbVersion:           post17ver,
				queryCache: map[string]*queryInfo{
					"testdb123456": {
						datname:    "testdb",
						queryId:    "123456",
						queryText:  "with cte as (select * from some_table where id = $1) select * from cte",
						calls:      int64(10),
						callsReset: time.Now(),
					},
				},
				queryDenylist:      map[string]*queryInfo{},
				finishedQueryCache: map[string]*queryInfo{},
				excludeDatabases:   []string{},
				perScrapeRatio:     1.0,
				logger:             log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
				currentBatchSize:   1,
				entryHandler:       lokiClient,
			}

			archive, err := txtar.ParseFile("./testdata/explain_plan/complex_aggregation_with_case.txtar")
			require.NoError(t, err)
			require.Equal(t, 1, len(archive.Files))
			jsonFile := archive.Files[0]
			require.Equal(t, "complex_aggregation_with_case.json", jsonFile.Name)
			jsonData := jsonFile.Data

			mock.ExpectExec("PREPARE explain_plan_123456 AS with cte as (select * from some_table where id = $1) select * from cte").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("SET search_path TO testdb, public").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("SET plan_cache_mode = force_generic_plan").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("EXPLAIN (FORMAT JSON) EXECUTE explain_plan_123456(null)").WillReturnRows(sqlmock.NewRows([]string{"json"}).AddRow(jsonData))
			mock.ExpectExec("DEALLOCATE explain_plan_123456").WillReturnResult(sqlmock.NewResult(0, 1))

			err = explainPlan.fetchExplainPlans(t.Context())
			require.NoError(t, err)

			assert.NoError(t, mock.ExpectationsWereMet())
			assert.Equal(t, 1, dbConnFactory.InstantiationCount)

			require.Eventually(
				t,
				func() bool {
					return len(lokiClient.Received()) == 1
				},
				5*time.Second,
				10*time.Millisecond,
				"did not receive the explain plan output log message within the timeout",
			)

			require.NotContains(t, logBuffer.String(), "error")
		})

		t.Run("explain prepared statement for queries without params executed without parens", func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			lokiClient.Clear()
			logBuffer.Reset()
			dbConnFactory := &mockDbConnectionFactory{
				db:                 db,
				Mock:               &mock,
				InstantiationCount: 0,
			}
			explainPlan = &ExplainPlans{
				dbConnection:        db,
				dbDSN:               "postgres://user:pass@host:1234/database",
				dbConnectionFactory: dbConnFactory.NewDBConnection,
				dbVersion:           post17ver,
				queryCache: map[string]*queryInfo{
					"testdb123456": {
						datname:    "testdb",
						queryId:    "123456",
						queryText:  "select * from some_table",
						calls:      int64(10),
						callsReset: time.Now(),
					},
				},
				queryDenylist:      map[string]*queryInfo{},
				finishedQueryCache: map[string]*queryInfo{},
				excludeDatabases:   []string{},
				perScrapeRatio:     1.0,
				logger:             log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
				currentBatchSize:   1,
				entryHandler:       lokiClient,
			}

			archive, err := txtar.ParseFile("./testdata/explain_plan/complex_aggregation_with_case.txtar")
			require.NoError(t, err)
			require.Equal(t, 1, len(archive.Files))
			jsonFile := archive.Files[0]
			require.Equal(t, "complex_aggregation_with_case.json", jsonFile.Name)
			jsonData := jsonFile.Data

			mock.ExpectExec("PREPARE explain_plan_123456 AS select * from some_table").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("SET search_path TO testdb, public").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("SET plan_cache_mode = force_generic_plan").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("EXPLAIN (FORMAT JSON) EXECUTE explain_plan_123456").WillReturnRows(sqlmock.NewRows([]string{"json"}).AddRow(jsonData))
			mock.ExpectExec("DEALLOCATE explain_plan_123456").WillReturnResult(sqlmock.NewResult(0, 1))

			err = explainPlan.fetchExplainPlans(t.Context())
			require.NoError(t, err)

			assert.NoError(t, mock.ExpectationsWereMet())
			assert.Equal(t, 1, dbConnFactory.InstantiationCount)

			require.Eventually(
				t,
				func() bool {
					return len(lokiClient.Received()) == 1
				},
				5*time.Second,
				10*time.Millisecond,
				"did not receive the explain plan output log message within the timeout",
			)

			require.NotContains(t, logBuffer.String(), "error")
		})

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})
}

func TestExplainPlans_ExcludeDatabases_NoLogSent(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	post17ver := semver.MustParse("17.0.0")
	logBuffer := syncbuffer.Buffer{}
	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	// Create ExplainPlans with excluded database
	explainPlan := &ExplainPlans{
		dbConnection:       db,
		dbVersion:          post17ver,
		queryCache:         make(map[string]*queryInfo),
		queryDenylist:      make(map[string]*queryInfo),
		finishedQueryCache: make(map[string]*queryInfo),
		excludeDatabases:   []string{"excluded_db"},
		perScrapeRatio:     1.0,
		logger:             log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
		entryHandler:       lokiClient,
	}

	// Verify the query uses the custom exclusion clause that includes both default and user-provided exclusions
	expectedQuery := fmt.Sprintf(selectQueriesForExplainPlanTemplate, "s.stats_since", buildExcludedDatabasesClause([]string{"excluded_db"}))

	// Return only non-excluded database rows (simulating SQL-level filtering)
	rows := sqlmock.NewRows([]string{"datname", "queryid", "query", "calls", "stats_since"}).
		AddRow("included_db", "222222", "SELECT * FROM included_table", int64(10), time.Now())

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)

	err = explainPlan.populateQueryCache(context.Background())
	require.NoError(t, err)

	// Verify only included_db query is in the cache
	assert.Len(t, explainPlan.queryCache, 1)
	assert.Contains(t, explainPlan.queryCache, "included_db222222")

	assert.NoError(t, mock.ExpectationsWereMet())
}
