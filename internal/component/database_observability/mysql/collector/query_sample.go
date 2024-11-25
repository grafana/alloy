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

	"github.com/grafana/alloy/internal/component/common/loki"
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
	WHERE last_seen > DATE_SUB(NOW(), INTERVAL 1 DAY)`

type QuerySampleArguments struct {
	DB             *sql.DB
	ScrapeInterval time.Duration
	EntryHandler   loki.EntryHandler

	Logger log.Logger
}

type QuerySample struct {
	dbConnection   *sql.DB
	scrapeInterval time.Duration
	entryHandler   loki.EntryHandler

	logger log.Logger

	ctx    context.Context
	cancel context.CancelFunc
}

func NewQuerySample(args QuerySampleArguments) (*QuerySample, error) {
	return &QuerySample{
		dbConnection:   args.DB,
		scrapeInterval: args.ScrapeInterval,
		entryHandler:   args.EntryHandler,
		logger:         args.Logger,
	}, nil
}

func (c *QuerySample) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "QuerySample collector started")

	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	go func() {
		ticker := time.NewTicker(c.scrapeInterval)

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

	c.Stop()
	return nil
}

func (c *QuerySample) Stop() {
	c.cancel()
	c.dbConnection.Close()
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
		var digest, query_sample_text, query_sample_seen, query_sample_timer_wait string
		err := rs.Scan(&digest, &query_sample_text, &query_sample_seen, &query_sample_timer_wait)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan query samples", "err", err)
			break
		}

		redacted, err := sqlparser.RedactSQLQuery(query_sample_text)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to redact sql query", "err", err)
		}

		c.entryHandler.Chan() <- loki.Entry{
			Labels: model.LabelSet{"job": "integrations/db-o11y"},
			Entry: logproto.Entry{
				Timestamp: time.Unix(0, time.Now().UnixNano()),
				Line:      fmt.Sprintf(`level=info msg="query samples fetched" op="%s" digest="%s" query_sample_text="%s" query_sample_seen="%s" query_sample_timer_wait="%s" query_redacted="%s"`, OP_QUERY_SAMPLE, digest, query_sample_text, query_sample_seen, query_sample_timer_wait, redacted),
			},
		}

		tables := c.tablesFromQuery(query_sample_text)
		for _, table := range tables {
			c.entryHandler.Chan() <- loki.Entry{
				Labels: model.LabelSet{"job": "integrations/db-o11y"},
				Entry: logproto.Entry{
					Timestamp: time.Unix(0, time.Now().UnixNano()),
					Line:      fmt.Sprintf(`level=info msg="table name parsed" op="%s" digest="%s" table="%s"`, OP_QUERY_PARSED_TABLE_NAME, digest, table),
				},
			}
		}
	}

	return nil
}

func (c QuerySample) tablesFromQuery(query string) []string {
	if strings.HasSuffix(query, "...") {
		level.Info(c.logger).Log("msg", "skipping parsing truncated query")
		return []string{}
	}

	stmt, err := sqlparser.Parse(query)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to parse sql query", "err", err)
		return []string{}
	}

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
			parsedTables = append(parsedTables, c.parseTableName(tableExpr.Expr.(sqlparser.TableName)))
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
