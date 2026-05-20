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
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const OP_ERROR = "error"

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
	receivedAt time.Time

	severity      string
	sqlstate      string
	sqlstateClass string
	datname       string
	user          string

	timestamp       time.Time
	clientAddr      string
	clientPort      string
	pid             string
	backendStart    string
	xid             string
	sessionID       string
	applicationName string
	errorMessage    string
}

// statementBuffer accumulates a STATEMENT continuation across the keyword line
// and the zero-or-more TAB-prefixed lines that follow.
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
	EnableErrorLogs  bool
}

type Logs struct {
	logger       log.Logger
	entryHandler loki.EntryHandler
	registry     *prometheus.Registry

	receiver         loki.LogsReceiver
	excludeDatabases []string
	excludeUsers     []string
	enableErrorLogs  bool

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
		logger:              log.With(args.Logger, "collector", LogsCollector),
		entryHandler:        args.EntryHandler,
		registry:            args.Registry,
		receiver:            args.Receiver,
		excludeDatabases:    args.ExcludeDatabases,
		excludeUsers:        args.ExcludeUsers,
		enableErrorLogs:     args.EnableErrorLogs,
		pendingErrors:       make(map[string]*pendingError),
		pendingErrorTimeout: 5 * time.Second,
		ctx:                 ctx,
		cancel:              cancel,
		stopped:             atomic.NewBool(false),
		startTime:           time.Now(),
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

	l.wg.Go(l.run)

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
	level.Debug(l.logger).Log("msg", "collector running, waiting for log entries")

	var tickerC <-chan time.Time
	if l.enableErrorLogs {
		tickPeriod := l.pendingErrorTimeout / 2
		if tickPeriod < 50*time.Millisecond {
			tickPeriod = 50 * time.Millisecond
		}
		t := time.NewTicker(tickPeriod)
		defer t.Stop()
		tickerC = t.C
	}

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
		case <-tickerC:
			l.flushExpiredPending()
		}
	}
}

func (l *Logs) parseTextLog(entry loki.Entry) error {
	line := entry.Entry.Line

	if strings.HasPrefix(line, "\t") {
		if l.enableErrorLogs {
			l.appendToStatement(line)
		}
		return nil
	}

	// Bare-keyword continuations are only emitted when log_line_prefix is empty.
	if isBareContinuationLine(line) {
		if l.enableErrorLogs {
			l.processBareContinuation(line)
		}
		return nil
	}

	if l.enableErrorLogs && logFormatRegex.MatchString(line) {
		l.flushStatement()
	}

	hasErrorKeyword := strings.Contains(line, "ERROR:") ||
		strings.Contains(line, "FATAL:") ||
		strings.Contains(line, "PANIC:")
	hasStatement := l.enableErrorLogs && strings.Contains(line, ":STATEMENT:")
	if !hasErrorKeyword && !hasStatement {
		return nil
	}

	if !logFormatRegex.MatchString(line) {
		l.trackInvalidFormat()
		l.parseErrors.Inc()
		return fmt.Errorf("log line does not match expected format")
	}

	l.trackValidFormat()

	var parsedTimestamp time.Time
	if len(line) > 30 {
		colonIdx := strings.Index(line[20:], ":")
		if colonIdx > 0 {
			timestampStr := strings.TrimSpace(line[:20+colonIdx])

			// "YYYY-MM-DD HH:MM:SS[.mmm] TZ:..." where TZ can be GMT, UTC, -03, etc.
			for _, layout := range []string{
				"2006-01-02 15:04:05.000 MST",
				"2006-01-02 15:04:05.000 -07",
				"2006-01-02 15:04:05 MST",
				"2006-01-02 15:04:05 -07",
			} {
				logTimestamp, err := time.Parse(layout, timestampStr)
				if err == nil {
					if !logTimestamp.After(l.startTime) {
						return nil
					}
					parsedTimestamp = logTimestamp
					break
				}
			}
		}
	}

	// Parse log line prefix format: %m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a:
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

	pidEndIdx := strings.Index(afterAt, "]")
	afterPid := afterAt[pidEndIdx+1:]

	parts := strings.SplitN(afterPid, ":", 4)
	sqlstateCode := strings.TrimSpace(parts[2])
	sqlstateClass := ""
	if len(sqlstateCode) >= 2 {
		sqlstateClass = sqlstateCode[:2]
	}

	pidStart := strings.Index(afterAt, "[")
	pidEnd := strings.Index(afterAt, "]")
	pid := ""
	if pidStart != -1 && pidEnd > pidStart {
		pid = afterAt[pidStart+1 : pidEnd]
	}

	if hasStatement {
		if statementIdx := strings.Index(line, ":STATEMENT:"); statementIdx != -1 {
			stmt := strings.TrimSpace(line[statementIdx+len(":STATEMENT:"):])
			l.startStatement(pid, stmt)
			return nil
		}
	}

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

	l.errorsBySQLState.WithLabelValues(
		severity,
		sqlstateCode,
		sqlstateClass,
		database,
		user,
	).Inc()

	if !l.enableErrorLogs || pid == "" {
		return nil
	}

	clientAddr, clientPort := parseRemote(extractRemoteSegment(line, lastColonBeforeAt))
	backendStart, xidRaw, sessionID, applicationName := parsePrefixTail(line, atIdx, pidEnd)
	xid := xidRaw
	if xid == "0" {
		xid = ""
	}

	errorMessage := ""
	if msgBody := line[messageStart+len(severity)+1:]; len(msgBody) > 0 {
		errorMessage = strings.TrimLeft(msgBody, " ")
		errorMessage = strings.TrimRight(errorMessage, " \t")
	}

	// A prior pending error from this PID is displaced and never reaches
	// op="error" — pg_errors_total still counts it. This keeps the op count
	// equal to "errors with captured SQL".
	l.pendingMu.Lock()
	l.pendingErrors[pid] = &pendingError{
		receivedAt:      time.Now(),
		severity:        severity,
		sqlstate:        sqlstateCode,
		sqlstateClass:   sqlstateClass,
		datname:         database,
		user:            user,
		timestamp:       parsedTimestamp,
		clientAddr:      clientAddr,
		clientPort:      clientPort,
		pid:             pid,
		backendStart:    backendStart,
		xid:             xid,
		sessionID:       sessionID,
		applicationName: applicationName,
		errorMessage:    errorMessage,
	}
	l.pendingMu.Unlock()

	return nil
}

