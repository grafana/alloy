package collector

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

// ConnectionErrorCodes are specific SQLSTATE codes for connection-related errors (class 08).
var ConnectionErrorCodes = map[string]string{
	"08000": "connection_exception",
	"08003": "connection_does_not_exist",
	"08006": "connection_failure",
	"08001": "sqlclient_unable_to_establish_sqlconnection",
	"08004": "sqlserver_rejected_establishment_of_sqlconnection",
	"08007": "transaction_resolution_unknown",
	"08P01": "protocol_violation",
}

// ConstraintViolationCodes are specific SQLSTATE codes for constraint violations (class 23).
var ConstraintViolationCodes = map[string]string{
	"23000": "integrity_constraint_violation",
	"23001": "restrict_violation",
	"23502": "not_null_violation",
	"23503": "foreign_key_violation",
	"23505": "unique_violation",
	"23514": "check_violation",
	"23P01": "exclusion_violation",
}

// TransactionRollbackCodes are specific SQLSTATE codes for transaction rollbacks (class 40).
var TransactionRollbackCodes = map[string]string{
	"40000": "transaction_rollback",
	"40001": "serialization_failure",
	"40002": "transaction_integrity_constraint_violation",
	"40003": "statement_completion_unknown",
	"40P01": "deadlock_detected",
}

// ResourceErrorCodes are specific SQLSTATE codes for resource exhaustion (classes 53, 54).
var ResourceErrorCodes = map[string]string{
	"53000": "insufficient_resources",
	"53100": "disk_full",
	"53200": "out_of_memory",
	"53300": "too_many_connections",
	"53400": "configuration_limit_exceeded",
	"54000": "program_limit_exceeded",
	"54001": "statement_too_complex",
	"54011": "too_many_columns",
	"54023": "too_many_arguments",
}

// GetSQLStateCategory returns the category for a given SQLSTATE code.
// If the exact code is not found, it returns the category for the class (first 2 characters).
func GetSQLStateCategory(sqlstate string) string {
	if sqlstate == "" {
		return ""
	}

	// Try to get the class (first 2 characters)
	if len(sqlstate) >= 2 {
		class := sqlstate[:2]
		if category, ok := SQLStateClass[class]; ok {
			return category
		}
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

// IsConstraintViolation returns true if the SQLSTATE code represents a constraint violation.
func IsConstraintViolation(sqlstate string) bool {
	if len(sqlstate) >= 2 {
		return sqlstate[:2] == "23"
	}
	return false
}

// IsTransactionRollback returns true if the SQLSTATE code represents a transaction rollback.
func IsTransactionRollback(sqlstate string) bool {
	if len(sqlstate) >= 2 {
		return sqlstate[:2] == "40"
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
