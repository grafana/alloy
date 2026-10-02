package collector

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// slowQueryGate and slowQueryRegex detect and extract a log_min_duration_statement
// LOG message. "ms  statement:" (two spaces) is this message's own shape,
// distinct from auto_explain's "ms  plan:" (see autoExplainGate) -- the two
// settings log the same completed statement differently and never produce
// both lines for the same execution unless both are enabled, in which case
// each is handled independently by its own gate/regex pair.
const slowQueryGate = "ms  statement:"

// (?s) lets '.' cross an embedded newline in the statement text itself
// (e.g. a multi-line query from an ORM); PostgreSQL preserves such newlines
// verbatim, and any resulting continuation line arrives here already
// unwrapped by the general tab-prefix continuation handling in
// parseTextLog, same as every other multi-line LOG message this collector
// handles.
var slowQueryRegex = regexp.MustCompile(`(?s)^duration: (?P<duration_ms>[\d.]+) ms  statement: (?P<statement>.*)$`)

// tryEmitSlowQuery checks whether message is a log_min_duration_statement
// LOG line and, if so, emits it as op="slow_query": duration_ms plus the
// same identifying/correlation fields op="error_message" and
// op="server_log" already carry (pid, xid, vxid, session_id, ...) -- never
// the raw statement text itself, consistent with every other log-sourced
// op in this collector. query_fingerprint is the join key back to this
// query's other appearances in this collector's own output (deadlock's
// query_fingerprint_blocker, op="error_message", op="explain_plan_output"
// sourced_from=logs, ...), and transitively to the queryid-keyed ops
// (op="query_sample", the live EXPLAIN-based op="explain_plan_output") via
// op="query_association", which already emits both queryid and
// query_fingerprint side by side for the same pg_stat_statements row (see
// query_details.go's fetchAndAssociate) -- no log_line_prefix change needed.
// pid+xid (the executing backend's own transaction, not a lock_wait
// target's) is an exact match, not a time-window heuristic, back to a
// concurrent op="server_log" category="lock_wait" entry on the same
// backend: a backend's xid identifies one transaction, so the same pid+xid
// pair appearing on both entries means the same execution.
//
// Returns false for anything that isn't this exact shape (an empty
// statement after trimming, for instance): the caller falls through to the
// server-log pattern registry, which doesn't model this shape either, so
// it's dropped the same way an unrecognized LOG line always is.
func (l *Logs) tryEmitSlowQuery(message string, ts time.Time, meta serverLogMeta) bool {
	if !strings.Contains(message, slowQueryGate) {
		return false
	}
	m := slowQueryRegex.FindStringSubmatch(message)
	if m == nil {
		return false
	}

	statement := strings.TrimSpace(m[2])
	queryFingerprint, err := fingerprint.Fingerprint(statement)
	if err != nil {
		return false
	}

	if ts.IsZero() {
		ts = time.Now()
	}

	// query_fingerprint, pid, etc. never need quoting; %q escapes datname,
	// user, and the client-controlled application_name.
	body := fmt.Sprintf("query_fingerprint=%s duration_ms=%s", queryFingerprint, m[1])
	if meta.datname != "" {
		body += fmt.Sprintf(" datname=%q", meta.datname)
	}
	if meta.user != "" {
		body += fmt.Sprintf(" user=%q", meta.user)
	}
	if meta.pid != "" {
		body += fmt.Sprintf(" pid=%s", meta.pid)
	}
	if meta.lineNumber != "" {
		body += fmt.Sprintf(" line_number=%s", meta.lineNumber)
	}
	if meta.sessionStartTime != "" {
		body += fmt.Sprintf(" session_start_time=%q", meta.sessionStartTime)
	}
	if meta.vxid != "" {
		body += fmt.Sprintf(" vxid=%s", meta.vxid)
	}
	if meta.xid != "" {
		body += fmt.Sprintf(" xid=%s", meta.xid)
	}
	if meta.sessionID != "" {
		body += fmt.Sprintf(" session_id=%s", meta.sessionID)
	}
	if meta.applicationName != "" {
		body += fmt.Sprintf(" application_name=%q", l.redact(meta.applicationName))
	}

	// traceparent is general-purpose, not slow-query-specific: see
	// emitErrorEntry's identical extraction for op="error_message". It's
	// comment-embedded W3C trace context, not query content, so it's safe
	// to emit even though the statement text itself never is.
	if tm := traceparentCommentRegex.FindStringSubmatch(statement); tm != nil {
		body += fmt.Sprintf(" traceparent=%q", tm[1])
	}

	select {
	case l.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		database_observability.OP_SLOW_QUERY,
		body,
		ts.UnixNano(),
	):
	case <-l.ctx.Done():
	}
	return true
}
