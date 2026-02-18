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
