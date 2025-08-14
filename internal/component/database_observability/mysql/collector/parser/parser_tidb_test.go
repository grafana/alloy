package parser_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
)

func TestParserTiDB_ExtractTableNames(t *testing.T) {
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
		{
			name:   "show create table",
			sql:    "SHOW CREATE TABLE some_table",
			tables: []string{"some_table"},
		},
		{
			name:   "create user with password",
			sql:    "CREATE USER 'exporter'@'%' IDENTIFIED BY <secret>",
			tables: nil,
		},
		{
			name:   "insert with redacted values",
			sql:    "INSERT INTO some_table(id, url) VALUES (...)",
			tables: []string{"some_table"},
		},
		{
			name:   "trim function (sql mode ignore case)",
			sql:    "SELECT TRIM (TRAILING '/' FROM url)",
			tables: nil,
		},
		{
			name:   "if with redacted values",
			sql:    "SELECT IF(`some_table`.`url` IS NULL, ?, ...) AS `url` FROM `some_table`",
			tables: []string{"some_table"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			p := parser.NewTiDBSqlParser()
			stmt, err := p.Parse(tc.sql)
			require.NoError(t, err)

			got := p.ExtractTableNames(stmt)
			require.ElementsMatch(t, tc.tables, got)
		})
	}
}

func TestParserTiDB_CleanTruncatedText(t *testing.T) {
	testcases := []struct {
		name    string
		sql     string
		want    string
		wantErr error
	}{
		{
			name:    "simple select",
			sql:     "select * from some_table where id = 1",
			want:    "select * from some_table where id = 1",
			wantErr: nil,
		},

		{
			name:    "truncated query",
			sql:     "insert into some_table (`id1`, `id2`, `id3`, `id...",
			want:    "insert into some_table (`id1`, `id2`, `id3`, `id...",
			wantErr: fmt.Errorf("sql text is truncated"),
		},
		{
			name:    "truncated in multi-line comment",
			sql:     "select * from some_table where id = 1 /*traceparent='00-abc...",
			want:    "select * from some_table where id = 1",
			wantErr: nil,
		},
		{
			name:    "truncated with properly closed comment",
			sql:     "select * from some_table where id = 1 /* comment that's closed */ and name = 'test...",
			want:    "select * from some_table where id = 1 /* comment that's closed */ and name = 'test...",
			wantErr: fmt.Errorf("sql text is truncated after a comment"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			p := parser.NewTiDBSqlParser()
			got, err := p.CleanTruncatedText(tc.sql)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}
