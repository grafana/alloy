package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const OP_ERROR_MESSAGE = "error_message"

const (
	LogsCollector         = "logs"
	expectedLogLinePrefix = "%m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a"
	selectLogTimezone     = `SELECT setting FROM pg_settings WHERE name = 'log_timezone';`
)

// log_timezone is a sighup-reloadable Postgres setting so we periodically poll for changes
const logTimezoneRefreshInterval = time.Hour

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

// pgLogLabels are the leading labels PostgreSQL writes at the start of a log
// message: message severities plus the detail/continuation labels. We scan the
// full set (not just supportedSeverities) to find a line's real leading label
// so an "ERROR:"/"FATAL:"/"PANIC:" substring appearing later in the message or
// in a logged SQL statement (e.g. on a LOG or STATEMENT line) is shadowed by
// that line's actual label and not miscounted as an error.
//
// A match requires the label be followed by ":  " (colon, two spaces) — the
// exact separator PostgreSQL emits — so a label-like substring in the
// client-controlled application_name (%a, which sits between the SQLSTATE
// anchor and the real label, e.g. an app named "etl-LOG:worker") does not
// shadow the message's actual label.
var pgLogLabels = []string{
	"DEBUG5", "DEBUG4", "DEBUG3", "DEBUG2", "DEBUG1", "DEBUG",
	"PANIC", "FATAL", "ERROR", "WARNING", "NOTICE", "INFO", "LOG",
	"STATEMENT", "DETAIL", "HINT", "CONTEXT", "QUERY", "LOCATION",
}

// pgLabelSeparator is what PostgreSQL writes between a log label and the
// message text.
const pgLabelSeparator = ":  "

// pendingError holds an ERROR/FATAL/PANIC line awaiting its STATEMENT
// continuation. The SQL builder accumulates the STATEMENT keyword line plus any
// TAB-prefixed continuation lines that follow. pid is the backend PID from the
// error line's prefix; a STATEMENT line only attaches when its PID matches, so
// interleaved log streams cannot pair one backend's error with another's SQL.
type pendingError struct {
	receivedAt time.Time
	pid        string
	severity   string
	datname    string
	timestamp  time.Time

	sql          strings.Builder
	hasStatement bool
}

type LogsArguments struct {
	Receiver         loki.LogsReceiver
	EntryHandler     loki.EntryHandler
	Logger           *slog.Logger
	Registry         *prometheus.Registry
	ExcludeDatabases []string
	ExcludeUsers     []string
	EnableErrorLogs  bool
	DB               *sql.DB
}

type Logs struct {
	logger       *slog.Logger
	entryHandler loki.EntryHandler
	registry     *prometheus.Registry

	receiver         loki.LogsReceiver
	excludeDatabases []string
	excludeUsers     []string
	enableErrorLogs  bool

	db              *sql.DB
	logTimezone     atomic.Pointer[time.Location]
	lastLogTimezone atomic.Pointer[string]

	errorsBySQLState      *prometheus.CounterVec
	parseErrors           prometheus.Counter
	logsProcessingEnabled prometheus.Gauge

	ctx     context.Context
	cancel  context.CancelFunc
	stopped *atomic.Bool
	wg      sync.WaitGroup

	lastFormatWarning     time.Time
	validLogsThisMinute   int
	invalidLogsThisMinute int

	// op="error" pairing state, owned exclusively by the run goroutine
	// (parseTextLog and flushExpiredPending both run there), so it needs no lock.
	// PostgreSQL's logging collector writes each message (the ERROR line, its
	// STATEMENT, and continuations) atomically and contiguously per backend, so a
	// single in-flight pending is sufficient — no PID-keyed map is needed. The
	// pending's PID guard covers the residual risk of interleaved streams (e.g.
	// stderr without the logging collector): a mismatched STATEMENT is dropped
	// rather than mispaired.
	pending             *pendingError
	pendingErrorTimeout time.Duration

	startTime time.Time
}

func NewLogs(args LogsArguments) (*Logs, error) {
	ctx, cancel := context.WithCancel(context.Background())

	l := &Logs{
		logger:              args.Logger.With("collector", LogsCollector),
		entryHandler:        args.EntryHandler,
		registry:            args.Registry,
		receiver:            args.Receiver,
		excludeDatabases:    args.ExcludeDatabases,
		excludeUsers:        args.ExcludeUsers,
		enableErrorLogs:     args.EnableErrorLogs,
		db:                  args.DB,
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

	// Report whether logs processing (op="error" emission) is enabled for this
	// instance so consumers can detect which servers produce op="error" entries.
	l.logsProcessingEnabled = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "database_observability",
			Name:      "logs_processing_enabled",
			Help:      "Whether logs processing (error-log capture) is enabled for this database instance.",
		},
	)

	l.registry.MustRegister(
		l.errorsBySQLState,
		l.parseErrors,
		l.logsProcessingEnabled,
	)

	enabled := 0.0
	if l.enableErrorLogs {
		enabled = 1.0
	}
	l.logsProcessingEnabled.Set(enabled)
}

func (l *Logs) Name() string {
	return LogsCollector
}

