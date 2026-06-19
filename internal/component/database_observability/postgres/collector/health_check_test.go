package collector

import (
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
)

func TestHealthCheck(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("all checks pass", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(
			sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual),
			sqlmock.MonitorPingsOption(true),
		)
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewHealthCheck(HealthCheckArguments{
			DB:              db,
			CollectInterval: 100 * time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		// Setup all checks to pass (no custom expectation)
		setupExpectQueryAssertions("", mock, nil)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 6
		}, 5*time.Second, 10*time.Millisecond)

		collector.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.GreaterOrEqual(t, len(lokiEntries), 6)

		for _, entry := range lokiEntries[:6] {
			require.Equal(t, model.LabelSet{"op": OP_HEALTH_STATUS}, entry.Labels)
			require.Contains(t, entry.Line, `result="true"`)
		}
	})

	t.Run("individual check failures", func(t *testing.T) {
		testCases := []struct {
			name             string
			failingCheckName string
			customSetup      func(mock sqlmock.Sqlmock)
			expectedResult   string
		}{
			{
				name:             "pg_stat_statements not installed",
				failingCheckName: "PgStatStatementsEnabled",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(pgStatStatementsEnabledQuery).
						WillReturnRows(sqlmock.NewRows([]string{"oid", "extname", "extowner", "extnamespace", "extrelocatable", "extversion"}))
				},
				expectedResult: `result="false"`,
			},
			{
				name:             "track_activity_query_size too small",
				failingCheckName: "TrackActivityQuerySize",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(trackActivityQuerySizeQuery).
						WillReturnRows(sqlmock.NewRows([]string{"track_activity_query_size"}).
							AddRow("1024"))
				},
				expectedResult: `result="false"`,
			},
			{
				name:             "compute_query_id is off",
				failingCheckName: "ComputeQueryIdEnabled",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(computeQueryIdQuery).
						WillReturnRows(sqlmock.NewRows([]string{"setting"}).
							AddRow("off"))
				},
				expectedResult: `result="false" value="off"`,
			},
			{
				name:             "pg_stat_statements has no usable rows",
				failingCheckName: "PgStatStatementsHasRows",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(pgStatStatementsHasRowsQuery(nil, nil)).
						WillReturnRows(sqlmock.NewRows([]string{"exists"}).
							AddRow(false))
				},
				expectedResult: `result="false"`,
			},
			{
				name:             "monitoring user lacks SELECT on pg_stat_statements",
				failingCheckName: "MonitoringUserPrivileges",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(monitoringUserPrivilegesQuery).
						WillReturnRows(sqlmock.NewRows([]string{
							"has_pg_monitor_role",
							"has_pg_read_all_stats_role",
							"can_select_pg_stat_statements",
							"sees_insufficient_privilege",
						}).AddRow(false, false, false, false))
				},
				expectedResult: `result="false" value="can_select_view=false,has_pg_monitor_role=false,has_pg_read_all_stats_role=false,sees_insufficient_privilege=false"`,
			},
			{
				name:             "monitoring user sees insufficient privilege rows",
				failingCheckName: "MonitoringUserPrivileges",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(monitoringUserPrivilegesQuery).
						WillReturnRows(sqlmock.NewRows([]string{
							"has_pg_monitor_role",
							"has_pg_read_all_stats_role",
							"can_select_pg_stat_statements",
							"sees_insufficient_privilege",
						}).AddRow(false, false, true, true))
				},
				expectedResult: `result="false" value="can_select_view=true,has_pg_monitor_role=false,has_pg_read_all_stats_role=false,sees_insufficient_privilege=true"`,
			},
			{
				// Informational: lacking pg_read_all_stats role membership alone
				// must NOT fail the check when SELECT works and no masking is
				// observed (e.g. direct GRANT SELECT on pg_stat_statements).
				name:             "monitoring user lacks role membership but no masking observed",
				failingCheckName: "MonitoringUserPrivileges",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(monitoringUserPrivilegesQuery).
						WillReturnRows(sqlmock.NewRows([]string{
							"has_pg_monitor_role",
							"has_pg_read_all_stats_role",
							"can_select_pg_stat_statements",
							"sees_insufficient_privilege",
						}).AddRow(false, false, true, false))
				},
				expectedResult: `result="true" value="can_select_view=true,has_pg_monitor_role=false,has_pg_read_all_stats_role=false,sees_insufficient_privilege=false"`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				db, mock, err := sqlmock.New(
					sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual),
					sqlmock.MonitorPingsOption(true),
				)
				require.NoError(t, err)
				defer db.Close()

				lokiClient := loki.NewCollectingHandler()

				collector, err := NewHealthCheck(HealthCheckArguments{
					DB:              db,
					CollectInterval: 100 * time.Millisecond,
					EntryHandler:    lokiClient,
					Logger:          util.TestAlloyLogger(t).Slog(),
				})
				require.NoError(t, err)

				setupExpectQueryAssertions(tc.failingCheckName, mock, tc.customSetup)

				err = collector.Start(t.Context())
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					return len(lokiClient.Received()) == 6
				}, 5*time.Second, 10*time.Millisecond)

				collector.Stop()

				require.Eventually(t, func() bool {
					return collector.Stopped()
				}, 5*time.Second, 10*time.Millisecond)

				lokiClient.Stop()

				err = mock.ExpectationsWereMet()
				require.NoError(t, err)

				lokiEntries := lokiClient.Received()

				found := false
				for _, entry := range lokiEntries {
					if strings.Contains(entry.Line, tc.failingCheckName) {
						require.Equal(t, model.LabelSet{"op": OP_HEALTH_STATUS}, entry.Labels)
						require.Contains(t, entry.Line, tc.expectedResult)
						found = true
						break
					}
				}
				require.True(t, found)
			})
		}
	})

	t.Run("PgStatStatementsHasRows renders excludes into SQL", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(pgStatStatementsEnabledQuery).
			WillReturnRows(sqlmock.NewRows([]string{"oid", "extname", "extowner", "extnamespace", "extrelocatable", "extversion"}).
				AddRow(1, "pg_stat_statements", 10, 11, false, "1.9"))
		mock.ExpectQuery(trackActivityQuerySizeQuery).
			WillReturnRows(sqlmock.NewRows([]string{"setting"}).AddRow("4096"))
		mock.ExpectQuery(computeQueryIdQuery).
			WillReturnRows(sqlmock.NewRows([]string{"setting"}).AddRow("on"))
		mock.ExpectQuery(monitoringUserPrivilegesQuery).
			WillReturnRows(sqlmock.NewRows([]string{
				"has_pg_monitor_role",
				"has_pg_read_all_stats_role",
				"can_select_pg_stat_statements",
				"sees_insufficient_privilege",
			}).AddRow(true, true, true, false))
		mock.ExpectQuery(pgStatStatementsHasRowsQuery([]string{"my_db"}, []string{"my_user"})).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewHealthCheck(HealthCheckArguments{
			DB:               db,
			CollectInterval:  100 * time.Millisecond,
			ExcludeDatabases: []string{"my_db"},
			ExcludeUsers:     []string{"my_user"},
			EntryHandler:     lokiClient,
			Logger:           util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 6
		}, 5*time.Second, 10*time.Millisecond)

		collector.Stop()
		require.Eventually(t, func() bool { return collector.Stopped() }, 5*time.Second, 10*time.Millisecond)
		lokiClient.Stop()

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func setupExpectQueryAssertions(checkName string, mock sqlmock.Sqlmock, customSetup func(mock sqlmock.Sqlmock)) {
	type checkSetup struct {
		name  string
		setup func(mock sqlmock.Sqlmock)
	}

	checks := []checkSetup{
		{
			name: "PgStatStatementsEnabled",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(pgStatStatementsEnabledQuery).
					WillReturnRows(sqlmock.NewRows([]string{"oid", "extname", "extowner", "extnamespace", "extrelocatable", "extversion"}).
						AddRow(1, "pg_stat_statements", 10, 11, false, "1.9"))
			},
		},
		{
			name: "TrackActivityQuerySize",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(trackActivityQuerySizeQuery).
					WillReturnRows(sqlmock.NewRows([]string{"setting"}).
						AddRow("4096"))
			},
		},
		{
			name: "ComputeQueryIdEnabled",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(computeQueryIdQuery).
					WillReturnRows(sqlmock.NewRows([]string{"setting"}).
						AddRow("on"))
			},
		},
		{
			name: "MonitoringUserPrivileges",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(monitoringUserPrivilegesQuery).
					WillReturnRows(sqlmock.NewRows([]string{
						"has_pg_monitor_role",
						"has_pg_read_all_stats_role",
						"can_select_pg_stat_statements",
						"sees_insufficient_privilege",
					}).AddRow(true, true, true, false))
			},
		},
		{
			name: "PgStatStatementsHasRows",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(pgStatStatementsHasRowsQuery(nil, nil)).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).
						AddRow(true))
			},
		},
	}

	for _, check := range checks {
		if check.name == checkName {
			customSetup(mock)
			continue
		}
		check.setup(mock)
	}
}
