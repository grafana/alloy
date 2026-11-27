package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/DataDog/go-sqllexer"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	QueryDetailsCollector = "query_details"
)

const selectQueryTablesSamples = `
	SELECT
		digest,
		digest_text,
		schema_name,
		query_sample_text
	FROM performance_schema.events_statements_summary_by_digest
	WHERE last_seen > DATE_SUB(NOW(), INTERVAL 1 DAY)
	AND schema_name NOT IN %s
	ORDER BY last_seen DESC
	LIMIT %d`

type QueryDetailsArguments struct {
	DB              *sql.DB
	CollectInterval time.Duration
	StatementsLimit int
	ExcludeSchemas  []string
	IncludeSchemas  []string
	EntryHandler    loki.EntryHandler
	TableRegistry   *TableRegistry

	Logger *slog.Logger
}

type queryDetailsKey struct {
	schema string
	digest string
}

type QueryDetails struct {
	dbConnection    *sql.DB
	collectInterval time.Duration
	statementsLimit int
	excludeSchemas  []string
	includeSchemas  []string
	entryHandler    loki.EntryHandler
	sqlParser       parser.Parser
	normalizer      *sqllexer.Normalizer
	tableRegistry   *TableRegistry

	// lastEmittedAt records the wall-clock time at which OP_QUERY_ASSOCIATION
	// was last emitted for a (schema, digest), used to throttle logging
	// to at most one emission per EmitInterval per query.
	lastEmittedAt map[queryDetailsKey]time.Time
	// unvalidated tracks schemas that were unavailable when a query's parsed
	// tables were emitted, so the query can be retried once those schemas appear.
	unvalidated map[queryDetailsKey]map[string]struct{}
	now         func() time.Time

	logger  *slog.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewQueryDetails(args QueryDetailsArguments) (*QueryDetails, error) {
	c := &QueryDetails{
		dbConnection:    args.DB,
		collectInterval: args.CollectInterval,
		statementsLimit: args.StatementsLimit,
		excludeSchemas:  args.ExcludeSchemas,
		includeSchemas:  args.IncludeSchemas,
		entryHandler:    args.EntryHandler,
		sqlParser:       parser.NewTiDBSqlParser(),
		normalizer:      sqllexer.NewNormalizer(sqllexer.WithCollectTables(true)),
		tableRegistry:   args.TableRegistry,
		lastEmittedAt:   map[queryDetailsKey]time.Time{},
		unvalidated: map[queryDetailsKey]map[string]struct{}{},
		now:             time.Now,
		logger:          args.Logger.With("collector", QueryDetailsCollector),
		running:         &atomic.Bool{},
	}

	return c, nil
}

func (c *QueryDetails) Name() string {
	return QueryDetailsCollector
}

func (c *QueryDetails) Start(ctx context.Context) error {
	c.logger.Debug("collector started")

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			if err := c.tablesFromEventsStatements(c.ctx); err != nil {
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

func (c *QueryDetails) Stopped() bool {
	return !c.running.Load()
}

func (c *QueryDetails) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *QueryDetails) tablesFromEventsStatements(ctx context.Context) error {
	// nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	query := fmt.Sprintf(selectQueryTablesSamples, buildSchemaFilterClause(c.excludeSchemas, c.includeSchemas, "schema_name"), c.statementsLimit) // #nosec G201 -- filter values are quote-escaped before interpolation
	rs, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch summary table samples: %w", err)
	}
	defer rs.Close()

	now := c.now()
	for rs.Next() {
		var digest, digestText, schema string
		var sampleText sql.NullString
		if err := rs.Scan(&digest, &digestText, &schema, &sampleText); err != nil {
			c.logger.Error("failed to scan result set from summary table samples", "schema", schema, "err", err)
			continue
		}

		key := queryDetailsKey{schema: schema, digest: digest}
		if last, ok := c.lastEmittedAt[key]; ok && now.Sub(last) < database_observability.EmitInterval {
			if missingSchemas, pending := c.unvalidated[key]; !pending || !c.tableRegistryContains(missingSchemas) {
				continue
			}
		}

		missingSchemas := c.emit(schema, digest, digestText, sampleText.String)
		c.lastEmittedAt[key] = now
		if len(missingSchemas) > 0 {
			c.unvalidated[key] = missingSchemas
		} else {
			delete(c.unvalidated, key)
		}
	}

	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate over samples result set: %w", err)
	}

	c.pruneThrottle(now)
	return nil
}

