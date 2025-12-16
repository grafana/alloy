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
			return len(lokiClient.Received()) >= 9
		}, 5*time.Second, 10*time.Millisecond)

		collector.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.GreaterOrEqual(t, len(lokiEntries), 9)

		for _, entry := range lokiEntries[:9] {
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
				name:             "performance schema disabled",
				failingCheckName: "PerformaneSchemaEnabled",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SHOW VARIABLES LIKE 'performance_schema'`).
						WillReturnRows(
							sqlmock.NewRows([]string{"Variable_name", "Value"}).
								AddRow("performance_schema", "OFF"),
						)
				},
				expectedResult: `result="false"`,
			},
			{
				name:             "mysql version too old",
				failingCheckName: "MySQLVersionSupported",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SELECT VERSION()`).
						WillReturnRows(
							sqlmock.NewRows([]string{"VERSION()"}).
								AddRow("5.7.44"),
						)
				},
				expectedResult: `result="false"`,
			},
			{
				name:             "missing grants",
				failingCheckName: "RequiredGrantsPresent",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SHOW GRANTS`).
						WillReturnRows(
							sqlmock.NewRows([]string{"Grants"}).
								AddRow("GRANT SELECT, SHOW VIEW ON *.* TO 'user'@'host'"),
						)
				},
				expectedResult: `result="false"`,
			},
			{
				name:             "digest variables too short",
				failingCheckName: "DigestVariablesLengthCheck",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`
SELECT
	@@performance_schema_max_sql_text_length,
	@@performance_schema_max_digest_length,
	@@max_digest_length`).
						WillReturnRows(
							sqlmock.NewRows([]string{"@@performance_schema_max_sql_text_length", "@@performance_schema_max_digest_length", "@@max_digest_length"}).
								AddRow(1024, 2048, 1024),
						)
				},
				expectedResult: `result="false"`,
			},
			{
				name:             "setup consumer cpu time disabled",
				failingCheckName: "SetupConsumerCPUTimeEnabled",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SELECT enabled FROM performance_schema.setup_consumers WHERE NAME = 'events_statements_cpu'`).
						WillReturnRows(
							sqlmock.NewRows([]string{"enabled"}).
								AddRow("NO"),
						)
				},
				expectedResult: `result="false"`,
			},
			{
				name:             "events waits consumer partially disabled",
				failingCheckName: "SetupConsumersEventsWaitsEnabled",
				customSetup: func(mock sqlmock.Sqlmock) {
					mock.ExpectQuery(`SELECT name, enabled FROM performance_schema.setup_consumers WHERE NAME IN ('events_waits_current','events_waits_history')`).
						WillReturnRows(
							sqlmock.NewRows([]string{"name", "enabled"}).
								AddRow("events_waits_current", "YES").
								AddRow("events_waits_history", "NO"),
						)
				},
				expectedResult: `result="false"`,
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
					return len(lokiClient.Received()) >= 9
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
			name: "DBConnectionValid",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectPing().WillDelayFor(10 * time.Millisecond)
			},
		},
		{
			name: "PerformaneSchemaEnabled",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SHOW VARIABLES LIKE 'performance_schema'`).
					WillReturnRows(
						sqlmock.NewRows([]string{"Variable_name", "Value"}).
							AddRow("performance_schema", "ON"),
					)
			},
		},
		{
			name: "MySQLVersionSupported",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT VERSION()`).
					WillReturnRows(
						sqlmock.NewRows([]string{"VERSION()"}).
							AddRow("8.0.36"),
					)
			},
		},
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
			name: "DigestVariablesLengthCheck",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`
SELECT
	@@performance_schema_max_sql_text_length,
	@@performance_schema_max_digest_length,
	@@max_digest_length`).
					WillReturnRows(
						sqlmock.NewRows([]string{"@@performance_schema_max_sql_text_length", "@@performance_schema_max_digest_length", "@@max_digest_length"}).
							AddRow(4096, 4096, 4096),
					)
			},
		},
		{
			name: "SetupConsumerCPUTimeEnabled",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT enabled FROM performance_schema.setup_consumers WHERE NAME = 'events_statements_cpu'`).
					WillReturnRows(
						sqlmock.NewRows([]string{"enabled"}).
							AddRow("YES"),
					)
			},
		},
		{
			name: "SetupConsumersEventsWaitsEnabled",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT name, enabled FROM performance_schema.setup_consumers WHERE NAME IN ('events_waits_current','events_waits_history')`).
					WillReturnRows(
						sqlmock.NewRows([]string{"name", "enabled"}).
							AddRow("events_waits_current", "YES").
							AddRow("events_waits_history", "YES"),
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
