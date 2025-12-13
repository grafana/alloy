package collector

import (
	"testing"

	"github.com/stretchr/testify/require"
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

func TestErrorLogsCollector_IsConstraintViolation(t *testing.T) {
	require.True(t, IsConstraintViolation("23505"))
	require.True(t, IsConstraintViolation("23503"))
	require.False(t, IsConstraintViolation("08006"))
	require.False(t, IsConstraintViolation("40P01"))
}

func TestErrorLogsCollector_IsResourceLimitError(t *testing.T) {
	require.True(t, IsResourceLimitError("53300"))  // too_many_connections
	require.True(t, IsResourceLimitError("53200"))  // out_of_memory
	require.True(t, IsResourceLimitError("53100"))  // disk_full
	require.False(t, IsResourceLimitError("54023")) // program_limit_exceeded (class 54, not 53)
	require.False(t, IsResourceLimitError("08006")) // connection exception
	require.False(t, IsResourceLimitError("40P01")) // deadlock
}
