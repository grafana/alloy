package collector

import (
	"context"
	"fmt"
	"regexp"
	"slices"
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
	LogsCollector         = "logs"
	expectedLogLinePrefix = "%m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a"
)

// Postgres log format regex
var logFormatRegex = regexp.MustCompile(
	`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d{3})? (?:[A-Z]{3,4}|[+-]\d{2}):` +
		`[^:]*:` +
		`[^@]*@[^:]*:` +
		`\[\d*\]:` +
		`\d+:` +
		`[A-Z0-9]{5}:`,
)

var supportedSeverities = map[string]struct{}{
	"ERROR": {},
	"FATAL": {},
	"PANIC": {},
}

type LogsArguments struct {
	Receiver         loki.LogsReceiver
	EntryHandler     loki.EntryHandler
	Logger           log.Logger
	Registry         *prometheus.Registry
	ExcludeDatabases []string
	ExcludeUsers     []string
}

type Logs struct {
	logger       log.Logger
	entryHandler loki.EntryHandler
	registry     *prometheus.Registry

	receiver         loki.LogsReceiver
	excludeDatabases []string
	excludeUsers     []string

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

	startTime time.Time
}

func NewLogs(args LogsArguments) (*Logs, error) {
	ctx, cancel := context.WithCancel(context.Background())

	l := &Logs{
		logger:           log.With(args.Logger, "collector", LogsCollector),
		entryHandler:     args.EntryHandler,
		registry:         args.Registry,
		receiver:         args.Receiver,
		excludeDatabases: args.ExcludeDatabases,
		excludeUsers:     args.ExcludeUsers,
		ctx:              ctx,
		cancel:           cancel,
		stopped:          atomic.NewBool(false),
		startTime:        time.Now(),
	}

	l.initMetrics()

	return l, nil
}

func (l *Logs) initMetrics() {
	l.errorsBySQLState = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "database_observability",
			Name:      "pg_errors_total",
			Help:      "Number of log lines with errors by severity and sql state code",
		},
		[]string{"severity", "sqlstate", "sqlstate_class", "datname", "user"},
	)

	l.parseErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "database_observability",
			Name:      "pg_error_log_parse_failures_total",
			Help:      "Number of log lines with errors that failed to parse",
		},
	)

	l.registry.MustRegister(
		l.errorsBySQLState,
		l.parseErrors,
	)
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

	l.registry.Unregister(l.errorsBySQLState)
	l.registry.Unregister(l.parseErrors)
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
			if err := l.parseTextLog(entry); err != nil {
				level.Warn(l.logger).Log(
					"msg", "failed to process log line",
					"error", err,
					"line_preview", truncateString(entry.Entry.Line, 100),
				)
			}
		}
	}
}

func (l *Logs) parseTextLog(entry loki.Entry) error {
	line := entry.Entry.Line

	if isContinuationLine(line) {
		return nil
	}

	if !strings.Contains(line, "ERROR:") && !strings.Contains(line, "FATAL:") && !strings.Contains(line, "PANIC:") {
		return nil
	}

	if !logFormatRegex.MatchString(line) {
		l.trackInvalidFormat()
		l.parseErrors.Inc()
		return fmt.Errorf("log line does not match expected format")
	}

	l.trackValidFormat()

	if len(line) > 30 {
		colonIdx := strings.Index(line[20:], ":")
		if colonIdx > 0 {
			timestampStr := strings.TrimSpace(line[:20+colonIdx])

			// Format: "YYYY-MM-DD HH:MM:SS[.mmm] TZ:..." where TZ can be GMT, UTC, -03, etc.
			for _, layout := range []string{
				"2006-01-02 15:04:05.000 MST",
				"2006-01-02 15:04:05.000 -07",
				"2006-01-02 15:04:05 MST",
				"2006-01-02 15:04:05 -07",
			} {
				logTimestamp, err := time.Parse(layout, timestampStr)
				if err == nil {
					if !logTimestamp.After(l.startTime) {
						return nil // Skip historical log
					}
					break // Found valid timestamp, continue processing
				}
			}
		}
	}

	// Parse log line prefix format: %m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a
	before, after, _ := strings.Cut(line, "@")
	afterAt := after
	before, _, _ := strings.Cut(afterAt, ":[")

	database := strings.TrimSpace(before)

	if slices.Contains(l.excludeDatabases, database) {
		return nil
	}

	beforeAt := before
	lastColonBeforeAt := strings.LastIndex(beforeAt, ":")
	user := strings.TrimSpace(beforeAt[lastColonBeforeAt+1:])

	if slices.Contains(l.excludeUsers, user) {
		return nil
	}

	// Extract SQLSTATE from format: [pid]:line_number:SQLSTATE:...
	_, after, _ := strings.Cut(afterAt, "]")
	afterPid := after

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

	if _, ok := supportedSeverities[severity]; !ok {
		return nil
	}

	l.errorsBySQLState.WithLabelValues(
		severity,
		sqlstateCode,
		sqlstateClass,
		database,
		user,
	).Inc()

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
				"expected_format", expectedLogLinePrefix,
				"hint", "ensure log_line_prefix is set correctly on PostgreSQL server",
			)
		}

		l.lastFormatWarning = now
		l.validLogsThisMinute = 0
		l.invalidLogsThisMinute = 0
	}
}
