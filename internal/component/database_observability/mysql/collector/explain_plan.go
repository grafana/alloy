package collector

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
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
	OP_EXPLAIN_PLAN_OUTPUT = "explain_plan_output"
	ExplainPlanName        = "explain_plan"
)

const selectDigestsForExplainPlan = `
	SELECT
		SCHEMA_NAME,
		DIGEST,
		QUERY_SAMPLE_TEXT,
		LAST_SEEN
	FROM performance_schema.events_statements_summary_by_digest
	WHERE LAST_SEEN > ?
	AND QUERY_SAMPLE_TEXT IS NOT NULL
	AND DIGEST IS NOT NULL
	AND SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'sys', 'information_schema')`

const selectExplainPlanPrefix = `EXPLAIN FORMAT=JSON `

const selectDBSchemaVersion = `SELECT VERSION()`

type explainPlanOutputOperation string

const (
	explainPlanOutputOperationTableScan            explainPlanOutputOperation = "Table Scan"
	explainPlanOutputOperationIndexScan            explainPlanOutputOperation = "Index Scan"
	explainPlanOutputOperationNestedLoopJoin       explainPlanOutputOperation = "Nested Loop Join"
	explainPlanOutputOperationHashJoin             explainPlanOutputOperation = "Hash Join"
	explainPlanOutputOperationMergeJoin            explainPlanOutputOperation = "Merge Join"
	explainPlanOutputOperationGroupingOperation    explainPlanOutputOperation = "Grouping Operation"
	explainPlanOutputOperationOrderingOperation    explainPlanOutputOperation = "Ordering Operation"
	explainPlanOutputOperationDuplicatesRemoval    explainPlanOutputOperation = "Duplicates Removal"
	explainPlanOutputOperationMaterializedSubquery explainPlanOutputOperation = "Materialized Subquery"
	explainPlanOutputOperationAttachedSubquery     explainPlanOutputOperation = "Attached Subquery"
	explainPlanOutputOperationUnion                explainPlanOutputOperation = "Union"
	explainPlanOutputOperationUnknown              explainPlanOutputOperation = "Unknown"
)

type explainPlanAccessType string

const (
	explainPlanAccessTypeAll   explainPlanAccessType = "all"
	explainPlanAccessTypeIndex explainPlanAccessType = "index"
	explainPlanAccessTypeRange explainPlanAccessType = "range"
	explainPlanAccessTypeRef   explainPlanAccessType = "ref"
	explainPlanAccessTypeEqRef explainPlanAccessType = "eq_ref"
)

type explainPlanJoinAlgorithm string

const (
	explainPlanJoinAlgorithmHash       explainPlanJoinAlgorithm = "hash"
	explainPlanJoinAlgorithmMerge      explainPlanJoinAlgorithm = "merge"
	explainPlanJoinAlgorithmNestedLoop explainPlanJoinAlgorithm = "nested_loop"
)

type explainPlanOutput struct {
	Metadata metadataInfo `json:"metadata"`
	Plan     planNode     `json:"plan"`
}

type metadataInfo struct {
	DatabaseEngine  string `json:"databaseEngine"`
	DatabaseVersion string `json:"databaseVersion"`
	QueryIdentifier string `json:"queryIdentifier"`
	GeneratedAt     string `json:"generatedAt"`
}

type planNode struct {
	Operation explainPlanOutputOperation `json:"operation"`
	Details   nodeDetails                `json:"details"`
	Children  []planNode                 `json:"children,omitempty"`
}

type nodeDetails struct {
	EstimatedRows int64                     `json:"estimatedRows"`
	EstimatedCost *float64                  `json:"estimatedCost,omitempty"`
	TableName     *string                   `json:"tableName,omitempty"`
	Alias         *string                   `json:"alias,omitempty"`
	AccessType    *explainPlanAccessType    `json:"accessType,omitempty"`
	KeyUsed       *string                   `json:"keyUsed,omitempty"`
	JoinType      *string                   `json:"joinType,omitempty"`
	JoinAlgorithm *explainPlanJoinAlgorithm `json:"joinAlgorithm,omitempty"`
	Condition     *string                   `json:"condition,omitempty"`
	GroupByKeys   []string                  `json:"groupByKeys,omitempty"`
	SortKeys      []string                  `json:"sortKeys,omitempty"`
	Warning       *string                   `json:"warning,omitempty"`
}

