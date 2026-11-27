package collector

import (
	"context"
	"database/sql"
	"database/sql/driver"
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
	mockSelectQueryStoreStateWithReplicaRoles(mock, state, false)
}

func mockSelectQueryStoreStateWithReplicaRoles(mock sqlmock.Sqlmock, state string, replicaRolesSupported bool) {
	mock.ExpectQuery(selectQueryStoreState).WillReturnRows(
		sqlmock.NewRows([]string{"database_name", "actual_state_desc", "query_capture_mode_desc", "readonly_reason", "replica_roles_supported"}).
			AddRow(testDatabase, state, "ALL", int64(0), replicaRolesSupported),
	)
}

func mockReplicaProperties(mock sqlmock.Sqlmock, engineEdition int, updateability string) {
	mockReplicaPropertiesWithEdition(mock, engineEdition, "GeneralPurpose", updateability)
}

func mockReplicaPropertiesWithEdition(mock sqlmock.Sqlmock, engineEdition int, edition, updateability string) {
	mock.ExpectQuery(selectDatabaseReplicaProperties).WillReturnRows(
		sqlmock.NewRows([]string{"", "", ""}).AddRow(engineEdition, updateability, edition),
	)
}

// mockHyperscaleReplicaRole mocks the connected Hyperscale replica's role. A
// nil parentRole means the replica has no parent.
func mockHyperscaleReplicaRole(mock sqlmock.Sqlmock, role int, parentRole any) {
	mock.ExpectQuery(selectHyperscaleReplicaRole).WillReturnRows(
		sqlmock.NewRows([]string{"", ""}).AddRow(role, parentRole),
	)
}

func mockGeoSecondaryLinks(mock sqlmock.Sqlmock, links int) {
	mock.ExpectQuery(selectGeoSecondaryLinks).WillReturnRows(
		sqlmock.NewRows([]string{""}).AddRow(links),
	)
}

func mockQueryMetricsRows(executions, errorExecutions int64, durationMicroseconds float64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"query_hash", "executions", "errors", "total_duration_us"}).
		AddRow(queryHashBytes, executions, errorExecutions, durationMicroseconds)
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
			WithArgs(sql.Named("limit", 50), sql.Named("lookback_window", 3600), sql.Named("exclude_current_user", false), sql.Named("collector_marker", QueryMarker)).
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

func queryMetricsArgs(extra ...any) []driver.Value {
	args := []driver.Value{sql.Named("limit", 50), sql.Named("lookback_window", 3600), sql.Named("exclude_current_user", false), sql.Named("collector_marker", QueryMarker)}
	for _, a := range extra {
		args = append(args, a)
	}
	return args
}

