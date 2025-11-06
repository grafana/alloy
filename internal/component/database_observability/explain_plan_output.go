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

type ExplainReservedWordMetadata struct {
	ExemptionPrefixes *[]string
}

// ExplainReservedWordDenyList contains SQL reserved words that indicate write operations
// to the database. These are primarily DML (Data Manipulation Language) and DDL
// (Data Definition Language) commands that modify database state.
// This was extracted from the MySQL and PostgreSQL documentation by Claude Sonnet 4 on Oct 28, 2025
// and audited by @rgeyer and others in the dbo11y team.
var ExplainReservedWordDenyList = map[string]ExplainReservedWordMetadata{
	// Data Manipulation Language (DML) - Write operations
	"INSERT": ExplainReservedWordMetadata{},
	"UPDATE": ExplainReservedWordMetadata{
		ExemptionPrefixes: &[]string{"FOR"},
	},
	"DELETE":  ExplainReservedWordMetadata{},
	"REPLACE": ExplainReservedWordMetadata{},
	"MERGE":   ExplainReservedWordMetadata{},
	"UPSERT":  ExplainReservedWordMetadata{},

	// Data Definition Language (DDL) - Schema modifications
	"CREATE":   ExplainReservedWordMetadata{},
	"ALTER":    ExplainReservedWordMetadata{},
	"DROP":     ExplainReservedWordMetadata{},
	"RENAME":   ExplainReservedWordMetadata{},
	"TRUNCATE": ExplainReservedWordMetadata{},

	// Transaction control that can commit writes
	"COMMIT":    ExplainReservedWordMetadata{},
	"ROLLBACK":  ExplainReservedWordMetadata{},
	"SAVEPOINT": ExplainReservedWordMetadata{},

	// Database/Schema management
	"USE":      ExplainReservedWordMetadata{},
	"DATABASE": ExplainReservedWordMetadata{},
	"SCHEMA":   ExplainReservedWordMetadata{},

	// Index operations
	"REINDEX":  ExplainReservedWordMetadata{},
	"ANALYZE":  ExplainReservedWordMetadata{},
	"OPTIMIZE": ExplainReservedWordMetadata{},

	// User/Permission management
	"GRANT":  ExplainReservedWordMetadata{},
	"REVOKE": ExplainReservedWordMetadata{},

	// MySQL specific write operations
	"LOAD":          ExplainReservedWordMetadata{},
	"DELAYED":       ExplainReservedWordMetadata{},
	"IGNORE":        ExplainReservedWordMetadata{},
	"LOW_PRIORITY":  ExplainReservedWordMetadata{},
	"HIGH_PRIORITY": ExplainReservedWordMetadata{},
	"QUICK":         ExplainReservedWordMetadata{},

	// PostgreSQL specific write operations
	"COPY":       ExplainReservedWordMetadata{},
	"VACUUM":     ExplainReservedWordMetadata{},
	"CLUSTER":    ExplainReservedWordMetadata{},
	"LISTEN":     ExplainReservedWordMetadata{},
	"NOTIFY":     ExplainReservedWordMetadata{},
	"DISCARD":    ExplainReservedWordMetadata{},
	"PREPARE":    ExplainReservedWordMetadata{},
	"EXECUTE":    ExplainReservedWordMetadata{},
	"DEALLOCATE": ExplainReservedWordMetadata{},
	"RESET":      ExplainReservedWordMetadata{},
	"SET":        ExplainReservedWordMetadata{},

	// dbo11 specific operations we'd like to exclude
	"EXPLAIN": ExplainReservedWordMetadata{},
}

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