func newExplainPlanOutput(logger log.Logger, dbVersion string, digest string, explainJson []byte, generatedAt string) (*explainPlanOutput, error) {
	output := &explainPlanOutput{
		Metadata: metadataInfo{
			DatabaseEngine:  "MySQL",
			DatabaseVersion: dbVersion,
			QueryIdentifier: digest,
			GeneratedAt:     generatedAt,
		},
	}

	qblock, _, _, err := jsonparser.Get(explainJson, "query_block")
	if err != nil {
		return output, fmt.Errorf("failed to get query block: %w", err)
	}

	planNode, err := parseTopLevelPlanNode(logger, qblock)
	output.Plan = planNode
	if err != nil {
		return output, err
	}

	return output, nil
}

func parseTopLevelPlanNode(logger log.Logger, topLevelPlanNode []byte) (planNode, error) {
	if table, _, _, err := jsonparser.Get(topLevelPlanNode, "table"); err == nil {
		tableDetails, err := parseTableNode(logger, table)
		if err != nil {
			return tableDetails, err
		}

		return tableDetails, nil
	} // else we don't find the table at the top level. This is likely not an error because the NodeDetails may be for a different type of operation.
	// We may have to check to see if no operation has been set after checking all known operation types.

	if nestedLoopJoin, _, _, err := jsonparser.Get(topLevelPlanNode, "nested_loop"); err == nil {
		pnode, err := parseNestedLoopJoinNode(logger, nestedLoopJoin)
		if err != nil {
			return pnode, err
		}
		return pnode, nil
	}

	if groupingOperation, _, _, err := jsonparser.Get(topLevelPlanNode, "grouping_operation"); err == nil {
		pnode, err := parseGroupingOperationNode(logger, groupingOperation)
		if err != nil {
			return pnode, err
		}
		return pnode, nil
	}

	if orderingOperation, _, _, err := jsonparser.Get(topLevelPlanNode, "ordering_operation"); err == nil {
		pnode, err := parseOrderingOperationNode(logger, orderingOperation)
		if err != nil {
			return pnode, err
		}
		return pnode, nil
	}

	if duplicatesRemoval, _, _, err := jsonparser.Get(topLevelPlanNode, "duplicates_removal"); err == nil {
		pnode, err := parseDuplicatesRemovalNode(logger, duplicatesRemoval)
		if err != nil {
			return pnode, err
		}
		return pnode, nil
	}

	if unionResult, _, _, err := jsonparser.Get(topLevelPlanNode, "union_result"); err == nil {
		pnode, err := parseUnionResultNode(logger, unionResult)
		if err != nil {
			return pnode, err
		}
		return pnode, nil
	}

	return planNode{
		Operation: explainPlanOutputOperationUnknown,
	}, nil
}

