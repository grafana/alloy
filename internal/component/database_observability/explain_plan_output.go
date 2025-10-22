package database_observability

type ExplainPlanOutputOperation string

const (
	ExplainPlanOutputOperationTableScan            ExplainPlanOutputOperation = "Table Scan"
	ExplainPlanOutputOperationIndexScan            ExplainPlanOutputOperation = "Index Scan"
	ExplainPlanOutputOperationNestedLoopJoin       ExplainPlanOutputOperation = "Nested Loop Join"
	ExplainPlanOutputOperationHashJoin             ExplainPlanOutputOperation = "Hash Join"
	ExplainPlanOutputOperationMergeJoin            ExplainPlanOutputOperation = "Merge Join"
	ExplainPlanOutputOperationGroupingOperation    ExplainPlanOutputOperation = "Grouping Operation"
	ExplainPlanOutputOperationOrderingOperation    ExplainPlanOutputOperation = "Ordering Operation"
	ExplainPlanOutputOperationDuplicatesRemoval    ExplainPlanOutputOperation = "Duplicates Removal"
	ExplainPlanOutputOperationMaterializedSubquery ExplainPlanOutputOperation = "Materialized Subquery"
	ExplainPlanOutputOperationAttachedSubquery     ExplainPlanOutputOperation = "Attached Subquery"
	ExplainPlanOutputOperationUnion                ExplainPlanOutputOperation = "Union"
	ExplainPlanOutputOperationUnknown              ExplainPlanOutputOperation = "Unknown"
)

type ExplainPlanAccessType string

const (
	ExplainPlanAccessTypeAll   ExplainPlanAccessType = "all"
	ExplainPlanAccessTypeIndex ExplainPlanAccessType = "index"
	ExplainPlanAccessTypeRange ExplainPlanAccessType = "range"
	ExplainPlanAccessTypeRef   ExplainPlanAccessType = "ref"
	ExplainPlanAccessTypeEqRef ExplainPlanAccessType = "eq_ref"
)

type ExplainPlanJoinAlgorithm string

const (
	ExplainPlanJoinAlgorithmHash       ExplainPlanJoinAlgorithm = "hash"
	ExplainPlanJoinAlgorithmMerge      ExplainPlanJoinAlgorithm = "merge"
	ExplainPlanJoinAlgorithmNestedLoop ExplainPlanJoinAlgorithm = "nested_loop"
)

type ExplainPlanOutput struct {
	Metadata ExplainPlanMetadataInfo `json:"metadata"`
	Plan     ExplainPlanNode         `json:"plan"`
}

type ExplainPlanMetadataInfo struct {
	DatabaseEngine  string `json:"databaseEngine"`
	DatabaseVersion string `json:"databaseVersion"`
	QueryIdentifier string `json:"queryIdentifier"`
	GeneratedAt     string `json:"generatedAt"`
}

type ExplainPlanNode struct {
	Operation ExplainPlanOutputOperation `json:"operation"`
	Details   ExplainPlanNodeDetails     `json:"details"`
	Children  []ExplainPlanNode          `json:"children,omitempty"`
}

type ExplainPlanNodeDetails struct {
	EstimatedRows int64                     `json:"estimatedRows"`
	EstimatedCost *float64                  `json:"estimatedCost,omitempty"`
	TableName     *string                   `json:"tableName,omitempty"`
	Alias         *string                   `json:"alias,omitempty"`
	AccessType    *ExplainPlanAccessType    `json:"accessType,omitempty"`
	KeyUsed       *string                   `json:"keyUsed,omitempty"`
	JoinType      *string                   `json:"joinType,omitempty"`
	JoinAlgorithm *ExplainPlanJoinAlgorithm `json:"joinAlgorithm,omitempty"`
	Condition     *string                   `json:"condition,omitempty"`
	GroupByKeys   []string                  `json:"groupByKeys,omitempty"`
	SortKeys      []string                  `json:"sortKeys,omitempty"`
	Warning       *string                   `json:"warning,omitempty"`
}
