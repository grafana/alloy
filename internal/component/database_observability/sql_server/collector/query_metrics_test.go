package collector

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/util"
)

var queryHashBytes = []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}

const (
	testDatabase = "books_store"
	testHash     = "0011223344556677"
)

func mockSelectQueryStoreState(mock sqlmock.Sqlmock, state string) {
	mock.ExpectQuery(selectQueryStoreState).WillReturnRows(
		sqlmock.NewRows([]string{"database_name", "actual_state_desc", "query_capture_mode_desc", "readonly_reason"}).
			AddRow(testDatabase, state, "ALL", int64(0)),
	)
}

func mockQueryMetricsRows(executions, errorExecutions int64, durationMicroseconds float64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"query_hash", "executions", "errors", "total_duration_us"}).
		AddRow(queryHashBytes, executions, errorExecutions, durationMicroseconds)
}

func TestQueryMetrics_CollectDeltas(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	c, err := NewQueryMetrics(QueryMetricsArguments{
		DB:              db,
		Registry:        prometheus.NewRegistry(),
		CollectInterval: time.Minute,
		Limit:           50,
		Lookback:        time.Hour,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	// First poll: first sighting, series created at zero.
	mockSelectQueryStoreState(mock, "READ_WRITE")
	mock.ExpectQuery(selectQueryMetrics).
		WithArgs(sql.Named("limit", 50), sql.Named("lookback_window", 3600)).
		WillReturnRows(mockQueryMetricsRows(10, 2, 5_000_000))
	require.NoError(t, c.collect(context.Background()))
	require.Equal(t, 0.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 0.0, counterValue(t, c.errorsMetric))
	require.Equal(t, 0.0, counterValue(t, c.durationMetric))

	// Second poll: only the increase is emitted.
	mockSelectQueryStoreState(mock, "READ_WRITE")
	mock.ExpectQuery(selectQueryMetrics).
		WithArgs(sql.Named("limit", 50), sql.Named("lookback_window", 3600)).
		WillReturnRows(mockQueryMetricsRows(14, 3, 8_500_000))
	require.NoError(t, c.collect(context.Background()))
	require.Equal(t, 4.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 1.0, counterValue(t, c.errorsMetric))
	require.InDelta(t, 3.5, counterValue(t, c.durationMetric), 1e-9)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryMetrics_CollectAcrossIntervalRollover(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	c, err := NewQueryMetrics(QueryMetricsArguments{
		DB:              db,
		Registry:        prometheus.NewRegistry(),
		CollectInterval: time.Minute,
		Limit:           50,
		Lookback:        time.Hour,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	poll := func(executions, errorExecutions int64, durationMicroseconds float64) {
		t.Helper()
		mockSelectQueryStoreState(mock, "READ_WRITE")
		mock.ExpectQuery(selectQueryMetrics).
			WithArgs(sql.Named("limit", 50), sql.Named("lookback_window", 3600)).
			WillReturnRows(mockQueryMetricsRows(executions, errorExecutions, durationMicroseconds))
		require.NoError(t, c.collect(context.Background()))
	}

	// First sighting: series created at zero, historical totals not emitted.
	poll(10, 2, 5_000_000)
	require.Equal(t, 0.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 0.0, counterValue(t, c.errorsMetric))
	require.Equal(t, 0.0, counterValue(t, c.durationMetric))

	// Interval A closes and B opens; the full-history sum keeps growing, so only
	// the increase is emitted.
	poll(14, 3, 8_500_000)
	require.Equal(t, 4.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 1.0, counterValue(t, c.errorsMetric))
	require.InDelta(t, 3.5, counterValue(t, c.durationMetric), 1e-9)

	// A new Query Store interval opens (rollover). Because the query sums full
	// history, the total keeps growing, so deltas continue to accumulate.
	poll(20, 4, 12_000_000)
	require.Equal(t, 10.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 2.0, counterValue(t, c.errorsMetric))
	require.InDelta(t, 7.0, counterValue(t, c.durationMetric), 1e-9)

	// Retention drops the oldest interval: the full-history sum decreases. The
	// counters must not regress, so nothing is emitted and the series
	// re-baselines to the new, lower totals.
	poll(6, 1, 3_000_000)
	require.Equal(t, 10.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 2.0, counterValue(t, c.errorsMetric))
	require.InDelta(t, 7.0, counterValue(t, c.durationMetric), 1e-9)

	// Growth after the retention drop is measured from the new baseline.
	poll(9, 2, 4_500_000)
	require.Equal(t, 13.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 3.0, counterValue(t, c.errorsMetric))
	require.InDelta(t, 8.5, counterValue(t, c.durationMetric), 1e-9)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryMetrics_QueryStoreStateChecks(t *testing.T) {
	defer goleak.VerifyNone(t)

	testCases := []struct {
		name     string
		state    string
		noRows   bool
		queryErr bool
	}{
		{name: "query store off", state: "OFF"},
		{name: "query store read only", state: "READ_ONLY"},
		{name: "no rows (missing permission)", noRows: true},
		{name: "preflight error", queryErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			c, err := NewQueryMetrics(QueryMetricsArguments{
				DB:              db,
				Registry:        prometheus.NewRegistry(),
				CollectInterval: time.Minute,
				Limit:           50,
				Lookback:        time.Hour,
				Logger:          util.TestAlloyLogger(t).Slog(),
			})
			require.NoError(t, err)

			switch {
			case tc.queryErr:
				mock.ExpectQuery(selectQueryStoreState).WillReturnError(errors.New("permission denied"))
			case tc.noRows:
				mock.ExpectQuery(selectQueryStoreState).WillReturnRows(
					sqlmock.NewRows([]string{"database_name", "actual_state_desc", "query_capture_mode_desc", "readonly_reason"}),
				)
			default:
				mockSelectQueryStoreState(mock, tc.state)
			}

			// Skipped collections must not surface as errors, so the collector stays healthy.
			require.NoError(t, c.collect(context.Background()))
			require.Equal(t, 0, testutil.CollectAndCount(c.executionsMetric))
			require.Equal(t, 0, testutil.CollectAndCount(c.errorsMetric))
			require.Equal(t, 0, testutil.CollectAndCount(c.durationMetric))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestQueryMetrics_StartRegistersAndStopUnregisters(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	reg := prometheus.NewRegistry()
	c, err := NewQueryMetrics(QueryMetricsArguments{
		DB:              db,
		Registry:        reg,
		CollectInterval: time.Minute,
		Limit:           50,
		Lookback:        time.Hour,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	mockSelectQueryStoreState(mock, "READ_WRITE")
	mock.ExpectQuery(selectQueryMetrics).
		WithArgs(sql.Named("limit", 50), sql.Named("lookback_window", 3600)).
		WillReturnRows(mockQueryMetricsRows(10, 2, 5_000_000))

	require.NoError(t, c.Start(context.Background()))

	require.Eventually(t, func() bool {
		n, err := testutil.GatherAndCount(reg, "database_observability_query_executions_total")
		return err == nil && n == 1
	}, 5*time.Second, 20*time.Millisecond)
	require.False(t, c.Stopped())

	c.Stop()
	require.True(t, c.Stopped())

	for _, metricName := range []string{
		"database_observability_query_executions_total",
		"database_observability_query_errors_total",
		"database_observability_query_duration_seconds_total",
	} {
		n, err := testutil.GatherAndCount(reg, metricName)
		require.NoError(t, err)
		require.Equal(t, 0, n, "expected %s to be unregistered after Stop", metricName)
	}

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFormatQueryHash(t *testing.T) {
	t.Parallel()

	hash, err := formatQueryHash(queryHashBytes)
	require.NoError(t, err)
	require.Equal(t, testHash, hash)

	_, err = formatQueryHash([]byte{0x00, 0x11})
	require.Error(t, err)
}
