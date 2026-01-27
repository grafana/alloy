package collector

import "github.com/grafana/alloy/internal/component/database_observability"

var defaultExcludedDatabases = []string{"azure_maintenance"}

var defaultExclusionClause = database_observability.BuildExclusionClause(defaultExcludedDatabases)

func buildExcludedDatabasesClause(excludedDatabases []string) string {
	if len(excludedDatabases) == 0 {
		return defaultExclusionClause
	}

	databases := make([]string, 0, len(defaultExcludedDatabases)+len(excludedDatabases))
	databases = append(databases, defaultExcludedDatabases...)
	databases = append(databases, excludedDatabases...)
	return database_observability.BuildExclusionClause(databases)
}
