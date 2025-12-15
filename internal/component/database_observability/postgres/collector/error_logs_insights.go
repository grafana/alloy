package collector

import (
	"regexp"
	"strconv"
	"strings"
)

// Regular expressions for extracting insights from error messages
var (
	// Lock and deadlock patterns
	tupleLocationPattern = regexp.MustCompile(`tuple \((\d+,\d+)\)`)
	lockTypePattern      = regexp.MustCompile(`waits for (\w+) on`)
	lockObtainPattern    = regexp.MustCompile(`could not obtain lock on (\w+)`)

	// Deadlock process patterns
	blockedByPattern    = regexp.MustCompile(`Process (\d+) waits for .+; blocked by process (\d+)`)
	processQueryPattern = regexp.MustCompile(`Process (\d+): (.+)(?:\n|$)`)

	// Authentication patterns
	// Matches methods like "password", "md5", "scram-sha-256", "gss-krb5", etc.
	authMethodPattern = regexp.MustCompile(`([\w-]+) authentication failed`)
	// Matches HBA config file line numbers from various formats:
	// - "Connection matched pg_hba.conf line 95: ..."
	// - "Connection matched file \"/etc/postgresql/pg_hba.conf\" line 4: ..."
	// - "Connection matched file \"/custom/pg_hba_cluster.conf\" line 123: ..."
	// Flexible pattern works with any HBA file name/path
	hbaLinePattern = regexp.MustCompile(`line (\d+):`)
)

// extractInsights extracts structured information from error messages.
func (c *ErrorLogs) extractInsights(parsed *ParsedError) {
	if parsed.SQLStateCode == "" {
		return
	}

	class := parsed.SQLStateClass

	switch class {
	case "28": // Invalid authorization specification (auth failures)
		c.extractAuthFailure(parsed)
	case "40": // Transaction rollback (deadlocks, etc.)
		c.extractTransactionRollback(parsed)
	case "55": // Object not in prerequisite state (includes lock_not_available)
		c.extractObjectState(parsed)
	case "57": // Operator intervention (includes query cancellation/timeout)
		c.extractTimeoutError(parsed)
	}
}

// extractTransactionRollback extracts information about transaction rollbacks (deadlocks).
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

		// Extract blocker PID from "blocked by process XXXX"
		if match := blockedByPattern.FindStringSubmatch(detail); len(match) > 2 {
			if blockerPID, err := strconv.ParseInt(match[2], 10, 32); err == nil {
				parsed.BlockerPID = int32(blockerPID)
			}
		}

		// Extract blocker query from detail (Process XXX: query)
		if parsed.BlockerPID > 0 {
			matches := processQueryPattern.FindAllStringSubmatch(detail, -1)
			for _, match := range matches {
				if len(match) > 2 {
					if pid, err := strconv.ParseInt(match[1], 10, 32); err == nil {
						if int32(pid) == parsed.BlockerPID {
							parsed.BlockerQuery = strings.TrimSpace(match[2])
							break
						}
					}
				}
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
