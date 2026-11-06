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
		defaults := []string{"system"}
		inherited := []string{"system", "inherited"}

		resolved := ResolveExclusions(defaults, inherited, nil)
		resolved[0] = "changed"

		require.Equal(t, []string{"changed", "inherited"}, resolved)
		require.Equal(t, []string{"system"}, defaults)
		require.Equal(t, []string{"system", "inherited"}, inherited)
	})

	t.Run("uses explicit empty override while keeping default exclusions", func(t *testing.T) {
		resolved := ResolveExclusions([]string{"system"}, []string{"inherited"}, []string{})

		require.Equal(t, []string{"system"}, resolved)
	})

	t.Run("uses configured override without mutating inputs", func(t *testing.T) {
		defaults := []string{"system"}
		override := []string{"system", "override"}

		resolved := ResolveExclusions(defaults, []string{"inherited"}, override)
		resolved[1] = "changed"

		require.Equal(t, []string{"system", "changed"}, resolved)
		require.Equal(t, []string{"system"}, defaults)
		require.Equal(t, []string{"system", "override"}, override)
	})
}

func TestSameExclusions(t *testing.T) {
	require.True(t, SameExclusions([]string{"b", "a", "a"}, []string{"a", "b"}))
	require.True(t, SameExclusions(nil, []string{}))
	require.False(t, SameExclusions([]string{"a", "b"}, []string{"a", "c"}))
}

func TestEffectiveExclusions(t *testing.T) {
	defaults := []string{"rdsadmin", "cloudsqladmin", "azuresu"}

	t.Run("untouched default slice is treated as unset user exclusions", func(t *testing.T) {
		require.Nil(t, UserConfiguredExclusions(defaults, defaults))
		require.Equal(t, []string{"rdsadmin", "cloudsqladmin", "azuresu"}, EffectiveExclusions(defaults, defaults, nil, false))
	})

	t.Run("include removes matching items from default exclusions", func(t *testing.T) {
		require.Equal(t, []string{"cloudsqladmin", "azuresu"}, EffectiveExclusions(defaults, defaults, []string{"rdsadmin", "app_db"}, false))
		require.Equal(t, []string{"cloudsqladmin", "azuresu", "legacy_db"}, EffectiveExclusions(defaults, []string{"legacy_db"}, []string{"rdsadmin", "app_db"}, false))
	})

	t.Run("case-insensitive include removes matching default exclusions regardless of case", func(t *testing.T) {
		require.Equal(t, []string{"cloudsqladmin", "azuresu"}, EffectiveExclusions(defaults, defaults, []string{"RDSAdmin"}, true))
	})
}

func TestResolveInclusions(t *testing.T) {
	t.Run("inherits when override is unset", func(t *testing.T) {
		inherited := []string{"db1", "db2", "db1"}

		resolved := ResolveInclusions(inherited, nil)
		resolved[0] = "changed"

		require.Equal(t, []string{"changed", "db2"}, resolved)
		require.Equal(t, []string{"db1", "db2", "db1"}, inherited)
	})

	t.Run("uses explicit empty override", func(t *testing.T) {
		resolved := ResolveInclusions([]string{"db1"}, []string{})

		require.Nil(t, resolved)
	})

	t.Run("uses configured override without mutating inputs", func(t *testing.T) {
		override := []string{"override1", "override2"}

		resolved := ResolveInclusions([]string{"inherited"}, override)
		resolved[0] = "changed"

		require.Equal(t, []string{"changed", "override2"}, resolved)
		require.Equal(t, []string{"override1", "override2"}, override)
	})
}

func TestValidateInclusions(t *testing.T) {
	alwaysExcluded := []string{"sys"}
	defaults := []string{"rdsadmin"}

	t.Run("accepts include overlapping only untouched defaults", func(t *testing.T) {
		require.NoError(t, ValidateInclusions([]string{"rdsadmin"}, alwaysExcluded, defaults, defaults, "include_schemas", "exclude_schemas", false))
	})

	t.Run("rejects include overlapping always-excluded items", func(t *testing.T) {
		err := ValidateInclusions([]string{"app", "sys"}, alwaysExcluded, defaults, defaults, "include_schemas", "exclude_schemas", false)
		require.EqualError(t, err, `include_schemas and exclude_schemas cannot both contain "sys"`)
	})
}

func TestValidateDisjointLists(t *testing.T) {
	t.Run("disjoint lists pass", func(t *testing.T) {
		require.NoError(t, validateDisjointLists([]string{"a", "b"}, []string{"c", "d"}, "include_schemas", "exclude_schemas", false))
	})

	t.Run("empty lists pass", func(t *testing.T) {
		require.NoError(t, validateDisjointLists(nil, []string{"c"}, "include_schemas", "exclude_schemas", false))
		require.NoError(t, validateDisjointLists([]string{"a"}, nil, "include_schemas", "exclude_schemas", false))
	})

	t.Run("exact overlap fails when case sensitive", func(t *testing.T) {
		err := validateDisjointLists([]string{"app", "shared"}, []string{"rdsadmin", "shared"}, "include_schemas", "exclude_schemas", false)
		require.EqualError(t, err, `include_schemas and exclude_schemas cannot both contain "shared"`)
	})

	t.Run("different case passes when case sensitive", func(t *testing.T) {
		require.NoError(t, validateDisjointLists([]string{"Shared"}, []string{"shared"}, "include_schemas", "exclude_schemas", false))
	})

	t.Run("different case fails when case insensitive", func(t *testing.T) {
		err := validateDisjointLists([]string{"Shared"}, []string{"shared"}, "include_databases", "exclude_databases", true)
		require.EqualError(t, err, `include_databases and exclude_databases cannot both contain "Shared"`)
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildExclusionClause(tc.items)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestBuildInclusionClause(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		require.Equal(t, "", BuildInclusionClause("schema_name", nil))
	})

	t.Run("single item", func(t *testing.T) {
		require.Equal(t, " AND schema_name IN ('app')", BuildInclusionClause("schema_name", []string{"app"}))
	})

	t.Run("items with special characters", func(t *testing.T) {
		require.Equal(t, " AND d.datname IN ('test''value', 'normal')", BuildInclusionClause("d.datname", []string{"test'value", "normal"}))
	})
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
