package collector

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/lib/pq"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/util/syncbuffer"
)

func Test_Postgres_SchemaDetails(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("collector selects and logs schema details", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/books_store",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
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
				}).AddRow("authors"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", true, "", "", true).
					AddRow("name", "character varying(255)", false, "", "", false),
			)

		mock.ExpectQuery(selectIndexes).WithArgs("public", "authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}).AddRow("authors_pkey", "btree", true, pq.StringArray{"id"}, pq.StringArray{}, true),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "authors").RowsWillBeClosed().
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
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		// Run this after Stop() to avoid race conditions
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()

		assert.Len(t, lokiEntries, 3)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" datname="books_store" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" datname="books_store" schema="public" table="authors"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"name","type":"character varying(255)"}],"indexes":[{"name":"authors_pkey","type":"btree","columns":["id"],"unique":true,"nullable":true}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" datname="books_store" schema="public" table="authors" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})

	t.Run("collector selects and logs multiple schemas and multiple tables", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/books_store",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
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
				}).AddRow("authors").
					AddRow("categories"),
			)

		mock.ExpectQuery(selectTableNames).WithArgs("postgis").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
				}).AddRow("spatial_ref_sys"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", true, nil, "", true),
			)

		mock.ExpectQuery(selectIndexes).WithArgs("public", "authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}).AddRow("authors_pkey", "btree", true, pq.StringArray{"id"}, pq.StringArray{}, false),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.categories").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", true, nil, "", true),
			)

		mock.ExpectQuery(selectIndexes).WithArgs("public", "categories").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}).AddRow("categories_pkey", "btree", true, pq.StringArray{"id"}, pq.StringArray{}, false),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "categories").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("postgis.spatial_ref_sys").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("srid", "integer", true, nil, "", true),
			)

		mock.ExpectQuery(selectIndexes).WithArgs("postgis", "spatial_ref_sys").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("postgis", "spatial_ref_sys").RowsWillBeClosed().
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
			return len(lokiClient.Received()) == 8
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		// Run this after Stop() to avoid race conditions
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()

		assert.Len(t, lokiEntries, 8)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" datname="books_store" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" datname="books_store" schema="postgis"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" datname="books_store" schema="public" table="authors"`, lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[3].Labels)
		require.Equal(t, `level="info" datname="books_store" schema="public" table="categories"`, lokiEntries[3].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[4].Labels)
		require.Equal(t, `level="info" datname="books_store" schema="postgis" table="spatial_ref_sys"`, lokiEntries[4].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[5].Labels)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[6].Labels)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[7].Labels)
		expectedAuthorsTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true}],"indexes":[{"name":"authors_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		expectedCategoriesTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true}],"indexes":[{"name":"categories_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		expectedSpatialTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"srid","type":"integer","not_null":true,"primary_key":true}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" datname="books_store" schema="public" table="authors" table_spec="%s"`, expectedAuthorsTableSpec), lokiEntries[5].Line)
		require.Equal(t, fmt.Sprintf(`level="info" datname="books_store" schema="public" table="categories" table_spec="%s"`, expectedCategoriesTableSpec), lokiEntries[6].Line)
		require.Equal(t, fmt.Sprintf(`level="info" datname="books_store" schema="postgis" table="spatial_ref_sys" table_spec="%s"`, expectedSpatialTableSpec), lokiEntries[7].Line)
	})

	t.Run("collector discovers and collects from multiple databases", func(t *testing.T) {
		t.Parallel()

		/*
			This is the only test that sets up 3 mock connections representing a Postgres instance with 3 separate databases,
			better representing the individual connections of database discovery.
			ExpectationsWereMet() is called on each connection at end of the test, asserting that the connections are
			correctly used.
			This is the only test that will fail if distinct connections are not made.
		*/
		initialConnectionDb, initialConnectionMock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer initialConnectionDb.Close()
		db1, db1Mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db1.Close()
		db2, db2Mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db2.Close()

		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              initialConnectionDb,
			DSN:             "postgres://user:pass@localhost:5432/postgres",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				switch dsn {
				case "postgres://user:pass@localhost:5432/db1":
					return db1, nil
				case "postgres://user:pass@localhost:5432/db2":
					return db2, nil
				default:
					return nil, fmt.Errorf("unexpected DSN: %s", dsn)
				}
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		initialConnectionMock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{"datname"}).
					AddRow("db1").
					AddRow("db2"),
			)

		db1Mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"schema_name"}).AddRow("public"))
		db1Mock.ExpectQuery(selectTableNames).WithArgs("public").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("users"))
		db1Mock.ExpectQuery(selectColumnNames).WithArgs("public.users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation", "is_primary_key"}).
					AddRow("id", "integer", true, nil, "", true),
			)
		db1Mock.ExpectQuery(selectIndexes).WithArgs("public", "users").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"index_name", "index_type", "unique", "column_names", "expressions", "has_nullable_column"}))
		db1Mock.ExpectQuery(selectForeignKeys).WithArgs("public", "users").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "column_name", "referenced_table_name", "referenced_column_name"}))

		db2Mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"schema_name"}).AddRow("public"))
		db2Mock.ExpectQuery(selectTableNames).WithArgs("public").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("metrics"))
		db2Mock.ExpectQuery(selectColumnNames).WithArgs("public.metrics").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation", "is_primary_key"}).
					AddRow("id", "bigint", true, nil, "", true),
			)
		db2Mock.ExpectQuery(selectIndexes).WithArgs("public", "metrics").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"index_name", "index_type", "unique", "column_names", "expressions", "has_nullable_column"}))
		db2Mock.ExpectQuery(selectForeignKeys).WithArgs("public", "metrics").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "column_name", "referenced_table_name", "referenced_column_name"}))

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 6
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.NoError(t, initialConnectionMock.ExpectationsWereMet())
		require.NoError(t, db1Mock.ExpectationsWereMet())
		require.NoError(t, db2Mock.ExpectationsWereMet())

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 6)

		assert.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" datname="db1" schema="public"`, lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		assert.Equal(t, `level="info" datname="db1" schema="public" table="users"`, lokiEntries[1].Line)
		assert.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		assert.Equal(t, fmt.Sprintf(`level="info" datname="db1" schema="public" table="users" table_spec="%s"`, base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true}]}`))), lokiEntries[2].Line)

		assert.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[3].Labels)
		assert.Equal(t, `level="info" datname="db2" schema="public"`, lokiEntries[3].Line)
		assert.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[4].Labels)
		assert.Equal(t, `level="info" datname="db2" schema="public" table="metrics"`, lokiEntries[4].Line)
		assert.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[5].Labels)
		assert.Equal(t, fmt.Sprintf(`level="info" datname="db2" schema="public" table="metrics" table_spec="%s"`, base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"bigint","not_null":true,"primary_key":true}]}`))), lokiEntries[5].Line)
	})

	t.Run("collector handles multiple indexes on single table", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/testdb",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"multi_index_db",
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
				}).AddRow("users"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", true, nil, "", true).
					AddRow("name", "character varying(255)", true, nil, "", false).
					AddRow("email", "character varying(255)", false, nil, "", false).
					AddRow("created_at", "timestamp with time zone", true, "now()", "", false),
			)

		mock.ExpectQuery(selectIndexes).WithArgs("public", "users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}).AddRow("users_pkey", "btree", true, pq.StringArray{"id"}, nil, false).
					AddRow("idx_users_email_unique", "btree", true, pq.StringArray{"email"}, nil, false).
					AddRow("idx_users_name", "btree", false, pq.StringArray{"name"}, nil, false).
					AddRow("idx_users_name_lower", "btree", false, nil, pq.StringArray{"lower(name::text)"}, true).
					AddRow("idx_users_created_at", "btree", false, pq.StringArray{"created_at"}, nil, false),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "users").RowsWillBeClosed().
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
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		// Run this after Stop() to avoid race conditions
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 3)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" datname="multi_index_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" datname="multi_index_db" schema="public" table="users"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"name","type":"character varying(255)","not_null":true},{"name":"email","type":"character varying(255)"},{"name":"created_at","type":"timestamp with time zone","not_null":true,"default_value":"now()"}],"indexes":[{"name":"users_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false},{"name":"idx_users_email_unique","type":"btree","columns":["email"],"unique":true,"nullable":false},{"name":"idx_users_name","type":"btree","columns":["name"],"unique":false,"nullable":false},{"name":"idx_users_name_lower","type":"btree","columns":null,"expressions":["lower(name::text)"],"unique":false,"nullable":true},{"name":"idx_users_created_at","type":"btree","columns":["created_at"],"unique":false,"nullable":false}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" datname="multi_index_db" schema="public" table="users" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})

	t.Run("no schemas found", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		logBuffer := syncbuffer.Buffer{}
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/testdb",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
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
			return strings.Contains(logBuffer.String(), `msg="no schema detected from pg_namespace"`)
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		// Run this after Stop() to avoid race conditions
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		assert.Len(t, lokiClient.Received(), 0)
	})

	t.Run("collector logs column with null and empty string default values", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/testdb",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
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
				}).AddRow("test_table"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).
					AddRow("id", "integer", true, nil, "", true).
					AddRow("name", "character varying(255)", false, "", "", false),
			)

		mock.ExpectQuery(selectIndexes).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "test_table").RowsWillBeClosed().
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
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		// Run this after Stop() to avoid race conditions
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 3)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" datname="test_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" datname="test_db" schema="public" table="test_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"name","type":"character varying(255)"}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" datname="test_db" schema="public" table="test_table" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})
}

