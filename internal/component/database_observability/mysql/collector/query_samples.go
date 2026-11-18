package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	QuerySamplesCollector = "query_samples"

	cpuTimeField              = `, statements.CPU_TIME`
	maxControlledMemoryField  = `, statements.MAX_CONTROLLED_MEMORY`
	maxTotalMemoryField       = `, statements.MAX_TOTAL_MEMORY`
	endOfTimeline             = ` AND statements.TIMER_END > ? AND statements.TIMER_END <= ?`
	beginningAndEndOfTimeline = ` AND statements.TIMER_END > ? OR statements.TIMER_END <= ?`
)

const selectUptime = `SELECT variable_value FROM performance_schema.global_status WHERE variable_name = 'UPTIME'`

const selectNowAndUptime = `
SELECT unix_timestamp() AS now,
       variable_value AS uptime
FROM performance_schema.global_status
WHERE variable_name = 'UPTIME'`

// selectLatestTimerEnd returns the latest performance_schema timer value in (bookmark, ceiling] among the rows
// of events_statements_current. That table holds each instrumented thread's latest statement and reports the
// current timer value as the TIMER_END of a statement still in progress, so the result is no later than the
// moment the query runs, and no earlier than the end of any statement in events_statements_history after the
// bookmark. It doesn't depend on the polling connection being instrumented.
const selectLatestTimerEnd = `
SELECT MAX(TIMER_END)
FROM performance_schema.events_statements_current
WHERE TIMER_END > ? AND TIMER_END <= ?`

// selectLatestTimerEndAcrossOverflow is selectLatestTimerEnd for when the timer may have overflowed since the
// bookmark: a value up to the ceiling comes after the overflow, so it is later than any value above the bookmark.
const selectLatestTimerEndAcrossOverflow = `
SELECT COALESCE(
	MAX(CASE WHEN TIMER_END <= ? THEN TIMER_END END),
	MAX(CASE WHEN TIMER_END > ? THEN TIMER_END END))
FROM performance_schema.events_statements_current`

// timerCeilingMarginSeconds is how far ahead of UPTIME the performance_schema timer is assumed to be at most.
// UPTIME is a whole number of seconds, and the timer starts counting when performance_schema initializes rather
// than when the server records its start time, so the timer can be up to about a second ahead of UPTIME. The
// margin only caps which timer values count as recent, so that a statement from before a timer overflow is not
// taken for one after it.
const timerCeilingMarginSeconds = 10

const selectQuerySamples = `
SELECT
	statements.CURRENT_SCHEMA,
	statements.THREAD_ID,
	statements.EVENT_ID,
	statements.END_EVENT_ID,
	statements.DIGEST,
	statements.SQL_TEXT,
	statements.TIMER_END,
	statements.TIMER_WAIT,
	statements.ROWS_EXAMINED,
	statements.ROWS_SENT,
	statements.ROWS_AFFECTED,
	statements.ERRORS,
	statements.MYSQL_ERRNO,
	statements.RETURNED_SQLSTATE,
	statements.MESSAGE_TEXT,
	waits.event_id as WAIT_EVENT_ID,
	waits.end_event_id as WAIT_END_EVENT_ID,
	waits.event_name as WAIT_EVENT_NAME,
	waits.object_name as WAIT_OBJECT_NAME,
	waits.object_type as WAIT_OBJECT_TYPE,
	waits.timer_wait as WAIT_TIMER_WAIT,
	nested_waits.event_id as NESTED_WAIT_EVENT_ID,
	nested_waits.end_event_id as NESTED_WAIT_END_EVENT_ID,
	nested_waits.event_name as NESTED_WAIT_EVENT_NAME,
	nested_waits.object_name as NESTED_WAIT_OBJECT_NAME,
	nested_waits.object_type as NESTED_WAIT_OBJECT_TYPE,
	nested_waits.timer_wait as NESTED_WAIT_TIMER_WAIT,
	threads.PROCESSLIST_USER as QUERY_USER,
	threads.PROCESSLIST_HOST as QUERY_HOST
	%s
FROM
	performance_schema.events_statements_history AS statements
LEFT JOIN
	performance_schema.events_waits_history waits
	ON statements.thread_id = waits.thread_id
	AND statements.EVENT_ID = waits.NESTING_EVENT_ID
	%s
LEFT JOIN
	performance_schema.events_waits_history nested_waits
	ON waits.thread_id = nested_waits.thread_id
	AND waits.event_id = nested_waits.nesting_event_id
	AND waits.event_name = 'wait/io/table/sql/handler'
	%s
LEFT JOIN
	performance_schema.threads threads
	ON statements.THREAD_ID = threads.THREAD_ID
WHERE
	statements.DIGEST IS NOT NULL
	AND statements.SQL_TEXT IS NOT NULL
	AND statements.CURRENT_SCHEMA NOT IN %s
	%s %s
ORDER BY statements.thread_id, statements.EVENT_ID`

