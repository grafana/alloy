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
	OP_QUERY_SAMPLE = "query_sample"
	QuerySampleName = "query_sample"
)

const selectLatestTimerEnd = `SELECT MAX(TIMER_END) FROM performance_schema.events_statements_history`

const selectQuerySamples = `
SELECT unix_timestamp()             AS now,
	global_status.variable_value AS uptime,
	statements.CURRENT_SCHEMA,
	statements.DIGEST,
	statements.DIGEST_TEXT,
	statements.TIMER_START,
	statements.TIMER_END,
	statements.TIMER_WAIT,
	statements.CPU_TIME,
	statements.ROWS_EXAMINED,
	statements.ROWS_SENT,
	statements.ROWS_AFFECTED,
	statements.ERRORS,
	statements.MAX_CONTROLLED_MEMORY,
	statements.MAX_TOTAL_MEMORY
	%s
FROM performance_schema.events_statements_history AS statements
JOIN performance_schema.global_status
WHERE statements.sql_text IS NOT NULL
	AND global_status.variable_name = 'UPTIME'
	AND statements.CURRENT_SCHEMA NOT IN ('mysql', 'performance_schema', 'sys', 'information_schema')
	AND statements.TIMER_END >= ?;`

type QuerySampleArguments struct {
	DB                    *sql.DB
	InstanceKey           string
	CollectInterval       time.Duration
	EntryHandler          loki.EntryHandler
	UseTiDBParser         bool
	DisableQueryRedaction bool

	Logger log.Logger
}

type QuerySample struct {
	dbConnection          *sql.DB
	instanceKey           string
	collectInterval       time.Duration
	entryHandler          loki.EntryHandler
	sqlParser             parser.Parser
	disableQueryRedaction bool

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc

	lastSampleSeenTimestamp float64
}

func NewQuerySample(args QuerySampleArguments) (*QuerySample, error) {
	c := &QuerySample{
		dbConnection:          args.DB,
		instanceKey:           args.InstanceKey,
		collectInterval:       args.CollectInterval,
		entryHandler:          args.EntryHandler,
		disableQueryRedaction: args.DisableQueryRedaction,
		logger:                log.With(args.Logger, "collector", QuerySampleName),
		running:               &atomic.Bool{},
	}

	if args.UseTiDBParser {
		c.sqlParser = parser.NewTiDBSqlParser()
	} else {
		c.sqlParser = parser.NewXwbSqlParser()
	}

	return c, nil
}

func (c *QuerySample) Name() string {
	return QuerySampleName
}

func (c *QuerySample) Start(ctx context.Context) error {
	if c.disableQueryRedaction {
		level.Warn(c.logger).Log("msg", "collector started with query redaction disabled. Query samples will include complete SQL text including query parameters.")
	} else {
		level.Debug(c.logger).Log("msg", "collector started")
	}

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	if err := c.setLastSampleSeenTimestamp(c.ctx); err != nil {
		level.Error(c.logger).Log("msg", "failed to set last sample seen timestamp", "err", err)
		return err
	}

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

// setLastSampleSeenTimestamp queries the database for the latest sample seen timestamp so that upon startup we don't collect
// samples that may have been collected previously.
func (c *QuerySample) setLastSampleSeenTimestamp(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectLatestTimerEnd)
	if err != nil {
		return fmt.Errorf("failed to fetch last sample seen timestamp: %w", err)
	}
	defer rs.Close()

	for rs.Next() {
		var ts sql.NullFloat64
		err := rs.Scan(&ts)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		if !ts.Valid {
			return fmt.Errorf("no valid timestamp found: %w", err)
		}
		c.lastSampleSeenTimestamp = ts.Float64
	}
	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate rows: %w", err)
	}
	return nil
}

func (c *QuerySample) fetchQuerySamples(ctx context.Context) error {
	var sqlTextField string
	if c.disableQueryRedaction {
		sqlTextField = ",statements.SQL_TEXT"
	}
	query := fmt.Sprintf(selectQuerySamples, sqlTextField)

	rs, err := c.dbConnection.QueryContext(ctx, query, c.lastSampleSeenTimestamp)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to fetch history table samples", "err", err)
		return err
	}
	defer rs.Close()

	for rs.Next() {
		row := struct {
			// system times
			NowSeconds    uint64
			UptimeSeconds uint64

			// sample query details
			Schema     sql.NullString
			Digest     sql.NullString
			DigestText sql.NullString
			SQLText    sql.NullString

			// sample time
			TimerStartPicoseconds  sql.NullFloat64
			TimerEndPicoseconds    sql.NullFloat64
			ElapsedTimePicoseconds sql.NullInt64
			CPUTime                uint64

			// sample row info
			RowsExamined uint64
			RowsSent     uint64
			RowsAffected uint64
			Errors       uint64

			// sample memory info
			MaxControlledMemory uint64
			MaxTotalMemory      uint64
		}{}

		scanArgs := []interface{}{
			&row.NowSeconds,
			&row.UptimeSeconds,
			&row.Schema,
			&row.Digest,
			&row.DigestText,
			&row.TimerStartPicoseconds,
			&row.TimerEndPicoseconds,
			&row.ElapsedTimePicoseconds,
			&row.CPUTime,
			&row.RowsExamined,
			&row.RowsSent,
			&row.RowsAffected,
			&row.Errors,
			&row.MaxControlledMemory,
			&row.MaxTotalMemory,
		}
		if c.disableQueryRedaction {
			scanArgs = append(scanArgs, &row.SQLText)
		}

		err := rs.Scan(scanArgs...)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan history table samples", "err", err)
			continue
		}

		if !row.TimerEndPicoseconds.Valid {
			level.Debug(c.logger).Log("msg", "skipping query with invalid timer end timestamp", "schema", row.Schema.String, "digest", row.Digest.String, "timer_end", row.TimerEndPicoseconds.Float64)
			continue
		}

		if row.TimerEndPicoseconds.Float64 > c.lastSampleSeenTimestamp {
			c.lastSampleSeenTimestamp = row.TimerEndPicoseconds.Float64
		}

		digestText, err := c.sqlParser.CleanTruncatedText(row.DigestText.String)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to handle truncated sql query", "schema", row.Schema.String, "digest", row.Digest.String, "err", err)
			continue
		}

		digestText, err = c.sqlParser.Redact(digestText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to redact sql query", "schema", row.Schema.String, "digest", row.Digest.String, "err", err)
			continue
		}

		cpuTime := float64(row.CPUTime) / 1e9
		elapsedTime := float64(row.ElapsedTimePicoseconds.Int64) / 1e9

		logMessage := fmt.Sprintf(
			`schema="%s" digest="%s" digest_text="%s" rows_examined="%d" rows_sent="%d" rows_affected="%d" errors="%d" max_controlled_memory="%db" max_total_memory="%db" cpu_time="%fms" elapsed_time="%fms" elapsed_time_ms="%fms"`,
			row.Schema.String,
			row.Digest.String,
			digestText,
			row.RowsExamined,
			row.RowsSent,
			row.RowsAffected,
			row.Errors,
			row.MaxControlledMemory,
			row.MaxTotalMemory,
			cpuTime,
			elapsedTime,
			elapsedTime,
		)
		if c.disableQueryRedaction && row.SQLText.Valid {
			logMessage += fmt.Sprintf(` sql_text="%s"`, row.SQLText.String)
		}

		c.entryHandler.Chan() <- buildLokiEntry(
			logging.LevelInfo,
			OP_QUERY_SAMPLE,
			c.instanceKey,
			logMessage,
		)
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over samples result set", "err", err)
		return err
	}

	return nil
}
