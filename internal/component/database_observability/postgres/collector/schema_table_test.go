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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/database_observability"
)

func TestSchemaTable(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("collector selects and logs database, schema, tables", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "postgres-db",
			CollectInterval: 10 * time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDatabaseName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"books_store",
				),
			)

		mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow("public"),
			)

		mock.ExpectQuery(selectTableNames).WithArgs("public").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
				}).AddRow("authors", "BASE TABLE"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", false, "", "", true).
					AddRow("name", "character varying(255)", true, "", "", false),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 4
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()

		assert.Len(t, lokiEntries, 4)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATABASE_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="books_store"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public" table="authors"`, lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[3].Labels)

		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"name","type":"character varying(255)"}]}`))

		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="public" table="authors" table_spec="%s"`, expectedTableSpec), lokiEntries[3].Line)
	})

	t.Run("collector selects and logs multiple schemas and multiple tables", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "postgres-db",
			CollectInterval: 10 * time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDatabaseName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"books_store",
				),
			)

		mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow("public").
					AddRow("postgis"),
			)

		mock.ExpectQuery(selectTableNames).WithArgs("public").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
				}).AddRow("authors", "BASE TABLE").
					AddRow("categories", "BASE TABLE"),
			)

		mock.ExpectQuery(selectTableNames).WithArgs("postgis").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
				}).AddRow("spatial_ref_sys", "BASE TABLE"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", false, "null", "", true),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.categories").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", false, "null", "", true),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("postgis.spatial_ref_sys").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("srid", "integer", false, "null", "", true),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 9
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()

		assert.Len(t, lokiEntries, 9)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATABASE_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="books_store"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" database="books_store" schema="postgis"`, lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[3].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public" table="authors"`, lokiEntries[3].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[4].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public" table="categories"`, lokiEntries[4].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[5].Labels)
		require.Equal(t, `level="info" database="books_store" schema="postgis" table="spatial_ref_sys"`, lokiEntries[5].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[6].Labels)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[7].Labels)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[8].Labels)

		expectedAuthorsTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true,"default_value":"null"}]}`))
		expectedCategoriesTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true,"default_value":"null"}]}`))
		expectedSpatialTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"srid","type":"integer","not_null":true,"primary_key":true,"default_value":"null"}]}`))

		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="public" table="authors" table_spec="%s"`, expectedAuthorsTableSpec), lokiEntries[6].Line)
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="public" table="categories" table_spec="%s"`, expectedCategoriesTableSpec), lokiEntries[7].Line)
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="postgis" table="spatial_ref_sys" table_spec="%s"`, expectedSpatialTableSpec), lokiEntries[8].Line)
	})

	t.Run("no schemas found", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "postgres-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDatabaseName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"books_store",
				),
			)

		mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 2*time.Second, 100*time.Millisecond)

		lokiEntries := lokiClient.Received()

		assert.Len(t, lokiEntries, 1)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATABASE_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="books_store"`, lokiEntries[0].Line)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiClient.Stop()
	})

	// TODO add test for "" default column value
	// TODO make sure assertion for null default column value is correct
	t.Run("collector handles null default column value", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "postgres-db",
			CollectInterval: 10 * time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDatabaseName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"test_db",
				),
			)

		mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow("public"),
			)

		mock.ExpectQuery(selectTableNames).WithArgs("public").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
				}).AddRow("test_table", "BASE TABLE"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow(
					"id",
					"integer",
					false,
					nil,
					"",
					true).AddRow("name", "character varying(255)", true, "John Doe", "", false),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 4
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 4)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATABASE_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="test_db"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public" table="test_table"`, lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[3].Labels)

		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true,"default_value":"null"},{"name":"name","type":"character varying(255)","default_value":"John Doe"}]}`))

		require.Equal(t, fmt.Sprintf(`level="info" database="test_db" schema="public" table="test_table" table_spec="%s"`, expectedTableSpec), lokiEntries[3].Line)
	})
}

func Test_collector_detects_auto_increment_column(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("collector detects auto increment column", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "postgres-db",
			CollectInterval: 10 * time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDatabaseName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"serial_test_db",
				),
			)

		mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow("public"),
			)

		mock.ExpectQuery(selectTableNames).WithArgs("public").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
				}).AddRow("users", "BASE TABLE"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", false, "nextval('users_id_seq'::regclass)", "", true).
					AddRow("username", "character varying(255)", false, "null", "", false),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 4
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 4)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATABASE_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="serial_test_db"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="serial_test_db" schema="public"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" database="serial_test_db" schema="public" table="users"`, lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[3].Labels)

		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"nextval('users_id_seq'::regclass)"},{"name":"username","type":"character varying(255)","not_null":true,"default_value":"null"}]}`))

		require.Equal(t, fmt.Sprintf(`level="info" database="serial_test_db" schema="public" table="users" table_spec="%s"`, expectedTableSpec), lokiEntries[3].Line)
	})

	t.Run("collector detects identity column", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "postgres-db",
			CollectInterval: 10 * time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDatabaseName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"identity_test_db",
				),
			)

		mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow("public"),
			)

		mock.ExpectQuery(selectTableNames).WithArgs("public").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
				}).AddRow("products", "BASE TABLE"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.products").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", false, "", "a", true).
					AddRow("code", "integer", false, "", "d", false).
					AddRow("name", "character varying(255)", false, "", "", false),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 4
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 4)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATABASE_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="identity_test_db"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="identity_test_db" schema="public"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" database="identity_test_db" schema="public" table="products"`, lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[3].Labels)

		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"auto_increment":true,"primary_key":true},{"name":"code","type":"integer","not_null":true,"auto_increment":true},{"name":"name","type":"character varying(255)","not_null":true}]}`))

		require.Equal(t, fmt.Sprintf(`level="info" database="identity_test_db" schema="public" table="products" table_spec="%s"`, expectedTableSpec), lokiEntries[3].Line)
	})
}

func Test_collector_detects_table_types(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("collector detects different table types", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "postgres-db",
			CollectInterval: 10 * time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDatabaseName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"test_db",
				),
			)

		mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow("public"),
			)

		mock.ExpectQuery(selectTableNames).WithArgs("public").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
				}).AddRow("users", "BASE TABLE").
					AddRow("user_view", "VIEW").
					AddRow("user_summary", "MATERIALIZED VIEW").
					AddRow("remote_data", "FOREIGN TABLE"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", false, "", "", true),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.user_view").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", false, "", "", false),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.user_summary").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("total_users", "bigint", false, "", "", false),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.remote_data").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"is_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("remote_id", "integer", false, "", "", false),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 10
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 10)

		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATABASE_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="test_db"`, lokiEntries[0].Line)

		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public"`, lokiEntries[1].Line)

		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public" table="users"`, lokiEntries[2].Line)

		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[3].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public" table="user_view"`, lokiEntries[3].Line)

		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[4].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public" table="user_summary"`, lokiEntries[4].Line)

		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[5].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public" table="remote_data"`, lokiEntries[5].Line)

		expectedUsersTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true}]}`))
		expectedViewTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true}]}`))
		expectedMaterializedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"total_users","type":"bigint","not_null":true}]}`))
		expectedForeignTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"remote_id","type":"integer","not_null":true}]}`))

		require.Equal(t, fmt.Sprintf(`level="info" database="test_db" schema="public" table="users" table_spec="%s"`, expectedUsersTableSpec), lokiEntries[6].Line)
		require.Equal(t, fmt.Sprintf(`level="info" database="test_db" schema="public" table="user_view" table_spec="%s"`, expectedViewTableSpec), lokiEntries[7].Line)
		require.Equal(t, fmt.Sprintf(`level="info" database="test_db" schema="public" table="user_summary" table_spec="%s"`, expectedMaterializedTableSpec), lokiEntries[8].Line)
		require.Equal(t, fmt.Sprintf(`level="info" database="test_db" schema="public" table="remote_data" table_spec="%s"`, expectedForeignTableSpec), lokiEntries[9].Line)
	})
}
