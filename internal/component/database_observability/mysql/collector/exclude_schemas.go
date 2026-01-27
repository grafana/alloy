package collector

import "github.com/grafana/alloy/internal/component/database_observability"

var defaultExcludedSchemas = []string{"mysql", "performance_schema", "sys", "information_schema"}

var defaultExclusionClause = database_observability.BuildExclusionClause(defaultExcludedSchemas)

func buildExcludedSchemasClause(excludedSchemas []string) string {
	if len(excludedSchemas) == 0 {
		return defaultExclusionClause
	}

	schemas := make([]string, 0, len(defaultExcludedSchemas)+len(excludedSchemas))
	schemas = append(schemas, defaultExcludedSchemas...)
	schemas = append(schemas, excludedSchemas...)
	return database_observability.BuildExclusionClause(schemas)
}
