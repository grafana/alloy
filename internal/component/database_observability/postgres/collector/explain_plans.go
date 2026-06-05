package collector

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/DataDog/go-sqllexer"
	"github.com/blang/semver/v4"
	"github.com/lib/pq"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	ExplainPlanCollector   = "explain_plans"
	OP_EXPLAIN_PLAN_OUTPUT = "explain_plan_output"
)

const selectQueriesForExplainPlanTemplate = `
	SELECT
		d.datname,
		s.queryid,
		s.query,
		s.calls,
		%s
	FROM pg_stat_statements s
		JOIN pg_database d ON s.dbid = d.oid AND NOT d.datistemplate AND d.datallowconn
	WHERE s.queryid IS NOT NULL AND s.query IS NOT NULL
		AND d.datname NOT IN %s
		%s
		%s`

const selectExplainPlanPrefix = `EXPLAIN (FORMAT JSON) EXECUTE `

// unrecoverablePostgresSQLErrors are substrings matched against err.Error() to
// catch driver-level failures (e.g. pg_hba rejection) that do not surface as a
// SQLSTATE on a *pq.Error.
var unrecoverablePostgresSQLErrors = []string{
	"pq: permission denied",
	"pq: pg_hba.conf rejects connection for host",
	"pq: syntax error",
}

// unrecoverablePostgresSQLStateCodes are SQLSTATE codes that indicate an
// extension-internal query cannot be EXPLAIN'd in its current shape, typically
// because recorded pg_stat_statements text references identifiers or types that
// don't resolve outside the original execution context. For ordinary application
// queries, these SQLSTATEs are left retryable.
var unrecoverablePostgresSQLStateCodes = map[string]struct{}{
	"42703": {}, // undefined_column          — e.g. TimescaleDB `htid` leakage
	"42P01": {}, // undefined_table
	"42804": {}, // datatype_mismatch         — e.g. UNION branch type mismatch
	"42P18": {}, // indeterminate_datatype
	"42883": {}, // undefined_function
	"42501": {}, // insufficient_privilege
	"42601": {}, // syntax_error
}

// isUnrecoverableExplainError reports whether the error from PREPARE/EXPLAIN
// indicates a permanent inability to plan this query, in which case the
// collector should denylist the queryid rather than retry it on every cycle.
func isUnrecoverableExplainError(err error, queryText string, detectedExtensions DetectedExtensions, skipExtensionInternals bool) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, code := range unrecoverablePostgresSQLErrors {
		if strings.Contains(msg, code) {
			return true
		}
	}
	if !skipExtensionInternals || !detectedExtensions.TimescaleDB || !queryReferencesTimescaleDBInternalSchema(queryText) {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		if _, ok := unrecoverablePostgresSQLStateCodes[string(pqErr.Code)]; ok {
			return true
		}
	}
	return false
}

