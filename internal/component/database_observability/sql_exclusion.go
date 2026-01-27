package database_observability

import "strings"

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