func Test_Postgres_SchemaDetails_collector_detects_auto_increment_column(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("collector detects auto increment column", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/testdb",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
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
				}).AddRow("users"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", true, "nextval('users_id_seq'::regclass)", "", true).
					AddRow("username", "character varying(255)", true, nil, "", false),
			)

		mock.ExpectQuery(selectIndexes).WithArgs("public", "users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "users").RowsWillBeClosed().
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
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		// Run this after Stop() to avoid race conditions
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 3)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" datname="serial_test_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" datname="serial_test_db" schema="public" table="users"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"nextval('users_id_seq'::regclass)"},{"name":"username","type":"character varying(255)","not_null":true}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" datname="serial_test_db" schema="public" table="users" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})

	t.Run("collector detects identity column", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})
		defer lokiClient.Stop()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/testdb",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
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
				}).AddRow("products"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.products").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", true, "", "a", true).
					AddRow("code", "integer", true, "", "d", false).
					AddRow("name", "character varying(255)", true, "", "", false),
			)

		mock.ExpectQuery(selectIndexes).WithArgs("public", "products").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "products").RowsWillBeClosed().
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
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		// Run this after Stop() to avoid race conditions
		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 3)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" datname="identity_test_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" datname="identity_test_db" schema="public" table="products"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"auto_increment":true,"primary_key":true},{"name":"code","type":"integer","not_null":true,"auto_increment":true},{"name":"name","type":"character varying(255)","not_null":true}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" datname="identity_test_db" schema="public" table="products" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})

	t.Run("collector detects foreign keys", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/testdb",
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
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
				}).AddRow("books"),
			)

		mock.ExpectQuery(selectColumnNames).WithArgs("public.books").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", true, "", "", true).
					AddRow("title", "character varying(255)", true, "", "", false).
					AddRow("author_id", "integer", true, "", "", false).
					AddRow("category_id", "integer", false, "", "", false),
			)

		mock.ExpectQuery(selectIndexes).WithArgs("public", "books").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}).AddRow("books_pkey", "btree", true, pq.StringArray{"id"}, pq.StringArray{}, false),
			)

		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "books").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}).AddRow("fk_books_author", "author_id", "authors", "id").
					AddRow("fk_books_category", "category_id", "categories", "id"),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 3)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"title","type":"character varying(255)","not_null":true},{"name":"author_id","type":"integer","not_null":true},{"name":"category_id","type":"integer"}],"indexes":[{"name":"books_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}],"foreign_keys":[{"name":"fk_books_author","column_name":"author_id","referenced_table_name":"authors","referenced_column_name":"id"},{"name":"fk_books_category","column_name":"category_id","referenced_table_name":"categories","referenced_column_name":"id"}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" datname="books_store" schema="public" table="books" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})
}