func queryReferencesTimescaleDBInternalSchema(queryText string) bool {
	lexer := sqllexer.New(queryText, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	for {
		token := lexer.Scan()
		switch token.Type {
		case sqllexer.ERROR:
			return timescaleDBInternalSchemaFallbackRegex.MatchString(queryText)
		case sqllexer.EOF:
			return false
		case sqllexer.IDENT, sqllexer.QUOTED_IDENT, sqllexer.FUNCTION:
			identifier := strings.ToLower(strings.ReplaceAll(token.Value, `"`, ""))
			for _, schema := range timescaleDBInternalSchemas {
				if strings.HasPrefix(identifier, schema+".") {
					return true
				}
			}
		}
	}
}

var (
	paramCountRegex                        = regexp.MustCompile(`\$\d+`)
	timescaleDBInternalSchemaFallbackRegex = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])"?_timescaledb_(cache|catalog|config|debug|functions|internal)"?\s*\.`)
	versSanitizeRegex                      = regexp.MustCompile(`^v?[0-9]+\.?[0-9]+`)
)

type PgSQLExplainplan struct {
	Plan PlanNode `json:"Plan"`
}

type PlanNode struct {
	NodeType           string     `json:"Node Type"`
	Alias              string     `json:"Alias"`
	RelationName       string     `json:"Relation Name"`
	ParentRelationship string     `json:"Parent Relationship"`
	PartialMode        string     `json:"Partial Mode"`
	Strategy           string     `json:"Strategy"`
	ParallelAware      bool       `json:"Parallel Aware"`
	AsyncCapable       bool       `json:"Async Capable"`
	JoinType           string     `json:"Join Type"`
	InnerUnique        bool       `json:"Inner Unique"`
	HashCond           string     `json:"Hash Cond"`
	Filter             string     `json:"Filter"`
	StartupCost        float64    `json:"Startup Cost"`
	TotalCost          float64    `json:"Total Cost"`
	PlanRows           int64      `json:"Plan Rows"`
	PlanWidth          int64      `json:"Plan Width"`
	GroupKey           []string   `json:"Group Key"`
	SortKey            []string   `json:"Sort Key"`
	WorkersPlanned     int64      `json:"Workers Planned"`
	PlannedPartitions  int64      `json:"Planned Partitions"`
	Plans              []PlanNode `json:"Plans"`
	IndexName          string     `json:"Index Name"`
}

func newExplainPlanOutput(explainJson []byte) (*database_observability.ExplainPlanNode, error) {
	var planNodes []PgSQLExplainplan
	if err := json.Unmarshal(explainJson, &planNodes); err != nil {
		return nil, err
	}

	planNode, err := planNodes[0].Plan.ToExplainPlanOutputNode()
	if err != nil {
		return nil, err
	}

	return &planNode, nil
}

func (p *PlanNode) ToExplainPlanOutputNode() (database_observability.ExplainPlanNode, error) {
	cost := p.totalCost()
	output := database_observability.ExplainPlanNode{
		Operation: p.explainPlanNodeOperation(),
		Details: database_observability.ExplainPlanNodeDetails{
			EstimatedRows: p.PlanRows,
			EstimatedCost: cost,
		},
	}

	if len(p.GroupKey) > 0 {
		output.Details.GroupByKeys = p.GroupKey
	}

	if len(p.SortKey) > 0 {
		output.Details.SortKeys = p.SortKey
	}

	if !strings.EqualFold(p.JoinType, "") {
		output.Details.JoinType = &p.JoinType
	}

	if !strings.EqualFold(p.Filter, "") {
		redacted := database_observability.RedactSql(p.Filter)
		output.Details.Condition = &redacted
	}

	if !strings.EqualFold(p.Alias, "") {
		output.Details.Alias = &p.Alias
	}

	if !strings.EqualFold(p.IndexName, "") {
		output.Details.KeyUsed = &p.IndexName
	}

	if strings.EqualFold(p.NodeType, "Hash Join") {
		algo := database_observability.ExplainPlanJoinAlgorithmHash
		output.Details.JoinAlgorithm = &algo
	}

	for _, child := range p.Plans {
		childNode, err := child.ToExplainPlanOutputNode()
		if err != nil {
			return output, err
		}
		output.Children = append(output.Children, childNode)
	}

	return output, nil
}

func (p *PlanNode) explainPlanNodeOperation() database_observability.ExplainPlanOutputOperation {
	stringbuilder := strings.Builder{}
	if !strings.EqualFold(p.PartialMode, "") {
		stringbuilder.WriteString(p.PartialMode)
		stringbuilder.WriteString(" ")
	}

	if !strings.EqualFold(p.Strategy, "") {
		switch p.Strategy {
		case "Sorted":
			stringbuilder.WriteString("Group ")
		case "Plain":
			break
		default:
			stringbuilder.WriteString(p.Strategy)
			stringbuilder.WriteString(" ")
		}
	}

	if p.ParallelAware {
		stringbuilder.WriteString("Parallel ")
	}

	stringbuilder.WriteString(p.NodeType)
	return database_observability.ExplainPlanOutputOperation(stringbuilder.String())
}

func (p *PlanNode) totalCost() *float64 {
	var result float64
	result = p.TotalCost
	for _, plan := range p.Plans {
		result -= plan.TotalCost
	}
	result = math.Round(result*100) / 100
	if result < 0 {
		result = 0
	}
	return &result
}

type queryInfo struct {
	datname      string
	queryId      string
	queryText    string
	failureCount int
	uniqueKey    string
	calls        int64
	callsReset   time.Time
}

func newQueryInfo(datname, queryId, queryText string, calls int64, callsReset time.Time) *queryInfo {
	return &queryInfo{
		datname:    datname,
		queryId:    queryId,
		queryText:  queryText,
		uniqueKey:  datname + queryId,
		calls:      calls,
		callsReset: callsReset,
	}
}

type ExplainPlansArguments struct {
	DB                     *sql.DB
	DSN                    string
	ScrapeInterval         time.Duration
	PerScrapeRatio         float64
	ExcludeDatabases       []string
	ExcludeUsers           []string
	EntryHandler           loki.EntryHandler
	DBVersion              string
	DetectedExtensions     DetectedExtensions
	SkipExtensionInternals bool

	Logger *slog.Logger
}

type ExplainPlans struct {
	dbConnection           *sql.DB
	dbDSN                  string
	dbVersion              semver.Version
	dbConnectionFactory    databaseConnectionFactory
	scrapeInterval         time.Duration
	queryCache             map[string]*queryInfo
	queryDenylist          map[string]*queryInfo
	finishedQueryCache     map[string]*queryInfo
	excludeDatabases       []string
	excludeUsers           []string
	detectedExtensions     DetectedExtensions
	skipExtensionInternals bool
	perScrapeRatio         float64
	currentBatchSize       int
	entryHandler           loki.EntryHandler
	logger                 *slog.Logger
	running                *atomic.Bool
	ctx                    context.Context
	cancel                 context.CancelFunc
	wg                     sync.WaitGroup
}

func NewExplainPlan(args ExplainPlansArguments) (*ExplainPlans, error) {
	ep := &ExplainPlans{
		dbConnection:           args.DB,
		dbDSN:                  args.DSN,
		dbConnectionFactory:    defaultDbConnectionFactory,
		scrapeInterval:         args.ScrapeInterval,
		perScrapeRatio:         args.PerScrapeRatio,
		excludeDatabases:       args.ExcludeDatabases,
		excludeUsers:           args.ExcludeUsers,
		detectedExtensions:     args.DetectedExtensions,
		skipExtensionInternals: args.SkipExtensionInternals,
		queryCache:             make(map[string]*queryInfo),
		queryDenylist:          make(map[string]*queryInfo),
		finishedQueryCache:     make(map[string]*queryInfo),
		entryHandler:           args.EntryHandler,
		logger:                 args.Logger.With("collector", ExplainPlanCollector),
		running:                atomic.NewBool(false),
	}
	// Pre-sanitize the version by removing any trailing characters before semver gets it
	foundVers := versSanitizeRegex.FindString(args.DBVersion)
	engineSemver, err := semver.ParseTolerant(foundVers)
	if err != nil {
		return ep, fmt.Errorf("failed to parse database engine version: %s: %w", args.DBVersion, err)
	}
	ep.dbVersion = engineSemver

	return ep, nil
}

func (c *ExplainPlans) sendExplainPlansOutput(schemaName string, digest string, generatedAt string, result database_observability.ExplainProcessingResult, reason string, plan *database_observability.ExplainPlanNode) error {
	output := &database_observability.ExplainPlanOutput{
		Metadata: database_observability.ExplainPlanMetadataInfo{
			DatabaseEngine:         "PostgreSQL",
			DatabaseVersion:        c.dbVersion.String(),
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
	return ExplainPlanCollector
}

func (c *ExplainPlans) Start(ctx context.Context) error {
	c.logger.Debug("collector started")

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.scrapeInterval)
		defer ticker.Stop()

		for {
			if err := c.fetchExplainPlans(c.ctx); err != nil {
				c.logger.Error("collector error", "err", err)
			}

			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	})

	return nil
}

func (c *ExplainPlans) Stopped() bool {
	return !c.running.Load()
}

func (c *ExplainPlans) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *ExplainPlans) populateQueryCache(ctx context.Context) error {
	var selectStatement string
	var resetTS time.Time
	excludedDatabasesClause := buildExcludedDatabasesClause(c.excludeDatabases)
	excludedUsersClause := buildExcludedUsersClause(c.excludeUsers, "pg_get_userbyid(s.userid)")
	extensionFilterClause := buildExtensionExplainFilterClause(c.skipExtensionInternals, c.detectedExtensions)
	version17Plus := semver.MustParseRange(">=17.0.0")(c.dbVersion)
	if version17Plus {
		selectStatement = fmt.Sprintf(selectQueriesForExplainPlanTemplate, "s.stats_since", excludedDatabasesClause, excludedUsersClause, extensionFilterClause)
	} else {
		statReset := c.dbConnection.QueryRowContext(ctx, "SELECT stats_reset FROM pg_stat_statements_info")
		if err := statReset.Err(); err != nil {
			return fmt.Errorf("failed to fetch stats reset time for explain plans: %w", err)
		}
		if err := statReset.Scan(&resetTS); err != nil {
			return fmt.Errorf("failed to scan stats reset time for explain plans: %w", err)
		}
		selectStatement = fmt.Sprintf(selectQueriesForExplainPlanTemplate, "NOW() AT TIME ZONE 'UTC' AS stats_since", excludedDatabasesClause, excludedUsersClause, extensionFilterClause)
	}

	rs, err := c.dbConnection.QueryContext(ctx, selectStatement)
	if err != nil {
		return fmt.Errorf("failed to fetch digests for explain plans: %w", err)
	}
	defer rs.Close()

	for rs.Next() {
		generatedAt := time.Now().Format(time.RFC3339)
		var datname, queryId, query string
		var calls int64
		var ls time.Time
		if err := rs.Scan(&datname, &queryId, &query, &calls, &ls); err != nil {
			return fmt.Errorf("failed to scan query for explain plan: %w", err)
		}

		statsReset := resetTS
		if version17Plus {
			statsReset = ls
		}
		qi := newQueryInfo(datname, queryId, query, calls, statsReset)
		if _, ok := c.queryDenylist[qi.uniqueKey]; !ok {
			if previous, ok := c.finishedQueryCache[qi.uniqueKey]; ok {
				if calls == previous.calls {
					continue
				}
				if calls < previous.calls && (statsReset.Equal(previous.callsReset) || statsReset.Before(previous.callsReset)) {
					continue
				}
				delete(c.finishedQueryCache, qi.uniqueKey)
			}
			c.queryCache[qi.uniqueKey] = qi
		} else {
			err := c.sendExplainPlansOutput(
				datname,
				queryId,
				generatedAt,
				database_observability.ExplainProcessingResultSkipped,
				"query denylisted",
				nil,
			)
			if err != nil {
				c.logger.Error("failed to send denylisted query skip explain plan output", "err", err)
			}
			continue
		}
	}

	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate query rows for explain plans: %w", err)
	}

	c.currentBatchSize = int(math.Ceil(float64(len(c.queryCache)) * c.perScrapeRatio))
	c.logger.Debug("populated query cache", "count", len(c.queryCache), "batch_size", c.currentBatchSize)
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
		logger := c.logger.With("query_id", qi.queryId)

		defer func(nonRecoverableFailureOccurred *bool) {
			if *nonRecoverableFailureOccurred {
				qi.failureCount++
				c.queryDenylist[qi.uniqueKey] = qi
			} else {
				c.finishedQueryCache[qi.uniqueKey] = qi
			}
			delete(c.queryCache, qi.uniqueKey)
			processedCount++
		}(&nonRecoverableFailureOccurred)

		if strings.HasSuffix(qi.queryText, "...") {
			err := c.sendExplainPlansOutput(
				qi.datname,
				qi.queryId,
				generatedAt,
				database_observability.ExplainProcessingResultSkipped,
				"query is truncated",
				nil,
			)
			if err != nil {
				c.logger.Error("failed to send truncated query skip explain plan output", "err", err)
			}
			continue
		}

		containsReservedWord, err := database_observability.ContainsReservedKeywords(qi.queryText, database_observability.ExplainReservedWordDenyList, sqllexer.DBMSPostgres)
		if err != nil {
			logger.Error("failed to check for reserved keywords", "err", err)
			err := c.sendExplainPlansOutput(
				qi.datname,
				qi.queryId,
				generatedAt,
				database_observability.ExplainProcessingResultError,
				fmt.Sprintf("failed to check for reserved keywords: %s", err.Error()),
				nil,
			)
			if err != nil {
				c.logger.Error("failed to send reserved keyword check error explain plan output", "err", err)
			}
			continue
		}

		if containsReservedWord {
			err := c.sendExplainPlansOutput(
				qi.datname,
				qi.queryId,
				generatedAt,
				database_observability.ExplainProcessingResultSkipped,
				"query contains reserved word",
				nil,
			)
			if err != nil {
				c.logger.Error("failed to send reserved keyword check error explain plan output", "err", err)
			}
			continue
		}

		logger = logger.With("datname", qi.datname)

		byteExplainPlanJSON, err := c.fetchExplainPlanJSON(ctx, *qi)
		if err != nil {
			logger.Debug("failed to fetch explain plan json bytes", "err", err)
			if isUnrecoverableExplainError(err, qi.queryText, c.detectedExtensions, c.skipExtensionInternals) {
				nonRecoverableFailureOccurred = true
			}
			continue
		}

		if len(byteExplainPlanJSON) == 0 {
			logger.Error("explain plan json bytes is empty")
			nonRecoverableFailureOccurred = true
			continue
		}

		if !utf8.Valid(byteExplainPlanJSON) {
			logger.Error("explain plan json bytes is not valid UTF-8")
			nonRecoverableFailureOccurred = true
			continue
		}

		redactedByteExplainPlanJSON := database_observability.RedactSql(string(byteExplainPlanJSON))

		logger.Debug("db native explain plan", "db_native_explain_plan", base64.StdEncoding.EncodeToString([]byte(redactedByteExplainPlanJSON)))

		explainPlanOutput, genErr := newExplainPlanOutput(byteExplainPlanJSON)
		explainPlanOutputJSON, err := json.Marshal(explainPlanOutput)
		if err != nil {
			logger.Error("failed to marshal explain plan output", "err", err)
			nonRecoverableFailureOccurred = true
			continue
		}

		if genErr != nil {
			logger.Error(
				"failed to create explain plan output",
				"incomplete_explain_plan", base64.StdEncoding.EncodeToString(explainPlanOutputJSON),
				"err", genErr,
			)
			nonRecoverableFailureOccurred = true
			continue
		}

		if err := c.sendExplainPlansOutput(
			qi.datname,
			qi.queryId,
			generatedAt,
			database_observability.ExplainProcessingResultSuccess,
			"",
			explainPlanOutput,
		); err != nil {
			c.logger.Error("failed to send explain plan output", "err", err)
		}
	}

	return nil
}

// postgresPreparedStatementParamCount returns N for EXECUTE, where N is the highest
// placeholder index in the query. The same index may appear multiple times (e.g. two $1)
// without increasing N.
func postgresPreparedStatementParamCount(queryText string) int {
	matches := paramCountRegex.FindAllString(queryText, -1)
	maxParam := 0
	for _, m := range matches {
		n, err := strconv.Atoi(m[1:])
		if err == nil && n > maxParam {
			maxParam = n
		}
	}
	return maxParam
}

func (c *ExplainPlans) fetchExplainPlanJSON(ctx context.Context, qi queryInfo) ([]byte, error) {
	querySpecificDSN, err := replaceDatabaseNameInDSN(c.dbDSN, qi.datname)
	if err != nil {
		return nil, fmt.Errorf("failed to replace database name in DSN: %w", err)
	}
	conn, err := c.dbConnectionFactory(querySpecificDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer conn.Close()

	setSearchPathStatement := fmt.Sprintf("SET SESSION search_path TO \"%s\", public", qi.datname)
	if _, err := conn.ExecContext(ctx, setSearchPathStatement); err != nil {
		return nil, fmt.Errorf("failed to set search path: %w", err)
	}

	preparedStatementName := strings.ReplaceAll(fmt.Sprintf("explain_plan_%s", qi.queryId), "-", "_")
	preparedStatementText := fmt.Sprintf("PREPARE %s AS %s", preparedStatementName, qi.queryText)
	logger := c.logger.With("query_id", qi.queryId, "datname", qi.datname, "preparedStatementName", preparedStatementName, "preparedStatementText", preparedStatementText)
	if _, err := conn.ExecContext(ctx, preparedStatementText); err != nil {
		return nil, fmt.Errorf("failed to prepare explain plan: %w", err)
	}

	defer func() {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf("DEALLOCATE %s", preparedStatementName)); err != nil {
			logger.Error("failed to deallocate explain plan", "err", err)
		}
	}()

	if _, err := conn.ExecContext(ctx, "SET plan_cache_mode = force_generic_plan"); err != nil {
		return nil, fmt.Errorf("failed to set plan cache mode: %w", err)
	}

	explainQuery := fmt.Sprintf("%s%s", selectExplainPlanPrefix, preparedStatementName)
	paramCount := postgresPreparedStatementParamCount(qi.queryText)
	if paramCount > 0 {
		nullParams := strings.Repeat("null,", paramCount)
		nullParams = nullParams[:len(nullParams)-1]

		explainQuery = fmt.Sprintf("%s%s(%s)", selectExplainPlanPrefix, preparedStatementName, nullParams)
	}
	rsExplain := conn.QueryRowContext(ctx, explainQuery)
	if err := rsExplain.Err(); err != nil {
		return nil, fmt.Errorf("failed to run explain plan: %w", err)
	}

	var byteExplainPlanJSON []byte
	if err := rsExplain.Scan(&byteExplainPlanJSON); err != nil {
		return nil, fmt.Errorf("failed to scan explain plan json: %w", err)
	}

	return byteExplainPlanJSON, nil
}
