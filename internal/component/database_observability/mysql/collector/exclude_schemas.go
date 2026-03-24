package collector

import "github.com/grafana/alloy/internal/component/database_observability"

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
