package database_observability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeExclusions(t *testing.T) {
	first := []string{"system", "shared"}
	second := []string{"shared", "configured"}

	merged := MergeExclusions(first, second)

	require.Equal(t, []string{"system", "shared", "configured"}, merged)
	require.Equal(t, []string{"system", "shared"}, first)
	require.Equal(t, []string{"shared", "configured"}, second)
}

func TestResolveExclusions(t *testing.T) {
	t.Run("inherits when override is unset", func(t *testing.T) {
		mandatory := []string{"system"}
		inherited := []string{"system", "inherited"}

		resolved := ResolveExclusions(mandatory, inherited, nil)
		resolved[0] = "changed"

		require.Equal(t, []string{"changed", "inherited"}, resolved)
		require.Equal(t, []string{"system"}, mandatory)
		require.Equal(t, []string{"system", "inherited"}, inherited)
	})

	t.Run("uses explicit empty override while keeping mandatory exclusions", func(t *testing.T) {
		resolved := ResolveExclusions([]string{"system"}, []string{"inherited"}, []string{})

		require.Equal(t, []string{"system"}, resolved)
	})

	t.Run("uses configured override without mutating inputs", func(t *testing.T) {
		mandatory := []string{"system"}
		override := []string{"system", "override"}

		resolved := ResolveExclusions(mandatory, []string{"inherited"}, override)
		resolved[1] = "changed"

		require.Equal(t, []string{"system", "changed"}, resolved)
		require.Equal(t, []string{"system"}, mandatory)
		require.Equal(t, []string{"system", "override"}, override)
	})
}

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
			result := escapeSQLString(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}
