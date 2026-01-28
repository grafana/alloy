package collector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildExcludedSchemasClause(t *testing.T) {
	tests := []struct {
		name                string
		userExcludedSchemas []string
		expected            string
	}{
		{
			name:                "nil user schemas returns default schemas",
			userExcludedSchemas: nil,
			expected:            "('mysql', 'performance_schema', 'sys', 'information_schema')",
		},
		{
			name:                "empty user schemas returns default schemas",
			userExcludedSchemas: []string{},
			expected:            "('mysql', 'performance_schema', 'sys', 'information_schema')",
		},
		{
			name:                "single user schema is appended to default schemas",
			userExcludedSchemas: []string{"my_schema"},
			expected:            "('mysql', 'performance_schema', 'sys', 'information_schema', 'my_schema')",
		},
		{
			name:                "multiple user schemas are appended to default schemas",
			userExcludedSchemas: []string{"schema1", "schema2", "schema3"},
			expected:            "('mysql', 'performance_schema', 'sys', 'information_schema', 'schema1', 'schema2', 'schema3')",
		},
		{
			name:                "schema with single quote is escaped to prevent SQL injection",
			userExcludedSchemas: []string{"test'schema"},
			expected:            "('mysql', 'performance_schema', 'sys', 'information_schema', 'test''schema')",
		},
		{
			name:                "schema with SQL injection attempt is escaped",
			userExcludedSchemas: []string{"'; DROP TABLE users; --"},
			expected:            "('mysql', 'performance_schema', 'sys', 'information_schema', '''; DROP TABLE users; --')",
		},
		{
			name:                "schema with multiple single quotes is escaped",
			userExcludedSchemas: []string{"it's a test's schema"},
			expected:            "('mysql', 'performance_schema', 'sys', 'information_schema', 'it''s a test''s schema')",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := buildExcludedSchemasClause(tc.userExcludedSchemas)
			require.Equal(t, tc.expected, result)
		})
	}
}
