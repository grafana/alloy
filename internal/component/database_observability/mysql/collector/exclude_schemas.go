package collector

import "strings"

var defaultExcludedSchemas = []string{"mysql", "performance_schema", "sys", "information_schema"}

var defaultExclusionClause = buildExclusionClause(defaultExcludedSchemas)

func buildExcludedSchemasClause(excludedSchemas []string) string {
	if len(excludedSchemas) == 0 {
		return defaultExclusionClause
	}

	allSchemas := make([]string, 0, len(defaultExcludedSchemas)+len(excludedSchemas))
	allSchemas = append(allSchemas, defaultExcludedSchemas...)
	allSchemas = append(allSchemas, excludedSchemas...)

	return buildExclusionClause(allSchemas)
}

func buildExclusionClause(schemas []string) string {
	escaped := make([]string, len(schemas))
	for i, schema := range schemas {
		escaped[i] = escapeSQLString(schema)
	}
	return "(" + strings.Join(escaped, ", ") + ")"
}

// escapeSQLString escapes single quotes by doubling them to prevent SQL injection.
func escapeSQLString(s string) string {
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}