// Receiver returns the logs receiver that loki.source.* can forward to
func (l *Logs) Receiver() loki.LogsReceiver {
	return l.receiver
}

func (l *Logs) Start(ctx context.Context) error {
	l.logger.Debug("collector started")

	if l.db != nil {
		l.refreshLogTimezone(l.ctx)
		l.wg.Go(l.logTimezoneRefreshLoop)
	}
	l.wg.Go(l.run)

	return nil
}

func (l *Logs) logTimezoneRefreshLoop() {
	ticker := time.NewTicker(logTimezoneRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			l.refreshLogTimezone(l.ctx)
		}
	}
}

func (l *Logs) refreshLogTimezone(ctx context.Context) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var tzName string
	if err := l.db.QueryRowContext(queryCtx, selectLogTimezone).Scan(&tzName); err != nil {
		l.logger.Debug("failed to query log_timezone", "err", err)
		return
	}

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		// PG accepts POSIX specs (e.g. 'EST5EDT,M3.2.0,M11.1.0') that Go's tzdata can't load.
		if prev := l.lastLogTimezone.Load(); prev == nil || *prev != tzName {
			l.logger.Warn("PostgreSQL log_timezone is not a Go-loadable IANA name; logs collector will skip its historical-log filter. Consider setting log_timezone to an IANA name (e.g. 'America/New_York') in postgresql.conf.", "log_timezone", tzName, "err", err)
			l.lastLogTimezone.Store(&tzName)
		}
		l.logTimezone.Store(nil)
		return
	}
	l.lastLogTimezone.Store(nil)
	l.logTimezone.Store(loc)
}

func (l *Logs) Stop() {
	l.cancel()
	l.stopped.Store(true)
	l.wg.Wait()

	l.registry.Unregister(l.errorsBySQLState)
	l.registry.Unregister(l.parseErrors)
	l.registry.Unregister(l.logsProcessingEnabled)
}

func (l *Logs) Stopped() bool {
	return l.stopped.Load()
}

