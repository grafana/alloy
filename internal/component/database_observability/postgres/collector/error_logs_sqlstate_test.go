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
