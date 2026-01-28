package collector

import (
	"database/sql/driver"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
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
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table where id = 1",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "insert query",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"INSERT INTO `some_table` (`id`, `name`) VALUES (...)",
				"some_schema",
				"insert into some_table (`id`, `name`) values (1, 'foo')",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"INSERT INTO `some_table` (`id`, `name`) VALUES (...)\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "update query",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"UPDATE `some_table` SET `active` = false, `reason` = ? WHERE `id` = ? AND `name` = ?",
				"some_schema",
				"update some_table set active=false, reason=null where id = 1 and name = 'foo'",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"UPDATE `some_table` SET `active` = false, `reason` = ? WHERE `id` = ? AND `name` = ?\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "delete query",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"DELETE FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"delete from some_table where id = 1",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"DELETE FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "join two tables",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT `t`.`id`, `t`.`val1`, `o`.`val2` FROM `some_table` `t` INNER JOIN `other_table` AS `o` ON `t`.`id` = `o`.`id` WHERE `o`.`val2` = ? ORDER BY `t`.`val1` DESC",
				"some_schema",
				"select t.id, t.val1, o.val2 FROM some_table t inner join other_table as o on t.id = o.id where o.val2 = 1 order by t.val1 desc",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT `t`.`id`, `t`.`val1`, `o`.`val2` FROM `some_table` `t` INNER JOIN `other_table` AS `o` ON `t`.`id` = `o`.`id` WHERE `o`.`val2` = ? ORDER BY `t`.`val1` DESC\"",
				`level="info" schema="some_schema" digest="abc123" table="other_table"`,
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "truncated query",
			eventStatementsRows: [][]driver.Value{{
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"false\" digest=\"xyz456\" digest_text=\"INSERT INTO `some_table`...\"",
				`level="info" schema="some_schema" digest="xyz456" table="some_table"`,
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `another_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" table="another_table"`,
			},
		},
		{
			name: "truncated in multi-line comment",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table where id = 1 /*traceparent='00-abc...",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "truncated with properly closed comment",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ? AND `name` =",
				"some_schema",
				"select * from some_table where id = 1 /* comment that's closed */ and name = 'test...",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"false\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ? AND `name` =\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "start transaction",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"START TRANSACTION",
				"some_schema",
				"START TRANSACTION",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
			},
			logsLines: []string{
				`level="info" schema="some_schema" parseable="true" digest="abc123" digest_text="START TRANSACTION"`,
			},
		},
		{
			name: "sql parse error",
			eventStatementsRows: [][]driver.Value{{
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "multiple schemas",
			eventStatementsRows: [][]driver.Value{{
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
				"level=\"info\" schema=\"other_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="other_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "subquery and union",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM (SELECT `id`, `name` FROM `employees_us_east` UNION SELECT `id`, `name` FROM `employees_us_west`) AS `employees_us` UNION SELECT `id`, `name` FROM `employees_emea`",
				"some_schema",
				"SELECT * FROM (SELECT id, name FROM employees_us_east UNION SELECT id, name FROM employees_us_west) as employees_us UNION SELECT id, name FROM employees_emea",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM (SELECT `id`, `name` FROM `employees_us_east` UNION SELECT `id`, `name` FROM `employees_us_west`) AS `employees_us` UNION SELECT `id`, `name` FROM `employees_emea`\"",
				`level="info" schema="some_schema" digest="abc123" table="employees_emea"`,
				`level="info" schema="some_schema" digest="abc123" table="employees_us_east"`,
				`level="info" schema="some_schema" digest="abc123" table="employees_us_west"`,
			},
		},
		{
			name: "show create table",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SHOW CREATE TABLE `some_table`",
				"some_schema",
				"SHOW CREATE TABLE some_table",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SHOW CREATE TABLE `some_table`\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "show variables",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SHOW VARIABLES LIKE ?",
				"some_schema",
				"SHOW VARIABLES LIKE 'version'",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
			},
			logsLines: []string{
				`level="info" schema="some_schema" parseable="true" digest="abc123" digest_text="SHOW VARIABLES LIKE ?"`,
			},
		},
		{
			name: "query truncated with dots fallback to digest_text",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table whe...",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "query truncated without dots fallback to digest_text",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				"select * from some_table where",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "both query and fallback query are truncated",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE",
				"some_schema",
				"select * from some_table where",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"false\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
		{
			name: "query truncated within table name fallback to digest_text",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE",
				"some_schema",
				"select * from `s...",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"false\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE\"",
				`level="info" schema="some_schema" digest="abc123" table="some_table"`,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			lokiClient := loki.NewCollectingHandler()

			collector, err := NewQueryDetails(QueryDetailsArguments{
				DB:              db,
				CollectInterval: time.Second,
				StatementsLimit: 250,
				EntryHandler:    lokiClient,
				Logger:          log.NewLogfmtLogger(os.Stderr),
			})
			require.NoError(t, err)
			require.NotNil(t, collector)

			mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"digest",
						"digest_text",
						"schema_name",
						"query_sample_text",
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

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQueryDetails(QueryDetailsArguments{
			DB:              db,
			CollectInterval: time.Second,
			StatementsLimit: 250,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest", // not enough columns
				}).AddRow(
					"abc123",
				))

		mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WithoutArgs().RowsWillBeClosed().
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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_ASSOCIATION}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"", lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_PARSED_TABLE_NAME}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[1].Line)
	})

	t.Run("result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQueryDetails(QueryDetailsArguments{
			DB:              db,
			CollectInterval: time.Second,
			StatementsLimit: 250,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WithoutArgs().RowsWillBeClosed().
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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_ASSOCIATION}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"", lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_PARSED_TABLE_NAME}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[1].Line)
	})

	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQueryDetails(QueryDetailsArguments{
			DB:              db,
			CollectInterval: time.Second,
			StatementsLimit: 250,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WithoutArgs().RowsWillBeClosed().
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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_ASSOCIATION}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"", lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_PARSED_TABLE_NAME}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" digest="abc123" table="some_table"`, lokiEntries[1].Line)
	})
}

func TestQueryDetailsExcludeSchemas(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Millisecond,
		StatementsLimit: 250,
		ExcludeSchemas:  []string{"excluded_schema"},
		EntryHandler:    lokiClient,
		Logger:          log.NewLogfmtLogger(os.Stderr),
	})
	require.NoError(t, err)

	// Verify the query uses the custom exclusion clause
	mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, buildExcludedSchemasClause([]string{"excluded_schema"}), 250)).
		WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"digest", "digest_text", "schema_name", "query_sample_text",
	}))

	c.tablesFromEventsStatements(t.Context())
	require.NoError(t, mock.ExpectationsWereMet())
}
