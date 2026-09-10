package collector

import (
	"strings"

	"github.com/grafana/alloy/internal/component/database_observability"
)

var excludedSchemas = []string{"mysql", "performance_schema", "sys", "information_schema"}

var exclusionClause = database_observability.BuildExclusionClause(excludedSchemas)

func buildExcludedSchemasClause(schemas []string) string {
	if len(schemas) == 0 {
		return exclusionClause
	}

	all := make([]string, 0, len(excludedSchemas)+len(schemas))
	all = append(all, excludedSchemas...)
	all = append(all, schemas...)
	return database_observability.BuildExclusionClause(all)
}

// excludedSchemasArgs returns the combined default + custom excluded schemas
// as query arguments, for binding as `?` placeholders instead of formatting
// them into the SQL text.
func excludedSchemasArgs(schemas []string) []any {
	args := make([]any, 0, len(excludedSchemas)+len(schemas))
	for _, s := range excludedSchemas {
		args = append(args, s)
	}
	for _, s := range schemas {
		args = append(args, s)
	}
	return args
}

// sqlPlaceholders returns a comma-separated list of n "?" placeholders, for
// building a parameterized `IN (...)` / `NOT IN (...)` clause whose argument
// count is dynamic.
func sqlPlaceholders(n int) string {
	placeholders := make([]string, n)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}
