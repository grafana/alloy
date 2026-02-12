package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	LogsCollector     = "logs"
	watermarkFilename = "dbo11y_pg_logs_watermark.txt"
	expectedLogFormat = "%m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a"
)

// Postgres log format regex
var logFormatRegex = regexp.MustCompile(
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
	Registry     *prometheus.Registry
	DataPath     string
}

type Logs struct {
	logger       log.Logger
	entryHandler loki.EntryHandler
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

	// Watermark tracking
	dataPath               string
	lastProcessedTimestamp *atomic.Time
	startTime              time.Time
	watermarkQuit          chan struct{}
	watermarkDone          chan struct{}
}

func NewLogs(args LogsArguments) (*Logs, error) {
	ctx, cancel := context.WithCancel(context.Background())

	l := &Logs{
		logger:                 log.With(args.Logger, "collector", LogsCollector),
		entryHandler:           args.EntryHandler,
		registry:               args.Registry,
		receiver:               args.Receiver,
		ctx:                    ctx,
		cancel:                 cancel,
		stopped:                atomic.NewBool(false),
		dataPath:               args.DataPath,
		lastProcessedTimestamp: atomic.NewTime(time.Time{}),
		startTime:              time.Now(),
		watermarkQuit:          make(chan struct{}),
		watermarkDone:          make(chan struct{}),
	}

	l.initMetrics()

	if err := l.loadWatermark(); err != nil {
		level.Warn(l.logger).Log("msg", "failed to load watermark, starting from now", "err", err)
		l.lastProcessedTimestamp.Store(l.startTime)
	}

	return l, nil
}

func (l *Logs) initMetrics() {
	l.errorsBySQLState = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "database_observability_postgres_errors_total",
			Help: "PostgreSQL errors by severity with database, user, and SQLSTATE",
		},
		[]string{"severity", "sqlstate", "sqlstate_class", "datname", "user"},
	)

	l.parseErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "database_observability_postgres_error_log_parse_failures_total",
			Help: "Failed to parse log lines",
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

	l.wg.Add(2)
	go l.run()
	go l.syncWatermark()

	return nil
}

func (l *Logs) Stop() {
	l.cancel()
	l.stopped.Store(true)
	close(l.watermarkQuit)
	l.wg.Wait()

	l.registry.Unregister(l.errorsBySQLState)
	l.registry.Unregister(l.parseErrors)
}

func (l *Logs) Stopped() bool {
	return l.stopped.Load()
}

func (l *Logs) watermarkPath() string {
	if l.dataPath == "" {
		return ""
	}
	return filepath.Join(l.dataPath, watermarkFilename)
}

func (l *Logs) loadWatermark() error {
	watermarkPath := l.watermarkPath()
	if watermarkPath == "" {
		level.Info(l.logger).Log("msg", "no watermark path specified, starting from now")
		l.lastProcessedTimestamp.Store(l.startTime)
		return nil
	}

	data, err := os.ReadFile(watermarkPath)
	if err != nil {
		if os.IsNotExist(err) {
			level.Info(l.logger).Log("msg", "watermark file does not exist, starting from now", "path", watermarkPath)
			l.lastProcessedTimestamp.Store(l.startTime)
			return nil
		}
		return fmt.Errorf("failed to read watermark file: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("failed to parse watermark timestamp: %w", err)
	}

	l.lastProcessedTimestamp.Store(timestamp)
	level.Info(l.logger).Log("msg", "loaded watermark from disk", "timestamp", timestamp, "path", watermarkPath)

	return nil
}

func (l *Logs) saveWatermark() error {
	watermarkPath := l.watermarkPath()
	if watermarkPath == "" {
		return nil
	}

	timestamp := l.lastProcessedTimestamp.Load()
	if timestamp.IsZero() {
		return nil
	}

	data := []byte(timestamp.Format(time.RFC3339Nano))

	dir := filepath.Dir(watermarkPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create watermark directory: %w", err)
	}

	tmpPath := watermarkPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write watermark temp file: %w", err)
	}

	if err := os.Rename(tmpPath, watermarkPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename watermark file: %w", err)
	}

	level.Debug(l.logger).Log("msg", "saved watermark to disk", "timestamp", timestamp, "path", watermarkPath)

	return nil
}

func (l *Logs) syncWatermark() {
	defer l.wg.Done()
	defer close(l.watermarkDone)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.watermarkQuit:
			if err := l.saveWatermark(); err != nil {
				level.Error(l.logger).Log("msg", "failed to save watermark on shutdown", "err", err)
			}
			return
		case <-ticker.C:
			if err := l.saveWatermark(); err != nil {
				level.Error(l.logger).Log("msg", "failed to sync watermark", "err", err)
			}
		}
	}
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
	watermark := l.lastProcessedTimestamp.Load()

	if !entry.Entry.Timestamp.After(watermark) {
		return nil
	}

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

	if entry.Entry.Timestamp.After(watermark) {
		l.lastProcessedTimestamp.Store(entry.Entry.Timestamp)
	}

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
				"expected_format", expectedLogFormat,
				"hint", "ensure log_line_prefix is set correctly on PostgreSQL server",
			)
		}

		l.lastFormatWarning = now
		l.validLogsThisMinute = 0
		l.invalidLogsThisMinute = 0
	}
}
