package collector

import (
	"context"
	"encoding/json"
	"fmt"
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
	Timestamp string `json:"timestamp"`  // Time stamp with milliseconds
	PID       int32  `json:"pid"`        // Process ID
	SessionID string `json:"session_id"` // Session ID
	LineNum   int32  `json:"line_num"`   // Per-session line number

	// User/Database context
	User   *string `json:"user"`   // User name (nullable)
	DBName *string `json:"dbname"` // Database name (nullable)

	// Client information
	RemoteHost      *string `json:"remote_host"`      // Client host (nullable)
	RemotePort      *int32  `json:"remote_port"`      // Client port (nullable)
	ApplicationName *string `json:"application_name"` // Client application name (nullable)

	// Session/Process info
	PS           *string `json:"ps"`            // Current ps display (nullable)
	SessionStart string  `json:"session_start"` // Session start time
	BackendType  string  `json:"backend_type"`  // Type of backend

	// Transaction information
	VXID *string `json:"vxid"` // Virtual transaction ID (nullable, format: "3/1234")
	TXID *int64  `json:"txid"` // Regular transaction ID (nullable)

	// Error/Log information
	ErrorSeverity string  `json:"error_severity"` // Error severity (LOG, ERROR, FATAL, etc.)
	StateCode     *string `json:"state_code"`     // SQLSTATE code (nullable)
	Message       string  `json:"message"`        // Error message
	Detail        *string `json:"detail"`         // Error message detail (nullable)
	Hint          *string `json:"hint"`           // Error message hint (nullable)
	Context       *string `json:"context"`        // Error context (nullable)

	// Query information
	Statement      *string `json:"statement"`       // Client-supplied query string (nullable)
	CursorPosition *int32  `json:"cursor_position"` // Cursor index into query string (nullable)
	QueryID        *int64  `json:"query_id"`        // Query ID (nullable)

	// Internal query (for errors in functions/procedures)
	InternalQuery    *string `json:"internal_query"`    // Internal query that led to error (nullable)
	InternalPosition *int32  `json:"internal_position"` // Cursor index into internal query (nullable)

	// Error location (for internal errors)
	FuncName    *string `json:"func_name"`     // Error location function name (nullable)
	FileName    *string `json:"file_name"`     // File name of error location (nullable)
	FileLineNum *int32  `json:"file_line_num"` // File line number of error location (nullable)

	// Parallel query support
	LeaderPID *int32 `json:"leader_pid"` // Process ID of leader for active parallel workers (nullable)
}

// ParsedError represents a fully parsed and enriched PostgreSQL error.
type ParsedError struct {
	Timestamp time.Time
	PID       int32
	SessionID string
	LineNum   int32

	ErrorSeverity     string
	SQLStateCode      string
	ErrorName         string
	SQLStateCodeClass string
	ErrorCategory     string

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

	TimeoutType string // "deadlock", "lock_timeout", "query_canceled", "idle_in_transaction_timeout"
}

type ErrorLogsArguments struct {
	Receiver     loki.LogsReceiver
	EntryHandler loki.EntryHandler
	Logger       log.Logger
	InstanceKey  string
	SystemID     string
	Registry     *prometheus.Registry
}

