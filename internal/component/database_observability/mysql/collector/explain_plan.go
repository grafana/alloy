package collector

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/buger/jsonparser"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	OP_EXPLAIN_PLAN        = "explain_plan"
	OP_EXPLAIN_PLAN_OUTPUT = "explain_plan_output"
	ExplainPlanName        = "explain_plan"
)

const selectDigestsForExplainPlan = `
	SELECT
		CURRENT_SCHEMA,
		DIGEST,
		SQL_TEXT
	FROM performance_schema.events_statements_history
	WHERE TIMER_END > DATE_SUB(NOW(), INTERVAL 1 DAY)
	AND SQL_TEXT IS NOT NULL
	AND DIGEST IS NOT NULL
	AND CURRENT_SCHEMA NOT IN ('mysql', 'performance_schema', 'sys', 'information_schema');`

const selectExplainPlansPrefix = `EXPLAIN FORMAT=JSON `

const selectDbSchemaVersion = `SELECT VERSION()`

type queryInfo struct {
	schemaName *string
	digest     string
	queryText  string
}

type ExplainPlanArguments struct {
	DB             *sql.DB
	InstanceKey    string
	ScrapeInterval time.Duration
	PerScrapeRatio float64
	EntryHandler   loki.EntryHandler

	Logger log.Logger
}

type ExplainPlan struct {
	dbConnection     *sql.DB
	instanceKey      string
	dbVersion        string
	scrapeInterval   time.Duration
	queryCache       []queryInfo
	perScrapeRatio   float64
	currentBatchSize int
	entryHandler     loki.EntryHandler

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

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
	Metadata MetadataInfo `json:"metadata"`
	Plan     PlanNode     `json:"plan"`
}

type MetadataInfo struct {
	DatabaseEngine  string `json:"database_engine"`
	DatabaseVersion string `json:"database_version"`
	QueryIdentifier string `json:"query_identifier"`
	GeneratedAt     string `json:"generated_at"`
}

type PlanNode struct {
	Operation ExplainPlanOutputOperation `json:"operation"`
	Details   NodeDetails                `json:"details"`
	Children  []PlanNode                 `json:"children,omitempty"`
}

