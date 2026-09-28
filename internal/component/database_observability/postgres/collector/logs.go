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

	"github.com/hoophq/alcatraz"
	"github.com/hoophq/alcatraz/anonymizer"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/alloy/internal/runtime/logging"
)

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

// sessionMetaRegex matches the %s:%v:%x:%c run of the required log_line_prefix
// (session start time, virtual transaction id, transaction id, session id),
// anchored at the start of the text right after the SQLSTATE. All four are
// server-generated (never client-controlled), unlike the %a that follows them,
// so no anti-spoofing care is needed here. A future log_line_prefix variant
// that omits this run simply fails to match; callers fall back to pid-based
// pairing in that case (see attachContinuation).
var sessionMetaRegex = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d{3})? \S+):(\d+/\d+):(\d+):([^:]+):`,
)

var supportedSeverities = map[string]struct{}{
	"ERROR": {},
	"FATAL": {},
	"PANIC": {},
}

// Continuation labels that carry captured, redacted text onto a pendingError
// (see pendingError.builderAndFlagFor).
const (
	pgLabelStatement = "STATEMENT"
	pgLabelDetail    = "DETAIL"
	pgLabelHint      = "HINT"
	pgLabelContext   = "CONTEXT"
)

// pgLogLabels are the labels PostgreSQL can write at the start of a log message:
// the severities plus the detail/continuation labels. Classification scans the
// full set (not just supportedSeverities) so a non-severity label can shadow a
// severity keyword that appears later in the message text or in logged SQL.
var pgLogLabels = []string{
	"DEBUG5", "DEBUG4", "DEBUG3", "DEBUG2", "DEBUG1", "DEBUG",
	"PANIC", "FATAL", "ERROR", "WARNING", "NOTICE", "INFO", "LOG",
	pgLabelStatement, pgLabelDetail, pgLabelHint, pgLabelContext, "QUERY", "LOCATION",
}

// pgLabelSeparator is what PostgreSQL writes between a log label and the
// message text.
const pgLabelSeparator = ":  "

// leadingLogLabel returns the PostgreSQL log label that s begins with (i.e. s
// starts with "<label>:  "), or "" if s does not start with a known label.
func leadingLogLabel(s string) string {
	for _, candidate := range pgLogLabels {
		if strings.HasPrefix(s, candidate+pgLabelSeparator) {
			return candidate
		}
	}
	return ""
}

// pendingError holds an ERROR/FATAL/PANIC line awaiting its STATEMENT
// continuation. The SQL builder accumulates the STATEMENT keyword line plus any
// TAB-prefixed continuation lines that follow. pid is the backend PID from the
// error line's prefix; a STATEMENT line only attaches when its PID matches, so
// interleaved log streams cannot pair one backend's error with another's SQL.
type pendingError struct {
	receivedAt    time.Time
	pid           string
	severity      string
	datname       string
	user          string
	sqlstate      string
	sqlstateClass string
	timestamp     time.Time
	message       string

	// Session-metadata fields already present in the required log_line_prefix.
	// All server-generated (never client-controlled), so none need redaction.
	lineNumber       string
	sessionStartTime string
	vxid             string
	xid              string
	sessionID        string // %c; preferred over pid for pairing when available (see attachContinuation)
	applicationName  string // %a; client-controlled, redacted like message/detail

	sql          strings.Builder
	hasStatement bool

	detail    strings.Builder
	hasDetail bool

	hint    strings.Builder
	hasHint bool

	context    strings.Builder
	hasContext bool

	// activeField is the label (pgLabelStatement, pgLabelDetail, pgLabelHint,
	// or pgLabelContext) that a following bare TAB-continuation line (no
	// embedded label of its own) appends to. Empty when no such field is open.
	activeField string
}

type LogsArguments struct {
	Receiver                  loki.LogsReceiver
	EntryHandler              loki.EntryHandler
	Logger                    *slog.Logger
	Registry                  *prometheus.Registry
	ExcludeDatabases          []string
	ExcludeUsers              []string
	EnableErrorLogsProcessing bool
	DB                        *sql.DB
}

type Logs struct {
	logger       *slog.Logger
	entryHandler loki.EntryHandler
	registry     *prometheus.Registry

	receiver                  loki.LogsReceiver
	excludeDatabases          []string
	excludeUsers              []string
	enableErrorLogsProcessing bool

	db              *sql.DB
	logTimezone     atomic.Pointer[time.Location]
	lastLogTimezone atomic.Pointer[string]

	errorsBySQLState      *prometheus.CounterVec
	parseErrors           prometheus.Counter
	logsProcessingEnabled prometheus.Gauge

	piiEngine *alcatraz.Engine

	ctx     context.Context
	cancel  context.CancelFunc
	stopped *atomic.Bool
	wg      sync.WaitGroup

	lastFormatWarning     time.Time
	validLogsThisMinute   int
	invalidLogsThisMinute int

	// op="error_message" pairing state, owned exclusively by the run goroutine
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
	// Fail loudly instead of silently emitting nothing: the error surfaces via
	// startCollectors into the component's health status.
	if args.EnableErrorLogsProcessing && !fingerprint.Supported() {
		return nil, fmt.Errorf("logs.enable_error_logs_processing requires a cgo-enabled Alloy build (CGO_ENABLED=1)")
	}

	ctx, cancel := context.WithCancel(context.Background())

	l := &Logs{
		logger:                    args.Logger.With("collector", LogsCollector),
		entryHandler:              args.EntryHandler,
		registry:                  args.Registry,
		receiver:                  args.Receiver,
		excludeDatabases:          args.ExcludeDatabases,
		excludeUsers:              args.ExcludeUsers,
		enableErrorLogsProcessing: args.EnableErrorLogsProcessing,
		db:                        args.DB,
		pendingErrorTimeout:       5 * time.Second,
		ctx:                       ctx,
		cancel:                    cancel,
		stopped:                   atomic.NewBool(false),
		startTime:                 time.Now(),
		piiEngine:                 alcatraz.NewEngine(),
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
		[]string{"severity", "sqlstate", "sqlstate_class", labelDatname, "user"},
	)

	l.parseErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "database_observability",
			Name:      "pg_error_log_parse_failures_total",
			Help:      "Number of log lines with errors that failed to parse",
		},
	)

	// Report whether logs processing (op="error_message" emission) is enabled for this
	// instance so consumers can detect which servers produce op="error_message" entries.
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
	if l.enableErrorLogsProcessing {
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
	if l.enableErrorLogsProcessing {
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
		if l.enableErrorLogsProcessing {
			l.appendToActiveField(line)
		}
		return nil
	}

	// Cheap keyword gate before the (expensive) format regex: most lines (e.g.
	// LOG statements) exit here. The one other line that matters is the boundary
	// line that flushes a completed ERROR+STATEMENT pair, only possible when
	// such a pair is actually buffered.
	hasErrorKeyword := strings.Contains(line, "ERROR:") ||
		strings.Contains(line, "FATAL:") ||
		strings.Contains(line, "PANIC:")
	hasContinuationKeyword := l.enableErrorLogsProcessing &&
		(strings.Contains(line, "STATEMENT:") || strings.Contains(line, "DETAIL:") ||
			strings.Contains(line, "HINT:") || strings.Contains(line, "CONTEXT:"))
	mayFlush := l.enableErrorLogsProcessing && l.pending != nil && l.pending.hasStatement
	if !hasErrorKeyword && !hasContinuationKeyword && !mayFlush {
		return nil
	}

	isFormat := logFormatRegex.MatchString(line)

	// A new prefixed line means any in-flight ERROR+STATEMENT pair is complete;
	// emit it before handling this line.
	if l.enableErrorLogsProcessing && isFormat {
		l.flushPending()
	}

	if !hasErrorKeyword && !hasContinuationKeyword {
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
					// Record the timestamp for the op="error_message" entry only when the
					// timezone resolved. time.Parse fabricates a zero-offset instant
					// for unknown abbreviations, which can be hours wrong (and Loki
					// rejects future timestamps); unresolved lines fall back to
					// arrival time at emit instead.
					if ok {
						parsedTimestamp = absolute
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

	// Classify by the line's real label. PostgreSQL appends the label (with the
	// ":  " separator) at the end of the log-line prefix, right after the
	// client-controlled application_name (%a) — which can hold a forged
	// "<label>:  ". Anchor past the SQLSTATE (so a db/user named "LOG" can't
	// match), then take the last token in the run of adjacent "<label>:  "
	// tokens: a forgery in %a always precedes the label PostgreSQL emits last.
	//
	// Residual: a STATEMENT/QUERY line whose logged SQL begins with "<label>:  "
	// makes the walk overshoot into the SQL. Only reachable by deliberate abuse,
	// self-scoped, and such SQL is unparseable anyway; normal SQL never starts
	// with a label, so real error/statement pairs are unaffected.
	searchFrom := 0
	if sqlstateCode != "" {
		if idx := strings.Index(line, ":"+sqlstateCode+":"); idx != -1 {
			searchFrom = idx + len(sqlstateCode) + 2
		}
	}

	rest := line[searchFrom:]
	label := ""
	labelAt := -1
	for _, candidate := range pgLogLabels {
		idx := strings.Index(rest, candidate+pgLabelSeparator)
		if idx != -1 && (labelAt == -1 || idx < labelAt) {
			labelAt = idx
			label = candidate
		}
	}
	// Advance through any forged "<label>:  " tokens in %a to the real (last) label.
	for label != "" {
		next := labelAt + len(label) + len(pgLabelSeparator)
		nextLabel := leadingLogLabel(rest[next:])
		if nextLabel == "" {
			break
		}
		label, labelAt = nextLabel, next
	}

	lineNumber := strings.TrimSpace(parts[1])

	// %s:%v:%x:%c, all server-generated. A prefix variant lacking this run
	// (e.g. one missing %v/%x/%c) simply fails to match here; sessionID stays
	// "" and callers fall back to pid-based pairing.
	var sessionStartTime, vxid, xid, sessionID, applicationName string
	if m := sessionMetaRegex.FindStringSubmatchIndex(rest); m != nil {
		sessionStartTime = rest[m[2]:m[3]]
		vxid = rest[m[4]:m[5]]
		xid = rest[m[6]:m[7]]
		sessionID = rest[m[8]:m[9]]
		if labelAt > m[1] {
			applicationName = strings.TrimSuffix(strings.TrimSpace(rest[m[1]:labelAt]), ":")
		}
	}

	switch label {
	case pgLabelStatement, pgLabelDetail, pgLabelHint, pgLabelContext:
		if l.enableErrorLogsProcessing {
			textStart := searchFrom + labelAt + len(label) + 1
			l.attachContinuation(pid, sessionID, label, strings.TrimSpace(line[textStart:]))
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

	if !l.enableErrorLogsProcessing {
		return nil
	}

	msgStart := searchFrom + labelAt + len(label) + 1
	message := strings.TrimSpace(line[msgStart:])

	// Start a new pending error awaiting its STATEMENT. Any prior un-flushed
	// pending is displaced here (no STATEMENT captured → no op="error_message" entry);
	// pg_errors_total still counted it above.
	l.pending = &pendingError{
		receivedAt:       time.Now(),
		pid:              pid,
		severity:         label,
		datname:          database,
		user:             user,
		sqlstate:         sqlstateCode,
		sqlstateClass:    sqlstateClass,
		timestamp:        parsedTimestamp,
		message:          message,
		lineNumber:       lineNumber,
		sessionStartTime: sessionStartTime,
		vxid:             vxid,
		xid:              xid,
		sessionID:        sessionID,
		applicationName:  applicationName,
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

// builderAndFlagFor returns the builder and "has content" flag for one of the
// pendingError's continuation fields, or (nil, nil) for an unrecognized label.
func (p *pendingError) builderAndFlagFor(label string) (*strings.Builder, *bool) {
	switch label {
	case pgLabelStatement:
		return &p.sql, &p.hasStatement
	case pgLabelDetail:
		return &p.detail, &p.hasDetail
	case pgLabelHint:
		return &p.hint, &p.hasHint
	case pgLabelContext:
		return &p.context, &p.hasContext
	default:
		return nil, nil
	}
}

// appendLine appends text to b as a new line, adding the separating newline
// only when b already holds a prior line.
func appendLine(b *strings.Builder, text string) {
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	b.WriteString(text)
}

// appendToActiveField appends a TAB-continuation line (no label of its own)
// to whichever field (STATEMENT, DETAIL, HINT, or CONTEXT) was most recently
// opened by attachContinuation. A bare line arriving with no field open is
// dropped.
func (l *Logs) appendToActiveField(line string) {
	if l.pending == nil {
		return
	}
	b, _ := l.pending.builderAndFlagFor(l.pending.activeField)
	if b == nil {
		return
	}
	appendLine(b, strings.TrimLeft(line, "\t"))
}

// attachContinuation records a STATEMENT/DETAIL/HINT/CONTEXT keyword line's
// text onto the pending error, provided the line's identity matches the
// pending's (session id when both have one, pid otherwise — see the matching
// note above). A mismatch means the streams interleaved; the pending is left
// in place to be displaced by the next error or dropped on timeout. Opening a
// field makes it "active" so a following bare continuation line (see
// appendToActiveField) knows which builder to extend.
func (l *Logs) attachContinuation(pid, sessionID, label, text string) {
	if l.pending == nil {
		return
	}
	// %c (session id) is preferred when both sides have it: it can't be
	// reused the way a backend pid can be, over a long-lived instance's
	// uptime. Fall back to pid when either side lacks it (a log_line_prefix
	// variant without %c).
	var matches bool
	if sessionID != "" && l.pending.sessionID != "" {
		matches = l.pending.sessionID == sessionID
	} else {
		matches = l.pending.pid == pid
	}
	if !matches {
		return
	}
	b, has := l.pending.builderAndFlagFor(label)
	if b == nil {
		return
	}
	appendLine(b, text)
	*has = true
	l.pending.activeField = label
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

// redact masks any PII the engine recognizes in free-form error text (message,
// DETAIL, HINT, CONTEXT) before it leaves the process. Structured fields
// (severity, sqlstate, datname, user, pid) never go through this path.
func (l *Logs) redact(text string) string {
	if text == "" {
		return text
	}
	hits := l.piiEngine.Analyze(text, alcatraz.Options{})
	return anonymizer.Anonymize(text, hits, anonymizer.Redact())
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

	// severity, pid, sqlstate, sqlstate_class, and fp never need quoting;
	// %q escapes datname, user, and message.
	body := fmt.Sprintf("severity=%s datname=%q query_fingerprint=%s user=%q pid=%s sqlstate=%s sqlstate_class=%s message=%q",
		p.severity, p.datname, fp, p.user, p.pid, p.sqlstate, p.sqlstateClass, l.redact(p.message))

	if p.lineNumber != "" {
		body += fmt.Sprintf(" line_number=%s", p.lineNumber)
	}
	if p.sessionStartTime != "" {
		body += fmt.Sprintf(" session_start_time=%q", p.sessionStartTime)
	}
	if p.vxid != "" {
		body += fmt.Sprintf(" vxid=%s", p.vxid)
	}
	if p.xid != "" {
		body += fmt.Sprintf(" xid=%s", p.xid)
	}
	if p.sessionID != "" {
		body += fmt.Sprintf(" session_id=%s", p.sessionID)
	}
	if p.applicationName != "" {
		body += fmt.Sprintf(" application_name=%q", l.redact(p.applicationName))
	}

	if p.hasDetail {
		body += fmt.Sprintf(" detail=%q", l.redact(strings.TrimSpace(p.detail.String())))
	}
	if p.hasHint {
		body += fmt.Sprintf(" hint=%q", l.redact(strings.TrimSpace(p.hint.String())))
	}
	if p.hasContext {
		body += fmt.Sprintf(" context=%q", l.redact(strings.TrimSpace(p.context.String())))
	}

	// Blocking send by design (backpressure over dropping entries), but guarded
	// by the collector context so a stalled downstream can't wedge Stop().
	select {
	case l.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		database_observability.OP_ERROR_MESSAGE,
		body,
		ts.UnixNano(),
	):
	case <-l.ctx.Done():
	}
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
