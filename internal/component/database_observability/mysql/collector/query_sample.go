package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	QuerySampleName = "query_sample"
	OP_QUERY_SAMPLE = "query_sample"
	OP_WAIT_EVENT   = "wait_event"

	sqlTextField              = `, statements.SQL_TEXT`
	sqlTextNotNullClause      = ` AND statements.SQL_TEXT IS NOT NULL`
	digestTextNotNullClause   = ` AND statements.DIGEST_TEXT IS NOT NULL`
	endOfTimeline             = ` AND statements.TIMER_END > ? AND statements.TIMER_END <= ?`
	beginningAndEndOfTimeline = ` AND statements.TIMER_END > ? OR statements.TIMER_END <= ?`
)

const selectUptime = `SELECT variable_value FROM performance_schema.global_status WHERE variable_name = 'UPTIME'`

const selectNowAndUptime = `
SELECT unix_timestamp() AS now,
       variable_value AS uptime
FROM performance_schema.global_status
WHERE variable_name = 'UPTIME'`

// selectQuerySamplesMysqlOlderThan8028 is used for MySQL versions older than 8.0.28
const selectQuerySamplesMysqlOlderThan8028 = `
SELECT
	statements.CURRENT_SCHEMA,
	statements.THREAD_ID,
	statements.EVENT_ID,
	statements.END_EVENT_ID,
	statements.DIGEST,
	statements.DIGEST_TEXT,
	statements.TIMER_END,
	statements.TIMER_WAIT,
	0,
	statements.ROWS_EXAMINED,
	statements.ROWS_SENT,
	statements.ROWS_AFFECTED,
	statements.ERRORS,
	0,
	0,
	waits.event_id as WAIT_EVENT_ID,
	waits.end_event_id as WAIT_END_EVENT_ID,
	waits.event_name as WAIT_EVENT_NAME,
	waits.object_name as WAIT_OBJECT_NAME,
	waits.object_type as WAIT_OBJECT_TYPE,
	waits.timer_wait as WAIT_TIMER_WAIT
	%s
FROM
	performance_schema.events_statements_history AS statements
LEFT JOIN
	performance_schema.events_waits_history waits
	ON statements.thread_id = waits.thread_id
	AND statements.EVENT_ID = waits.NESTING_EVENT_ID
WHERE
	statements.DIGEST IS NOT NULL
	AND statements.CURRENT_SCHEMA NOT IN ('mysql', 'performance_schema', 'sys', 'information_schema')
	%s %s`

// selectQuerySamplesMysqlOlderThan8031 is used for MySQL versions older than 8.0.31
const selectQuerySamplesMysqlOlderThan8031 = `
SELECT
	statements.CURRENT_SCHEMA,
	statements.THREAD_ID,
	statements.EVENT_ID,
	statements.END_EVENT_ID,
	statements.DIGEST,
	statements.DIGEST_TEXT,
	statements.TIMER_END,
	statements.TIMER_WAIT,
	statements.CPU_TIME,
	statements.ROWS_EXAMINED,
	statements.ROWS_SENT,
	statements.ROWS_AFFECTED,
	statements.ERRORS,
	0,
	0,
	waits.event_id as WAIT_EVENT_ID,
	waits.end_event_id as WAIT_END_EVENT_ID,
	waits.event_name as WAIT_EVENT_NAME,
	waits.object_name as WAIT_OBJECT_NAME,
	waits.object_type as WAIT_OBJECT_TYPE,
	waits.timer_wait as WAIT_TIMER_WAIT
	%s
FROM
	performance_schema.events_statements_history AS statements
LEFT JOIN
	performance_schema.events_waits_history waits
	ON statements.thread_id = waits.thread_id
	AND statements.EVENT_ID = waits.NESTING_EVENT_ID
WHERE
	statements.DIGEST IS NOT NULL
	AND statements.CURRENT_SCHEMA NOT IN ('mysql', 'performance_schema', 'sys', 'information_schema')
	%s %s`

