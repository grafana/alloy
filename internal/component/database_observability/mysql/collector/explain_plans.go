package collector

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DataDog/go-sqllexer"
	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/buger/jsonparser"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	ExplainPlansCollector  = "explain_plans"
	OP_EXPLAIN_PLAN_OUTPUT = "explain_plan_output"
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
	AND SCHEMA_NAME NOT IN %s`

const selectExplainPlanPrefix = `EXPLAIN FORMAT=JSON `

func newExplainPlansOutput(logger log.Logger, explainJson []byte) (*database_observability.ExplainPlanNode, error) {
	qblock, _, _, err := jsonparser.Get(explainJson, "query_block")
	if err != nil {
		return nil, fmt.Errorf("failed to get query block: %w", err)
	}

	planNode, err := parseTopLevelPlanNode(logger, qblock)
	return &planNode, err
}

func parseTopLevelPlanNode(logger log.Logger, topLevelPlanNode []byte) (database_observability.ExplainPlanNode, error) {
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

	return database_observability.ExplainPlanNode{
		Operation: database_observability.ExplainPlanOutputOperationUnknown,
	}, nil
}

func parseTableNode(logger log.Logger, tableNode []byte) (database_observability.ExplainPlanNode, error) {
	pnode := database_observability.ExplainPlanNode{
		Operation: database_observability.ExplainPlanOutputOperationTableScan,
		Details:   database_observability.ExplainPlanNodeDetails{},
	}

	// Check for join algorithm. Nested loop would be set in parseNestedLoopJoinNode, since not all table nodes are children of nested loop joins.
	if joinAlgorithm, _, _, err := jsonparser.Get(tableNode, "using_join_buffer"); err == nil {
		if string(joinAlgorithm) == "hash join" {
			joinAlgorithmConst := database_observability.ExplainPlanJoinAlgorithmHash
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
	accessTypeconst := database_observability.ExplainPlanAccessType(strings.ToLower(accessType))
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

func parseNestedLoopJoinNode(logger log.Logger, nestedLoopJoinNode []byte) (database_observability.ExplainPlanNode, error) {
	algo := database_observability.ExplainPlanJoinAlgorithmNestedLoop
	pnode := database_observability.ExplainPlanNode{
		Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
		Details: database_observability.ExplainPlanNodeDetails{
			JoinAlgorithm: &algo,
		},
		Children: make([]database_observability.ExplainPlanNode, 0),
	}
	var previousChild *database_observability.ExplainPlanNode
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
			thisLoop := database_observability.ExplainPlanNode{
				Operation: database_observability.ExplainPlanOutputOperationNestedLoopJoin,
				Details: database_observability.ExplainPlanNodeDetails{
					JoinAlgorithm: &algo,
				},
			}
			if childDetails.Details.JoinAlgorithm != nil && *childDetails.Details.JoinAlgorithm != algo {
				thisLoop.Details.JoinAlgorithm = childDetails.Details.JoinAlgorithm
				switch *childDetails.Details.JoinAlgorithm {
				case database_observability.ExplainPlanJoinAlgorithmHash:
					thisLoop.Operation = database_observability.ExplainPlanOutputOperationHashJoin
				case database_observability.ExplainPlanJoinAlgorithmMerge:
					thisLoop.Operation = database_observability.ExplainPlanOutputOperationMergeJoin
				default:
					thisLoop.Operation = database_observability.ExplainPlanOutputOperationNestedLoopJoin
				}
				// Remove join algorithm from child details since we've set it in the parent
				childDetails.Details.JoinAlgorithm = nil
			}

			thisLoop.Children = []database_observability.ExplainPlanNode{
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
		if previousChild.Operation != database_observability.ExplainPlanOutputOperationNestedLoopJoin && previousChild.Operation != database_observability.ExplainPlanOutputOperationHashJoin {
			pnode.Children = append(pnode.Children, *previousChild)
		} else {
			return *previousChild, nil
		}
	}
	return pnode, nil
}

func parseGroupingOperationNode(logger log.Logger, groupingOperationNode []byte) (database_observability.ExplainPlanNode, error) {
	pnode := database_observability.ExplainPlanNode{
		Operation: database_observability.ExplainPlanOutputOperationGroupingOperation,
	}

	children, err := parseTopLevelPlanNode(logger, groupingOperationNode)
	if err != nil {
		return pnode, err
	}
	pnode.Children = append(pnode.Children, children)

	return pnode, nil
}

func parseOrderingOperationNode(logger log.Logger, orderingOperationNode []byte) (database_observability.ExplainPlanNode, error) {
	pnode := database_observability.ExplainPlanNode{
		Operation: database_observability.ExplainPlanOutputOperationOrderingOperation,
	}

	children, err := parseTopLevelPlanNode(logger, orderingOperationNode)
	if err != nil {
		return pnode, err
	}
	pnode.Children = append(pnode.Children, children)

	return pnode, nil
}

func parseDuplicatesRemovalNode(logger log.Logger, duplicatesRemovalNode []byte) (database_observability.ExplainPlanNode, error) {
	pnode := database_observability.ExplainPlanNode{
		Operation: database_observability.ExplainPlanOutputOperationDuplicatesRemoval,
	}

	children, err := parseTopLevelPlanNode(logger, duplicatesRemovalNode)
	if err != nil {
		return pnode, err
	}
	pnode.Children = append(pnode.Children, children)

	return pnode, nil
}

func parseMaterializedSubqueryNode(logger log.Logger, materializedSubqueryNode []byte) (database_observability.ExplainPlanNode, error) {
	pnode := database_observability.ExplainPlanNode{
		Operation: database_observability.ExplainPlanOutputOperationMaterializedSubquery,
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

func parseAttachedSubqueryNode(logger log.Logger, attachedSubqueryNode []byte) (database_observability.ExplainPlanNode, error) {
	pnode := database_observability.ExplainPlanNode{
		Operation: database_observability.ExplainPlanOutputOperationAttachedSubquery,
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

func parseUnionResultNode(logger log.Logger, unionResultNode []byte) (database_observability.ExplainPlanNode, error) {
	pnode := database_observability.ExplainPlanNode{
		Operation: database_observability.ExplainPlanOutputOperationUnion,
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
	schemaName   string
	digest       string
	queryText    string
	failureCount int
	uniqueKey    string
}

func newQueryInfo(schemaName, digest, queryText string) *queryInfo {
	return &queryInfo{
		schemaName: schemaName,
		digest:     digest,
		queryText:  queryText,
		uniqueKey:  schemaName + digest,
	}
}

type knownSQLCodes string

const (
	accessDeniedSQLCode knownSQLCodes = "1044"
)

var unrecoverableSQLCodes = []knownSQLCodes{
	accessDeniedSQLCode,
}

type ExplainPlansArguments struct {
	DB              *sql.DB
	ScrapeInterval  time.Duration
	PerScrapeRatio  float64
	InitialLookback time.Time
	ExcludeSchemas  []string
	EntryHandler    loki.EntryHandler
	DBVersion       string

	Logger log.Logger
}

type ExplainPlans struct {
	dbConnection     *sql.DB
	dbVersion        string
	scrapeInterval   time.Duration
	queryCache       map[string]*queryInfo
	queryDenylist    map[string]*queryInfo
	excludeSchemas   []string
	perScrapeRatio   float64
	currentBatchSize int
	entryHandler     loki.EntryHandler
	lastSeen         time.Time
	logger           log.Logger
	running          *atomic.Bool
	ctx              context.Context
	cancel           context.CancelFunc
}

func NewExplainPlans(args ExplainPlansArguments) (*ExplainPlans, error) {
	return &ExplainPlans{
		dbConnection:   args.DB,
		dbVersion:      args.DBVersion,
		scrapeInterval: args.ScrapeInterval,
		perScrapeRatio: args.PerScrapeRatio,
		excludeSchemas: args.ExcludeSchemas,
		lastSeen:       args.InitialLookback,
		queryCache:     make(map[string]*queryInfo),
		queryDenylist:  make(map[string]*queryInfo),
		entryHandler:   args.EntryHandler,
		logger:         log.With(args.Logger, "collector", ExplainPlansCollector),
		running:        atomic.NewBool(false),
	}, nil
}

func (c *ExplainPlans) sendExplainPlansOutput(schemaName string, digest string, generatedAt string, result database_observability.ExplainProcessingResult, reason string, plan *database_observability.ExplainPlanNode) error {
	output := &database_observability.ExplainPlanOutput{
		Metadata: database_observability.ExplainPlanMetadataInfo{
			DatabaseEngine:         "MySQL",
			DatabaseVersion:        c.dbVersion,
			QueryIdentifier:        digest,
			GeneratedAt:            generatedAt,
			ProcessingResult:       result,
			ProcessingResultReason: reason,
		},
	}
	if plan != nil {
		output.Plan = *plan
	}

	explainPlanOutputJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal explain plan output: %w", err)
	}

	logMessage := fmt.Sprintf(
		`schema="%s" digest="%s" explain_plan_output="%s"`,
		schemaName,
		digest,
		base64.StdEncoding.EncodeToString(explainPlanOutputJSON),
	)

	c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
		logging.LevelInfo,
		OP_EXPLAIN_PLAN_OUTPUT,
		logMessage,
	)

	return nil
}

func (c *ExplainPlans) Name() string {
	return ExplainPlansCollector
}

func (c *ExplainPlans) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "collector started")

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

func (c *ExplainPlans) Stopped() bool {
	return !c.running.Load()
}

func (c *ExplainPlans) Stop() {
	c.cancel()
}

func (c *ExplainPlans) populateQueryCache(ctx context.Context) error {
	query := fmt.Sprintf(selectDigestsForExplainPlan, buildExcludedSchemasClause(c.excludeSchemas))
	rs, err := c.dbConnection.QueryContext(ctx, query, c.lastSeen)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to fetch digests for explain plans", "err", err)
		return err
	}
	defer rs.Close()

	// Populate cache
	for rs.Next() {
		generatedAt := time.Now().Format(time.RFC3339)
		var schemaName, digest, queryText string
		var ls time.Time
		if err = rs.Scan(&schemaName, &digest, &queryText, &ls); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan digest for explain plans", "err", err)
			return err
		}

		qi := newQueryInfo(schemaName, digest, queryText)
		if _, ok := c.queryDenylist[qi.uniqueKey]; !ok {
			c.queryCache[qi.uniqueKey] = qi
		} else {
			err := c.sendExplainPlansOutput(
				schemaName,
				digest,
				generatedAt,
				database_observability.ExplainProcessingResultSkipped,
				"query denylisted",
				nil,
			)
			if err != nil {
				level.Error(c.logger).Log("msg", "failed to send denylisted query skip explain plan output", "err", err)
			}
			continue
		}
		if ls.After(c.lastSeen) {
			c.lastSeen = ls
		}
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate digest rows for explain plans", "err", err)
		return err
	}

	// Calculate batch size based on current cache size
	c.currentBatchSize = int(math.Ceil(float64(len(c.queryCache)) * c.perScrapeRatio))
	level.Debug(c.logger).Log("msg", "populated query cache", "count", len(c.queryCache), "batch_size", c.currentBatchSize)
	return nil
}

func (c *ExplainPlans) fetchExplainPlans(ctx context.Context) error {
	if len(c.queryCache) == 0 {
		if err := c.populateQueryCache(ctx); err != nil {
			return err
		}
	}

	processedCount := 0
	for _, qi := range c.queryCache {
		generatedAt := time.Now().Format(time.RFC3339)
		nonRecoverableFailureOccurred := false
		if processedCount >= c.currentBatchSize {
			break
		}
		logger := log.With(c.logger, "digest", qi.digest)

		defer func(nonRecoverableFailureOccurred *bool) {
			if *nonRecoverableFailureOccurred {
				qi.failureCount++
				c.queryDenylist[qi.uniqueKey] = qi
			}
			delete(c.queryCache, qi.uniqueKey)
			processedCount++
		}(&nonRecoverableFailureOccurred)

		if strings.HasSuffix(qi.queryText, "...") {
			err := c.sendExplainPlansOutput(
				qi.schemaName,
				qi.digest,
				generatedAt,
				database_observability.ExplainProcessingResultSkipped,
				"query is truncated",
				nil,
			)
			if err != nil {
				level.Error(c.logger).Log("msg", "failed to send truncated query skip explain plan output", "err", err)
			}
			continue
		}

		containsReservedWord, err := database_observability.ContainsReservedKeywords(qi.queryText, database_observability.ExplainReservedWordDenyList, sqllexer.DBMSMySQL)
		if err != nil {
			level.Error(logger).Log("msg", "failed to check for reserved keywords", "err", err)
			err := c.sendExplainPlansOutput(
				qi.schemaName,
				qi.digest,
				generatedAt,
				database_observability.ExplainProcessingResultError,
				fmt.Sprintf("failed to check for reserved keywords: %s", err.Error()),
				nil,
			)
			if err != nil {
				level.Error(c.logger).Log("msg", "failed to send reserved keyword check error explain plan output", "err", err)
			}
			continue
		}

		if containsReservedWord {
			err := c.sendExplainPlansOutput(
				qi.schemaName,
				qi.digest,
				generatedAt,
				database_observability.ExplainProcessingResultSkipped,
				"query contains reserved word",
				nil,
			)
			if err != nil {
				level.Error(c.logger).Log("msg", "failed to send reserved keyword check error explain plan output", "err", err)
			}
			continue
		}

		logger = log.With(logger, "schema_name", qi.schemaName)

		byteExplainPlanJSON, err := c.fetchExplainPlanJSON(ctx, *qi)
		if err != nil {
			level.Debug(logger).Log("msg", "failed to fetch explain plan json bytes", "err", err)
			for _, code := range unrecoverableSQLCodes {
				if strings.Contains(err.Error(), fmt.Sprintf("Error %s", code)) {
					nonRecoverableFailureOccurred = true
					break
				}
			}
			continue
		}

		if len(byteExplainPlanJSON) == 0 {
			level.Error(logger).Log("msg", "explain plan json bytes is empty")
			nonRecoverableFailureOccurred = true
			continue
		}

		if !utf8.Valid(byteExplainPlanJSON) {
			level.Error(logger).Log("msg", "explain plan json bytes is not valid UTF-8")
			nonRecoverableFailureOccurred = true
			continue
		}

		redactedByteExplainPlanJSON, _, err := redactAttachedConditions(byteExplainPlanJSON)
		if err != nil {
			level.Error(logger).Log("msg", "failed to redact explain plan json", "err", err)
			nonRecoverableFailureOccurred = true
			continue
		}

		msgBlock, msgDatType, _, err := jsonparser.Get(redactedByteExplainPlanJSON, "query_block", "message")
		if err != nil {
			// An explain plan response typically will not have a message field if it is successful.
			if msgDatType != jsonparser.NotExist {
				level.Error(logger).Log("msg", "failed to parse explain plan json", "err", err)
			}
		} else {
			if strings.Contains(string(msgBlock), "no matching row in const table") {
				level.Debug(logger).Log("msg", "explain plan query resulted in no rows, skipping", "message", string(msgBlock))
				continue
			}
		}

		level.Debug(logger).Log("msg", "db native explain plan",
			"db_native_explain_plan", base64.StdEncoding.EncodeToString(redactedByteExplainPlanJSON))

		explainPlanOutput, genErr := newExplainPlansOutput(logger, byteExplainPlanJSON)
		explainPlanOutputJSON, err := json.Marshal(explainPlanOutput)
		if err != nil {
			level.Error(logger).Log("msg", "failed to marshal explain plan output", "err", err)
			nonRecoverableFailureOccurred = true
			continue
		}

		if genErr != nil {
			level.Error(logger).Log(
				"msg", "failed to create explain plan output",
				"incomplete_explain_plan", base64.StdEncoding.EncodeToString(explainPlanOutputJSON),
				"err", genErr,
			)
			nonRecoverableFailureOccurred = true
			continue
		}

		if err := c.sendExplainPlansOutput(
			qi.schemaName,
			qi.digest,
			generatedAt,
			database_observability.ExplainProcessingResultSuccess,
			"",
			explainPlanOutput,
		); err != nil {
			level.Error(c.logger).Log("msg", "failed to send explain plan output", "err", err)
		}
	}

	return nil
}

func (c *ExplainPlans) fetchExplainPlanJSON(ctx context.Context, qi queryInfo) ([]byte, error) {
	conn, err := c.dbConnection.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer conn.Close()

	useStatement := fmt.Sprintf("USE `%s`", qi.schemaName)
	if _, err := conn.ExecContext(ctx, useStatement); err != nil {
		return nil, fmt.Errorf("failed to set schema: %w", err)
	}
	defer func() {
		// Switch to performance_schema to avoid that when the connection
		// is reused by some other collector, it uses the wrong schema.
		_, _ = conn.ExecContext(ctx, "USE `performance_schema`")
	}()

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