const updateSetupConsumers = `
	UPDATE performance_schema.setup_consumers
		SET enabled = 'yes'
		WHERE name in ('events_statements_cpu', 'events_waits_current', 'events_waits_history')`

type QuerySamplesArguments struct {
	DB                            *sql.DB
	EngineVersion                 semver.Version
	CollectInterval               time.Duration
	ExcludeSchemas                []string
	IncludeSchemas                []string
	EntryHandler                  loki.EntryHandler
	Registry                      *prometheus.Registry
	DisableQueryRedaction         bool
	AutoEnableSetupConsumers      bool
	SetupConsumersCheckInterval   time.Duration
	SampleMinDuration             time.Duration
	WaitEventMinDuration          time.Duration
	EnablePreClassifiedWaitEvents bool

	Logger *slog.Logger
}

type QuerySamples struct {
	dbConnection                  *sql.DB
	engineVersion                 semver.Version
	collectInterval               time.Duration
	excludeSchemas                []string
	includeSchemas                []string
	entryHandler                  loki.EntryHandler
	registry                      *prometheus.Registry
	disableQueryRedaction         bool
	autoEnableSetupConsumers      bool
	setupConsumersCheckInterval   time.Duration
	sampleMinDuration             time.Duration
	waitEventMinDuration          time.Duration
	waitEventCounter              *prometheus.CounterVec
	enablePreClassifiedWaitEvents bool

	logger  *slog.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	timerBookmark          float64
	timerBookmarkOverflows int
	lastUptime             float64
}

// timerWindow is the range of the performance_schema timer that a poll collects statements from: after the
// bookmark, up to and including limit.
type timerWindow struct {
	clause    string  // endOfTimeline, or beginningAndEndOfTimeline when the window spans a timer overflow
	limit     float64 // timer value in picoseconds since the last overflow
	overflows int     // number of timer overflows before limit
}

// overflowsBefore returns the number of timer overflows before a statement in the window ended.
func (w timerWindow) overflowsBefore(timerEnd float64) int {
	if w.clause == beginningAndEndOfTimeline && timerEnd > w.limit {
		// the statement ended before the overflow that the window spans
		return w.overflows - 1
	}
	return w.overflows
}

