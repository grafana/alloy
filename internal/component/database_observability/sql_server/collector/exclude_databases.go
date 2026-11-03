package collector

import "github.com/grafana/alloy/internal/component/database_observability"

// excludedDatabases lists SQL Server system databases that must never be
// iterated: their catalogs do not correspond to user-managed schemas.
var excludedDatabases = []string{"master", "model", "msdb", "tempdb"}

var databasesExclusionClause = database_observability.BuildExclusionClause(excludedDatabases)

func buildExcludedDatabasesClause(databases []string) string {
	if len(databases) == 0 {
		return databasesExclusionClause
	}

	return database_observability.BuildExclusionClause(
		database_observability.MergeExclusions(excludedDatabases, databases),
	)
}
