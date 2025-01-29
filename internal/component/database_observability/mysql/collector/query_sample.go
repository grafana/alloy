package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/xwb1989/sqlparser"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/loki/v3/pkg/logproto"
)

const (
	OP_QUERY_SAMPLE            = "query_sample"
	OP_QUERY_PARSED_TABLE_NAME = "query_parsed_table_name"
)

const selectQuerySamples = `
	SELECT
		digest,
		schema_name,
		query_sample_text,
		query_sample_seen,
		query_sample_timer_wait
	FROM performance_schema.events_statements_summary_by_digest
	WHERE schema_name NOT IN ('mysql', 'performance_schema', 'information_schema')
	AND last_seen > DATE_SUB(NOW(), INTERVAL 1 DAY)`

type QuerySampleArguments struct {
	DB              *sql.DB
	InstanceKey     string
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler

	Logger log.Logger
}

type QuerySample struct {
	dbConnection    *sql.DB
	instanceKey     string
	collectInterval time.Duration
	entryHandler    loki.EntryHandler

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewQuerySample(args QuerySampleArguments) (*QuerySample, error) {
	return &QuerySample{
		dbConnection:    args.DB,
		instanceKey:     args.InstanceKey,
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		logger:          log.With(args.Logger, "collector", "QuerySample"),
		running:         &atomic.Bool{},
	}, nil
}

func (c *QuerySample) Name() string {
	return "QuerySample"
}

func (c *QuerySample) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "QuerySample collector started")

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
			if err := c.fetchQuerySamples(c.ctx); err != nil {
				level.Error(c.logger).Log("msg", "collector stopping due to error", "err", err)
				c.Stop()
				break
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

func (c *QuerySample) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *QuerySample) Stop() {
	c.cancel()
}

func (c *QuerySample) fetchQuerySamples(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectQuerySamples)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to fetch query samples", "err", err)
		return err
	}
	defer rs.Close()

	for rs.Next() {
		var digest, schemaName, sampleText, sampleSeen, sampleTimerWait string
		err := rs.Scan(&digest, &schemaName, &sampleText, &sampleSeen, &sampleTimerWait)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan result set for query samples", "err", err)
			continue
		}

		if strings.HasSuffix(sampleText, "...") {
			// best-effort attempt to detect truncated trailing comment
			if idx := strings.LastIndex(sampleText, "/*"); idx >= 0 {
				trailingPart := sampleText[idx:]
				if strings.LastIndex(trailingPart, "*/") < 0 {
					sampleText = sampleText[:idx]
				}
			} else {
				level.Debug(c.logger).Log("msg", "skipping parsing truncated query", "schema", schemaName, "digest", digest)
				continue
			}
		}

		stmt, err := sqlparser.Parse(sampleText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to parse sql query", "schema", schemaName, "digest", digest, "err", err)
			continue
		}

		sampleRedactedText, err := sqlparser.RedactSQLQuery(sampleText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to redact sql query", "schema", schemaName, "digest", digest, "err", err)
			continue
		}

		c.entryHandler.Chan() <- loki.Entry{
			Labels: model.LabelSet{"job": database_observability.JobName},
			Entry: logproto.Entry{
				Timestamp: time.Unix(0, time.Now().UnixNano()),
				Line: fmt.Sprintf(
					`level=info msg="query samples fetched" op="%s" instance="%s" schema="%s" digest="%s" query_type="%s" query_sample_seen="%s" query_sample_timer_wait="%s" query_sample_redacted="%s"`,
					OP_QUERY_SAMPLE, c.instanceKey, schemaName, digest, c.stmtType(stmt), sampleSeen, sampleTimerWait, sampleRedactedText,
				),
			},
		}

		tables := c.tablesFromQuery(digest, stmt)
		for _, table := range tables {
			c.entryHandler.Chan() <- loki.Entry{
				Labels: model.LabelSet{"job": database_observability.JobName},
				Entry: logproto.Entry{
					Timestamp: time.Unix(0, time.Now().UnixNano()),
					Line: fmt.Sprintf(
						`level=info msg="table name parsed" op="%s" instance="%s" schema="%s" digest="%s" table="%s"`,
						OP_QUERY_PARSED_TABLE_NAME, c.instanceKey, schemaName, digest, table,
					),
				},
			}
		}
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over samples result set", "err", err)
		return err
	}

	return nil
}

