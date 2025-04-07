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
	OP_QUERY_TABLES            = "query_tables"
	OP_QUERY_PARSED_TABLE_NAME = "query_parsed_table_name"
	QueryTablesName            = "query_tables"
)

const selectQueryTablesSamples = `
	SELECT
		digest,
		schema_name,
		query_sample_text,
		query_sample_seen,
		query_sample_timer_wait
	FROM performance_schema.events_statements_summary_by_digest
	WHERE schema_name NOT IN ('mysql', 'performance_schema', 'information_schema')
	AND last_seen > DATE_SUB(NOW(), INTERVAL 1 DAY)`

type QueryTablesArguments struct {
	DB              *sql.DB
	InstanceKey     string
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler
	UseTiDBParser   bool

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
		logger:          log.With(args.Logger, "collector", QueryTablesName),
		running:         &atomic.Bool{},
	}

	if args.UseTiDBParser {
		c.sqlParser = parser.NewTiDBSqlParser()
	} else {
		c.sqlParser = parser.NewXwbSqlParser()
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
			if err := c.fetchQueryTables(c.ctx); err != nil {
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

func (c *QueryTables) fetchQueryTables(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectQueryTablesSamples)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to fetch summary table samples", "err", err)
		return err
	}
	defer rs.Close()

	for rs.Next() {
		var digest, schemaName, sampleText, sampleSeen, sampleTimerWait string
		err := rs.Scan(&digest, &schemaName, &sampleText, &sampleSeen, &sampleTimerWait)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan result set from summary table samples", "schema", schemaName, "err", err)
			continue
		}

		sampleText, err = c.sqlParser.CleanTruncatedText(sampleText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to handle truncated sql query", "schema", schemaName, "digest", digest, "err", err)
			continue
		}

		stmt, err := c.sqlParser.Parse(sampleText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to parse sql query", "schema", schemaName, "digest", digest, "err", err)
			continue
		}

		sampleRedactedText, err := c.sqlParser.Redact(sampleText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to redact sql query", "schema", schemaName, "digest", digest, "err", err)
			continue
		}

		c.entryHandler.Chan() <- buildLokiEntry(
			logging.LevelInfo,
			OP_QUERY_TABLES,
			c.instanceKey,
			fmt.Sprintf(
				`schema="%s" digest="%s" query_type="%s" query_sample_seen="%s" query_sample_timer_wait="%s" query_sample_redacted="%s"`,
				schemaName, digest, c.sqlParser.StmtType(stmt), sampleSeen, sampleTimerWait, sampleRedactedText,
			),
		)

		tables := c.sqlParser.ExtractTableNames(c.logger, digest, stmt)
		for _, table := range tables {
			c.entryHandler.Chan() <- buildLokiEntry(
				logging.LevelInfo,
				OP_QUERY_PARSED_TABLE_NAME,
				c.instanceKey,
				fmt.Sprintf(`schema="%s" digest="%s" table="%s"`, schemaName, digest, table),
			)
		}
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over samples result set", "err", err)
		return err
	}

	return nil
}