func NewQuerySamples(args QuerySamplesArguments) (*QuerySamples, error) {
	c := &QuerySamples{
		dbConnection:                  args.DB,
		engineVersion:                 args.EngineVersion,
		collectInterval:               args.CollectInterval,
		excludeSchemas:                args.ExcludeSchemas,
		includeSchemas:                args.IncludeSchemas,
		entryHandler:                  args.EntryHandler,
		disableQueryRedaction:         args.DisableQueryRedaction,
		autoEnableSetupConsumers:      args.AutoEnableSetupConsumers,
		setupConsumersCheckInterval:   args.SetupConsumersCheckInterval,
		sampleMinDuration:             args.SampleMinDuration,
		waitEventMinDuration:          args.WaitEventMinDuration,
		enablePreClassifiedWaitEvents: args.EnablePreClassifiedWaitEvents,
		logger:                        args.Logger.With("collector", QuerySamplesCollector),
		running:                       &atomic.Bool{},
	}

	if args.EnablePreClassifiedWaitEvents && args.Registry != nil {
		c.registry = args.Registry
		c.waitEventCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "database_observability",
			Name:      "wait_event_seconds_total",
			Help:      "Total duration of wait events in seconds.",
		}, []string{"digest", "schema"})
		args.Registry.MustRegister(c.waitEventCounter)
	}

	return c, nil
}

func (c *QuerySamples) Name() string {
	return QuerySamplesCollector
}

func (c *QuerySamples) Start(ctx context.Context) error {
	if c.disableQueryRedaction {
		c.logger.Warn("collector started with query redaction disabled. SQL text in query samples may include query parameters.")
	} else {
		c.logger.Debug("collector started")
	}

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	if err := c.initializeBookmark(c.ctx); err != nil {
		return fmt.Errorf("failed to initialize bookmark: %w", err)
	}

	// Start setup_consumers check goroutine if enabled
	if c.autoEnableSetupConsumers {
		c.wg.Go(c.runSetupConsumersCheck)
	}

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			if err := c.fetchQuerySamples(c.ctx); err != nil {
				c.logger.Error("collector error", "err", err)
			}

			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	})

	return nil
}

func (c *QuerySamples) Stopped() bool {
	return !c.running.Load()
}

func (c *QuerySamples) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	if c.registry != nil && c.waitEventCounter != nil {
		c.registry.Unregister(c.waitEventCounter)
	}
}

func (c *QuerySamples) runSetupConsumersCheck() {
	ticker := time.NewTicker(c.setupConsumersCheckInterval)
	defer ticker.Stop()

	for {
		if err := c.updateSetupConsumersSettings(c.ctx); err != nil {
			c.logger.Error("error with performance_schema.setup_consumers check", "err", err)
		}

		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// continue loop
		}
	}
}

// initializeBookmark sets the bookmark to the latest performance_schema timer value, so that upon startup we don't
// collect samples that may have been collected previously, nor skip ones that end right after startup.
func (c *QuerySamples) initializeBookmark(ctx context.Context) error {
	rs := c.dbConnection.QueryRowContext(ctx, selectUptime)

	var uptime float64
	err := rs.Scan(&uptime)
	if err != nil {
		return fmt.Errorf("failed to scan uptime: %w", err)
	}

	c.lastUptime = uptime

	// Look for the latest timer value from somewhat before UPTIME, as the timer can be behind it. If no statement
	// has ended since then, nothing before startup is left to skip.
	since := max(uptime-timerCeilingMarginSeconds, 0)
	c.timerBookmark = uptimeSinceOverflow(since)
	c.timerBookmarkOverflows = calculateNumberOfOverflows(since)

	window, err := c.determineTimerWindow(ctx, uptime)
	if err != nil {
		return err
	}
	if window != nil {
		c.timerBookmark = window.limit
		c.timerBookmarkOverflows = window.overflows
	}
	return nil
}

