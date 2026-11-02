package collector

import "github.com/grafana/alloy/internal/component/database_observability"

var excludedSchemas = AlwaysExcludedSchemas()

var exclusionClause = database_observability.BuildExclusionClause(excludedSchemas)

// AlwaysExcludedSchemas returns schemas that collectors must never query.
func AlwaysExcludedSchemas() []string {
	return []string{"mysql", "performance_schema", "sys", "information_schema"}
}

func buildExcludedSchemasClause(schemas []string) string {
	if len(schemas) == 0 {
		return exclusionClause
	}

	all := make([]string, 0, len(excludedSchemas)+len(schemas))
	all = append(all, excludedSchemas...)
	all = append(all, schemas...)
	return database_observability.BuildExclusionClause(all)
}
