package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	LogsCollector = "logs"

	selectStatementDigest = `SELECT STATEMENT_DIGEST(?)`

	digestLookupTimeout = 5 * time.Second
)

// MySQL 8.0+ structured error-log line:
// <ISO8601>Z <thread_id> [<Severity>] [MY-xxxxxx] [<Subsystem>] <message>
// e.g. "2020-08-06T14:25:02.835618Z 0 [Note] [MY-012487] [InnoDB] DDL log recovery : begin"
var errorLogFormatRegex = regexp.MustCompile(
	`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z\s+(\d+)\s+\[(System|Error|Warning|Note)\]\s+\[MY-(\d{6})\]\s+\[(\w+)\]\s+(.*)$`,
)

// Slow query log block header lines, e.g.:
//
//	# Time: 2024-01-15T10:23:45.123456Z
//	# User@Host: user[user] @ host [ip]  Id: 123
//	# Query_time: 1.234567  Lock_time: 0.000123  Rows_sent: 10  Rows_examined: 1000
//	SET timestamp=1705315425;
//	SELECT ...;
var (
	slowLogTimeRegex         = regexp.MustCompile(`^#\s*Time:`)
	slowLogUserHostRegex     = regexp.MustCompile(`^#\s*User@Host:\s*\S*\[([^\]]*)\]\s*@.*?Id:\s*(\d+)`)
	slowLogStatsRegex        = regexp.MustCompile(`^#\s*Query_time:\s*(\S+)\s+Lock_time:\s*(\S+)\s+Rows_sent:\s*(\d+)\s+Rows_examined:\s*(\d+)`)
	slowLogSetTimestampRegex = regexp.MustCompile(`^SET timestamp=\d+;$`)
)

// General query log line: Time<TAB>Id<TAB>Command<TAB>Argument. Time is
// only populated on the first line of a timestamp group; the exact column
// spacing here is a best-effort shape pending verification against a real
// sample (see the implementation plan's open items).
var generalLogLineRegex = regexp.MustCompile(`^(\S*)\t\s*(\d+)\s+([A-Za-z][A-Za-z ]*?)\t(.*)$`)

// numericLogFields holds the field names that print unquoted in the logfmt
// body; everything else is %q-quoted.
var numericLogFields = map[string]struct{}{
	"thread_id":         {},
	"query_time":        {},
	"lock_time":         {},
	"rows_sent":         {},
	"rows_examined":     {},
	"replication_errno": {},
	"binlog_position":   {},
	"pid":               {},
	"port":              {},
}

type LogsArguments struct {
	Receiver       loki.LogsReceiver
	EntryHandler   loki.EntryHandler
	Logger         *slog.Logger
	Registry       *prometheus.Registry
	DB             *sql.DB
	ExcludeSchemas []string
	ExcludeUsers   []string
}

// slowQueryBlock accumulates one slow-query-log entry across its header
// lines and (in-memory only) SQL text, keyed per log source so that
// interleaved lines from a different file tailed into the same receiver
// can't corrupt it. The SQL text is never emitted: it exists only long
// enough to pass to STATEMENT_DIGEST(), then is discarded.
type slowQueryBlock struct {
	threadID     string
	user         string
	queryTime    string
	lockTime     string
	rowsSent     string
	rowsExamined string
	sql          strings.Builder
}

type Logs struct {
	logger       *slog.Logger
	entryHandler loki.EntryHandler
	registry     *prometheus.Registry
	receiver     loki.LogsReceiver
	db           *sql.DB

	excludeSchemas []string
	excludeUsers   []string

	errorsByCode  *prometheus.CounterVec
	slowQueries   prometheus.Counter
	parseFailures *prometheus.CounterVec

	// pendingSlowQueries holds the in-flight slow-query block for each log
	// source (keyed by the entry's label set), owned exclusively by the run
	// goroutine. A source key is only ever present while a block is open.
	pendingSlowQueries map[string]*slowQueryBlock

	ctx     context.Context
	cancel  context.CancelFunc
	stopped *atomic.Bool
	wg      sync.WaitGroup
}

func NewLogs(args LogsArguments) (*Logs, error) {
	ctx, cancel := context.WithCancel(context.Background())

	l := &Logs{
		logger:             args.Logger.With("collector", LogsCollector),
		entryHandler:       args.EntryHandler,
		registry:           args.Registry,
		receiver:           args.Receiver,
		db:                 args.DB,
		excludeSchemas:     args.ExcludeSchemas,
		excludeUsers:       args.ExcludeUsers,
		pendingSlowQueries: make(map[string]*slowQueryBlock),
		ctx:                ctx,
		cancel:             cancel,
		stopped:            atomic.NewBool(false),
	}

	l.initMetrics()

	return l, nil
}

