package collector

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
)

func TestQueryExclusionPlacement(t *testing.T) {
	// All three stages must read filtered records, so a shared hash doesn't
	// reintroduce tagged query IDs during historical aggregation.
	require.Equal(t, 1, strings.Count(selectQueryMetrics, "FROM sys.query_store_query q"))
	require.Equal(t, 3, strings.Count(selectQueryMetrics, "FROM filtered_queries q"))
	require.Less(t, strings.Index(selectQueryMetrics, "CHARINDEX"), strings.Index(selectQueryMetrics, "eligible AS"))
	require.Contains(t, selectQueryMetrics, "qt.query_text_id = q.query_text_id")
	require.Contains(t, selectQueryMetrics, "@exclude_current_user = 0 OR CHARINDEX(@collector_marker, qt.query_sql_text) = 0")
	require.NotContains(t, selectQueryMetrics, "query_hash NOT IN")
	require.Contains(t, selectQueryTextTemplate, "CHARINDEX(@collector_marker, qt.query_sql_text) = 0")
	require.Contains(t, selectExplainPlansTemplate, "qt.query_text_id = q.query_text_id")
	require.Less(t, strings.Index(selectExplainPlansTemplate, "CHARINDEX"), strings.Index(selectExplainPlansTemplate, "FROM ranked_plans"))
	require.Contains(t, selectQuerySamplesTemplate, "@exclude_current_user = 0 OR s.program_name IS NULL OR s.program_name <> @collector_app_name")
	require.Less(t, strings.Index(selectQuerySamplesTemplate, "s.program_name <> @collector_app_name"), strings.Index(selectQuerySamplesTemplate, "current_waits AS"))
	require.Contains(t, selectQuerySamplesTemplate, "r.statement_text IS NULL OR CHARINDEX(@collector_marker, r.statement_text) = 0")
	require.NotContains(t, selectQuerySamplesTemplate, "CHARINDEX(@collector_marker, st.text)")
}

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