func (c *QuerySamples) fetchQuerySamples(ctx context.Context) error {
	timeRow := c.dbConnection.QueryRowContext(ctx, selectNowAndUptime)

	var now, uptime float64
	if err := timeRow.Scan(&now, &uptime); err != nil {
		return fmt.Errorf("failed to scan now and uptime info: %w", err)
	}

	if uptime < c.lastUptime {
		// server has restarted
		c.timerBookmark = 0
		c.timerBookmarkOverflows = 0
	}

	window, err := c.determineTimerWindow(ctx, uptime)
	if err != nil {
		return err
	}
	if window == nil {
		// no statement has ended since the bookmark
		c.lastUptime = uptime
		return nil
	}

	schemaFilterClause := buildSchemaFilterClause(c.excludeSchemas, c.includeSchemas, "statements.CURRENT_SCHEMA")

	var waitDurationClause, nestedWaitDurationClause string
	if c.waitEventMinDuration > 0 {
		minPs := secondsToPicoseconds(c.waitEventMinDuration.Seconds())
		waitDurationClause = fmt.Sprintf("AND waits.timer_wait >= %.0f", minPs)
		nestedWaitDurationClause = fmt.Sprintf("AND nested_waits.timer_wait >= %.0f", minPs)
	}

	var sampleDurationClause string
	if c.sampleMinDuration > 0 {
		sampleDurationClause = fmt.Sprintf("AND statements.TIMER_WAIT >= %.0f", secondsToPicoseconds(c.sampleMinDuration.Seconds()))
	}

	query := ""
	if semver.MustParseRange("<8.0.28")(c.engineVersion) {
		query = fmt.Sprintf(selectQuerySamples, "", waitDurationClause, nestedWaitDurationClause, schemaFilterClause, sampleDurationClause, window.clause)
	} else if semver.MustParseRange("<8.0.31")(c.engineVersion) {
		query = fmt.Sprintf(selectQuerySamples, cpuTimeField, waitDurationClause, nestedWaitDurationClause, schemaFilterClause, sampleDurationClause, window.clause)
	} else {
		additionalFields := cpuTimeField + maxControlledMemoryField + maxTotalMemoryField
		query = fmt.Sprintf(selectQuerySamples, additionalFields, waitDurationClause, nestedWaitDurationClause, schemaFilterClause, sampleDurationClause, window.clause)
	}

	rs, err := c.dbConnection.QueryContext(ctx, query, c.timerBookmark, window.limit)
	if err != nil {
		return fmt.Errorf("failed to fetch query samples: %w", err)
	}
	defer rs.Close()

	// set the new bookmarks
	c.timerBookmark = window.limit
	c.timerBookmarkOverflows = window.overflows
	c.lastUptime = uptime

	lastThreadIDLogged := ""
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

			// sample error info
			MysqlErrno       sql.NullInt64
			ReturnedSQLState sql.NullString
			MessageText      sql.NullString

			// sample memory info
			MaxControlledMemory uint64
			MaxTotalMemory      uint64

			// wait event info
			WaitEventID    sql.NullString
			WaitEndEventID sql.NullString
			WaitEventName  sql.NullString
			WaitObjectName sql.NullString
			WaitObjectType sql.NullString
			WaitTime       sql.NullFloat64

			// nested wait event info (when outer event is wait/io/table/sql/handler)
			NestedWaitEventID    sql.NullString
			NestedWaitEndEventID sql.NullString
			NestedWaitEventName  sql.NullString
			NestedWaitObjectName sql.NullString
			NestedWaitObjectType sql.NullString
			NestedWaitTime       sql.NullFloat64

			// user and host who issued the query
			User sql.NullString
			Host sql.NullString
		}{}

		scanArgs := []any{
			&row.Schema,
			&row.ThreadID,
			&row.StatementEventID,
			&row.StatementEndEventID,
			&row.Digest,
			&row.SQLText,
			&row.TimerEndPicoseconds,
			&row.ElapsedTimePicoseconds,
			&row.RowsExamined,
			&row.RowsSent,
			&row.RowsAffected,
			&row.Errors,
			&row.MysqlErrno,
			&row.ReturnedSQLState,
			&row.MessageText,
			&row.WaitEventID,
			&row.WaitEndEventID,
			&row.WaitEventName,
			&row.WaitObjectName,
			&row.WaitObjectType,
			&row.WaitTime,
			&row.NestedWaitEventID,
			&row.NestedWaitEndEventID,
			&row.NestedWaitEventName,
			&row.NestedWaitObjectName,
			&row.NestedWaitObjectType,
			&row.NestedWaitTime,
			&row.User,
			&row.Host,
		}

		if semver.MustParseRange(">=8.0.28")(c.engineVersion) {
			scanArgs = append(scanArgs, &row.CPUTime)
		}
		if semver.MustParseRange(">=8.0.31")(c.engineVersion) {
			scanArgs = append(scanArgs, &row.MaxControlledMemory)
			scanArgs = append(scanArgs, &row.MaxTotalMemory)
		}

		err := rs.Scan(scanArgs...)
		if err != nil {
			c.logger.Error("failed to scan history table samples", "err", err)
			continue
		}

		if !row.TimerEndPicoseconds.Valid {
			c.logger.Debug("skipping query with invalid timer end timestamp", "schema", row.Schema.String, "digest", row.Digest.String, "timer_end", row.TimerEndPicoseconds.Float64)
			continue
		}

		serverStartTime := now - uptime
		endMilliseconds := calculateWallTime(serverStartTime, row.TimerEndPicoseconds.Float64, window.overflowsBefore(row.TimerEndPicoseconds.Float64))
		elapsedTime := picosecondsToMilliseconds(row.ElapsedTimePicoseconds.Float64)
		row.TimestampMilliseconds = endMilliseconds - elapsedTime
		cpuTime := picosecondsToMilliseconds(row.CPUTime)
		traceParent := database_observability.TryExtractTraceParent(row.SQLText.String)

		logMessage := fmt.Sprintf(
			`schema=%s user=%s client_host=%s thread_id=%s event_id=%s end_event_id=%s digest=%s rows_examined="%d" rows_sent="%d" rows_affected="%d" errors="%d" mysql_errno="%d" returned_sqlstate=%s max_controlled_memory="%db" max_total_memory="%db" cpu_time="%fms" elapsed_time="%fms" elapsed_time_ms="%fms"`,
			strconv.Quote(row.Schema.String), strconv.Quote(row.User.String), strconv.Quote(row.Host.String), strconv.Quote(row.ThreadID.String),
			strconv.Quote(row.StatementEventID.String), strconv.Quote(row.StatementEndEventID.String),
			strconv.Quote(row.Digest.String),
			row.RowsExamined,
			row.RowsSent,
			row.RowsAffected,
			row.Errors,
			row.MysqlErrno.Int64,
			strconv.Quote(row.ReturnedSQLState.String),
			row.MaxControlledMemory,
			row.MaxTotalMemory,
			cpuTime,
			elapsedTime,
			elapsedTime,
		)
		if traceParent != "" {
			logMessage += fmt.Sprintf(` traceparent=%s`, strconv.Quote(traceParent))
		}
		if c.disableQueryRedaction && row.SQLText.Valid {
			logMessage += fmt.Sprintf(` sql_text=%s`, strconv.Quote(row.SQLText.String))
		}
		// message_text can embed literal values (e.g. a duplicate key value), so
		// it is only emitted when query redaction is disabled, like sql_text.
		if c.disableQueryRedaction && row.MessageText.Valid {
			logMessage += fmt.Sprintf(` message_text=%s`, strconv.Quote(row.MessageText.String))
		}

		if lastThreadIDLogged != row.ThreadID.String || lastDigestLogged != row.Digest.String || lastEventIDLogged != row.StatementEventID.String {
			lastThreadIDLogged = row.ThreadID.String
			lastDigestLogged = row.Digest.String
			lastEventIDLogged = row.StatementEventID.String

			c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
				logging.LevelInfo,
				database_observability.OP_QUERY_SAMPLE,
				logMessage,
				int64(millisecondsToNanoseconds(row.TimestampMilliseconds)),
			)
		}

		if row.WaitEventID.Valid && row.WaitTime.Valid {
			eventID := row.WaitEventID.String
			endEventID := row.WaitEndEventID.String
			eventName := row.WaitEventName.String
			objectName := row.WaitObjectName.String
			objectType := row.WaitObjectType.String
			waitTime := row.WaitTime.Float64

			// wait/io/table/sql/handler is a molecule event wrapping the actual I/O;
			// the SQL JOIN populates nested_waits.* only for that outer name, so a
			// valid nested row means we should surface its fields instead.
			// See https://dev.mysql.com/doc/refman/8.0/en/performance-schema-atom-molecule-events.html
			if row.NestedWaitEventID.Valid && row.NestedWaitTime.Valid {
				eventID = row.NestedWaitEventID.String
				endEventID = row.NestedWaitEndEventID.String
				eventName = row.NestedWaitEventName.String
				objectName = row.NestedWaitObjectName.String
				objectType = row.NestedWaitObjectType.String
				waitTime = row.NestedWaitTime.Float64
			}

			waitTimeMs := picosecondsToMilliseconds(waitTime)

			if c.enablePreClassifiedWaitEvents {
				waitV2LogMessage := fmt.Sprintf(
					`schema=%s user=%s client_host=%s thread_id=%s digest=%s event_id=%s wait_event_id=%s wait_end_event_id=%s wait_event_name=%s wait_event_type=%s wait_object_name=%s wait_object_type=%s wait_time="%fms"`,
					strconv.Quote(row.Schema.String),
					strconv.Quote(row.User.String),
					strconv.Quote(row.Host.String),
					strconv.Quote(row.ThreadID.String),
					strconv.Quote(row.Digest.String),
					strconv.Quote(row.StatementEventID.String),
					strconv.Quote(eventID),
					strconv.Quote(endEventID),
					strconv.Quote(eventName),
					strconv.Quote(classifyMySQLWaitEventType(eventName)),
					strconv.Quote(objectName),
					strconv.Quote(objectType),
					waitTimeMs,
				)
				c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
					logging.LevelInfo,
					database_observability.OP_WAIT_EVENT_V2,
					waitV2LogMessage,
					int64(millisecondsToNanoseconds(row.TimestampMilliseconds)),
				)

				if c.waitEventCounter != nil {
					c.waitEventCounter.WithLabelValues(row.Digest.String, row.Schema.String).Add(picosecondsToSeconds(waitTime))
				}
			} else {
				waitLogMessage := fmt.Sprintf(
					`schema=%s user=%s client_host=%s thread_id=%s digest=%s event_id=%s wait_event_id=%s wait_end_event_id=%s wait_event_name=%s wait_object_name=%s wait_object_type=%s wait_time="%fms"`,
					strconv.Quote(row.Schema.String),
					strconv.Quote(row.User.String),
					strconv.Quote(row.Host.String),
					strconv.Quote(row.ThreadID.String),
					strconv.Quote(row.Digest.String),
					strconv.Quote(row.StatementEventID.String),
					strconv.Quote(eventID),
					strconv.Quote(endEventID),
					strconv.Quote(eventName),
					strconv.Quote(objectName),
					strconv.Quote(objectType),
					waitTimeMs,
				)
				c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
					logging.LevelInfo,
					database_observability.OP_WAIT_EVENT,
					waitLogMessage,
					int64(millisecondsToNanoseconds(row.TimestampMilliseconds)),
				)
			}
		}
	}

	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate over samples result set: %w", err)
	}

	return nil
}

