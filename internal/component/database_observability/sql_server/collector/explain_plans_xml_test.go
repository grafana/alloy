package collector

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"

	"github.com/grafana/alloy/internal/component/database_observability"
)

func stringPtr(s string) *string {
	return &s
}

func floatPtr(f float64) *float64 {
	return &f
}

func accessTypePtr(a database_observability.ExplainPlanAccessType) *database_observability.ExplainPlanAccessType {
	return &a
}

func joinAlgorithmPtr(a database_observability.ExplainPlanJoinAlgorithm) *database_observability.ExplainPlanJoinAlgorithm {
	return &a
}

func loadShowPlanFixture(t *testing.T, name string) []byte {
	t.Helper()

	archive, err := txtar.ParseFile(fmt.Sprintf("./testdata/explain_plan/%s.txtar", name))
	require.NoError(t, err)
	require.Len(t, archive.Files, 1)
	require.Equal(t, fmt.Sprintf("%s.xml", name), archive.Files[0].Name)
	return archive.Files[0].Data
}

func TestNewExplainPlanOutputFromShowPlanXML(t *testing.T) {
	tests := []struct {
		fixture string
		want    database_observability.ExplainPlanNode
	}{
		{
			fixture: "nested_loop_seek_and_scan",
			want: database_observability.ExplainPlanNode{
				Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
				Details: database_observability.ExplainPlanNodeDetails{
					EstimatedRows: 12,
					EstimatedCost: floatPtr(0.045),
					JoinAlgorithm: joinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmNestedLoop),
					JoinType:      stringPtr("Inner Join"),
				},
				Children: []database_observability.ExplainPlanNode{
					{
						Operation: database_observability.ExplainPlanOutputOperationIndexScan,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 8,
							EstimatedCost: floatPtr(0.012),
							TableName:     stringPtr("Orders"),
							KeyUsed:       stringPtr("PK_Orders"),
							AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeIndex),
							Condition:     stringPtr("[dbo].[Orders].[Total]>(?)"),
						},
					},
					{
						Operation: database_observability.ExplainPlanOutputOperationIndexScan,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 1,
							EstimatedCost: floatPtr(0.003),
							TableName:     stringPtr("Customers"),
							KeyUsed:       stringPtr("PK_Customers"),
							AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeEqRef),
						},
					},
				},
			},
		},
		{
			fixture: "hash_match_join",
			want: database_observability.ExplainPlanNode{
				Operation: database_observability.ExplainPlanOutputOperationHashJoin,
				Details: database_observability.ExplainPlanNodeDetails{
					EstimatedRows: 500,
					EstimatedCost: floatPtr(1.2),
					JoinAlgorithm: joinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
					JoinType:      stringPtr("Inner Join"),
				},
				Children: []database_observability.ExplainPlanNode{
					{
						Operation: database_observability.ExplainPlanOutputOperationTableScan,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 1000,
							EstimatedCost: floatPtr(0.5),
							TableName:     stringPtr("Customers"),
							AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeAll),
						},
					},
					{
						Operation: database_observability.ExplainPlanOutputOperationTableScan,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 2000,
							EstimatedCost: floatPtr(0.6),
							TableName:     stringPtr("Orders"),
							AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeAll),
						},
					},
				},
			},
		},
		{
			fixture: "hash_match_aggregate",
			want: database_observability.ExplainPlanNode{
				Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
				Details: database_observability.ExplainPlanNodeDetails{
					EstimatedRows: 300,
					EstimatedCost: floatPtr(0.9),
					GroupByKeys:   []string{"CustomerId"},
				},
				Children: []database_observability.ExplainPlanNode{
					{
						Operation: database_observability.ExplainPlanOutputOperationTableScan,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 2000,
							EstimatedCost: floatPtr(0.6),
							TableName:     stringPtr("Orders"),
							AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeAll),
						},
					},
				},
			},
		},
		{
			fixture: "sort_and_top",
			want: database_observability.ExplainPlanNode{
				Operation: database_observability.ExplainPlanOutputOperationTop,
				Details: database_observability.ExplainPlanNodeDetails{
					EstimatedRows: 10,
					EstimatedCost: floatPtr(0.7),
				},
				Children: []database_observability.ExplainPlanNode{
					{
						Operation: database_observability.ExplainPlanOutputOperationOrderingOperation,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 2000,
							EstimatedCost: floatPtr(0.65),
							SortKeys:      []string{"OrderDate"},
						},
						Children: []database_observability.ExplainPlanNode{
							{
								Operation: database_observability.ExplainPlanOutputOperationTableScan,
								Details: database_observability.ExplainPlanNodeDetails{
									EstimatedRows: 2000,
									EstimatedCost: floatPtr(0.6),
									TableName:     stringPtr("Orders"),
									AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeAll),
								},
							},
						},
					},
				},
			},
		},
		{
			fixture: "compute_scalar_with_filter",
			want: database_observability.ExplainPlanNode{
				Operation: database_observability.ExplainPlanOutputOperationComputeScalar,
				Details: database_observability.ExplainPlanNodeDetails{
					EstimatedRows: 80,
					EstimatedCost: floatPtr(0.55),
				},
				Children: []database_observability.ExplainPlanNode{
					{
						Operation: database_observability.ExplainPlanOutputOperationFilter,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 80,
							EstimatedCost: floatPtr(0.54),
							Condition:     stringPtr("[dbo].[Orders].[Total]>(?)"),
						},
						Children: []database_observability.ExplainPlanNode{
							{
								Operation: database_observability.ExplainPlanOutputOperationTableScan,
								Details: database_observability.ExplainPlanNodeDetails{
									EstimatedRows: 2000,
									EstimatedCost: floatPtr(0.5),
									TableName:     stringPtr("Orders"),
									AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeAll),
								},
							},
						},
					},
				},
			},
		},
		{
			fixture: "concatenation",
			want: database_observability.ExplainPlanNode{
				Operation: database_observability.ExplainPlanOutputOperationUnion,
				Details: database_observability.ExplainPlanNodeDetails{
					EstimatedRows: 1500,
					EstimatedCost: floatPtr(1.1),
				},
				Children: []database_observability.ExplainPlanNode{
					{
						Operation: database_observability.ExplainPlanOutputOperationTableScan,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 1000,
							EstimatedCost: floatPtr(0.5),
							TableName:     stringPtr("ActiveCustomers"),
							AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeAll),
						},
					},
					{
						Operation: database_observability.ExplainPlanOutputOperationTableScan,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 500,
							EstimatedCost: floatPtr(0.4),
							TableName:     stringPtr("ArchivedCustomers"),
							AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeAll),
						},
					},
				},
			},
		},
		{
			fixture: "warnings",
			want: database_observability.ExplainPlanNode{
				Operation: database_observability.ExplainPlanOutputOperationHashJoin,
				Details: database_observability.ExplainPlanNodeDetails{
					EstimatedRows: 2000000,
					EstimatedCost: floatPtr(45.2),
					JoinAlgorithm: joinAlgorithmPtr(database_observability.ExplainPlanJoinAlgorithmHash),
					JoinType:      stringPtr("Inner Join"),
					Warnings:      []string{"no join predicate (cartesian product)", "spilled to tempdb"},
				},
				Children: []database_observability.ExplainPlanNode{
					{
						Operation: database_observability.ExplainPlanOutputOperationTableScan,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 1000,
							EstimatedCost: floatPtr(0.5),
							TableName:     stringPtr("Customers"),
							AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeAll),
						},
					},
					{
						Operation: database_observability.ExplainPlanOutputOperationTableScan,
						Details: database_observability.ExplainPlanNodeDetails{
							EstimatedRows: 2000,
							EstimatedCost: floatPtr(0.6),
							TableName:     stringPtr("Orders"),
							AccessType:    accessTypePtr(database_observability.ExplainPlanAccessTypeAll),
							Warnings:      []string{"columns with no statistics"},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			xmlData := loadShowPlanFixture(t, test.fixture)
			got, err := newExplainPlanOutputFromShowPlanXML(xmlData)
			require.NoError(t, err)
			require.Equal(t, test.want, *got)
		})
	}
}

func TestNewExplainPlanOutputFromShowPlanXML_Errors(t *testing.T) {
	t.Run("invalid xml", func(t *testing.T) {
		_, err := newExplainPlanOutputFromShowPlanXML([]byte("not xml"))
		require.Error(t, err)
	})

	t.Run("no statements", func(t *testing.T) {
		_, err := newExplainPlanOutputFromShowPlanXML([]byte(`<ShowPlanXML xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><BatchSequence><Batch><Statements></Statements></Batch></BatchSequence></ShowPlanXML>`))
		require.Error(t, err)
	})
}
