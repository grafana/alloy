package collector

import (
	"regexp"
	"strconv"
	"strings"
)

// Regular expressions for extracting insights from error messages
var (
	// Constraint name pattern: matches quoted constraint names
	constraintNamePattern = regexp.MustCompile(`constraint "([^"]+)"`)

	// Detail key pattern: matches Key (column_name)=(value) in DETAIL
	detailKeyPattern = regexp.MustCompile(`Key \(([^)]+)\)`)

	// Foreign key detail pattern: matches foreign key violation details
	foreignKeyDetailPattern = regexp.MustCompile(`Key \(([^)]+)\)=\([^)]+\) is not present in table "([^"]+)"`)

	// Not null patterns
	notNullColumnPattern = regexp.MustCompile(`null value in column "([^"]+)"`)
	notNullTablePattern  = regexp.MustCompile(`of relation "([^"]+)"`)

	// Table name from constraint: users_email_key -> users
	tableFromConstraintPattern = regexp.MustCompile(`^([^_]+)`)

	// Deadlock pattern from detail field
	deadlockProcessPattern = regexp.MustCompile(`Process (\d+) waits for (\w+) on transaction (\d+); blocked by process (\d+)`)

	// Relation (table) name patterns
	relationPattern = regexp.MustCompile(`relation "([^"]+)"`)

	// Column name in various error contexts
	columnPattern = regexp.MustCompile(`column "([^"]+)"`)

	// Lock and deadlock patterns
	tupleLocationPattern = regexp.MustCompile(`tuple \((\d+,\d+)\)`)
	lockTypePattern      = regexp.MustCompile(`waits for (\w+) on`)
	lockObtainPattern    = regexp.MustCompile(`could not obtain lock on (\w+)`)

	// Deadlock process patterns
	blockedByPattern    = regexp.MustCompile(`Process (\d+) waits for .+; blocked by process (\d+)`)
	processQueryPattern = regexp.MustCompile(`Process (\d+): (.+)(?:\n|$)`)

	// Authentication patterns
	authMethodPattern = regexp.MustCompile(`(\w+) authentication failed`)
	hbaLinePattern    = regexp.MustCompile(`pg_hba\.conf line (\d+)`)

	// Function patterns (PL/pgSQL and SQL)
	functionPattern = regexp.MustCompile(`(?:PL/pgSQL|SQL) function (?:")?([^"(\s]+)`)
)

// extractInsights extracts structured information from error messages.
func (c *ErrorLogs) extractInsights(parsed *ParsedError) {
	if parsed.SQLStateCode == "" {
		return
	}

	class := parsed.SQLStateClass

	switch class {
	case "23": // Constraint violations
		c.extractConstraintViolation(parsed)
	case "28": // Invalid authorization specification (auth failures)
		c.extractAuthFailure(parsed)
	case "40": // Transaction rollback (deadlocks, etc.)
		c.extractTransactionRollback(parsed)
	case "42": // Syntax errors or access violations
		c.extractSyntaxError(parsed)
	case "55": // Object not in prerequisite state (includes lock_not_available)
		c.extractObjectState(parsed)
	case "57": // Operator intervention (includes query cancellation/timeout)
		c.extractTimeoutError(parsed)
	}

	// Extract function context if present (PL/pgSQL errors) - multiple classes
	if parsed.Context != "" {
		c.extractFunctionInfo(parsed)
	}
}

// extractConstraintViolation extracts constraint violation details.
func (c *ErrorLogs) extractConstraintViolation(parsed *ParsedError) {
	msg := parsed.Message
	detail := parsed.Detail

	// Unique constraint: "duplicate key value violates unique constraint \"users_email_key\""
	if strings.Contains(msg, "unique constraint") {
		parsed.ConstraintType = "unique"

		if match := constraintNamePattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ConstraintName = match[1]

			// Try to extract table name from constraint name pattern
			// Common pattern: tablename_columnname_key or tablename_columnname_idx
			if tableMatch := tableFromConstraintPattern.FindStringSubmatch(parsed.ConstraintName); len(tableMatch) > 1 {
				parsed.TableName = tableMatch[1]
			}
		}

		if detail != "" {
			if match := detailKeyPattern.FindStringSubmatch(detail); len(match) > 1 {
				parsed.ColumnName = match[1]
			}
		}
		return
	}

	// Foreign key: "violates foreign key constraint \"posts_user_id_fkey\""
	if strings.Contains(msg, "foreign key constraint") {
		parsed.ConstraintType = "foreign_key"

		if match := constraintNamePattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ConstraintName = match[1]

			// Extract table name from constraint name
			if tableMatch := tableFromConstraintPattern.FindStringSubmatch(parsed.ConstraintName); len(tableMatch) > 1 {
				parsed.TableName = tableMatch[1]
			}
		}

		if detail != "" {
			if match := foreignKeyDetailPattern.FindStringSubmatch(detail); len(match) > 2 {
				parsed.ColumnName = match[1]
				parsed.ReferencedTable = match[2]
			}
		}

		// Also check message for table names
		if parsed.TableName == "" {
			// Message might be: update or delete on table "users" violates foreign key constraint
			if strings.Contains(msg, "on table") {
				if match := relationPattern.FindStringSubmatch(msg); len(match) > 1 {
					parsed.TableName = match[1]
				}
			}
		}
		return
	}

	// Not null: "null value in column \"username\" violates not-null constraint"
	if strings.Contains(msg, "not-null constraint") || strings.Contains(msg, "violates not-null constraint") {
		parsed.ConstraintType = "not_null"

		if match := notNullColumnPattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ColumnName = match[1]
		}

		if match := notNullTablePattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.TableName = match[1]
		}
		return
	}

	// Check constraint: "violates check constraint \"check_age_positive\""
	if strings.Contains(msg, "check constraint") {
		parsed.ConstraintType = "check"

		if match := constraintNamePattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ConstraintName = match[1]

			// Extract table name from constraint name
			if tableMatch := tableFromConstraintPattern.FindStringSubmatch(parsed.ConstraintName); len(tableMatch) > 1 {
				parsed.TableName = tableMatch[1]
			}
		}

		if parsed.TableName == "" {
			if match := relationPattern.FindStringSubmatch(msg); len(match) > 1 {
				parsed.TableName = match[1]
			}
		}
		return
	}

	// Exclusion constraint (PostgreSQL-specific): "violates exclusion constraint"
	if strings.Contains(msg, "exclusion constraint") {
		parsed.ConstraintType = "exclusion"

		if match := constraintNamePattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ConstraintName = match[1]

			// Extract table name from constraint name
			if tableMatch := tableFromConstraintPattern.FindStringSubmatch(parsed.ConstraintName); len(tableMatch) > 1 {
				parsed.TableName = tableMatch[1]
			}
		}
		return
	}
}

