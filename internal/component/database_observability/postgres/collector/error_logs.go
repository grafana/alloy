package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const ErrorLogsCollector = "error_logs"
const OP_ERROR_LOGS = "error_logs"

// Supported error severities that will be processed
var supportedSeverities = map[string]bool{
	"ERROR": true,
	"FATAL": true,
	"PANIC": true,
}

// PostgreSQLJSONLog represents the structure of a PostgreSQL JSON log entry.
// See: https://www.postgresql.org/docs/current/runtime-config-logging.html
type PostgreSQLJSONLog struct {
	// Core identification
	Timestamp string `json:"timestamp"`
	PID       int32  `json:"pid"`
	SessionID string `json:"session_id"`
	LineNum   int32  `json:"line_num"`

	// User/Database context
	User   *string `json:"user"`
	DBName *string `json:"dbname"`

	// Client information
	RemoteHost      *string `json:"remote_host"`
	RemotePort      *int32  `json:"remote_port"`
	ApplicationName *string `json:"application_name"`

	// Session/Process info
	PS           *string `json:"ps"` // Current ps display
	SessionStart string  `json:"session_start"`
	BackendType  string  `json:"backend_type"`

	// Transaction information
	VXID *string `json:"vxid"` // Format: "3/1234"
	TXID *int64  `json:"txid"`

	// Error/Log information
	ErrorSeverity string  `json:"error_severity"`
	SqlState      *string `json:"state_code"`
	Message       string  `json:"message"`
	Detail        *string `json:"detail"`
	Hint          *string `json:"hint"`
	Context       *string `json:"context"`

	// Query information
	Statement      *string `json:"statement"`
	CursorPosition *int32  `json:"cursor_position"`
	QueryID        *int64  `json:"query_id"`

	// Internal query (for errors in functions/procedures)
	InternalQuery    *string `json:"internal_query"`
	InternalPosition *int32  `json:"internal_position"`

	// Error location (for internal errors)
	FuncName    *string `json:"func_name"`
	FileName    *string `json:"file_name"`
	FileLineNum *int32  `json:"file_line_num"`

	// Parallel query support
	LeaderPID *int32 `json:"leader_pid"` // PID of leader for active parallel workers
}

type ParsedError struct {
	Timestamp time.Time
	PID       int32
	SessionID string
	LineNum   int32

	ErrorSeverity string
	SQLState      string
	ErrorName     string
	SQLStateClass string
	ErrorCategory string

	User            string
	DatabaseName    string
	RemoteHost      string
	RemotePort      int32
	ApplicationName string
	BackendType     string
	PS              string

	SessionStart time.Time
	VXID         string
	TXID         string

	Message string
	Detail  string
	Hint    string
	Context string

	Statement      string
	CursorPosition int32
	QueryID        int64

	InternalQuery    string
	InternalPosition int32

	FuncName    string
	FileName    string
	FileLineNum int32

	LeaderPID int32
}

type ErrorLogsArguments struct {
	Receiver              loki.LogsReceiver
	EntryHandler          loki.EntryHandler
	Logger                log.Logger
	InstanceKey           string
	SystemID              string
	Registry              *prometheus.Registry
	DisableQueryRedaction bool
}

type ErrorLogs struct {
	logger                log.Logger
	entryHandler          loki.EntryHandler
	instanceKey           string
	systemID              string
	registry              *prometheus.Registry
	disableQueryRedaction bool

	receiver loki.LogsReceiver

	logsProcessed       prometheus.Counter
	errorsTotal         *prometheus.CounterVec
	errorsBySQLState    *prometheus.CounterVec
	errorsByBackendType *prometheus.CounterVec
	parseErrors         prometheus.Counter

	ctx     context.Context
	cancel  context.CancelFunc
	stopped *atomic.Bool
	wg      sync.WaitGroup
}

func NewErrorLogs(args ErrorLogsArguments) (*ErrorLogs, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &ErrorLogs{
		logger:                log.With(args.Logger, "collector", ErrorLogsCollector),
		entryHandler:          args.EntryHandler,
		instanceKey:           args.InstanceKey,
		systemID:              args.SystemID,
		registry:              args.Registry,
		disableQueryRedaction: args.DisableQueryRedaction,
		receiver:              args.Receiver,
		ctx:                   ctx,
		cancel:                cancel,
		stopped:               atomic.NewBool(false),
	}

	e.initMetrics()

	return e, nil
}

