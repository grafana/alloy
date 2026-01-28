package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	ErrorLogsCollector = "error_logs"
	OP_ERROR_LOGS      = "error_logs"
)

// Supported error severities that will be processed
var supportedSeverities = map[string]bool{
	"ERROR": true,
	"FATAL": true,
	"PANIC": true,
}

// PostgreSQL Text Log Format (stderr) - RDS Format
//
// RDS log_line_prefix format: %m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a
//
// Example log line:
// 2025-01-12 10:30:45 UTC:10.0.1.5:54321:app-user@books_store:[9112]:4:57014:2025-01-12 10:29:15 UTC:25/112:0:693c34cb.2398::psqlERROR:  canceling statement
//
// Field mapping:
// %m  - Timestamp with milliseconds (e.g., "2025-01-12 10:30:45 UTC")
// %r  - Remote host:port (e.g., "10.0.1.5:54321" or "[local]")
// %u@%d - User@Database (e.g., "app-user@books_store")
// [%p] - Process ID in brackets (e.g., "[9112]")
// %l  - Session line number
// %e  - SQLSTATE error code
// %s  - Session start timestamp
// %v  - Virtual transaction ID
// %x  - Transaction ID
// %c  - Session ID
// %q  - Query text (usually empty)
// %a  - Application name
// Message - Log message (severity: message text)

// ParsedError contains the extracted error information for metrics.
type ParsedError struct {
	ErrorSeverity string // ERROR, FATAL, PANIC
	User          string // Database user
	DatabaseName  string // Database name
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

	errorsBySQLState *prometheus.CounterVec
	parseErrors      prometheus.Counter

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
	c.errorsBySQLState = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_errors_total",
			Help: "PostgreSQL errors by severity with database, user, and instance tracking",
		},
		[]string{"severity", "database", "user", "instance", "server_id"},
	)

	c.parseErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "postgres_error_log_parse_failures_total",
			Help: "Failed to parse log lines",
		},
	)

	if c.registry != nil {
		c.registry.MustRegister(
			c.errorsBySQLState,
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
	return c.parseTextLog(entry)
}

// parseTextLog extracts fields from stderr text format logs for metrics.
// Parses RDS format: %m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a
func (c *ErrorLogs) parseTextLog(entry loki.Entry) error {
	line := entry.Entry.Line

	// CloudWatch/OTLP logs come wrapped in JSON with a "body" field
	// Example: {"body":"2026-01-15 15:39:17 UTC:10.24.194.153(35450):...","attributes":{...}}
	// Extract the actual log line from the body field if present
	if strings.HasPrefix(line, "{") {
		var jsonLog struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal([]byte(line), &jsonLog); err == nil && jsonLog.Body != "" {
			line = jsonLog.Body
		}
	}

	// Check if this is a continuation line (DETAIL, HINT, CONTEXT, STATEMENT, etc.)
	// These are expected in multi-line errors and should not be counted as parse failures
	if isContinuationLine(line) {
		return nil // Skip continuation lines silently
	}

	// Parse RDS format: %m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a
	// Example: 2025-01-12 10:30:45 UTC:10.0.1.5:54321:app-user@books_store:[9112]:4:57014:...ERROR:  message

	// Find the @database: pattern which reliably marks the database field
	// The format is %u@%d:[%p] so we look for @...:[
	atIdx := strings.Index(line, "@")
	if atIdx == -1 {
		c.parseErrors.Inc()
		return fmt.Errorf("invalid RDS log format: missing @ in user@database")
	}

	// From @ onwards, find the :[ pattern that marks the PID
	// We need to find :[digit...]: pattern
	afterAt := line[atIdx+1:]
	pidMarkerIdx := strings.Index(afterAt, ":[")
	if pidMarkerIdx == -1 {
		c.parseErrors.Inc()
		return fmt.Errorf("invalid RDS log format: missing :[pid] marker after database")
	}

	// Extract database name (between @ and :[)
	database := strings.TrimSpace(afterAt[:pidMarkerIdx])

	// Extract user name (work backwards from @)
	// Find the last : before the @ to get the user field
	beforeAt := line[:atIdx]
	lastColonBeforeAt := strings.LastIndex(beforeAt, ":")
	if lastColonBeforeAt == -1 {
		c.parseErrors.Inc()
		return fmt.Errorf("invalid RDS log format: cannot locate user field")
	}

	user := strings.TrimSpace(beforeAt[lastColonBeforeAt+1:])

	// Find the message part - it starts after %a (application name)
	// Look for severity keywords (ERROR:, FATAL:, PANIC:) as they mark the start of the message
	messageStart := -1
	severity := ""
	for sev := range supportedSeverities {
		idx := strings.Index(line, sev+":")
		if idx != -1 && (messageStart == -1 || idx < messageStart) {
			messageStart = idx
			severity = sev
		}
	}

	if messageStart == -1 {
		// No supported severity found, skip this line
		return nil
	}

	// Filter: only process ERROR, FATAL, PANIC
	if !supportedSeverities[severity] {
		return nil // Skip INFO, LOG, WARNING, etc.
	}

	// Create ParsedError for metrics
	parsed := &ParsedError{
		ErrorSeverity: severity,
		User:          user,
		DatabaseName:  database,
	}

	// Emit metrics
	c.updateMetrics(parsed)

	return nil
}

// extractSeverity parses the severity from the message part.
// Input: "ERROR:  canceling statement due to timeout"
// Output: "ERROR"
// isContinuationLine checks if a log line is a multi-line error continuation.
// PostgreSQL outputs multi-line errors with continuation lines that start with:
// - Tab character(s) for indented content
// - Known keywords: DETAIL, HINT, CONTEXT, STATEMENT, QUERY, LOCATION
func isContinuationLine(line string) bool {
	// Check for tab-indented lines (common for continuation context)
	if strings.HasPrefix(line, "\t") {
		return true
	}

	// Check for known continuation keywords at the start of the line
	continuationKeywords := []string{
		"DETAIL:",
		"HINT:",
		"CONTEXT:",
		"STATEMENT:",
		"QUERY:",
		"LOCATION:",
	}

	trimmedLine := strings.TrimSpace(line)
	for _, keyword := range continuationKeywords {
		if strings.HasPrefix(trimmedLine, keyword) {
			return true
		}
	}

	return false
}

func extractSeverity(message string) string {
	// Format is typically "SEVERITY:  message text"
	if idx := strings.Index(message, ":"); idx > 0 {
		return strings.TrimSpace(message[:idx])
	}
	return ""
}

func (c *ErrorLogs) updateMetrics(parsed *ParsedError) {
	c.errorsBySQLState.WithLabelValues(
		parsed.ErrorSeverity, // severity: "ERROR"
		parsed.DatabaseName,  // database: "books_store"
		parsed.User,          // user: "app-user"
		c.instanceKey,        // instance: "orders_db"
		c.systemID,           // server_id: "a1b2c3d4..."
	).Inc()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
