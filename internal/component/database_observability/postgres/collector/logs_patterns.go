package collector

import (
	"regexp"
	"strings"
)

// serverLogPattern matches one non-error (LOG/WARNING/NOTICE) server-log
// message shape: checkpoints, autovacuum, connections/disconnections, lock
// waits, temp files, and replication commands. Unlike the free-text
// message/DETAIL/HINT/CONTEXT captured for ERROR/FATAL/PANIC lines, every
// field these 7 categories capture is structural or numeric (PIDs, timings,
// byte counts, internal paths, schema identifiers) — none need PII
// redaction.
type serverLogPattern struct {
	name  string         // emitted as category=<name>
	gates []string       // cheap substring pre-check: at least one must be present
	regex *regexp.Regexp // matched against the message text; named groups become fields
	// postProcess, when set, runs after the regex's named captures are
	// collected, to derive further fields from them -- e.g. lock_wait's
	// opaque "target" capture gets split into xid or
	// relation_oid/database_oid when it matches one of those common shapes.
	postProcess func(fields map[string]string)
}

// categoryConfigReload is shared by 4 serverLogPatterns entries below (one
// per config-reload sub-shape, distinguished by their "kind" field), so
// it's named once instead of repeating the literal.
const categoryConfigReload = "config_reload"

var serverLogPatterns = []serverLogPattern{
	{
		name:  "checkpoint",
		gates: []string{"checkpoint complete:"},
		regex: regexp.MustCompile(`^checkpoint complete: wrote (?P<buffers_written>\d+) buffers \([\d.]+%\); (?P<wal_added>\d+) WAL file\(s\) added, (?P<wal_removed>\d+) removed, (?P<wal_recycled>\d+) recycled; write=(?P<write_seconds>[\d.]+) s, sync=(?P<sync_seconds>[\d.]+) s, total=(?P<total_seconds>[\d.]+) s`),
	},
	{
		name:  "autovacuum",
		gates: []string{`automatic vacuum of table "`, `automatic analyze of table "`},
		// (?s) lets '.' cross the embedded newlines PostgreSQL's ereport() puts
		// in this message: the whole thing arrives as one loki.Entry.Line.
		regex: regexp.MustCompile(`(?s)^automatic (?P<operation>vacuum|analyze) of table "(?P<table>[^"]+)".*?elapsed: (?P<elapsed_seconds>[\d.]+) s`),
	},
	{
		name:  "connection",
		gates: []string{"connection authorized:"},
		// "connection received:" (host/port only, no user/database) is
		// deliberately not matched here: client_addr stays deferred, same as
		// the error-line %r field, and that line carries nothing else useful.
		regex: regexp.MustCompile(`^connection authorized: user=(?P<user>\S+) database=(?P<database>\S+)(?: application_name=(?P<application_name>\S+))?`),
	},
	{
		name:  "connection_authenticated",
		gates: []string{"connection authenticated:"},
		// The authentication step between "connection received:" (deferred)
		// and "connection authorized:" (above): which role authenticated,
		// via which method, and which pg_hba.conf line matched. No
		// client_addr here either -- same policy. The authenticated identity
		// is named "user", not "identity": it's the same role the general
		// log_line_prefix meta's own user field already carries (%u is set
		// from the startup packet before this line is even logged), so the
		// two end up deduplicated in emitServerLogEntry the same way
		// connection/disconnection's user already is.
		regex: regexp.MustCompile(`^connection authenticated: identity="(?P<user>[^"]+)" method=(?P<method>\S+) \([^:]+:(?P<pg_hba_line>\d+)\)`),
	},
	{
		name:  "disconnection",
		gates: []string{"disconnection:"},
		// host=/port= (client_addr) intentionally not captured, same policy.
		regex: regexp.MustCompile(`^disconnection: session time: (?P<session_time>[\d:.]+)\s+user=(?P<user>\S+) database=(?P<database>\S+)`),
	},
	{
		name: "lock_wait",
		// "failed to acquire" is PostgreSQL's third log_lock_waits phase,
		// logged when lock_timeout expires instead of the lock eventually
		// being granted.
		gates:       []string{"still waiting for", "acquired", "failed to acquire"},
		regex:       regexp.MustCompile(`^process (?P<pid>\d+) (?P<phase>still waiting for|acquired|failed to acquire) (?P<lock_type>\S+) on (?P<target>.+) after (?P<wait_ms>[\d.]+) ms`),
		postProcess: splitLockWaitTarget,
	},
	{
		name:  "temp_file",
		gates: []string{"temporary file:"},
		regex: regexp.MustCompile(`^temporary file: path "(?P<path>[^"]+)", size (?P<size_bytes>\d+)`),
	},
	{
		name:  "client_io_error",
		gates: []string{"could not receive data from client:"},
		// Abrupt client disconnect / network issue mid-session (e.g. "Connection
		// reset by peer"). reason is the OS-level error text, never anything
		// client-supplied.
		regex: regexp.MustCompile(`^could not receive data from client: (?P<reason>.+)$`),
	},
	{
		name:  "tls_handshake_failure",
		gates: []string{"could not accept SSL connection:"},
		regex: regexp.MustCompile(`^could not accept SSL connection: (?P<reason>.+)$`),
	},
	{
		name:        categoryConfigReload,
		gates:       []string{"received SIGHUP, reloading configuration files"},
		regex:       regexp.MustCompile(`^received SIGHUP, reloading configuration files$`),
		postProcess: withKind("sighup"),
	},
	{
		name:        categoryConfigReload,
		gates:       []string{`parameter "`},
		regex:       regexp.MustCompile(`^parameter "(?P<param_name>[^"]+)" changed to "(?P<param_value>[^"]*)"`),
		postProcess: withKind("parameter_changed"),
	},
	{
		name:        categoryConfigReload,
		gates:       []string{"contains errors; unaffected changes were applied"},
		regex:       regexp.MustCompile(`^configuration file "(?P<config_file>[^"]+)" contains errors; unaffected changes were applied`),
		postProcess: withKind("file_error"),
	},
	{
		name:        categoryConfigReload,
		gates:       []string{"invalid configuration parameter name "},
		regex:       regexp.MustCompile(`^invalid configuration parameter name "(?P<param_name>[^"]+)"`),
		postProcess: withKind("invalid_parameter"),
	},
	{
		name:  "replication_command",
		gates: []string{"received replication command:"},
		regex: regexp.MustCompile(`^received replication command: (?P<command>\S+)(?: (?P<args>.+))?$`),
	},
}

