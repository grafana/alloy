package collector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/DataDog/go-sqllexer"
	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	QueryDetailsCollector      = "query_details"
	OP_QUERY_ASSOCIATION       = "query_association"
	OP_QUERY_PARSED_TABLE_NAME = "query_parsed_table_name"
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
	EntryHandler    loki.EntryHandler

	Logger log.Logger
}

type QueryDetails struct {
	dbConnection    *sql.DB
	collectInterval time.Duration
	statementsLimit int
	excludeSchemas  []string
	entryHandler    loki.EntryHandler
	sqlParser       parser.Parser
	normalizer      *sqllexer.Normalizer

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewQueryDetails(args QueryDetailsArguments) (*QueryDetails, error) {
	c := &QueryDetails{
		dbConnection:    args.DB,
		collectInterval: args.CollectInterval,
		statementsLimit: args.StatementsLimit,
		excludeSchemas:  args.ExcludeSchemas,
		entryHandler:    args.EntryHandler,
		sqlParser:       parser.NewTiDBSqlParser(),
		normalizer:      sqllexer.NewNormalizer(sqllexer.WithCollectTables(true)),
		logger:          log.With(args.Logger, "collector", QueryDetailsCollector),
		running:         &atomic.Bool{},
	}

	return c, nil
}

func (c *QueryDetails) Name() string {
	return QueryDetailsCollector
}

func (c *QueryDetails) Start(ctx context.Context) error {
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

		ticker := time.NewTicker(c.collectInterval)

		for {
			if err := c.tablesFromEventsStatements(c.ctx); err != nil {
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

func (c *QueryDetails) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *QueryDetails) Stop() {
	c.cancel()
}

func (c *QueryDetails) tablesFromEventsStatements(ctx context.Context) error {
	query := fmt.Sprintf(selectQueryTablesSamples, buildExcludedSchemasClause(c.excludeSchemas), c.statementsLimit)
	rs, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch summary table samples: %w", err)
	}
	defer rs.Close()

	for rs.Next() {
		var digest, digestText, schema, sampleText string
		if err := rs.Scan(&digest, &digestText, &schema, &sampleText); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan result set from summary table samples", "schema", schema, "err", err)
			continue
		}

		var tables []string
		var parserErr, lexerErr error
		if tables, parserErr = c.tryParseTableNames(sampleText, digestText); parserErr != nil {
			if tables, lexerErr = c.tryTokenizeTableNames(sampleText, digestText); lexerErr != nil {
				level.Warn(c.logger).Log("msg", "failed to extract tables from sql text", "schema", schema, "digest", digest, "parser_err", parserErr, "lexer_err", lexerErr)
				continue
			}
		}

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_QUERY_ASSOCIATION,
			fmt.Sprintf(`schema="%s" parseable="%t" digest="%s" digest_text="%s"`, schema, parserErr == nil, digest, digestText),
		)

		for _, table := range tables {
			c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
				logging.LevelInfo,
				OP_QUERY_PARSED_TABLE_NAME,
				fmt.Sprintf(`schema="%s" digest="%s" table="%s"`, schema, digest, table),
			)
		}
	}

	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate over samples result set: %w", err)
	}

	return nil
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
			return nil, fmt.Errorf("failed to tokenize sql text (without fallback): %w", err)
		}

		if _, metadata, err = c.normalizer.Normalize(fallbackSqlText, sqllexer.WithDBMS(sqllexer.DBMSMySQL)); err != nil {
			return nil, fmt.Errorf("failed to tokenize sql text (fallback): %w", err)
		}
	}

	return metadata.Tables, nil
}
