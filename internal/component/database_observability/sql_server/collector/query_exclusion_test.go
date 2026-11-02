package collector

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
)

func TestQueryMetricsExcludeCurrentUser(t *testing.T) {
	raw, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer raw.Close()

	c, err := NewQueryMetrics(QueryMetricsArguments{
		DB:                 raw,
		Registry:           prometheus.NewRegistry(),
		CollectInterval:    time.Minute,
		Limit:              50,
		Lookback:           time.Hour,
		Logger:             util.TestAlloyLogger(t).Slog(),
		ExcludeCurrentUser: true,
	})
	require.NoError(t, err)

	mockSelectQueryStoreState(mock, "READ_WRITE")
	mock.ExpectQuery(selectQueryMetrics).
		WithArgs(sql.Named("limit", 50), sql.Named("lookback_window", 3600), sql.Named("exclude_current_user", true), sql.Named("collector_marker", QueryMarker)).
		WillReturnRows(mockQueryMetricsRows(10, 0, 100))
	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryCollectorsExcludeCurrentUser(t *testing.T) {
	for _, kind := range []string{"details", "plans", "samples"} {
		t.Run(kind, func(t *testing.T) {
			raw, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer raw.Close()
			db := raw
			logger := util.TestAlloyLogger(t).Slog()
			tracker := trackerFor(testHash)
			var collect func(context.Context) error
			var query string
			var args []any
			var rows *sqlmock.Rows
			switch kind {
			case "details":
				c, e := NewQueryDetails(QueryDetailsArguments{DB: db, Tracker: tracker, Logger: logger, ExcludeCurrentUser: true})
				require.NoError(t, e)
				collect = c.collect
				query, args, err = buildQueryTextStatement([]string{testHash}, true)
				rows = sqlmock.NewRows([]string{"query_hash", "query_sql_text"})
			case "plans":
				handler := loki.NewCollectingHandler()
				t.Cleanup(handler.Stop)
				c, e := NewExplainPlans(ExplainPlansArguments{EntryHandler: handler, DB: db, Tracker: tracker, Logger: logger, ExcludeCurrentUser: true})
				require.NoError(t, e)
				collect = c.collect
				query, args, err = buildExplainPlansStatement([]string{testHash}, true)
				rows = sqlmock.NewRows([]string{"query_hash", "query_plan"})
			case "samples":
				c, e := NewQuerySamples(QuerySamplesArguments{DB: db, Tracker: tracker, Logger: logger, ExcludeCurrentUser: true, ExcludeUsers: []string{"db-o11y"}})
				require.NoError(t, e)
				collect = c.collect
				query, args, err = buildQuerySamplesStatement([]string{testHash}, []string{"db-o11y"}, true)
				rows = querySampleRows()
			}
			require.NoError(t, err)
			mockSelectQueryStoreState(mock, "READ_WRITE")
			mock.ExpectQuery(query).WithArgs(namedArgs(args)...).WillReturnRows(rows).RowsWillBeClosed()
			require.NoError(t, collect(context.Background()))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestServerMetricsExcludeCurrentUser(t *testing.T) {
	t.Run("sql server user sessions", func(t *testing.T) {
		raw, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer raw.Close()

		c, err := NewServerMetrics(ServerMetricsArguments{
			DB:                 raw,
			Registry:           prometheus.NewRegistry(),
			Logger:             util.TestAlloyLogger(t).Slog(),
			EngineEdition:      3,
			ExcludeCurrentUser: true,
		})
		require.NoError(t, err)

		mock.ExpectQuery(selectUserSessions).
			WithArgs(sql.Named("exclude_current_user", true), sql.Named("collector_app_name", AppName)).
			WillReturnRows(sqlmock.NewRows(userSessionColumns).AddRow(1, 2, 0, 0, 0)).
			RowsWillBeClosed()

		ch := make(chan prometheus.Metric, 8)
		c.collectUserSessions(context.Background(), 3, ch)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("azure sql database user sessions", func(t *testing.T) {
		raw, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer raw.Close()

		c, err := NewServerMetrics(ServerMetricsArguments{
			DB:                 raw,
			Registry:           prometheus.NewRegistry(),
			Logger:             util.TestAlloyLogger(t).Slog(),
			EngineEdition:      azureSQLDatabaseEngineEdition,
			ExcludeCurrentUser: true,
		})
		require.NoError(t, err)

		mock.ExpectQuery(selectAzureSQLDatabaseUserSessions).
			WithArgs(sql.Named("exclude_current_user", true), sql.Named("collector_app_name", AppName)).
			WillReturnRows(sqlmock.NewRows(azureSQLDatabaseUserSessionColumns).AddRow(1, 2, 0, 0)).
			RowsWillBeClosed()

		ch := make(chan prometheus.Metric, 8)
		c.collectAzureSQLDatabaseUserSessions(context.Background(), ch)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