// lockWaitTransactionTargetRegex, lockWaitRelationTargetRegex and
// lockWaitTupleTargetRegex match lock_wait's most common target shapes (see
// PostgreSQL's DescribeLockTag in lock.c): a plain transaction ID, a
// relation OID within a database OID, or a specific tuple (page, offset)
// within a relation/database -- the shape row-level contention (e.g. two
// sessions' SELECT ... FOR UPDATE on the same row) logs. Other shapes
// (advisory locks, virtual transactions, ...) stay as the opaque target
// string.
var (
	lockWaitTransactionTargetRegex = regexp.MustCompile(`^transaction (\d+)$`)
	lockWaitRelationTargetRegex    = regexp.MustCompile(`^relation (\d+) of database (\d+)$`)
	lockWaitTupleTargetRegex       = regexp.MustCompile(`^tuple \((\d+,\d+)\) of relation (\d+) of database (\d+)$`)
)

// splitLockWaitTarget replaces lock_wait's opaque target capture with xid,
// relation_oid plus database_oid, or tuple plus relation_oid and
// database_oid, when it matches one of those common shapes -- PostgreSQL's
// plain log_lock_waits message has no CONTEXT to resolve a relation_oid to
// a table name (unlike a deadlock's DETAIL+CONTEXT), so that resolution is
// left to the consumer for now. It also normalizes the "still waiting for"
// phase down to "waiting" -- PostgreSQL's own wording, kept verbatim for
// "acquired"/"failed to acquire".
func splitLockWaitTarget(fields map[string]string) {
	if fields["phase"] == "still waiting for" {
		fields["phase"] = "waiting"
	}

	target := fields["target"]
	if m := lockWaitTransactionTargetRegex.FindStringSubmatch(target); m != nil {
		delete(fields, "target")
		// Named xid (the specific transaction being waited on), not
		// transaction_id: emitServerLogEntry prefers this over the general
		// log_line_prefix meta's own xid, which is this backend's own
		// transaction (often 0 for a read-only waiter) rather than the one
		// it's actually blocked on.
		fields["xid"] = m[1]
		return
	}
	if m := lockWaitTupleTargetRegex.FindStringSubmatch(target); m != nil {
		delete(fields, "target")
		fields["tuple"] = m[1]
		fields["relation_oid"] = m[2]
		fields["database_oid"] = m[3]
		return
	}
	if m := lockWaitRelationTargetRegex.FindStringSubmatch(target); m != nil {
		delete(fields, "target")
		fields["relation_oid"] = m[1]
		fields["database_oid"] = m[2]
	}
}

