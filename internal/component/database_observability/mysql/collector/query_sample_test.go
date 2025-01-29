package collector

import (
	"context"
	"database/sql/driver"
	"fmt"
	"os"
	"testing"
	"time"

	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/prometheus/common/model"
	"go.uber.org/goleak"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestQuerySample(t *testing.T) {
	defer goleak.VerifyNone(t)

	testcases := []struct {
		name string
		rows [][]driver.Value
		logs []string
	}{
		{
			name: "select query",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"select * from some_table where id = 1",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "insert query",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"insert into some_table (`id`, `name`) values (1, 'foo')",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="insert" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="insert into some_table(id, name) values (:redacted1, :redacted2)"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "update query",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"update some_table set active=false, reason=null where id = 1 and  name = 'foo'",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="update" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="update some_table set active = false, reason = null where id = :redacted1 and name = :redacted2"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "delete query",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"delete from some_table where id = 1",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="delete" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="delete from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "join two tables",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"select t.id, t.val1, o.val2 FROM some_table t inner join other_table as o on t.id = o.id where o.val2 = 1 order by t.val1 desc",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select t.id, t.val1, o.val2 from some_table as t join other_table as o on t.id = o.id where o.val2 = :redacted1 order by t.val1 desc"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="other_table"`,
			},
		},
		{
			name: "subquery",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				`select ifnull(schema_name, 'none') as schema_name, digest, count_star from
				(select * from performance_schema.events_statements_summary_by_digest where schema_name not in ('mysql', 'performance_schema', 'information_schema')
				and last_seen > date_sub(now(), interval 86400 second) order by last_seen desc)q
				group by q.schema_name, q.digest, q.count_star limit 100`,
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" ` +
					`query_sample_redacted="select ifnull(schema_name, :redacted1) as schema_name, digest, count_star from (select * from ` +
					`performance_schema.events_statements_summary_by_digest where schema_name not in ::redacted2 ` +
					`and last_seen > date_sub(now(), interval :redacted3 second) order by last_seen desc) as q ` +
					`group by q.schema_name, q.digest, q.count_star limit :redacted4"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="performance_schema.events_statements_summary_by_digest"`,
			},
		},
		{
			name: "with comment",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"select val1, /* val2,*/ val3 from some_table where id = 1",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select val1, val3 from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "truncated query",
			rows: [][]driver.Value{{
				"xyz456",
				"some_schema",
				"insert into some_table (`id1`, `id2`, `id3`, `id...",
				"2024-02-02T00:00:00.000Z",
				"2000",
			}, {
				"abc123",
				"some_schema",
				"select * from some_table where id = 1",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "truncated in multi-line comment",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"select * from some_table where id = 1 /*traceparent='00-abc...",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "start transaction",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"START TRANSACTION",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="begin"`,
			},
		},
		{
			name: "commit",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"COMMIT",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="commit"`,
			},
		},
		{
			name: "alter table",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"alter table some_table modify enumerable enum('val1', 'val2') not null",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="alter table some_table"`,
			},
		},
		{
			name: "sql parse error",
			rows: [][]driver.Value{{
				"xyz456",
				"some_schema",
				"not valid sql",
				"2024-02-02T00:00:00.000Z",
				"2000",
			}, {
				"abc123",
				"some_schema",
				"select * from some_table where id = 1",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "multiple schemas",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"select * from some_table where id = 1",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}, {
				"abc123",
				"other_schema",
				"select * from some_table where id = 1",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="other_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="other_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "union query",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"SELECT id, name FROM employees_ny UNION SELECT id, name FROM employees_ca UNION SELECT id, name FROM employees_tx",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select id, name from employees_ny union select id, name from employees_ca union select id, name from employees_tx"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_ny"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_ca"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_tx"`,
			},
		},
		{
			name: "from subquery with union",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"SELECT COUNT(DISTINCT t.role_id) AS roles, COUNT(DISTINCT r.id) AS fixed_roles FROM (SELECT role_id FROM user_role UNION ALL SELECT role_id FROM team_role) AS t LEFT JOIN (SELECT id FROM role WHERE name LIKE 'prefix%') AS r ON t.role_id = r.id",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select COUNT(distinct t.role_id) as roles, COUNT(distinct r.id) as fixed_roles from (select role_id from user_role union all select role_id from team_role) as t left join (select id from role where name like :redacted1) as r on t.role_id = r.id"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="user_role"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="team_role"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="role"`,
			},
		},
		{
			name: "subquery and union",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"SELECT * FROM (SELECT id, name FROM employees_us_east UNION SELECT id, name FROM employees_us_west) as employees_us UNION SELECT id, name FROM employees_emea",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from (select id, name from employees_us_east union select id, name from employees_us_west) as employees_us union select id, name from employees_emea"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_us_east"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_us_west"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_emea"`,
			},
		},
		{
			name: "insert with subquery and union",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"INSERT INTO customers (id, name) SELECT id, name FROM customers_us UNION SELECT id, name FROM customers_eu",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="insert" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="insert into customers(id, name) select id, name from customers_us union select id, name from customers_eu"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="customers"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="customers_us"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="customers_eu"`,
			},
		},
		{
			name: "join with subquery and union",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"SELECT * FROM departments dep JOIN (SELECT id, name FROM employees_us UNION SELECT id, name FROM employees_eu) employees ON dep.id = employees.id",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from departments as dep join (select id, name from employees_us union select id, name from employees_eu) as employees on dep.id = employees.id"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="departments"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_us"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_eu"`,
			},
		},
		{
			name: "insert with subquery and join",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"INSERT INTO some_table SELECT * FROM departments dep JOIN (SELECT id, name FROM employees_us UNION SELECT id, name FROM employees_eu) employees ON dep.id = employees.id",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logs: []string{
				`level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="insert" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="insert into some_table select * from departments as dep join (select id, name from employees_us union select id, name from employees_eu) as employees on dep.id = employees.id"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="departments"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_us"`,
				`level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="employees_eu"`,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			lokiClient := loki_fake.NewClient(func() {})

			collector, err := NewQuerySample(QuerySampleArguments{
				DB:              db,
				InstanceKey:     "mysql-db",
				CollectInterval: time.Minute,
				EntryHandler:    lokiClient,
				Logger:          log.NewLogfmtLogger(os.Stderr),
			})
			require.NoError(t, err)
			require.NotNil(t, collector)

			mock.ExpectQuery(selectQuerySamples).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"digest",
						"schema_name",
						"query_sample_text",
						"query_sample_seen",
						"query_sample_timer_wait",
					}).AddRows(
						tc.rows...,
					),
				)

			err = collector.Start(context.Background())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return len(lokiClient.Received()) == len(tc.logs)
			}, 5*time.Second, 100*time.Millisecond)

			collector.Stop()
			lokiClient.Stop()

			lokiEntries := lokiClient.Received()
			for i, entry := range lokiEntries {
				require.Equal(t, model.LabelSet{"job": database_observability.JobName}, entry.Labels)
				require.Equal(t, tc.logs[i], entry.Line)
			}

			err = mock.ExpectationsWereMet()
			require.NoError(t, err)
		})
	}
}