func (c *QuerySamples) updateSetupConsumersSettings(ctx context.Context) error {
	rs, err := c.dbConnection.ExecContext(ctx, updateSetupConsumers)
	if err != nil {
		return fmt.Errorf("failed to update performance_schema.setup_consumers: %w", err)
	}

	rowsAffected, err := rs.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected from performance_schema.setup_consumers: %w", err)
	}
	c.logger.Debug("updated performance_schema.setup_consumers", "rows_affected", rowsAffected)

	return nil
}

// determineTimerWindow returns the window of the performance_schema timer to collect statements from, or nil if
// no statement has ended since the bookmark.
//
// The window ends at the latest timer value read from events_statements_current, not at UPTIME. UPTIME is in
// whole seconds and its origin differs from the timer's by a fraction of a second, so a limit derived from it
// lands up to a second before or after the moment of the poll. Too early, and statements that have already ended
// wait for a later poll, by when their rows are gone if their client has disconnected. Too late, and statements
// that end after the poll but before the limit are not in events_statements_history yet, and the next window
// starts after them.
func (c *QuerySamples) determineTimerWindow(ctx context.Context, uptime float64) (*timerWindow, error) {
	// No statement can have ended after the ceiling.
	ceiling := uptimeSinceOverflow(uptime + timerCeilingMarginSeconds)
	ceilingOverflows := calculateNumberOfOverflows(uptime + timerCeilingMarginSeconds)

	var latest sql.NullFloat64
	var err error
	if ceilingOverflows > c.timerBookmarkOverflows {
		err = c.dbConnection.QueryRowContext(ctx, selectLatestTimerEndAcrossOverflow, ceiling, c.timerBookmark).Scan(&latest)
	} else {
		err = c.dbConnection.QueryRowContext(ctx, selectLatestTimerEnd, c.timerBookmark, ceiling).Scan(&latest)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest statement timer: %w", err)
	}
	if !latest.Valid {
		return nil, nil
	}

	window := &timerWindow{clause: endOfTimeline, limit: latest.Float64, overflows: ceilingOverflows}
	if latest.Float64 > ceiling {
		// the latest statement ended before the overflow that the ceiling is past
		window.overflows--
	}
	if window.overflows > c.timerBookmarkOverflows {
		// the timer has overflowed since the bookmark: collect both the end and the beginning of the timeline
		window.clause = beginningAndEndOfTimeline
	}
	return window, nil
}