func Test_Postgres_SchemaDetails_caching(t *testing.T) {
	t.Run("uses cache on second run when caching is enabled", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/cache_test_db",
			CollectInterval: time.Millisecond,
			CacheEnabled:    true,
			CacheSize:       256,
			CacheTTL:        10 * time.Minute,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		// first run mock declarations
		// selectDatabaseName, selectSchemaNames, selectTableNames always called
		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"cache_test_db",
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
				}).AddRow("test_table"),
			)

		// selectColumnNames, selectIndexes, selectForeignKeys called only first run
		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"column_name",
					"column_type",
					"not_nullable",
					"column_default",
					"identity_generation",
					"is_primary_key",
				}).AddRow("id", "integer", true, nil, "", true),
			)
		mock.ExpectQuery(selectIndexes).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
					"expressions",
					"has_nullable_column",
				}),
			)
		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}),
			)

		// first run invocation
		require.NoError(t, collector.extractNames(context.Background()))
		require.Eventually(t, func() bool { return len(lokiClient.Received()) == 3 }, 2*time.Second, 100*time.Millisecond)
		firstRunEntries := lokiClient.Received()
		require.Len(t, firstRunEntries, 3)

		lokiClient.Clear()

		// second run mock declarations
		// selectDatabaseName, selectSchemaNames, selectTableNames always called
		mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"cache_test_db",
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
				}).AddRow("test_table"),
			)
		// Here, note that selectColumnNames, selectIndexes, selectForeignKeys mocks are not declared: they should not be called due to caching

		// second run invocation
		require.NoError(t, collector.extractNames(context.Background()))
		require.Eventually(t, func() bool { return len(lokiClient.Received()) == 3 }, 2*time.Second, 100*time.Millisecond)
		secondRunEntries := lokiClient.Received()
		require.Len(t, secondRunEntries, 3)

		// assert that first and second run results are identical
		for i := range firstRunEntries {
			assert.Equal(t, firstRunEntries[i].Labels, secondRunEntries[i].Labels)
			assert.Equal(t, firstRunEntries[i].Line, secondRunEntries[i].Line)
		}

		// ensure that selectColumnNames, selectIndexes, selectForeignKeys are not called
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()
	})

	t.Run("bypasses cache when caching is disabled", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			DSN:             "postgres://user:pass@localhost:5432/no_cache_test_db",
			CollectInterval: time.Millisecond,
			CacheEnabled:    false,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
			dbConnectionFactory: func(dsn string) (*sql.DB, error) {
				return db, nil
			},
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		// declare mocks for two runs
		for i := 0; i < 2; i++ {
			mock.ExpectQuery(selectAllDatabases).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"datname",
					}).AddRow(
						"no_cache_test_db",
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
					}).AddRow("test_table"),
				)

			mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"column_name",
						"column_type",
						"not_nullable",
						"column_default",
						"identity_generation",
						"is_primary_key",
					}).AddRow("id", "integer", true, nil, "", true),
				)

			mock.ExpectQuery(selectIndexes).WithArgs("public", "test_table").RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"index_name",
						"index_type",
						"unique",
						"column_names",
						"expressions",
						"has_nullable_column",
					}),
				)

			mock.ExpectQuery(selectForeignKeys).WithArgs("public", "test_table").RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"constraint_name",
						"column_name",
						"referenced_table_name",
						"referenced_column_name",
					}),
				)
		}

		// first run
		require.NoError(t, collector.extractNames(context.Background()))
		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 2*time.Second, 100*time.Millisecond)
		lokiClient.Clear()

		// second run
		require.NoError(t, collector.extractNames(context.Background()))
		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 2*time.Second, 100*time.Millisecond)

		// ensure that selectColumnNames, selectIndexes, selectForeignKeys are called twice
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()
	})
}

