package collector

import (
	"context"
	"encoding/base64"
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

func TestSchemaTable(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("detect table schema", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().RowsWillBeClosed().
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

		mock.ExpectQuery("SHOW CREATE TABLE some_schema.some_table").WithoutArgs().RowsWillBeClosed().
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

		err = collector.Start(context.Background())
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

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="schema detected" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "mysql-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level=info msg="table detected" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "mysql-db"}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level=info msg="create table" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)")), base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}]}`))), lokiEntries[2].Line)
	})
	t.Run("detect table schema, cache enabled (write)", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		// Enable caching. This will exercise the code path
		// that writes to cache (but we explicitly assert it in this test)
		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheEnabled:    true,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})

		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().RowsWillBeClosed().
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

		mock.ExpectQuery("SHOW CREATE TABLE some_schema.some_table").WithoutArgs().RowsWillBeClosed().
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

		err = collector.Start(context.Background())
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

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="schema detected" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "mysql-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level=info msg="table detected" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "mysql-db"}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level=info msg="create table" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)")), base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}]}`))), lokiEntries[2].Line)
	})
	t.Run("detect table schema, cache enabled (write and read)", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		// first loop, table info will be written to cache
		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheEnabled:    true,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})

		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().RowsWillBeClosed().
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

		mock.ExpectQuery("SHOW CREATE TABLE some_schema.some_table").WithoutArgs().RowsWillBeClosed().
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

		// second loop, table info will be read from cache
		// and no further queries will be executed
		mock.ExpectQuery(selectSchemaName).WithoutArgs().RowsWillBeClosed().
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

		err = collector.Start(context.Background())
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

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="schema detected" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "mysql-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level=info msg="table detected" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "mysql-db"}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level=info msg="create table" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)")), base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}]}`))), lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "mysql-db"}, lokiEntries[3].Labels)
		require.Equal(t, `level=info msg="schema detected" schema="some_schema"`, lokiEntries[3].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "mysql-db"}, lokiEntries[4].Labels)
		require.Equal(t, `level=info msg="table detected" schema="some_schema" table="some_table"`, lokiEntries[4].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "mysql-db"}, lokiEntries[5].Labels)
		require.Equal(t, fmt.Sprintf(`level=info msg="create table" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)")), base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}]}`))), lokiEntries[5].Line)
	})
	t.Run("detect view schema", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})

		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().RowsWillBeClosed().
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

		mock.ExpectQuery("SHOW CREATE TABLE some_schema.some_table").WithoutArgs().RowsWillBeClosed().
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

		err = collector.Start(context.Background())
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

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="schema detected" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "mysql-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level=info msg="table detected" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "mysql-db"}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level=info msg="create table" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, base64.StdEncoding.EncodeToString([]byte("CREATE VIEW some_view (id INT)")), base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}]}`))), lokiEntries[2].Line)
	})
	t.Run("schemas result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().WillReturnRows(
			sqlmock.NewRows(
				[]string{"schema_name"},
			).AddRow(
				"some_schema",
			).AddRow(
				"another_schema",
			).RowError(1, fmt.Errorf("rs error"))) // error on the second row

		err = collector.Start(context.Background())
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="schema detected" schema="some_schema"`, lokiEntries[0].Line)
	})
	t.Run("tables result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().WillReturnRows(
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="schema detected" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "mysql-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level=info msg="table detected" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
	})
	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(selectSchemaName).WithoutArgs().WillReturnRows(
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
		mock.ExpectQuery("SHOW CREATE TABLE some_schema.some_table").WithoutArgs().WillReturnRows(
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

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level=info msg="schema detected" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "mysql-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level=info msg="table detected" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "mysql-db"}, lokiEntries[2].Labels)
		require.Equal(t, fmt.Sprintf(`level=info msg="create table" schema="some_schema" table="some_table" create_statement="%s" table_spec="%s"`, base64.StdEncoding.EncodeToString([]byte("CREATE TABLE some_table (id INT)")), base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"null"}]}`))), lokiEntries[2].Line)
	})
}
