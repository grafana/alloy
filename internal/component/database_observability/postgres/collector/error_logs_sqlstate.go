package collector

// SQLStateErrors is a comprehensive map of all PostgreSQL SQLSTATE codes to their error names.
// Reference: https://www.postgresql.org/docs/current/errcodes-appendix.html
var SQLStateErrors = map[string]string{
	// Class 08 — Connection Exception
	"08000": "connection_exception",
	"08003": "connection_does_not_exist",
	"08006": "connection_failure",
	"08001": "sqlclient_unable_to_establish_sqlconnection",
	"08004": "sqlserver_rejected_establishment_of_sqlconnection",
	"08007": "transaction_resolution_unknown",
	"08P01": "protocol_violation",

	// Class 23 — Integrity Constraint Violation
	"23000": "integrity_constraint_violation",
	"23001": "restrict_violation",
	"23502": "not_null_violation",
	"23503": "foreign_key_violation",
	"23505": "unique_violation",
	"23514": "check_violation",
	"23P01": "exclusion_violation",

	// Class 25 — Invalid Transaction State
	"25000": "invalid_transaction_state",
	"25001": "active_sql_transaction",
	"25002": "branch_transaction_already_active",
	"25003": "inappropriate_access_mode_for_branch_transaction",
	"25004": "inappropriate_isolation_level_for_branch_transaction",
	"25005": "no_active_sql_transaction_for_branch_transaction",
	"25006": "read_only_sql_transaction",
	"25007": "schema_and_data_statement_mixing_not_supported",
	"25008": "held_cursor_requires_same_isolation_level",
	"25P01": "no_active_sql_transaction",
	"25P02": "in_failed_sql_transaction",
	"25P03": "idle_in_transaction_session_timeout",

	// Class 28 — Invalid Authorization Specification
	"28000": "invalid_authorization_specification",
	"28P01": "invalid_password",

	// Class 40 — Transaction Rollback
	"40000": "transaction_rollback",
	"40001": "serialization_failure",
	"40002": "transaction_integrity_constraint_violation",
	"40003": "statement_completion_unknown",
	"40P01": "deadlock_detected",

	// Class 53 — Insufficient Resources
	"53000": "insufficient_resources",
	"53100": "disk_full",
	"53200": "out_of_memory",
	"53300": "too_many_connections",
	"53400": "configuration_limit_exceeded",

	// Class 54 — Program Limit Exceeded
	"54000": "program_limit_exceeded",
	"54001": "statement_too_complex",
	"54011": "too_many_columns",
	"54023": "too_many_arguments",

	// Class 55 — Object Not In Prerequisite State
	"55000": "object_not_in_prerequisite_state",
	"55006": "object_in_use",
	"55P02": "cant_change_runtime_param",
	"55P03": "lock_not_available",
	"55P04": "unsafe_new_enum_value_usage",

	// Class 57 — Operator Intervention
	"57000": "operator_intervention",
	"57014": "query_canceled",
	"57P01": "admin_shutdown",
	"57P02": "crash_shutdown",
	"57P03": "cannot_connect_now",
	"57P04": "database_dropped",
	"57P05": "idle_session_timeout",

	// Class 58 — System Error
	"58000": "system_error",
	"58030": "io_error",
	"58P01": "undefined_file",
	"58P02": "duplicate_file",

	// Class P0 — PL/pgSQL Error
	"P0000": "plpgsql_error",
	"P0001": "raise_exception",
	"P0002": "no_data_found",
	"P0003": "too_many_rows",
	"P0004": "assert_failure",

	// Class XX — Internal Error
	"XX000": "internal_error",
	"XX001": "data_corrupted",
	"XX002": "index_corrupted",
}