func Test_Postgres_SchemaDetails_ErrorCases(t *testing.T) {
	t.Run("getAllDatabases returns error when there is an error iterating database rows", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectAllDatabases).
			WillReturnRows(
				sqlmock.NewRows([]string{"datname"}).
					AddRow("testdb").
					RowError(0, fmt.Errorf("row iteration error")))

		collector := &SchemaDetails{
			logger:            log.NewNopLogger(),
			initialConnection: db,
		}
		_, err = collector.getAllDatabases(context.Background())

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("extractSchemas returns error when there is an error iterating over pg_namespace result set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectSchemaNames).
			WillReturnRows(
				sqlmock.NewRows([]string{"nspname"}).
					AddRow("public").
					RowError(0, fmt.Errorf("schema iteration error")))

		collector := &SchemaDetails{
			logger: log.NewNopLogger(),
		}

		err = collector.extractSchemas(context.Background(), "testdb", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchTableDefinitions returns error when selectColumnNames query fails", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").
			WillReturnError(fmt.Errorf("column query error"))

		collector := &SchemaDetails{logger: log.NewNopLogger()}
		_, err = collector.fetchTableDefinitions(context.Background(), &tableInfo{
			database:  "testdb",
			schema:    "public",
			tableName: "test_table",
		}, db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchColumnsDefinitions returns error when selectColumnNames query fails", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		collector := &SchemaDetails{
			logger: log.NewNopLogger(),
		}

		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").
			WillReturnError(fmt.Errorf("column query error"))

		_, err = collector.fetchColumnsDefinitions(context.Background(), "testdb", "public", "test_table", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchColumnsDefinitions returns error when selectColumnNames rows fail to scan", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		collector := &SchemaDetails{
			logger: log.NewNopLogger(),
		}

		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation"}).
				AddRow("id", "integer", true, nil, ""))

		_, err = collector.fetchColumnsDefinitions(context.Background(), "testdb", "public", "test_table", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchColumnsDefinitions returns error when there is an error iterating selectColumnNames result set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").
			WillReturnRows(
				sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation", "is_primary_key"}).
					AddRow("id", "integer", true, nil, "", true).
					RowError(0, fmt.Errorf("result set error")))

		collector := &SchemaDetails{logger: log.NewNopLogger()}
		_, err = collector.fetchColumnsDefinitions(context.Background(), "testdb", "public", "test_table", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchColumnsDefinitions returns error when selectIndexes returns an error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation", "is_primary_key"}).
				AddRow("id", "integer", true, nil, "", true))
		mock.ExpectQuery(selectIndexes).WithArgs("public", "test_table").
			WillReturnError(fmt.Errorf("index query error"))

		collector := &SchemaDetails{logger: log.NewNopLogger()}
		_, err = collector.fetchColumnsDefinitions(context.Background(), "testdb", "public", "test_table", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchColumnsDefinitions returns error when selectIndexes rows fail to scan", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation", "is_primary_key"}).
				AddRow("id", "integer", true, nil, "", true))
		mock.ExpectQuery(selectIndexes).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"index_name", "index_type", "unique", "column_names", "expressions"}).
				AddRow("idx_test", "btree", true, pq.StringArray{"id"}, pq.StringArray{}))

		collector := &SchemaDetails{logger: log.NewNopLogger()}
		_, err = collector.fetchColumnsDefinitions(context.Background(), "testdb", "public", "test_table", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchColumnsDefinitions returns error when there is an error iterating selectIndexes result set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation", "is_primary_key"}).
				AddRow("id", "integer", true, nil, "", true))
		mock.ExpectQuery(selectIndexes).WithArgs("public", "test_table").
			WillReturnRows(
				sqlmock.NewRows([]string{"index_name", "index_type", "unique", "column_names", "expressions", "has_nullable_column"}).
					AddRow("idx_test", "btree", true, pq.StringArray{"id"}, pq.StringArray{}, false).
					RowError(0, fmt.Errorf("result set error")))

		collector := &SchemaDetails{logger: log.NewNopLogger()}
		_, err = collector.fetchColumnsDefinitions(context.Background(), "testdb", "public", "test_table", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchColumnsDefinitions returns error when selectForeignKeys query fails", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation", "is_primary_key"}).
				AddRow("id", "integer", true, nil, "", true))
		mock.ExpectQuery(selectIndexes).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"index_name", "index_type", "unique", "column_names", "expressions", "has_nullable_column"}))
		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "test_table").
			WillReturnError(fmt.Errorf("foreign key query error"))

		collector := &SchemaDetails{logger: log.NewNopLogger()}
		_, err = collector.fetchColumnsDefinitions(context.Background(), "testdb", "public", "test_table", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchColumnsDefinitions returns error when selectForeignKeys rows fail to scan", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation", "is_primary_key"}).
				AddRow("id", "integer", true, nil, "", true))
		mock.ExpectQuery(selectIndexes).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"index_name", "index_type", "unique", "column_names", "expressions", "has_nullable_column"}))
		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "column_name", "referenced_table_name"}).
				AddRow("fk_test", "author_id", "authors"))

		collector := &SchemaDetails{logger: log.NewNopLogger()}
		_, err = collector.fetchColumnsDefinitions(context.Background(), "testdb", "public", "test_table", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fetchColumnsDefinitions returns error when there is an error iterating selectForeignKeys result set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectColumnNames).WithArgs("public.test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "not_nullable", "column_default", "identity_generation", "is_primary_key"}).
				AddRow("id", "integer", true, nil, "", true))
		mock.ExpectQuery(selectIndexes).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"index_name", "index_type", "unique", "column_names", "expressions", "has_nullable_column"}))
		mock.ExpectQuery(selectForeignKeys).WithArgs("public", "test_table").
			WillReturnRows(
				sqlmock.NewRows([]string{"constraint_name", "column_name", "referenced_table_name", "referenced_column_name"}).
					AddRow("fk_test", "author_id", "authors", "id").
					RowError(0, fmt.Errorf("result set error")))

		collector := &SchemaDetails{logger: log.NewNopLogger()}
		_, err = collector.fetchColumnsDefinitions(context.Background(), "testdb", "public", "test_table", db)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func Test_TableRegistry_IsValid(t *testing.T) {
	t.Run("returns true when table exists in registry", func(t *testing.T) {
		tr := NewTableRegistry()
		tr.SetTablesForDatabase("mydb", []*tableInfo{
			{database: "mydb", schema: "public", tableName: "users"},
			{database: "mydb", schema: "public", tableName: "orders"},
		})

		assert.True(t, tr.IsValid("mydb", "users"))
		assert.True(t, tr.IsValid("mydb", "orders"))
	})

	t.Run("returns false when table does not exist in registry", func(t *testing.T) {
		tr := NewTableRegistry()
		tr.SetTablesForDatabase("mydb", []*tableInfo{
			{database: "mydb", schema: "public", tableName: "users"},
		})

		assert.False(t, tr.IsValid("mydb", "nonexistent"))
	})

	t.Run("returns false given nonexistent database", func(t *testing.T) {
		tr := NewTableRegistry()
		tr.SetTablesForDatabase("mydb", []*tableInfo{
			{database: "mydb", schema: "public", tableName: "users"},
		})

		assert.False(t, tr.IsValid("otherdb", "users"))
	})

	t.Run("returns false for empty registry", func(t *testing.T) {
		tr := NewTableRegistry()

		assert.False(t, tr.IsValid("mydb", "users"))
	})

	t.Run("returns true when table exists in multiple schemas", func(t *testing.T) {
		tr := NewTableRegistry()
		tr.SetTablesForDatabase("mydb", []*tableInfo{
			{database: "mydb", schema: "public", tableName: "users"},
			{database: "mydb", schema: "private", tableName: "users"},
		})

		assert.True(t, tr.IsValid("mydb", "users"))
	})

	t.Run("returns true when schema-qualified table exists", func(t *testing.T) {
		tr := NewTableRegistry()
		tr.SetTablesForDatabase("mydb", []*tableInfo{
			{database: "mydb", schema: "private", tableName: "users"},
		})

		assert.True(t, tr.IsValid("mydb", "private.users"))
	})
}