// selectQuerySamples is used for MySQL versions 8.0.31 and newer
const selectQuerySamples = `
SELECT
	statements.CURRENT_SCHEMA,
	statements.THREAD_ID,
	statements.EVENT_ID,
	statements.END_EVENT_ID,
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
	statements.MAX_TOTAL_MEMORY,
	waits.event_id as WAIT_EVENT_ID,
	waits.end_event_id as WAIT_END_EVENT_ID,
	waits.event_name as WAIT_EVENT_NAME,
	waits.object_name as WAIT_OBJECT_NAME,
	waits.object_type as WAIT_OBJECT_TYPE,
	waits.timer_wait as WAIT_TIMER_WAIT
	%s
FROM
	performance_schema.events_statements_history AS statements
LEFT JOIN
	performance_schema.events_waits_history waits
	ON statements.thread_id = waits.thread_id
	AND statements.EVENT_ID = waits.NESTING_EVENT_ID
WHERE
	statements.DIGEST IS NOT NULL
	AND statements.CURRENT_SCHEMA NOT IN ('mysql', 'performance_schema', 'sys', 'information_schema')
	%s %s`

const updateSetupConsumers = `
	UPDATE performance_schema.setup_consumers
		SET enabled = 'yes'
		WHERE name in ('events_statements_cpu', 'events_waits_current', 'events_waits_history')`

type QuerySampleArguments struct {
	DB                          *sql.DB
	InstanceKey                 string
	CollectInterval             time.Duration
	EntryHandler                loki.EntryHandler
	DisableQueryRedaction       bool
	AutoEnableSetupConsumers    bool
	SetupConsumersCheckInterval time.Duration
	DBVersion                   string

	Logger log.Logger
}

type QuerySample struct {
	dbConnection                *sql.DB
	collectInterval             time.Duration
	entryHandler                loki.EntryHandler
	sqlParser                   parser.Parser
	disableQueryRedaction       bool
	autoEnableSetupConsumers    bool
	setupConsumersCheckInterval time.Duration
	DBVersion                   string

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc

	timerBookmark float64
	lastUptime    float64
}

