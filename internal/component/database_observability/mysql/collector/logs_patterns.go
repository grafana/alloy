package collector

import "regexp"

const (
	categoryAbortedConnection   = "aborted_connection"
	categoryAccessDenied        = "access_denied"
	categoryReplicationIOError  = "replication_io_error"
	categoryReplicationSQLError = "replication_sql_error"
	categoryServerStartup       = "server_startup"
	categoryServerShutdown      = "server_shutdown"
)

// mysqlErrorLogPattern maps one MY-xxxxxx error-log code to a semantic
// category and a regex that pulls structured fields out of the message
// body. Named capture groups become Loki entry fields directly (via
// regexp.SubexpNames()); an empty capture is dropped. Client host is never
// captured by any of these patterns, by design.
type mysqlErrorLogPattern struct {
	category string
	regex    *regexp.Regexp
}

// mysqlErrorLogPatterns is keyed by the MY-xxxxxx code exactly as it appears
// in the error log line (e.g. "MY-010914"). Codes and message templates are
// confirmed from share/messages_to_error_log.txt in github.com/mysql/mysql-server
// (branch 8.0) — see the implementation plan for the per-code source citations.
var mysqlErrorLogPatterns = map[string]mysqlErrorLogPattern{
	// aborted_connection: ER_ABORTING_USER_CONNECTION, ER_CONNECTION_ABORTED,
	// ER_SERVER_NEW_ABORTING_CONNECTION — same message shape, different call
	// sites. The diagnostics-area suffix ER_SERVER_NEW_ABORTING_CONNECTION
	// adds still falls inside the one parenthesized reason group.
	"MY-010914": {category: categoryAbortedConnection, regex: abortedConnectionRegex},
	"MY-013104": {category: categoryAbortedConnection, regex: abortedConnectionRegex},
	"MY-013130": {category: categoryAbortedConnection, regex: abortedConnectionRegex},

	// access_denied: with/without password, and account-locked variants all
	// share the same message shape.
	"MY-010925": {category: categoryAccessDenied, regex: accessDeniedRegex},
	"MY-010926": {category: categoryAccessDenied, regex: accessDeniedRegex},
	"MY-010927": {category: categoryAccessDenied, regex: accessDeniedRegex},

	// replication_io_error: ER_SERVER_SOURCE_FATAL_ERROR_READING_BINLOG.
	"MY-013114": {category: categoryReplicationIOError, regex: replicationIOErrorRegex},

	// replication_sql_error: ER_RPL_REPLICA_ERROR_RUNNING_QUERY (root cause)
	// and its diagnostics-area wrapper ER_RPL_REPLICA_ERROR_INFO_FROM_DA.
	"MY-010586": {category: categoryReplicationSQLError, regex: replicationSQLErrorRegex},
	"MY-010584": {category: categoryReplicationSQLError, regex: replicationSQLWrapperRegex},

	// server_startup: ER_STARTING_AS (process start) and
	// ER_SERVER_STARTUP_MSG (ready for connections).
	"MY-010116": {category: categoryServerStartup, regex: serverStartingRegex},
	"MY-010931": {category: categoryServerStartup, regex: serverReadyRegex},

	// server_shutdown: ER_NORMAL_SERVER_SHUTDOWN.
	"MY-013105": {category: categoryServerShutdown, regex: serverShutdownRegex},
}

