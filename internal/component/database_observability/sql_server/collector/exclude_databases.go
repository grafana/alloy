package collector

import "github.com/grafana/alloy/internal/component/database_observability"

var excludedDatabases = AlwaysExcludedDatabases()

var databasesExclusionClause = database_observability.BuildExclusionClause(excludedDatabases)

// AlwaysExcludedDatabases returns SQL Server system databases that must never
// be iterated: their catalogs do not correspond to user-managed schemas.
func AlwaysExcludedDatabases() []string {
	return []string{"master", "model", "msdb", "tempdb"}
}

func buildExcludedDatabasesClause(databases []string) string {
	if len(databases) == 0 {
		return databasesExclusionClause
	}

	return database_observability.BuildExclusionClause(
		database_observability.MergeExclusions(excludedDatabases, databases),
	)
}

func buildDatabaseFilterClause(excludeDatabases, includeDatabases []string, columnExpr string) string {
	return buildExcludedDatabasesClause(excludeDatabases) + database_observability.BuildInclusionClause(columnExpr, includeDatabases)
}
