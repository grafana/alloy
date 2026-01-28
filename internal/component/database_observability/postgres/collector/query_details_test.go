package collector

import (
	"database/sql/driver"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/DataDog/go-sqllexer"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestQueryDetails(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	testcases := []struct {
		name                string
		eventStatementsRows [][]driver.Value
		logsLabels          []model.LabelSet
		logsLines           []string
		tableRegistry       *TableRegistry
	}{
		{
			name: "select query",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM some_table WHERE id = $1",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="SELECT * FROM some_table WHERE id = $1" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="true"`,
			},
			tableRegistry: &TableRegistry{
				tables: map[database]map[schema]map[table]struct{}{
					"some_database": {
						"public": {
							"some_table": struct{}{},
						},
					},
				},
			},
		},
		{
			name: "select query with schema-qualified table",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"SELECT * FROM public.users WHERE id = $1",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="SELECT * FROM public.users WHERE id = $1" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="public.users" validated="true"`,
			},
			tableRegistry: &TableRegistry{
				tables: map[database]map[schema]map[table]struct{}{
					"some_database": {
						"public": {
							"users": struct{}{},
						},
					},
				},
			},
		},
		{
			name: "select query containing with",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"WITH some_with_table AS (SELECT * FROM some_table WHERE id = $1) SELECT * FROM some_with_table",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="WITH some_with_table AS (SELECT * FROM some_table WHERE id = $1) SELECT * FROM some_with_table" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="INSERT INTO some_table (id, name) VALUES (...)" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
			},
		},
		{
			name: "insert query containing with",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"WITH some_with_table AS (SELECT id, name FROM some_other_table WHERE id = $1) INSERT INTO some_table (id, name) SELECT id, name FROM some_with_table",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="WITH some_with_table AS (SELECT id, name FROM some_other_table WHERE id = $1) INSERT INTO some_table (id, name) SELECT id, name FROM some_with_table" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_other_table" validated="false"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="UPDATE some_table SET active = false, reason = ? WHERE id = $1 AND name = $2" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="DELETE FROM some_table WHERE id = $1" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
			},
		},
		{
			name: "delete query containing with",
			eventStatementsRows: [][]driver.Value{{
				"abc123",
				"WITH some_with_table AS (SELECT id, name FROM some_other_table WHERE id = $1) DELETE FROM some_table WHERE id IN (SELECT id FROM some_with_table)",
				"some_database",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="WITH some_with_table AS (SELECT id, name FROM some_other_table WHERE id = $1) DELETE FROM some_table WHERE id IN (SELECT id FROM some_with_table)" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_other_table" validated="false"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="SELECT t.id, t.val1, o.val2 FROM some_table t INNER JOIN other_table AS o ON t.id = o.id WHERE o.val2 = $1 ORDER BY t.val1 DESC" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
				`level="info" queryid="abc123" datname="some_database" table="other_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="xyz456" querytext="INSERT INTO some_table..." datname="some_database"`,
				`level="info" queryid="xyz456" datname="some_database" table="some_table" validated="false"`,
				`level="info" queryid="abc123" querytext="SELECT * FROM another_table WHERE id = $1" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="another_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="SELECT * FROM some_table WHERE id = $1 AND name =" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="START TRANSACTION" datname="some_database"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="xyz456" querytext="not valid sql" datname="some_database"`,
				`level="info" queryid="abc123" querytext="SELECT * FROM some_table WHERE id = $1" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="SELECT * FROM some_table WHERE id = $1" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
				`level="info" queryid="abc123" querytext="SELECT * FROM some_table WHERE id = $1" datname="other_schema"`,
				`level="info" queryid="abc123" datname="other_schema" table="some_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="SELECT * FROM (SELECT id, name FROM employees_us_east UNION SELECT id, name FROM employees_us_west) AS employees_us UNION SELECT id, name FROM employees_emea" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="employees_us_east" validated="false"`,
				`level="info" queryid="abc123" datname="some_database" table="employees_us_west" validated="false"`,
				`level="info" queryid="abc123" datname="some_database" table="employees_emea" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="SHOW CREATE TABLE some_table" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
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
				{"op": OP_QUERY_ASSOCIATION},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="SHOW VARIABLES LIKE $1" datname="some_database"`,
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
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="abc123" querytext="SELECT * FROM some_table WHERE" datname="some_database"`,
				`level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`,
			},
		},
		{
			name: "correctly escape quoted queries",
			eventStatementsRows: [][]driver.Value{
				{
					"3871016669222913500",
					`SELECT "pizza_to_ingredients"."pizza_id", "i"."id", "i"."name", "i"."calories_per_slice", "i"."vegetarian", "i"."type" FROM "ingredients" AS "i" JOIN "pizza_to_ingredients" AS "pizza_to_ingredients" ON ("pizza_to_ingredients"."pizza_id") IN ($1) WHERE ("i"."id" = "pizza_to_ingredients"."ingredient_id")`,
					"quickpizza",
				},
				{
					"7865322458849960000",
					`SELECT "quote"."name" FROM "quotes" AS "quote"`,
					"quickpizza",
				},
				{
					"5775615007769463000",
					`SELECT "classical_name"."name" FROM "classical_names" AS "classical_name"`,
					"quickpizza",
				},
				{
					"7007034463187741000",
					`SELECT "dough"."id", "dough"."name", "dough"."calories_per_slice" FROM "doughs" AS "dough"`,
					"quickpizza",
				},
			},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
				{"op": OP_QUERY_ASSOCIATION},
				{"op": OP_QUERY_PARSED_TABLE_NAME},
			},
			logsLines: []string{
				`level="info" queryid="3871016669222913500" querytext="SELECT \"pizza_to_ingredients\".\"pizza_id\", \"i\".\"id\", \"i\".\"name\", \"i\".\"calories_per_slice\", \"i\".\"vegetarian\", \"i\".\"type\" FROM \"ingredients\" AS \"i\" JOIN \"pizza_to_ingredients\" AS \"pizza_to_ingredients\" ON (\"pizza_to_ingredients\".\"pizza_id\") IN ($1) WHERE (\"i\".\"id\" = \"pizza_to_ingredients\".\"ingredient_id\")" datname="quickpizza"`,
				`level="info" queryid="3871016669222913500" datname="quickpizza" table="ingredients" validated="false"`,
				`level="info" queryid="3871016669222913500" datname="quickpizza" table="pizza_to_ingredients" validated="false"`,
				`level="info" queryid="7865322458849960000" querytext="SELECT \"quote\".\"name\" FROM \"quotes\" AS \"quote\"" datname="quickpizza"`,
				`level="info" queryid="7865322458849960000" datname="quickpizza" table="quotes" validated="false"`,
				`level="info" queryid="5775615007769463000" querytext="SELECT \"classical_name\".\"name\" FROM \"classical_names\" AS \"classical_name\"" datname="quickpizza"`,
				`level="info" queryid="5775615007769463000" datname="quickpizza" table="classical_names" validated="false"`,
				`level="info" queryid="7007034463187741000" querytext="SELECT \"dough\".\"id\", \"dough\".\"name\", \"dough\".\"calories_per_slice\" FROM \"doughs\" AS \"dough\"" datname="quickpizza"`,
				`level="info" queryid="7007034463187741000" datname="quickpizza" table="doughs" validated="false"`,
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
				EntryHandler:    lokiClient,
				TableRegistry:   tc.tableRegistry,
				Logger:          log.NewLogfmtLogger(os.Stderr),
			})
			require.NoError(t, err)
			require.NotNil(t, collector)

			mock.ExpectQuery(fmt.Sprintf(selectQueriesFromActivity, exclusionClause)).WithoutArgs().RowsWillBeClosed().
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

func TestQueryDetails_SQLDriverErrors(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("recoverable sql error in result set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQueryDetails(QueryDetailsArguments{
			DB:              db,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectQueriesFromActivity, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"queryid", // not enough columns
				}).AddRow(
					"abc123",
				))

		mock.ExpectQuery(fmt.Sprintf(selectQueriesFromActivity, exclusionClause)).WithoutArgs().RowsWillBeClosed().
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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_ASSOCIATION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" queryid="abc123" querytext="SELECT * FROM some_table WHERE id = ?" datname="some_database"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_PARSED_TABLE_NAME}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`, lokiEntries[1].Line)
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
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectQueriesFromActivity, exclusionClause)).WithoutArgs().RowsWillBeClosed().
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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_ASSOCIATION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" queryid="abc123" querytext="SELECT * FROM some_table WHERE id = ?" datname="some_database"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_PARSED_TABLE_NAME}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`, lokiEntries[1].Line)
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
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectQueriesFromActivity, exclusionClause)).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(fmt.Sprintf(selectQueriesFromActivity, exclusionClause)).WithoutArgs().RowsWillBeClosed().
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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_ASSOCIATION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" queryid="abc123" querytext="SELECT * FROM some_table WHERE id = ?" datname="some_database"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_PARSED_TABLE_NAME}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" queryid="abc123" datname="some_database" table="some_table" validated="false"`, lokiEntries[1].Line)
	})
}

func TestQueryDetails_TokenizeTableNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sql     string
		want    []string
		wantErr bool
	}{
		{
			name: "simple select",
			sql:  "SELECT * FROM users",
			want: []string{"users"},
		},
		{
			name: "select with join",
			sql:  "SELECT * FROM users u JOIN orders o ON u.id = o.user_id",
			want: []string{"orders", "users"},
		},
		{
			name: "select with schema qualified tables",
			sql:  "SELECT * FROM public.users JOIN sales.orders ON users.id = orders.user_id",
			want: []string{"public.users", "sales.orders"},
		},
		{
			name: "insert statement",
			sql:  "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')",
			want: []string{"users"},
		},
		{
			name: "update statement",
			sql:  "UPDATE users SET last_login = NOW() WHERE id = 1",
			want: []string{"users"},
		},
		{
			name: "delete statement",
			sql:  "DELETE FROM users WHERE id = 1",
			want: []string{"users"},
		},
		{
			name: "with clause",
			sql: `WITH active_users AS (
				SELECT * FROM users WHERE status = 'active'
			)
			SELECT * FROM active_users au
			JOIN orders o ON o.user_id = au.id`,
			want: []string{"orders", "users"},
		},
		{
			name: "subquery in where clause",
			sql: `SELECT * FROM orders
				WHERE user_id IN (SELECT id FROM users WHERE status = 'active')`,
			want: []string{"orders", "users"},
		},
		{
			name: "multiple schema qualified tables with aliases",
			sql: `SELECT u.name, o.total, p.status
				FROM public.users u
				JOIN sales.orders o ON u.id = o.user_id
				LEFT JOIN shipping.packages p ON o.id = p.order_id`,
			want: []string{"public.users", "sales.orders", "shipping.packages"},
		},
		{
			name: "truncated query with ...",
			sql:  "SELECT * FROM users JOIN orders ON users.id = orders.user_id AND...",
			want: []string{"users", "orders"},
		},
		{
			name: "truncated query with incomplete comment",
			sql:  "SELECT * FROM users JOIN orders ON users.id = orders.user_id /* some comment that gets truncated...",
			want: []string{"users", "orders"},
		},
		{
			name: "truncated query mid-table name",
			sql:  "SELECT * FROM users JOIN ord...",
			want: []string{"users", "ord"},
		},
		{
			name: "truncated query with schema qualified tables",
			sql:  "SELECT * FROM public.users JOIN sales.orders ON users.id = orders.user_id AND...",
			want: []string{"public.users", "sales.orders"},
		},
		{
			name: "query with table.* expression",
			sql:  "SELECT u.*, o.* FROM users u JOIN orders o ON u.id = o.user_id",
			want: []string{"users", "orders"},
		},
		{
			name: "query with type cast",
			sql:  "SELECT u.id, '2024-03-20'::timestamp FROM users u",
			want: []string{"users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenizeTableNames(sqllexer.NewNormalizer(sqllexer.WithCollectTables(true)), tt.sql)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, got, tt.want)
		})
	}
}

func TestQueryDetails_RemoveComments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sql     string
		want    string
		wantErr bool
	}{
		{
			name: "simple select",
			sql:  "SELECT * FROM users",
			want: "SELECT * FROM users",
		},
		{
			name: "inline comment",
			sql:  "SELECT * FROM users -- getting all users",
			want: "SELECT * FROM users",
		},
		{
			name: "block comment",
			sql:  "SELECT * FROM /* important table */ users",
			want: "SELECT * FROM  users",
		},
		{
			name: "multiple comments",
			sql:  "SELECT /* cols */ * FROM users -- table",
			want: "SELECT  * FROM users",
		},
		{
			name: "comment in string literal preserved",
			sql:  "SELECT ' -- not a comment ' FROM users",
			want: "SELECT ' -- not a comment ' FROM users",
		},
		{
			name: "multiline block comment",
			sql:  "SELECT * FROM users /* \n multiline \n comment */ WHERE id = 1",
			want: "SELECT * FROM users  WHERE id = 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := removeComments(sqllexer.NewNormalizer(sqllexer.WithCollectComments(true)), tt.sql)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestQueryDetails_ExcludeDatabases(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	collector, err := NewQueryDetails(QueryDetailsArguments{
		DB:               db,
		CollectInterval:  time.Second,
		ExcludeDatabases: []string{"excluded_database"},
		EntryHandler:     lokiClient,
		Logger:           log.NewLogfmtLogger(os.Stderr),
	})
	require.NoError(t, err)
	require.NotNil(t, collector)

	mock.ExpectQuery(fmt.Sprintf(selectQueriesFromActivity, buildExcludedDatabasesClause([]string{"excluded_database"}))).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{
				"queryid",
				"query",
				"datname",
			}).AddRow(
				"def456",
				"SELECT * FROM orders",
				"another_database",
			),
		)

	err = collector.Start(t.Context())
	require.NoError(t, err)

	// Only the another_database should have logs emitted
	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) >= 2 // query_association + query_parsed_table_name
	}, 5*time.Second, 100*time.Millisecond)

	collector.Stop()

	require.Eventually(t, func() bool {
		return collector.Stopped()
	}, 5*time.Second, 100*time.Millisecond)

	require.NoError(t, mock.ExpectationsWereMet())

	// Verify only another_database logs were emitted
	for _, entry := range lokiClient.Received() {
		require.Contains(t, entry.Line, "another_database", "included database should appear in logs")
	}
}