func parseTableNode(logger log.Logger, tableNode []byte) (planNode, error) {
	pnode := planNode{
		Operation: explainPlanOutputOperationTableScan,
		Details:   nodeDetails{},
	}

	// Check for join algorithm. Nested loop would be set in parseNestedLoopJoinNode, since not all table nodes are children of nested loop joins.
	if joinAlgorithm, _, _, err := jsonparser.Get(tableNode, "using_join_buffer"); err == nil {
		if string(joinAlgorithm) == "hash join" {
			joinAlgorithmConst := explainPlanJoinAlgorithmHash
			pnode.Details.JoinAlgorithm = &joinAlgorithmConst
		}
	}

	tableAlias, err := jsonparser.GetString(tableNode, "table_name")
	// TODO: This should output some of the context in the original explain plan JSON so that errors in nested objects can be found.
	if err != nil {
		return pnode, fmt.Errorf("failed to get table alias: %w", err)
	}
	pnode.Details.Alias = &tableAlias

	accessType, err := jsonparser.GetString(tableNode, "access_type")
	if err != nil {
		return pnode, fmt.Errorf("failed to get access type: %w", err)
	}
	accessTypeconst := explainPlanAccessType(strings.ToLower(accessType))
	pnode.Details.AccessType = &accessTypeconst

	// Until now, the properties being parsed were probably mandatory, now let's look for ones that are optional.
	estimatedRows, err := jsonparser.GetInt(tableNode, "rows_produced_per_join")
	if err == nil {
		pnode.Details.EstimatedRows = estimatedRows
	}

	estimatedCost, err := jsonparser.GetString(tableNode, "cost_info", "prefix_cost")
	if err == nil {
		estimatedCostFloat, err := strconv.ParseFloat(estimatedCost, 64)
		if err != nil {
			return pnode, fmt.Errorf("failed to parse estimated cost as float: %w", err)
		}
		pnode.Details.EstimatedCost = &estimatedCostFloat
	}

	attachedCondition, _, _, err := jsonparser.Get(tableNode, "attached_condition")
	if err == nil {
		parser := parser.NewTiDBSqlParser()
		redactedAttachedCondition, err := parser.Redact(string(attachedCondition))
		if err != nil {
			return pnode, fmt.Errorf("failed to redact attached condition: %s error: %w", string(attachedCondition), err)
		}
		pnode.Details.Condition = &redactedAttachedCondition
	}

	keyUsed, err := jsonparser.GetString(tableNode, "key")
	if err == nil {
		pnode.Details.KeyUsed = &keyUsed
	}

	if materializedSubquery, _, _, err := jsonparser.Get(tableNode, "materialized_from_subquery"); err == nil {
		childNode, err := parseMaterializedSubqueryNode(logger, materializedSubquery)
		if err != nil {
			return pnode, err
		}
		pnode.Children = append(pnode.Children, childNode)
	}

	if attachedSubquery, _, _, err := jsonparser.Get(tableNode, "attached_subqueries"); err == nil {
		_, err := jsonparser.ArrayEach(attachedSubquery, func(value []byte, dataType jsonparser.ValueType, offset int, inerr error) {
			childNode, err := parseAttachedSubqueryNode(logger, value)
			if err != nil {
				return
			}
			pnode.Children = append(pnode.Children, childNode)
		})
		if err != nil {
			return pnode, err
		}
	}

	return pnode, nil
}

func parseNestedLoopJoinNode(logger log.Logger, nestedLoopJoinNode []byte) (planNode, error) {
	algo := explainPlanJoinAlgorithmNestedLoop
	pnode := planNode{
		Operation: explainPlanOutputOperationNestedLoopJoin,
		Details: nodeDetails{
			JoinAlgorithm: &algo,
		},
		Children: make([]planNode, 0),
	}
	var previousChild *planNode
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
		if previousChild != nil {
			thisLoop := planNode{
				Operation: explainPlanOutputOperationNestedLoopJoin,
				Details: nodeDetails{
					JoinAlgorithm: &algo,
				},
			}
			if childDetails.Details.JoinAlgorithm != nil && *childDetails.Details.JoinAlgorithm != algo {
				thisLoop.Details.JoinAlgorithm = childDetails.Details.JoinAlgorithm
				switch *childDetails.Details.JoinAlgorithm {
				case explainPlanJoinAlgorithmHash:
					thisLoop.Operation = explainPlanOutputOperationHashJoin
				case explainPlanJoinAlgorithmMerge:
					thisLoop.Operation = explainPlanOutputOperationMergeJoin
				default:
					thisLoop.Operation = explainPlanOutputOperationNestedLoopJoin
				}
				// Remove join algorithm from child details since we've set it in the parent
				childDetails.Details.JoinAlgorithm = nil
			}

			thisLoop.Children = []planNode{
				*previousChild,
				childDetails,
			}
			previousChild = &thisLoop
		} else {
			previousChild = &childDetails
		}
	})
	if err != nil {
		return pnode, err
	}
	if previousChild != nil {
		if previousChild.Operation != explainPlanOutputOperationNestedLoopJoin && previousChild.Operation != explainPlanOutputOperationHashJoin {
			pnode.Children = append(pnode.Children, *previousChild)
		} else {
			return *previousChild, nil
		}
	}
	return pnode, nil
}

