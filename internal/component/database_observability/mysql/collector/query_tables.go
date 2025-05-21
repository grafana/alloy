package collector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
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

		stmt, err := c.tryParse(schema, digest, sampleText, digestText)
		if err != nil {
			// let tryParse log the error
			continue
		}

		tables := c.sqlParser.ExtractTableNames(c.logger, digest, stmt)
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

func (c *QueryTables) tryParse(schema, digest, sqlText, fallbackSqlText string) (any, error) {
	sqlText, err := c.sqlParser.CleanTruncatedText(sqlText)
	if err != nil {
		sqlText, err = c.sqlParser.CleanTruncatedText(fallbackSqlText)
		if err != nil {
			level.Warn(c.logger).Log("msg", "failed to handle truncated sql text", "schema", schema, "digest", digest, "err", err)
			return nil, err
		}
	}

	stmt, err := c.sqlParser.Parse(sqlText)
	if err != nil {
		if fallbackSqlText == sqlText {
			level.Warn(c.logger).Log("msg", "failed to parse sql text (without fallback)", "schema", schema, "digest", digest, "err", err)
			return nil, err
		}

		stmt, err = c.sqlParser.Parse(fallbackSqlText)
		if err != nil {
			level.Warn(c.logger).Log("msg", "failed to parse sql text (fallback)", "schema", schema, "digest", digest, "err", err)
			return nil, err
		}
	}

	return stmt, nil
}
