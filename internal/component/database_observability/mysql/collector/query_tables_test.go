package collector

import (
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

func TestQueryTables(t *testing.T) {
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
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table where id = 1",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "insert query",
			rows: [][]driver.Value{{
				"abc123",
				"INSERT INTO `some_table` (`id`, `name`) VALUES (...)",
				"some_schema",
				"insert into some_table (`id`, `name`) values (1, 'foo')",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "update query",
			rows: [][]driver.Value{{
				"abc123",
				"UPDATE `some_table` SET `active` = false, `reason` = ? WHERE `id` = ? AND `name` = ?",
				"some_schema",
				"update some_table set active=false, reason=null where id = 1 and name = 'foo'",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "delete query",
			rows: [][]driver.Value{{
				"abc123",
				"DELETE FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"delete from some_table where id = 1",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "join two tables",
			rows: [][]driver.Value{{
				"abc123",
				"SELECT `t`.`id`, `t`.`val1`, `o`.`val2` FROM `some_table` `t` INNER JOIN `other_table` AS `o` ON `t`.`id` = `o`.`id` WHERE `o`.`val2` = ? ORDER BY `t`.`val1` DESC",
				"some_schema",
				"select t.id, t.val1, o.val2 FROM some_table t inner join other_table as o on t.id = o.id where o.val2 = 1 order by t.val1 desc",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
				`schema="some_schema" digest="abc123" table="other_table"`,
			},
		},
		{
			name: "truncated query",
			rows: [][]driver.Value{{
				"xyz456",
				"INSERT INTO `some_table`...",
				"some_schema",
				"insert into some_table (`id1`, `id2`, `id3`, `id...",
			}, {
				"abc123",
				"SELECT * FROM `another_table` WHERE `id` = ?",
				"some_schema",
				"select * from another_table where id = 1",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="another_table"`,
			},
		},
		{
			name: "truncated in multi-line comment",
			rows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table where id = 1 /*traceparent='00-abc...",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "truncated with properly closed comment",
			rows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ? AND `name` =",
				"some_schema",
				"select * from some_table where id = 1 /* comment that's closed */ and name = 'test...",
			}},
			logsLabels: []model.LabelSet{},
			logsLines:  []string{},
		},
		{
			name: "start transaction",
			rows: [][]driver.Value{{
				"abc123",
				"START TRANSACTION",
				"some_schema",
				"START TRANSACTION",
			}},
			logsLabels: []model.LabelSet{},
			logsLines:  []string{},
		},
		{
			name: "sql parse error",
			rows: [][]driver.Value{{
				"xyz456",
				"not valid sql",
				"some_schema",
				"not valid sql",
			}, {
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table where id = 1",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "multiple schemas",
			rows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table where id = 1",
			}, {
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"other_schema",
				"select * from some_table where id = 1",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
				`schema="other_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "subquery and union",
			rows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM (SELECT `id`, `name` FROM `employees_us_east` UNION SELECT `id`, `name` FROM `employees_us_west`) AS `employees_us` UNION SELECT `id`, `name` FROM `employees_emea`",
				"some_schema",
				"SELECT * FROM (SELECT id, name FROM employees_us_east UNION SELECT id, name FROM employees_us_west) as employees_us UNION SELECT id, name FROM employees_emea",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="employees_us_east"`,
				`schema="some_schema" digest="abc123" table="employees_us_west"`,
				`schema="some_schema" digest="abc123" table="employees_emea"`,
			},
		},
		{
			name: "show create table (table name is not parsed)",
			rows: [][]driver.Value{{
				"abc123",
				"SHOW CREATE TABLE `some_table`",
				"some_schema",
				"SHOW CREATE TABLE some_table",
			}},
			logsLabels: []model.LabelSet{},
			logsLines:  []string{},
		},
		{
			name: "show variables",
			rows: [][]driver.Value{{
				"abc123",
				"SHOW VARIABLES LIKE ?",
				"some_schema",
				"SHOW VARIABLES LIKE 'version'",
			}},
			logsLabels: []model.LabelSet{},
			logsLines:  []string{},
		},
		{
			name: "query truncated with dots fallback to digest_text",
			rows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table whe...",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "query truncated without dots fallback to digest_text",
			rows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table where",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="abc123" table="some_table"`,
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

			collector, err := NewQueryTables(QueryTablesArguments{
				DB:              db,
				InstanceKey:     "mysql-db",
				CollectInterval: time.Second,
				EntryHandler:    lokiClient,
				Logger:          log.NewLogfmtLogger(os.Stderr),
			})
			require.NoError(t, err)
			require.NotNil(t, collector)

			mock.ExpectQuery(selectQueryTablesSamples).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"digest",
						"digest_text",
						"schema_name",
						"query_sample_text",
					}).AddRows(
						tc.rows...,
					),
				)

			err = collector.Start(t.Context())
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

func TestQueryTablesSQLDriverErrors(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("recoverable sql error in result set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQueryTables(QueryTablesArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectQueryTablesSamples).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest", // not enough columns
				}).AddRow(
					"abc123",
				))

		mock.ExpectQuery(selectQueryTablesSamples).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest",
					"digest_text",
					"schema_name",
					"query_sample_text",
				}).AddRow(
					"abc123",
					"SELECT * FROM `some_table` WHERE `id` = ?",
					"some_schema",
					"select * from some_table where id = 1",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[0].Line)
	})

	t.Run("result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQueryTables(QueryTablesArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectQueryTablesSamples).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest",
					"digest_text",
					"schema_name",
					"query_sample_text",
				}).AddRow(
					"abc123",
					"SELECT * FROM `some_table` WHERE `id` = ?",
					"some_schema",
					"select * from some_table where id = 1",
				).AddRow(
					"def456",
					"SELECT * FROM `another_table` WHERE `id` = ?",
					"another_schema",
					"select * from another_table where id = 2",
				).RowError(1, fmt.Errorf("rs error")), // error on second row
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[0].Line)
	})

	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQueryTables(QueryTablesArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectQueryTablesSamples).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(selectQueryTablesSamples).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest",
					"digest_text",
					"schema_name",
					"query_sample_text",
				}).AddRow(
					"abc123",
					"SELECT * FROM `some_table` WHERE `id` = ?",
					"some_schema",
					"select * from some_table where id = 1",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[0].Line)
	})
}
