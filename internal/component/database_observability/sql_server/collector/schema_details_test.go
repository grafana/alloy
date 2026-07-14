package collector

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/util"
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
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"TABLE_CATALOG",
					"TABLE_SCHEMA",
					"TABLE_NAME",
					"TABLE_TYPE",
				}).AddRow(
					"some_db",
					"dbo",
					"some_table",
					"BASE TABLE",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"column_name",
					"column_type",
					"is_nullable",
					"is_identity",
					"default_value",
					"is_primary_key",
				}).AddRow(
					"some_table", "id", "int", false, true, nil, true,
				).AddRow(
					"some_table", "category", "int", false, false, nil, false,
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"index_name",
					"seq_in_index",
					"column_name",
					"index_type",
					"is_unique",
					"is_nullable",
				}).AddRow(
					"some_table", "PK_some_table", 1, "id", "CLUSTERED", true, false,
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"constraint_name",
					"column_name",
					"referenced_table_name",
					"referenced_column_name",
				}).AddRow(
					"some_table", "fk_name", "category", "categories", "id",
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

		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true},{"name":"category","type":"int","not_null":true}],"indexes":[{"name":"PK_some_table","type":"CLUSTERED","columns":["id"],"unique":true,"nullable":false}],"foreign_keys":[{"name":"fk_name","column_name":"category","referenced_table_name":"categories","referenced_column_name":"id"}]}`))

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, `level="info" database="some_db" schema="dbo" table="some_table"`, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[1].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" database="some_db" schema="dbo" table="some_table" table_spec="%s"`, expectedTableSpec), entries[1].Line)
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
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"TABLE_CATALOG",
					"TABLE_SCHEMA",
					"TABLE_NAME",
					"TABLE_TYPE",
				}).AddRow(
					"some_db",
					"dbo",
					"some_view",
					"VIEW",
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{"some_view"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"column_name",
					"column_type",
					"is_nullable",
					"is_identity",
					"default_value",
					"is_primary_key",
				}).AddRow(
					"some_view", "id", "int", true, false, nil, false,
				),
			)

		mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{"some_view"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name",
				"index_type", "is_unique", "is_nullable",
			}))

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{"some_view"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

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

		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int"}]}`))

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, `level="info" database="some_db" schema="dbo" table="some_view"`, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[1].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" database="some_db" schema="dbo" table="some_view" table_spec="%s"`, expectedTableSpec), entries[1].Line)
	})

	t.Run("detect table with multi-column index", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
				}).AddRow("some_db", "dbo", "some_table", "BASE TABLE"),
			)

		mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name", "column_name", "column_type", "is_nullable",
					"is_identity", "default_value", "is_primary_key",
				}).
					AddRow("some_table", "id", "int", false, false, nil, true).
					AddRow("some_table", "name", "varchar", true, false, nil, false),
			)

		mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name", "index_name", "seq_in_index", "column_name",
					"index_type", "is_unique", "is_nullable",
				}).
					AddRow("some_table", "PK_some_table", 1, "id", "CLUSTERED", true, false).
					AddRow("some_table", "idx_name", 1, "id", "NONCLUSTERED", false, false).
					AddRow("some_table", "idx_name", 2, "name", "NONCLUSTERED", false, true),
			)

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

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

		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"primary_key":true},{"name":"name","type":"varchar"}],"indexes":[{"name":"PK_some_table","type":"CLUSTERED","columns":["id"],"unique":true,"nullable":false},{"name":"idx_name","type":"NONCLUSTERED","columns":["id","name"],"unique":false,"nullable":true}]}`))

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[1].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" database="some_db" schema="dbo" table="some_table" table_spec="%s"`, expectedTableSpec), entries[1].Line)
	})

	t.Run("detect tables across multiple schemas in one bulk query", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
				}).
					AddRow("some_db", "schema_a", "table_a", "BASE TABLE").
					AddRow("some_db", "schema_b", "table_b", "BASE TABLE"),
			)

		// Per-schema follow-ups for both schemas.
		for _, tc := range []struct{ schema, table string }{
			{"schema_a", "table_a"},
			{"schema_b", "table_b"},
		} {
			mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{tc.table}))).WithArgs(tc.schema).RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"table_name", "column_name", "column_type", "is_nullable",
						"is_identity", "default_value", "is_primary_key",
					}).AddRow(tc.table, "id", "int", false, false, nil, true),
				)

			mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{tc.table}))).WithArgs(tc.schema).RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"table_name", "index_name", "seq_in_index", "column_name",
					"index_type", "is_unique", "is_nullable",
				}))

			mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{tc.table}))).WithArgs(tc.schema).RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"table_name", "constraint_name", "column_name",
					"referenced_table_name", "referenced_column_name",
				}))
		}

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 4
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, `level="info" database="some_db" schema="schema_a" table="table_a"`, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[1].Labels)
		require.Equal(t, `level="info" database="some_db" schema="schema_b" table="table_b"`, entries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[2].Labels)
		require.Contains(t, entries[2].Line, `schema="schema_a" table="table_a"`)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[3].Labels)
		require.Contains(t, entries[3].Line, `schema="schema_b" table="table_b"`)
	})

	t.Run("second scrape within emit_interval emits OP_TABLE_DETECTION but not OP_CREATE_STATEMENT", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		fakeNow := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:           db,
			EntryHandler: lokiClient,
			Logger:       util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)
		collector.now = func() time.Time { return fakeNow }

		// First scrape: tables list + metadata queries.
		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
			}).AddRow("some_db", "dbo", "some_table", "BASE TABLE"))

		mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "column_name", "column_type", "is_nullable",
				"is_identity", "default_value", "is_primary_key",
			}).AddRow("some_table", "id", "int", false, true, nil, true))

		mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name",
				"index_type", "is_unique", "is_nullable",
			}).AddRow("some_table", "PK_some_table", 1, "id", "CLUSTERED", true, false))

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

		// Second scrape: only the tables list. The scrape is throttled (still
		// within emitInterval) so it must not trigger any metadata queries.
		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
			}).AddRow("some_db", "dbo", "some_table", "BASE TABLE"))

		require.NoError(t, collector.extractSchema(t.Context()))
		fakeNow = fakeNow.Add(time.Minute) // well within emitInterval
		require.NoError(t, collector.extractSchema(t.Context()))

		// First scrape emits OP_TABLE_DETECTION + OP_CREATE_STATEMENT; second
		// scrape emits only OP_TABLE_DETECTION (still emitted on every scrape).
		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[1].Labels)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[2].Labels)
	})

	t.Run("second scrape after emit_interval re-emits OP_CREATE_STATEMENT", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		fakeNow := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)
		collector.now = func() time.Time { return fakeNow }

		// Two scrapes' worth of expectations.
		for i := 0; i < 2; i++ {
			expectListDatabases(mock, "some_db")
			expectUseDatabase(mock, "some_db")

			mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
				}).AddRow("some_db", "dbo", "some_table", "BASE TABLE"))

			mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"table_name", "column_name", "column_type", "is_nullable",
					"is_identity", "default_value", "is_primary_key",
				}).AddRow("some_table", "id", "int", false, true, nil, true))

			mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"table_name", "index_name", "seq_in_index", "column_name",
					"index_type", "is_unique", "is_nullable",
				}).AddRow("some_table", "PK_some_table", 1, "id", "CLUSTERED", true, false))

			mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"table_name", "constraint_name", "column_name",
					"referenced_table_name", "referenced_column_name",
				}))
		}

		require.NoError(t, collector.extractSchema(t.Context()))
		fakeNow = fakeNow.Add(emitInterval + time.Minute) // past the throttle window
		require.NoError(t, collector.extractSchema(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 4
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[1].Labels)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[2].Labels)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[3].Labels)
	})

	t.Run("table dropped between scrapes is removed from throttle map", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		fakeNow := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)
		collector.now = func() time.Time { return fakeNow }

		// First scrape: two tables in the same schema.
		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
			}).
				AddRow("some_db", "dbo", "table_a", "BASE TABLE").
				AddRow("some_db", "dbo", "table_b", "BASE TABLE"))

		mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{"table_a", "table_b"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "column_name", "column_type", "is_nullable",
				"is_identity", "default_value", "is_primary_key",
			}).
				AddRow("table_a", "id", "int", false, true, nil, true).
				AddRow("table_b", "id", "int", false, true, nil, true))

		mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{"table_a", "table_b"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name",
				"index_type", "is_unique", "is_nullable",
			}))

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{"table_a", "table_b"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

		require.NoError(t, collector.extractSchema(t.Context()))
		require.Contains(t, collector.lastEmittedAt, "some_db")
		require.Contains(t, collector.lastEmittedAt["some_db"], schemaTableKey("dbo", "table_a"))
		require.Contains(t, collector.lastEmittedAt["some_db"], schemaTableKey("dbo", "table_b"))

		// Second scrape: only table_a remains. table_b should be evicted from
		// the throttle map. Since table_a is still within emitInterval, no
		// metadata queries are expected.
		fakeNow = fakeNow.Add(time.Minute)
		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")
		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
			}).AddRow("some_db", "dbo", "table_a", "BASE TABLE"))

		require.NoError(t, collector.extractSchema(t.Context()))

		require.NoError(t, mock.ExpectationsWereMet())
		require.Contains(t, collector.lastEmittedAt, "some_db")
		require.Contains(t, collector.lastEmittedAt["some_db"], schemaTableKey("dbo", "table_a"))
		require.NotContains(t, collector.lastEmittedAt["some_db"], schemaTableKey("dbo", "table_b"))
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
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
			}).AddRow("some_db", "dbo", "some_table", "BASE TABLE"))

		// All three per-schema queries return empty result sets, simulating a
		// table that was dropped between the tables query and the metadata
		// queries. The table is skipped: only OP_TABLE_DETECTION is emitted
		// and no OP_CREATE_STATEMENT follows.
		mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "column_name", "column_type", "is_nullable",
				"is_identity", "default_value", "is_primary_key",
			}))
		mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name",
				"index_type", "is_unique", "is_nullable",
			}))
		mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

		// No OP_CREATE_STATEMENT means no loki-side sync point; drive
		// extractSchema synchronously to avoid racing the collector loop.
		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()

		entries := lokiClient.Received()
		require.Len(t, entries, 1)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, `level="info" database="some_db" schema="dbo" table="some_table"`, entries[0].Line)
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
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		// Pre-populate the throttle map to simulate a table that was emitted
		// in a previous scrape but no longer exists in INFORMATION_SCHEMA.TABLES.
		collector.lastEmittedAt["some_db"] = map[string]time.Time{schemaTableKey("dbo", "stale_table"): time.Now()}

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
			}))

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())
		require.Empty(t, collector.lastEmittedAt)
	})

	t.Run("no tables detected", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		collector, err := NewSchemaDetails(SchemaDetailsArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"TABLE_CATALOG",
				"TABLE_SCHEMA",
				"TABLE_NAME",
				"TABLE_TYPE",
			}),
		)

		require.NoError(t, collector.extractSchema(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())
		require.Empty(t, lokiClient.Received())
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
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"TABLE_CATALOG",
				"TABLE_SCHEMA",
				"TABLE_NAME",
				"TABLE_TYPE",
			}).AddRow(
				"some_db",
				"dbo",
				"some_table",
				"BASE TABLE",
			).AddRow(
				"some_db",
				"dbo",
				"another_table",
				"BASE TABLE",
			).RowError(1, fmt.Errorf("rs error")),
		)

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

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, `level="info" database="some_db" schema="dbo" table="some_table"`, entries[0].Line)
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
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		// First scrape: discovery + USE + tables query fails. The collector
		// logs the failure and continues; the second scrape succeeds.
		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")
		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")
		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"TABLE_CATALOG",
				"TABLE_SCHEMA",
				"TABLE_NAME",
				"TABLE_TYPE",
			}).AddRow(
				"some_db",
				"dbo",
				"some_table",
				"BASE TABLE",
			),
		)

		mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "column_name", "column_type", "is_nullable",
				"is_identity", "default_value", "is_primary_key",
			}).AddRow("some_table", "id", "int", false, true, nil, true))

		mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name",
				"index_type", "is_unique", "is_nullable",
			}).AddRow("some_table", "PK_some_table", 1, "id", "CLUSTERED", true, false))

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{"some_table"}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))

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

		expectedTableSpec := base64.StdEncoding.EncodeToString([]byte(`{"columns":[{"name":"id","type":"int","not_null":true,"auto_increment":true,"primary_key":true}],"indexes":[{"name":"PK_some_table","type":"CLUSTERED","columns":["id"],"unique":true,"nullable":false}]}`))

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, `level="info" database="some_db" schema="dbo" table="some_table"`, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[1].Labels)
		require.Equal(t, fmt.Sprintf(`level="info" database="some_db" schema="dbo" table="some_table" table_spec="%s"`, expectedTableSpec), entries[1].Line)
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
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	expectListDatabases(mock, "some_db")
	expectUseDatabase(mock, "some_db")

	mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, buildExcludedSchemasClause([]string{"excluded_schema"}))).
		WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"TABLE_CATALOG",
		"TABLE_SCHEMA",
		"TABLE_NAME",
		"TABLE_TYPE",
	}))

	require.NoError(t, c.extractSchema(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchemaDetailsExcludeDatabases(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewSchemaDetails(SchemaDetailsArguments{
		DB:               db,
		CollectInterval:  time.Millisecond,
		ExcludeDatabases: []string{"excluded_db"},
		EntryHandler:     lokiClient,
		Logger:           util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	// The user-supplied exclude_db is merged with the hardcoded system DBs.
	// Discovery returns no databases; extractSchema returns cleanly and no
	// USE / tables queries follow.
	mock.ExpectQuery(fmt.Sprintf(selectDatabasesTemplate, buildExcludedDatabasesClause([]string{"excluded_db"}))).
		WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"name", "quoted_name"}))

	require.NoError(t, c.extractSchema(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaDetailsMultiDatabase exercises the full multi-database iteration
// path: one discovery, N USE statements, and per-database bulk metadata.
func TestSchemaDetailsMultiDatabase(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	fakeNow := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	c, err := NewSchemaDetails(SchemaDetailsArguments{
		DB:              db,
		CollectInterval: time.Hour,
		EntryHandler:    lokiClient,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)
	c.now = func() time.Time { return fakeNow }

	// Discovery returns two databases; loop USEs each in turn and runs the
	// tables + per-schema bulk queries against the pinned session. USE is
	// interleaved with each database's queries because the collector switches
	// context and immediately queries before moving to the next database.
	expectListDatabases(mock, "db_a", "db_b")

	for _, dbName := range []string{"db_a", "db_b"} {
		expectUseDatabase(mock, dbName)
		tableName := "t_" + dbName
		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
			}).AddRow(dbName, "dbo", tableName, "BASE TABLE"))

		mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{tableName}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "column_name", "column_type", "is_nullable",
				"is_identity", "default_value", "is_primary_key",
			}).AddRow(tableName, "id", "int", false, true, nil, true))

		mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{tableName}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "index_name", "seq_in_index", "column_name",
				"index_type", "is_unique", "is_nullable",
			}))

		mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{tableName}))).WithArgs("dbo").RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{
				"table_name", "constraint_name", "column_name",
				"referenced_table_name", "referenced_column_name",
			}))
	}

	require.NoError(t, c.extractSchema(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())

	// Each database contributes an OP_TABLE_DETECTION + OP_CREATE_STATEMENT.
	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) == 4
	}, 5*time.Second, 10*time.Millisecond)

	entries := lokiClient.Received()
	require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
	require.Contains(t, entries[0].Line, `database="db_a"`)
	require.Contains(t, entries[0].Line, `table="t_db_a"`)
	require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[1].Labels)
	require.Contains(t, entries[1].Line, `database="db_a"`)
	require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[2].Labels)
	require.Contains(t, entries[2].Line, `database="db_b"`)
	require.Contains(t, entries[2].Line, `table="t_db_b"`)
	require.Equal(t, model.LabelSet{"op": OP_CREATE_STATEMENT}, entries[3].Labels)
	require.Contains(t, entries[3].Line, `database="db_b"`)

	// Throttle keys are qualified with the database, so same-named tables in
	// different databases would coexist.
	require.Contains(t, c.lastEmittedAt, "db_a")
	require.Contains(t, c.lastEmittedAt["db_a"], schemaTableKey("dbo", "t_db_a"))
	require.Contains(t, c.lastEmittedAt, "db_b")
	require.Contains(t, c.lastEmittedAt["db_b"], schemaTableKey("dbo", "t_db_b"))
}