func (l *Logs) run() {
	l.logger.Debug("collector running, waiting for log entries")

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
			l.logger.Debug("collector stopping")
			return
		case entry := <-l.receiver.Chan():
			if err := l.parseTextLog(entry); err != nil {
				l.logger.Warn(
					"failed to process log line",
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
			l.appendStatement(line)
		}
		return nil
	}

	isFormat := logFormatRegex.MatchString(line)

	// A new prefixed line means any in-flight ERROR+STATEMENT pair is complete;
	// emit it before handling this line.
	if l.enableErrorLogs && isFormat {
		l.flushPending()
	}

	hasErrorKeyword := strings.Contains(line, "ERROR:") ||
		strings.Contains(line, "FATAL:") ||
		strings.Contains(line, "PANIC:")
	hasStatementKeyword := l.enableErrorLogs && strings.Contains(line, "STATEMENT:")
	if !hasErrorKeyword && !hasStatementKeyword {
		return nil
	}

	if !isFormat {
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
					absolute, ok := l.resolveAbsolute(logTimestamp)
					if ok && !absolute.After(l.startTime) {
						return nil // Skip historical log
					}
					// Record the timestamp for the op="error" entry, preferring the
					// timezone-resolved instant; fall back to the raw parse when the
					// log_timezone can't be resolved (same case main declines to skip).
					if ok {
						parsedTimestamp = absolute
					} else {
						parsedTimestamp = logTimestamp
					}
					break
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

	pidEndIdx := strings.Index(afterAt, "]")
	pid := afterAt[pidMarkerIdx+2 : pidEndIdx]
	afterPid := afterAt[pidEndIdx+1:]

	parts := strings.SplitN(afterPid, ":", 4)
	sqlstateCode := strings.TrimSpace(parts[2])
	sqlstateClass := ""
	if len(sqlstateCode) >= 2 {
		sqlstateClass = sqlstateCode[:2]
	}

	// Classify the message once by its real label: the leftmost PostgreSQL log
	// label (with PostgreSQL's ":  " separator) after the SQLSTATE field.
	// Anchoring past the SQLSTATE keeps a database or user literally named
	// after a label (e.g. "LOG") from matching, and scanning the full label set
	// shadows a severity or STATEMENT keyword that appears inside the message
	// text or a logged SQL statement.
	searchFrom := 0
	if sqlstateCode != "" {
		if idx := strings.Index(line, ":"+sqlstateCode+":"); idx != -1 {
			searchFrom = idx + len(sqlstateCode) + 2
		}
	}

	label := ""
	labelAt := -1
	for _, candidate := range pgLogLabels {
		idx := strings.Index(line[searchFrom:], candidate+pgLabelSeparator)
		if idx != -1 && (labelAt == -1 || idx < labelAt) {
			labelAt = idx
			label = candidate
		}
	}

	if label == "STATEMENT" {
		if l.enableErrorLogs {
			sqlStart := searchFrom + labelAt + len("STATEMENT:")
			l.attachStatement(pid, strings.TrimSpace(line[sqlStart:]))
		}
		return nil
	}

	// A non-error message (LOG/WARNING/DETAIL/…) or no recognizable label:
	// don't count it, even if it contains an error keyword in its text.
	if _, ok := supportedSeverities[label]; !ok {
		return nil
	}

	l.errorsBySQLState.WithLabelValues(
		label,
		sqlstateCode,
		sqlstateClass,
		database,
		user,
	).Inc()

	if !l.enableErrorLogs {
		return nil
	}

	// Start a new pending error awaiting its STATEMENT. Any prior un-flushed
	// pending is displaced here (no STATEMENT captured → no op="error" entry);
	// pg_errors_total still counted it above.
	l.pending = &pendingError{
		receivedAt: time.Now(),
		pid:        pid,
		severity:   label,
		datname:    database,
		timestamp:  parsedTimestamp,
	}

	return nil
}

// resolveAbsolute returns a trustworthy UTC instant. time.Parse fabricates a
// zero-offset Location for unknown abbreviations (PST, PDT, EDT, ...); we
// recover the real instant via the configured log_timezone, trusting the
// recovery only when its abbreviation matches the log line's.
func (l *Logs) resolveAbsolute(parsed time.Time) (time.Time, bool) {
	name, offset := parsed.Zone()
	if offset != 0 || name == "UTC" || name == "GMT" {
		return parsed, true
	}

	loc := l.logTimezone.Load()
	if loc == nil {
		return time.Time{}, false
	}

	y, mo, d := parsed.Date()
	h, mi, s := parsed.Clock()
	reconstructed := time.Date(y, mo, d, h, mi, s, parsed.Nanosecond(), loc)
	reconstructedName, _ := reconstructed.Zone()
	if reconstructedName != name {
		return time.Time{}, false
	}
	return reconstructed, true
}

// appendStatement appends a TAB-continuation line to the in-flight STATEMENT's SQL.
func (l *Logs) appendStatement(line string) {
	if l.pending == nil || !l.pending.hasStatement {
		return
	}
	if l.pending.sql.Len() > 0 {
		l.pending.sql.WriteByte('\n')
	}
	l.pending.sql.WriteString(strings.TrimLeft(line, "\t"))
}

// attachStatement records the STATEMENT keyword line's SQL onto the pending
// error, provided the line's backend PID matches the pending's. A mismatched
// PID means the streams interleaved; the pending is left in place to be
// displaced by the next error or dropped on timeout.
func (l *Logs) attachStatement(pid, sql string) {
	if l.pending == nil || l.pending.pid != pid {
		return
	}
	if l.pending.sql.Len() > 0 {
		l.pending.sql.WriteByte('\n')
	}
	l.pending.sql.WriteString(sql)
	l.pending.hasStatement = true
}

// flushPending emits the pending error if its STATEMENT was captured, then
// clears it. A pending without a STATEMENT is left in place — it is displaced
// by the next error or dropped on timeout.
func (l *Logs) flushPending() {
	p := l.pending
	if p == nil || !p.hasStatement {
		return
	}
	l.pending = nil

	l.emitErrorEntry(p)
}

func (l *Logs) emitErrorEntry(p *pendingError) {
	stmt := strings.TrimSpace(p.sql.String())
	if stmt == "" {
		return
	}
	fp, err := fingerprint.Fingerprint(stmt)
	if err != nil {
		return
	}

	ts := p.timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	l.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		OP_ERROR_MESSAGE,
		buildErrorLine(p, fp),
		ts.UnixNano(),
	)
}

// buildErrorLine assembles the minimal logfmt body for one ERROR + STATEMENT
// pair: just the fields needed to compute per-query error rate. The SQL text
// and error-detail fields are intentionally omitted (deferred to a follow-up
// PR); consumers recover the SQL by joining on query_fingerprint.
func buildErrorLine(p *pendingError, fp string) string {
	type kv struct{ k, v string }
	fields := []kv{
		{"severity", p.severity},
		{"datname", p.datname},
		{"query_fingerprint", fp},
	}

	var b strings.Builder
	for _, f := range fields {
		if f.v == "" {
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

// flushExpiredPending handles a pending error older than pendingErrorTimeout
// that no following log line has flushed: emit it if its STATEMENT was captured
// (its continuations have all arrived by now), otherwise drop it.
func (l *Logs) flushExpiredPending() {
	deadline := time.Now().Add(-l.pendingErrorTimeout)

	p := l.pending
	if p == nil || !p.receivedAt.Before(deadline) {
		return
	}
	l.pending = nil

	if p.hasStatement {
		l.emitErrorEntry(p)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// trackValidFormat tracks that we've seen a valid log format this minute
func (l *Logs) trackValidFormat() {
	l.validLogsThisMinute++
}

// trackInvalidFormat tracks invalid format and emits warning once per minute if ALL logs were invalid
func (l *Logs) trackInvalidFormat() {
	l.invalidLogsThisMinute++

	// Emit warning once per minute if ALL logs were invalid
	now := time.Now()
	if now.Sub(l.lastFormatWarning) >= time.Minute {
		if l.validLogsThisMinute == 0 && l.invalidLogsThisMinute > 0 {
			l.logger.Warn(
				"all PostgreSQL error logs in the last minute had invalid format",
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