func TestQuerySampleSQLDriverErrors(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("unrecoverable sql error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			CollectInterval: time.Minute,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectQuerySamples).WithoutArgs().WillReturnError(driver.ErrBadConn)

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Equal(t, 0, len(lokiClient.Received()))

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})

	t.Run("recoverable sql error in result set", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		// Expect to loop twice, first time to fail, second time to succeed
		mock.ExpectQuery(selectQuerySamples).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest", // not enough columns
				}).AddRow(
					"abc123",
				))

		mock.ExpectQuery(selectQuerySamples).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest",
					"schema_name",
					"query_sample_text",
					"query_sample_seen",
					"query_sample_timer_wait",
				}).AddRow(
					"abc123",
					"some_schema",
					"select * from some_table where id = 1",
					"2024-01-01T00:00:00.000Z",
					"1000",
				),
			)

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		lokiEntries := lokiClient.Received()
		for _, entry := range lokiEntries {
			require.Equal(t, model.LabelSet{"job": database_observability.JobName}, entry.Labels)
		}

		require.Equal(t, `level=info msg="query samples fetched" op="query_sample" instance="mysql-db" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`, lokiEntries[0].Line)
		require.Equal(t, `level=info msg="table name parsed" op="query_parsed_table_name" instance="mysql-db" schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[1].Line)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})

	t.Run("result set iteration error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectQuerySamples).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest",
					"query_sample_text",
					"query_sample_seen",
					"query_sample_timer_wait",
				}).AddRow(
					"abc123",
					"SELECT 1",
					"2024-01-01",
					"1000",
				).RowError(0, fmt.Errorf("rs error")),
			)

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})
}
