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
		name       string
		rows       [][]driver.Value
		logsLabels []model.LabelSet
		logsLines  []string
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`,
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="insert" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="insert into some_table(id, name) values (:redacted1, :redacted2)"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`,
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="update" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="update some_table set active = false, reason = null where id = :redacted1 and name = :redacted2"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`,
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="delete" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="delete from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`,
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select t.id, t.val1, o.val2 from some_table as t join other_table as o on t.id = o.id where o.val2 = :redacted1 order by t.val1 desc"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="other_table"`,
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`,
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "truncated with properly closed comment",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"select * from some_table where id = 1 /* comment that's closed */ and name = 'test...",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logsLabels: []model.LabelSet{},
			logsLines:  []string{},
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="begin"`,
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`,
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`,
				`level=info msg="query samples fetched" schema="other_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`,
				`level=info msg="table name parsed" schema="other_schema" digest="abc123" table="some_table"`,
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
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from (select id, name from employees_us_east union select id, name from employees_us_west) as employees_us union select id, name from employees_emea"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="employees_us_east"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="employees_us_west"`,
				`level=info msg="table name parsed" schema="some_schema" digest="abc123" table="employees_emea"`,
			},
		},
		{
			name: "show create table (table name is not parsed)",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"SHOW CREATE TABLE some_table",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="show create table"`,
			},
		},
		{
			name: "show variables",
			rows: [][]driver.Value{{
				"abc123",
				"some_schema",
				"SHOW VARIABLES LIKE 'version'",
				"2024-01-01T00:00:00.000Z",
				"1000",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="show variables"`,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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
				return len(lokiClient.Received()) == len(tc.logsLines)
			}, 5*time.Second, 100*time.Millisecond)

			collector.Stop()
			lokiClient.Stop()

			require.Eventually(t, func() bool {
				return collector.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

			err = mock.ExpectationsWereMet()
			require.NoError(t, err)

			lokiEntries := lokiClient.Received()
			require.Equal(t, len(tc.logsLines), len(lokiEntries))
			for i, entry := range lokiEntries {
				require.Equal(t, tc.logsLabels[i], entry.Labels)
				require.Equal(t, tc.logsLines[i], entry.Line)
			}
		})
	}
}

func TestQuerySampleSQLDriverErrors(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("recoverable sql error in result set", func(t *testing.T) {
		t.Parallel()

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

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[1].Line)
	})

	t.Run("result set iteration error", func(t *testing.T) {
		t.Parallel()

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
				).AddRow(
					"def456",
					"another_schema",
					"select * from another_table where id = 2",
					"2024-01-01T00:00:00.000Z",
					"1000",
				).RowError(1, fmt.Errorf("rs error")), // error on second row
			)

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[1].Line)
	})

	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

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

		mock.ExpectQuery(selectQuerySamples).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

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

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="query samples fetched" schema="some_schema" digest="abc123" query_type="select" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_sample_redacted="select * from some_table where id = :redacted1"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level=info msg="table name parsed" schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[1].Line)
	})
}