func parseGroupingOperationNode(logger log.Logger, groupingOperationNode []byte) (planNode, error) {
	pnode := planNode{
		Operation: explainPlanOutputOperationGroupingOperation,
	}

	children, err := parseTopLevelPlanNode(logger, groupingOperationNode)
	if err != nil {
		return pnode, err
	}
	pnode.Children = append(pnode.Children, children)

	return pnode, nil
}

func parseOrderingOperationNode(logger log.Logger, orderingOperationNode []byte) (planNode, error) {
	pnode := planNode{
		Operation: explainPlanOutputOperationOrderingOperation,
	}

	children, err := parseTopLevelPlanNode(logger, orderingOperationNode)
	if err != nil {
		return pnode, err
	}
	pnode.Children = append(pnode.Children, children)

	return pnode, nil
}

func parseDuplicatesRemovalNode(logger log.Logger, duplicatesRemovalNode []byte) (planNode, error) {
	pnode := planNode{
		Operation: explainPlanOutputOperationDuplicatesRemoval,
	}

	children, err := parseTopLevelPlanNode(logger, duplicatesRemovalNode)
	if err != nil {
		return pnode, err
	}
	pnode.Children = append(pnode.Children, children)

	return pnode, nil
}

func parseMaterializedSubqueryNode(logger log.Logger, materializedSubqueryNode []byte) (planNode, error) {
	pnode := planNode{
		Operation: explainPlanOutputOperationMaterializedSubquery,
	}

	queryBlock, _, _, err := jsonparser.Get(materializedSubqueryNode, "query_block")
	if err != nil {
		return pnode, fmt.Errorf("failed to get query block: %w", err)
	}

	childNode, err := parseTopLevelPlanNode(logger, queryBlock)
	if err != nil {
		return pnode, fmt.Errorf("failed to parse top level plan node: %w", err)
	}
	pnode.Children = append(pnode.Children, childNode)

	return pnode, nil
}

func parseAttachedSubqueryNode(logger log.Logger, attachedSubqueryNode []byte) (planNode, error) {
	pnode := planNode{
		Operation: explainPlanOutputOperationAttachedSubquery,
	}

	queryBlock, _, _, err := jsonparser.Get(attachedSubqueryNode, "query_block")
	if err != nil {
		return pnode, fmt.Errorf("failed to get query block: %w", err)
	}

	childNode, err := parseTopLevelPlanNode(logger, queryBlock)
	if err != nil {
		return pnode, err
	}
	pnode.Children = append(pnode.Children, childNode)

	return pnode, nil
}

func parseUnionResultNode(logger log.Logger, unionResultNode []byte) (planNode, error) {
	pnode := planNode{
		Operation: explainPlanOutputOperationUnion,
	}

	querySpecifications, _, _, err := jsonparser.Get(unionResultNode, "query_specifications")
	if err != nil {
		return pnode, fmt.Errorf("failed to get query specifications: %w", err)
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
		pnode.Children = append(pnode.Children, childNode)
	})
	if err != nil {
		return pnode, err
	}
	return pnode, nil
}

type queryInfo struct {
	schemaName *string
	digest     string
	queryText  string
}

type ExplainPlanArguments struct {
	DB              *sql.DB
	InstanceKey     string
	ScrapeInterval  time.Duration
	PerScrapeRatio  float64
	EntryHandler    loki.EntryHandler
	InitialLookback time.Time

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
	lastSeen         time.Time

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewExplainPlan(args ExplainPlanArguments) (*ExplainPlan, error) {
	rs := args.DB.QueryRowContext(context.Background(), selectDBSchemaVersion)
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
		lastSeen:       args.InitialLookback,
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

func (c *ExplainPlan) populateQueryCache(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectDigestsForExplainPlan, c.lastSeen)
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
		var ls time.Time
		if err = rs.Scan(&qi.schemaName, &qi.digest, &qi.queryText, &ls); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan digest for explain plans", "err", err)
			return err
		}
		c.queryCache = append(c.queryCache, qi)
		if ls.After(c.lastSeen) {
			c.lastSeen = ls
		}
	}
	// Calculate batch size based on current cache size
	c.currentBatchSize = int(math.Ceil(float64(len(c.queryCache)) * c.perScrapeRatio))
	level.Info(c.logger).Log("msg", "fetched digests", "count", len(c.queryCache), "batch_size", c.currentBatchSize)
	return nil
}