type ErrorLogs struct {
	logger       log.Logger
	entryHandler loki.EntryHandler
	instanceKey  string
	systemID     string
	registry     *prometheus.Registry

	receiver loki.LogsReceiver

	logsProcessed       prometheus.Counter
	errorsTotal         *prometheus.CounterVec
	errorsBySQLState    *prometheus.CounterVec
	connectionErrors    *prometheus.CounterVec
	authFailures        *prometheus.CounterVec
	errorsByUser        *prometheus.CounterVec
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
		logger:       log.With(args.Logger, "collector", ErrorLogsCollector),
		entryHandler: args.EntryHandler,
		instanceKey:  args.InstanceKey,
		systemID:     args.SystemID,
		registry:     args.Registry,
		receiver:     args.Receiver,
		ctx:          ctx,
		cancel:       cancel,
		stopped:      atomic.NewBool(false),
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
			Name: "postgres_errors_by_sqlstate_total",
			Help: "PostgreSQL errors by SQLSTATE code with category, class, and query tracking",
		},
		[]string{"sqlstate_code", "error_name", "sqlstate_code_class", "error_category", "severity", "database", "queryid", "instance"},
	)

	c.connectionErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_connection_errors_total",
			Help: "Connection-related errors",
		},
		[]string{"sqlstate_code", "severity", "database", "instance"},
	)

	c.authFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_auth_failures_total",
			Help: "Authentication failures by user",
		},
		[]string{"user", "remote_host", "database", "instance"},
	)

	c.errorsByUser = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_errors_by_user_total",
			Help: "Errors by database user",
		},
		[]string{"user", "severity", "database", "instance"},
	)

	c.errorsByBackendType = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_errors_by_backend_type_total",
			Help: "Errors by backend type",
		},
		[]string{"backend_type", "severity", "instance"},
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
			c.connectionErrors,
			c.authFailures,
			c.errorsByUser,
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

	c.setTimeoutType(parsed)

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
	timestampFormats := []string{
		"2006-01-02 15:04:05.999999 MST", // With microseconds
		"2006-01-02 15:04:05.999 MST",    // With milliseconds
		"2006-01-02 15:04:05 MST",        // Without fractional seconds
		time.RFC3339Nano,                 // ISO 8601 with nanoseconds
		time.RFC3339,                     // ISO 8601 without nanoseconds
	}

	for _, format := range timestampFormats {
		parsed.Timestamp, err = time.Parse(format, log.Timestamp)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	if log.SessionStart != "" {
		for _, format := range timestampFormats {
			parsed.SessionStart, err = time.Parse(format, log.SessionStart)
			if err == nil {
				break
			}
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

	if log.StateCode != nil {
		parsed.SQLStateCode = *log.StateCode
		parsed.ErrorName = GetSQLStateErrorName(parsed.SQLStateCode)
		parsed.SQLStateCodeClass = parsed.SQLStateCode[:2]
		parsed.ErrorCategory = GetSQLStateCategory(parsed.SQLStateCode)
	}

	return parsed, nil
}

func (c *ErrorLogs) updateMetrics(parsed *ParsedError) {
	c.errorsTotal.WithLabelValues(
		parsed.ErrorSeverity,
		parsed.DatabaseName,
		c.instanceKey,
	).Inc()

	if parsed.SQLStateCode != "" {
		queryIDStr := ""
		if parsed.QueryID > 0 {
			queryIDStr = fmt.Sprintf("%d", parsed.QueryID)
		}

		c.errorsBySQLState.WithLabelValues(
			parsed.SQLStateCode,
			parsed.ErrorName,
			parsed.SQLStateCodeClass,
			parsed.ErrorCategory,
			parsed.ErrorSeverity,
			parsed.DatabaseName,
			queryIDStr,
			c.instanceKey,
		).Inc()
	}

	if IsConnectionError(parsed.SQLStateCode) {
		c.connectionErrors.WithLabelValues(
			parsed.SQLStateCode,
			parsed.ErrorSeverity,
			parsed.DatabaseName,
			c.instanceKey,
		).Inc()
	}

	if IsAuthenticationError(parsed.SQLStateCode) {
		c.authFailures.WithLabelValues(
			parsed.User,
			parsed.RemoteHost,
			parsed.DatabaseName,
			c.instanceKey,
		).Inc()
	}

	if parsed.User != "" {
		c.errorsByUser.WithLabelValues(
			parsed.User,
			parsed.ErrorSeverity,
			parsed.DatabaseName,
			c.instanceKey,
		).Inc()
	}

	c.errorsByBackendType.WithLabelValues(
		parsed.BackendType,
		parsed.ErrorSeverity,
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

	if parsed.SQLStateCode != "" {
		logMessage += fmt.Sprintf(` sqlstate=%s`, strconv.Quote(parsed.SQLStateCode))
		if parsed.ErrorName != "" {
			logMessage += fmt.Sprintf(` error_name=%s`, strconv.Quote(parsed.ErrorName))
		}
		if parsed.ErrorCategory != "" {
			logMessage += fmt.Sprintf(` sqlstate_class=%s`, strconv.Quote(parsed.ErrorCategory))
		}
		if parsed.SQLStateCodeClass != "" {
			logMessage += fmt.Sprintf(` sqlstate_class_code=%s`, strconv.Quote(parsed.SQLStateCodeClass))
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
		logMessage += fmt.Sprintf(` detail=%s`, strconv.Quote(parsed.Detail))
	}

	if parsed.Hint != "" {
		logMessage += fmt.Sprintf(` hint=%s`, strconv.Quote(parsed.Hint))
	}

	if parsed.Context != "" {
		logMessage += fmt.Sprintf(` context=%s`, strconv.Quote(parsed.Context))
	}

	if parsed.Statement != "" {
		logMessage += fmt.Sprintf(` statement=%s`, strconv.Quote(parsed.Statement))
	}

	if parsed.CursorPosition > 0 {
		logMessage += fmt.Sprintf(` cursor_position=%d`, parsed.CursorPosition)
	}

	if parsed.InternalQuery != "" {
		logMessage += fmt.Sprintf(` internal_query=%s`, strconv.Quote(parsed.InternalQuery))
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

	if parsed.TimeoutType != "" {
		logMessage += fmt.Sprintf(` timeout_type=%s`, strconv.Quote(parsed.TimeoutType))
	}

	c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		OP_ERROR_LOGS,
		logMessage,
		parsed.Timestamp.UnixNano(),
	)

	return nil
}

// Helper functions to safely dereference pointers
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

// Helper function to truncate strings for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