// extractTransactionRollback extracts information about transaction rollbacks.
func (c *ErrorLogs) extractTransactionRollback(parsed *ParsedError) {
	msg := parsed.Message
	detail := parsed.Detail
	context := parsed.Context

	if strings.Contains(msg, "deadlock detected") {
		// Tuple location from context: "while locking tuple (3,88)..." or detail: "Process 12345 waits for ShareLock on tuple (0,1)..."
		if match := tupleLocationPattern.FindStringSubmatch(context); len(match) > 1 {
			parsed.TupleLocation = match[1]
		} else if match := tupleLocationPattern.FindStringSubmatch(detail); len(match) > 1 {
			parsed.TupleLocation = match[1]
		}

		if match := lockTypePattern.FindStringSubmatch(detail); len(match) > 1 {
			parsed.LockType = match[1]
		}

		if match := blockedByPattern.FindStringSubmatch(detail); len(match) > 2 {
			if blockedPID, err := strconv.ParseInt(match[1], 10, 32); err == nil {
				parsed.BlockedPID = int32(blockedPID)
			}
			if blockerPID, err := strconv.ParseInt(match[2], 10, 32); err == nil {
				parsed.BlockerPID = int32(blockerPID)
			}
		}

		matches := processQueryPattern.FindAllStringSubmatch(detail, -1)
		for _, match := range matches {
			if len(match) > 2 {
				if pid, err := strconv.ParseInt(match[1], 10, 32); err == nil {
					query := strings.TrimSpace(match[2])
					if int32(pid) == parsed.BlockedPID {
						parsed.BlockedQuery = query
					} else if int32(pid) == parsed.BlockerPID {
						parsed.BlockerQuery = query
					}
				}
			}
		}

		return
	}
}

// extractSyntaxError extracts information about syntax errors and access violations.
func (c *ErrorLogs) extractSyntaxError(parsed *ParsedError) {
	msg := parsed.Message

	// Check column before general "does not exist" to avoid false matches
	if strings.Contains(msg, "column") && strings.Contains(msg, "does not exist") {
		if match := columnPattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ColumnName = match[1]
		}
		return
	}

	if strings.Contains(msg, "does not exist") {
		if match := relationPattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.TableName = match[1]
		}
		return
	}

	if strings.Contains(msg, "permission denied") {
		if match := relationPattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.TableName = match[1]
		} else {
			tablePattern := regexp.MustCompile(`permission denied for table (\w+)`)
			if match := tablePattern.FindStringSubmatch(msg); len(match) > 1 {
				parsed.TableName = match[1]
			}
		}
		return
	}
}

// extractObjectState extracts details from object state errors (Class 55).
func (c *ErrorLogs) extractObjectState(parsed *ParsedError) {
	msg := parsed.Message

	if parsed.SQLStateCode == "55P03" {
		parsed.TimeoutType = "lock_timeout"
		if match := lockObtainPattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.LockType = match[1]
		}
	}
}

// extractAuthFailure extracts details from authentication failures.
func (c *ErrorLogs) extractAuthFailure(parsed *ParsedError) {
	msg := parsed.Message
	detail := parsed.Detail

	// Example: "password authentication failed for user \"myuser\""
	if match := authMethodPattern.FindStringSubmatch(msg); len(match) > 1 {
		parsed.AuthMethod = match[1]
	}

	// Example: "Connection matched pg_hba.conf line 95: \"host all all 0.0.0.0/0 md5\""
	if match := hbaLinePattern.FindStringSubmatch(detail); len(match) > 1 {
		parsed.HBALineNumber = match[1]
	}
}

// extractTimeoutError extracts timeout type from query cancellation errors.
func (c *ErrorLogs) extractTimeoutError(parsed *ParsedError) {
	msg := parsed.Message

	if strings.Contains(msg, "statement timeout") {
		parsed.TimeoutType = "statement_timeout"
	} else if strings.Contains(msg, "lock timeout") {
		parsed.TimeoutType = "lock_timeout"
	} else if strings.Contains(msg, "canceling statement due to user request") {
		parsed.TimeoutType = "user_cancel"
	} else if strings.Contains(msg, "idle_in_transaction_session_timeout") {
		parsed.TimeoutType = "idle_in_transaction_timeout"
	}
}

// extractFunctionInfo extracts function names from PL/pgSQL error contexts.
// Examples: "PL/pgSQL function my_function(integer) line 42 at RAISE" or "SQL function \"my_func\" statement 1"
func (c *ErrorLogs) extractFunctionInfo(parsed *ParsedError) {
	if match := functionPattern.FindStringSubmatch(parsed.Context); len(match) > 1 {
		parsed.FunctionContext = match[1]
	}
}
