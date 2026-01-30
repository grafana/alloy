package collector

import (
	"encoding/base64"
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

func TestSchemaDetails(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("detect table schema", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
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

		mock.ExpectQuery(selectColumnNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				).AddRow(
					"category",
					"null",
					"NO",
					"int",
					"",
					"",
				),
			)

		mock.ExpectQuery(selectIndexNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}).AddRow(
					"fk_name",
					"category",
					"categories",
					"id",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT, category INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"},{"name":"category","type":"int","not_null":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}],"foreign_keys":[{"name":"fk_name","column_name":"category","referenced_table_name":"categories","referenced_column_name":"id"}]}`))

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[2].Line)
	})
	t.Run("detect table schema, index with expression", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
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

		mock.ExpectQuery(selectColumnNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				).AddRow(
					"category",
					"null",
					"NO",
					"int",
					"",
					"",
				),
			)

		mock.ExpectQuery(selectIndexNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"idx_category",
					1,
					"category",
					nil,
					"",
					0,
					"BTREE",
				).AddRow(
					"idx_category",
					2,
					nil,
					"category = 0",
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT, category INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"},{"name":"category","type":"int","not_null":true,"default_value":"null"}],"indexes":[{"name":"idx_category","type":"BTREE","columns":["category"],"expressions":["category = 0"],"unique":true,"nullable":false}]}`))

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[2].Line)
	})
	t.Run("detect table schema, index with multiple columns", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
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

		mock.ExpectQuery(selectColumnNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				).AddRow(
					"category",
					"null",
					"NO",
					"int",
					"",
					"",
				).AddRow(
					"name",
					"null",
					"YES",
					"varchar(255)",
					"",
					"",
				),
			)

		mock.ExpectQuery(selectIndexNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				).AddRow(
					"idx_name",
					1,
					"name",
					nil,
					"",
					0,
					"BTREE",
				).AddRow(
					"idx_name",
					2,
					nil,
					"name = 'test'",
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}).AddRow(
					"fk_name",
					"category",
					"categories",
					"id",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT, category INT, name VARCHAR(255))"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"},{"name":"category","type":"int","not_null":true,"default_value":"null"},{"name":"name","type":"varchar(255)","default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false},{"name":"idx_name","type":"BTREE","columns":["name"],"expressions":["name = 'test'"],"unique":true,"nullable":false}],"foreign_keys":[{"name":"fk_name","column_name":"category","referenced_table_name":"categories","referenced_column_name":"id"}]}`))

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[2].Line)
	})
	t.Run("detect table schema, index with expression", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
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

		mock.ExpectQuery(selectColumnNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				).AddRow(
					"category",
					"null",
					"NO",
					"int",
					"",
					"",
				),
			)

		mock.ExpectQuery(selectIndexNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"idx_category",
					1,
					"category",
					nil,
					"",
					0,
					"BTREE",
				).AddRow(
					"idx_category",
					2,
					nil,
					"category = 0",
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT, category INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"},{"name":"category","type":"int","not_null":true,"default_value":"null"}],"indexes":[{"name":"idx_category","type":"BTREE","columns":["category"],"expressions":["category = 0"],"unique":true,"nullable":false}]}`))

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[2].Line)
	})
	t.Run("detect table schema, index with multiple columns", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
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

		mock.ExpectQuery(selectColumnNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				).AddRow(
					"category",
					"null",
					"NO",
					"int",
					"",
					"",
				).AddRow(
					"name",
					"null",
					"YES",
					"varchar(255)",
					"",
					"",
				),
			)

		mock.ExpectQuery(selectIndexNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				).AddRow(
					"idx_name",
					1,
					"name",
					nil,
					"",
					0,
					"BTREE",
				).AddRow(
					"idx_name",
					2,
					nil,
					"name = 'test'",
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}).AddRow(
					"fk_name",
					"category",
					"categories",
					"id",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT, category INT, name VARCHAR(255))"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"},{"name":"category","type":"int","not_null":true,"default_value":"null"},{"name":"name","type":"varchar(255)","default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false},{"name":"idx_name","type":"BTREE","columns":["name"],"expressions":["name = 'test'"],"unique":true,"nullable":false}],"foreign_keys":[{"name":"fk_name","column_name":"category","referenced_table_name":"categories","referenced_column_name":"id"}]}`))

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[2].Line)
	})
	t.Run("detect table schema, cache enabled (write)", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		// Enable caching. This will exercise the code path
		// that writes to cache (but we don't explicitly assert it in this test)
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    true,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})

		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
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

		mock.ExpectQuery(selectColumnNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				).AddRow(
					"category",
					"null",
					"NO",
					"int",
					"",
					"",
				),
			)

		mock.ExpectQuery(selectIndexNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}).AddRow(
					"fk_name",
					"category",
					"categories",
					"id",
				),
			)
		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		require.Equal(t, 1, collector.cache.Len())

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT, category INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"},{"name":"category","type":"int","not_null":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}],"foreign_keys":[{"name":"fk_name","column_name":"category","referenced_table_name":"categories","referenced_column_name":"id"}]}`))

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[2].Line)
	})
	t.Run("detect table schema, cache enabled (write and read)", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		// first loop, table info will be written to cache
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    true,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})

		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		mock.ExpectQuery("SHOW CREATE TABLE `some_schema`.`some_table`").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"create_statement",
				}).AddRow(
					"some_schema.some_table",
					"CREATE TABLE some_table (id INT)",
				),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				),
			)

		mock.ExpectQuery(selectIndexNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		// second loop, table info will be read from cache
		// and no further queries will be executed
		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 6
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Equal(t, 1, collector.cache.Len())

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}]}`))

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[3].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[3].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[4].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[4].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[5].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[5].Line)
	})
	t.Run("detect view schema", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})

		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"VIEW",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
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

		mock.ExpectQuery(selectColumnNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				),
			)

		mock.ExpectQuery(selectIndexNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)
		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE VIEW some_view (id INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}]}`))

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[2].Line)
	})
	t.Run("schemas result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows(
				[]string{"schema_name"},
			).AddRow(
				"some_schema",
			).AddRow(
				"another_schema",
			).RowError(1, fmt.Errorf("rs error"))) // error on the second row

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
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
	})
	t.Run("tables result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"schema_name",
			}).AddRow(
				"some_schema",
			),
		)
		mock.ExpectQuery(selectTableName).WithArgs("some_schema").WillReturnRows(
			sqlmock.NewRows([]string{
				"table_name",
				"table_type",
				"create_time",
				"update_time",
			}).AddRow(
				"some_table",
				"BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			).AddRow(
				"another_table",
				"BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			).RowError(1, fmt.Errorf("rs error")), // error on the second row
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
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
	})
	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"schema_name",
			}).AddRow(
				"some_schema",
			),
		)
		mock.ExpectQuery(selectTableName).WithArgs("some_schema").WillReturnRows(
			sqlmock.NewRows([]string{
				"table_name",
				"table_type",
				"create_time",
				"update_time",
			}).AddRow(
				"some_table",
				"BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			),
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

		mock.ExpectQuery(selectColumnNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"COLUMN_NAME",
					"COLUMN_DEFAULT",
					"IS_NULLABLE",
					"COLUMN_TYPE",
					"COLUMN_KEY",
					"EXTRA",
				}).AddRow(
					"id",
					"null",
					"NO",
					"int",
					"PRI",
					"auto_increment",
				),
			)

		mock.ExpectQuery(selectIndexNames).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"column_name",
					"expression",
					"nullable",
					"non_unique",
					"index_type",
				}).AddRow(
					"PRIMARY",
					1,
					"id",
					nil,
					"",
					0,
					"BTREE",
				),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("some_schema", "some_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		expectedCreateStmt := base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)"))
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}],"indexes":[{"name":"PRIMARY","type":"BTREE","columns":["id"],"unique":true,"nullable":false}]}`))

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, expectedCreateStmt, expectedTableSpec), lokiEntries[2].Line)
	})
}

func TestSchemaDetailsExcludeSchemas(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

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
		CacheEnabled:    false,
		Logger:          log.NewLogfmtLogger(os.Stderr),
	})
	require.NoError(t, err)

	// Verify the query uses the custom exclusion clause
	mock.ExpectQuery(fmt.Sprintf(selectSchemaNameTemplate, buildExcludedSchemasClause([]string{"excluded_schema"}))).
		WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"schema_name"}))

	c.extractSchema(t.Context())
	require.NoError(t, mock.ExpectationsWereMet())
}
