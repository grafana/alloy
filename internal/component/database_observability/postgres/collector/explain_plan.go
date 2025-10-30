package collector

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"go.uber.org/atomic"
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
`

const selectExplainPlanPrefix = `EXPLAIN (FORMAT JSON) EXECUTE `

var unrecoverablePostgresSQLErrors = []string{
	"pq: permission denied for table",
	"pq: pg_hba.conf rejects connection for host",
}

var dsnParseRegex = regexp.MustCompile(`^(\w+:\/\/.+\/)(?<dbname>[\w\-_\$]+)(\??.*$)`)
var paramCountRegex = regexp.MustCompile(`\$\d+`)

var defaultDbConnectionFactory = func(dsn string) (*sql.DB, error) {
	return sql.Open("postgres", dsn)
}

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

func newExplainPlanOutput(dbVersion string, queryId string, explainJson []byte, generatedAt string) (*database_observability.ExplainPlanOutput, error) {
	var planNodes []PgSQLExplainplan
	if err := json.Unmarshal(explainJson, &planNodes); err != nil {
		return nil, err
	}

	planNode, err := planNodes[0].Plan.ToExplainPlanOutputNode()
	if err != nil {
		return nil, err
	}

	output := &database_observability.ExplainPlanOutput{
		Metadata: database_observability.ExplainPlanMetadataInfo{
			DatabaseEngine:  "PostgreSQL",
			DatabaseVersion: dbVersion,
			QueryIdentifier: queryId,
			GeneratedAt:     generatedAt,
		},
		Plan: planNode,
	}

	return output, nil
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

type databaseConnectionFactory func(dsn string) (*sql.DB, error)

type ExplainPlanArguments struct {
	DB              *sql.DB
	DSN             string
	ScrapeInterval  time.Duration
	PerScrapeRatio  float64
	ExcludeSchemas  []string
	EntryHandler    loki.EntryHandler
	InitialLookback time.Time
	DBVersion       semver.Version

	Logger log.Logger
}

type ExplainPlan struct {
	dbConnection        *sql.DB
	dbDSN               string
	dbVersion           semver.Version
	dbConnectionFactory databaseConnectionFactory
	scrapeInterval      time.Duration
	queryCache          map[string]*queryInfo
	queryDenylist       map[string]*queryInfo
	finishedQueryCache  map[string]*queryInfo
	excludeSchemas      []string
	perScrapeRatio      float64
	currentBatchSize    int
	entryHandler        loki.EntryHandler
	logger              log.Logger
	running             *atomic.Bool
	ctx                 context.Context
	cancel              context.CancelFunc
}

func NewExplainPlan(args ExplainPlanArguments) (*ExplainPlan, error) {
	return &ExplainPlan{
		dbConnection:        args.DB,
		dbDSN:               args.DSN,
		dbVersion:           args.DBVersion,
		dbConnectionFactory: defaultDbConnectionFactory,
		scrapeInterval:      args.ScrapeInterval,
		queryCache:          make(map[string]*queryInfo),
		queryDenylist:       make(map[string]*queryInfo),
		finishedQueryCache:  make(map[string]*queryInfo),
		excludeSchemas:      args.ExcludeSchemas,
		perScrapeRatio:      args.PerScrapeRatio,
		entryHandler:        args.EntryHandler,
		logger:              log.With(args.Logger, "collector", ExplainPlanCollector),
		running:             atomic.NewBool(false),
	}, nil
}

func (c *ExplainPlan) Name() string {
	return ExplainPlanCollector
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
	var selectStatement string
	var resetTS time.Time
	version17Plus := semver.MustParseRange(">=17.0.0")(c.dbVersion)
	if version17Plus {
		selectStatement = fmt.Sprintf(selectQueriesForExplainPlanTemplate, "s.stats_since")
	} else {
		statReset, err := c.dbConnection.QueryContext(ctx, "SELECT stats_reset FROM pg_stat_statements_info")
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to fetch stats reset time for explain plans", "err", err)
			return err
		}
		defer statReset.Close()
		if statReset.Next() {
			if err := statReset.Scan(&resetTS); err != nil {
				level.Error(c.logger).Log("msg", "failed to scan stats reset time for explain plans", "err", err)
				return err
			}
		}
		selectStatement = fmt.Sprintf(selectQueriesForExplainPlanTemplate, "NOW() AT TIME ZONE 'UTC' AS stats_since")
	}

	rs, err := c.dbConnection.QueryContext(ctx, selectStatement)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to fetch digests for explain plans", "err", err)
		return err
	}
	defer rs.Close()

	for rs.Next() {
		var datname, queryId, query string
		var calls int64
		var ls time.Time
		if err := rs.Scan(&datname, &queryId, &query, &calls, &ls); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan query for explain plan", "err", err)
			return err
		}

		if slices.ContainsFunc(c.excludeSchemas, func(schema string) bool {
			return strings.EqualFold(schema, datname)
		}) {

			continue
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
		}
	}

	c.currentBatchSize = int(math.Ceil(float64(len(c.queryCache)) * c.perScrapeRatio))
	level.Info(c.logger).Log("msg", "populated query cache", "count", len(c.queryCache), "batch_size", c.currentBatchSize)
	return nil
}

func (c *ExplainPlan) fetchExplainPlans(ctx context.Context) error {
	if len(c.queryCache) == 0 {
		if err := c.populateQueryCache(ctx); err != nil {
			return err
		}
	}

	processedCount := 0
	for _, qi := range c.queryCache {
		nonRecoverableFailureOccurred := false
		if processedCount >= c.currentBatchSize {
			break
		}
		logger := log.With(c.logger, "query_id", qi.queryId)

		defer func(nonRecoverableFailureOccurred *bool) {
			if *nonRecoverableFailureOccurred {
				qi.failureCount++
				c.queryDenylist[qi.uniqueKey] = qi
				level.Info(c.logger).Log("msg", "query denylisted", "query_id", qi.queryId)
			} else {
				c.finishedQueryCache[qi.uniqueKey] = qi
			}
			delete(c.queryCache, qi.uniqueKey)
			processedCount++
		}(&nonRecoverableFailureOccurred)

		if strings.HasSuffix(qi.queryText, "...") {
			level.Debug(logger).Log("msg", "skipping truncated query")
			continue
		}

		containsReservedWord := database_observability.ContainsReservedKeywords(qi.queryText, database_observability.ExplainReservedWordDenyList)

		if containsReservedWord {
			level.Debug(logger).Log("msg", "skipping query containing reserved word", "query", qi.queryText)
			continue
		}

		logger = log.With(logger, "datname", qi.datname)

		byteExplainPlanJSON, err := c.fetchExplainPlanJSON(ctx, *qi)
		if err != nil {
			level.Error(logger).Log("msg", "failed to fetch explain plan json bytes", "err", err)
			for _, code := range unrecoverablePostgresSQLErrors {
				if strings.Contains(err.Error(), code) {
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

		redactedByteExplainPlanJSON := database_observability.RedactSql(string(byteExplainPlanJSON))

		level.Debug(logger).Log("msg", "db native explain plan", "db_native_explain_plan", base64.StdEncoding.EncodeToString([]byte(redactedByteExplainPlanJSON)))

		generatedAt := time.Now().Format(time.RFC3339)
		explainPlanOutput, genErr := newExplainPlanOutput(c.dbVersion.String(), qi.queryId, byteExplainPlanJSON, generatedAt)
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

		logMessage := fmt.Sprintf(
			`schema="%s" digest="%s" explain_plan_output="%s"`,
			qi.datname,
			qi.queryId,
			base64.StdEncoding.EncodeToString(explainPlanOutputJSON),
		)

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_EXPLAIN_PLAN_OUTPUT,
			logMessage,
		)
	}

	return nil
}

// replaceDatabaseNameInDSN safely replaces the database name in a PostgreSQL DSN
// using regex to ensure only the database name portion is replaced, not other occurrences
func (c *ExplainPlan) replaceDatabaseNameInDSN(dsn, newDatabaseName string) (string, error) {
	// Use the same regex pattern as in NewExplainPlan to find the database name
	matches := dsnParseRegex.FindStringSubmatch(dsn)

	if len(matches) < 4 {
		return "", errors.New("failed to parse DSN for database name replacement")
	}

	// Reconstruct the DSN with the new database name
	// matches[1] = prefix (protocol://user:pass@host:port/)
	// matches[2] = original database name (captured group)
	// matches[3] = suffix (query parameters)
	newDSN := matches[1] + newDatabaseName + matches[3]
	return newDSN, nil
}

func (c *ExplainPlan) fetchExplainPlanJSON(ctx context.Context, qi queryInfo) ([]byte, error) {
	querySpecificDSN, err := c.replaceDatabaseNameInDSN(c.dbDSN, qi.datname)
	if err != nil {
		return nil, fmt.Errorf("failed to replace database name in DSN: %w", err)
	}
	conn, err := c.dbConnectionFactory(querySpecificDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer conn.Close()

	preparedStatementName := strings.ReplaceAll(fmt.Sprintf("explain_plan_%s", qi.queryId), "-", "_")
	preparedStatementText := fmt.Sprintf("PREPARE %s AS %s", preparedStatementName, qi.queryText)
	logger := log.With(c.logger, "query_id", qi.queryId, "datname", qi.datname, "preparedStatementName", preparedStatementName, "preparedStatementText", preparedStatementText)
	if _, err := conn.ExecContext(ctx, preparedStatementText); err != nil {
		return nil, fmt.Errorf("failed to prepare explain plan: %w", err)
	}

	defer func() {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf("DEALLOCATE %s", preparedStatementName)); err != nil {
			level.Error(logger).Log("msg", "failed to deallocate explain plan", "err", err)
		}
	}()

	setSearchPathStatement := fmt.Sprintf("SET search_path TO %s, public", qi.datname)
	if _, err := conn.ExecContext(ctx, setSearchPathStatement); err != nil {
		return nil, fmt.Errorf("failed to set search path: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "SET plan_cache_mode = force_generic_plan"); err != nil {
		return nil, fmt.Errorf("failed to set plan cache mode: %w", err)
	}

	paramCount := len(paramCountRegex.FindAllString(qi.queryText, -1))

	nullParams := strings.Repeat("null,", paramCount)
	if paramCount > 0 {
		nullParams = nullParams[:len(nullParams)-1]
	}

	explainQuery := fmt.Sprintf("%s%s(%s)", selectExplainPlanPrefix, preparedStatementName, nullParams)
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