// parsePrefixTail extracts %s, %x, %c, %a from a prefix-formatted line.
// The prefix ends `...:<%c>:<%a>:KEYWORD:`. Between `]` (after [%p]) and the
// `:` before %a sits `%l:%e:%s:%v:%x:%c`, and %s contributes 3 colon-separated
// fields (HH:MM:SS), so the segment splits into exactly 8 parts.
func parsePrefixTail(line string, atIdx, pidEnd int) (backendStart, xidRaw, sessionID, applicationName string) {
	kwStart := findKeywordPos(line)
	if kwStart == -1 {
		return
	}
	appEnd := strings.LastIndex(line[:kwStart], ":")
	appStart := strings.LastIndex(line[:appEnd], ":") + 1
	if appStart > 0 && appStart < appEnd {
		applicationName = line[appStart:appEnd]
	}

	segL := atIdx + 1 + pidEnd + 2
	segR := appStart - 1
	if segL >= segR || segR > len(line) {
		return
	}
	parts := strings.Split(line[segL:segR], ":")
	if len(parts) < 8 {
		return
	}
	// parts: %l, %e, HH, MM, SS+tz, %v, %x, %c
	backendStart = parts[2] + ":" + parts[3] + ":" + parts[4]
	xidRaw = parts[6]
	sessionID = parts[7]
	return
}

// extractRemoteSegment returns the %r portion of a prefixed line. Skip past
// the timestamp's HH:MM:SS before scanning for the colon that ends %m.
func extractRemoteSegment(line string, lastColonBeforeAt int) string {
	if len(line) < 30 {
		return ""
	}
	rel := strings.Index(line[20:], ":")
	if rel < 0 {
		return ""
	}
	rStart := 20 + rel + 1
	if rStart >= lastColonBeforeAt {
		return ""
	}
	return line[rStart:lastColonBeforeAt]
}

func parseRemote(s string) (host, port string) {
	if s == "" || s == "[local]" {
		return s, ""
	}
	open := strings.LastIndex(s, "(")
	close := strings.LastIndex(s, ")")
	if open == -1 || close == -1 || close <= open {
		return s, ""
	}
	return s[:open], s[open+1 : close]
}

func findKeywordPos(line string) int {
	keywords := []string{"ERROR", "FATAL", "PANIC", "STATEMENT", "DETAIL", "HINT", "CONTEXT", "QUERY", "LOCATION"}
	best := -1
	for _, kw := range keywords {
		if idx := strings.Index(line, ":"+kw+":"); idx != -1 && (best == -1 || idx < best) {
			best = idx
		}
	}
	if best == -1 {
		return -1
	}
	return best + 1 // skip the leading ':'
}

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

// processBareContinuation matches a no-prefix STATEMENT to the most recent
// pending error — PostgreSQL's ereport mutex emits ERROR + STATEMENT
// contiguously per backend.
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

	l.emitErrorEntry(bestEntry, stmt)
}

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

	l.emitErrorEntry(entry, stmt)
}

func (l *Logs) emitErrorEntry(entry *pendingError, stmt string) {
	if entry == nil {
		return
	}
	fp, _, err := fingerprint.Fingerprint(stmt, fingerprint.SourceLog, 0)
	if err != nil {
		return
	}

	body := buildErrorLine(entry, fp)
	ts := entry.timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	l.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		OP_ERROR,
		body,
		ts.UnixNano(),
	)
}

// buildErrorLine assembles the logfmt body for one ERROR + STATEMENT pair.
// The SQL is not emitted; consumers join on (query_fingerprint, pid) to
// recover it from the query_samples / query_details streams.
func buildErrorLine(entry *pendingError, fp string) string {
	type kv struct{ k, v string }
	fields := []kv{
		{"severity", entry.severity},
		{"sqlstate", entry.sqlstate},
		{"sqlstate_class", entry.sqlstateClass},
		{"xid", entry.xid},
		{"datname", entry.datname},
		{"query_fingerprint", fp},
		{"pid", entry.pid},
		{"backend_start", entry.backendStart},
		{"application_name", entry.applicationName},
		{"client_addr", entry.clientAddr},
		{"client_port", entry.clientPort},
		{"session_id", entry.sessionID},
		{"user", entry.user},
		{"error_message", entry.errorMessage},
	}

	var b strings.Builder
	for _, f := range fields {
		// error_message is always emitted, even when empty.
		if f.v == "" && f.k != "error_message" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(f.k)
		b.WriteByte('=')
		b.WriteString(logfmtQuote(f.v))
	}
	return b.String()
}

func logfmtQuote(v string) string {
	if !strings.ContainsAny(v, " =\"\\") {
		return v
	}
	var b strings.Builder
	b.Grow(len(v) + 2)
	b.WriteByte('"')
	for _, r := range v {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// flushExpiredPending drops pending entries older than pendingErrorTimeout.
// The statement buffer flushes FIRST so it can consume its matching pending
// before the pending is expired — both share approximately the same
// receivedAt, so without this order they'd expire on the same ticker fire.
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
