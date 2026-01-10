package collector

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
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
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		// Setup all checks to pass (no custom expectation)
		setupExpectQueryAssertions("", mock, nil)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) >= 4
		}, 5*time.Second, 10*time.Millisecond)

		collector.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.GreaterOrEqual(t, len(lokiEntries), 4)

		for _, entry := range lokiEntries[:4] {
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
					mock.ExpectQuery(`SELECT * FROM pg_extension WHERE extname = 'pg_stat_statements'`).
						WillReturnRows(sqlmock.NewRows([]string{"oid", "extname", "extowner", "extnamespace", "extrelocatable", "extversion"}))
				},
				expectedResult: `result="false"`,
			},
			{
				name:             "track_activity_query_size too small",
				failingCheckName: "TrackActivityQuerySize",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SELECT setting FROM pg_settings WHERE name = 'track_activity_query_size'`).
						WillReturnRows(sqlmock.NewRows([]string{"track_activity_query_size"}).
							AddRow("1024"))
				},
				expectedResult: `result="false"`,
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
					Logger:          log.NewLogfmtLogger(os.Stderr),
				})
				require.NoError(t, err)

				setupExpectQueryAssertions(tc.failingCheckName, mock, tc.customSetup)

				err = collector.Start(t.Context())
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					return len(lokiClient.Received()) >= 4
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
				mock.ExpectQuery(`SELECT * FROM pg_extension WHERE extname = 'pg_stat_statements'`).
					WillReturnRows(sqlmock.NewRows([]string{"oid", "extname", "extowner", "extnamespace", "extrelocatable", "extversion"}).
						AddRow(1, "pg_stat_statements", 10, 11, false, "1.9"))
			},
		},
		{
			name: "TrackActivityQuerySize",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT setting FROM pg_settings WHERE name = 'track_activity_query_size'`).
					WillReturnRows(sqlmock.NewRows([]string{"setting"}).
						AddRow("4096"))
			},
		},
		{
			name: "MonitoringUserPrivileges",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT * FROM pg_stat_statements LIMIT 1`).
					WillReturnRows(sqlmock.NewRows([]string{"userid", "dbid", "queryid"}).
						AddRow(1, 1, 123))
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