func (c *ErrorLogs) initMetrics() {
	c.logsProcessed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "postgres_error_logs_processed_total",
			Help: "Total number of log lines processed",
		},
	)

	c.errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_errors_total",
			Help: "Total PostgreSQL errors by severity and database",
		},
		[]string{"severity", "database", "instance"},
	)

	c.errorsBySQLState = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_errors_by_sqlstate_query_user_total",
			Help: "PostgreSQL errors by SQLSTATE code with database, user, queryid, and instance tracking",
		},
		[]string{"sqlstate", "error_name", "sqlstate_class", "error_category", "severity", "database", "user", "queryid", "instance"},
	)

	c.errorsByBackendType = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_errors_by_backend_type_total",
			Help: "Errors by backend type and database",
		},
		[]string{"backend_type", "severity", "database", "instance"},
	)

	c.parseErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "postgres_error_log_parse_failures_total",
			Help: "Failed to parse log lines",
		},
	)

	if c.registry != nil {
		c.registry.MustRegister(
			c.logsProcessed,
			c.errorsTotal,
			c.errorsBySQLState,
			c.errorsByBackendType,
			c.parseErrors,
		)
	} else {
		level.Warn(c.logger).Log("msg", "no Prometheus registry provided, metrics will not be exposed")
	}
}

func (c *ErrorLogs) Name() string {
	return ErrorLogsCollector
}

// Receiver returns the logs receiver that loki.source.* can forward to
func (c *ErrorLogs) Receiver() loki.LogsReceiver {
	return c.receiver
}

func (c *ErrorLogs) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "collector started")

	c.wg.Add(1)
	go c.run()
	return nil
}

func (c *ErrorLogs) Stop() {
	c.cancel()
	c.stopped.Store(true)
	c.wg.Wait()
}

func (c *ErrorLogs) Stopped() bool {
	return c.stopped.Load()
}

func (c *ErrorLogs) run() {
	defer c.wg.Done()

	level.Debug(c.logger).Log("msg", "collector running, waiting for log entries")

	for {
		select {
		case <-c.ctx.Done():
			level.Debug(c.logger).Log("msg", "collector stopping")
			return
		case entry := <-c.receiver.Chan():
			c.logsProcessed.Inc()
			if err := c.processLogLine(entry); err != nil {
				level.Warn(c.logger).Log(
					"msg", "failed to process log line",
					"error", err,
					"line_preview", truncateString(entry.Entry.Line, 100),
				)
			}
		}
	}
}

func (c *ErrorLogs) processLogLine(entry loki.Entry) error {
	var jsonLog PostgreSQLJSONLog
	if err := json.Unmarshal([]byte(entry.Entry.Line), &jsonLog); err != nil {
		c.parseErrors.Inc()
		return fmt.Errorf("failed to parse JSON log line: %w", err)
	}

	if !supportedSeverities[jsonLog.ErrorSeverity] {
		return nil
	}

	parsed, err := c.buildParsedError(&jsonLog)
	if err != nil {
		return fmt.Errorf("failed to parse error: %w", err)
	}

	c.updateMetrics(parsed)

	return c.emitToLoki(parsed)
}

