package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/loki/v3/pkg/logproto"
)

const (
	OP_QUERY_SAMPLE            = "query_sample"
	OP_QUERY_PARSED_TABLE_NAME = "query_parsed_table_name"
	QuerySampleName            = "query_sample"
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
		logger:          log.With(args.Logger, "collector", QuerySampleName),
		running:         &atomic.Bool{},
	}, nil
}

func (c *QuerySample) Name() string {
	return QuerySampleName
}

func (c *QuerySample) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", QuerySampleName+" collector started")

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
			idx := strings.LastIndex(sampleText, "/*")
			if idx < 0 {
				level.Debug(c.logger).Log("msg", "skipping parsing truncated query", "schema", schemaName, "digest", digest)
				continue
			}

			trailingText := sampleText[idx:]
			if strings.LastIndex(trailingText, "*/") >= 0 {
				level.Debug(c.logger).Log("msg", "skipping parsing truncated query with comment", "schema", schemaName, "digest", digest)
				continue
			}

			sampleText = sampleText[:idx]
		}

		stmt, err := ParseSql(sampleText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to parse sql query", "schema", schemaName, "digest", digest, "err", err)
			continue
		}

		sampleRedactedText, err := RedactSQL(sampleText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to redact sql query", "schema", schemaName, "digest", digest, "err", err)
			continue
		}

		c.entryHandler.Chan() <- loki.Entry{
			Labels: model.LabelSet{
				"job":      database_observability.JobName,
				"op":       OP_QUERY_SAMPLE,
				"instance": model.LabelValue(c.instanceKey),
			},
			Entry: logproto.Entry{
				Timestamp: time.Unix(0, time.Now().UnixNano()),
				Line: fmt.Sprintf(
					`level=info msg="query samples fetched" schema="%s" digest="%s" query_type="%s" query_sample_seen="%s" query_sample_timer_wait="%s" query_sample_redacted="%s"`,
					schemaName, digest, StmtType(stmt), sampleSeen, sampleTimerWait, sampleRedactedText,
				),
			},
		}

		tables := ExtractTableNames(c.logger, digest, stmt)
		for _, table := range tables {
			c.entryHandler.Chan() <- loki.Entry{
				Labels: model.LabelSet{
					"job":      database_observability.JobName,
					"op":       OP_QUERY_PARSED_TABLE_NAME,
					"instance": model.LabelValue(c.instanceKey),
				},
				Entry: logproto.Entry{
					Timestamp: time.Unix(0, time.Now().UnixNano()),
					Line: fmt.Sprintf(
						`level=info msg="table name parsed" schema="%s" digest="%s" table="%s"`,
						schemaName, digest, table,
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
