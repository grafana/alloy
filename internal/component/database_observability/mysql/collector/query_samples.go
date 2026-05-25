package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
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
	OP_WAIT_EVENT_V2      = "wait_event_v2"

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
	EntryHandler                  loki.EntryHandler
	Registry                      *prometheus.Registry
	DisableQueryRedaction         bool
	AutoEnableSetupConsumers      bool
	SetupConsumersCheckInterval   time.Duration
	SampleMinDuration             time.Duration
	WaitEventMinDuration          time.Duration
	EnablePreClassifiedWaitEvents bool

	Logger log.Logger
}

type QuerySamples struct {
	dbConnection                  *sql.DB
	engineVersion                 semver.Version
	collectInterval               time.Duration
	excludeSchemas                []string
	entryHandler                  loki.EntryHandler
	registry                      *prometheus.Registry
	disableQueryRedaction         bool
	autoEnableSetupConsumers      bool
	setupConsumersCheckInterval   time.Duration
	sampleMinDuration             time.Duration
	waitEventMinDuration          time.Duration
	waitEventCounter              *prometheus.CounterVec
	enablePreClassifiedWaitEvents bool

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
		dbConnection:                  args.DB,
		engineVersion:                 args.EngineVersion,
		collectInterval:               args.CollectInterval,
		excludeSchemas:                args.ExcludeSchemas,
		entryHandler:                  args.EntryHandler,
		disableQueryRedaction:         args.DisableQueryRedaction,
		autoEnableSetupConsumers:      args.AutoEnableSetupConsumers,
		setupConsumersCheckInterval:   args.SetupConsumersCheckInterval,
		sampleMinDuration:             args.SampleMinDuration,
		waitEventMinDuration:          args.WaitEventMinDuration,
		enablePreClassifiedWaitEvents: args.EnablePreClassifiedWaitEvents,
		logger:                        log.With(args.Logger, "collector", QuerySamplesCollector),
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
		c.wg.Go(c.runSetupConsumersCheck)
	}

	c.wg.Go(func() {
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

	excludedSchemasClause := buildExcludedSchemasClause(c.excludeSchemas)

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
		query = fmt.Sprintf(selectQuerySamples, "", waitDurationClause, nestedWaitDurationClause, excludedSchemasClause, sampleDurationClause, timerClause)
	} else if semver.MustParseRange("<8.0.31")(c.engineVersion) {
		query = fmt.Sprintf(selectQuerySamples, cpuTimeField, waitDurationClause, nestedWaitDurationClause, excludedSchemasClause, sampleDurationClause, timerClause)
	} else {
		additionalFields := cpuTimeField + maxControlledMemoryField + maxTotalMemoryField
		query = fmt.Sprintf(selectQuerySamples, additionalFields, waitDurationClause, nestedWaitDurationClause, excludedSchemasClause, sampleDurationClause, timerClause)
	}

	rs, err := c.dbConnection.QueryContext(ctx, query, c.timerBookmark, limit)
	if err != nil {
		return fmt.Errorf("failed to fetch query samples: %w", err)
	}
	defer rs.Close()

	// set the new bookmarks
	c.timerBookmark = limit
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
		traceParent := tryExtractTraceParent(row.SQLText.String)

		logMessage := fmt.Sprintf(
			`schema="%s" user="%s" client_host="%s" thread_id="%s" event_id="%s" end_event_id="%s" digest="%s" rows_examined="%d" rows_sent="%d" rows_affected="%d" errors="%d" max_controlled_memory="%db" max_total_memory="%db" cpu_time="%fms" elapsed_time="%fms" elapsed_time_ms="%fms"`,
			row.Schema.String, row.User.String, row.Host.String, row.ThreadID.String,
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
		if traceParent != "" {
			logMessage += fmt.Sprintf(` traceparent="%s"`, traceParent)
		}
		if c.disableQueryRedaction && row.SQLText.Valid {
			logMessage += fmt.Sprintf(` sql_text="%s"`, row.SQLText.String)
		}

		if lastThreadIDLogged != row.ThreadID.String || lastDigestLogged != row.Digest.String || lastEventIDLogged != row.StatementEventID.String {
			lastThreadIDLogged = row.ThreadID.String
			lastDigestLogged = row.Digest.String
			lastEventIDLogged = row.StatementEventID.String

			c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
				logging.LevelInfo,
				OP_QUERY_SAMPLE,
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
					`schema="%s" user="%s" client_host="%s" thread_id="%s" digest="%s" event_id="%s" wait_event_id="%s" wait_end_event_id="%s" wait_event_name="%s" wait_event_type="%s" wait_object_name="%s" wait_object_type="%s" wait_time="%fms"`,
					row.Schema.String,
					row.User.String,
					row.Host.String,
					row.ThreadID.String,
					row.Digest.String,
					row.StatementEventID.String,
					eventID,
					endEventID,
					eventName,
					classifyMySQLWaitEventType(eventName),
					objectName,
					objectType,
					waitTimeMs,
				)
				c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
					logging.LevelInfo,
					OP_WAIT_EVENT_V2,
					waitV2LogMessage,
					int64(millisecondsToNanoseconds(row.TimestampMilliseconds)),
				)

				if c.waitEventCounter != nil {
					c.waitEventCounter.WithLabelValues(row.Digest.String, row.Schema.String).Add(picosecondsToSeconds(waitTime))
				}
			} else {
				waitLogMessage := fmt.Sprintf(
					`schema="%s" user="%s" client_host="%s" thread_id="%s" digest="%s" event_id="%s" wait_event_id="%s" wait_end_event_id="%s" wait_event_name="%s" wait_object_name="%s" wait_object_type="%s" wait_time="%fms"`,
					row.Schema.String,
					row.User.String,
					row.Host.String,
					row.ThreadID.String,
					row.Digest.String,
					row.StatementEventID.String,
					eventID,
					endEventID,
					eventName,
					objectName,
					objectType,
					waitTimeMs,
				)
				c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
					logging.LevelInfo,
					OP_WAIT_EVENT,
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
		return "Replication Wait"
	}
	rest, ok := strings.CutPrefix(waitEventName, "wait/")
	if !ok {
		return "Other Wait"
	}
	switch {
	case strings.HasPrefix(rest, "io/file/"), strings.HasPrefix(rest, "io/table/"):
		return "IO Wait"
	case strings.HasPrefix(rest, "io/socket/"):
		return "Network Wait"
	case strings.HasPrefix(rest, "io/lock/"), strings.HasPrefix(rest, "lock/"):
		return "Lock Wait"
	case strings.HasPrefix(rest, "synch/"):
		return "Engine Wait"
	}
	return "Other Wait"
}

// tryExtractTraceParent attempts to extract a W3C traceparent value added at the end of SQL text as a trailing
// block comment, e.g. "/*traceparent='00-<traceid>-<spanid>-<flags>'*/".
// It returns the traceparent string when matched, otherwise an empty string.
func tryExtractTraceParent(sqlText string) string {
	if strings.HasSuffix(sqlText, "...") {
		return ""
	}

	// Find the last comment: strip out /* and */
	start := strings.LastIndex(sqlText, "/*")
	if start < 0 {
		return ""
	}
	body := sqlText[start+2:]
	end := strings.Index(body, "*/")
	if end < 0 {
		return ""
	}

	body = body[:end]
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	// Split the comment by comma into key value pairs
	pairs := strings.Split(body, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		key, val, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}

		if !strings.EqualFold(strings.TrimSpace(key), "traceparent") {
			continue
		}

		// SQL unescape: trim ' or " at beginning and end of value
		if strings.HasPrefix(val, "'") || strings.HasPrefix(val, `"`) {
			quote := string(val[0])
			val = strings.TrimPrefix(val, quote)
			val = strings.TrimSuffix(val, quote)
		}

		return strings.TrimSpace(val)
	}

	return ""
}
