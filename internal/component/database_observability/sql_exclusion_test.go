package database_observability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildExclusionClause(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		expected string
	}{
		{
			name:     "single item",
			items:    []string{"information_schema"},
			expected: "('information_schema')",
		},
		{
			name:     "multiple items",
			items:    []string{"information_schema", "performance_schema", "sys"},
			expected: "('information_schema', 'performance_schema', 'sys')",
		},
		{
			name:     "items with special characters",
			items:    []string{"test'value", "normal"},
			expected: "('test''value', 'normal')",
		},
		{
			name:     "empty slice",
			items:    []string{},
			expected: "()",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildExclusionClause(tc.items)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestEscapeSQLString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "test",
			expected: "'test'",
		},
		{
			name:     "string with single quote",
			input:    "test'value",
			expected: "'test''value'",
		},
		{
			name:     "string with multiple single quotes",
			input:    "it's a test's value",
			expected: "'it''s a test''s value'",
		},
		{
			name:     "SQL injection attempt",
			input:    "'; DROP TABLE users; --",
			expected: "'''; DROP TABLE users; --'",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := EscapeSQLString(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}