func TestQueryMetrics_ReadsConnectedReplicaRole(t *testing.T) {
	defer goleak.VerifyNone(t)

	testCases := []struct {
		name string
		// expectPreflight mocks the queries that run before the metrics query
		// on every collection.
		expectPreflight func(mock sqlmock.Sqlmock)
		query           string
		args            []driver.Value
	}{
		{
			name: "primary with replica roles reads primary rows only",
			expectPreflight: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreStateWithReplicaRoles(mock, "READ_WRITE", true)
			},
			query: selectRoleQueryMetrics,
			args:  queryMetricsArgs(sql.Named("role_type", 1)),
		},
		{
			name: "primary without replica roles reads every row",
			expectPreflight: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreStateWithReplicaRoles(mock, "READ_WRITE", false)
			},
			query: selectQueryMetrics,
			args:  queryMetricsArgs(),
		},
		{
			name: "azure geo-secondary reads geo-secondary rows only",
			expectPreflight: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreStateWithReplicaRoles(mock, "READ_CAPTURE_SECONDARY", true)
				mockReplicaProperties(mock, azureSQLDatabaseEngineEdition, "READ_ONLY")
				mockGeoSecondaryLinks(mock, 1)
			},
			query: selectRoleQueryMetrics,
			args:  queryMetricsArgs(sql.Named("role_type", 3)),
		},
		{
			name: "azure high-availability read replica reads secondary rows only",
			expectPreflight: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreStateWithReplicaRoles(mock, "READ_CAPTURE_SECONDARY", true)
				mockReplicaProperties(mock, azureSQLDatabaseEngineEdition, "READ_ONLY")
				mockGeoSecondaryLinks(mock, 0)
			},
			query: selectRoleQueryMetrics,
			args:  queryMetricsArgs(sql.Named("role_type", 2)),
		},
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

			tc.expectPreflight(mock)
			mock.ExpectQuery(tc.query).WithArgs(tc.args...).WillReturnRows(mockQueryMetricsRows(10, 2, 5_000_000))
			tc.expectPreflight(mock)
			mock.ExpectQuery(tc.query).WithArgs(tc.args...).WillReturnRows(mockQueryMetricsRows(14, 3, 8_500_000))

			require.NoError(t, c.collect(context.Background()))
			require.NoError(t, c.collect(context.Background()))
			require.Equal(t, 4.0, counterValue(t, c.executionsMetric))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestQueryMetrics_ReplicaRoleSupportSelectsStatementOnEveryCollection(t *testing.T) {
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

	poll := func(supported bool, query string, args []driver.Value, executions int64) {
		t.Helper()
		mockSelectQueryStoreStateWithReplicaRoles(mock, "READ_WRITE", supported)
		mock.ExpectQuery(query).WithArgs(args...).WillReturnRows(mockQueryMetricsRows(executions, 0, 0))
		require.NoError(t, c.collect(context.Background()))
	}

	// Without replica role support, every runtime_stats row is read.
	poll(false, selectQueryMetrics, queryMetricsArgs(), 10)
	poll(false, selectQueryMetrics, queryMetricsArgs(), 14)
	require.Equal(t, 4.0, counterValue(t, c.executionsMetric))

	// The connection moves to a server that records replica roles, e.g. after a
	// failover during a rolling upgrade: the next collection reads only the
	// primary's rows. Their lower total re-baselines the series rather than
	// decreasing the counter.
	primaryArgs := queryMetricsArgs(sql.Named("role_type", replicaRolePrimary))
	poll(true, selectRoleQueryMetrics, primaryArgs, 8)
	require.Equal(t, 4.0, counterValue(t, c.executionsMetric))

	poll(true, selectRoleQueryMetrics, primaryArgs, 11)
	require.Equal(t, 7.0, counterValue(t, c.executionsMetric))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryMetrics_FailoverToPrimaryRebaselines(t *testing.T) {
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

	pollGeoSecondary := func(executions, errorExecutions int64, durationMicroseconds float64) {
		t.Helper()
		mockSelectQueryStoreStateWithReplicaRoles(mock, "READ_CAPTURE_SECONDARY", true)
		mockReplicaProperties(mock, azureSQLDatabaseEngineEdition, "READ_ONLY")
		mockGeoSecondaryLinks(mock, 1)
		mock.ExpectQuery(selectRoleQueryMetrics).
			WithArgs(queryMetricsArgs(sql.Named("role_type", replicaRoleGeoSecondary))...).
			WillReturnRows(mockQueryMetricsRows(executions, errorExecutions, durationMicroseconds))
		require.NoError(t, c.collect(context.Background()))
	}
	pollPrimary := func(executions, errorExecutions int64, durationMicroseconds float64) {
		t.Helper()
		mockSelectQueryStoreStateWithReplicaRoles(mock, "READ_WRITE", true)
		mock.ExpectQuery(selectRoleQueryMetrics).
			WithArgs(queryMetricsArgs(sql.Named("role_type", replicaRolePrimary))...).
			WillReturnRows(mockQueryMetricsRows(executions, errorExecutions, durationMicroseconds))
		require.NoError(t, c.collect(context.Background()))
	}

	pollGeoSecondary(100, 1, 50_000_000)
	pollGeoSecondary(105, 2, 52_000_000)
	require.Equal(t, 5.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 1.0, counterValue(t, c.errorsMetric))
	require.InDelta(t, 2.0, counterValue(t, c.durationMetric), 1e-9)

	// A geo-failover makes the connected database the primary. The primary
	// role's totals are higher, but they include executions the previous
	// primary ran, so the series re-baselines instead of adding the difference.
	pollPrimary(10_000, 40, 9_000_000_000)
	require.Equal(t, 5.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 1.0, counterValue(t, c.errorsMetric))
	require.InDelta(t, 2.0, counterValue(t, c.durationMetric), 1e-9)

	// Growth after the failover is measured from the new baseline.
	pollPrimary(10_012, 41, 9_006_000_000)
	require.Equal(t, 17.0, counterValue(t, c.executionsMetric))
	require.Equal(t, 2.0, counterValue(t, c.errorsMetric))
	require.InDelta(t, 8.0, counterValue(t, c.durationMetric), 1e-9)

	require.NoError(t, mock.ExpectationsWereMet())
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
		WithArgs(sql.Named("limit", 50), sql.Named("lookback_window", 3600), sql.Named("exclude_current_user", false), sql.Named("collector_marker", QueryMarker)).
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
