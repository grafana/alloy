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

// ExplainReservedWordDenyList contains SQL reserved words that indicate write operations
// to the database. These are primarily DML (Data Manipulation Language) and DDL
// (Data Definition Language) commands that modify database state.
// This was extracted from the MySQL and PostgreSQL documentation by Claude Sonnet 4 on Oct 28, 2025
// and audited by @rgeyer and others in the dbo11y team.
var ExplainReservedWordDenyList = map[string]bool{
	// Data Manipulation Language (DML) - Write operations
	"INSERT": true, "UPDATE": true, "DELETE": true, "REPLACE": true, "MERGE": true, "UPSERT": true,
	"FOR UPDATE": true,

	// Data Definition Language (DDL) - Schema modifications
	"CREATE": true, "ALTER": true, "DROP": true, "RENAME": true, "TRUNCATE": true,

	// Transaction control that can commit writes
	"COMMIT": true, "ROLLBACK": true, "SAVEPOINT": true,

	// Database/Schema management
	"USE": true, "DATABASE": true, "SCHEMA": true,

	// Index operations
	"REINDEX": true, "ANALYZE": true, "OPTIMIZE": true,
	"REINDEX TABLE": true, "ANALYZE TABLE": true, "OPTIMIZE TABLE": true,
	// User/Permission management
	"GRANT": true, "REVOKE": true, "CREATE USER": true, "DROP USER": true, "ALTER USER": true,

	// MySQL specific write operations
	"LOAD": true, "DELAYED": true, "IGNORE": true, "ON DUPLICATE KEY": true,
	"LOW_PRIORITY": true, "HIGH_PRIORITY": true, "QUICK": true, "LOCK": true, "UNLOCK": true,
	"LOCK IN SHARE MODE": true,

	// PostgreSQL specific write operations
	"COPY": true, "VACUUM": true, "CLUSTER": true, "LISTEN": true, "NOTIFY": true, "DISCARD": true,
	"PREPARE": true, "EXECUTE": true, "DEALLOCATE": true, "RESET": true, "SET": true,

	// dbo11 specific operations we'd like to exclude
	"EXPLAIN": true,
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
