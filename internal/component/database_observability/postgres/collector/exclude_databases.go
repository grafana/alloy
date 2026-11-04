package collector

import "github.com/grafana/alloy/internal/component/database_observability"

var excludedDatabases = AlwaysExcludedDatabases()

var exclusionClause = database_observability.BuildExclusionClause(excludedDatabases)

// AlwaysExcludedDatabases returns databases that collectors must never query.
func AlwaysExcludedDatabases() []string {
	return []string{
		"azure_maintenance", // Azure Database for PostgreSQL exposes this database monitoring users cannot read its pgavailability schema
	}
}

func buildExcludedDatabasesClause(databases []string) string {
	if len(databases) == 0 {
		return exclusionClause
	}

	return database_observability.BuildExclusionClause(
		database_observability.MergeExclusions(excludedDatabases, databases),
	)
}

func buildDatabaseFilterClause(excludeDatabases, includeDatabases []string, columnExpr string) string {
	return buildExcludedDatabasesClause(excludeDatabases) + database_observability.BuildInclusionClause(columnExpr, includeDatabases)
}