func (l *Logs) initMetrics() {
	l.errorsByCode = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "database_observability",
			Name:      "mysql_errors_total",
			Help:      "Number of error-log Error/Warning lines by severity, error code, and subsystem",
		},
		[]string{"severity", "err_code", "subsystem", "modeled"},
	)
	l.slowQueries = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "database_observability",
			Name:      "mysql_slow_queries_total",
			Help:      "Number of slow query log entries processed",
		},
	)
	l.parseFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "database_observability",
			Name:      "mysql_log_parse_failures_total",
			Help:      "Number of log lines that failed to parse, by log source",
		},
		[]string{"source"},
	)

	l.registry.MustRegister(l.errorsByCode, l.slowQueries, l.parseFailures)
}

func (l *Logs) Name() string {
	return LogsCollector
}

// Receiver returns the logs receiver that loki.source.* can forward to.
func (l *Logs) Receiver() loki.LogsReceiver {
	return l.receiver
}

func (l *Logs) Start(ctx context.Context) error {
	l.logger.Debug("collector started")
	l.wg.Go(l.run)
	return nil
}

func (l *Logs) Stop() {
	l.cancel()
	l.stopped.Store(true)
	l.wg.Wait()

	l.registry.Unregister(l.errorsByCode)
	l.registry.Unregister(l.slowQueries)
	l.registry.Unregister(l.parseFailures)
}

func (l *Logs) Stopped() bool {
	return l.stopped.Load()
}

func (l *Logs) run() {
	l.logger.Debug("collector running, waiting for log entries")

	for {
		select {
		case <-l.ctx.Done():
			l.logger.Debug("collector stopping")
			return
		case entry := <-l.receiver.Chan():
			if err := l.parseLine(entry); err != nil {
				l.logger.Warn(
					"failed to process log line",
					"error", err,
					"line_preview", truncateString(entry.Entry.Line, 100),
				)
			}
		}
	}
}

// parseLine detects which of the three MySQL log formats a line belongs to
// and dispatches to the matching sub-parser. Detection is per-line shape,
// except for slow-query continuation lines, which are routed by whether a
// block is currently pending for this entry's source.
func (l *Logs) parseLine(entry loki.Entry) error {
	line := strings.TrimRight(entry.Entry.Line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return nil
	}
	key := entry.Labels.String()

	if slowLogTimeRegex.MatchString(line) || strings.HasPrefix(line, "#") {
		return l.parseSlowLogLine(key, line)
	}
	if _, pending := l.pendingSlowQueries[key]; pending {
		return l.parseSlowLogLine(key, line)
	}
	if errorLogFormatRegex.MatchString(line) {
		return l.parseErrorLogLine(line)
	}
	if generalLogLineRegex.MatchString(line) {
		return l.parseGeneralLogLine(line)
	}

	l.parseFailures.WithLabelValues("unknown").Inc()
	return fmt.Errorf("log line does not match any known MySQL log format")
}

func (l *Logs) parseErrorLogLine(line string) error {
	m := errorLogFormatRegex.FindStringSubmatch(line)
	if m == nil {
		l.parseFailures.WithLabelValues("error").Inc()
		return fmt.Errorf("error log line does not match expected format")
	}
	threadID, severity, errCode, subsystem, message := m[1], m[2], "MY-"+m[3], m[4], m[5]

	pattern, modeled := mysqlErrorLogPatterns[errCode]

	// Only Error/Warning severities count toward the error-rate metric;
	// System/Note lifecycle lines (e.g. startup/shutdown) are still eligible
	// to be modeled and emitted below, just not counted as errors.
	if severity == "Error" || severity == "Warning" {
		l.errorsByCode.WithLabelValues(severity, errCode, subsystem, strconv.FormatBool(modeled)).Inc()
	}

	if !modeled {
		return nil
	}

	fields, matched := matchMySQLErrorLog(pattern, message)
	if !matched {
		return nil
	}

	if user, ok := fields["user"]; ok && slices.Contains(l.excludeUsers, user) {
		return nil
	}
	if db, ok := fields["db"]; ok && slices.Contains(l.excludeSchemas, db) {
		return nil
	}

	fields["category"] = pattern.category
	fields["err_code"] = errCode
	fields["subsystem"] = subsystem
	if threadID != "" && threadID != "0" {
		fields["thread_id"] = threadID
	}

	op := database_observability.OP_SERVER_LOG
	if severity == "Error" || severity == "Warning" {
		op = database_observability.OP_ERROR_MESSAGE
	}
	l.emit(op, fields)
	return nil
}

func (l *Logs) parseSlowLogLine(key, line string) error {
	if slowLogTimeRegex.MatchString(line) {
		if prev, ok := l.pendingSlowQueries[key]; ok && prev != nil {
			l.flushSlowQueryBlock(prev)
		}
		l.pendingSlowQueries[key] = &slowQueryBlock{}
		return nil
	}

	blk, pending := l.pendingSlowQueries[key]
	if !pending {
		l.parseFailures.WithLabelValues("slow").Inc()
		return fmt.Errorf("slow log line without a preceding Time line")
	}

	if m := slowLogUserHostRegex.FindStringSubmatch(line); m != nil {
		blk.user, blk.threadID = m[1], m[2]
		return nil
	}
	if m := slowLogStatsRegex.FindStringSubmatch(line); m != nil {
		blk.queryTime, blk.lockTime, blk.rowsSent, blk.rowsExamined = m[1], m[2], m[3], m[4]
		return nil
	}
	if strings.HasPrefix(line, "#") {
		// Other '#'-prefixed stat lines (e.g. log_slow_extra fields) are
		// ignored in v1.
		return nil
	}
	if slowLogSetTimestampRegex.MatchString(line) {
		return nil
	}

	// The statement text itself (never emitted — see flushSlowQueryBlock).
	if blk.sql.Len() > 0 {
		blk.sql.WriteByte('\n')
	}
	blk.sql.WriteString(line)

	if strings.HasSuffix(strings.TrimSpace(line), ";") {
		l.flushSlowQueryBlock(blk)
		delete(l.pendingSlowQueries, key)
	}
	return nil
}

