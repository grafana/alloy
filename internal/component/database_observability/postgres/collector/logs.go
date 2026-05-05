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
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	LogsCollector         = "logs"
	expectedLogLinePrefix = "%m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a:"
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

type pendingError struct {
	receivedAt    time.Time
	severity      string
	sqlstate      string
	sqlstateClass string
	datname       string
}

// statementBuffer holds a STATEMENT continuation as it accumulates across
// multiple log lines. PostgreSQL emits the SQL text of an error's statement
// over a keyword line (`<prefix>STATEMENT:  <first line of SQL>`) plus zero
// or more TAB-prefixed continuation lines for the rest of the SQL.
type statementBuffer struct {
	pid        string
	receivedAt time.Time
	sql        strings.Builder
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

	errorsBySQLState    *prometheus.CounterVec
	errorsByFingerprint *prometheus.CounterVec
	parseErrors         prometheus.Counter

	ctx     context.Context
	cancel  context.CancelFunc
	stopped *atomic.Bool
	wg      sync.WaitGroup

	formatCheckMutex      sync.Mutex
	lastFormatWarning     time.Time
	validLogsThisMinute   int
	invalidLogsThisMinute int

	pendingErrors       map[string]*pendingError
	pendingMu           sync.Mutex
	pendingErrorTimeout time.Duration

	currentStatement *statementBuffer
	statementMu      sync.Mutex

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

	l.pendingErrors = make(map[string]*pendingError)
	l.pendingErrorTimeout = 5 * time.Second

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

	l.errorsByFingerprint = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "database_observability",
			Name:      "pg_errors_by_fingerprint_total",
			Help:      "Number of PostgreSQL log errors with a captured query fingerprint, partitioned by severity, SQL state, and the originating query's fingerprint. Counts a subset of pg_errors_total (only errors for which Alloy successfully observed the matching STATEMENT continuation).",
		},
		[]string{"severity", "sqlstate", "sqlstate_class", "datname", "query_fingerprint"},
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
		l.errorsByFingerprint,
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

	l.wg.Go(l.run)

	return nil
}

func (l *Logs) Stop() {
	l.cancel()
	l.stopped.Store(true)
	l.wg.Wait()

	l.registry.Unregister(l.errorsBySQLState)
	l.registry.Unregister(l.errorsByFingerprint)
	l.registry.Unregister(l.parseErrors)
}

func (l *Logs) Stopped() bool {
	return l.stopped.Load()
}

func (l *Logs) run() {
	level.Debug(l.logger).Log("msg", "collector running, waiting for log entries")

	tickPeriod := l.pendingErrorTimeout / 2
	if tickPeriod < 50*time.Millisecond {
		tickPeriod = 50 * time.Millisecond
	}
	timeoutTicker := time.NewTicker(tickPeriod)
	defer timeoutTicker.Stop()

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
		case <-timeoutTicker.C:
			l.flushExpiredPending()
		}
	}
}

