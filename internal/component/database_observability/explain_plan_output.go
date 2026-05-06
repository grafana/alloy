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

type ExplainProcessingResult string

const (
	ExplainProcessingResultSuccess ExplainProcessingResult = "success"
	ExplainProcessingResultError   ExplainProcessingResult = "error"
	ExplainProcessingResultSkipped ExplainProcessingResult = "skipped"
)

type ExplainReservedWordMetadata struct {
	ExemptionPrefixes *[]string
}

// ExplainReservedWordDenyList contains SQL reserved words that indicate write operations
// to the database. These are primarily DML and DDL commands that modify database state,
// as well as some common operations that are not relevant to the explain plan output.
var ExplainReservedWordDenyList = map[string]ExplainReservedWordMetadata{
	// Data Manipulation Language (DML) - Write operations
	"INSERT": {},
	"UPDATE": {
		ExemptionPrefixes: &[]string{"FOR"},
	},
	"DELETE":  {},
	"REPLACE": {},
	"MERGE":   {},
	"UPSERT":  {},

	// Data Definition Language (DDL) - Schema modifications
	"CREATE":   {},
	"ALTER":    {},
	"DROP":     {},
	"RENAME":   {},
	"TRUNCATE": {},

	// Transaction control that can commit writes
	"BEGIN":       {},
	"COMMIT":      {},
	"ROLLBACK":    {},
	"SAVEPOINT":   {},
	"TRANSACTION": {},
	"CALL":        {},
	"DO":          {},

	// Database/Schema management
	"USE":      {},
	"DATABASE": {},
	"SCHEMA":   {},

	// Index operations
	"REINDEX":  {},
	"ANALYZE":  {},
	"OPTIMIZE": {},

	// User/Permission management
	"GRANT":  {},
	"REVOKE": {},

	// MySQL specific write operations
	"LOAD":          {},
	"DELAYED":       {},
	"IGNORE":        {},
	"LOW_PRIORITY":  {},
	"HIGH_PRIORITY": {},
	"QUICK":         {},
	"SHOW":          {},
	"KILL":          {},

	// PostgreSQL specific write operations
	"COPY":       {},
	"VACUUM":     {},
	"CLUSTER":    {},
	"LISTEN":     {},
	"NOTIFY":     {},
	"DISCARD":    {},
	"PREPARE":    {},
	"EXECUTE":    {},
	"DEALLOCATE": {},
	"RESET":      {},
	"SET":        {},
	"UNLISTEN":   {},
	"DECLARE":    {},
	"CLOSE":      {},

	// DBo11y-specific operations we'd like to exclude
	"EXPLAIN": {},
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

	ProcessingResult       ExplainProcessingResult `json:"processingResult"`
	ProcessingResultReason string                  `json:"processingResultReason"`
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
