package collector

import (
	"context"
	"database/sql"
	"fmt"
	"math"
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

const selectUptime = `SELECT variable_value FROM performance_schema.global_status WHERE variable_name = 'UPTIME'`

const selectNowAndUptime = `
SELECT unix_timestamp() AS now,
       variable_value AS uptime
FROM performance_schema.global_status
WHERE variable_name = 'UPTIME'`

const selectQuerySamples = `
SELECT statements.CURRENT_SCHEMA,
	statements.DIGEST,
	statements.DIGEST_TEXT,
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
WHERE statements.sql_text IS NOT NULL
  AND statements.CURRENT_SCHEMA NOT IN ('mysql', 'performance_schema', 'sys', 'information_schema')
  %s`

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

	timerBookmark float64
	lastUptime    float64
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

	if err := c.initializeBookmark(c.ctx); err != nil {
		level.Error(c.logger).Log("msg", "failed to initialize bookmark", "err", err)
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

// initializeBookmark queries the database for the uptime since overflow (if any) so that upon startup we don't collect
// samples that may have been collected previously.
func (c *QuerySample) initializeBookmark(ctx context.Context) error {
	rs := c.dbConnection.QueryRowContext(ctx, selectUptime)

	var uptime float64
	err := rs.Scan(&uptime)
	if err != nil {
		return fmt.Errorf("failed to scan uptime: %w", err)
	}

	c.lastUptime = uptime
	c.timerBookmark = uptimeSinceOverflow(uptime)
	return nil
}

func (c *QuerySample) fetchQuerySamples(ctx context.Context) error {
	timeRow := c.dbConnection.QueryRowContext(ctx, selectNowAndUptime)

	var now, uptime float64
	if err := timeRow.Scan(&now, &uptime); err != nil {
		return fmt.Errorf("failed to scan now and uptime info: %w", err)
	}

	timerClause, limit := c.determineTimerClauseAndLimit(uptime)

	var sqlTextField string
	if c.disableQueryRedaction {
		sqlTextField = ",statements.SQL_TEXT"
	}

	query := fmt.Sprintf(selectQuerySamples, sqlTextField, timerClause)

	rs, err := c.dbConnection.QueryContext(ctx, query, c.timerBookmark, limit)
	if err != nil {
		return fmt.Errorf("failed to fetch query samples: %w", err)
	}
	defer rs.Close()

	// set the new bookmarks
	c.timerBookmark = limit
	c.lastUptime = uptime

	for rs.Next() {
		row := struct {
			// sample query details
			Schema     sql.NullString
			Digest     sql.NullString
			DigestText sql.NullString
			SQLText    sql.NullString

			// sample time
			TimerEndPicoseconds    sql.NullFloat64
			TimestampMilliseconds  float64
			ElapsedTimePicoseconds sql.NullFloat64
			CPUTime                float64

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
			&row.Schema,
			&row.Digest,
			&row.DigestText,
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

		serverStartTime := now - uptime
		row.TimestampMilliseconds = c.calculateWallTime(serverStartTime, row.TimerEndPicoseconds.Float64)

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

		cpuTime := picosecondsToMilliseconds(row.CPUTime)
		elapsedTime := picosecondsToMilliseconds(row.ElapsedTimePicoseconds.Float64)

		logMessage :=
			fmt.Sprintf(
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

		timestampNanoseconds := millisecondsToNanoseconds(row.TimestampMilliseconds)
		c.entryHandler.Chan() <- buildLokiEntry(
			logging.LevelInfo,
			OP_QUERY_SAMPLE,
			c.instanceKey,
			logMessage,
			&timestampNanoseconds,
		)
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over samples result set", "err", err)
		return err
	}

	return nil
}

func (c *QuerySample) calculateWallTime(serverStartTime, timer float64) float64 {
	// timer indicates event timing since server startup.
	// The timer value is in picoseconds with a column type of bigint unsigned. This value can overflow after about ~213 days.
	// We need to account for this overflow when calculating the timestamp.

	// Knowing the number of overflows that occurred, we can calculate how much overflow time to compensate.
	previousOverflows := calculateNumberOfOverflows(c.lastUptime)
	overflowTime := float64(previousOverflows) * picosecondsOverflowInSeconds
	// We then add this overflow compensation to the server start time, and also add the timer value (remember this is counted from server start).
	// The resulting value is the timestamp in seconds at which an event happened.
	timerSeconds := picosecondsToSeconds(timer)
	timestampSeconds := serverStartTime + overflowTime + timerSeconds

	return secondsToMilliseconds(timestampSeconds)
}

func calculateNumberOfOverflows(uptime float64) int {
	return int(math.Floor(uptime / picosecondsOverflowInSeconds))
}

const (
	endOfTimeline             = ` AND statements.TIMER_END > ? AND statements.TIMER_END <= ?;`
	beginningAndEndOfTimeline = ` AND statements.TIMER_END > ? OR statements.TIMER_END <= ?;`
)

func (c *QuerySample) determineTimerClauseAndLimit(uptime float64) (string, float64) {
	timerClause := endOfTimeline
	currentOverflows := calculateNumberOfOverflows(uptime)
	previousOverflows := calculateNumberOfOverflows(c.lastUptime)
	switch {
	case currentOverflows > previousOverflows:
		// if we have just overflowed, collect both the beginning and end of the timeline
		timerClause = beginningAndEndOfTimeline
	case uptime < c.lastUptime:
		// server has restarted
		c.timerBookmark = 0
	}

	limit := uptimeSinceOverflow(uptime)

	return timerClause, limit
}

// uptimeSinceOverflow calculates the uptime "modulo" overflows (if any): it returns the remainder of the uptime value with any
// overflowed time removed
func uptimeSinceOverflow(uptime float64) float64 {
	overflowAdjustment := float64(calculateNumberOfOverflows(uptime)) * picosecondsOverflowInSeconds
	return secondsToPicoseconds(uptime - overflowAdjustment)
}

var picosecondsOverflowInSeconds = picosecondsToSeconds(float64(math.MaxUint64))

const (
	picosecondsPerSecond      float64 = 1e12
	millisecondsPerSecond     float64 = 1e3
	millisecondsPerPicosecond float64 = 1e9
	nanosecondsPerMillisecond float64 = 1e6
)

func picosecondsToSeconds(picoseconds float64) float64 {
	return picoseconds / picosecondsPerSecond
}

func picosecondsToMilliseconds(picoseconds float64) float64 {
	return picoseconds / millisecondsPerPicosecond
}

func millisecondsToNanoseconds(milliseconds float64) float64 {
	return milliseconds * nanosecondsPerMillisecond
}

func secondsToPicoseconds(seconds float64) float64 {
	return seconds * picosecondsPerSecond
}

func secondsToMilliseconds(seconds float64) float64 {
	return seconds * millisecondsPerSecond
}