// TestSchemaDetailsMultiDatabase_UseFailureContinues verifies that a USE
// failure for one database does not abort the whole cycle.
func TestSchemaDetailsMultiDatabase_UseFailureContinues(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewSchemaDetails(SchemaDetailsArguments{
		DB:              db,
		CollectInterval: time.Hour,
		EntryHandler:    lokiClient,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	mock.ExpectQuery(fmt.Sprintf(selectDatabasesTemplate, databasesExclusionClause)).
		WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"name", "quoted_name"}).
			AddRow("bad_db", "[bad_db]").
			AddRow("good_db", "[good_db]"))

	// First USE fails; the second one succeeds and the loop continues.
	mock.ExpectExec("USE [bad_db]").WillReturnError(fmt.Errorf("cannot open database"))
	mock.ExpectExec("USE [good_db]").WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{
			"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
		}).AddRow("good_db", "dbo", "ok_table", "BASE TABLE"))

	mock.ExpectQuery(fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause([]string{"ok_table"}))).WithArgs("dbo").RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{
			"table_name", "column_name", "column_type", "is_nullable",
			"is_identity", "default_value", "is_primary_key",
		}).AddRow("ok_table", "id", "int", false, true, nil, true))

	mock.ExpectQuery(fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause([]string{"ok_table"}))).WithArgs("dbo").RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{
			"table_name", "index_name", "seq_in_index", "column_name",
			"index_type", "is_unique", "is_nullable",
		}))

	mock.ExpectQuery(fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause([]string{"ok_table"}))).WithArgs("dbo").RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{
			"table_name", "constraint_name", "column_name",
			"referenced_table_name", "referenced_column_name",
		}))

	require.NoError(t, c.extractSchema(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) == 2
	}, 5*time.Second, 10*time.Millisecond)

	entries := lokiClient.Received()
	require.Contains(t, entries[0].Line, `database="good_db"`)
	require.Contains(t, entries[1].Line, `database="good_db"`)
}

