package database_observability

import (
	"slices"
	"strings"
)

func DefaultMySQLExcludedSchemas() []string {
	return []string{
		"alloydbadmin",
		"alloydbmetadata",
		"azure_maintenance",
		"azure_sys",
		"cloudsqladmin",
		"rdsadmin",
	}
}

func AlwaysExcludedMySQLSchemas() []string {
	return []string{"mysql", "performance_schema", "sys", "information_schema"}
}

func DefaultPostgresExcludedDatabases() []string {
	return []string{
		"alloydbadmin",
		"alloydbmetadata",
		"azure_maintenance",
		"azure_sys",
		"cloudsqladmin",
		"rdsadmin",
	}
}

func DefaultPostgresExcludedUsers() []string {
	return []string{
		"azuresu",
		"cloudsqladmin",
		"db-o11y", // default recommended user
		"rdsadmin",
	}
}

func AlwaysExcludedPostgresDatabases() []string {
	return []string{
		"azure_maintenance", // Azure Database for PostgreSQL exposes this database monitoring users cannot read its pgavailability schema
	}
}

func DefaultSQLServerExcludedSchemas() []string {
	return []string{
		"alloydbadmin",
		"alloydbmetadata",
		"azure_maintenance",
		"azure_sys",
		"cloudsqladmin",
		"rdsadmin",
	}
}

func DefaultSQLServerExcludedDatabases() []string {
	return []string{
		"alloydbadmin",
		"alloydbmetadata",
		"azure_maintenance",
		"azure_sys",
		"cloudsqladmin",
		"rdsadmin",
	}
}

func DefaultSQLServerExcludedUsers() []string {
	return []string{
		"azuresu",
		"cloudsqladmin",
		"db-o11y", // default recommended user
		"rdsadmin",
	}
}

func AlwaysExcludedSQLServerSchemas() []string {
	return []string{"sys", "information_schema"}
}

// MergeExclusions merges exclusion lists in order and removes duplicates.
func MergeExclusions(exclusionLists ...[]string) []string {
	seen := make(map[string]struct{})
	var merged []string
	for _, exclusions := range exclusionLists {
		for _, exclusion := range exclusions {
			if _, exists := seen[exclusion]; exists {
				continue
			}
			seen[exclusion] = struct{}{}
			merged = append(merged, exclusion)
		}
	}
	return merged
}

// ResolveExclusionOverride returns the override when it is set, otherwise it returns the inherited exclusions.
func ResolveExclusionOverride(inherited, override []string) []string {
	if override != nil {
		return slices.Clone(override)
	}
	return slices.Clone(inherited)
}

// BuildExclusionClause builds a SQL IN clause from a list of items.
func BuildExclusionClause(items []string) string {
	escaped := make([]string, len(items))
	for i, item := range items {
		escaped[i] = EscapeSQLString(item)
	}
	return "(" + strings.Join(escaped, ", ") + ")"
}

// EscapeSQLString escapes single quotes by doubling them to prevent SQL injection.
func EscapeSQLString(s string) string {
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}