// SQLStateClass maps SQLSTATE class codes to human-readable categories.
// See: https://www.postgresql.org/docs/current/errcodes-appendix.html
var SQLStateClass = map[string]string{
	"00": "Successful Completion",
	"01": "Warning",
	"02": "No Data",
	"03": "SQL Statement Not Yet Complete",
	"08": "Connection Exception",
	"09": "Triggered Action Exception",
	"0A": "Feature Not Supported",
	"0B": "Invalid Transaction Initiation",
	"0F": "Locator Exception",
	"0L": "Invalid Grantor",
	"0P": "Invalid Role Specification",
	"0Z": "Diagnostics Exception",
	"20": "Case Not Found",
	"21": "Cardinality Violation",
	"22": "Data Exception",
	"23": "Integrity Constraint Violation",
	"24": "Invalid Cursor State",
	"25": "Invalid Transaction State",
	"26": "Invalid SQL Statement Name",
	"27": "Triggered Data Change Violation",
	"28": "Invalid Authorization Specification",
	"2B": "Dependent Privilege Descriptors Still Exist",
	"2D": "Invalid Transaction Termination",
	"2F": "SQL Routine Exception",
	"34": "Invalid Cursor Name",
	"38": "External Routine Exception",
	"39": "External Routine Invocation Exception",
	"3B": "Savepoint Exception",
	"3D": "Invalid Catalog Name",
	"3F": "Invalid Schema Name",
	"40": "Transaction Rollback",
	"42": "Syntax Error or Access Rule Violation",
	"44": "WITH CHECK OPTION Violation",
	"53": "Insufficient Resources",
	"54": "Program Limit Exceeded",
	"55": "Object Not In Prerequisite State",
	"57": "Operator Intervention",
	"58": "System Error",
	"72": "Snapshot Failure",
	"F0": "Configuration File Error",
	"HV": "Foreign Data Wrapper Error",
	"P0": "PL/pgSQL Error",
	"XX": "Internal Error",
}

// GetSQLStateErrorName returns the specific error name for a given SQLSTATE code.
// Returns empty string if the code is not found.
func GetSQLStateErrorName(sqlstate string) string {
	if sqlstate == "" {
		return ""
	}
	if name, ok := SQLStateErrors[sqlstate]; ok {
		return name
	}
	return ""
}

// GetSQLStateCategory returns the category for a given SQLSTATE code.
// If the exact code is not found, it returns the category for the class (first 2 characters).
func GetSQLStateCategory(sqlstate string) string {
	if sqlstate == "" {
		return ""
	}

	class := sqlstate[:2]
	if category, ok := SQLStateClass[class]; ok {
		return category
	}

	return "Unknown"
}

// IsConnectionError returns true if the SQLSTATE code represents a connection error.
func IsConnectionError(sqlstate string) bool {
	if len(sqlstate) >= 2 {
		return sqlstate[:2] == "08"
	}
	return false
}

// IsAuthenticationError returns true if the SQLSTATE code represents an authentication error.
// Class 28 = Invalid Authorization Specification (password failures, invalid credentials, etc.)
func IsAuthenticationError(sqlstate string) bool {
	if len(sqlstate) >= 2 {
		return sqlstate[:2] == "28"
	}
	return false
}

// IsResourceLimitError returns true if the SQLSTATE code represents a resource limit error.
// Class 53 = Insufficient Resources (out of memory, disk full, too many connections, etc.)
func IsResourceLimitError(sqlstate string) bool {
	if len(sqlstate) >= 2 {
		return sqlstate[:2] == "53"
	}
	return false
}

// setTimeoutType sets the timeout type based on SQLSTATE code.
// Reference: https://www.postgresql.org/docs/current/errcodes-appendix.html
func (c *ErrorLogs) setTimeoutType(parsed *ParsedError) {
	if parsed.SQLStateCode == "" {
		return
	}

	// Map specific SQLSTATE codes to timeout types
	switch parsed.SQLStateCode {
	case "40P01":
		// Deadlock detected
		parsed.TimeoutType = "deadlock"
	case "55P03":
		// Lock not available (lock_timeout)
		parsed.TimeoutType = "lock_timeout"
	case "57014":
		// Query canceled (can be statement_timeout, user cancel, etc.)
		// Without parsing message text, we can't distinguish the specific reason
		parsed.TimeoutType = "query_canceled"
	case "25P03":
		// Idle in transaction session timeout
		parsed.TimeoutType = "idle_in_transaction_timeout"
	case "57P05":
		// Idle session timeout (PostgreSQL 14+)
		parsed.TimeoutType = "idle_session_timeout"
	}
}
