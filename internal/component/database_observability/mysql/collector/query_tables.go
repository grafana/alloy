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
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	OP_QUERY_ASSOCIATION       = "query_association"
	OP_QUERY_PARSED_TABLE_NAME = "query_parsed_table_name"
	QueryTablesName            = "query_tables"
)

const selectQueryTablesSamples = `
	SELECT
		digest,
		digest_text,
		schema_name,
		query_sample_text
	FROM performance_schema.events_statements_summary_by_digest
	WHERE schema_name NOT IN ('mysql', 'performance_schema', 'information_schema')
	AND last_seen > DATE_SUB(NOW(), INTERVAL 1 DAY)`

type QueryTablesArguments struct {
	DB              *sql.DB
	InstanceKey     string
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler

	Logger log.Logger
}

type QueryTables struct {
	dbConnection    *sql.DB
	instanceKey     string
	collectInterval time.Duration
	entryHandler    loki.EntryHandler
	sqlParser       parser.Parser
	normalizer      *sqllexer.Normalizer

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewQueryTables(args QueryTablesArguments) (*QueryTables, error) {
	c := &QueryTables{
		dbConnection:    args.DB,
		instanceKey:     args.InstanceKey,
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		sqlParser:       parser.NewTiDBSqlParser(),
		normalizer:      sqllexer.NewNormalizer(sqllexer.WithCollectTables(true)),
		logger:          log.With(args.Logger, "collector", QueryTablesName),
		running:         &atomic.Bool{},
	}

	return c, nil
}

func (c *QueryTables) Name() string {
	return QueryTablesName
}

func (c *QueryTables) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", QueryTablesName+" collector started")

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

func (c *QueryTables) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *QueryTables) Stop() {
	c.cancel()
}

func (c *QueryTables) tablesFromEventsStatements(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectQueryTablesSamples)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to fetch summary table samples", "err", err)
		return err
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

		c.entryHandler.Chan() <- buildLokiEntry(
			logging.LevelInfo,
			OP_QUERY_ASSOCIATION,
			c.instanceKey,
			fmt.Sprintf(`schema="%s" parseable="%t" digest="%s" digest_text="%s"`, schema, parserErr == nil, digest, digestText),
		)

		for _, table := range tables {
			c.entryHandler.Chan() <- buildLokiEntry(
				logging.LevelInfo,
				OP_QUERY_PARSED_TABLE_NAME,
				c.instanceKey,
				fmt.Sprintf(`schema="%s" digest="%s" table="%s"`, schema, digest, table),
			)
		}
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over samples result set", "err", err)
		return err
	}

	return nil
}

func (c *QueryTables) tryParseTableNames(sqlText, fallbackSqlText string) ([]string, error) {
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

func (c *QueryTables) tryTokenizeTableNames(sqlText, fallbackSqlText string) ([]string, error) {
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
