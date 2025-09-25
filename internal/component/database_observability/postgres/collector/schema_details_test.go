package collector

import (
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

func TestSchemaTable(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("collector selects and logs schema details", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:           db,
			EntryHandler: lokiClient,
			Logger:       log.NewLogfmtLogger(os.Stderr),
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
		require.Equal(t, `level="info" database="books_store" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public" table="authors"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"name","type":"character varying(255)"}],"indexes":[{"name":"authors_pkey","type":"btree","columns":["id"],"unique":true,"nullable":true}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="public" table="authors" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})

	t.Run("collector selects and logs multiple schemas and multiple tables", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:           db,
			EntryHandler: lokiClient,
			Logger:       log.NewLogfmtLogger(os.Stderr),
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
		require.Equal(t, `level="info" database="books_store" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="books_store" schema="postgis"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public" table="authors"`, lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[3].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public" table="categories"`, lokiEntries[3].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[4].Labels)
		require.Equal(t, `level="info" database="books_store" schema="postgis" table="spatial_ref_sys"`, lokiEntries[4].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[5].Labels)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[6].Labels)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[7].Labels)
		expectedAuthorsTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true}],"indexes":[{"name":"authors_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		expectedCategoriesTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true}],"indexes":[{"name":"categories_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		expectedSpatialTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"srid","type":"integer","not_null":true,"primary_key":true}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="public" table="authors" table_spec="%s"`, expectedAuthorsTableSpec), lokiEntries[5].Line)
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="public" table="categories" table_spec="%s"`, expectedCategoriesTableSpec), lokiEntries[6].Line)
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="postgis" table="spatial_ref_sys" table_spec="%s"`, expectedSpatialTableSpec), lokiEntries[7].Line)
	})

	t.Run("collector handles multiple indexes on single table", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:           db,
			EntryHandler: lokiClient,
			Logger:       log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDatabaseName).WithoutArgs().RowsWillBeClosed().
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
		assert.Len(t, lokiEntries, 3)
		require.Equal(t, model.LabelSet{"op": OP_SCHEMA_DETECTION}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="multi_index_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="multi_index_db" schema="public" table="users"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"name","type":"character varying(255)","not_null":true},{"name":"email","type":"character varying(255)"},{"name":"created_at","type":"timestamp with time zone","not_null":true,"default_value":"now()"}],"indexes":[{"name":"users_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false},{"name":"idx_users_email_unique","type":"btree","columns":["email"],"unique":true,"nullable":false},{"name":"idx_users_name","type":"btree","columns":["name"],"unique":false,"nullable":false},{"name":"idx_users_name_lower","type":"btree","columns":null,"expressions":["lower(name::text)"],"unique":false,"nullable":true},{"name":"idx_users_created_at","type":"btree","columns":["created_at"],"unique":false,"nullable":false}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="multi_index_db" schema="public" table="users" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})

	t.Run("no schemas found", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		logBuffer := syncbuffer.Buffer{}
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:           db,
			EntryHandler: lokiClient,
			Logger:       log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
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

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:           db,
			EntryHandler: lokiClient,
			Logger:       log.NewLogfmtLogger(os.Stderr),
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
		require.Equal(t, `level="info" database="test_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public" table="test_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"name","type":"character varying(255)"}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="test_db" schema="public" table="test_table" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
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

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:           db,
			EntryHandler: lokiClient,
			Logger:       log.NewLogfmtLogger(os.Stderr),
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
		require.Equal(t, `level="info" database="serial_test_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="serial_test_db" schema="public" table="users"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"nextval('users_id_seq'::regclass)"},{"name":"username","type":"character varying(255)","not_null":true}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="serial_test_db" schema="public" table="users" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})

	t.Run("collector detects identity column", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:           db,
			EntryHandler: lokiClient,
			Logger:       log.NewLogfmtLogger(os.Stderr),
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
		require.Equal(t, `level="info" database="identity_test_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="identity_test_db" schema="public" table="products"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"auto_increment":true,"primary_key":true},{"name":"code","type":"integer","not_null":true,"auto_increment":true},{"name":"name","type":"character varying(255)","not_null":true}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="identity_test_db" schema="public" table="products" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})
}
