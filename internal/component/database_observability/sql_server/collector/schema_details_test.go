package collector

import (
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
)

func TestSchemaDetails(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("detect tables", func(t *testing.T) {
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
				).AddRow(
					"some_db",
					"dbo",
					"some_view",
					"VIEW",
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

		entries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[0].Labels)
		require.Equal(t, `level="info" database="some_db" schema="dbo" table="some_table"`, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_TABLE_DETECTION}, entries[1].Labels)
		require.Equal(t, `level="info" database="some_db" schema="dbo" table="some_view"`, entries[1].Line)
	})

	t.Run("no tables detected", func(t *testing.T) {
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

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"TABLE_CATALOG",
				"TABLE_SCHEMA",
				"TABLE_NAME",
				"TABLE_TYPE",
			}),
		)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

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

		mock.ExpectQuery(fmt.Sprintf(selectTablesTemplate, exclusionClause)).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

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
