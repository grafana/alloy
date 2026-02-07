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
			return len(lokiClient.Received()) >= 3
		}, 5*time.Second, 10*time.Millisecond)

		collector.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.GreaterOrEqual(t, len(lokiEntries), 3)

		for _, entry := range lokiEntries[:3] {
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
			expectedValue    string
		}{
			{
				name:             "missing PROCESS and REPLICATION CLIENT grants",
				failingCheckName: "RequiredGrantsPresent",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SHOW GRANTS`).
						WillReturnRows(
							sqlmock.NewRows([]string{"Grants"}).
								AddRow("GRANT SELECT, SHOW VIEW ON *.* TO 'user'@'host'"),
						)
				},
				expectedResult: `result="false"`,
				expectedValue:  `value="missing grants: PROCESS, REPLICATION CLIENT"`,
			},
			{
				name:             "missing SELECT on performance_schema",
				failingCheckName: "RequiredGrantsPresent",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SHOW GRANTS`).
						WillReturnRows(
							sqlmock.NewRows([]string{"Grants"}).
								AddRow("GRANT PROCESS, REPLICATION CLIENT, SHOW VIEW ON *.* TO 'user'@'host'").
								AddRow("GRANT SELECT ON cars.* TO 'user'@'host'"),
						)
				},
				expectedResult: `result="false"`,
				expectedValue:  `value="missing grants: SELECT on performance_schema.*"`,
			},
			{
				name:             "missing SELECT and SHOW VIEW grants",
				failingCheckName: "RequiredGrantsPresent",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SHOW GRANTS`).
						WillReturnRows(
							sqlmock.NewRows([]string{"Grants"}).
								AddRow("GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'user'@'host'").
								AddRow("GRANT SELECT ON cars.* TO 'user'@'host'"),
						)
				},
				expectedResult: `result="false"`,
				expectedValue:  `value="missing grants: SELECT on performance_schema.*, SHOW VIEW"`,
			},
			{
				name:             "no rows in events statements digest",
				failingCheckName: "PerformanceSchemaHasRows",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest`).
						WillReturnRows(
							sqlmock.NewRows([]string{"COUNT(*)"}).
								AddRow(0),
						)
				},
				expectedResult: `result="false"`,
				expectedValue:  "",
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
					return len(lokiClient.Received()) >= 3
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
						if tc.expectedValue != "" {
							require.Contains(t, entry.Line, tc.expectedValue)
						}
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
			name: "RequiredGrantsPresent",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SHOW GRANTS`).
					WillReturnRows(
						sqlmock.NewRows([]string{"Grants"}).
							AddRow("GRANT PROCESS, REPLICATION CLIENT, SELECT, SHOW VIEW ON *.* TO 'user'@'host'"),
					)
			},
		},
		{
			name: "PerformanceSchemaHasRows",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest`).
					WillReturnRows(
						sqlmock.NewRows([]string{"COUNT(*)"}).
							AddRow(100),
					)
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
