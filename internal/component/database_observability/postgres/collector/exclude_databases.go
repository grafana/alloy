package collector

import "github.com/grafana/alloy/internal/component/database_observability"

var excludedDatabases = []string{"azure_maintenance"}

var exclusionClause = database_observability.BuildExclusionClause(excludedDatabases)

func buildExcludedDatabasesClause(databases []string) string {
	if len(databases) == 0 {
		return exclusionClause
	}

	all := make([]string, 0, len(excludedDatabases)+len(databases))
	all = append(all, excludedDatabases...)
	all = append(all, databases...)
	return database_observability.BuildExclusionClause(all)
}
