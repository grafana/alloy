package collector

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestErrorLogsCollector_SQLStateClass(t *testing.T) {
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

func TestErrorLogsCollector_IsAuthenticationError(t *testing.T) {
	require.True(t, IsAuthenticationError("28000"))  // invalid_authorization_specification
	require.True(t, IsAuthenticationError("28P01"))  // invalid_password
	require.False(t, IsAuthenticationError("08006")) // connection exception
	require.False(t, IsAuthenticationError("23505")) // unique_violation
}

func TestErrorLogsCollector_IsResourceLimitError(t *testing.T) {
	require.True(t, IsResourceLimitError("53300"))  // too_many_connections
	require.True(t, IsResourceLimitError("53200"))  // out_of_memory
	require.True(t, IsResourceLimitError("53100"))  // disk_full
	require.False(t, IsResourceLimitError("54023")) // program_limit_exceeded (class 54, not 53)
	require.False(t, IsResourceLimitError("08006")) // connection exception
	require.False(t, IsResourceLimitError("40P01")) // deadlock
}

func TestErrorLogsCollector_SetTimeoutType(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
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
		expectedTimeout string
	}{
		{
			name:            "deadlock detected (40P01)",
			sqlstate:        "40P01",
			expectedTimeout: "deadlock",
		},
		{
			name:            "lock timeout (55P03)",
			sqlstate:        "55P03",
			expectedTimeout: "lock_timeout",
		},
		{
			name:            "query canceled (57014)",
			sqlstate:        "57014",
			expectedTimeout: "query_canceled",
		},
		{
			name:            "idle in transaction timeout (25P03)",
			sqlstate:        "25P03",
			expectedTimeout: "idle_in_transaction_timeout",
		},
		{
			name:            "idle session timeout (57P05)",
			sqlstate:        "57P05",
			expectedTimeout: "idle_session_timeout",
		},
		{
			name:            "serialization failure - no timeout type",
			sqlstate:        "40001",
			expectedTimeout: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := &ParsedError{
				SQLStateCode:      tt.sqlstate,
				SQLStateCodeClass: tt.sqlstate[:2],
			}

			collector.setTimeoutType(parsed)

			require.Equal(t, tt.expectedTimeout, parsed.TimeoutType, "timeout type should match")
		})
	}
}