// TestSchemaDetailsUseFailurePreservesThrottle verifies that a USE failure
// for one database does not evict its throttle entries. Without this
// guarantee a flapping database would re-emit OP_CREATE_STATEMENT on every
// recovery, defeating emitInterval.
func TestSchemaDetailsUseFailurePreservesThrottle(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewSchemaDetails(SchemaDetailsArguments{
		DB:              db,
		CollectInterval: time.Hour,
		EntryHandler:    lokiClient,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	// Seed db_b with a recent throttle entry as if a previous cycle had
	// already emitted for it. The USE failure this cycle must not evict it.
	seededAt := time.Now()
	c.lastEmittedAt["db_b"] = map[string]time.Time{
		schemaTableKey("dbo", "existing_table"): seededAt,
	}

	expectListDatabases(mock, "db_a", "db_b")

	// db_a: USE succeeds; tables query returns no rows (nothing to do).
	expectUseDatabase(mock, "db_a")
	mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{
			"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
		}))

	// db_b: USE fails. No tables query should follow.
	mock.ExpectExec("USE [db_b]").WillReturnError(fmt.Errorf("cannot open database"))

	require.NoError(t, c.extractSchema(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Contains(t, c.lastEmittedAt, "db_b")
	require.Equal(t, seededAt, c.lastEmittedAt["db_b"][schemaTableKey("dbo", "existing_table")])
}

// TestSchemaDetailsMidScanBreakPreservesThrottle verifies that a scan error
// on a tables-result row (partial view) does not prune the database's
// throttle map.
func TestSchemaDetailsMidScanBreakPreservesThrottle(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewSchemaDetails(SchemaDetailsArguments{
		DB:              db,
		CollectInterval: time.Hour,
		EntryHandler:    lokiClient,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	seededAt := time.Now()
	c.lastEmittedAt["some_db"] = map[string]time.Time{
		schemaTableKey("dbo", "existing_table"): seededAt,
	}

	expectListDatabases(mock, "some_db")
	expectUseDatabase(mock, "some_db")

	// A NULL TABLE_NAME triggers a scan error when scanning into a plain
	// string, which the collector treats as an incomplete scan and skips
	// pruning for this cycle.
	mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{
			"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
		}).AddRow("some_db", "dbo", nil, "BASE TABLE"))

	require.NoError(t, c.extractSchema(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Contains(t, c.lastEmittedAt, "some_db")
	require.Equal(t, seededAt, c.lastEmittedAt["some_db"][schemaTableKey("dbo", "existing_table")])
}

// TestSchemaDetailsUndiscoveredDatabaseEvicted verifies that a database no
// longer returned by discovery has its throttle entries dropped, so we do
// not leak state for dropped or newly-excluded databases.
func TestSchemaDetailsUndiscoveredDatabaseEvicted(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewSchemaDetails(SchemaDetailsArguments{
		DB:              db,
		CollectInterval: time.Hour,
		EntryHandler:    lokiClient,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	c.lastEmittedAt["gone_db"] = map[string]time.Time{
		schemaTableKey("dbo", "old_table"): time.Now(),
	}

	expectListDatabases(mock, "some_db")
	expectUseDatabase(mock, "some_db")
	mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{
			"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE",
		}))

	require.NoError(t, c.extractSchema(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())

	require.NotContains(t, c.lastEmittedAt, "gone_db")
}

func expectListDatabases(mock sqlmock.Sqlmock, databases ...string) {
	rows := sqlmock.NewRows([]string{"name", "quoted_name"})
	for _, db := range databases {
		rows.AddRow(db, "["+db+"]")
	}
	mock.ExpectQuery(fmt.Sprintf(selectDatabasesTemplate, databasesExclusionClause)).
		WithoutArgs().RowsWillBeClosed().WillReturnRows(rows)
}

func expectUseDatabase(mock sqlmock.Sqlmock, dbName string) {
	mock.ExpectExec(fmt.Sprintf("USE [%s]", dbName)).WillReturnResult(sqlmock.NewResult(0, 0))
}
