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
	"github.com/lib/pq"
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

		// Mock the two index queries (combined columns + expressions)
		mock.ExpectQuery(selectIndexColumns).WithArgs("public", "authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
				}).AddRow("authors_pkey", "btree", true, pq.StringArray{"id"}),
			)

		mock.ExpectQuery(selectIndexExpressions).WithArgs("public", "authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"expression",
				}), // No expressions for this index
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public" table="authors"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"name","type":"character varying(255)"}],"indexes":[{"name":"authors_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="public" table="authors" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
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

		mock.ExpectQuery(selectIndexColumns).WithArgs("public", "authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
				}).AddRow("authors_pkey", "btree", true, pq.StringArray{"id"}),
			)

		mock.ExpectQuery(selectIndexExpressions).WithArgs("public", "authors").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"expression",
				}), // No expressions for this index
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

		mock.ExpectQuery(selectIndexColumns).WithArgs("public", "categories").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
				}).AddRow("categories_pkey", "btree", true, pq.StringArray{"id"}),
			)

		mock.ExpectQuery(selectIndexExpressions).WithArgs("public", "categories").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"expression",
				}), // No expressions for this index
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

		mock.ExpectQuery(selectIndexColumns).WithArgs("postgis", "spatial_ref_sys").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
				}).AddRow("spatial_ref_sys_pkey", "btree", true, pq.StringArray{"srid"}),
			)

		mock.ExpectQuery(selectIndexExpressions).WithArgs("postgis", "spatial_ref_sys").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"expression",
				}), // No expressions for this index
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 8
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()

		assert.Len(t, lokiEntries, 8)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="books_store" schema="postgis"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public" table="authors"`, lokiEntries[2].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[3].Labels)
		require.Equal(t, `level="info" database="books_store" schema="public" table="categories"`, lokiEntries[3].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[4].Labels)
		require.Equal(t, `level="info" database="books_store" schema="postgis" table="spatial_ref_sys"`, lokiEntries[4].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[5].Labels)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[6].Labels)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[7].Labels)
		expectedAuthorsTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true}],"indexes":[{"name":"authors_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		expectedCategoriesTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true}],"indexes":[{"name":"categories_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		expectedSpatialTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"srid","type":"integer","not_null":true,"primary_key":true}],"indexes":[{"name":"spatial_ref_sys_pkey","type":"btree","columns":["srid"],"unique":true,"nullable":false}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="public" table="authors" table_spec="%s"`, expectedAuthorsTableSpec), lokiEntries[5].Line)
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="public" table="categories" table_spec="%s"`, expectedCategoriesTableSpec), lokiEntries[6].Line)
		require.Equal(t, fmt.Sprintf(`level="info" database="books_store" schema="postgis" table="spatial_ref_sys" table_spec="%s"`, expectedSpatialTableSpec), lokiEntries[7].Line)
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
			return len(lokiClient.Received()) == 0
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Len(t, lokiEntries, 0)
	})

	t.Run("collector handles column with no default value", func(t *testing.T) {
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
					AddRow("name", "character varying(255)", false, "John Doe", "", false),
			)

		mock.ExpectQuery(selectIndexColumns).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
				}).AddRow("test_table_pkey", "btree", true, pq.StringArray{"id"}),
			)

		mock.ExpectQuery(selectIndexExpressions).WithArgs("public", "test_table").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"expression",
				}), // No expressions for this index
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="test_db" schema="public" table="test_table"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"primary_key":true},{"name":"name","type":"character varying(255)","default_value":"John Doe"}],"indexes":[{"name":"test_table_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="test_db" schema="public" table="test_table" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})
}

func TestSchemaTable_index_collection(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("collector collects and parses index information", func(t *testing.T) {
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
				}).AddRow("id", "integer", true, "", "", true).
					AddRow("email", "character varying(255)", false, "", "", false).
					AddRow("name", "character varying(100)", true, "", "", false),
			)

		mock.ExpectQuery(selectIndexColumns).WithArgs("public", "users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
				}).AddRow("users_email_idx", "btree", false, pq.StringArray{"email"}).
					AddRow("users_name_gin_idx", "gin", false, pq.StringArray{"name"}),
			)

		mock.ExpectQuery(selectIndexExpressions).WithArgs("public", "users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"expression",
				}), // No expressions for this index
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

		// Check that the last entry contains the table spec with indexes
		tableSpecEntry := lokiEntries[2]
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, tableSpecEntry.Labels)

		// The table spec should include the parsed indexes
		require.Contains(t, tableSpecEntry.Line, `table_spec=`)

		// Extract and decode the table spec
		parts := strings.Split(tableSpecEntry.Line, `table_spec="`)
		require.Len(t, parts, 2)
		encodedSpec := strings.TrimSuffix(parts[1], `"`)

		decodedBytes, err := base64.StdEncoding.DecodeString(encodedSpec)
		require.NoError(t, err)

		var tableSpec tableSpec
		err = json.Unmarshal(decodedBytes, &tableSpec)
		require.NoError(t, err)

		// Verify the indexes were parsed correctly
		require.Len(t, tableSpec.Indexes, 2)

		// Check first index
		emailIdx := tableSpec.Indexes[0]
		assert.Equal(t, "users_email_idx", emailIdx.Name)
		assert.Equal(t, []string{"email"}, emailIdx.Columns)

		// Check second index
		nameIdx := tableSpec.Indexes[1]
		assert.Equal(t, "users_name_gin_idx", nameIdx.Name)
		assert.Equal(t, []string{"name"}, nameIdx.Columns)
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

		mock.ExpectQuery(selectIndexColumns).WithArgs("public", "users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
				}).AddRow("users_pkey", "btree", true, pq.StringArray{"id"}),
			)

		mock.ExpectQuery(selectIndexExpressions).WithArgs("public", "users").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"expression",
				}), // No expressions for this index
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="serial_test_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="serial_test_db" schema="public" table="users"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"auto_increment":true,"primary_key":true,"default_value":"nextval('users_id_seq'::regclass)"},{"name":"username","type":"character varying(255)","not_null":true}],"indexes":[{"name":"users_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="serial_test_db" schema="public" table="users" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
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

		mock.ExpectQuery(selectIndexColumns).WithArgs("public", "products").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"index_type",
					"unique",
					"column_names",
				}).AddRow("products_pkey", "btree", true, pq.StringArray{"id"}),
			)

		mock.ExpectQuery(selectIndexExpressions).WithArgs("public", "products").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"index_name",
					"seq_in_index",
					"expression",
				}), // No expressions for this index
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" database="identity_test_db" schema="public"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_TABLE_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" database="identity_test_db" schema="public" table="products"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_CREATE_STATEMENT, "instance": "postgres-db"}, lokiEntries[2].Labels)
		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"integer","not_null":true,"auto_increment":true,"primary_key":true},{"name":"code","type":"integer","not_null":true,"auto_increment":true},{"name":"name","type":"character varying(255)","not_null":true}],"indexes":[{"name":"products_pkey","type":"btree","columns":["id"],"unique":true,"nullable":false}]}`))
		require.Equal(t, fmt.Sprintf(`level="info" database="identity_test_db" schema="public" table="products" table_spec="%s"`, expectedTableSpec), lokiEntries[2].Line)
	})
}
