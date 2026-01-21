package collector

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	QuerySamplesCollector = "query_samples"
	OP_QUERY_SAMPLE       = "query_sample"
	OP_WAIT_EVENT         = "wait_event"

	cpuTimeField              = `, statements.CPU_TIME`
	maxControlledMemoryField  = `, statements.MAX_CONTROLLED_MEMORY`
	maxTotalMemoryField       = `, statements.MAX_TOTAL_MEMORY`
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

const selectQuerySamples = `
SELECT
	statements.CURRENT_SCHEMA,
	statements.THREAD_ID,
	statements.EVENT_ID,
	statements.END_EVENT_ID,
	statements.DIGEST,
	statements.TIMER_END,
	statements.TIMER_WAIT,
	statements.ROWS_EXAMINED,
	statements.ROWS_SENT,
	statements.ROWS_AFFECTED,
	statements.ERRORS,
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
	AND statements.CURRENT_SCHEMA NOT IN ` + EXCLUDED_SCHEMAS +
	` %s %s`

const updateSetupConsumers = `
	UPDATE performance_schema.setup_consumers
		SET enabled = 'yes'
		WHERE name in ('events_statements_cpu', 'events_waits_current', 'events_waits_history')`

type QuerySamplesArguments struct {
	DB                          *sql.DB
	EngineVersion               semver.Version
	CollectInterval             time.Duration
	EntryHandler                loki.EntryHandler
	DisableQueryRedaction       bool
	AutoEnableSetupConsumers    bool
	SetupConsumersCheckInterval time.Duration

	Logger log.Logger
}

type QuerySamples struct {
	dbConnection                *sql.DB
	engineVersion               semver.Version
	collectInterval             time.Duration
	entryHandler                loki.EntryHandler
	disableQueryRedaction       bool
	autoEnableSetupConsumers    bool
	setupConsumersCheckInterval time.Duration

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	timerBookmark float64
	lastUptime    float64
}

func NewQuerySamples(args QuerySamplesArguments) (*QuerySamples, error) {
	c := &QuerySamples{
		dbConnection:                args.DB,
		engineVersion:               args.EngineVersion,
		collectInterval:             args.CollectInterval,
		entryHandler:                args.EntryHandler,
		disableQueryRedaction:       args.DisableQueryRedaction,
		autoEnableSetupConsumers:    args.AutoEnableSetupConsumers,
		setupConsumersCheckInterval: args.SetupConsumersCheckInterval,
		logger:                      log.With(args.Logger, "collector", QuerySamplesCollector),
		running:                     &atomic.Bool{},
	}

	return c, nil
}

func (c *QuerySamples) Name() string {
	return QuerySamplesCollector
}

func (c *QuerySamples) Start(ctx context.Context) error {
	if c.disableQueryRedaction {
		level.Warn(c.logger).Log("msg", "collector started with query redaction disabled. SQL text in query samples may include query parameters.")
	} else {
		level.Debug(c.logger).Log("msg", "collector started")
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
		c.wg.Add(1)
		go c.runSetupConsumersCheck()
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

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

func (c *QuerySamples) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *QuerySamples) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *QuerySamples) runSetupConsumersCheck() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.setupConsumersCheckInterval)
	defer ticker.Stop()

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
func (c *QuerySamples) initializeBookmark(ctx context.Context) error {
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

func (c *QuerySamples) fetchQuerySamples(ctx context.Context) error {
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

	query := ""
	if semver.MustParseRange("<8.0.28")(c.engineVersion) {
		query = fmt.Sprintf(selectQuerySamples, textField, textNotNullClause, timerClause)
	} else if semver.MustParseRange("<8.0.31")(c.engineVersion) {
		additionalFields := cpuTimeField + textField
		query = fmt.Sprintf(selectQuerySamples, additionalFields, textNotNullClause, timerClause)
	} else {
		additionalFields := cpuTimeField + maxControlledMemoryField + maxTotalMemoryField + textField
		query = fmt.Sprintf(selectQuerySamples, additionalFields, textNotNullClause, timerClause)
	}

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
			&row.TimerEndPicoseconds,
			&row.ElapsedTimePicoseconds,
			&row.RowsExamined,
			&row.RowsSent,
			&row.RowsAffected,
			&row.Errors,
			&row.WaitEventID,
			&row.WaitEndEventID,
			&row.WaitEventName,
			&row.WaitObjectName,
			&row.WaitObjectType,
			&row.WaitTime,
		}

		if semver.MustParseRange(">=8.0.28")(c.engineVersion) {
			scanArgs = append(scanArgs, &row.CPUTime)
		}
		if semver.MustParseRange(">=8.0.31")(c.engineVersion) {
			scanArgs = append(scanArgs, &row.MaxControlledMemory)
			scanArgs = append(scanArgs, &row.MaxTotalMemory)
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
		cpuTime := picosecondsToMilliseconds(row.CPUTime)
		elapsedTime := picosecondsToMilliseconds(row.ElapsedTimePicoseconds.Float64)

		logMessage := fmt.Sprintf(
			`schema="%s" thread_id="%s" event_id="%s" end_event_id="%s" digest="%s" rows_examined="%d" rows_sent="%d" rows_affected="%d" errors="%d" max_controlled_memory="%db" max_total_memory="%db" cpu_time="%fms" elapsed_time="%fms" elapsed_time_ms="%fms"`,
			row.Schema.String, row.ThreadID.String,
			row.StatementEventID.String, row.StatementEndEventID.String,
			row.Digest.String,
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
				`schema="%s" thread_id="%s" digest="%s" event_id="%s" wait_event_id="%s" wait_end_event_id="%s" wait_event_name="%s" wait_object_name="%s" wait_object_type="%s" wait_time="%fms"`,
				row.Schema.String,
				row.ThreadID.String,
				row.Digest.String,
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
	level.Debug(c.logger).Log("msg", "updated performance_schema.setup_consumers", "rows_affected", rowsAffected)

	return nil
}

func (c *QuerySamples) determineTimerClauseAndLimit(uptime float64) (string, float64) {
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