func (c *ErrorLogs) buildParsedError(log *PostgreSQLJSONLog) (*ParsedError, error) {
	parsed := &ParsedError{
		PID:           log.PID,
		SessionID:     log.SessionID,
		LineNum:       log.LineNum,
		ErrorSeverity: log.ErrorSeverity,
		Message:       log.Message,
		BackendType:   log.BackendType,
	}

	var err error
	// PostgreSQL jsonlog format uses pg_strftime with "%Y-%m-%d %H:%M:%S.%3f %Z"
	// Source: src/backend/utils/error/elog.c write_jsonlog()
	// Example: "2023-11-04 08:50:59.000 CET"
	const timestampFormat = "2006-01-02 15:04:05.999 MST"

	parsed.Timestamp, err = time.Parse(timestampFormat, log.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	if log.SessionStart != "" {
		parsed.SessionStart, err = time.Parse(timestampFormat, log.SessionStart)
		if err != nil {
			return nil, fmt.Errorf("failed to parse session_start timestamp: %w", err)
		}
	}

	parsed.User = ptrToString(log.User)
	parsed.DatabaseName = ptrToString(log.DBName)
	parsed.RemoteHost = ptrToString(log.RemoteHost)
	parsed.RemotePort = ptrToInt32(log.RemotePort)
	parsed.ApplicationName = ptrToString(log.ApplicationName)
	parsed.PS = ptrToString(log.PS)
	parsed.VXID = ptrToString(log.VXID)
	parsed.TXID = ptrInt64ToString(log.TXID)
	parsed.Detail = strings.TrimSpace(ptrToString(log.Detail))
	parsed.Hint = strings.TrimSpace(ptrToString(log.Hint))
	parsed.Context = strings.TrimSpace(ptrToString(log.Context))
	parsed.Statement = strings.TrimSpace(ptrToString(log.Statement))
	parsed.CursorPosition = ptrToInt32(log.CursorPosition)
	parsed.InternalQuery = strings.TrimSpace(ptrToString(log.InternalQuery))
	parsed.InternalPosition = ptrToInt32(log.InternalPosition)
	parsed.LeaderPID = ptrToInt32(log.LeaderPID)
	parsed.QueryID = ptrToInt64(log.QueryID)
	parsed.FuncName = ptrToString(log.FuncName)
	parsed.FileName = ptrToString(log.FileName)
	parsed.FileLineNum = ptrToInt32(log.FileLineNum)

	if log.SqlState != nil {
		parsed.SQLState = *log.SqlState
		parsed.ErrorName = GetSQLStateErrorName(parsed.SQLState)
		parsed.SQLStateClass = parsed.SQLState[:2]
		parsed.ErrorCategory = GetSQLStateCategory(parsed.SQLState)
	}

	return parsed, nil
}

func (c *ErrorLogs) updateMetrics(parsed *ParsedError) {
	c.errorsTotal.WithLabelValues(
		parsed.ErrorSeverity,
		parsed.DatabaseName,
		c.instanceKey,
	).Inc()

	if parsed.SQLState != "" {
		queryIDStr := ""
		if parsed.QueryID > 0 {
			queryIDStr = fmt.Sprintf("%d", parsed.QueryID)
		}

		c.errorsBySQLState.WithLabelValues(
			parsed.SQLState,
			parsed.ErrorName,
			parsed.SQLStateClass,
			parsed.ErrorCategory,
			parsed.ErrorSeverity,
			parsed.DatabaseName,
			parsed.User,
			queryIDStr,
			c.instanceKey,
		).Inc()
	}

	c.errorsByBackendType.WithLabelValues(
		parsed.BackendType,
		parsed.ErrorSeverity,
		parsed.DatabaseName,
		c.instanceKey,
	).Inc()
}

func (c *ErrorLogs) emitToLoki(parsed *ParsedError) error {
	logMessage := fmt.Sprintf(
		`severity=%s datname=%s user=%s pid="%d" backend_type=%s message=%s`,
		strconv.Quote(parsed.ErrorSeverity),
		strconv.Quote(parsed.DatabaseName),
		strconv.Quote(parsed.User),
		parsed.PID,
		strconv.Quote(parsed.BackendType),
		strconv.Quote(parsed.Message),
	)

	if parsed.QueryID > 0 {
		logMessage += fmt.Sprintf(` queryid="%d"`, parsed.QueryID)
	}

	if parsed.SQLState != "" {
		logMessage += fmt.Sprintf(` sqlstate=%s`, strconv.Quote(parsed.SQLState))
		if parsed.ErrorName != "" {
			logMessage += fmt.Sprintf(` error_name=%s`, strconv.Quote(parsed.ErrorName))
		}
		if parsed.SQLStateClass != "" {
			logMessage += fmt.Sprintf(` sqlstate_class=%s`, strconv.Quote(parsed.SQLStateClass))
		}
		if parsed.ErrorCategory != "" {
			logMessage += fmt.Sprintf(` error_category=%s`, strconv.Quote(parsed.ErrorCategory))
		}
	}

	if parsed.SessionID != "" {
		logMessage += fmt.Sprintf(` session_id=%s`, strconv.Quote(parsed.SessionID))
	}

	if parsed.LineNum > 0 {
		logMessage += fmt.Sprintf(` line_num=%d`, parsed.LineNum)
	}

	if parsed.PS != "" {
		logMessage += fmt.Sprintf(` ps=%s`, strconv.Quote(parsed.PS))
	}

	if parsed.VXID != "" {
		logMessage += fmt.Sprintf(` vxid=%s`, strconv.Quote(parsed.VXID))
	}

	if parsed.TXID != "" {
		logMessage += fmt.Sprintf(` txid=%s`, strconv.Quote(parsed.TXID))
	}

	if !parsed.SessionStart.IsZero() {
		logMessage += fmt.Sprintf(` session_start=%s`, strconv.Quote(parsed.SessionStart.Format(time.RFC3339)))
	}

	if parsed.ApplicationName != "" {
		logMessage += fmt.Sprintf(` app=%s`, strconv.Quote(parsed.ApplicationName))
	}

	if parsed.RemoteHost != "" {
		client := parsed.RemoteHost
		if parsed.RemotePort > 0 {
			client = fmt.Sprintf("%s:%d", parsed.RemoteHost, parsed.RemotePort)
		}
		logMessage += fmt.Sprintf(` client=%s`, strconv.Quote(client))
	}

	if parsed.Detail != "" {
		detail := parsed.Detail
		if !c.disableQueryRedaction {
			detail = redactMixedTextWithSQL(detail)
		}
		logMessage += fmt.Sprintf(` detail=%s`, strconv.Quote(detail))
	}

	if parsed.Hint != "" {
		logMessage += fmt.Sprintf(` hint=%s`, strconv.Quote(parsed.Hint))
	}

	if parsed.Context != "" {
		context := parsed.Context
		if !c.disableQueryRedaction {
			context = redactMixedTextWithSQL(context)
		}
		logMessage += fmt.Sprintf(` context=%s`, strconv.Quote(context))
	}

	if parsed.Statement != "" {
		statement := parsed.Statement
		if !c.disableQueryRedaction {
			statement = database_observability.RedactSql(statement)
		}
		logMessage += fmt.Sprintf(` statement=%s`, strconv.Quote(statement))
	}

	if parsed.CursorPosition > 0 {
		logMessage += fmt.Sprintf(` cursor_position=%d`, parsed.CursorPosition)
	}

	if parsed.InternalQuery != "" {
		internalQuery := parsed.InternalQuery
		if !c.disableQueryRedaction {
			internalQuery = database_observability.RedactSql(internalQuery)
		}
		logMessage += fmt.Sprintf(` internal_query=%s`, strconv.Quote(internalQuery))
	}

	if parsed.InternalPosition > 0 {
		logMessage += fmt.Sprintf(` internal_position=%d`, parsed.InternalPosition)
	}

	if parsed.FuncName != "" {
		logMessage += fmt.Sprintf(` func_name=%s`, strconv.Quote(parsed.FuncName))
	}

	if parsed.FileName != "" {
		logMessage += fmt.Sprintf(` file_name=%s`, strconv.Quote(parsed.FileName))
	}

	if parsed.FileLineNum > 0 {
		logMessage += fmt.Sprintf(` file_line_num=%d`, parsed.FileLineNum)
	}

	if parsed.LeaderPID > 0 {
		logMessage += fmt.Sprintf(` leader_pid=%d`, parsed.LeaderPID)
	}

	c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		OP_ERROR_LOGS,
		logMessage,
		parsed.Timestamp.UnixNano(),
	)

	return nil
}

func ptrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptrToInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

func ptrToInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func ptrInt64ToString(p *int64) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", *p)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// redactMixedTextWithSQL attempts to find and redact SQL statements within mixed text that
// could contain PII (Personally Identifiable Information) or sensitive data.
// It preserves surrounding context text (like process IDs, error descriptions, etc.)
// and does NOT redact administrative commands that don't contain data.
func redactMixedTextWithSQL(text string) string {
	if text == "" {
		return text
	}

	// SQL keywords for statements that could contain PII or sensitive data
	// Multi-word keywords (e.g., "CREATE USER") are handled by escaping spaces in the pattern
	sqlKeywords := []string{
		// DML (Data Manipulation Language) - contains actual data values
		"SELECT", "INSERT", "UPDATE", "DELETE", "MERGE",

		// WITH (CTEs) - can contain data in subqueries
		"WITH",

		// COPY - imports/exports actual data
		"COPY",

		// Procedural - can execute statements with data
		"DO", "CALL", "EXECUTE",

		// Prepared Statements - contain data values
		"PREPARE",

		// User/Role DDL - contains credentials/usernames
		"CREATE USER", "CREATE ROLE",
		"ALTER USER", "ALTER ROLE",
		"DROP USER", "DROP ROLE",

		// Grants - may contain sensitive role/permission info
		"GRANT", "REVOKE",

		// SET - could contain sensitive configuration values
		"SET",

		// VALUES - standalone VALUES clause with data
		"VALUES",
	}

	result := text

	for _, keyword := range sqlKeywords {
		// Handle both single and multi-word keywords by escaping spaces
		escapedKeyword := strings.ReplaceAll(keyword, " ", `\s+`)
		pattern := fmt.Sprintf(`(?i)\b%s\b[^;]*(?:;|$)`, escapedKeyword)
		re := regexp.MustCompile(pattern)

		matches := re.FindAllString(result, -1)
		for _, match := range matches {
			redacted := database_observability.RedactSql(match)
			result = strings.Replace(result, match, redacted, 1)
		}
	}

	return result
}
