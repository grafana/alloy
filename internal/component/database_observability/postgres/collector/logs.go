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

const LogsCollector = "logs"

// RDS log format: %m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a
var rdsLogFormatRegex = regexp.MustCompile(
	`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d{3})? [A-Z]{3,4}:` +
		`[^:]*:` +
		`[^@]*@[^:]*:` +
		`\[\d*\]:` +
		`\d+:` +
		`[A-Z0-9]{5}:`,
)

var supportedSeverities = map[string]bool{
	"ERROR": true,
	"FATAL": true,
	"PANIC": true,
}

type ParsedError struct {
	ErrorSeverity string
	SQLStateCode  string
	SQLStateClass string
	User          string
	DatabaseName  string
}

type LogsArguments struct {
	Receiver     loki.LogsReceiver
	EntryHandler loki.EntryHandler
	Logger       log.Logger
	InstanceKey  string
	SystemID     string
	Registry     *prometheus.Registry
}

type Logs struct {
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

	formatCheckMutex      sync.Mutex
	lastFormatWarning     time.Time
	validLogsThisMinute   int
	invalidLogsThisMinute int
}

func NewLogs(args LogsArguments) (*Logs, error) {
	ctx, cancel := context.WithCancel(context.Background())

	l := &Logs{
		logger:       log.With(args.Logger, "collector", LogsCollector),
		entryHandler: args.EntryHandler,
		instanceKey:  args.InstanceKey,
		systemID:     args.SystemID,
		registry:     args.Registry,
		receiver:     args.Receiver,
		ctx:          ctx,
		cancel:       cancel,
		stopped:      atomic.NewBool(false),
	}

	l.initMetrics()

	return l, nil
}

func (l *Logs) initMetrics() {
	l.errorsBySQLState = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_errors_total",
			Help: "PostgreSQL errors by severity with database, user, SQLSTATE, and instance tracking",
		},
		[]string{"severity", "sqlstate", "sqlstate_class", "database", "user", "instance", "server_id"},
	)

	l.parseErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "postgres_error_log_parse_failures_total",
			Help: "Failed to parse log lines",
		},
	)

	if l.registry != nil {
		l.registry.MustRegister(
			l.errorsBySQLState,
			l.parseErrors,
		)
	} else {
		level.Warn(l.logger).Log("msg", "no Prometheus registry provided, metrics will not be exposed")
	}
}

func (l *Logs) Name() string {
	return LogsCollector
}

// Receiver returns the logs receiver that loki.source.* can forward to
func (l *Logs) Receiver() loki.LogsReceiver {
	return l.receiver
}

func (l *Logs) Start(ctx context.Context) error {
	level.Debug(l.logger).Log("msg", "collector started")

	l.wg.Add(1)
	go l.run()
	return nil
}

func (l *Logs) Stop() {
	l.cancel()
	l.stopped.Store(true)
	l.wg.Wait()
}

func (l *Logs) Stopped() bool {
	return l.stopped.Load()
}

func (l *Logs) run() {
	defer l.wg.Done()

	level.Debug(l.logger).Log("msg", "collector running, waiting for log entries")

	for {
		select {
		case <-l.ctx.Done():
			level.Debug(l.logger).Log("msg", "collector stopping")
			return
		case entry := <-l.receiver.Chan():
			if err := l.processLogLine(entry); err != nil {
				level.Warn(l.logger).Log(
					"msg", "failed to process log line",
					"error", err,
					"line_preview", truncateString(entry.Entry.Line, 100),
				)
			}
		}
	}
}

func (l *Logs) processLogLine(entry loki.Entry) error {
	return l.parseTextLog(entry)
}

// parseTextLog extracts fields from stderr text format logs for metrics
func (l *Logs) parseTextLog(entry loki.Entry) error {
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
		l.trackInvalidFormat()
		l.parseErrors.Inc()
		return fmt.Errorf("log line does not match expected RDS format")
	}

	l.trackValidFormat()

	// Parse RDS format: %m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a
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

	l.updateMetrics(parsed)
	return nil
}

// isContinuationLine checks if a line is part of a multi-line PostgreSQL error
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

func (l *Logs) updateMetrics(parsed *ParsedError) {
	l.errorsBySQLState.WithLabelValues(
		parsed.ErrorSeverity,
		parsed.SQLStateCode,
		parsed.SQLStateClass,
		parsed.DatabaseName,
		parsed.User,
		l.instanceKey,
		l.systemID,
	).Inc()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// trackValidFormat tracks that we've seen a valid log format this minute
func (l *Logs) trackValidFormat() {
	l.formatCheckMutex.Lock()
	defer l.formatCheckMutex.Unlock()
	l.validLogsThisMinute++
}

// trackInvalidFormat tracks invalid format and emits warning once per minute if ALL logs were invalid
func (l *Logs) trackInvalidFormat() {
	l.formatCheckMutex.Lock()
	defer l.formatCheckMutex.Unlock()

	l.invalidLogsThisMinute++

	// Emit warning once per minute if ALL logs were invalid
	now := time.Now()
	if now.Sub(l.lastFormatWarning) >= time.Minute {
		if l.validLogsThisMinute == 0 && l.invalidLogsThisMinute > 0 {
			level.Warn(l.logger).Log(
				"msg", "all PostgreSQL error logs in the last minute had invalid format",
				"invalid_count", l.invalidLogsThisMinute,
				"expected_format", "%m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a",
				"hint", "ensure log_line_prefix is set correctly on PostgreSQL server",
			)
		}

		l.lastFormatWarning = now
		l.validLogsThisMinute = 0
		l.invalidLogsThisMinute = 0
	}
}