// flushSlowQueryBlock computes blk's digest and emits op="slow_query", or
// drops it (counted as a parse failure) if the block never accumulated any
// SQL text or the digest lookup fails. blk.sql is read exactly once here,
// to build the single digest-lookup query, and is never stored or emitted.
func (l *Logs) flushSlowQueryBlock(blk *slowQueryBlock) {
	// Strip the trailing statement terminator the slow log always records:
	// the client-submitted statement that generated this entry (and whose
	// digest performance_schema would compute) never includes it.
	sqlText := strings.TrimSpace(blk.sql.String())
	sqlText = strings.TrimSpace(strings.TrimSuffix(sqlText, ";"))
	if sqlText == "" {
		l.parseFailures.WithLabelValues("slow").Inc()
		return
	}
	if blk.user != "" && slices.Contains(l.excludeUsers, blk.user) {
		return
	}

	digest, err := l.computeDigest(sqlText)
	l.slowQueries.Inc()
	if err != nil {
		l.logger.Warn("failed to compute statement digest for slow query", "err", err)
		l.parseFailures.WithLabelValues("slow").Inc()
		return
	}

	fields := map[string]string{"digest": digest}
	if blk.threadID != "" {
		fields["thread_id"] = blk.threadID
	}
	if blk.user != "" {
		fields["user"] = blk.user
	}
	if blk.queryTime != "" {
		fields["query_time"] = blk.queryTime
	}
	if blk.lockTime != "" {
		fields["lock_time"] = blk.lockTime
	}
	if blk.rowsSent != "" {
		fields["rows_sent"] = blk.rowsSent
	}
	if blk.rowsExamined != "" {
		fields["rows_examined"] = blk.rowsExamined
	}

	l.emit(database_observability.OP_SLOW_QUERY, fields)
}

// computeDigest asks the live server to compute sqlText's statement digest
// via the built-in STATEMENT_DIGEST() function — the same digest MySQL
// would compute for this text in performance_schema.events_statements_*.DIGEST.
// sqlText is passed as a bound parameter and is never logged.
func (l *Logs) computeDigest(sqlText string) (string, error) {
	if l.db == nil {
		return "", fmt.Errorf("no database connection configured for digest lookup")
	}

	ctx, cancel := context.WithTimeout(l.ctx, digestLookupTimeout)
	defer cancel()

	var digest sql.NullString
	if err := l.db.QueryRowContext(ctx, selectStatementDigest, sqlText).Scan(&digest); err != nil {
		return "", err
	}
	if !digest.Valid {
		return "", fmt.Errorf("STATEMENT_DIGEST returned NULL")
	}
	return digest.String, nil
}

func (l *Logs) parseGeneralLogLine(line string) error {
	m := generalLogLineRegex.FindStringSubmatch(line)
	if m == nil {
		l.parseFailures.WithLabelValues("general").Inc()
		return fmt.Errorf("general log line does not match expected format")
	}
	threadID, command := m[2], strings.TrimSpace(m[3])
	argument := m[4]

	cmd, ok := generalLogCommands[command]
	if !ok {
		// Query/Prepare/Execute/etc — intentionally out of scope, see plan.
		return nil
	}

	fields := matchGeneralLogCommand(cmd, argument)

	if user, ok := fields["user"]; ok && slices.Contains(l.excludeUsers, user) {
		return nil
	}
	if db, ok := fields["db"]; ok && slices.Contains(l.excludeSchemas, db) {
		return nil
	}

	fields["category"] = cmd.category
	if threadID != "" && threadID != "0" {
		fields["thread_id"] = threadID
	}

	l.emit(database_observability.OP_SERVER_LOG, fields)
	return nil
}

// emit builds a logfmt body from fields (sorted for deterministic output)
// and sends it to the entry handler, guarded by the collector context so a
// stalled downstream can't wedge Stop().
func (l *Logs) emit(op string, fields map[string]string) {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(' ')
		}
		if _, numeric := numericLogFields[k]; numeric {
			fmt.Fprintf(&b, "%s=%s", k, fields[k])
		} else {
			fmt.Fprintf(&b, "%s=%q", k, fields[k])
		}
	}

	select {
	case l.entryHandler.Chan() <- database_observability.BuildLokiEntry(logging.LevelInfo, op, b.String()):
	case <-l.ctx.Done():
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
