package collector

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
)

func TestSchemaDetails(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("detect table schema", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_schema",
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_schema",
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"TABLE_NAME",
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"some_table",
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				).AddRow(
					"some_table",
					"category",
					"null",
					"NO",
					"int",
					"",
					"",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"some_table",
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}).AddRow(
					"some_table",
					"fk_name",
					"category",
					"categories",
					"id",
				),
			)

		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"create_statement",
				}).AddRow(
					"some_schema.some_table",
					"CREATE TABLE some_table (id INT, category INT)",
				),
			)

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT, category INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"},{"name":"category","type":"int","not_null":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}],"foreign_keys":[{"name":"fk_name","column_name":"category","referenced_table_name":"categories","referenced_column_name":"id"}]}`))

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[1].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[1].Line)
	})
	t.Run("detect table schema, index with expression", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_schema",
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_schema",
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"TABLE_NAME",
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"some_table",
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				).AddRow(
					"some_table",
					"category",
					"null",
					"NO",
					"int",
					"",
					"",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"some_table",
					"idx_category",
					1,
					"category",
					nil,
					"",
					0,
					"BTREE",
				).AddRow(
					"some_table",
					"idx_category",
					2,
					nil,
					"category = 0",
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"create_statement",
				}).AddRow(
					"some_schema.some_table",
					"CREATE TABLE some_table (id INT, category INT)",
				),
			)

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT, category INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"},{"name":"category","type":"int","not_null":true,"default_value":"null"}],"indexes":[{"name":"idx_category","type":"BTREE","columns":["category"],"expressions":["category = 0"],"unique":true,"nullable":false}]}`))

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[1].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[1].Line)
	})
	t.Run("detect table schema, index with multiple columns", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_schema",
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_schema",
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"TABLE_NAME",
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"some_table",
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				).AddRow(
					"some_table",
					"category",
					"null",
					"NO",
					"int",
					"",
					"",
				).AddRow(
					"some_table",
					"name",
					"null",
					"YES",
					"varchar(255)",
					"",
					"",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"some_table",
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				).AddRow(
					"some_table",
					"idx_name",
					1,
					"name",
					nil,
					"",
					0,
					"BTREE",
				).AddRow(
					"some_table",
					"idx_name",
					2,
					nil,
					"name = 'test'",
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}).AddRow(
					"some_table",
					"fk_name",
					"category",
					"categories",
					"id",
				),
			)

		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"create_statement",
				}).AddRow(
					"some_schema.some_table",
					"CREATE TABLE some_table (id INT, category INT, name VARCHAR(255))",
				),
			)

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT, category INT, name VARCHAR(255))"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"},{"name":"category","type":"int","not_null":true,"default_value":"null"},{"name":"name","type":"varchar(255)","default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false},{"name":"idx_name","type":"BTREE","columns":["name"],"expressions":["name = 'test'"],"unique":true,"nullable":false}],"foreign_keys":[{"name":"fk_name","column_name":"category","referenced_table_name":"categories","referenced_column_name":"id"}]}`))

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[1].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[1].Line)
	})
	t.Run("second scrape within emit_interval emits OP_TABLE_DETECTION but not OP_CREATE_STATEMENT", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		fakeNow := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour, // unused; extractSchema is invoked manually
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)
		collector.now = func() time.Time { return fakeNow }

		// First scrape: tables list, bulk metadata, then SHOW CREATE TABLE.
		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_schema", "table_name", "table_type", "create_time", "update_time",
			}).AddRow(
				"some_schema", "some_table", "BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			))

		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_NAME", "COLUMN_NAME", "COLUMN_DEFAULT", "IS_NULLABLE",
				"COLUMN_TYPE", "COLUMN_KEY", "EXTRA",
			}).AddRow("some_table", "id", "null", "NO", "int", "PRI", "auto_increment"))

		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name", "expression",
				"nullable", "non_unique", "index_type",
			}).AddRow("some_table", "PRIMARY", 1, "id", nil, "", 0, "BTREE"))

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"table_name", "create_statement"}).
				AddRow("some_schema.some_table", "CREATE TABLE some_table (id INT)"))

		// Second scrape: only the tables list. The scrape is throttled (still
		// within EmitInterval) so it must not trigger any create-statement
		// related queries.
		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_schema", "table_name", "table_type", "create_time", "update_time",
			}).AddRow(
				"some_schema", "some_table", "BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			))

		require.NoError(t, collector.extractSchema(t.Context()))
		fakeNow = fakeNow.Add(time.Minute) // well within EmitInterval
		require.NoError(t, collector.extractSchema(t.Context()))

		// First scrape emits OP_TABLE_DETECTION + OP_CREATE_STATEMENT; second
		// scrape emits only OP_TABLE_DETECTION (still emitted on every scrape).
		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}]}`))
		expectedCreateLine := fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec)

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[1].Labels)
		require.Equal(t, expectedCreateLine, entries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[2].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, entries[2].Line)
	})
	t.Run("second scrape after emit_interval re-emits OP_CREATE_STATEMENT", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		fakeNow := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)
		collector.now = func() time.Time { return fakeNow }

		// Two scrapes' worth of expectations: tables list + bulk metadata +
		// SHOW CREATE each time.
		for i := 0; i < 2; i++ {
			mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"table_schema", "table_name", "table_type", "create_time", "update_time",
				}).AddRow(
					"some_schema", "some_table", "BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				))

			mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"TABLE_NAME", "COLUMN_NAME", "COLUMN_DEFAULT", "IS_NULLABLE",
					"COLUMN_TYPE", "COLUMN_KEY", "EXTRA",
				}).AddRow("some_table", "id", "null", "NO", "int", "PRI", "auto_increment"))

			mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"table_name", "index_name", "seq_in_index", "column_name", "expression",
					"nullable", "non_unique", "index_type",
				}).AddRow("some_table", "PRIMARY", 1, "id", nil, "", 0, "BTREE"))

			mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"table_name", "constraint_name", "column_name",
					"referenced_table_name", "referenced_column_name",
				}))

			mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{"table_name", "create_statement"}).
					AddRow("some_schema.some_table", "CREATE TABLE some_table (id INT)"))
		}

		require.NoError(t, collector.extractSchema(t.Context()))
		fakeNow = fakeNow.Add(emitInterval + time.Minute) // past the throttle window
		require.NoError(t, collector.extractSchema(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 4
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}]}`))
		expectedTableDetectionLine := `level="info" schema="some_schema" table="some_table"`
		expectedCreateLine := fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec)

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, expectedTableDetectionLine, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[1].Labels)
		require.Equal(t, expectedCreateLine, entries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[2].Labels)
		require.Equal(t, expectedTableDetectionLine, entries[2].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[3].Labels)
		require.Equal(t, expectedCreateLine, entries[3].Line)
	})
	t.Run("table dropped between scrapes is removed from throttle map", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		fakeNow := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)
		collector.now = func() time.Time { return fakeNow }

		// First scrape: two tables in the same schema.
		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_schema", "table_name", "table_type", "create_time", "update_time",
			}).AddRow(
				"some_schema", "table_a", "BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			).AddRow(
				"some_schema", "table_b", "BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			))

		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"table_a", "table_b"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_NAME", "COLUMN_NAME", "COLUMN_DEFAULT", "IS_NULLABLE",
				"COLUMN_TYPE", "COLUMN_KEY", "EXTRA",
			}).
				AddRow("table_a", "id", "null", "NO", "int", "PRI", "auto_increment").
				AddRow("table_b", "id", "null", "NO", "int", "PRI", "auto_increment"))

		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"table_a", "table_b"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name", "expression",
				"nullable", "non_unique", "index_type",
			}).
				AddRow("table_a", "PRIMARY", 1, "id", nil, "", 0, "BTREE").
				AddRow("table_b", "PRIMARY", 1, "id", nil, "", 0, "BTREE"))

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"table_a", "table_b"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

		for _, table := range []string{"table_a", "table_b"} {
			mock.ExpectQuery(fmt.Sprintf("SHOW CREATE TABLE `some_schema`.`%s`", table)).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{"table_name", "create_statement"}).
					AddRow("some_schema."+table, fmt.Sprintf("CREATE TABLE %s (id INT)", table)))
		}

		require.NoError(t, collector.extractSchema(t.Context()))
		require.Contains(t, collector.lastEmittedAt, fullyQualifiedName("some_schema", "table_a"))
		require.Contains(t, collector.lastEmittedAt, fullyQualifiedName("some_schema", "table_b"))

		// Second scrape: only table_a remains. table_b should be evicted from
		// the throttle map by housekeeping. Since table_a was already emitted
		// less than EmitInterval ago, no further fetch queries are expected.
		fakeNow = fakeNow.Add(time.Minute)
		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_schema", "table_name", "table_type", "create_time", "update_time",
			}).AddRow(
				"some_schema", "table_a", "BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			))

		require.NoError(t, collector.extractSchema(t.Context()))

		require.NoError(t, mock.ExpectationsWereMet())
		require.Contains(t, collector.lastEmittedAt, fullyQualifiedName("some_schema", "table_a"))
		require.NotContains(t, collector.lastEmittedAt, fullyQualifiedName("some_schema", "table_b"))
	})
	t.Run("detect view schema", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})

		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_schema",
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_schema",
					"some_table",
					"VIEW",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"TABLE_NAME",
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"some_table",
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"some_table",
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"create_statement",
					"character_set_client",
					"collation_connection",
				}).AddRow(
					"some_schema.some_table",
					"CREATE VIEW some_view (id INT)",
					"some_charset",
					"some_charset_connection",
				),
			)

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE VIEW some_view (id INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}]}`))

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[1].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[1].Line)
	})
	t.Run("detect tables across multiple schemas in one bulk query", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_schema",
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"schema_a",
					"table_a",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				).AddRow(
					"schema_b",
					"table_b",
					"BASE TABLE",
					time.Date(2024, 3, 3, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 4, 4, 0, 0, 0, 0, time.UTC),
				),
			)

		// Per-table follow-ups for both schemas.
		for _, tc := range []struct{ schema, table, ddl string }{
			{"schema_a", "table_a", "CREATE TABLE table_a (id INT)"},
			{"schema_b", "table_b", "CREATE TABLE table_b (id INT)"},
		} {
			mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{tc.table}))).WithArgs(tc.schema).RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"TABLE_NAME", "COLUMN_NAME", "COLUMN_DEFAULT", "IS_NULLABLE",
						"COLUMN_TYPE", "COLUMN_KEY", "EXTRA",
					}).AddRow(tc.table, "id", "null", "NO", "int", "PRI", "auto_increment"),
				)

			mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{tc.table}))).WithArgs(tc.schema).RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"table_name", "index_name", "seq_in_index", "column_name", "expression",
						"nullable", "non_unique", "index_type",
					}).AddRow(tc.table, "PRIMARY", 1, "id", nil, "", 0, "BTREE"),
				)

			mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{tc.table}))).WithArgs(tc.schema).RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"table_name", "constraint_name", "column_name",
					"referenced_table_name", "referenced_column_name",
				}))

			mock.ExpectQuery("SHOW CREATE TABLE " + fullyQualifiedName(tc.schema, tc.table)).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{"table_name", "create_statement"}).
						AddRow(tc.schema+"."+tc.table, tc.ddl),
				)
		}

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 4)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="schema_a" table="table_a"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="schema_b" table="table_b"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Contains(t, lokiEntries[2].Line, `schema="schema_a" table="table_a"`)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[3].Labels)
		require.Contains(t, lokiEntries[3].Line, `schema="schema_b" table="table_b"`)
	})
	t.Run("no tables detected", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"table_schema",
				"table_name",
				"table_type",
				"create_time",
				"update_time",
			}),
		)

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		require.Empty(t, lokiClient.Received())
	})
	t.Run("empty tables list clears throttle map", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		// Pre-populate the throttle map to simulate a table that was emitted
		// in a previous scrape but no longer exists in information_schema.
		collector.lastEmittedAt[fullyQualifiedName("some_schema", "stale_table")] = time.Now()

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_schema", "table_name", "table_type", "create_time", "update_time",
			}))

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())
		require.Empty(t, collector.lastEmittedAt)
	})
	t.Run("tables result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"table_schema",
				"table_name",
				"table_type",
				"create_time",
				"update_time",
			}).AddRow(
				"some_schema",
				"some_table",
				"BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			).AddRow(
				"some_schema",
				"another_table",
				"BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			).RowError(1, fmt.Errorf("rs error")), // error on the second row
		)

		err = collector.extractSchema(t.Context())
		require.ErrorContains(t, err, "failed to iterate over tables result set")
		require.ErrorContains(t, err, "rs error")
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 1)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[0].Line)
	})
	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"table_schema",
				"table_name",
				"table_type",
				"create_time",
				"update_time",
			}).AddRow(
				"some_schema",
				"some_table",
				"BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			),
		)

		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"TABLE_NAME",
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"some_table",
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"some_table",
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"table_name",
				"create_statement",
			}).AddRow(
				"some_schema.some_table",
				"CREATE TABLE some_table (id INT)",
			),
		)

		// First scrape: the tables query fails. The collector logs the error
		// and the loop would normally retry on the next tick.
		require.ErrorContains(t, collector.extractSchema(t.Context()), "connection error")
		// Second scrape: connection recovers and the table is processed end to end.
		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}]}`))

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[1].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[1].Line)
	})
	t.Run("bulk metadata returns rows for multiple tables in one schema", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_schema", "table_name", "table_type",
					"create_time", "update_time",
				}).AddRow(
					"some_schema", "table_a", "BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				).AddRow(
					"some_schema", "table_b", "BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		// Single per-schema bulk fetches returning rows for both tables.
		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"table_a", "table_b"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_NAME", "COLUMN_NAME", "COLUMN_DEFAULT", "IS_NULLABLE",
				"COLUMN_TYPE", "COLUMN_KEY", "EXTRA",
			}).
				AddRow("table_a", "id", "null", "NO", "int", "PRI", "auto_increment").
				AddRow("table_b", "id", "null", "NO", "int", "PRI", "auto_increment"))

		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"table_a", "table_b"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name", "expression",
				"nullable", "non_unique", "index_type",
			}).
				AddRow("table_a", "PRIMARY", 1, "id", nil, "", 0, "BTREE").
				AddRow("table_b", "PRIMARY", 1, "id", nil, "", 0, "BTREE"))

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"table_a", "table_b"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

		for _, table := range []string{"table_a", "table_b"} {
			mock.ExpectQuery(fmt.Sprintf("SHOW CREATE TABLE `some_schema`.`%s`", table)).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{"table_name", "create_statement"}).
						AddRow("some_schema."+table, fmt.Sprintf("CREATE TABLE %s (id INT)", table)),
				)
		}

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		lokiEntries := lokiClient.Received()
		// Two table-detection entries plus two create-statement entries; order
		// within entry types follows the bulk-tables result order.
		require.Len(t, lokiEntries, 4)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="table_a"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="table_b"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Contains(t, lokiEntries[2].Line, `schema="some_schema" table="table_a"`)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[3].Labels)
		require.Contains(t, lokiEntries[3].Line, `schema="some_schema" table="table_b"`)
	})
	t.Run("bulk metadata returns no rows for a table", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_schema", "table_name", "table_type",
					"create_time", "update_time",
				}).AddRow(
					"some_schema", "some_table", "BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		// All three per-schema queries return empty result sets, simulating a
		// table that was dropped between the tables query and the metadata
		// queries. The table is skipped — only OP_TABLE_DETECTION is emitted
		// and no OP_CREATE_STATEMENT follows.
		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_NAME", "COLUMN_NAME", "COLUMN_DEFAULT", "IS_NULLABLE",
				"COLUMN_TYPE", "COLUMN_KEY", "EXTRA",
			}))
		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name", "expression",
				"nullable", "non_unique", "index_type",
			}))
		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{"table_name", "create_statement"}).
					AddRow("some_schema.some_table", "CREATE TABLE some_table (id INT)"),
			)

		// No OP_CREATE_STATEMENT means no loki-side sync point; drive
		// extractSchema synchronously to avoid racing the collector loop
		// against sqlmock state.
		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 1)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[0].Line)
	})
	t.Run("extractSchema table spec column defaults and index branches", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_schema", "table_name", "table_type", "create_time", "update_time",
			}).AddRow(
				"some_schema", "some_table", "BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			))

		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_NAME", "COLUMN_NAME", "COLUMN_DEFAULT", "IS_NULLABLE",
				"COLUMN_TYPE", "COLUMN_KEY", "EXTRA",
			}).
				AddRow("some_table", "updated_at", "CURRENT_TIMESTAMP", "NO", "timestamp", "", "on update current_timestamp").
				AddRow("some_table", "name", nil, "YES", "varchar(255)", "", ""))

		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name", "expression",
				"nullable", "non_unique", "index_type",
			}).
				AddRow("some_table", "idx_composite", 1, "col_a", nil, "", 0, "BTREE").
				AddRow("some_table", "idx_composite", 2, "col_b", nil, "", 0, "BTREE").
				AddRow("some_table", "idx_expr_only", 1, nil, "lower(name)", "", 0, "BTREE").
				AddRow("some_table", "idx_dup", 1, "name", nil, "YES", 1, "BTREE"))

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}).AddRow("some_table", "fk_name", "name", "other_table", "id"))

		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"table_name", "create_statement"}).
				AddRow("some_schema.some_table", "CREATE TABLE some_table (updated_at TIMESTAMP, name VARCHAR(255))"))

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[1].Labels)

		_, after, ok := strings.Cut(lokiEntries[1].Line, `table_spec="`)
		require.True(t, ok)
		specB64, _, ok := strings.Cut(after, `"`)
		require.True(t, ok)
		specJSON, err := base64.StdEncoding.DecodeString(specB64)
		require.NoError(t, err)

		var spec tableSpec
		require.NoError(t, json.Unmarshal(specJSON, &spec))

		require.Len(t, spec.Columns, 2)
		require.Equal(t, "updated_at", spec.Columns[0].Name)
		require.Equal(t, "CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP", spec.Columns[0].DefaultValue)
		require.True(t, spec.Columns[0].NotNull)
		require.Equal(t, "name", spec.Columns[1].Name)
		require.Equal(t, "", spec.Columns[1].DefaultValue)
		require.False(t, spec.Columns[1].NotNull)

		require.Len(t, spec.Indexes, 3)

		var idxComposite, idxExprOnly, idxDup *indexSpec
		for i := range spec.Indexes {
			switch spec.Indexes[i].Name {
			case "idx_composite":
				idxComposite = &spec.Indexes[i]
			case "idx_expr_only":
				idxExprOnly = &spec.Indexes[i]
			case "idx_dup":
				idxDup = &spec.Indexes[i]
			}
		}
		require.NotNil(t, idxComposite)
		require.Equal(t, []string{"col_a", "col_b"}, idxComposite.Columns)
		require.Empty(t, idxComposite.Expressions)
		require.True(t, idxComposite.Unique)

		require.NotNil(t, idxExprOnly)
		require.Empty(t, idxExprOnly.Columns)
		require.Equal(t, []string{"lower(name)"}, idxExprOnly.Expressions)

		require.NotNil(t, idxDup)
		require.Equal(t, []string{"name"}, idxDup.Columns)
		require.False(t, idxDup.Unique)
		require.True(t, idxDup.Nullable)

		require.Len(t, spec.ForeignKeys, 1)
		require.Equal(t, "fk_name", spec.ForeignKeys[0].Name)
	})
	t.Run("Start and Stop drive a single scrape and clean up", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_schema", "table_name", "table_type", "create_time", "update_time",
			}).AddRow(
				"some_schema", "some_table", "BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			))
		mock.ExpectQuery(fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_NAME", "COLUMN_NAME", "COLUMN_DEFAULT", "IS_NULLABLE",
				"COLUMN_TYPE", "COLUMN_KEY", "EXTRA",
			}).AddRow("some_table", "id", "null", "NO", "int", "PRI", "auto_increment"))
		mock.ExpectQuery(fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name", "expression",
				"nullable", "non_unique", "index_type",
			}).AddRow("some_table", "PRIMARY", 1, "id", nil, "", 0, "BTREE"))
		mock.ExpectQuery(fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))
		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"table_name", "create_statement"}).
				AddRow("some_schema.some_table", "CREATE TABLE some_table (id INT)"))

		require.NoError(t, collector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 5*time.Second, 10*time.Millisecond)

		collector.Stop()
		require.True(t, collector.Stopped())

		lokiClient.Stop()
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSchemaDetailsExcludeSchemas(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewSchemaDetails(SchemaDetailsArguments{
		DB:              db,
		CollectInterval: time.Millisecond,
		ExcludeSchemas:  []string{"excluded_schema"},
		EntryHandler:    lokiClient,
		Logger:          log.NewLogfmtLogger(os.Stderr),
	})
	require.NoError(t, err)

	// Verify the query uses the custom exclusion clause
	mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, buildExcludedSchemasClause([]string{"excluded_schema"}))).
		WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"table_schema",
		"table_name",
		"table_type",
		"create_time",
		"update_time",
	}))

	c.extractSchema(t.Context())
	require.NoError(t, mock.ExpectationsWereMet())
}
