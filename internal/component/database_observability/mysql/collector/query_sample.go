package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/xwb1989/sqlparser"

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
		query_sample_text,
		query_sample_seen,
		query_sample_timer_wait
	FROM performance_schema.events_statements_summary_by_digest
	WHERE schema_name NOT IN ('mysql', 'performance_schema', 'information_schema')
	AND last_seen > DATE_SUB(NOW(), INTERVAL 1 DAY)`

type QuerySampleArguments struct {
	DB              *sql.DB
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler

	Logger log.Logger
}

type QuerySample struct {
	dbConnection    *sql.DB
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
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		logger:          args.Logger,
		running:         &atomic.Bool{},
	}, nil
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
		if err := rs.Err(); err != nil {
			level.Error(c.logger).Log("msg", "failed to iterate rs", "err", err)
			break
		}

		var digest, sampleText, sampleSeen, sampleTimerWait string
		err := rs.Scan(&digest, &sampleText, &sampleSeen, &sampleTimerWait)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan result set for query samples", "err", err)
			continue
		}

		if strings.HasSuffix(sampleText, "...") {
			level.Info(c.logger).Log("msg", "skipping parsing truncated query", "digest", digest)
			continue
		}

		stmt, err := sqlparser.Parse(sampleText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to parse sql query", "digest", digest, "err", err)
			continue
		}

		sampleRedactedText, err := sqlparser.RedactSQLQuery(sampleText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to redact sql query", "digest", digest, "err", err)
			continue
		}

		c.entryHandler.Chan() <- loki.Entry{
			Labels: model.LabelSet{"job": database_observability.JobName},
			Entry: logproto.Entry{
				Timestamp: time.Unix(0, time.Now().UnixNano()),
				Line: fmt.Sprintf(
					`level=info msg="query samples fetched" op="%s" digest="%s" query_type="%s" query_sample_seen="%s" query_sample_timer_wait="%s" query_sample_redacted="%s"`,
					OP_QUERY_SAMPLE, digest, c.stmtType(stmt), sampleSeen, sampleTimerWait, sampleRedactedText,
				),
			},
		}

		tables := c.tablesFromQuery(stmt)
		for _, table := range tables {
			c.entryHandler.Chan() <- loki.Entry{
				Labels: model.LabelSet{"job": database_observability.JobName},
				Entry: logproto.Entry{
					Timestamp: time.Unix(0, time.Now().UnixNano()),
					Line: fmt.Sprintf(
						`level=info msg="table name parsed" op="%s" digest="%s" table="%s"`,
						OP_QUERY_PARSED_TABLE_NAME, digest, table,
					),
				},
			}
		}
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
	default:
		return ""
	}
}

func (c QuerySample) tablesFromQuery(stmt sqlparser.Statement) []string {
	var parsedTables []string

	switch stmt := stmt.(type) {
	case *sqlparser.Select:
		parsedTables = c.parseTableExprs(stmt.From)
	case *sqlparser.Insert:
		parsedTables = []string{c.parseTableName(stmt.Table)}
	case *sqlparser.Update:
		parsedTables = c.parseTableExprs(stmt.TableExprs)
	case *sqlparser.Delete:
		parsedTables = c.parseTableExprs(stmt.TableExprs)
	}

	return parsedTables
}

func (c QuerySample) parseTableExprs(tables sqlparser.TableExprs) []string {
	parsedTables := []string{}
	for i := 0; i < len(tables); i++ {
		t := tables[i]
		switch tableExpr := t.(type) {
		case *sqlparser.AliasedTableExpr:
			switch expr := tableExpr.Expr.(type) {
			case sqlparser.TableName:
				parsedTables = append(parsedTables, c.parseTableName(expr))
			case *sqlparser.Subquery:
				subquery := expr.Select.(*sqlparser.Select)
				parsedTables = append(parsedTables, c.parseTableExprs(subquery.From)...)
			default:
				level.Error(c.logger).Log("msg", "unknown nested table expression", "table", tableExpr)
			}
		case *sqlparser.JoinTableExpr:
			// continue parsing both sides of join
			tables = append(tables, tableExpr.LeftExpr, tableExpr.RightExpr)
		default:
			level.Error(c.logger).Log("msg", "unknown table type", "table", t)
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
