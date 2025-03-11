package parser_test

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
	"github.com/stretchr/testify/require"
)

func TestExtractTableNames(t *testing.T) {
	testcases := []struct {
		name   string
		sql    string
		tables []string
	}{
		{
			name:   "simple select",
			sql:    "select * from some_table",
			tables: []string{"some_table"},
		},
		{
			name:   "simple insert",
			sql:    "insert into some_table (`id`, `name`) values (1, 'foo')",
			tables: []string{"some_table"},
		},
		{
			name:   "simple update",
			sql:    "update some_table set active=false, reason=null where id = 1 and  name = 'foo'",
			tables: []string{"some_table"},
		},
		{
			name:   "simple delete",
			sql:    "delete from some_table where id = 1",
			tables: []string{"some_table"},
		},
		{
			name:   "select with join",
			sql:    "select t.id, t.val1, o.val2 FROM some_table t inner join other_table as o on t.id = o.id where o.val2 = 1 order by t.val1 desc",
			tables: []string{"some_table", "other_table"},
		},
		{
			name: "select with subquery",
			sql: `select ifnull(schema_name, 'none') as schema_name, digest, count_star from
				(select * from performance_schema.events_statements_summary_by_digest where schema_name not in ('mysql', 'performance_schema', 'information_schema')
				and last_seen > date_sub(now(), interval 86400 second) order by last_seen desc)q
				group by q.schema_name, q.digest, q.count_star limit 100`,
			tables: []string{"performance_schema.events_statements_summary_by_digest"},
		},
		{
			name:   "select with aggregate",
			sql:    "select count(*) from some_table group by id",
			tables: []string{"some_table"},
		},
		{
			name:   "select with comment",
			sql:    "select val1, /* val2,*/ val3 from some_table where id = 1",
			tables: []string{"some_table"},
		},
		{
			name:   "select with case statement",
			sql:    "select case when enabled then 'yes' else 'no' end as enabled from some_table where id = 1",
			tables: []string{"some_table"},
		},
		{
			name:   "parentheses in select",
			sql:    "select (select count(*) from some_table) as count from other_table",
			tables: []string{"some_table", "other_table"},
		},
		{
			name:   "start transaction",
			sql:    "START TRANSACTION",
			tables: nil,
		},
		{
			name:   "commit",
			sql:    "COMMIT",
			tables: nil,
		},
		{
			name:   "alter table",
			sql:    "alter table some_table modify enumerable enum('val1', 'val2') not null",
			tables: []string{"some_table"},
		},
		{
			name:   "simple union",
			sql:    "SELECT id, name FROM employees_ny UNION SELECT id, name FROM employees_ca UNION SELECT id, name FROM employees_tx",
			tables: []string{"employees_ny", "employees_ca", "employees_tx"},
		},
		{
			name:   "subquery with union",
			sql:    "SELECT COUNT(DISTINCT t.role_id) AS roles, COUNT(DISTINCT r.id) AS fixed_roles FROM (SELECT role_id FROM user_role UNION ALL SELECT role_id FROM team_role) AS t LEFT JOIN (SELECT id FROM role WHERE name LIKE 'prefix%') AS r ON t.role_id = r.id",
			tables: []string{"user_role", "team_role", "role"},
		},
		{
			name:   "subquery with union and alias",
			sql:    "SELECT * FROM (SELECT id, name FROM employees_us_east UNION SELECT id, name FROM employees_us_west) as employees_us UNION SELECT id, name FROM employees_emea",
			tables: []string{"employees_us_east", "employees_us_west", "employees_emea"},
		},
		{
			name:   "insert with subquery and union",
			sql:    "INSERT INTO customers (id, name) SELECT id, name FROM customers_us UNION SELECT id, name FROM customers_eu",
			tables: []string{"customers", "customers_us", "customers_eu"},
		},
		{
			name:   "join with subquery and union",
			sql:    "SELECT * FROM departments dep JOIN (SELECT id, name FROM employees_us UNION SELECT id, name FROM employees_eu) employees ON dep.id = employees.id",
			tables: []string{"departments", "employees_us", "employees_eu"},
		},
		{
			name:   "insert with subquery and join",
			sql:    "INSERT INTO some_table SELECT * FROM departments dep JOIN (SELECT id, name FROM employees_us UNION SELECT id, name FROM employees_eu) employees ON dep.id = employees.id",
			tables: []string{"some_table", "departments", "employees_us", "employees_eu"},
		},
		// TODO(cristian): currently unsupported
		// {
		// 	name:   "show create table",
		// 	sql:    "SHOW CREATE TABLE some_table",
		// 	tables: []string{"some_table"},
		// },
		{
			name:   "show variables",
			sql:    "SHOW VARIABLES LIKE 'version'",
			tables: nil,
		},
		{
			name:   "drop table",
			sql:    "DROP TABLE IF EXISTS some_table",
			tables: []string{"some_table"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name+"_xwbparser", func(t *testing.T) {
			p := parser.NewXwbSqlParser()
			stmt, err := p.Parse(tc.sql)
			require.NoError(t, err)

			got := p.ExtractTableNames(log.NewNopLogger(), "", stmt)
			require.ElementsMatch(t, tc.tables, got)
		})
		t.Run(tc.name+"_tidbparser", func(t *testing.T) {
			p := parser.NewTiDBSqlParser()
			stmt, err := p.Parse(tc.sql)
			require.NoError(t, err)

			got := p.ExtractTableNames(log.NewNopLogger(), "", stmt)
			require.ElementsMatch(t, tc.tables, got)
		})
	}
}
