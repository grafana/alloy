package collector

import (
	"database/sql/driver"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestQueryTables(t *testing.T) {
	defer goleak.VerifyNone(t)

	testcases := []struct {
		name                string
		eventStatementsRows [][]driver.Value
		logsLabels          []model.LabelSet
		logsLines           []string
	}{
		{
			name: "select query",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM some_table WHERE id = $1",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM some_table WHERE id = $1\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
			},
		},
		{
			name: "insert query",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"INSERT INTO some_table (id, name) VALUES (...)",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"INSERT INTO some_table (id, name) VALUES (...)\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
			},
		},
		{
			name: "update query",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"UPDATE some_table SET active = false, reason = ? WHERE id = $1 AND name = $2",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"UPDATE some_table SET active = false, reason = ? WHERE id = $1 AND name = $2\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
			},
		},
		{
			name: "delete query",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"DELETE FROM some_table WHERE id = $1",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"DELETE FROM some_table WHERE id = $1\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
			},
		},
		{
			name: "join two tables",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT t.id, t.val1, o.val2 FROM some_table t INNER JOIN other_table AS o ON t.id = o.id WHERE o.val2 = $1 ORDER BY t.val1 DESC",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"SELECT t.id, t.val1, o.val2 FROM some_table t INNER JOIN other_table AS o ON t.id = o.id WHERE o.val2 = $1 ORDER BY t.val1 DESC\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
				`level="info" queryID="abc123" datName="some_database" table="other_table" engine="postgres"`,
			},
		},
		{
			name: "truncated query",
			eventStatementsRows: [][]driver.Value{{
				"xyz456",
				"INSERT INTO some_table...",
				"some_database",
			}, {
				"abc123",
				"SELECT * FROM another_table WHERE id = $1",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"xyz456\" query_text=\"INSERT INTO some_table...\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="xyz456" datName="some_database" table="some_table" engine="postgres"`,
				"level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM another_table WHERE id = $1\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="another_table" engine="postgres"`,
			},
		},
		{
			name: "truncated with properly closed comment",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM some_table WHERE id = $1 AND name =",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM some_table WHERE id = $1 AND name =\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
			},
		},
		{
			name: "start transaction",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"START TRANSACTION",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
			},
			logsLines: []string{
				`level="info" queryID="abc123" query_text="START TRANSACTION" datName="some_database" engine="postgres"`,
			},
		},
		{
			name: "sql parse error",
			eventStatementsRows: [][]driver.Value{{
				"xyz456",
				"not valid sql",
				"some_database",
			}, {
				"abc123",
				"SELECT * FROM some_table WHERE id = $1",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"xyz456\" query_text=\"not valid sql\" datName=\"some_database\" engine=\"postgres\"",
				"level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM some_table WHERE id = $1\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
			},
		},
		{
			name: "multiple schemas",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM some_table WHERE id = $1",
				"some_database",
			}, {
				"abc123",
				"SELECT * FROM some_table WHERE id = $1",
				"other_schema",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM some_table WHERE id = $1\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
				"level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM some_table WHERE id = $1\" datName=\"other_schema\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="other_schema" table="some_table" engine="postgres"`,
			},
		},
		{
			name: "subquery and union",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM (SELECT id, name FROM employees_us_east UNION SELECT id, name FROM employees_us_west) AS employees_us UNION SELECT id, name FROM employees_emea",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM (SELECT id, name FROM employees_us_east UNION SELECT id, name FROM employees_us_west) AS employees_us UNION SELECT id, name FROM employees_emea\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="employees_us_east" engine="postgres"`,
				`level="info" queryID="abc123" datName="some_database" table="employees_us_west" engine="postgres"`,
				`level="info" queryID="abc123" datName="some_database" table="employees_emea" engine="postgres"`,
			},
		},
		{
			name: "show create table",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SHOW CREATE TABLE some_table",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"SHOW CREATE TABLE some_table\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
			},
		},
		{
			name: "show variables",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SHOW VARIABLES LIKE $1",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"SHOW VARIABLES LIKE $1\" datName=\"some_database\" engine=\"postgres\"",
			},
		},
		{
			name: "query is truncated",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM some_table WHERE",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"},
			},
			logsLines: []string{
				"level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM some_table WHERE\" datName=\"some_database\" engine=\"postgres\"",
				`level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`,
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
				InstanceKey:     "postgres-db",
				CollectInterval: time.Second,
				EntryHandler:    lokiClient,
				Logger:          log.NewLogfmtLogger(os.Stderr),
			})
			require.NoError(t, err)
			require.NotNil(t, collector)

			mock.ExpectQuery(selectQueriesFromActivity).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"queryid",
						"query",
						"datname",
					}).AddRows(
						tc.eventStatementsRows...,
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
			InstanceKey:     "postgres-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectQueriesFromActivity).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"queryid", // not enough columns
				}).AddRow(
					"abc123",
				))

		mock.ExpectQuery(selectQueriesFromActivity).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"queryid",
					"query",
					"datname",
				}).AddRow(
					"abc123",
					"SELECT * FROM some_table WHERE id = ?",
					"some_database",
				),
			)

		err = collector.Start(t.Context())
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM some_table WHERE id = ?\" datName=\"some_database\" engine=\"postgres\"", lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`, lokiEntries[1].Line)
	})

	t.Run("result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQueryTables(QueryTablesArguments{
			DB:              db,
			InstanceKey:     "postgres-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectQueriesFromActivity).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"queryid",
					"query",
					"datname",
				}).AddRow(
					"abc123",
					"SELECT * FROM some_table WHERE id = ?",
					"some_database",
				).AddRow(
					"def456",
					"SELECT * FROM another_table WHERE id = ?",
					"another_schema",
				).RowError(1, fmt.Errorf("rs error")), // error on second row
			)

		err = collector.Start(t.Context())
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM some_table WHERE id = ?\" datName=\"some_database\" engine=\"postgres\"", lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`, lokiEntries[1].Line)
	})

	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQueryTables(QueryTablesArguments{
			DB:              db,
			InstanceKey:     "postgres-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectQueriesFromActivity).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(selectQueriesFromActivity).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"queryid",
					"query",
					"datname",
				}).AddRow(
					"abc123",
					"SELECT * FROM some_table WHERE id = ?",
					"some_database",
				),
			)

		err = collector.Start(t.Context())
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_ASSOCIATION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" queryID=\"abc123\" query_text=\"SELECT * FROM some_table WHERE id = ?\" datName=\"some_database\" engine=\"postgres\"", lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" queryID="abc123" datName="some_database" table="some_table" engine="postgres"`, lokiEntries[1].Line)
	})
}
