package collector

import "github.com/grafana/alloy/internal/component/database_observability"

var excludedSchemas = []string{"sys", "information_schema"}

var exclusionClause = database_observability.BuildExclusionClause(excludedSchemas)

func buildExcludedSchemasClause(schemas []string) string {
	if len(schemas) == 0 {
		return exclusionClause
	}

	return database_observability.BuildExclusionClause(
		database_observability.MergeExclusions(excludedSchemas, schemas),
	)
}