var mysqlSynchReplicationSymbolPrefixes = []string{
	"Slave_",
	"Replica_",
	"Master_info",
	"Source_info",
	"Relay_log_info",
	"MYSQL_RELAY_LOG",
	"Mts_",
}

func isMySQLReplicationWaitEvent(name string) bool {
	if strings.HasPrefix(name, "wait/io/file/sql/relaylog") {
		return true
	}
	rest, ok := strings.CutPrefix(name, "wait/synch/")
	if !ok {
		return false
	}
	// rest is "<primitive>/<owner>/<symbol>", e.g. "mutex/sql/Slave_jobs_lock".
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 3 {
		return false
	}
	if parts[1] != "sql" {
		return false
	}
	for _, p := range mysqlSynchReplicationSymbolPrefixes {
		if strings.HasPrefix(parts[2], p) {
			return true
		}
	}
	return false
}

func classifyMySQLWaitEventType(waitEventName string) string {
	if isMySQLReplicationWaitEvent(waitEventName) {
		return database_observability.WAIT_EVENT_TYPE_REPLICATION
	}
	rest, ok := strings.CutPrefix(waitEventName, "wait/")
	if !ok {
		return database_observability.WAIT_EVENT_TYPE_OTHER
	}
	switch {
	case strings.HasPrefix(rest, "io/file/"), strings.HasPrefix(rest, "io/table/"):
		return database_observability.WAIT_EVENT_TYPE_IO
	case strings.HasPrefix(rest, "io/socket/"):
		return database_observability.WAIT_EVENT_TYPE_NETWORK
	case strings.HasPrefix(rest, "io/lock/"), strings.HasPrefix(rest, "lock/"):
		return database_observability.WAIT_EVENT_TYPE_LOCK
	case strings.HasPrefix(rest, "synch/"):
		return database_observability.WAIT_EVENT_TYPE_ENGINE
	}
	return database_observability.WAIT_EVENT_TYPE_OTHER
}
