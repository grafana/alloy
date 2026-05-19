package database_observability

import (
	"slices"
	"strings"
)

var defaultExcludedSchemas = []string{
	"alloydbadmin",
	"alloydbmetadata",
	"azure_maintenance",
	"azure_sys",
	"cloudsqladmin",
	"rdsadmin",
}

var defaultExcludedUsers = []string{
	"azuresu",
	"cloudsqladmin",
	"db-o11y", // default recommended user
	"rdsadmin",
}

func DefaultExcludedDatabases() []string {
	return slices.Clone(defaultExcludedSchemas)
}

func DefaultExcludedSchemas() []string {
	return slices.Clone(defaultExcludedSchemas)
}

func DefaultExcludedUsers() []string {
	return slices.Clone(defaultExcludedUsers)
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