func (c *ExplainPlan) fetchExplainPlans(ctx context.Context) error {
	if len(c.queryCache) == 0 {
		if err := c.populateQueryCache(ctx); err != nil {
			return err
		}
	}

	processedCount := 0
	for i, qi := range c.queryCache {
		if processedCount >= c.currentBatchSize {
			break
		}
		logger := log.With(c.logger, "digest", qi.digest)

		defer func(index int) {
			c.queryCache = slices.Delete(c.queryCache, index, index+1)
			processedCount++
		}(i)

		if strings.HasSuffix(qi.queryText, "...") {
			level.Debug(logger).Log("msg", "skipping truncated query")
			continue
		}

		if !strings.HasPrefix(strings.ToLower(qi.queryText), "select") {
			continue
		}

		if qi.schemaName == nil {
			continue
		}
		logger = log.With(logger, "schema_name", *qi.schemaName)

		byteExplainPlanJSON, err := c.fetchExplainPlanJSON(ctx, qi)
		if err != nil {
			level.Error(logger).Log("msg", "failed to fetch explain plan json bytes", "err", err)
			continue
		}

		if len(byteExplainPlanJSON) == 0 {
			level.Error(logger).Log("msg", "explain plan json bytes is empty")
			continue
		}

		if !utf8.Valid(byteExplainPlanJSON) {
			level.Error(logger).Log("msg", "explain plan json bytes is not valid UTF-8")
			continue
		}

		redactedByteExplainPlanJSON, _, err := redactAttachedConditions(byteExplainPlanJSON)
		if err != nil {
			level.Error(logger).Log("msg", "failed to redact explain plan json", "err", err)
			continue
		}

		level.Debug(logger).Log("msg", "db native explain plan",
			"digest", qi.digest,
			"db_native_explain_plan", base64.StdEncoding.EncodeToString(redactedByteExplainPlanJSON))

		generatedAt := time.Now().Format(time.RFC3339)

		explainPlanOutput, genErr := newExplainPlanOutput(logger, c.dbVersion, qi.digest, byteExplainPlanJSON, generatedAt)
		explainPlanOutputJSON, err := json.Marshal(explainPlanOutput)
		if err != nil {
			level.Error(logger).Log("msg", "failed to marshal explain plan output", "err", err)
			continue
		}

		if genErr != nil {
			level.Error(logger).Log(
				"msg", "failed to create explain plan output",
				"incomplete_explain_plan", base64.StdEncoding.EncodeToString(explainPlanOutputJSON),
				"err", genErr,
			)
			continue
		}

		logMessage := fmt.Sprintf(
			`schema="%s" digest="%s" explain_plan_output="%s"`,
			*qi.schemaName,
			qi.digest,
			base64.StdEncoding.EncodeToString(explainPlanOutputJSON),
		)

		c.entryHandler.Chan() <- buildLokiEntry(
			logging.LevelInfo,
			OP_EXPLAIN_PLAN_OUTPUT,
			c.instanceKey,
			logMessage,
		)
		// TODO: Add context to logging when errors occur so the original node can be found.
		// I.E. query_block->nested_loop[1]->table etc..
	}

	return nil
}

func (c *ExplainPlan) fetchExplainPlanJSON(ctx context.Context, qi queryInfo) ([]byte, error) {
	conn, err := c.dbConnection.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer conn.Close()

	useStatement := fmt.Sprintf("USE `%s`", *qi.schemaName)
	if _, err := conn.ExecContext(ctx, useStatement); err != nil {
		return nil, fmt.Errorf("failed to set schema: %w", err)
	}

	rsExplain := conn.QueryRowContext(ctx, selectExplainPlanPrefix+qi.queryText)
	if err := rsExplain.Err(); err != nil {
		return nil, fmt.Errorf("failed to run explain plan: %w", err)
	}

	var byteExplainPlanJSON []byte
	if err := rsExplain.Scan(&byteExplainPlanJSON); err != nil {
		return nil, fmt.Errorf("failed to scan explain plan json: %w", err)
	}

	return byteExplainPlanJSON, nil
}

func redactAttachedConditions(explainPlanJSON []byte) ([]byte, int, error) {
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
