package collector

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestErrorLogsCollector_ParseJSON(t *testing.T) {
	tests := []struct {
		name          string
		jsonLog       string
		expectedError bool
		checkFields   func(*testing.T, *ParsedError)
	}{
		{
			name: "unique constraint violation",
			jsonLog: `{
				"timestamp": "2024-11-28 10:15:30.123 UTC",
				"user": "testuser",
				"dbname": "testdb",
				"pid": 12345,
				"session_id": "64c6e836.474e",
				"line_num": 1,
				"session_start": "2024-11-28 10:15:00 UTC",
				"backend_type": "client backend",
				"error_severity": "ERROR",
				"state_code": "23505",
				"message": "duplicate key value violates unique constraint \"users_email_key\"",
				"detail": "Key (email)=(user@example.com) already exists.",
				"statement": "INSERT INTO users (email) VALUES ('user@example.com')",
				"query_id": 1234567890
			}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "ERROR", p.ErrorSeverity)
				require.Equal(t, "23505", p.SQLStateCode)
				require.Equal(t, "23", p.SQLStateClass)
				require.Equal(t, "Integrity Constraint Violation", p.ErrorCategory)
				require.Equal(t, "testuser", p.User)
				require.Equal(t, "testdb", p.DatabaseName)
				require.Equal(t, int32(12345), p.PID)
				require.Equal(t, int64(1234567890), p.QueryID)
			},
		},
		{
			name: "foreign key violation",
			jsonLog: `{
				"timestamp": "2024-11-28 10:16:30.123 UTC",
				"user": "appuser",
				"dbname": "appdb",
				"pid": 23456,
				"session_id": "64c6e836.474f",
				"line_num": 2,
				"session_start": "2024-11-28 10:16:00 UTC",
				"backend_type": "client backend",
				"error_severity": "ERROR",
				"state_code": "23503",
				"message": "insert or update on table \"posts\" violates foreign key constraint \"posts_user_id_fkey\"",
				"detail": "Key (user_id)=(999) is not present in table \"users\".",
				"statement": "INSERT INTO posts (user_id, title) VALUES (999, 'Test')"
			}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "ERROR", p.ErrorSeverity)
				require.Equal(t, "23503", p.SQLStateCode)
				require.Equal(t, "23", p.SQLStateClass)
				require.Equal(t, "Integrity Constraint Violation", p.ErrorCategory)
			},
		},
		{
			name: "connection error",
			jsonLog: `{
				"timestamp": "2024-11-28 10:17:30.123 UTC",
				"pid": 34567,
				"session_id": "64c6e836.4750",
				"line_num": 1,
				"session_start": "2024-11-28 10:17:00 UTC",
				"backend_type": "client backend",
				"error_severity": "FATAL",
				"state_code": "08006",
				"message": "terminating connection due to administrator command"
			}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "FATAL", p.ErrorSeverity)
				require.Equal(t, "08006", p.SQLStateCode)
				require.Equal(t, "08", p.SQLStateClass)
				require.Equal(t, "Connection Exception", p.ErrorCategory)
			},
		},
		{
			name: "deadlock detected",
			jsonLog: `{
				"timestamp": "2024-11-28 10:18:30.123 UTC",
				"user": "appuser",
				"dbname": "appdb",
				"pid": 45678,
				"session_id": "64c6e836.4751",
				"line_num": 5,
				"session_start": "2024-11-28 10:18:00 UTC",
				"backend_type": "client backend",
				"error_severity": "ERROR",
				"state_code": "40P01",
				"message": "deadlock detected",
				"detail": "Process 45678 waits for ShareLock on transaction 12345; blocked by process 56789.",
				"hint": "See server log for query details.",
				"context": "while updating tuple (0,1) in relation \"accounts\"",
				"statement": "UPDATE accounts SET balance = balance - 100 WHERE id = 1"
			}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "ERROR", p.ErrorSeverity)
				require.Equal(t, "40P01", p.SQLStateCode)
				require.Equal(t, "40", p.SQLStateClass)
				require.Equal(t, "Transaction Rollback", p.ErrorCategory)
				require.NotEmpty(t, p.Detail)
				require.NotEmpty(t, p.Hint)
				require.NotEmpty(t, p.Context)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock collector
			entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
			collector, err := NewErrorLogs(ErrorLogsArguments{
				Receiver:     loki.NewLogsReceiver(),
				Severities:   []string{"ERROR", "FATAL", "PANIC"},
				PassThrough:  false,
				EntryHandler: entryHandler,
				Logger:       log.NewNopLogger(),
				InstanceKey:  "test-instance",
				SystemID:     "test-system",
			})
			require.NoError(t, err)

			// Parse JSON log
			var jsonLog PostgreSQLJSONLog
			err = json.Unmarshal([]byte(tt.jsonLog), &jsonLog)
			require.NoError(t, err)

			// Build parsed error
			parsed, err := collector.buildParsedError(&jsonLog)
			if tt.expectedError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, parsed)

			// Check fields
			if tt.checkFields != nil {
				tt.checkFields(t, parsed)
			}
		})
	}
}