var (
	// "Aborted connection %u to db: '%-.192s' user: '%-.48s' host: '%-.255s' (%-.64s)."
	abortedConnectionRegex = regexp.MustCompile(
		`^Aborted connection \d+ to db: '(?P<db>[^']*)' user: '(?P<user>[^']*)' host: '[^']*' \((?P<reason>.*)\)\.?$`,
	)

	// "Access denied for user '%-.48s'@'%-.64s' (using password: %s)"
	accessDeniedRegex = regexp.MustCompile(
		`^Access denied for user '(?P<user>[^']*)'@'[^']*'\s*\(using password: (?P<using_password>\w+)\)`,
	)

	// "Got fatal error %d from source when reading data from binary log: '%-.512s'"
	replicationIOErrorRegex = regexp.MustCompile(
		`^Got fatal error (?P<replication_errno>\d+) from source when reading data from binary log: '(?P<detail>.*)'$`,
	)

	// "...We stopped at log '%s' position %s"
	replicationSQLErrorRegex = regexp.MustCompile(
		`We stopped at log '(?P<binlog_file>[^']*)' position (?P<binlog_position>\d+)`,
	)

	// "Replica: %s Error_code: MY-%06d"
	replicationSQLWrapperRegex = regexp.MustCompile(
		`^Replica: (?P<replica_detail>.*) Error_code: MY-(?P<wrapped_err_code>\d{6})$`,
	)

	// "%s (mysqld %s) starting as process %lu"
	serverStartingRegex = regexp.MustCompile(`starting as process (?P<pid>\d+)`)

	// "%s: ready for connections. Version: '%s'  socket: '%s'  port: %d  %s."
	serverReadyRegex = regexp.MustCompile(`Version: '(?P<version>[^']*)'.*?port: (?P<port>\d+)`)

	// "%s: Normal shutdown." — nothing to capture; the code alone identifies it.
	serverShutdownRegex = regexp.MustCompile(`Normal shutdown`)
)

// matchMySQLErrorLog applies pattern's regex to message and returns the
// captured named fields, or ok=false if the message didn't actually match
// the code's expected shape (e.g. an unexpected message for a MY-code whose
// template changed between server versions).
func matchMySQLErrorLog(pattern mysqlErrorLogPattern, message string) (fields map[string]string, ok bool) {
	m := pattern.regex.FindStringSubmatch(message)
	if m == nil {
		return nil, false
	}
	fields = make(map[string]string)
	for i, groupName := range pattern.regex.SubexpNames() {
		if i == 0 || groupName == "" || m[i] == "" {
			continue
		}
		fields[groupName] = m[i]
	}
	return fields, true
}

// generalLogCommand maps one general-log `Command` value we model to a
// category and an optional regex that extracts fields from `Argument`. Only
// non-query-tied, connection-lifecycle commands are modeled — see the
// implementation plan for why Query/Prepare/Execute are intentionally out
// of scope.
type generalLogCommand struct {
	category string
	regex    *regexp.Regexp // nil means no fields to extract beyond category/thread_id
}

var generalLogCommands = map[string]generalLogCommand{
	"Connect":     {category: "connection", regex: generalLogConnectRegex},
	"Quit":        {category: "disconnection"},
	"Change user": {category: "change_user", regex: generalLogChangeUserRegex},
	"Init DB":     {category: "init_db", regex: generalLogInitDBRegex},
}

var (
	// Argument shape: "user@host on db_name using ...". Host is intentionally
	// never captured.
	generalLogConnectRegex = regexp.MustCompile(`^(?P<user>[^@\s]+)@\S*\s+on\s+(?P<db>\S*)`)

	// Argument shape: "new_user@host". Host is intentionally never captured.
	generalLogChangeUserRegex = regexp.MustCompile(`^(?P<user>[^@\s]+)`)

	// Argument shape: the schema name alone.
	generalLogInitDBRegex = regexp.MustCompile(`^(?P<db>\S+)`)
)

// matchGeneralLogCommand applies cmd's regex (if any) to argument and
// returns the captured named fields. A command with no regex (e.g. Quit)
// always returns an empty, non-nil field set.
func matchGeneralLogCommand(cmd generalLogCommand, argument string) map[string]string {
	fields := make(map[string]string)
	if cmd.regex == nil {
		return fields
	}
	m := cmd.regex.FindStringSubmatch(argument)
	if m == nil {
		return fields
	}
	for i, groupName := range cmd.regex.SubexpNames() {
		if i == 0 || groupName == "" || m[i] == "" {
			continue
		}
		fields[groupName] = m[i]
	}
	return fields
}