type NodeDetails struct {
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

func NewExplainPlanOutput(logger log.Logger, dbVersion string, digest string, explainJson []byte, generatedAt string) (*ExplainPlanOutput, error) {
	output := &ExplainPlanOutput{
		Metadata: MetadataInfo{
			DatabaseEngine:  "MySQL",
			DatabaseVersion: dbVersion,
			QueryIdentifier: digest,
			GeneratedAt:     generatedAt,
		},
	}

	qblock, qblockType, _, err := jsonparser.Get(explainJson, "query_block")
	if qblockType == jsonparser.NotExist {
		errMsg := "no query block found in explain plan"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		return nil, errors.New(errMsg)
	}

	planNode, err := parseTopLevelPlanNode(logger, qblock)
	if err != nil {
		return nil, err
	}
	output.Plan = planNode

	return output, nil
}

func parseTopLevelPlanNode(logger log.Logger, topLevelPlanNode []byte) (PlanNode, error) {
	// Table at top level
	if table, _, _, err := jsonparser.Get(topLevelPlanNode, "table"); err == nil {
		tableDetails, err := parseTableNode(logger, table)
		if err != nil {
			return PlanNode{}, err
		}

		return tableDetails, nil
	} // else we don't find the table at the top level. This is likely not an error because the NodeDetails may be for a different type of operation.
	// We may have to check to see if no operation has been set after checking all known operation types.

	// Nested Loop Join
	if nestedLoopJoin, _, _, err := jsonparser.Get(topLevelPlanNode, "nested_loop"); err == nil {
		planNode, err := parseNestedLoopJoinNode(logger, nestedLoopJoin)
		if err != nil {
			return PlanNode{}, err
		}
		return planNode, nil
	}

	// Grouping Operation
	if groupingOperation, _, _, err := jsonparser.Get(topLevelPlanNode, "grouping_operation"); err == nil {
		planNode, err := parseGroupingOperationNode(logger, groupingOperation)
		if err != nil {
			return PlanNode{}, err
		}
		return planNode, nil
	}

	// Ordering Operation
	if orderingOperation, _, _, err := jsonparser.Get(topLevelPlanNode, "ordering_operation"); err == nil {
		planNode, err := parseOrderingOperationNode(logger, orderingOperation)
		if err != nil {
			return PlanNode{}, err
		}
		return planNode, nil
	}

	// Duplicates Removal
	if duplicatesRemoval, _, _, err := jsonparser.Get(topLevelPlanNode, "duplicates_removal"); err == nil {
		planNode, err := parseDuplicatesRemovalNode(logger, duplicatesRemoval)
		if err != nil {
			return PlanNode{}, err
		}
		return planNode, nil
	}

	if unionResult, _, _, err := jsonparser.Get(topLevelPlanNode, "union_result"); err == nil {
		planNode, err := parseUnionResultNode(logger, unionResult)
		if err != nil {
			return PlanNode{}, err
		}
		return planNode, nil
	}

	return PlanNode{}, nil
}

func parseTableNode(logger log.Logger, tableNode []byte) (PlanNode, error) {
	planNode := PlanNode{
		Operation: ExplainPlanOutputOperationTableScan,
		Details:   NodeDetails{},
	}

	// Check for join algorithm. Nested loop would be set in parseNestedLoopJoinNode, since not all table nodes are children of nested loop joins.
	if joinAlgorithm, _, _, err := jsonparser.Get(tableNode, "using_join_buffer"); err == nil {
		if string(joinAlgorithm) == "hash join" {
			joinAlgorithmConst := ExplainPlanJoinAlgorithmHash
			planNode.Details.JoinAlgorithm = &joinAlgorithmConst
		}
	}

	tableAlias, err := jsonparser.GetString(tableNode, "table_name")
	// TODO: This should output some of the context in the original explain plan JSON so that errors in nested objects can be found.
	if err != nil {
		return planNode, fmt.Errorf("failed to get table alias: %w", err)
	}
	planNode.Details.Alias = &tableAlias

	accessType, err := jsonparser.GetString(tableNode, "access_type")
	if err != nil {
		return planNode, fmt.Errorf("failed to get access type: %w", err)
	}
	accessTypeconst := ExplainPlanAccessType(strings.ToLower(accessType))
	planNode.Details.AccessType = &accessTypeconst

	// Until now, the properties being parsed were probably mandatory, now let's look for ones that are optional.
	estimatedRows, err := jsonparser.GetInt(tableNode, "rows_produced_per_join")
	if err == nil {
		planNode.Details.EstimatedRows = estimatedRows
	}

	estimatedCost, err := jsonparser.GetString(tableNode, "cost_info", "prefix_cost")
	if err == nil {
		estimatedCostFloat, err := strconv.ParseFloat(estimatedCost, 64)
		if err != nil {
			return planNode, fmt.Errorf("failed to parse estimated cost as float: %w", err)
		}
		planNode.Details.EstimatedCost = &estimatedCostFloat
	}

	attachedCondition, _, _, err := jsonparser.Get(tableNode, "attached_condition")
	if err == nil {
		parser := parser.NewTiDBSqlParser()
		redactedAttachedCondition, err := parser.Redact(string(attachedCondition))
		if err != nil {
			return planNode, fmt.Errorf("failed to redact attached condition: %s error: %w", string(attachedCondition), err)
		}
		planNode.Details.Condition = &redactedAttachedCondition
	}

	keyUsed, err := jsonparser.GetString(tableNode, "key")
	if err == nil {
		planNode.Details.KeyUsed = &keyUsed
	}

	if materializedSubquery, _, _, err := jsonparser.Get(tableNode, "materialized_from_subquery"); err == nil {
		childNode, err := parseMaterializedSubqueryNode(logger, materializedSubquery)
		if err != nil {
			return PlanNode{}, err
		}
		planNode.Children = append(planNode.Children, childNode)
	}

	if attachedSubquery, _, _, err := jsonparser.Get(tableNode, "attached_subqueries"); err == nil {
		_, err := jsonparser.ArrayEach(attachedSubquery, func(value []byte, dataType jsonparser.ValueType, offset int, inerr error) {
			childNode, err := parseAttachedSubqueryNode(logger, value)
			if err != nil {
				return
			}
			planNode.Children = append(planNode.Children, childNode)
		})
		if err != nil {
			return planNode, err
		}
	}

	return planNode, nil
}

func parseNestedLoopJoinNode(logger log.Logger, nestedLoopJoinNode []byte) (PlanNode, error) {
	algo := ExplainPlanJoinAlgorithmNestedLoop
	planNode := PlanNode{
		Operation: ExplainPlanOutputOperationNestedLoopJoin,
		Details: NodeDetails{
			JoinAlgorithm: &algo,
		},
		Children: make([]PlanNode, 0),
	}
	var previousChild *PlanNode
	_, err := jsonparser.ArrayEach(nestedLoopJoinNode, func(value []byte, dataType jsonparser.ValueType, offset int, inerr error) {
		tableNode, _, _, err := jsonparser.Get(value, "table")
		if err != nil {
			level.Debug(logger).Log("msg", "no table node found in nested loop join", "error", err)
			// In theory, this could be okay? Are there other things that could be in nested loop?
			return
		}
		childDetails, err := parseTableNode(logger, tableNode)
		if err != nil {
			level.Error(logger).Log("msg", "failed to parse table node in nested loop join", "error", err)
			return
		}
		if childDetails.Details.JoinAlgorithm == nil {
			childDetails.Details.JoinAlgorithm = &algo // TODO:This is a duplicate from the parent node. Is that necessary?
		}
		if previousChild != nil {
			thisLoop := PlanNode{
				Operation: ExplainPlanOutputOperationNestedLoopJoin,
				Details: NodeDetails{
					JoinAlgorithm: &algo,
				},
				Children: []PlanNode{
					*previousChild,
					childDetails,
				},
			}
			if childDetails.Details.JoinAlgorithm != nil && *childDetails.Details.JoinAlgorithm != algo {
				thisLoop.Details.JoinAlgorithm = childDetails.Details.JoinAlgorithm
				if *childDetails.Details.JoinAlgorithm == ExplainPlanJoinAlgorithmHash {
					thisLoop.Operation = ExplainPlanOutputOperationHashJoin
				} else if *childDetails.Details.JoinAlgorithm == ExplainPlanJoinAlgorithmMerge {
					thisLoop.Operation = ExplainPlanOutputOperationMergeJoin
				} else {
					thisLoop.Operation = ExplainPlanOutputOperationNestedLoopJoin
				}
			}
			previousChild = &thisLoop
		} else {
			previousChild = &childDetails
		}
	})
	if err != nil {
		return planNode, err
	}
	if previousChild != nil {
		if previousChild.Operation != ExplainPlanOutputOperationNestedLoopJoin && previousChild.Operation != ExplainPlanOutputOperationHashJoin {
			planNode.Children = append(planNode.Children, *previousChild)
		} else {
			return *previousChild, nil
		}
	}
	return planNode, nil
}

func parseGroupingOperationNode(logger log.Logger, groupingOperationNode []byte) (PlanNode, error) {
	planNode := PlanNode{
		Operation: ExplainPlanOutputOperationGroupingOperation,
	}

	children, err := parseTopLevelPlanNode(logger, groupingOperationNode)
	if err != nil {
		return PlanNode{}, err
	}
	planNode.Children = append(planNode.Children, children)

	return planNode, nil
}

func parseOrderingOperationNode(logger log.Logger, orderingOperationNode []byte) (PlanNode, error) {
	planNode := PlanNode{
		Operation: ExplainPlanOutputOperationOrderingOperation,
	}

	children, err := parseTopLevelPlanNode(logger, orderingOperationNode)
	if err != nil {
		return PlanNode{}, err
	}
	planNode.Children = append(planNode.Children, children)

	return planNode, nil
}

func parseDuplicatesRemovalNode(logger log.Logger, duplicatesRemovalNode []byte) (PlanNode, error) {
	planNode := PlanNode{
		Operation: ExplainPlanOutputOperationDuplicatesRemoval,
	}

	children, err := parseTopLevelPlanNode(logger, duplicatesRemovalNode)
	if err != nil {
		return PlanNode{}, err
	}
	planNode.Children = append(planNode.Children, children)

	return planNode, nil
}

func parseMaterializedSubqueryNode(logger log.Logger, materializedSubqueryNode []byte) (PlanNode, error) {
	planNode := PlanNode{
		Operation: ExplainPlanOutputOperationMaterializedSubquery,
	}

	queryBlock, _, _, err := jsonparser.Get(materializedSubqueryNode, "query_block")
	if err != nil {
		return planNode, fmt.Errorf("failed to get query block: %w", err)
	}

	childNode, err := parseTopLevelPlanNode(logger, queryBlock)
	if err != nil {
		return planNode, fmt.Errorf("failed to parse top level plan node: %w", err)
	}
	planNode.Children = append(planNode.Children, childNode)

	return planNode, nil
}

func parseAttachedSubqueryNode(logger log.Logger, attachedSubqueryNode []byte) (PlanNode, error) {
	planNode := PlanNode{
		Operation: ExplainPlanOutputOperationAttachedSubquery,
	}

	queryBlock, _, _, err := jsonparser.Get(attachedSubqueryNode, "query_block")
	if err != nil {
		return planNode, fmt.Errorf("failed to get query block: %w", err)
	}

	childNode, err := parseTopLevelPlanNode(logger, queryBlock)
	if err != nil {
		return PlanNode{}, err
	}
	planNode.Children = append(planNode.Children, childNode)

	return planNode, nil
}

func parseUnionResultNode(logger log.Logger, unionResultNode []byte) (PlanNode, error) {
	planNode := PlanNode{
		Operation: ExplainPlanOutputOperationUnion,
	}

	querySpecifications, _, _, err := jsonparser.Get(unionResultNode, "query_specifications")
	if err != nil {
		return planNode, fmt.Errorf("failed to get query specifications: %w", err)
	}

	_, err = jsonparser.ArrayEach(querySpecifications, func(value []byte, dataType jsonparser.ValueType, offset int, inerr error) {
		queryBlock, _, _, err := jsonparser.Get(value, "query_block")
		if err != nil {
			return
		}
		childNode, err := parseTopLevelPlanNode(logger, queryBlock)
		if err != nil {
			return
		}
		planNode.Children = append(planNode.Children, childNode)
	})
	if err != nil {
		return planNode, err
	}
	return planNode, nil
}

func NewExplainPlan(args ExplainPlanArguments) (*ExplainPlan, error) {
	rs := args.DB.QueryRowContext(context.Background(), selectDbSchemaVersion)
	if rs.Err() != nil {
		return nil, rs.Err()
	}

	var dbVersion string
	if err := rs.Scan(&dbVersion); err != nil {
		return nil, err
	}

	return &ExplainPlan{
		dbConnection:   args.DB,
		instanceKey:    args.InstanceKey,
		dbVersion:      dbVersion,
		scrapeInterval: args.ScrapeInterval,
		queryCache:     make([]queryInfo, 0),
		perScrapeRatio: args.PerScrapeRatio,
		entryHandler:   args.EntryHandler,
		logger:         log.With(args.Logger, "collector", ExplainPlanName),
		running:        atomic.NewBool(false),
	}, nil
}

func (c *ExplainPlan) Name() string {
	return ExplainPlanName
}

func (c *ExplainPlan) Start(ctx context.Context) error {
	level.Info(c.logger).Log("msg", "collector started")

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	go func() {
		defer func() {
			c.Stop()
			c.running.Store(false)
		}()

		ticker := time.NewTicker(c.scrapeInterval)

		for {
			if err := c.fetchExplainPlans(c.ctx); err != nil {
				level.Error(c.logger).Log("msg", "collector error", "err", err)
			}

			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	}()

	return nil
}

func (c *ExplainPlan) Stopped() bool {
	return !c.running.Load()
}

func (c *ExplainPlan) Stop() {
	c.cancel()
}

func (c *ExplainPlan) fetchExplainPlans(ctx context.Context) error {
	// If cache is empty, fetch all available queries
	if len(c.queryCache) == 0 {
		rs, err := c.dbConnection.QueryContext(ctx, selectDigestsForExplainPlan)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to fetch digests for explain plans", "err", err)
			return err
		}
		defer rs.Close()

		// Populate cache
		for rs.Next() {
			if err := rs.Err(); err != nil {
				level.Error(c.logger).Log("msg", "failed to iterate rs digests for explain plans", "err", err)
				return err
			}

			var qi queryInfo
			if err = rs.Scan(&qi.schemaName, &qi.digest, &qi.queryText); err != nil {
				level.Error(c.logger).Log("msg", "failed to scan digest for explain plans", "err", err)
				return err
			}
			c.queryCache = append(c.queryCache, qi)
		}
		// Calculate batch size based on current cache size
		c.currentBatchSize = int(math.Ceil(float64(len(c.queryCache)) * c.perScrapeRatio))
		level.Info(c.logger).Log("msg", "fetched digests", "count", len(c.queryCache), "batch_size", c.currentBatchSize)
	}

	// Process up to batchSize queries from cache
	processedCount := 0
	for i, qi := range c.queryCache {
		logger := log.With(c.logger, "digest", qi.digest)
		if processedCount >= c.currentBatchSize {
			break
		}

		// Defer deletion of current item from cache and increment processed count
		defer func(index int) {
			c.queryCache = slices.Delete(c.queryCache, index, index+1)
			processedCount++
		}(i)

		// Skip truncated queries
		if strings.HasSuffix(qi.queryText, "...") {
			level.Debug(logger).Log("msg", "skipping truncated query")
			continue
		}

		// Skip non-select queries
		if !strings.HasPrefix(strings.ToLower(qi.queryText), "select") {
			parser := parser.NewTiDBSqlParser()
			redacted, err := parser.Redact(qi.queryText)
			if err != nil {
				level.Error(logger).Log("msg", "failed to redact sql", "err", err)
				continue
			}
			level.Debug(logger).Log("msg", "skipping non-select query", "query_text", redacted)
			continue
		}

		// Add schema context if available
		if qi.schemaName != nil {
			logger = log.With(logger, "schema_name", *qi.schemaName)
			// First set the schema
			if _, err := c.dbConnection.ExecContext(ctx, "USE "+*qi.schemaName); err != nil {
				level.Error(logger).Log("msg", "failed to set schema", "err", err)
				continue
			}
		}

		rsExplain := c.dbConnection.QueryRowContext(ctx, selectExplainPlansPrefix+qi.queryText)

		if err := rsExplain.Err(); err != nil {
			level.Error(logger).Log("msg", "failed to iterate rs for explain plan", "err", err)
			continue
		}

		var byteExplainPlanJSON []byte
		if err := rsExplain.Scan(&byteExplainPlanJSON); err != nil {
			level.Error(logger).Log("msg", "failed to scan explain plan json", "err", err)
			continue
		}

		generatedAt := time.Now().Format(time.RFC3339)

		// Skip if byteExplainPlanJSON is nil or empty
		if len(byteExplainPlanJSON) == 0 {
			level.Error(logger).Log("msg", "explain plan json bytes is empty")
			continue
		}

		// Validate that it's valid UTF-8 encoded JSON
		if !utf8.Valid(byteExplainPlanJSON) {
			level.Error(logger).Log("msg", "explain plan json bytes is not valid UTF-8")
			continue
		}

		loggedSchemaName := "<nil>"
		if qi.schemaName != nil {
			loggedSchemaName = *qi.schemaName
		}

		redactedByteExplainPlanJSON, _, err := RedactAttachedConditions(byteExplainPlanJSON)
		if err != nil {
			level.Error(logger).Log("msg", "failed to redact explain plan json", "err", err)
			continue
		}

		level.Debug(logger).Log("msg", "explain plan output",
			"op", OP_EXPLAIN_PLAN_OUTPUT,
			"instance", c.instanceKey,
			"explain_plan_output", base64.StdEncoding.EncodeToString(redactedByteExplainPlanJSON))

		explainPlanOutput, err := NewExplainPlanOutput(logger, c.dbVersion, qi.digest, byteExplainPlanJSON, generatedAt)
		if err != nil {
			level.Error(logger).Log("msg", "failed to create explain plan output", "err", err)
			continue
		}

		explainPlanOutputJSON, err := json.Marshal(explainPlanOutput)
		if err != nil {
			level.Error(logger).Log("msg", "failed to marshal explain plan output", "err", err)
			continue
		}

		logMessage := fmt.Sprintf(
			`schema="%s" digest="%s" explain_plan_output="%s"`,
			loggedSchemaName,
			qi.digest,
			base64.StdEncoding.EncodeToString(explainPlanOutputJSON),
		)

		c.entryHandler.Chan() <- buildLokiEntryWithTimestamp(
			logging.LevelInfo,
			OP_EXPLAIN_PLAN_OUTPUT,
			c.instanceKey,
			logMessage,
			int64(time.Now().UnixNano()),
		)
		// TODO: Add context to logging when errors occur so the original node can be found.
		// I.E. query_block->nested_loop[1]->table etc..
	}

	return nil
}

func RedactAttachedConditions(explainPlanJSON []byte) ([]byte, int, error) {
	parser := parser.NewTiDBSqlParser()
	attachedConditions, err := traverseJSONForAttachedConditions(explainPlanJSON)
	if err != nil {
		return make([]byte, 0), 0, fmt.Errorf("failed to traverse JSON for attached conditions: %w", err)
	}

	for _, condition := range attachedConditions {
		redacted, err := parser.Redact(string(condition))
		if err != nil {
			return make([]byte, 0), 0, fmt.Errorf("failed to redact attached condition: %s error: %w", condition, err)
		}
		explainPlanJSON = bytes.Replace(explainPlanJSON, condition, []byte(redacted), 1)
	}
	return explainPlanJSON, len(attachedConditions), nil
}

func traverseJSONForAttachedConditions(explainPlanJSON []byte) ([][]byte, error) {
	attachedConditions := make([][]byte, 0)

	if condition, _, _, err := jsonparser.Get(explainPlanJSON, "attached_condition"); err == nil {
		attachedConditions = append(attachedConditions, condition)
	}

	err := jsonparser.ObjectEach(explainPlanJSON, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		if dataType == jsonparser.Object {
			if ac, err := traverseJSONForAttachedConditions(value); err == nil {
				attachedConditions = append(attachedConditions, ac...)
			}
		}
		if dataType == jsonparser.Array {
			_, err := jsonparser.ArrayEach(value, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
				if dataType == jsonparser.Object {
					if ac, err := traverseJSONForAttachedConditions(value); err == nil {
						attachedConditions = append(attachedConditions, ac...)
					}
				}
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return attachedConditions, err
}