// withKind returns a postProcess hook that tags fields with a fixed "kind"
// value, for patterns that share one category name but represent distinct
// sub-shapes (e.g. config_reload's sighup/parameter_changed/file_error/
// invalid_parameter).
func withKind(kind string) func(map[string]string) {
	return func(fields map[string]string) {
		fields["kind"] = kind
	}
}

// serverLogNumericFields are the captured field names that are always plain
// numbers (or bare words with no spaces, for "operation"/"phase"): emitted
// unquoted in the op="server_log" body, the same convention emitErrorEntry
// uses for pid/sqlstate/vxid. Every other captured field is emitted quoted,
// since several (e.g. lock_wait's "target") can contain spaces.
var serverLogNumericFields = map[string]struct{}{
	"buffers_written": {},
	"wal_added":       {},
	"wal_removed":     {},
	"wal_recycled":    {},
	"write_seconds":   {},
	"sync_seconds":    {},
	"total_seconds":   {},
	"elapsed_seconds": {},
	"pid":             {},
	"wait_ms":         {},
	"size_bytes":      {},
	"xid":             {},
	"relation_oid":    {},
	"database_oid":    {},
	"pg_hba_line":     {},
}

// anyServerLogGateMatches reports whether line contains at least one of the
// serverLogPatterns' cheap substrings. Used by parseTextLog's early keyword
// gate so the expensive format regex and label classification only run for
// lines that could plausibly be one of the 7 categories, mirroring the
// existing ERROR/FATAL/PANIC keyword gate.
func anyServerLogGateMatches(line string) bool {
	for _, p := range serverLogPatterns {
		for _, gate := range p.gates {
			if strings.Contains(line, gate) {
				return true
			}
		}
	}
	return false
}

// matchServerLog checks msg (the text following the LOG/WARNING/NOTICE
// label) against the registry, returning the first matching category's name
// and its named-capture fields. ok is false when no category recognizes the
// message — callers drop the line silently in that case; Alloy only ever
// emits the categories it has explicitly modeled here, never an unrecognized
// LOG line captured wholesale.
func matchServerLog(msg string) (category string, fields map[string]string, ok bool) {
	for _, p := range serverLogPatterns {
		gated := false
		for _, gate := range p.gates {
			if strings.Contains(msg, gate) {
				gated = true
				break
			}
		}
		if !gated {
			continue
		}

		m := p.regex.FindStringSubmatch(msg)
		if m == nil {
			continue
		}

		fields = map[string]string{}
		for i, name := range p.regex.SubexpNames() {
			if i == 0 || name == "" || m[i] == "" {
				continue
			}
			fields[name] = m[i]
		}
		if p.postProcess != nil {
			p.postProcess(fields)
		}
		return p.name, fields, true
	}
	return "", nil, false
}
