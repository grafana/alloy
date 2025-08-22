package collector

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
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

func explainPlanAccessTypePtr(s database_observability.ExplainPlanAccessType) *database_observability.ExplainPlanAccessType {
	return &s
}

func explainPlanJoinAlgorithmPtr(s database_observability.ExplainPlanJoinAlgorithm) *database_observability.ExplainPlanJoinAlgorithm {
	return &s
}

func iterateChildren(t *testing.T, expectChildren, actualChildren []database_observability.ExplainPlanNode) {
	require.Len(t, expectChildren, len(actualChildren))
	for i := range expectChildren {
		require.Equal(t, expectChildren[i].Operation, actualChildren[i].Operation)
		require.Equal(t, expectChildren[i].Details, actualChildren[i].Details)
		iterateChildren(t, expectChildren[i].Children, actualChildren[i].Children)
	}
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
														EstimatedRows: 141178,
														EstimatedCost: floatPtr(546.3),
													},
													Children: []database_observability.ExplainPlanNode{
														{
															Operation: database_observability.ExplainPlanOutputOperation("Parallel Hash Join"),
															Details: database_observability.ExplainPlanNodeDetails{
																EstimatedRows: 141178,
																EstimatedCost: floatPtr(7855.26),
																JoinType:      stringPtr("Inner"),
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
	}

	for _, tt := range tests {
		t.Run(tt.fname, func(t *testing.T) {
			archive, err := txtar.ParseFile(fmt.Sprintf("./testdata/explain_plan/%s.txtar", tt.fname))
			require.NoError(t, err)
			require.Equal(t, 1, len(archive.Files))
			jsonFile := archive.Files[0]
			require.Equal(t, fmt.Sprintf("%s.json", tt.fname), jsonFile.Name)
			jsonData := jsonFile.Data
			logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
			output, err := newExplainPlanOutput(logger, tt.engineVersion, tt.queryid, jsonData, currentTime)
			require.NoError(t, err, "Failed generate explain plan output: %s", tt.fname)
			// Override the generated at time to ensure the test is deterministic
			output.Metadata.GeneratedAt = currentTime
			require.Equal(t, tt.result.Metadata, output.Metadata)
			iterateChildren(t, tt.result.Plan.Children, output.Plan.Children)
		})
	}
}