func NewQuerySample(args QuerySampleArguments) (*QuerySample, error) {
	c := &QuerySample{
		dbConnection:                args.DB,
		collectInterval:             args.CollectInterval,
		entryHandler:                args.EntryHandler,
		sqlParser:                   parser.NewTiDBSqlParser(),
		disableQueryRedaction:       args.DisableQueryRedaction,
		autoEnableSetupConsumers:    args.AutoEnableSetupConsumers,
		setupConsumersCheckInterval: args.SetupConsumersCheckInterval,
		DBVersion:                   args.DBVersion,
		logger:                      log.With(args.Logger, "collector", QuerySampleName),
		running:                     &atomic.Bool{},
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

	// Start setup_consumers check goroutine if enabled
	if c.autoEnableSetupConsumers {
		go c.runSetupConsumersCheck()
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

func (c *QuerySample) runSetupConsumersCheck() {
	ticker := time.NewTicker(c.setupConsumersCheckInterval)

	for {
		if err := c.updateSetupConsumersSettings(c.ctx); err != nil {
			level.Error(c.logger).Log("msg", "error with performance_schema.setup_consumers check", "err", err)
		}

		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// continue loop
		}
	}
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

	var textField, textNotNullClause string
	if c.disableQueryRedaction {
		textField = sqlTextField
		textNotNullClause = sqlTextNotNullClause
	} else {
		textField = ""
		textNotNullClause = digestTextNotNullClause
	}

	// let's find out which major/minor/revision we're working with
	parts := strings.Split(c.DBVersion, ".")

	// we ignore errors here as this comes from component.go
	mysqlMajor, err := strconv.Atoi(parts[0])
	mysqlMinor, err := strconv.Atoi(parts[1])
	mysqlRevision, err := strconv.Atoi(parts[2])

	var baseQuery string

	// we start by assuming we're in version 8.4 or more recent
	baseQuery = selectQuerySamples

	// however... if we're on major version 8
	if mysqlMajor == 8 {
		level.Debug(c.logger).Log("msg", "We're in major version 8")
		// and minor version 0
		if mysqlMinor == 0 {
			level.Debug(c.logger).Log("msg", "We're in minor version 0")
			// older than v8.0.31
			if mysqlRevision < 31 && mysqlRevision >= 28 {
				level.Debug(c.logger).Log("msg", "We're past revisions 28 and 30")
				// we don't have MAX_CONTROLLED_MEMORY and MAX_TOTAL_MEMORY yet
				baseQuery = selectQuerySamplesMysqlOlderThan8031
			}
			// older than v8.0.28
			if mysqlRevision < 28 {
				level.Debug(c.logger).Log("msg", "We're in revision older than 28")
				// we don't even have CPU_TIME yet
				baseQuery = selectQuerySamplesMysqlOlderThan8028
			}
		}
	}

	query := fmt.Sprintf(baseQuery, textField, textNotNullClause, timerClause)

	rs, err := c.dbConnection.QueryContext(ctx, query, c.timerBookmark, limit)
	if err != nil {
		return fmt.Errorf("failed to fetch query samples: %w", err)
	}
	defer rs.Close()

	// set the new bookmarks
	c.timerBookmark = limit
	c.lastUptime = uptime

	lastDigestLogged := ""
	lastEventIDLogged := ""

	for rs.Next() {
		row := struct {
			// sample query details
			Schema              sql.NullString
			ThreadID            sql.NullString
			StatementEventID    sql.NullString
			StatementEndEventID sql.NullString
			Digest              sql.NullString
			DigestText          sql.NullString
			SQLText             sql.NullString

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

			// sample wait info, if any
			WaitEventID    sql.NullString
			WaitEndEventID sql.NullString
			WaitEventName  sql.NullString
			WaitObjectName sql.NullString
			WaitObjectType sql.NullString
			WaitTime       sql.NullFloat64
		}{}

		scanArgs := []interface{}{
			&row.Schema,
			&row.ThreadID,
			&row.StatementEventID,
			&row.StatementEndEventID,
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
			&row.WaitEventID,
			&row.WaitEndEventID,
			&row.WaitEventName,
			&row.WaitObjectName,
			&row.WaitObjectType,
			&row.WaitTime,
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
		row.TimestampMilliseconds = calculateWallTime(serverStartTime, row.TimerEndPicoseconds.Float64, uptime)

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

		logMessage := fmt.Sprintf(
			`schema="%s" thread_id="%s" event_id="%s" end_event_id="%s" digest="%s" digest_text="%s" rows_examined="%d" rows_sent="%d" rows_affected="%d" errors="%d" max_controlled_memory="%db" max_total_memory="%db" cpu_time="%fms" elapsed_time="%fms" elapsed_time_ms="%fms"`,
			row.Schema.String, row.ThreadID.String,
			row.StatementEventID.String, row.StatementEndEventID.String,
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

		if lastDigestLogged != row.Digest.String || lastEventIDLogged != row.StatementEventID.String {
			lastDigestLogged = row.Digest.String
			lastEventIDLogged = row.StatementEventID.String

			c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
				logging.LevelInfo,
				OP_QUERY_SAMPLE,
				logMessage,
				int64(millisecondsToNanoseconds(row.TimestampMilliseconds)),
			)
		}

		if row.WaitEventID.Valid {
			waitTime := picosecondsToMilliseconds(row.WaitTime.Float64)
			waitLogMessage := fmt.Sprintf(
				`schema="%s" thread_id="%s" digest="%s" digest_text="%s" event_id="%s" wait_event_id="%s" wait_end_event_id="%s" wait_event_name="%s" wait_object_name="%s" wait_object_type="%s" wait_time="%fms"`,
				row.Schema.String,
				row.ThreadID.String,
				row.Digest.String,
				digestText,
				row.StatementEventID.String,
				row.WaitEventID.String,
				row.WaitEndEventID.String,
				row.WaitEventName.String,
				row.WaitObjectName.String,
				row.WaitObjectType.String,
				waitTime,
			)

			if c.disableQueryRedaction && row.SQLText.Valid {
				waitLogMessage += fmt.Sprintf(` sql_text="%s"`, row.SQLText.String)
			}

			c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
				logging.LevelInfo,
				OP_WAIT_EVENT,
				waitLogMessage,
				int64(millisecondsToNanoseconds(row.TimestampMilliseconds)),
			)
		}
	}

	if err := rs.Err(); err != nil {
		return fmt.Errorf("error during iterating over samples result set: %w", err)
	}

	return nil
}

func (c *QuerySample) updateSetupConsumersSettings(ctx context.Context) error {
	rs, err := c.dbConnection.ExecContext(ctx, updateSetupConsumers)
	if err != nil {
		return fmt.Errorf("failed to update performance_schema.setup_consumers: %w", err)
	}

	rowsAffected, err := rs.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected from performance_schema.setup_consumers: %w", err)
	}
	level.Debug(c.logger).Log("msg", "updated performance_schema.setup_consumers", "rows_affected", rowsAffected)

	return nil
}

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
