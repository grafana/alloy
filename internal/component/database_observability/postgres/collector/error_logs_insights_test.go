package collector

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

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
				Registry:     prometheus.NewRegistry(),
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

func TestErrorLogsCollector_ExtractTransactionRollback(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		Severities:   []string{"ERROR"},
		PassThrough:  false,
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)

	tests := []struct {
		name            string
		sqlstate        string
		message         string
		detail          string
		expectedLock    string
		expectedTuple   string
		expectedTimeout string
	}{
		{
			name:          "deadlock with tuple location",
			sqlstate:      "40P01",
			message:       "deadlock detected",
			detail:        "Process 12345 waits for ShareLock on tuple (0,1) of relation 12345",
			expectedLock:  "ShareLock",
			expectedTuple: "0,1",
		},
		{
			name:          "deadlock with exclusive lock",
			sqlstate:      "40P01",
			message:       "deadlock detected",
			detail:        "Process 67890 waits for ExclusiveLock on tuple (1,234) of relation 67890",
			expectedLock:  "ExclusiveLock",
			expectedTuple: "1,234",
		},
		{
			name:            "lock timeout",
			sqlstate:        "55P03",
			message:         "could not obtain lock on relation",
			detail:          "",
			expectedTimeout: "lock_timeout",
		},
		{
			name:     "serialization failure",
			sqlstate: "40001",
			message:  "could not serialize access due to concurrent update",
			detail:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := &ParsedError{
				SQLStateCode:  tt.sqlstate,
				SQLStateClass: tt.sqlstate[:2],
				Message:       tt.message,
				Detail:        tt.detail,
			}

			collector.extractInsights(parsed)

			if tt.expectedLock != "" {
				require.Equal(t, tt.expectedLock, parsed.LockType, "lock type should be extracted")
			}
			if tt.expectedTuple != "" {
				require.Equal(t, tt.expectedTuple, parsed.TupleLocation, "tuple location should be extracted")
			}
			if tt.expectedTimeout != "" {
				require.Equal(t, tt.expectedTimeout, parsed.TimeoutType, "timeout type should be extracted")
			}
		})
	}
}

func TestErrorLogsCollector_ExtractSyntaxError(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		Severities:   []string{"ERROR"},
		PassThrough:  false,
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		sqlstate      string
		message       string
		expectedTable string
		expectedCol   string
	}{
		{
			name:          "relation does not exist",
			sqlstate:      "42P01",
			message:       `relation "users" does not exist`,
			expectedTable: "users",
		},
		{
			name:        "column does not exist",
			sqlstate:    "42703",
			message:     `column "email" does not exist`,
			expectedCol: "email",
		},
		{
			name:          "permission denied",
			sqlstate:      "42501",
			message:       `permission denied for table orders`,
			expectedTable: "orders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := &ParsedError{
				SQLStateCode:  tt.sqlstate,
				SQLStateClass: tt.sqlstate[:2],
				Message:       tt.message,
			}

			collector.extractInsights(parsed)

			if tt.expectedTable != "" {
				require.Equal(t, tt.expectedTable, parsed.TableName, "table name should be extracted")
			}
			if tt.expectedCol != "" {
				require.Equal(t, tt.expectedCol, parsed.ColumnName, "column name should be extracted")
			}
		})
	}
}

func TestErrorLogsCollector_ExtractAuthFailure(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		Severities:   []string{"FATAL"},
		PassThrough:  false,
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)

	tests := []struct {
		name            string
		sqlstate        string
		message         string
		detail          string
		expectedAuth    string
		expectedHBALine string
	}{
		{
			name:            "password auth failed",
			sqlstate:        "28P01",
			message:         `password authentication failed for user "myuser"`,
			detail:          `Connection matched pg_hba.conf line 95: "host all all 0.0.0.0/0 md5"`,
			expectedAuth:    "password",
			expectedHBALine: "95",
		},
		{
			name:            "md5 auth failed",
			sqlstate:        "28000",
			message:         `md5 authentication failed for user "admin"`,
			detail:          `Connection matched pg_hba.conf line 10: "local all all md5"`,
			expectedAuth:    "md5",
			expectedHBALine: "10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := &ParsedError{
				SQLStateCode:  tt.sqlstate,
				SQLStateClass: tt.sqlstate[:2],
				Message:       tt.message,
				Detail:        tt.detail,
			}

			collector.extractInsights(parsed)

			if tt.expectedAuth != "" {
				require.Equal(t, tt.expectedAuth, parsed.AuthMethod, "auth method should be extracted")
			}
			if tt.expectedHBALine != "" {
				require.Equal(t, tt.expectedHBALine, parsed.HBALineNumber, "HBA line number should be extracted")
			}
		})
	}
}

func TestErrorLogsCollector_ExtractTimeoutError(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		Severities:   []string{"ERROR"},
		PassThrough:  false,
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)

	tests := []struct {
		name            string
		sqlstate        string
		message         string
		expectedTimeout string
	}{
		{
			name:            "statement timeout",
			sqlstate:        "57014",
			message:         "canceling statement due to statement timeout",
			expectedTimeout: "statement_timeout",
		},
		{
			name:            "lock timeout",
			sqlstate:        "57014",
			message:         "canceling statement due to lock timeout",
			expectedTimeout: "lock_timeout",
		},
		{
			name:            "user cancel",
			sqlstate:        "57014",
			message:         "canceling statement due to user request",
			expectedTimeout: "user_cancel",
		},
		{
			name:            "idle in transaction timeout",
			sqlstate:        "57014",
			message:         "terminating connection due to idle_in_transaction_session_timeout",
			expectedTimeout: "idle_in_transaction_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := &ParsedError{
				SQLStateCode:  tt.sqlstate,
				SQLStateClass: tt.sqlstate[:2],
				Message:       tt.message,
			}

			collector.extractInsights(parsed)

			if tt.expectedTimeout != "" {
				require.Equal(t, tt.expectedTimeout, parsed.TimeoutType, "timeout type should be extracted")
			}
		})
	}
}

func TestErrorLogsCollector_ExtractFunctionInfo(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		Severities:   []string{"ERROR"},
		PassThrough:  false,
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)

	tests := []struct {
		name             string
		context          string
		expectedFunction string
	}{
		{
			name:             "PL/pgSQL function with arguments",
			context:          "PL/pgSQL function my_function(integer) line 42 at RAISE",
			expectedFunction: "my_function",
		},
		{
			name:             "PL/pgSQL function without arguments",
			context:          "PL/pgSQL function calculate_total line 10 at assignment",
			expectedFunction: "calculate_total",
		},
		{
			name:             "SQL function with quotes",
			context:          `SQL function "my_func" statement 1`,
			expectedFunction: "my_func",
		},
		{
			name:             "SQL function without quotes",
			context:          "SQL function process_order statement 2",
			expectedFunction: "process_order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := &ParsedError{
				SQLStateCode:  "42883", // function does not exist
				SQLStateClass: "42",
				Context:       tt.context,
			}

			collector.extractInsights(parsed)

			if tt.expectedFunction != "" {
				require.Equal(t, tt.expectedFunction, parsed.FunctionContext, "function name should be extracted")
			}
		})
	}
}
