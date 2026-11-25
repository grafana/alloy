package collector

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-logfmt/logfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/util"
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"INSERT INTO `some_table` (`id`, `name`) VALUES (...)\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"UPDATE `some_table` SET `active` = false, `reason` = ? WHERE `id` = ? AND `name` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"DELETE FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT `t`.`id`, `t`.`val1`, `o`.`val2` FROM `some_table` `t` INNER JOIN `other_table` AS `o` ON `t`.`id` = `o`.`id` WHERE `o`.`val2` = ? ORDER BY `t`.`val1` DESC\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"other_table\",\"validated\":false},{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"false\" digest=\"xyz456\" digest_text=\"INSERT INTO `some_table`...\"",
				`level="info" schema="some_schema" digest="xyz456" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `another_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"another_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"false\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ? AND `name` =\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				`level="info" schema="some_schema" parseable="false" digest="xyz456" digest_text="not valid sql"`,
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
			},
		},
		{
			name: "truncated table-less statement with identical sample and digest text",
			eventStatementsRows: [][]driver.Value{{
				"xyz456",
				"SET @@session.sql_mode = ...",
				"some_schema",
				"SET @@session.sql_mode = ...",
			}},
			logsLabels: []model.LabelSet{
				{"op": database_observability.OP_QUERY_ASSOCIATION},
			},
			logsLines: []string{
				`level="info" schema="some_schema" parseable="false" digest="xyz456" digest_text="SET @@session.sql_mode = ..."`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
				"level=\"info\" schema=\"other_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="other_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM (SELECT `id`, `name` FROM `employees_us_east` UNION SELECT `id`, `name` FROM `employees_us_west`) AS `employees_us` UNION SELECT `id`, `name` FROM `employees_emea`\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"employees_emea\",\"validated\":false},{\"table\":\"employees_us_east\",\"validated\":false},{\"table\":\"employees_us_west\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SHOW CREATE TABLE `some_table`\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"false\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"false\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
			},
		},
		{
			name: "null query_sample_text uses digest_text",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM `some_table` WHERE `id` = ?",
				"some_schema",
				nil, // NULL query_sample_text
			}},
			logsLabels: []model.LabelSet{
				{"op": database_observability.OP_QUERY_ASSOCIATION},
				{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"",
				`level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`,
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
				Logger:          util.TestAlloyLogger(t).Slog(),
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
			Logger:          util.TestAlloyLogger(t).Slog(),
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
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_ASSOCIATION}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"", lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`, lokiEntries[1].Line)
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
			Logger:          util.TestAlloyLogger(t).Slog(),
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
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_ASSOCIATION}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"", lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`, lokiEntries[1].Line)
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
			Logger:          util.TestAlloyLogger(t).Slog(),
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
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_ASSOCIATION}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" parseable=\"true\" digest=\"abc123\" digest_text=\"SELECT * FROM `some_table` WHERE `id` = ?\"", lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"some_table\",\"validated\":false}]"`, lokiEntries[1].Line)
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
		Logger:          util.TestAlloyLogger(t).Slog(),
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

func TestQueryDetailsIncludeSchemas(t *testing.T) {
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
		IncludeSchemas:  []string{"included_schema"},
		EntryHandler:    lokiClient,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, buildSchemaFilterClause(nil, []string{"included_schema"}, "schema_name"), 250)).
		WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"digest", "digest_text", "schema_name", "query_sample_text",
	}))

	require.NoError(t, c.tablesFromEventsStatements(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryDetails_ResolvesTableNameCasingAndPreservesValidation(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()

	tableRegistry := NewTableRegistry()
	tableRegistry.SetTables([]*tableInfo{
		// e.g. information_schema.tables reports this lowercased because the cluster is
		// configured with lower_case_table_names=1, but the query below references it with
		// its literal, mixed-case spelling.
		{schema: "some_schema", tableName: "emailageconsumer"},
	})

	collector, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Second,
		StatementsLimit: 250,
		EntryHandler:    lokiClient,
		TableRegistry:   tableRegistry,
		Logger:          util.TestAlloyLogger(t).Slog(),
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
				"SELECT * FROM `EmailAgeConsumer` WHERE `id` = ?",
				"some_schema",
				"select * from EmailAgeConsumer where id = 1",
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

	require.NoError(t, mock.ExpectationsWereMet())

	lokiEntries := lokiClient.Received()
	require.Len(t, lokiEntries, 2)
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, lokiEntries[1].Labels)
	// Resolved to the registry's canonical (lowercased) casing rather than the query's
	// literal mixed-case spelling.
	require.Equal(t, `level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"emailageconsumer\",\"validated\":true}]"`, lokiEntries[1].Line)
}

func TestQueryDetails_MarksAnUnresolvedParsedNameAsNotValidated(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()

	// The registry only knows about a real table; it never contains whatever the query below
	// parses out as a "table" name, standing in for the SQL parser/tokenizer occasionally
	// misidentifying something else (a column, an alias, a function) as a table reference.
	tableRegistry := NewTableRegistry()
	tableRegistry.SetTables([]*tableInfo{
		{schema: "some_schema", tableName: "some_table"},
	})

	collector, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Second,
		StatementsLimit: 250,
		EntryHandler:    lokiClient,
		TableRegistry:   tableRegistry,
		Logger:          util.TestAlloyLogger(t).Slog(),
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
				"SELECT * FROM `not_a_real_table` WHERE `id` = ?",
				"some_schema",
				"select * from not_a_real_table where id = 1",
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

	require.NoError(t, mock.ExpectationsWereMet())

	lokiEntries := lokiClient.Received()
	require.Len(t, lokiEntries, 2)
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, lokiEntries[1].Labels)
	require.Equal(t, `level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"not_a_real_table\",\"validated\":false}]"`, lokiEntries[1].Line)
}

func TestQueryDetails_ResolvesASchemaQualifiedCrossDatabaseReference(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()

	// The query below runs against "some_schema" but references a table in a different
	// database ("other_schema"), casing-mismatched the same way an unqualified reference
	// can be.
	tableRegistry := NewTableRegistry()
	tableRegistry.SetTables([]*tableInfo{
		{schema: "other_schema", tableName: "emailageconsumer"},
	})

	collector, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Second,
		StatementsLimit: 250,
		EntryHandler:    lokiClient,
		TableRegistry:   tableRegistry,
		Logger:          util.TestAlloyLogger(t).Slog(),
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
				"SELECT * FROM `other_schema`.`EmailAgeConsumer` WHERE `id` = ?",
				"some_schema",
				"select * from other_schema.EmailAgeConsumer where id = 1",
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

	require.NoError(t, mock.ExpectationsWereMet())

	lokiEntries := lokiClient.Received()
	require.Len(t, lokiEntries, 2)
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, lokiEntries[1].Labels)
	// The "schema" field stays the connection's active schema ("some_schema"); the resolved
	// table carries its own schema qualifier ("other_schema"), lowercased to match the registry.
	require.Equal(t, `level="info" schema="some_schema" digest="abc123" tables="[{\"table\":\"other_schema.emailageconsumer\",\"validated\":true}]"`, lokiEntries[1].Line)
}

func TestQueryDetails_EscapesQuotedIdentifiers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector, err := NewQueryDetails(QueryDetailsArguments{
		DB: db, StatementsLimit: 250, EntryHandler: handler, Logger: util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)
	schema := "example\"schema"
	table := "orders\"archive"
	query := "SELECT * FROM `" + table + "`"
	mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"}).
			AddRow("abc123", query, schema, query))
	require.NoError(t, collector.tablesFromEventsStatements(t.Context()))
	require.Eventually(t, func() bool { return len(handler.Received()) == 2 }, time.Second, time.Millisecond)
	entries := handler.Received()
	for _, entry := range entries {
		decoder := logfmt.NewDecoder(strings.NewReader(entry.Line))
		fields := map[string]string{}
		for decoder.ScanRecord() {
			for decoder.ScanKeyval() {
				fields[string(decoder.Key())] = string(decoder.Value())
			}
		}
		require.NoError(t, decoder.Err())
		require.Equal(t, schema, fields["schema"])
		if entry.Labels["op"] == database_observability.OP_QUERY_ASSOCIATION {
			require.Equal(t, query, fields["digest_text"])
		} else {
			require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, entry.Labels)
			var tables []database_observability.ParsedTable
			require.NoError(t, json.Unmarshal([]byte(fields["tables"]), &tables))
			require.Equal(t, []database_observability.ParsedTable{{Table: table}}, tables)
		}
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTryTokenizeTableNames(t *testing.T) {
	c, err := NewQueryDetails(QueryDetailsArguments{
		Logger: util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	t.Run("no tables with identical fallback returns nil without error", func(t *testing.T) {
		tables, err := c.tryTokenizeTableNames("SET @@session.sql_mode = ...", "SET @@session.sql_mode = ...")
		require.NoError(t, err)
		require.Nil(t, tables)
	})

	t.Run("falls back to digest text when sample text yields no tables", func(t *testing.T) {
		tables, err := c.tryTokenizeTableNames("", "SELECT * FROM `some_table` WHERE `id` = ?")
		require.NoError(t, err)
		require.Equal(t, []string{"some_table"}, tables)
	})
}

func TestQueryDetails_ThrottlesWithinEmitInterval(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Minute,
		StatementsLimit: 250,
		EntryHandler:    lokiClient,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	base := time.Now()
	c.now = func() time.Time { return base }
	key := queryDetailsKey{schema: "shop", digest: "abc123"}
	query := fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)
	poll := func() {
		t.Helper()
		mock.ExpectQuery(query).WillReturnRows(
			sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"}).
				AddRow("abc123", "SELECT * FROM books", "shop", "SELECT * FROM books"))
		require.NoError(t, c.tablesFromEventsStatements(t.Context()))
	}

	// First cycle emits the association + parsed table (2 entries).
	poll()
	require.Equal(t, base, c.lastEmittedAt[key])

	// Second cycle within EmitInterval: the query still runs but nothing new is emitted.
	c.now = func() time.Time { return base.Add(database_observability.EmitInterval / 2) }
	poll()
	require.Equal(t, base, c.lastEmittedAt[key])

	// Once EmitInterval has elapsed, the query is emitted again (2 more entries).
	afterInterval := base.Add(database_observability.EmitInterval + time.Minute)
	c.now = func() time.Time { return afterInterval }
	poll()
	require.Equal(t, afterInterval, c.lastEmittedAt[key])

	require.NoError(t, mock.ExpectationsWereMet())
	lokiClient.Stop()
	require.Len(t, lokiClient.Received(), 4)
}

func TestQueryDetails_RetriesWhenSchemaRegistryIsNotReady(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	tableRegistry := NewTableRegistry()
	c, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Minute,
		StatementsLimit: 250,
		EntryHandler:    lokiClient,
		TableRegistry:   tableRegistry,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	base := time.Now()
	c.now = func() time.Time { return base }
	key := queryDetailsKey{schema: "shop", digest: "abc123"}
	query := fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)
	poll := func() {
		t.Helper()
		mock.ExpectQuery(query).WillReturnRows(
			sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"}).
				AddRow("abc123", "SELECT * FROM books", "shop", "SELECT * FROM books"))
		require.NoError(t, c.tablesFromEventsStatements(t.Context()))
	}

	// Poll 1: schema is not yet in tableRegistry; emit immediately with validated="false"
	// and mark the query as pending validation.
	poll()
	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) == 2
	}, 5*time.Second, 20*time.Millisecond)
	require.Equal(t, base, c.lastEmittedAt[key])
	require.Contains(t, c.unvalidated, key)

	entries := lokiClient.Received()
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_ASSOCIATION}, entries[0].Labels)
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, entries[1].Labels)
	require.Equal(t, `level="info" schema="shop" digest="abc123" tables="[{\"table\":\"books\",\"validated\":false}]"`, entries[1].Line)

	// Poll 2: still within EmitInterval, and schema metadata is still absent;
	// throttled so it does not re-emit on every cycle.
	c.now = func() time.Time { return base.Add(time.Minute) }
	poll()
	require.Equal(t, base, c.lastEmittedAt[key])
	require.Contains(t, c.unvalidated, key)

	// Poll 3: within EmitInterval, once this schema's metadata arrives,
	// bypass EmitInterval once to re-emit with validated="true".
	tableRegistry.SetTables([]*tableInfo{{schema: "shop", tableName: "books"}})
	readyAt := base.Add(2 * time.Minute)
	c.now = func() time.Time { return readyAt }
	poll()
	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) == 4
	}, 5*time.Second, 20*time.Millisecond)
	require.Equal(t, readyAt, c.lastEmittedAt[key])
	require.NotContains(t, c.unvalidated, key)

	entries = lokiClient.Received()
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_ASSOCIATION}, entries[2].Labels)
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, entries[3].Labels)
	require.Equal(t, `level="info" schema="shop" digest="abc123" tables="[{\"table\":\"books\",\"validated\":true}]"`, entries[3].Line)

	// Poll 4: within EmitInterval and already validated; throttled normally.
	c.now = func() time.Time { return base.Add(4 * time.Minute) }
	poll()
	require.Equal(t, readyAt, c.lastEmittedAt[key])

	require.NoError(t, mock.ExpectationsWereMet())
	lokiClient.Stop()
	require.Len(t, lokiClient.Received(), 4)
}

func TestQueryDetails_Throttling(t *testing.T) {
	t.Run("same digest is throttled independently per schema", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		t.Cleanup(func() {
			mock.ExpectClose()
			require.NoError(t, db.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		logs := loki.NewCollectingHandler()
		t.Cleanup(logs.Stop)
		c, err := NewQueryDetails(QueryDetailsArguments{
			DB: db, StatementsLimit: 250, EntryHandler: logs, Logger: util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		startTime := time.Now()
		poll := func(elapsed time.Duration, schemas ...string) {
			t.Helper()
			c.now = func() time.Time { return startTime.Add(elapsed) }
			rows := sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"})
			for _, schema := range schemas {
				rows.AddRow("abc123", "SELECT * FROM books", schema, "SELECT * FROM books")
			}
			mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WillReturnRows(rows)
			require.NoError(t, c.tablesFromEventsStatements(t.Context()))
		}

		poll(0, "shop")
		poll(time.Minute, "shop", "other_shop") // assertion below shows "shop" query is throttled

		logs.Stop()
		entries := logs.Received()
		require.Len(t, entries, 4)
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_ASSOCIATION}, entries[0].Labels)
		require.Contains(t, entries[0].Line, `schema="shop"`)
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, entries[1].Labels)
		require.Contains(t, entries[1].Line, `schema="shop"`)

		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_ASSOCIATION}, entries[2].Labels)
		require.Contains(t, entries[2].Line, `schema="other_shop"`)
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2}, entries[3].Labels)
		require.Contains(t, entries[3].Line, `schema="other_shop"`)
	})

	t.Run("query is throttled for EmitInterval time period", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		t.Cleanup(func() {
			mock.ExpectClose()
			require.NoError(t, db.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		logs := loki.NewCollectingHandler()
		t.Cleanup(logs.Stop)
		tableRegistry := NewTableRegistry()
		c, err := NewQueryDetails(QueryDetailsArguments{
			DB: db, StatementsLimit: 250, EntryHandler: logs, TableRegistry: tableRegistry, Logger: util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		startTime := time.Now()
		key := queryDetailsKey{schema: "shop", digest: "abc123"}
		poll := func(elapsed time.Duration, includeQuery bool) {
			t.Helper()
			c.now = func() time.Time { return startTime.Add(elapsed) }
			rows := sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"})
			if includeQuery {
				rows.AddRow("abc123", "SELECT * FROM books", "shop", "SELECT * FROM books")
			}
			mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WillReturnRows(rows)
			require.NoError(t, c.tablesFromEventsStatements(t.Context()))
		}

		// Poll 1: successful scan finds query, emits it, and records it for throttling
		poll(0, true)
		require.Equal(t, startTime, c.lastEmittedAt[key])

		// Poll 2: successful scan, query no longer present
		poll(database_observability.EmitInterval/3, false)
		require.Equal(t, startTime, c.lastEmittedAt[key])

		// Poll 3: query is back before EmitInterval expires: stays throttled
		poll(2*database_observability.EmitInterval/3, true)
		require.Equal(t, startTime, c.lastEmittedAt[key])

		// Poll 4: query is present, EmitInterval has expired, query is emitted again
		poll(database_observability.EmitInterval+time.Second, true)
		require.Equal(t, startTime.Add(database_observability.EmitInterval+time.Second), c.lastEmittedAt[key])

		// One query emission created two entries; assertion of 4 confirms emission after Poll 1 and Poll 4
		logs.Stop()
		require.Len(t, logs.Received(), 4)
	})

	t.Run("pruneThrottle removes queries absent after EmitInterval throttle state", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		t.Cleanup(func() {
			mock.ExpectClose()
			require.NoError(t, db.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		logs := loki.NewCollectingHandler()
		t.Cleanup(logs.Stop)
		c, err := NewQueryDetails(QueryDetailsArguments{
			DB: db, StatementsLimit: 250, EntryHandler: logs, TableRegistry: NewTableRegistry(), Logger: util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		startTime := time.Now()
		key := queryDetailsKey{schema: "shop", digest: "abc123"}
		poll := func(elapsed time.Duration, includeQuery bool) {
			t.Helper()
			c.now = func() time.Time { return startTime.Add(elapsed) }
			rows := sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"})
			if includeQuery {
				rows.AddRow("abc123", "SELECT * FROM books", "shop", "SELECT * FROM books")
			}
			mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)).WillReturnRows(rows)
			require.NoError(t, c.tablesFromEventsStatements(t.Context()))
		}

		// Poll 1: query present but no schema metadata, so both throttle maps contain it
		poll(0, true)
		require.Contains(t, c.lastEmittedAt, key)
		require.Contains(t, c.unvalidated, key)

		// Poll 2: query absent, EmitInterval expired: both entries pruned
		poll(database_observability.EmitInterval+time.Second, false)
		require.NotContains(t, c.lastEmittedAt, key)
		require.NotContains(t, c.unvalidated, key)
	})

	t.Run("scan query error preserves throttling", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		t.Cleanup(func() {
			mock.ExpectClose()
			require.NoError(t, db.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		logs := loki.NewCollectingHandler()
		t.Cleanup(logs.Stop)
		c, err := NewQueryDetails(QueryDetailsArguments{
			DB: db, StatementsLimit: 250, EntryHandler: logs, Logger: util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		key := queryDetailsKey{schema: "shop", digest: "abc123"}
		startTime := time.Now()
		c.now = func() time.Time { return startTime }

		// Poll 1: successful scan finds query, emits it, and records it for throttling
		query := fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)
		rows := sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"}).AddRow("abc123", "SELECT * FROM books", "shop", "SELECT * FROM books")
		mock.ExpectQuery(query).WillReturnRows(rows)
		require.NoError(t, c.tablesFromEventsStatements(t.Context()))
		require.Contains(t, c.lastEmittedAt, key)
		require.Equal(t, startTime, c.lastEmittedAt[key])

		// Poll 2: scan fails; nothing is emitted and previous emission time is unchanged
		mock.ExpectQuery(query).WillReturnError(fmt.Errorf("query failed"))
		require.Error(t, c.tablesFromEventsStatements(t.Context()))
		require.Equal(t, startTime, c.lastEmittedAt[key])

		// Poll 3: successful scan finds the same query within EmitInterval, so it stays throttled
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"}).AddRow("abc123", "SELECT * FROM books", "shop", "SELECT * FROM books"))
		require.NoError(t, c.tablesFromEventsStatements(t.Context()))

		// One query emission created two entries; assertion of 2 confirms no re-emission occurred during poll 2 and poll 3
		logs.Stop()
		require.Len(t, logs.Received(), 2)
	})

	t.Run("iteration error preserves throttling", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		t.Cleanup(func() {
			mock.ExpectClose()
			require.NoError(t, db.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		logs := loki.NewCollectingHandler()
		t.Cleanup(logs.Stop)
		c, err := NewQueryDetails(QueryDetailsArguments{
			DB: db, StatementsLimit: 250, EntryHandler: logs, Logger: util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		key := queryDetailsKey{schema: "shop", digest: "abc123"}
		initialEmissionAt := time.Now()
		c.now = func() time.Time { return initialEmissionAt }
		query := fmt.Sprintf(selectQueryTablesSamples, exclusionClause, 250)
		rows := func() *sqlmock.Rows {
			return sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"}).AddRow("abc123", "SELECT * FROM books", "shop", "SELECT * FROM books")
		}

		// Poll 1: successful scan finds query, emits it, and records it for throttling
		mock.ExpectQuery(query).WillReturnRows(rows())
		require.NoError(t, c.tablesFromEventsStatements(t.Context()))
		require.Contains(t, c.lastEmittedAt, key)
		require.Equal(t, initialEmissionAt, c.lastEmittedAt[key])

		// Poll 2: row iteration fails; nothing is emitted and previous emission time is unchanged
		mock.ExpectQuery(query).WillReturnRows(rows().RowError(0, fmt.Errorf("iteration failed")))
		require.Error(t, c.tablesFromEventsStatements(t.Context()))
		require.Equal(t, initialEmissionAt, c.lastEmittedAt[key])

		// Poll 3: successful scan finds the same query within EmitInterval, so it stays throttled
		mock.ExpectQuery(query).WillReturnRows(rows())
		require.NoError(t, c.tablesFromEventsStatements(t.Context()))
		logs.Stop()
		// One query emission created two entries; assertion of 2 confirms no re-emission occurred during poll 2 and poll 3
		require.Len(t, logs.Received(), 2)
	})
}