func TestErrorLogsCollector_ExtractConstraintViolation(t *testing.T) {
	tests := []struct {
		name               string
		message            string
		detail             string
		sqlstate           string
		expectedType       string
		expectedConstraint string
		expectedTable      string
		expectedColumn     string
	}{
		{
			name:               "unique constraint",
			message:            `duplicate key value violates unique constraint "users_email_key"`,
			detail:             "Key (email)=(user@example.com) already exists.",
			sqlstate:           "23505",
			expectedType:       "unique",
			expectedConstraint: "users_email_key",
			expectedTable:      "users",
			expectedColumn:     "email",
		},
		{
			name:               "foreign key constraint",
			message:            `insert or update on table "posts" violates foreign key constraint "posts_user_id_fkey"`,
			detail:             `Key (user_id)=(999) is not present in table "users".`,
			sqlstate:           "23503",
			expectedType:       "foreign_key",
			expectedConstraint: "posts_user_id_fkey",
			expectedTable:      "posts",
			expectedColumn:     "user_id",
		},
		{
			name:               "not null constraint",
			message:            `null value in column "username" of relation "users" violates not-null constraint`,
			detail:             `Failing row contains (1, null, user@example.com).`,
			sqlstate:           "23502",
			expectedType:       "not_null",
			expectedConstraint: "",
			expectedTable:      "users",
			expectedColumn:     "username",
		},
		{
			name:               "check constraint",
			message:            `new row for relation "products" violates check constraint "check_price_positive"`,
			detail:             `Failing row contains (1, Widget, -10).`,
			sqlstate:           "23514",
			expectedType:       "check",
			expectedConstraint: "check_price_positive",
			expectedTable:      "",
			expectedColumn:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create collector
			entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
			collector, err := NewErrorLogs(ErrorLogsArguments{
				Receiver:     loki.NewLogsReceiver(),
				Severities:   []string{"ERROR"},
				PassThrough:  false,
				EntryHandler: entryHandler,
				Logger:       log.NewNopLogger(),
				InstanceKey:  "test",
				SystemID:     "test",
			})
			require.NoError(t, err)

			// Create parsed error
			parsed := &ParsedError{
				Message:       tt.message,
				Detail:        tt.detail,
				SQLStateCode:  tt.sqlstate,
				SQLStateClass: tt.sqlstate[:2],
			}

			// Extract insights
			collector.extractInsights(parsed)

			// Check results
			require.Equal(t, tt.expectedType, parsed.ConstraintType, "constraint type mismatch")
			if tt.expectedConstraint != "" {
				require.Equal(t, tt.expectedConstraint, parsed.ConstraintName, "constraint name mismatch")
			}
			if tt.expectedTable != "" {
				require.Equal(t, tt.expectedTable, parsed.TableName, "table name mismatch")
			}
			if tt.expectedColumn != "" {
				require.Equal(t, tt.expectedColumn, parsed.ColumnName, "column name mismatch")
			}
		})
	}
}

func TestErrorLogsCollector_SQLStateCategories(t *testing.T) {
	tests := []struct {
		sqlstate string
		category string
	}{
		{"23505", "Integrity Constraint Violation"},
		{"08006", "Connection Exception"},
		{"40P01", "Transaction Rollback"},
		{"42P01", "Syntax Error or Access Rule Violation"},
		{"53200", "Insufficient Resources"},
		{"XX000", "Internal Error"},
	}

	for _, tt := range tests {
		t.Run(tt.sqlstate, func(t *testing.T) {
			category := GetSQLStateCategory(tt.sqlstate)
			require.Equal(t, tt.category, category)
		})
	}
}

func TestErrorLogsCollector_IsConnectionError(t *testing.T) {
	require.True(t, IsConnectionError("08006"))
	require.True(t, IsConnectionError("08001"))
	require.False(t, IsConnectionError("23505"))
	require.False(t, IsConnectionError("40P01"))
}

func TestErrorLogsCollector_IsConstraintViolation(t *testing.T) {
	require.True(t, IsConstraintViolation("23505"))
	require.True(t, IsConstraintViolation("23503"))
	require.False(t, IsConstraintViolation("08006"))
	require.False(t, IsConstraintViolation("40P01"))
}

func TestErrorLogsCollector_StartStop(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})

	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		Severities:   []string{"ERROR", "FATAL", "PANIC"},
		PassThrough:  false,
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
	})
	require.NoError(t, err)
	require.NotNil(t, collector)
	require.NotNil(t, collector.Receiver(), "receiver should be exported")

	// Start collector
	err = collector.Start(context.Background())
	require.NoError(t, err)
	require.False(t, collector.Stopped())

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop collector
	collector.Stop()

	// Give it a moment to stop
	time.Sleep(10 * time.Millisecond)
	require.True(t, collector.Stopped())
}
