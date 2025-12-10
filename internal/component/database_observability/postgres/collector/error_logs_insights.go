package collector

import (
	"regexp"
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
)

// extractInsights extracts structured information from error messages.
func (c *ErrorLogs) extractInsights(parsed *ParsedError) {
	// Only extract insights for errors with SQLSTATE codes
	if parsed.SQLStateCode == "" {
		return
	}

	class := parsed.SQLStateClass

	switch class {
	case "23": // Constraint violations
		c.extractConstraintViolation(parsed)
	case "40": // Transaction rollback (deadlocks, etc.)
		c.extractTransactionRollback(parsed)
	case "42": // Syntax errors or access violations
		c.extractSyntaxError(parsed)
	}
}

// extractConstraintViolation extracts constraint violation details.
func (c *ErrorLogs) extractConstraintViolation(parsed *ParsedError) {
	msg := parsed.Message
	detail := parsed.Detail

	// Unique constraint: "duplicate key value violates unique constraint \"users_email_key\""
	if strings.Contains(msg, "unique constraint") {
		parsed.ConstraintType = "unique"

		// Extract constraint name
		if match := constraintNamePattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ConstraintName = match[1]

			// Try to extract table name from constraint name pattern
			// Common pattern: tablename_columnname_key or tablename_columnname_idx
			if tableMatch := tableFromConstraintPattern.FindStringSubmatch(parsed.ConstraintName); len(tableMatch) > 1 {
				parsed.TableName = tableMatch[1]
			}
		}

		// Extract column from detail: "Key (email)=(user@example.com) already exists."
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

		// Extract constraint name
		if match := constraintNamePattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ConstraintName = match[1]

			// Extract table name from constraint name
			if tableMatch := tableFromConstraintPattern.FindStringSubmatch(parsed.ConstraintName); len(tableMatch) > 1 {
				parsed.TableName = tableMatch[1]
			}
		}

		// Extract referenced table and column from detail
		if detail != "" {
			// Pattern: Key (user_id)=(123) is not present in table "users".
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

		// Extract column name
		if match := notNullColumnPattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ColumnName = match[1]
		}

		// Extract table name
		if match := notNullTablePattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.TableName = match[1]
		}
		return
	}

	// Check constraint: "violates check constraint \"check_age_positive\""
	if strings.Contains(msg, "check constraint") {
		parsed.ConstraintType = "check"

		// Extract constraint name
		if match := constraintNamePattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ConstraintName = match[1]

			// Extract table name from constraint name
			if tableMatch := tableFromConstraintPattern.FindStringSubmatch(parsed.ConstraintName); len(tableMatch) > 1 {
				parsed.TableName = tableMatch[1]
			}
		}

		// Try to extract table from message
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

		// Extract constraint name
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

	// Deadlock detected
	if strings.Contains(msg, "deadlock detected") {
		// Deadlock information is usually in the detail field
		// Example: Process 12345 waits for ShareLock on transaction 67890; blocked by process 23456.
		if detail != "" {
			// We could extract process IDs and lock types here if needed
			// For now, the message and detail are sufficient
		}
		return
	}

	// Serialization failure
	if strings.Contains(msg, "could not serialize access") {
		// These usually don't have additional structured info to extract
		return
	}
}

// extractSyntaxError extracts information about syntax errors and access violations.
func (c *ErrorLogs) extractSyntaxError(parsed *ParsedError) {
	msg := parsed.Message

	// Relation does not exist: "relation \"users\" does not exist"
	if strings.Contains(msg, "does not exist") {
		if match := relationPattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.TableName = match[1]
		}
		return
	}

	// Column does not exist
	if strings.Contains(msg, "column") && strings.Contains(msg, "does not exist") {
		if match := columnPattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.ColumnName = match[1]
		}
		return
	}

	// Permission denied: "permission denied for table users"
	if strings.Contains(msg, "permission denied") {
		if match := relationPattern.FindStringSubmatch(msg); len(match) > 1 {
			parsed.TableName = match[1]
		}
		return
	}
}