func (c QuerySample) stmtType(stmt sqlparser.Statement) string {
	switch stmt.(type) {
	case *sqlparser.Select:
		return "select"
	case *sqlparser.Insert:
		return "insert"
	case *sqlparser.Update:
		return "update"
	case *sqlparser.Delete:
		return "delete"
	case *sqlparser.Union:
		return "select" // label union as a select
	default:
		return ""
	}
}

func (c QuerySample) tablesFromQuery(digest string, stmt sqlparser.Statement) []string {
	var parsedTables []string

	switch stmt := stmt.(type) {
	case *sqlparser.Select:
		parsedTables = c.parseTableExprs(digest, stmt.From)
	case *sqlparser.Update:
		parsedTables = c.parseTableExprs(digest, stmt.TableExprs)
	case *sqlparser.Delete:
		parsedTables = c.parseTableExprs(digest, stmt.TableExprs)
	case *sqlparser.Insert:
		parsedTables = []string{c.parseTableName(stmt.Table)}
		switch insRowsStmt := stmt.Rows.(type) {
		case *sqlparser.Select:
			parsedTables = append(parsedTables, c.tablesFromQuery(digest, insRowsStmt)...)
		case *sqlparser.Union:
			for _, side := range []sqlparser.SelectStatement{insRowsStmt.Left, insRowsStmt.Right} {
				parsedTables = append(parsedTables, c.tablesFromQuery(digest, side)...)
			}
		case *sqlparser.ParenSelect:
			parsedTables = append(parsedTables, c.tablesFromQuery(digest, insRowsStmt.Select)...)
		default:
			level.Error(c.logger).Log("msg", "unknown insert type", "digest", digest)
		}
	case *sqlparser.Union:
		for _, side := range []sqlparser.SelectStatement{stmt.Left, stmt.Right} {
			parsedTables = append(parsedTables, c.tablesFromQuery(digest, side)...)
		}
	default:
		level.Error(c.logger).Log("msg", "unknown statement type", "digest", digest)
	}

	return parsedTables
}

func (c QuerySample) parseTableExprs(digest string, tables sqlparser.TableExprs) []string {
	parsedTables := []string{}
	for i := 0; i < len(tables); i++ {
		t := tables[i]
		switch tableExpr := t.(type) {
		case *sqlparser.AliasedTableExpr:
			switch expr := tableExpr.Expr.(type) {
			case sqlparser.TableName:
				parsedTables = append(parsedTables, c.parseTableName(expr))
			case *sqlparser.Subquery:
				switch subqueryExpr := expr.Select.(type) {
				case *sqlparser.Select:
					parsedTables = append(parsedTables, c.parseTableExprs(digest, subqueryExpr.From)...)
				case *sqlparser.Union:
					for _, side := range []sqlparser.SelectStatement{subqueryExpr.Left, subqueryExpr.Right} {
						parsedTables = append(parsedTables, c.tablesFromQuery(digest, side)...)
					}
				case *sqlparser.ParenSelect:
					parsedTables = append(parsedTables, c.tablesFromQuery(digest, subqueryExpr.Select)...)
				default:
					level.Error(c.logger).Log("msg", "unknown subquery type", "digest", digest)
				}
			default:
				level.Error(c.logger).Log("msg", "unknown nested table expression", "digest", digest, "table", tableExpr)
			}
		case *sqlparser.JoinTableExpr:
			// continue parsing both sides of join
			tables = append(tables, tableExpr.LeftExpr, tableExpr.RightExpr)
		case *sqlparser.ParenTableExpr:
			tables = append(tables, tableExpr.Exprs...)
		default:
			level.Error(c.logger).Log("msg", "unknown table type", "digest", digest, "table", t)
		}
	}
	return parsedTables
}

func (c QuerySample) parseTableName(t sqlparser.TableName) string {
	qualifier := t.Qualifier.String()
	tableName := t.Name.String()
	if qualifier != "" {
		return qualifier + "." + tableName
	}
	return tableName
}
