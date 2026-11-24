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

	return database_observability.BuildExclusionClause(
		database_observability.MergeExclusions(excludedSchemas, schemas),
	)
}

func buildSchemaFilterClause(excludeSchemas, includeSchemas []string, columnExpr string) string {
	return buildExcludedSchemasClause(excludeSchemas) + database_observability.BuildInclusionClause(columnExpr, includeSchemas)
}