// emit logs the query association and any table names parsed from the query.
// It returns schemas that were unavailable during validation, so the query can
// be re-emitted once schema collection populates tableRegistry.
func (c *QueryDetails) emit(schema, digest, digestText, sampleText string) map[string]struct{} {
	tables, parserErr := c.tryParseTableNames(sampleText, digestText)
	c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
		logging.LevelInfo,
		database_observability.OP_QUERY_ASSOCIATION,
		fmt.Sprintf(`schema=%q parseable="%t" digest=%q digest_text=%q`, schema, parserErr == nil, digest, digestText),
	)

	if parserErr != nil {
		var lexerErr error
		tables, lexerErr = c.tryTokenizeTableNames(sampleText, digestText)
		if lexerErr != nil {
			c.logger.Warn("failed to extract tables from sql text", "schema", schema, "digest", digest, "parser_err", parserErr, "lexer_err", lexerErr)
			return nil
		}
	}

	if len(tables) == 0 {
		return nil
	}

	missingSchemas := make(map[string]struct{})
	parsedTables := make([]database_observability.ParsedTable, 0, len(tables))
	for _, table := range tables {
		validated := false
		resolvedTable := table
		if c.tableRegistry != nil {
			referencedSchema := schema
			if qualifiedSchema, _, qualified := splitSchemaQualified(table); qualified {
				referencedSchema = qualifiedSchema
			}
			if !c.tableRegistry.HasSchema(referencedSchema) {
				missingSchemas[referencedSchema] = struct{}{}
			}
			resolvedTable, validated = c.tableRegistry.IsValid(schema, table)
		}
		parsedTables = append(parsedTables, database_observability.ParsedTable{Table: resolvedTable, Validated: validated})
	}
	tablesJSON, err := json.Marshal(parsedTables)
	if err != nil {
		c.logger.Error("failed to encode parsed tables", "err", err)
		return nil
	}
	c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
		logging.LevelInfo,
		database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2,
		fmt.Sprintf(`schema=%q digest=%q tables=%q`, schema, digest, tablesJSON),
	)

	return missingSchemas
}

func (c *QueryDetails) tableRegistryContains(schemas map[string]struct{}) bool {
	if c.tableRegistry == nil {
		return false
	}
	for schema := range schemas {
		if !c.tableRegistry.HasSchema(schema) {
			return false
		}
	}
	return true
}

// pruneThrottle drops entries whose last emission was at least EmitInterval ago
func (c *QueryDetails) pruneThrottle(now time.Time) {
	for key, last := range c.lastEmittedAt {
		if now.Sub(last) >= database_observability.EmitInterval {
			delete(c.lastEmittedAt, key)
			delete(c.unvalidated, key)
		}
	}
}

func (c *QueryDetails) tryParseTableNames(sqlText, fallbackSqlText string) ([]string, error) {
	sqlText, err := c.sqlParser.CleanTruncatedText(sqlText)
	if err != nil {
		sqlText, err = c.sqlParser.CleanTruncatedText(fallbackSqlText)
		if err != nil {
			return nil, fmt.Errorf("failed to handle truncated sql text: %w", err)
		}
	}

	stmt, err := c.sqlParser.Parse(sqlText)
	if err != nil {
		if fallbackSqlText == sqlText {
			return nil, fmt.Errorf("failed to parse sql text (without fallback): %w", err)
		}

		stmt, err = c.sqlParser.Parse(fallbackSqlText)
		if err != nil {
			return nil, fmt.Errorf("failed to parse sql text (fallback): %w", err)
		}
	}

	return c.sqlParser.ExtractTableNames(stmt), nil
}

func (c *QueryDetails) tryTokenizeTableNames(sqlText, fallbackSqlText string) ([]string, error) {
	var metadata *sqllexer.StatementMetadata
	var err error

	_, metadata, err = c.normalizer.Normalize(sqlText, sqllexer.WithDBMS(sqllexer.DBMSMySQL))
	if err != nil || len(metadata.Tables) == 0 {
		if fallbackSqlText == sqlText {
			if err != nil {
				return nil, fmt.Errorf("failed to tokenize sql text (without fallback): %w", err)
			}
			return nil, nil
		}

		if _, metadata, err = c.normalizer.Normalize(fallbackSqlText, sqllexer.WithDBMS(sqllexer.DBMSMySQL)); err != nil {
			return nil, fmt.Errorf("failed to tokenize sql text (fallback): %w", err)
		}
	}

	return metadata.Tables, nil
}
