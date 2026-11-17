package collector

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/database_observability"
)

func buildExcludedUsersClause(users []string, columnExpr string) string {
	if len(users) == 0 {
		return ""
	}
	return fmt.Sprintf("AND %s NOT IN %s", columnExpr, database_observability.BuildExclusionClause(users))
}

func buildUserFilterClause(excludeUsers, includeUsers []string, columnExpr string) string {
	clause := buildExcludedUsersClause(excludeUsers, columnExpr)
	if len(includeUsers) > 0 {
		inClause := fmt.Sprintf("AND %s IN %s", columnExpr, database_observability.BuildExclusionClause(includeUsers))
		if clause != "" {
			clause += " " + inClause
		} else {
			clause = inClause
		}
	}
	return clause
}
