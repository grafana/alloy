package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

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

// RDS log format regex pattern to validate log line structure
// Expected format: %m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a
// Example: 2026-02-02 21:35:40.130 UTC:10.24.155.141(34110):mybooks-app@books_store:[32032]:2:40001:2026-02-02 21:33:19 UTC:...
// Note: Timezone can be any 3-4 letter abbreviation (UTC, GMT, EST, PST, etc.)
var rdsLogFormatRegex = regexp.MustCompile(
	`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d{3})? [A-Z]{3,4}:` + // timestamp (%m) with timezone
		`[^:]+:` + // host:port (%r)
		`[^@]+@[^:]+:` + // user@database (%u@%d)
		`\[\d*\]:` + // [pid] (%p) - may be empty if not available
		`\d+:` + // line number (%l)
		`[A-Z0-9]{5}:`, // SQLSTATE (%e) - exactly 5 alphanumeric chars
)

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

type ParsedError struct {
	ErrorSeverity string
	SQLStateCode  string
	SQLStateClass string
	User          string
	DatabaseName  string
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
	systemIDMutex sync.RWMutex
	registry     *prometheus.Registry

	receiver loki.LogsReceiver

	errorsBySQLState *prometheus.CounterVec
	parseErrors      prometheus.Counter

	ctx     context.Context
	cancel  context.CancelFunc
	stopped *atomic.Bool
	wg      sync.WaitGroup

	// Format validation tracking (for rate-limited warnings)
	formatCheckMutex      sync.Mutex
	lastFormatWarning     time.Time
	validLogsThisMinute   int
	invalidLogsThisMinute int
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
			Help: "PostgreSQL errors by severity with database, user, SQLSTATE, and instance tracking",
		},
		[]string{"severity", "sqlstate", "sqlstate_class", "database", "user", "instance", "server_id"},
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

	// CloudWatch/OTLP logs come wrapped in JSON - extract the body field
	if strings.HasPrefix(line, "{") {
		var jsonLog struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal([]byte(line), &jsonLog); err == nil && jsonLog.Body != "" {
			line = jsonLog.Body
		}
	}

	// Skip multi-line continuation lines (DETAIL, HINT, CONTEXT, etc.)
	if isContinuationLine(line) {
		return nil
	}

	// Only process ERROR, FATAL, PANIC lines
	if !strings.Contains(line, "ERROR:") && !strings.Contains(line, "FATAL:") && !strings.Contains(line, "PANIC:") {
		return nil
	}

	// Validate log format matches expected RDS pattern
	if !rdsLogFormatRegex.MatchString(line) {
		c.trackInvalidFormat()
		c.parseErrors.Inc()
		return fmt.Errorf("log line does not match expected RDS format")
	}

	// Track that we've seen a valid format
	c.trackValidFormat()

	// Parse RDS format: %m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a
	// Format already validated by regex, so we can safely extract fields
	atIdx := strings.Index(line, "@")
	afterAt := line[atIdx+1:]
	pidMarkerIdx := strings.Index(afterAt, ":[")

	database := strings.TrimSpace(afterAt[:pidMarkerIdx])

	beforeAt := line[:atIdx]
	lastColonBeforeAt := strings.LastIndex(beforeAt, ":")
	user := strings.TrimSpace(beforeAt[lastColonBeforeAt+1:])

	// Extract SQLSTATE from format: [pid]:line_number:SQLSTATE:...
	pidEndIdx := strings.Index(afterAt, "]")
	afterPid := afterAt[pidEndIdx+1:]
	
	parts := strings.SplitN(afterPid, ":", 4)
	sqlstateCode := strings.TrimSpace(parts[2])
	sqlstateClass := ""
	if len(sqlstateCode) >= 2 {
		sqlstateClass = sqlstateCode[:2]
	}

	// Find severity keyword (ERROR:, FATAL:, PANIC:)
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
		return nil
	}

	if !supportedSeverities[severity] {
		return nil
	}

	parsed := &ParsedError{
		ErrorSeverity: severity,
		SQLStateCode:  sqlstateCode,
		SQLStateClass: sqlstateClass,
		User:          user,
		DatabaseName:  database,
	}

	c.updateMetrics(parsed)
	return nil
}

// isContinuationLine checks if a line is part of a multi-line PostgreSQL error.
// Returns true for tab-indented lines or lines starting with DETAIL, HINT, etc.
func isContinuationLine(line string) bool {
	if strings.HasPrefix(line, "\t") {
		return true
	}

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
	if idx := strings.Index(message, ":"); idx > 0 {
		return strings.TrimSpace(message[:idx])
	}
	return ""
}

func (c *ErrorLogs) updateMetrics(parsed *ParsedError) {
	c.systemIDMutex.RLock()
	systemID := c.systemID
	c.systemIDMutex.RUnlock()
	
	c.errorsBySQLState.WithLabelValues(
		parsed.ErrorSeverity,
		parsed.SQLStateCode,
		parsed.SQLStateClass,
		parsed.DatabaseName,
		parsed.User,
		c.instanceKey,
		systemID,
	).Inc()
}

// UpdateSystemID updates the system ID used in metrics labels.
// This is thread-safe and can be called while the collector is running.
func (c *ErrorLogs) UpdateSystemID(systemID string) {
	c.systemIDMutex.Lock()
	defer c.systemIDMutex.Unlock()
	c.systemID = systemID
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// trackValidFormat tracks that we've seen a valid log format this minute
func (c *ErrorLogs) trackValidFormat() {
	c.formatCheckMutex.Lock()
	defer c.formatCheckMutex.Unlock()
	c.validLogsThisMinute++
}

// trackInvalidFormat tracks invalid format and emits warning if ALL logs in past minute were invalid
func (c *ErrorLogs) trackInvalidFormat() {
	c.formatCheckMutex.Lock()
	defer c.formatCheckMutex.Unlock()

	c.invalidLogsThisMinute++

	// Check if we should emit a warning (once per minute)
	now := time.Now()
	if now.Sub(c.lastFormatWarning) >= time.Minute {
		// Only warn if ALL logs in this window were invalid
		if c.validLogsThisMinute == 0 && c.invalidLogsThisMinute > 0 {
			level.Warn(c.logger).Log(
				"msg", "all PostgreSQL error logs in the last minute had invalid format",
				"invalid_count", c.invalidLogsThisMinute,
				"expected_format", "%m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a",
				"hint", "ensure log_line_prefix is set correctly on PostgreSQL server",
			)
		}

		// Reset counters for next minute window
		c.lastFormatWarning = now
		c.validLogsThisMinute = 0
		c.invalidLogsThisMinute = 0
	}
}