func (l *Logs) parseTextLog(entry loki.Entry) error {
	line := entry.Entry.Line

	// TAB-prefixed lines continue the current STATEMENT body's SQL text.
	if strings.HasPrefix(line, "\t") {
		l.appendToStatement(line)
		return nil
	}

	// Bare-keyword continuation (no log_line_prefix) — used in tests and in
	// configurations without a log_line_prefix. Production lines carry the
	// prefix and are handled below by the prefixed-keyword path.
	if isBareContinuationLine(line) {
		l.processBareContinuation(line)
		return nil
	}

	// A new prefix-formatted line ends any in-progress STATEMENT accumulation.
	// Detect prefix-shaped lines via the format regex so we don't flush on
	// arbitrary unrelated input.
	prefixed := logFormatRegex.MatchString(line)
	if prefixed {
		l.flushStatement()
	}

	if !strings.Contains(line, "ERROR:") &&
		!strings.Contains(line, "FATAL:") &&
		!strings.Contains(line, "PANIC:") &&
		!strings.Contains(line, ":STATEMENT:") {
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
	atIdx := strings.Index(line, "@")
	afterAt := line[atIdx+1:]
	pidMarkerIdx := strings.Index(afterAt, ":[")

	database := strings.TrimSpace(afterAt[:pidMarkerIdx])

	if slices.Contains(l.excludeDatabases, database) {
		return nil
	}

	beforeAt := line[:atIdx]
	lastColonBeforeAt := strings.LastIndex(beforeAt, ":")
	user := strings.TrimSpace(beforeAt[lastColonBeforeAt+1:])

	if slices.Contains(l.excludeUsers, user) {
		return nil
	}

	// Extract SQLSTATE from format: [pid]:line_number:SQLSTATE:...
	pidEndIdx := strings.Index(afterAt, "]")
	afterPid := afterAt[pidEndIdx+1:]

	parts := strings.SplitN(afterPid, ":", 4)
	sqlstateCode := strings.TrimSpace(parts[2])
	sqlstateClass := ""
	if len(sqlstateCode) >= 2 {
		sqlstateClass = sqlstateCode[:2]
	}

	// Extract PID for STATEMENT/error correlation.
	pidStart := strings.Index(afterAt, "[")
	pidEnd := strings.Index(afterAt, "]")
	pid := ""
	if pidStart != -1 && pidEnd > pidStart {
		pid = afterAt[pidStart+1 : pidEnd]
	}

	// Detect STATEMENT continuation that arrived with a log_line_prefix.
	// The `:STATEMENT:` substring is unambiguous because the prefix ends
	// with `:` followed by the message keyword.
	if statementIdx := strings.Index(line, ":STATEMENT:"); statementIdx != -1 {
		stmt := strings.TrimSpace(line[statementIdx+len(":STATEMENT:"):])
		l.startStatement(pid, stmt)
		return nil
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

	if pid != "" {
		// Buffer the error so a matching STATEMENT continuation can stitch the
		// SQL text and we can record per-fingerprint cardinality.
		l.pendingMu.Lock()
		// If a previous error from this PID is still pending (no STATEMENT
		// arrived within the timeout window), the new error displaces it.
		// The displaced error is NOT credited to the fingerprint counter —
		// only pg_errors_total counts it. This keeps the new metric strictly
		// equal to "errors with successfully captured SQL".
		l.pendingErrors[pid] = &pendingError{
			receivedAt:    time.Now(),
			severity:      severity,
			sqlstate:      sqlstateCode,
			sqlstateClass: sqlstateClass,
			datname:       database,
		}
		l.pendingMu.Unlock()
	}

	return nil
}

// isBareContinuationLine reports whether a line is a continuation message that
// arrives without a log_line_prefix — only emitted by PostgreSQL when
// log_line_prefix is empty, but also useful for tests.
func isBareContinuationLine(line string) bool {
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

// processBareContinuation handles a STATEMENT continuation line that has no
// log_line_prefix. It matches the most recently buffered pending error.
//
// In a non-prefixed configuration we cannot extract the originating PID from
// the continuation line, so we rely on PostgreSQL's ereport mutex emitting
// ERROR + STATEMENT contiguously per backend.
func (l *Logs) processBareContinuation(line string) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "STATEMENT:") {
		return
	}
	stmt := strings.TrimSpace(strings.TrimPrefix(trimmed, "STATEMENT:"))

	l.pendingMu.Lock()
	var bestPID string
	var bestEntry *pendingError
	for pid, p := range l.pendingErrors {
		if bestEntry == nil || p.receivedAt.After(bestEntry.receivedAt) {
			bestPID = pid
			bestEntry = p
		}
	}
	if bestEntry != nil {
		delete(l.pendingErrors, bestPID)
	}
	l.pendingMu.Unlock()

	l.incrementByFingerprint(bestEntry, stmt)
}

// startStatement begins a new STATEMENT accumulation, flushing any prior
// in-progress buffer first.
func (l *Logs) startStatement(pid, initialSQL string) {
	l.statementMu.Lock()
	defer l.statementMu.Unlock()

	l.flushStatementLocked()
	l.currentStatement = &statementBuffer{
		pid:        pid,
		receivedAt: time.Now(),
	}
	l.currentStatement.sql.WriteString(initialSQL)
}

// appendToStatement appends a TAB-prefixed continuation line's SQL text to the
// in-progress STATEMENT buffer. Lines that arrive without an active buffer are
// silently dropped (they may be continuations of non-STATEMENT messages).
func (l *Logs) appendToStatement(line string) {
	l.statementMu.Lock()
	defer l.statementMu.Unlock()

	if l.currentStatement == nil {
		return
	}
	if l.currentStatement.sql.Len() > 0 {
		l.currentStatement.sql.WriteByte('\n')
	}
	l.currentStatement.sql.WriteString(strings.TrimLeft(line, "\t"))
}

// flushStatement consumes the in-progress STATEMENT buffer and matches it to
// the buffered pending error sharing its PID, incrementing the fingerprint
// counter. Safe to call when no statement is in progress.
func (l *Logs) flushStatement() {
	l.statementMu.Lock()
	defer l.statementMu.Unlock()
	l.flushStatementLocked()
}

func (l *Logs) flushStatementLocked() {
	if l.currentStatement == nil {
		return
	}
	sb := l.currentStatement
	l.currentStatement = nil

	stmt := strings.TrimSpace(sb.sql.String())
	if stmt == "" {
		return
	}

	var entry *pendingError
	l.pendingMu.Lock()
	if e, ok := l.pendingErrors[sb.pid]; ok {
		entry = e
		delete(l.pendingErrors, sb.pid)
	}
	l.pendingMu.Unlock()

	l.incrementByFingerprint(entry, stmt)
}

// incrementByFingerprint fingerprints stmt and increments the fingerprint
// counter against the pending error's labels. No-op if entry is nil or if
// the SQL fingerprints to ErrEmpty.
func (l *Logs) incrementByFingerprint(entry *pendingError, stmt string) {
	if entry == nil {
		return
	}
	fp, _, err := fingerprint.Fingerprint(stmt, fingerprint.SourceLog, 0)
	if err != nil {
		return
	}
	l.errorsByFingerprint.WithLabelValues(
		entry.severity,
		entry.sqlstate,
		entry.sqlstateClass,
		entry.datname,
		fp,
	).Inc()
}

// flushExpiredPending drops pending entries older than pendingErrorTimeout
// and flushes any in-progress STATEMENT buffer that has aged past the same
// window. Errors without a matching STATEMENT continuation never increment
// the pg_errors_by_fingerprint_total counter — they remain counted only on
// pg_errors_total.
//
// Order matters: an in-progress STATEMENT buffer is flushed first so it can
// consume its matching pending error before that pending is expired. The
// statement and its pending share a `receivedAt` of approximately the same
// instant (PG emits ERROR + STATEMENT contiguously per backend), so without
// this ordering both would expire on the same ticker fire and the statement
// would lose its pending — under-counting the fingerprint metric for any
// error whose STATEMENT precedes a long log gap.
func (l *Logs) flushExpiredPending() {
	deadline := time.Now().Add(-l.pendingErrorTimeout)

	l.statementMu.Lock()
	if l.currentStatement != nil && l.currentStatement.receivedAt.Before(deadline) {
		l.flushStatementLocked()
	}
	l.statementMu.Unlock()

	l.pendingMu.Lock()
	for pid, p := range l.pendingErrors {
		if p.receivedAt.Before(deadline) {
			delete(l.pendingErrors, pid)
		}
	}
	l.pendingMu.Unlock()
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
