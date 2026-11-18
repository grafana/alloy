package collector

import (
	"database/sql/driver"
	"fmt"
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/syncbuffer"
)

var latestCompatibleVersion = semver.MustParse("8.0.32")

func TestQuerySamples(t *testing.T) {
	defer goleak.VerifyNone(t)

	testcases := []struct {
		name       string
		rows       [][]driver.Value
		logsLabels []model.LabelSet
		logsLines  []string
		errorLine  string
	}{
		{
			name: "select query",
			rows: [][]driver.Value{{
				"some_schema",
				"890",
				"123",
				"234",
				"some_digest",
				"select * from some_table where id = ?",
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": database_observability.OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
			},
		},
		{
			name: "select query with traceparent",
			rows: [][]driver.Value{{
				"some_schema",
				"890",
				"123",
				"234",
				"some_digest",
				"select * from some_table where id = 1 /*traceparent='00-00bd5199fe2a4c8506368b55ef212cf1-d49c5e2fb232379b-01'*/",
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": database_observability.OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\" traceparent=\"00-00bd5199fe2a4c8506368b55ef212cf1-d49c5e2fb232379b-01\"",
			},
		},
		{
			name: "SQL_TEXT is NULL",
			rows: [][]driver.Value{{
				"some_schema",
				"890",
				"123",
				"234",
				"some_digest",
				nil,
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": database_observability.OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
			},
		},
		{
			name: "select query with truncated traceparent",
			rows: [][]driver.Value{{
				"some_schema",
				"890",
				"123",
				"234",
				"some_digest",
				"select * from some_table where id = 1 /*traceparent='00-abc...",
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": database_observability.OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
			},
		},
		{
			name: "multiple schemas",
			rows: [][]driver.Value{{
				"some_schema",
				"890",
				"123",
				"234",
				"some_digest",
				"select * from some_table where id = ?",
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}, {
				"some_other_schema",
				"891",
				"124",
				"235",
				"some_digest",
				"select * from some_table where id = ?",
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": database_observability.OP_QUERY_SAMPLE},
				{"op": database_observability.OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
				"level=\"info\" schema=\"some_other_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"891\" event_id=\"124\" end_event_id=\"235\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
			},
		},
		{
			name: "query with error code and sqlstate",
			rows: [][]driver.Value{{
				"some_schema",
				"890",
				"123",
				"234",
				"some_digest",
				"insert into users (email) values (?)",
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"1",     // errors
				"1062",  // mysql_errno
				"23000", // returned_sqlstate
				nil,     // message_text (redaction gating is covered in TestQuerySamples_DisableQueryRedaction)
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": database_observability.OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"1\" mysql_errno=\"1062\" returned_sqlstate=\"23000\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			logBuffer := syncbuffer.Buffer{}
			logger, err := logging.New(&logBuffer, logging.Options{
				Level:  logging.LevelDebug,
				Format: logging.FormatLogfmt,
			})
			require.NoError(t, err)

			lokiClient := loki.NewCollectingHandler()

			collector, err := NewQuerySamples(QuerySamplesArguments{
				DB:              db,
				EngineVersion:   latestCompatibleVersion,
				CollectInterval: time.Second,
				EntryHandler:    lokiClient,
				Logger:          logger.Slog(),
			})
			require.NoError(t, err)
			require.NotNil(t, collector)

			mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"uptime",
					}).AddRow(
						"1", // corresponds to initial timerBookmark
					),
				)
			expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

			mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
				sqlmock.NewRows([]string{
					"now",
					"uptime",
				}).AddRow(
					5,
					1,
				))

			expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
			mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
				1e12, // initial timerBookmark
				2e12,
			).RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"statements.CURRENT_SCHEMA",
						"statements.THREAD_ID",
						"statements.EVENT_ID",
						"statements.END_EVENT_ID",
						"statements.DIGEST",
						"statements.SQL_TEXT",
						"statements.TIMER_END",
						"statements.TIMER_WAIT",
						"statements.ROWS_EXAMINED",
						"statements.ROWS_SENT",
						"statements.ROWS_AFFECTED",
						"statements.ERRORS",
						"statements.MYSQL_ERRNO",
						"statements.RETURNED_SQLSTATE",
						"statements.MESSAGE_TEXT",
						"waits.event_id",
						"waits.end_event_id",
						"waits.event_name",
						"waits.object_name",
						"waits.object_type",
						"waits.timer_wait",
						"nested_waits.event_id",
						"nested_waits.end_event_id",
						"nested_waits.event_name",
						"nested_waits.object_name",
						"nested_waits.object_type",
						"nested_waits.timer_wait",
						"threads.PROCESSLIST_USER",
						"threads.PROCESSLIST_HOST",
						"statements.CPU_TIME",
						"statements.MAX_CONTROLLED_MEMORY",
						"statements.MAX_TOTAL_MEMORY",
					}).AddRows(
						tc.rows...,
					),
				)

			err = collector.Start(t.Context())
			require.NoError(t, err)

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				entries := lokiClient.Received()
				require.Equal(t, len(entries), len(tc.logsLines))

				require.Contains(t, logBuffer.String(), tc.errorLine)
			}, 5*time.Second, 100*time.Millisecond)

			collector.Stop()
			lokiClient.Stop()

			require.Eventually(t, func() bool {
				return collector.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

			// Run this after Stop() to avoid race conditions
			err = mock.ExpectationsWereMet()
			require.NoError(t, err)

			lokiEntries := lokiClient.Received()
			require.Equal(t, len(tc.logsLines), len(lokiEntries))
			require.Equal(t, len(tc.logsLabels), len(lokiEntries))

			for i, entry := range lokiEntries {
				require.Equal(t, tc.logsLabels[i], entry.Labels)
				require.Equal(t, tc.logsLines[i], entry.Line)
			}
		})
	}
}

func TestQuerySamples_WaitEvents(t *testing.T) {
	t.Run("both query sample and associated wait event is collected", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			EngineVersion:   latestCompatibleVersion,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRows(
					[]driver.Value{
						"some_schema",
						"890",
						"123",
						"234",
						"some_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						"124",
						"124",
						"wait/io/file/innodb/innodb_data_file",
						"wait_object_name",
						"wait_object_type",
						"100000000",
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"124\" wait_event_name=\"wait/io/file/innodb/innodb_data_file\" wait_object_name=\"wait_object_name\" wait_object_type=\"wait_object_type\" wait_time=\"0.100000ms\"", lokiEntries[1].Line)
	})

	t.Run("wait event with NULL timer_wait is skipped", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			EngineVersion:   latestCompatibleVersion,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRows(
					[]driver.Value{
						"some_schema",
						"890",
						"123",
						"234",
						"some_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						"124",
						"124",
						"wait/io/file/innodb/innodb_data_file",
						"wait_object_name",
						"wait_object_type",
						nil, // NULL timer_wait
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		// Only the query sample should be collected, the wait event should be skipped
		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 1)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
	})

	t.Run("query sample and multiple wait events are collected", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			EngineVersion:   latestCompatibleVersion,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRows(
					[]driver.Value{
						"books_store",
						"890",
						"123",
						"234",
						"some_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						"124",
						"125",
						"wait/lock/table/sql/handler",
						"books",
						"TABLE",
						"150000",
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
					[]driver.Value{
						"books_store",
						"890",
						"123",
						"234",
						"some_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						"126",
						"126",
						"wait/lock/table/sql/handler",
						"categories",
						"TABLE",
						"350000",
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
					[]driver.Value{
						"books_store",
						"890",
						"123",
						"234",
						"some_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						"127",
						"127",
						"wait/io/table/sql/handler",
						"books",
						"TABLE",
						"500000",
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
					[]driver.Value{
						"books_store",
						"890",
						"123",
						"234",
						"some_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						"128",
						"128",
						"wait/io/table/sql/handler",
						"categories",
						"TABLE",
						"700000",
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 5
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"125\" wait_event_name=\"wait/lock/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"0.000150ms\"", lokiEntries[1].Line)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[2].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"126\" wait_end_event_id=\"126\" wait_event_name=\"wait/lock/table/sql/handler\" wait_object_name=\"categories\" wait_object_type=\"TABLE\" wait_time=\"0.000350ms\"", lokiEntries[2].Line)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[3].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"127\" wait_end_event_id=\"127\" wait_event_name=\"wait/io/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"0.000500ms\"", lokiEntries[3].Line)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[4].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"128\" wait_end_event_id=\"128\" wait_event_name=\"wait/io/table/sql/handler\" wait_object_name=\"categories\" wait_object_type=\"TABLE\" wait_time=\"0.000700ms\"", lokiEntries[4].Line)
	})

	t.Run("query sample and its wait event and another query sample are collected", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			EngineVersion:   latestCompatibleVersion,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRows(
					[]driver.Value{
						"books_store",
						"890",
						"123",
						"234",
						"some_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						"124",
						"125",
						"wait/lock/table/sql/handler",
						"books",
						"TABLE",
						"150000",
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
					[]driver.Value{
						"books_store",
						"890",
						"126",
						"234",
						"another_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"125\" wait_event_name=\"wait/lock/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"0.000150ms\"", lokiEntries[1].Line)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[2].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"126\" end_event_id=\"234\" digest=\"another_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[2].Line)
	})

	t.Run("wait event with disabled sql redaction", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			EngineVersion:         latestCompatibleVersion,
			CollectInterval:       time.Second,
			EntryHandler:          lokiClient,
			Logger:                util.TestAlloyLogger(t).Slog(),
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				),
			)
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, // initial timerBookmark
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					"124",
					"125",
					"wait/io/file/innodb/innodb_data_file",
					"wait_object_name",
					"wait_object_type",
					"100000000",
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\" sql_text=\"select * from some_table where id = 1\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"125\" wait_event_name=\"wait/io/file/innodb/innodb_data_file\" wait_object_name=\"wait_object_name\" wait_object_type=\"wait_object_type\" wait_time=\"0.100000ms\"", lokiEntries[1].Line)
	})

	t.Run("wait event below wait_min_duration is filtered by SQL", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                   db,
			EngineVersion:        latestCompatibleVersion,
			CollectInterval:      time.Second,
			EntryHandler:         lokiClient,
			Logger:               util.TestAlloyLogger(t).Slog(),
			WaitEventMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "AND waits.timer_wait >= 1000000000", "AND nested_waits.timer_wait >= 1000000000", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRows(
					[]driver.Value{
						"some_schema",
						"890",
						"123",
						"234",
						"some_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						nil, // no wait joined (filtered by SQL)
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 1)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
	})

	t.Run("wait event at or above wait_min_duration is collected", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                   db,
			EngineVersion:        latestCompatibleVersion,
			CollectInterval:      time.Second,
			EntryHandler:         lokiClient,
			Logger:               util.TestAlloyLogger(t).Slog(),
			WaitEventMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "AND waits.timer_wait >= 1000000000", "AND nested_waits.timer_wait >= 1000000000", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRows(
					[]driver.Value{
						"some_schema",
						"890",
						"123",
						"234",
						"some_digest",
						nil,
						"70000000",
						"20000000",
						"5",
						"5",
						"0",
						"0",
						nil,
						nil,
						nil,
						"124",
						"124",
						"wait/io/file/innodb/innodb_data_file",
						"wait_object_name",
						"wait_object_type",
						"1000000000", // 1ms in picoseconds
						nil,
						nil,
						nil,
						nil,
						nil,
						nil,
						"some_user",
						"some_host",
						"10000000",
						"456",
						"457",
					},
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"124\" wait_event_name=\"wait/io/file/innodb/innodb_data_file\" wait_object_name=\"wait_object_name\" wait_object_type=\"wait_object_type\" wait_time=\"1.000000ms\"", lokiEntries[1].Line)
	})

	// No wait_event_min_duration is set, so the nested atom passes through
	// unfiltered and the Go-side override surfaces it instead of the molecule.
	t.Run("wait/io/table/sql/handler with nested event uses nested event data", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			EngineVersion:   latestCompatibleVersion,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					"select * from books where id = ?",
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					"200",                                  // WAIT_EVENT_ID (outer table handler)
					"201",                                  // WAIT_END_EVENT_ID
					"wait/io/table/sql/handler",            // WAIT_EVENT_NAME (the handler wrapper)
					"books",                                // WAIT_OBJECT_NAME
					"TABLE",                                // WAIT_OBJECT_TYPE
					"900000000",                            // WAIT_TIMER_WAIT (0.9ms)
					"210",                                  // NESTED_WAIT_EVENT_ID
					"211",                                  // NESTED_WAIT_END_EVENT_ID
					"wait/io/file/innodb/innodb_data_file", // NESTED_WAIT_EVENT_NAME (actual I/O)
					"ibdata1",                              // NESTED_WAIT_OBJECT_NAME
					"FILE",                                 // NESTED_WAIT_OBJECT_TYPE
					"500000000",                            // NESTED_WAIT_TIMER_WAIT (0.5ms)
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"210\" wait_end_event_id=\"211\" wait_event_name=\"wait/io/file/innodb/innodb_data_file\" wait_object_name=\"ibdata1\" wait_object_type=\"FILE\" wait_time=\"0.500000ms\"", lokiEntries[1].Line)
	})

	t.Run("wait/io/table/sql/handler with nested event classifies on nested name in v2 mode", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                            db,
			EngineVersion:                 latestCompatibleVersion,
			CollectInterval:               time.Second,
			EntryHandler:                  lokiClient,
			Logger:                        util.TestAlloyLogger(t).Slog(),
			EnablePreClassifiedWaitEvents: true,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					"select * from books where id = ?",
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					"200",                                  // WAIT_EVENT_ID (outer table handler)
					"201",                                  // WAIT_END_EVENT_ID
					"wait/io/table/sql/handler",            // WAIT_EVENT_NAME (the handler wrapper)
					"books",                                // WAIT_OBJECT_NAME
					"TABLE",                                // WAIT_OBJECT_TYPE
					"900000000",                            // WAIT_TIMER_WAIT (0.9ms)
					"210",                                  // NESTED_WAIT_EVENT_ID
					"211",                                  // NESTED_WAIT_END_EVENT_ID
					"wait/io/file/innodb/innodb_data_file", // NESTED_WAIT_EVENT_NAME (actual I/O)
					"ibdata1",                              // NESTED_WAIT_OBJECT_NAME
					"FILE",                                 // NESTED_WAIT_OBJECT_TYPE
					"500000000",                            // NESTED_WAIT_TIMER_WAIT (0.5ms)
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT_V2}, lokiEntries[1].Labels)
		// Pins that wait_event_type is derived from the substituted (nested) name,
		// not the outer wrapper.
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"210\" wait_end_event_id=\"211\" wait_event_name=\"wait/io/file/innodb/innodb_data_file\" wait_event_type=\"IO Wait\" wait_object_name=\"ibdata1\" wait_object_type=\"FILE\" wait_time=\"0.500000ms\"", lokiEntries[1].Line)
	})

	// When the outer molecule wait passes wait_event_min_duration but the nested
	// atom is below the threshold, the SQL nested_waits.timer_wait predicate
	// nullifies the nested columns. The Go-side override is then skipped and the
	// outer molecule (wait/io/table/sql/handler) is emitted, ensuring every
	// wait_event has wait_time >= wait_event_min_duration.
	t.Run("wait/io/table/sql/handler falls back to molecule when nested atom is below threshold", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                   db,
			EngineVersion:        latestCompatibleVersion,
			CollectInterval:      time.Second,
			EntryHandler:         lokiClient,
			Logger:               util.TestAlloyLogger(t).Slog(),
			WaitEventMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "AND waits.timer_wait >= 1000000000", "AND nested_waits.timer_wait >= 1000000000", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					"select * from books where id = ?",
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					"200",                       // WAIT_EVENT_ID (outer table handler)
					"201",                       // WAIT_END_EVENT_ID
					"wait/io/table/sql/handler", // WAIT_EVENT_NAME (the handler wrapper)
					"books",                     // WAIT_OBJECT_NAME
					"TABLE",                     // WAIT_OBJECT_TYPE
					"50000000000",               // WAIT_TIMER_WAIT (50ms, qualifies)
					nil,                         // NESTED_WAIT_EVENT_ID (filtered by SQL)
					nil,                         // NESTED_WAIT_END_EVENT_ID
					nil,                         // NESTED_WAIT_EVENT_NAME
					nil,                         // NESTED_WAIT_OBJECT_NAME
					nil,                         // NESTED_WAIT_OBJECT_TYPE
					nil,                         // NESTED_WAIT_TIMER_WAIT
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"200\" wait_end_event_id=\"201\" wait_event_name=\"wait/io/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"50.000000ms\"", lokiEntries[1].Line)
	})
}

func TestQuerySamples_SampleMinDuration(t *testing.T) {
	t.Run("query sample below sample_min_duration is filtered by SQL", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                db,
			EngineVersion:     latestCompatibleVersion,
			CollectInterval:   time.Second,
			EntryHandler:      lokiClient,
			Logger:            util.TestAlloyLogger(t).Slog(),
			SampleMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "AND statements.TIMER_WAIT >= 1000000000", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 0)
	})

	t.Run("query sample at or above sample_min_duration is collected", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                db,
			EngineVersion:     latestCompatibleVersion,
			CollectInterval:   time.Second,
			EntryHandler:      lokiClient,
			Logger:            util.TestAlloyLogger(t).Slog(),
			SampleMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "AND statements.TIMER_WAIT >= 1000000000", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					nil,
					"70000000",
					"1000000000", // 1ms in picoseconds
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 1)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
	})
}

func TestQuerySamples_DisableQueryRedaction(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Run("collects sql text when enabled", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			EngineVersion:         latestCompatibleVersion,
			CollectInterval:       time.Second,
			EntryHandler:          lokiClient,
			Logger:                util.TestAlloyLogger(t).Slog(),
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(1e12, 2e12).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\" sql_text=\"select * from some_table where id = 1\"", lokiEntries[0].Line)
	})

	t.Run("does not collect sql text when disabled", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			EngineVersion:         latestCompatibleVersion,
			CollectInterval:       time.Second,
			EntryHandler:          lokiClient,
			Logger:                util.TestAlloyLogger(t).Slog(),
			DisableQueryRedaction: false,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(1e12, 2e12).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					nil,
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
	})

	// erroredRow represents a statement that failed with a duplicate-key error,
	// used to verify that the error message text is only emitted when query
	// redaction is disabled.
	const erroredSQLText = "insert into users (email) values ('jdoe@example.com') /* \"quoted\" C:\\tmp\\file\nnext line */"
	const errorMessageText = "Duplicate entry 'jdoe@example.com' for key 'users.email': \"quoted\"\nC:\\tmp\\file"
	erroredRow := []driver.Value{
		"some_schema",
		"890",
		"123",
		"234",
		"some_digest",
		erroredSQLText,
		"70000000",
		"20000000",
		"5",
		"5",
		"0",
		"1",     // errors
		"1062",  // mysql_errno
		"23000", // returned_sqlstate
		errorMessageText,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"some_user",
		"some_host",
		"10000000",
		"456",
		"457",
	}

	querySampleColumns := []string{
		"statements.CURRENT_SCHEMA",
		"statements.THREAD_ID",
		"statements.EVENT_ID",
		"statements.END_EVENT_ID",
		"statements.DIGEST",
		"statements.SQL_TEXT",
		"statements.TIMER_END",
		"statements.TIMER_WAIT",
		"statements.ROWS_EXAMINED",
		"statements.ROWS_SENT",
		"statements.ROWS_AFFECTED",
		"statements.ERRORS",
		"statements.MYSQL_ERRNO",
		"statements.RETURNED_SQLSTATE",
		"statements.MESSAGE_TEXT",
		"waits.event_id",
		"waits.end_event_id",
		"waits.event_name",
		"waits.object_name",
		"waits.object_type",
		"waits.timer_wait",
		"nested_waits.event_id",
		"nested_waits.end_event_id",
		"nested_waits.event_name",
		"nested_waits.object_name",
		"nested_waits.object_type",
		"nested_waits.timer_wait",
		"threads.PROCESSLIST_USER",
		"threads.PROCESSLIST_HOST",
		"statements.CPU_TIME",
		"statements.MAX_CONTROLLED_MEMORY",
		"statements.MAX_TOTAL_MEMORY",
	}

	t.Run("redacts error message text by default", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			EngineVersion:         latestCompatibleVersion,
			CollectInterval:       time.Second,
			EntryHandler:          lokiClient,
			Logger:                util.TestAlloyLogger(t).Slog(),
			DisableQueryRedaction: false,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(1e12, 2e12).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(querySampleColumns).AddRow(erroredRow...))

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

		require.NoError(t, mock.ExpectationsWereMet())

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"1\" mysql_errno=\"1062\" returned_sqlstate=\"23000\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		// message_text can leak literal values, so it must not appear when redaction is enabled.
		assert.NotContains(t, lokiEntries[0].Line, "message_text=")
	})

	t.Run("emits error message text when query redaction is disabled", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			EngineVersion:         latestCompatibleVersion,
			CollectInterval:       time.Second,
			EntryHandler:          lokiClient,
			Logger:                util.TestAlloyLogger(t).Slog(),
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(1e12, 2e12).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(querySampleColumns).AddRow(erroredRow...))

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

		require.NoError(t, mock.ExpectationsWereMet())

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" user="some_user" client_host="some_host" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="1" mysql_errno="1062" returned_sqlstate="23000" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms" sql_text="insert into users (email) values ('jdoe@example.com') /* \"quoted\" C:\\tmp\\file\nnext line */" message_text="Duplicate entry 'jdoe@example.com' for key 'users.email': \"quoted\"\nC:\\tmp\\file"`, lokiEntries[0].Line)
	})
}

func TestQuerySamplesMySQLVersions(t *testing.T) {
	defer goleak.VerifyNone(t)

	testCases := []struct {
		name              string
		mysqlVersion      string
		expectedFields    string
		expectedColumns   []string
		expectedLogOutput string
		scanValues        []driver.Value
	}{
		{
			name:           "MySQL version <8.0.28 - no CPU or memory fields",
			mysqlVersion:   "8.0.27",
			expectedFields: "",
			expectedColumns: []string{
				"statements.CURRENT_SCHEMA",
				"statements.THREAD_ID",
				"statements.EVENT_ID",
				"statements.END_EVENT_ID",
				"statements.DIGEST",
				"statements.SQL_TEXT",
				"statements.TIMER_END",
				"statements.TIMER_WAIT",
				"statements.ROWS_EXAMINED",
				"statements.ROWS_SENT",
				"statements.ROWS_AFFECTED",
				"statements.ERRORS",
				"statements.MYSQL_ERRNO",
				"statements.RETURNED_SQLSTATE",
				"statements.MESSAGE_TEXT",
				"waits.event_id",
				"waits.end_event_id",
				"waits.event_name",
				"waits.object_name",
				"waits.object_type",
				"waits.timer_wait",
				"nested_waits.event_id",
				"nested_waits.end_event_id",
				"nested_waits.event_name",
				"nested_waits.object_name",
				"nested_waits.object_type",
				"nested_waits.timer_wait",
				"threads.PROCESSLIST_USER",
				"threads.PROCESSLIST_HOST",
			},
			expectedLogOutput: `level="info" schema="test_schema" user="some_user" client_host="some_host" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" mysql_errno="0" returned_sqlstate="" max_controlled_memory="0b" max_total_memory="0b" cpu_time="0.000000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			scanValues: []driver.Value{
				"test_schema",
				"890",
				"123",
				"234",
				"some_digest",
				nil,
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
			},
		},
		{
			name:           "MySQL version >8.0.28 <8.0.31 - has CPU time but no memory fields",
			mysqlVersion:   "8.0.30",
			expectedFields: cpuTimeField,
			expectedColumns: []string{
				"statements.CURRENT_SCHEMA",
				"statements.THREAD_ID",
				"statements.EVENT_ID",
				"statements.END_EVENT_ID",
				"statements.DIGEST",
				"statements.SQL_TEXT",
				"statements.TIMER_END",
				"statements.TIMER_WAIT",
				"statements.ROWS_EXAMINED",
				"statements.ROWS_SENT",
				"statements.ROWS_AFFECTED",
				"statements.ERRORS",
				"statements.MYSQL_ERRNO",
				"statements.RETURNED_SQLSTATE",
				"statements.MESSAGE_TEXT",
				"waits.event_id",
				"waits.end_event_id",
				"waits.event_name",
				"waits.object_name",
				"waits.object_type",
				"waits.timer_wait",
				"nested_waits.event_id",
				"nested_waits.end_event_id",
				"nested_waits.event_name",
				"nested_waits.object_name",
				"nested_waits.object_type",
				"nested_waits.timer_wait",
				"threads.PROCESSLIST_USER",
				"threads.PROCESSLIST_HOST",
				"statements.CPU_TIME",
			},
			expectedLogOutput: `level="info" schema="test_schema" user="some_user" client_host="some_host" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" mysql_errno="0" returned_sqlstate="" max_controlled_memory="0b" max_total_memory="0b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			scanValues: []driver.Value{
				"test_schema",
				"890",
				"123",
				"234",
				"some_digest",
				nil,
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				"10000000", // CPU_TIME
			},
		},
		{
			name:           "MySQL version >=8.0.31 - has CPU time and memory fields",
			mysqlVersion:   "8.0.32",
			expectedFields: cpuTimeField + maxControlledMemoryField + maxTotalMemoryField,
			expectedColumns: []string{
				"statements.CURRENT_SCHEMA",
				"statements.THREAD_ID",
				"statements.EVENT_ID",
				"statements.END_EVENT_ID",
				"statements.DIGEST",
				"statements.SQL_TEXT",
				"statements.TIMER_END",
				"statements.TIMER_WAIT",
				"statements.ROWS_EXAMINED",
				"statements.ROWS_SENT",
				"statements.ROWS_AFFECTED",
				"statements.ERRORS",
				"statements.MYSQL_ERRNO",
				"statements.RETURNED_SQLSTATE",
				"statements.MESSAGE_TEXT",
				"waits.event_id",
				"waits.end_event_id",
				"waits.event_name",
				"waits.object_name",
				"waits.object_type",
				"waits.timer_wait",
				"nested_waits.event_id",
				"nested_waits.end_event_id",
				"nested_waits.event_name",
				"nested_waits.object_name",
				"nested_waits.object_type",
				"nested_waits.timer_wait",
				"threads.PROCESSLIST_USER",
				"threads.PROCESSLIST_HOST",
				"statements.CPU_TIME",
				"statements.MAX_CONTROLLED_MEMORY",
				"statements.MAX_TOTAL_MEMORY",
			},
			expectedLogOutput: `level="info" schema="test_schema" user="some_user" client_host="some_host" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" mysql_errno="0" returned_sqlstate="" max_controlled_memory="1024b" max_total_memory="2048b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			scanValues: []driver.Value{
				"test_schema",
				"890",
				"123",
				"234",
				"some_digest",
				nil,
				"70000000",
				"20000000",
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				"10000000", // CPU_TIME
				"1024",     // MAX_CONTROLLED_MEMORY
				"2048",     // MAX_TOTAL_MEMORY
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			lokiClient := loki.NewCollectingHandler()

			collector, err := NewQuerySamples(QuerySamplesArguments{
				DB:              db,
				EngineVersion:   semver.MustParse(tc.mysqlVersion),
				CollectInterval: time.Second,
				EntryHandler:    lokiClient,
				Logger:          util.TestAlloyLogger(t).Slog(),
			})
			require.NoError(t, err)
			require.NotNil(t, collector)

			mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"uptime",
					}).AddRow(
						"1",
					),
				)
			expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

			mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
				sqlmock.NewRows([]string{
					"now",
					"uptime",
				}).AddRow(
					5,
					1,
				))

			expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
			mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, tc.expectedFields, "", "", exclusionClause, "", endOfTimeline)).WithArgs(1e12, 2e12).RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows(tc.expectedColumns).AddRow(tc.scanValues...),
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

			err = mock.ExpectationsWereMet()
			require.NoError(t, err)

			lokiEntries := lokiClient.Received()
			require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
			require.Equal(t, tc.expectedLogOutput, lokiEntries[0].Line)
		})
	}
}

func TestQuerySamples_SQLDriverErrors(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("recoverable sql error in result set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			EngineVersion:   latestCompatibleVersion,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest", // not enough columns
				}).AddRow(
					"abc123",
				))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		// the first poll's window moved the bookmark even though its rows failed to scan
		expectLatestTimerEnd(mock, 2e12, 11e12, 3e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			2e12,
			3e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					nil,
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
	})

	t.Run("result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			EngineVersion:   latestCompatibleVersion,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				2,
			))

		expectLatestTimerEnd(mock, 1e12, 12e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(1e12, 2e12).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					nil,
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
				).AddRow(
					"some_schema",
					"891",
					"124",
					"235",
					"some_digest",
					nil,
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
				).RowError(1, fmt.Errorf("rs error")), // error on second row
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
	})

	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			EngineVersion:   latestCompatibleVersion,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				2,
			))

		expectLatestTimerEnd(mock, 1e12, 12e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				2,
			))

		expectLatestTimerEnd(mock, 1e12, 12e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					nil,
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
	})
}

func TestQuerySamples_initializeTimer(t *testing.T) {
	t.Run("selects uptime, sets timerBookmark to the latest timer value", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{
			"uptime",
		}).AddRow(
			5,
		))
		expectLatestTimerEnd(mock,
			float64(0), // a margin before uptime, at most the server start
			15e12,      // ceiling: uptime plus margin
			4.6e12,
		)

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db, Logger: util.TestAlloyLogger(t).Slog()})
		require.NoError(t, err)

		require.NoError(t, c.initializeBookmark(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.Equal(t, 4.6e12, c.timerBookmark)
		assert.Equal(t, 0, c.timerBookmarkOverflows)
		assert.EqualValues(t, 5, c.lastUptime)
	})

	t.Run("sets timerBookmark as the latest timer value modulo overflows (uptime is comprised of 1 overflow)", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{
			"uptime",
		}).AddRow(
			picosecondsToSeconds(math.MaxUint64) + 15,
		))
		expectLatestTimerEnd(mock,
			5e12,  // a margin before uptime, modulo overflows
			25e12, // ceiling: uptime plus margin, modulo overflows
			15.3e12,
		)

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db, Logger: util.TestAlloyLogger(t).Slog()})
		require.NoError(t, err)

		require.NoError(t, c.initializeBookmark(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.Equal(t, 15.3e12, c.timerBookmark)
		assert.Equal(t, 1, c.timerBookmarkOverflows)
	})

	t.Run("uptime shows that the timer has just overflowed: timerBookmark is the latest timer value after the overflow", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		overflowSeconds := picosecondsToSeconds(math.MaxUint64)

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow(overflowSeconds + 5))
		expectLatestTimerEndAcrossOverflow(mock,
			15e12, // ceiling: uptime plus margin, after the overflow
			secondsToPicoseconds(overflowSeconds-5), // a margin before uptime, before the overflow
			3.2e12,
		)

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db, Logger: util.TestAlloyLogger(t).Slog()})
		require.NoError(t, err)

		require.NoError(t, c.initializeBookmark(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.Equal(t, 3.2e12, c.timerBookmark)
		assert.Equal(t, 1, c.timerBookmarkOverflows)
	})

	t.Run("uptime is close to an overflow: timerBookmark is the latest timer value before the overflow", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		overflow := float64(math.MaxUint64)
		overflowSeconds := picosecondsToSeconds(overflow)

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow(overflowSeconds - 3))
		expectLatestTimerEnd(mock,
			secondsToPicoseconds(overflowSeconds-13), // a margin before uptime
			overflow, // ceiling: the overflow, as the timer may not have reached it
			overflow-1e12,
		)

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db, Logger: util.TestAlloyLogger(t).Slog()})
		require.NoError(t, err)

		require.NoError(t, c.initializeBookmark(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.Equal(t, overflow-1e12, c.timerBookmark)
		assert.Equal(t, 0, c.timerBookmarkOverflows)
	})

	t.Run("no statement ended recently: timerBookmark is a margin before uptime", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow(100))
		expectLatestTimerEnd(mock, 90e12, 110e12, nil)

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db, Logger: util.TestAlloyLogger(t).Slog()})
		require.NoError(t, err)

		require.NoError(t, c.initializeBookmark(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.Equal(t, 90e12, c.timerBookmark)
		assert.Equal(t, 0, c.timerBookmarkOverflows)
	})

	t.Run("returns error when the latest timer query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow(100))
		mock.ExpectQuery(selectLatestTimerEnd).WithArgs(90e12, 110e12).WillReturnError(fmt.Errorf("some error"))

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db, Logger: util.TestAlloyLogger(t).Slog()})
		require.NoError(t, err)

		require.EqualError(t, c.initializeBookmark(t.Context()), "failed to fetch latest statement timer: some error")
	})
}

func TestQuerySamples_logs_query_start_as_timestamp(t *testing.T) {
	t.Run("query_sample and legacy wait_event", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				10,
				10,
			),
		)
		expectLatestTimerEnd(mock, float64(0), 20e12, 10e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			float64(0), // initial timerBookmark
			10e12,      // latest timer value
		).WillReturnRows(sqlmock.NewRows([]string{
			"statements.CURRENT_SCHEMA",
			"statements.THREAD_ID",
			"statements.EVENT_ID",
			"statements.END_EVENT_ID",
			"statements.DIGEST",
			"statements.SQL_TEXT",
			"statements.TIMER_END",
			"statements.TIMER_WAIT",
			"statements.ROWS_EXAMINED",
			"statements.ROWS_SENT",
			"statements.ROWS_AFFECTED",
			"statements.ERRORS",
			"statements.MYSQL_ERRNO",
			"statements.RETURNED_SQLSTATE",
			"statements.MESSAGE_TEXT",
			"waits.event_id",
			"waits.end_event_id",
			"waits.event_name",
			"waits.object_name",
			"waits.object_type",
			"waits.timer_wait",
			"nested_waits.event_id",
			"nested_waits.end_event_id",
			"nested_waits.event_name",
			"nested_waits.object_name",
			"nested_waits.object_type",
			"nested_waits.timer_wait",
			"threads.PROCESSLIST_USER",
			"threads.PROCESSLIST_HOST",
			"statements.CPU_TIME",
			"statements.MAX_CONTROLLED_MEMORY",
			"statements.MAX_TOTAL_MEMORY",
		}).
			AddRow(
				"test_schema",
				890,
				123,
				234,
				"some digest",
				nil,
				4e12, // timer_end
				1e12, // elapsed time
				1000,
				100,
				0,
				0,
				nil,
				nil,
				nil,
				"124",
				"125",
				"wait/io/file/innodb/innodb_data_file",
				"wait_object_name",
				"wait_object_type",
				100000000,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user",
				"some_host",
				555555,
				1048576,
				2097152,
			),
		)

		lokiClient := loki.NewCollectingHandler()
		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			entryHandler:  lokiClient,
			logger:        util.TestAlloyLogger(t).Slog(),
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))

		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 5*time.Second, 100*time.Millisecond)
		require.Len(t, lokiClient.Received(), 2)

		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiClient.Received()[0].Labels)
		assert.Equal(t, time.Unix(3, 0), lokiClient.Received()[0].Timestamp) // timer_end - elapsed time = timer_start
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiClient.Received()[1].Labels)
		assert.Equal(t, time.Unix(3, 0), lokiClient.Received()[1].Timestamp) // timer_end - elapsed time = timer_start
	})

	t.Run("wait_event v2", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				10,
				10,
			),
		)

		expectLatestTimerEnd(mock, float64(0), 20e12, 10e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			float64(0), // initial timerBookmark
			10e12,      // latest timer value
		).WillReturnRows(sqlmock.NewRows([]string{
			"statements.CURRENT_SCHEMA",
			"statements.THREAD_ID",
			"statements.EVENT_ID",
			"statements.END_EVENT_ID",
			"statements.DIGEST",
			"statements.SQL_TEXT",
			"statements.TIMER_END",
			"statements.TIMER_WAIT",
			"statements.ROWS_EXAMINED",
			"statements.ROWS_SENT",
			"statements.ROWS_AFFECTED",
			"statements.ERRORS",
			"statements.MYSQL_ERRNO",
			"statements.RETURNED_SQLSTATE",
			"statements.MESSAGE_TEXT",
			"waits.event_id",
			"waits.end_event_id",
			"waits.event_name",
			"waits.object_name",
			"waits.object_type",
			"waits.timer_wait",
			"nested_waits.event_id",
			"nested_waits.end_event_id",
			"nested_waits.event_name",
			"nested_waits.object_name",
			"nested_waits.object_type",
			"nested_waits.timer_wait",
			"threads.PROCESSLIST_USER",
			"threads.PROCESSLIST_HOST",
			"statements.CPU_TIME",
			"statements.MAX_CONTROLLED_MEMORY",
			"statements.MAX_TOTAL_MEMORY",
		}).
			AddRow(
				"schema_X",
				"1",
				"10",
				"11",
				"digest_A",
				"sql1",
				4e12, // timer_end
				1e12, // elapsed time
				"5",
				"5",
				"0",
				"0",
				nil,
				nil,
				nil,
				"100",
				"101",
				"wait/io/file/x",
				"obj",
				"typ",
				"100000000",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"u",
				"h",
				"1000",
				"1",
				"1",
			),
		)

		lokiClient := loki.NewCollectingHandler()
		c := &QuerySamples{
			dbConnection:                  db,
			engineVersion:                 latestCompatibleVersion,
			entryHandler:                  lokiClient,
			logger:                        util.TestAlloyLogger(t).Slog(),
			enablePreClassifiedWaitEvents: true,
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))

		lokiClient.Stop()

		require.Eventually(t, func() bool { return len(lokiClient.Received()) == 2 }, 5*time.Second, 100*time.Millisecond)

		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT_V2}, lokiClient.Received()[1].Labels)
		assert.Equal(t, time.Unix(3, 0), lokiClient.Received()[1].Timestamp) // timer_end - elapsed time = timer_start
	})
}

func TestQuerySamples_handles_timer_overflows(t *testing.T) {
	t.Run("selects query sample summary: first run uses initialized timerBookmark and the latest timer value as limit", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				6,
				5,
			),
		)
		expectLatestTimerEnd(mock,
			1e12,  // initial timerBookmark
			15e12, // ceiling: uptime plus margin, in picoseconds
			5.3e12,
		)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,   // initial timerBookmark
			5.3e12, // latest timer value
		).WillReturnRows(sqlmock.NewRows([]string{
			"statements.CURRENT_SCHEMA",
			"statements.THREAD_ID",
			"statements.EVENT_ID",
			"statements.END_EVENT_ID",
			"statements.DIGEST",
			"statements.SQL_TEXT",
			"statements.TIMER_END",
			"statements.TIMER_WAIT",
			"statements.ROWS_EXAMINED",
			"statements.ROWS_SENT",
			"statements.ROWS_AFFECTED",
			"statements.ERRORS",
			"statements.MYSQL_ERRNO",
			"statements.RETURNED_SQLSTATE",
			"statements.MESSAGE_TEXT",
			"waits.event_id",
			"waits.end_event_id",
			"waits.event_name",
			"waits.object_name",
			"waits.object_type",
			"waits.timer_wait",
			"nested_waits.event_id",
			"nested_waits.end_event_id",
			"nested_waits.event_name",
			"nested_waits.object_name",
			"nested_waits.object_type",
			"nested_waits.timer_wait",
			"threads.PROCESSLIST_USER",
			"threads.PROCESSLIST_HOST",
			"statements.CPU_TIME",
			"statements.MAX_CONTROLLED_MEMORY",
			"statements.MAX_TOTAL_MEMORY",
		}).
			AddRow(
				"test_schema", // current_schema
				890,           // THREAD
				123,           // EVENT_ID
				234,           // END_EVENT_ID
				"some digest", // digest
				nil,           // SQL_TEXT
				2e12,          // timer_end
				2e12,          // timer_wait
				1000,          // rows_examined
				100,           // rows_sent
				0,             // rows_affected
				0,             // errors
				nil,           // mysql_errno
				nil,           // returned_sqlstate
				nil,           // message_text
				nil,           // WAIT_EVENT_ID
				nil,           // WAIT_END_EVENT_ID
				nil,           // WAIT_EVENT_NAME
				nil,           // WAIT_OBJECT_NAME
				nil,           // WAIT_OBJECT_TYPE
				nil,           // WAIT_TIME
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user", // PROCESSLIST_USER
				"some_host", // PROCESSLIST_HOST
				555555,      // cpu_time
				1048576,     // max_controlled_memory (1MB)
				2097152,     // max_total_memory (2MB)
			),
		)

		lokiClient := loki.NewCollectingHandler()
		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			timerBookmark: 1e12,
			lastUptime:    4,
			entryHandler:  lokiClient,
			logger:        util.TestAlloyLogger(t).Slog(),
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))

		assert.Equal(t, 5.3e12, c.timerBookmark) // timerBookmark is updated to the latest timer value
		assert.EqualValues(t, 5, c.lastUptime)   // lastUptime is updated to the uptime in seconds

		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)
		require.Len(t, lokiClient.Received(), 1)
		assert.Equal(t, model.LabelSet{
			"op": database_observability.OP_QUERY_SAMPLE,
		}, lokiClient.Received()[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"test_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some digest\" rows_examined=\"1000\" rows_sent=\"100\" rows_affected=\"0\" errors=\"0\" mysql_errno=\"0\" returned_sqlstate=\"\" max_controlled_memory=\"1048576b\" max_total_memory=\"2097152b\" cpu_time=\"0.000556ms\" elapsed_time=\"2000.000000ms\" elapsed_time_ms=\"2000.000000ms\"", lokiClient.Received()[0].Line)
	})

	t.Run("asserts that expected query text is used in the constants", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(`
			SELECT unix_timestamp() AS now,
			       variable_value AS uptime
			FROM performance_schema.global_status
			WHERE variable_name = 'UPTIME'`).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				6,
				5,
			),
		)
		mock.ExpectQuery(`
			SELECT MAX(TIMER_END)
			FROM performance_schema.events_statements_current
			WHERE TIMER_END > ? AND TIMER_END <= ?`).WithArgs(
			1e12,  // initial timerBookmark
			15e12, // ceiling
		).WillReturnRows(sqlmock.NewRows([]string{"MAX(TIMER_END)"}).AddRow(5e12))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, // initial timerBookmark
			5e12, // latest timer value
		).WillReturnRows(sqlmock.NewRows([]string{
			"current_schema",
			"thread_id",
			"event_id",
			"end_event_id",
			"digest",
			"sql_text",
			"timer_end",
			"timer_wait",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"errors",
			"waits.event_id",
			"waits.end_event_id",
			"waits.event_name",
			"waits.object_name",
			"waits.object_type",
			"waits.timer_wait",
			"nested_waits.event_id",
			"nested_waits.end_event_id",
			"nested_waits.event_name",
			"nested_waits.object_name",
			"nested_waits.object_type",
			"nested_waits.timer_wait",
			"threads.PROCESSLIST_USER",
			"threads.PROCESSLIST_HOST",
			"cpu_time",
			"max_controlled_memory",
			"max_total_memory",
		}))

		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			timerBookmark: 1e12,
			lastUptime:    4,
			logger:        util.TestAlloyLogger(t).Slog(),
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))
	})

	t.Run("timer overflows: select with beginningAndEndOfTimeline clause, up to the latest timer value after the overflow", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		overflow := float64(math.MaxUint64)
		overflowSeconds := picosecondsToSeconds(overflow)
		serverStart := 1000.0

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(serverStart+overflowSeconds+2, overflowSeconds+2),
		)
		expectLatestTimerEndAcrossOverflow(mock,
			12e12,         // ceiling: uptime plus margin, after the overflow
			overflow-2e12, // timerBookmark, before the overflow
			0.8e12,
		)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", beginningAndEndOfTimeline)).WithArgs(
			overflow-2e12,
			0.8e12,
		).RowsWillBeClosed().WillReturnRows(querySamplesRows(
			overflow-1e12, // ended before the overflow
			0.5e12,        // ended after the overflow
		))

		lokiClient := loki.NewCollectingHandler()
		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			entryHandler:  lokiClient,
			logger:        util.TestAlloyLogger(t).Slog(),
			timerBookmark: overflow - 2e12,
			lastUptime:    overflowSeconds - 1,
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.Equal(t, 0.8e12, c.timerBookmark)
		assert.Equal(t, 1, c.timerBookmarkOverflows)
		assert.Equal(t, overflowSeconds+2, c.lastUptime)

		lokiClient.Stop()
		require.Eventually(t, func() bool { return len(lokiClient.Received()) == 2 }, 5*time.Second, 100*time.Millisecond)
		entries := lokiClient.Received()
		// Each statement's timestamp counts the overflows before it: 1.5 seconds apart, not an overflow apart.
		assert.InDelta(t, float64(1500*time.Millisecond), float64(entries[1].Timestamp.Sub(entries[0].Timestamp)), float64(time.Millisecond))
		assert.InDelta(t, (serverStart+overflowSeconds+0.5)*1e9-1e6, float64(entries[1].Timestamp.UnixNano()), 1e6)
	})

	t.Run("timer has overflowed, next query reverts back to endOfTimeline clause", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		overflowSeconds := picosecondsToSeconds(float64(math.MaxUint64))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(overflowSeconds+1003, overflowSeconds+3),
		)
		expectLatestTimerEnd(mock,
			0.8e12, // timerBookmark, after the overflow
			13e12,  // ceiling: uptime plus margin, after the overflow
			2.9e12,
		)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			0.8e12,
			2.9e12,
		).RowsWillBeClosed().WillReturnRows(querySamplesRows())

		c := &QuerySamples{
			dbConnection:           db,
			engineVersion:          latestCompatibleVersion,
			logger:                 util.TestAlloyLogger(t).Slog(),
			timerBookmark:          0.8e12,
			timerBookmarkOverflows: 1,
			lastUptime:             overflowSeconds + 1,
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.Equal(t, 2.9e12, c.timerBookmark)
		assert.Equal(t, 1, c.timerBookmarkOverflows)
	})

	t.Run("uptime reaches an overflow the timer may not have: the window stops at the overflow", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		overflow := float64(math.MaxUint64)
		overflowSeconds := picosecondsToSeconds(overflow)
		serverStart := 1000.0

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(serverStart+overflowSeconds, overflowSeconds),
		)
		expectLatestTimerEnd(mock,
			overflow-3e12, // timerBookmark
			overflow,      // ceiling: the overflow, as the timer may not have reached it
			overflow-0.4e12,
		)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			overflow-3e12,
			overflow-0.4e12,
		).RowsWillBeClosed().WillReturnRows(querySamplesRows(overflow - 1e12))

		lokiClient := loki.NewCollectingHandler()
		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			entryHandler:  lokiClient,
			logger:        util.TestAlloyLogger(t).Slog(),
			timerBookmark: overflow - 3e12,
			lastUptime:    overflowSeconds - 1,
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.Equal(t, overflow-0.4e12, c.timerBookmark)
		assert.Equal(t, 0, c.timerBookmarkOverflows)

		lokiClient.Stop()
		require.Eventually(t, func() bool { return len(lokiClient.Received()) == 1 }, 5*time.Second, 100*time.Millisecond)
		// The statement ended before the overflow, so its timestamp doesn't count it.
		assert.InDelta(t, (serverStart+overflowSeconds-1)*1e9-1e6, float64(lokiClient.Received()[0].Timestamp.UnixNano()), 1e6)
	})

	t.Run("server restarts, timer bookmark is reset", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(picosecondsToSeconds(math.MaxUint64)+15, 10),
		)
		expectLatestTimerEnd(mock,
			float64(0), // timerBookmark is reset
			20e12,      // ceiling: uptime plus margin
			9.7e12,
		)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			float64(0),
			9.7e12,
		).RowsWillBeClosed().WillReturnRows(querySamplesRows())

		c := &QuerySamples{
			dbConnection:           db,
			engineVersion:          latestCompatibleVersion,
			logger:                 util.TestAlloyLogger(t).Slog(),
			timerBookmark:          3e12,
			timerBookmarkOverflows: 1,
			lastUptime:             picosecondsToSeconds(math.MaxUint64) + 11,
		}
		require.NoError(t, c.fetchQuerySamples(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.EqualValues(t, 10, c.lastUptime)
		assert.Equal(t, 9.7e12, c.timerBookmark)
		assert.Equal(t, 0, c.timerBookmarkOverflows)
	})

	t.Run("server restarts and no statement has ended since: the reset bookmark is kept", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(picosecondsToSeconds(math.MaxUint64)+15, 10),
		)
		expectLatestTimerEnd(mock,
			float64(0), // timerBookmark is reset
			20e12,      // ceiling: uptime plus margin
			nil,
		)

		c := &QuerySamples{
			dbConnection:           db,
			engineVersion:          latestCompatibleVersion,
			logger:                 util.TestAlloyLogger(t).Slog(),
			timerBookmark:          3e12,
			timerBookmarkOverflows: 1,
			lastUptime:             picosecondsToSeconds(math.MaxUint64) + 11,
		}
		require.NoError(t, c.fetchQuerySamples(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.EqualValues(t, 10, c.lastUptime)
		assert.Equal(t, float64(0), c.timerBookmark)
		assert.Equal(t, 0, c.timerBookmarkOverflows)
	})

	t.Run("no statement has ended since the bookmark: samples are not selected and the bookmark is kept", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(10, 6),
		)
		expectLatestTimerEnd(mock, 5e12, 16e12, nil)

		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			logger:        util.TestAlloyLogger(t).Slog(),
			timerBookmark: 5e12,
			lastUptime:    5,
		}
		require.NoError(t, c.fetchQuerySamples(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		assert.Equal(t, 5e12, c.timerBookmark)
		assert.EqualValues(t, 6, c.lastUptime)
	})

	t.Run("statement that ends after a poll is collected by the next poll", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		// The timer is behind UPTIME: at uptime 10 it reads 9.6s. A window ending at UPTIME would end at 10s,
		// after the poll, before a statement that ends at 9.8s is in the table, and the next window would
		// start after that statement.
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(1010, 10),
		)
		expectLatestTimerEnd(mock, 9e12, 20e12, 9.6e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			9e12,
			9.6e12,
		).RowsWillBeClosed().WillReturnRows(querySamplesRows())

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(1011, 11),
		)
		expectLatestTimerEnd(mock, 9.6e12, 21e12, 10.6e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			9.6e12, // the next window starts where the previous one ended
			10.6e12,
		).RowsWillBeClosed().WillReturnRows(querySamplesRows(9.8e12))

		lokiClient := loki.NewCollectingHandler()
		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			entryHandler:  lokiClient,
			logger:        util.TestAlloyLogger(t).Slog(),
			timerBookmark: 9e12,
			lastUptime:    9,
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))
		require.NoError(t, c.fetchQuerySamples(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())

		lokiClient.Stop()
		require.Eventually(t, func() bool { return len(lokiClient.Received()) == 1 }, 5*time.Second, 100*time.Millisecond)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, lokiClient.Received()[0].Labels)
	})

	t.Run("statement that ended just before a poll is collected by that poll", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		// The timer is ahead of UPTIME: at uptime 10 it reads 10.4s. A window ending at UPTIME would end at 10s
		// and leave out a statement that ended at 10.2s, whose rows are gone by the next poll if its client
		// disconnects.
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(1010, 10),
		)
		expectLatestTimerEnd(mock, 9e12, 20e12, 10.4e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			9e12,
			10.4e12,
		).RowsWillBeClosed().WillReturnRows(querySamplesRows(10.2e12))

		lokiClient := loki.NewCollectingHandler()
		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			entryHandler:  lokiClient,
			logger:        util.TestAlloyLogger(t).Slog(),
			timerBookmark: 9e12,
			lastUptime:    9,
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))
		require.NoError(t, mock.ExpectationsWereMet())
		assert.Equal(t, 10.4e12, c.timerBookmark)

		lokiClient.Stop()
		require.Eventually(t, func() bool { return len(lokiClient.Received()) == 1 }, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("bookmarks are not updated if the latest timer query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{"now", "uptime"}).AddRow(1101, 101),
		)
		mock.ExpectQuery(selectLatestTimerEnd).WithArgs(3e12, 111e12).WillReturnError(fmt.Errorf("some error"))

		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			timerBookmark: 3e12,
			lastUptime:    100,
		}

		require.EqualError(t, c.fetchQuerySamples(t.Context()), "failed to fetch latest statement timer: some error")
		require.NoError(t, mock.ExpectationsWereMet())

		assert.EqualValues(t, 100, c.lastUptime)
		assert.Equal(t, 3e12, c.timerBookmark)
	})

	t.Run("bookmarks are not updated if selectNowAndUptime query fails", func(t *testing.T) {
		// Please note that if the loop breaks due to a rows scanning error, the bookmarks will have already been updated.
		// This means that the next iteration will use the updated bookmarks and some samples may be skipped.
		// This is a known limitation and is a best effort approach.

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnError(fmt.Errorf("some error"))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			float64(0),
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"event_id",
			"thread_id",
			"current_schema",
			"digest",
			"sql_text",
			"timer_wait",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"waits.event_id",
			"waits.end_event_id",
			"waits.event_name",
			"waits.object_name",
			"waits.object_type",
			"waits.timer_wait",
			"nested_waits.event_id",
			"nested_waits.end_event_id",
			"nested_waits.event_name",
			"nested_waits.object_name",
			"nested_waits.object_type",
			"nested_waits.timer_wait",
			"threads.PROCESSLIST_USER",
			"threads.PROCESSLIST_HOST",
			"cpu_time",
			"max_controlled_memory",
			"max_total_memory",
		}))
		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			timerBookmark: 3e12,
			lastUptime:    100,
		}

		require.Error(t, c.fetchQuerySamples(t.Context()))

		assert.EqualValues(t, 100, c.lastUptime)
		assert.Equal(t, 3e12, c.timerBookmark)
	})

	t.Run("returns error when selectNowAndUptime query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnError(fmt.Errorf("some error"))

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db, Logger: util.TestAlloyLogger(t).Slog()})
		require.NoError(t, err)

		err = c.fetchQuerySamples(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "failed to scan now and uptime info: some error", err.Error())
	})

	t.Run("returns error when selectQuerySamples query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(picosecondsToSeconds(math.MaxUint64)+15, 10))

		expectLatestTimerEnd(mock, 3e12, 20e12, 10e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", "", "", exclusionClause, "", endOfTimeline)).WithArgs(3e12, 10e12).WillReturnError(fmt.Errorf("some error"))

		c := &QuerySamples{
			dbConnection:  db,
			timerBookmark: 3e12,
		}
		err = c.fetchQuerySamples(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "failed to fetch query samples: some error", err.Error())
	})

	t.Run("continues even when parser.Redact fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(picosecondsToSeconds(math.MaxUint64)+15, 10))
		expectLatestTimerEnd(mock, 2e12, 20e12, 10e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			2e12,
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"current_schema",
			"digest",
			"sql_text",
			"timer_end",
			"timer_wait",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"errors",
			"waits.event_id",
			"waits.end_event_id",
			"waits.event_name",
			"waits.object_name",
			"waits.object_type",
			"waits.timer_wait",
			"nested_waits.event_id",
			"nested_waits.end_event_id",
			"nested_waits.event_name",
			"nested_waits.object_name",
			"nested_waits.object_type",
			"nested_waits.timer_wait",
			"threads.PROCESSLIST_USER",
			"threads.PROCESSLIST_HOST",
			"cpu_time",
			"max_controlled_memory",
			"max_total_memory",
		}).
			AddRow(
				"test_schema", // current_schema
				"some digest", // digest
				nil,           // sql_text
				2e12,          // timer_end
				2e12,          // timer_wait
				1000,          // rows_examined
				100,           // rows_sent
				0,             // rows_affected
				0,             // errors
				nil,           // WAIT_EVENT_ID
				nil,           // WAIT_END_EVENT_ID
				nil,           // WAIT_EVENT_NAME
				nil,           // WAIT_OBJECT_NAME
				nil,           // WAIT_OBJECT_TYPE
				nil,           // WAIT_TIME
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"some_user", // PROCESSLIST_USER
				"some_host", // PROCESSLIST_HOST
				555555,      // cpu_time
				1048576,     // max_controlled_memory (1MB)
				2097152,     // max_total_memory (2MB)
			),
		)
		mockParser := &parser.MockParser{}
		c := &QuerySamples{
			dbConnection:  db,
			engineVersion: latestCompatibleVersion,
			timerBookmark: 2e12,
			logger:        util.TestAlloyLogger(t).Slog(),
		}

		mockParser.On("CleanTruncatedText", "SELECT * FROM users").Return("SELECT * FROM users", nil)
		mockParser.On("Redact", "SELECT * FROM users").Return("", fmt.Errorf("some error"))

		err = c.fetchQuerySamples(t.Context())

		assert.NoError(t, err)
	})
}

// expectLatestTimerEnd expects the query for the latest statement timer value in (bookmark, ceiling], and
// returns latest from it.
func expectLatestTimerEnd(mock sqlmock.Sqlmock, bookmark, ceiling float64, latest any) {
	mock.ExpectQuery(selectLatestTimerEnd).WithArgs(bookmark, ceiling).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"MAX(TIMER_END)"}).AddRow(latest))
}

// expectLatestTimerEndAcrossOverflow is expectLatestTimerEnd for selectLatestTimerEndAcrossOverflow, which takes
// the ceiling before the bookmark.
func expectLatestTimerEndAcrossOverflow(mock sqlmock.Sqlmock, ceiling, bookmark float64, latest any) {
	mock.ExpectQuery(selectLatestTimerEndAcrossOverflow).WithArgs(ceiling, bookmark).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"latest"}).AddRow(latest))
}

// querySamplesRows returns rows of the query samples query, one statement per TIMER_END value, each lasting 1ms.
func querySamplesRows(timerEnds ...float64) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{
		"statements.CURRENT_SCHEMA",
		"statements.THREAD_ID",
		"statements.EVENT_ID",
		"statements.END_EVENT_ID",
		"statements.DIGEST",
		"statements.SQL_TEXT",
		"statements.TIMER_END",
		"statements.TIMER_WAIT",
		"statements.ROWS_EXAMINED",
		"statements.ROWS_SENT",
		"statements.ROWS_AFFECTED",
		"statements.ERRORS",
		"statements.MYSQL_ERRNO",
		"statements.RETURNED_SQLSTATE",
		"statements.MESSAGE_TEXT",
		"waits.event_id",
		"waits.end_event_id",
		"waits.event_name",
		"waits.object_name",
		"waits.object_type",
		"waits.timer_wait",
		"nested_waits.event_id",
		"nested_waits.end_event_id",
		"nested_waits.event_name",
		"nested_waits.object_name",
		"nested_waits.object_type",
		"nested_waits.timer_wait",
		"threads.PROCESSLIST_USER",
		"threads.PROCESSLIST_HOST",
		"statements.CPU_TIME",
		"statements.MAX_CONTROLLED_MEMORY",
		"statements.MAX_TOTAL_MEMORY",
	})
	for i, timerEnd := range timerEnds {
		rows.AddRow(
			"test_schema", 890, 100+i, 101+i, "some digest", nil, timerEnd, 1e9, 1, 1, 0, 0, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			"some_user", "some_host", 0, 0, 0,
		)
	}
	return rows
}

func TestQuerySamples_determineTimerWindow(t *testing.T) {
	overflow := float64(math.MaxUint64)
	overflowSeconds := picosecondsToSeconds(overflow)

	tests := map[string]struct {
		bookmark          float64
		bookmarkOverflows int
		uptime            float64
		acrossOverflow    bool    // whether the query for a window that may span an overflow is expected
		ceiling           float64 // expected ceiling argument
		latest            any     // the latest timer value the query returns
		expected          *timerWindow
	}{
		"window ends at the latest timer value, not at uptime": {
			bookmark: 99e12,
			uptime:   100,
			ceiling:  110e12,
			latest:   100.4e12,
			expected: &timerWindow{clause: endOfTimeline, limit: 100.4e12},
		},
		"window can end before uptime": {
			bookmark: 99e12,
			uptime:   100,
			ceiling:  110e12,
			latest:   99.6e12,
			expected: &timerWindow{clause: endOfTimeline, limit: 99.6e12},
		},
		"no statement ended since the bookmark": {
			bookmark: 99e12,
			uptime:   100,
			ceiling:  110e12,
			latest:   nil,
			expected: nil,
		},
		"uptime plus margin is past an overflow: the window stops at the overflow": {
			bookmark: overflow - 7e12,
			uptime:   overflowSeconds - 5,
			ceiling:  overflow,
			latest:   overflow - 4e12,
			expected: &timerWindow{clause: endOfTimeline, limit: overflow - 4e12},
		},
		"the timer may have overflowed before uptime does: the window stops at the overflow": {
			// A small timer value could be from after the overflow, or the latest statement of a thread that has
			// been idle since early in the current period, so the window doesn't look past the overflow yet.
			bookmark: overflow - 7e12,
			uptime:   overflowSeconds - 1,
			ceiling:  overflow,
			latest:   overflow - 0.5e12,
			expected: &timerWindow{clause: endOfTimeline, limit: overflow - 0.5e12},
		},
		"uptime is past an overflow by less than the timer can lag: the window stops at the overflow": {
			bookmark: overflow - 7e12,
			uptime:   overflowSeconds + 1,
			ceiling:  overflow,
			latest:   overflow - 0.2e12,
			expected: &timerWindow{clause: endOfTimeline, limit: overflow - 0.2e12},
		},
		"uptime shows that the timer has overflowed since the bookmark": {
			bookmark:       overflow - 7e12,
			uptime:         overflowSeconds + 2,
			acrossOverflow: true,
			ceiling:        12e12,
			latest:         1.5e12,
			expected:       &timerWindow{clause: beginningAndEndOfTimeline, limit: 1.5e12, overflows: 1},
		},
		"the timer has overflowed, but no statement has ended since the overflow": {
			bookmark:       overflow - 7e12,
			uptime:         overflowSeconds + 2,
			acrossOverflow: true,
			ceiling:        12e12,
			latest:         overflow - 1e12,
			expected:       &timerWindow{clause: endOfTimeline, limit: overflow - 1e12},
		},
		"already overflowed once": {
			bookmark:          1.5e12,
			bookmarkOverflows: 1,
			uptime:            overflowSeconds + 3,
			ceiling:           13e12,
			latest:            2.5e12,
			expected:          &timerWindow{clause: endOfTimeline, limit: 2.5e12, overflows: 1},
		},
		"second overflow occurs": {
			bookmark:          overflow - 7e12,
			bookmarkOverflows: 1,
			uptime:            overflowSeconds*2 + 2,
			acrossOverflow:    true,
			ceiling:           12e12,
			latest:            1.5e12,
			expected:          &timerWindow{clause: beginningAndEndOfTimeline, limit: 1.5e12, overflows: 2},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			if tc.acrossOverflow {
				expectLatestTimerEndAcrossOverflow(mock, tc.ceiling, tc.bookmark, tc.latest)
			} else {
				expectLatestTimerEnd(mock, tc.bookmark, tc.ceiling, tc.latest)
			}

			c := &QuerySamples{
				dbConnection:           db,
				timerBookmark:          tc.bookmark,
				timerBookmarkOverflows: tc.bookmarkOverflows,
			}

			window, err := c.determineTimerWindow(t.Context(), tc.uptime)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, window)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	t.Run("returns error when the query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectLatestTimerEnd).WithArgs(1e12, 11e12).WillReturnError(fmt.Errorf("some error"))

		c := &QuerySamples{dbConnection: db, timerBookmark: 1e12}

		_, err = c.determineTimerWindow(t.Context(), 1)
		require.EqualError(t, err, "failed to fetch latest statement timer: some error")
	})
}

func TestTimerWindow_overflowsBefore(t *testing.T) {
	t.Run("statements in a window that doesn't span an overflow", func(t *testing.T) {
		w := timerWindow{clause: endOfTimeline, limit: 5e12, overflows: 1}
		assert.Equal(t, 1, w.overflowsBefore(4e12))
		assert.Equal(t, 1, w.overflowsBefore(5e12))
	})

	t.Run("statements in a window that spans an overflow", func(t *testing.T) {
		w := timerWindow{clause: beginningAndEndOfTimeline, limit: 5e12, overflows: 1}
		assert.Equal(t, 1, w.overflowsBefore(4e12))
		assert.Equal(t, 0, w.overflowsBefore(float64(math.MaxUint64)-1e12))
	})
}

func TestQuerySamples_AutoEnableSetupConsumers(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("executes updateSetupConsumers query when autoEnableSetupConsumers is true", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                          db,
			EngineVersion:               latestCompatibleVersion,
			CollectInterval:             time.Second,
			EntryHandler:                lokiClient,
			Logger:                      util.TestAlloyLogger(t).Slog(),
			AutoEnableSetupConsumers:    true,
			SetupConsumersCheckInterval: time.Second,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				),
			)
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

		mock.ExpectExec(updateSetupConsumers).WithoutArgs().WillReturnResult(sqlmock.NewResult(0, 3))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
					"statements.SQL_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MYSQL_ERRNO",
					"statements.RETURNED_SQLSTATE",
					"statements.MESSAGE_TEXT",
					"waits.event_id",
					"waits.end_event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"nested_waits.event_id",
					"nested_waits.end_event_id",
					"nested_waits.event_name",
					"nested_waits.object_name",
					"nested_waits.object_type",
					"nested_waits.timer_wait",
					"threads.PROCESSLIST_USER",
					"threads.PROCESSLIST_HOST",
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
					nil,
					"70000000",
					"20000000",
					"5",
					"5",
					"0",
					"0",
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					"some_user",
					"some_host",
					"10000000",
					"456",
					"457",
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

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})

	t.Run("handles updateSetupConsumers query error gracefully", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                          db,
			EngineVersion:               latestCompatibleVersion,
			CollectInterval:             time.Second,
			EntryHandler:                lokiClient,
			Logger:                      util.TestAlloyLogger(t).Slog(),
			AutoEnableSetupConsumers:    true,
			SetupConsumersCheckInterval: time.Second,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				),
			)
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark

		mock.ExpectExec(updateSetupConsumers).WithoutArgs().WillReturnError(fmt.Errorf("setup consumers update failed"))

		err = collector.Start(t.Context())
		require.NoError(t, err)

		// Start runs the query in a background task and we need enough time
		// to pass so that the query has been triggered atleast once.
		time.Sleep(500 * time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})
}

func TestQuerySamplesExcludeSchemas(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQuerySamples(QuerySamplesArguments{
		DB:              db,
		EngineVersion:   latestCompatibleVersion,
		CollectInterval: time.Millisecond,
		ExcludeSchemas:  []string{"excluded_schema"},
		EntryHandler:    lokiClient,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	// Initialize the timerBookmark as Start() would do
	c.timerBookmark = 1e12

	mock.ExpectQuery(selectNowAndUptime).WithoutArgs().
		WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))

	// Verify the query uses the custom exclusion clause
	customClause := buildExcludedSchemasClause([]string{"excluded_schema"})
	expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
	mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", customClause, "", endOfTimeline)).
		WithArgs(1e12, 2e12).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"current_schema", "thread_id", "event_id", "end_event_id", "digest",
		"timer_end", "timer_wait", "rows_examined", "rows_sent", "rows_affected",
		"errors", "object_schema", "object_name", "object_type", "index_name",
		"lock_time", "digest_text", "threads.PROCESSLIST_USER", "threads.PROCESSLIST_HOST", "cpu_time", "max_controlled_memory", "max_total_memory",
	}))

	c.fetchQuerySamples(t.Context())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuerySamplesIncludeSchemas(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQuerySamples(QuerySamplesArguments{
		DB:              db,
		EngineVersion:   latestCompatibleVersion,
		CollectInterval: time.Millisecond,
		IncludeSchemas:  []string{"included_schema"},
		EntryHandler:    lokiClient,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	c.timerBookmark = 1e12

	mock.ExpectQuery(selectNowAndUptime).WithoutArgs().
		WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))

	customClause := buildSchemaFilterClause(nil, []string{"included_schema"}, "statements.CURRENT_SCHEMA")
	expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
	mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", customClause, "", endOfTimeline)).
		WithArgs(1e12, 2e12).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"current_schema", "thread_id", "event_id", "end_event_id", "digest",
		"timer_end", "timer_wait", "rows_examined", "rows_sent", "rows_affected",
		"errors", "object_schema", "object_name", "object_type", "index_name",
		"lock_time", "digest_text", "threads.PROCESSLIST_USER", "threads.PROCESSLIST_HOST", "cpu_time", "max_controlled_memory", "max_total_memory",
	}))

	require.NoError(t, c.fetchQuerySamples(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestQuerySamples_WaitEventCounter_MatchesLogLines pins the invariant that
// the counter delta for (digest, schema) equals the sum of wait_time on
// every emitted wait_event_v2 line for that key.
func TestQuerySamples_WaitEventCounter_MatchesLogLines(t *testing.T) {
	waitOp := database_observability.OP_WAIT_EVENT_V2
	t.Run("wait_event_v2", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()
		registry := prometheus.NewRegistry()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                            db,
			EngineVersion:                 latestCompatibleVersion,
			CollectInterval:               time.Second,
			EntryHandler:                  lokiClient,
			Registry:                      registry,
			Logger:                        util.TestAlloyLogger(t).Slog(),
			EnablePreClassifiedWaitEvents: true,
		})
		require.NoError(t, err)
		require.NotNil(t, collector.waitEventCounter)

		rows := [][]driver.Value{
			// (digest_A, schema_X): two rows, 0.1ms + 0.2ms = 0.3ms
			{"schema_X", "1", "10", "11", "digest_A", "sql1", "70000000", "20000000", "5", "5", "0", "0", nil, nil, nil, "100", "101", "wait/io/file/x", "obj", "typ", "100000000", nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
			{"schema_X", "1", "20", "21", "digest_A", "sql1", "70000000", "20000000", "5", "5", "0", "0", nil, nil, nil, "200", "201", "wait/io/file/y", "obj", "typ", "200000000", nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
			// (digest_B, schema_X): 0.05ms
			{"schema_X", "1", "30", "31", "digest_B", "sql2", "70000000", "20000000", "5", "5", "0", "0", nil, nil, nil, "300", "301", "wait/lock/metadata", "obj", "typ", "50000000", nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
			// (digest_A, schema_Y): 0.075ms — same digest, different schema
			{"schema_Y", "1", "40", "41", "digest_A", "sql1", "70000000", "20000000", "5", "5", "0", "0", nil, nil, nil, "400", "401", "wait/synch/mutex", "obj", "typ", "75000000", nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
			// (digest_D, schema_X): wait/io/table/sql/handler molecule with two
			// nested atoms (same outer event_id, two rows from the JOIN). Pre-fix,
			// the counter added outer 5ms twice (over-count by N). Post-fix, each
			// row contributes its own nested timer, matching the log line.
			// Expected counter contribution: 0.4ms + 0.6ms = 1.0ms total.
			{"schema_X", "1", "60", "61", "digest_D", "sql4", "70000000", "20000000", "5", "5", "0", "0", nil, nil, nil, "500", "501", "wait/io/table/sql/handler", "books", "TABLE", "5000000000", "510", "511", "wait/io/file/innodb/innodb_data_file", "ibdata1", "FILE", "400000000", "u", "h", "1000", "1", "1"},
			{"schema_X", "1", "60", "61", "digest_D", "sql4", "70000000", "20000000", "5", "5", "0", "0", nil, nil, nil, "500", "501", "wait/io/table/sql/handler", "books", "TABLE", "5000000000", "520", "521", "wait/io/file/innodb/innodb_data_file", "ibdata1", "FILE", "600000000", "u", "h", "1000", "1", "1"},
			// digest_C: no wait event (nil wait fields) — must not increment any counter
			{"schema_X", "1", "50", "51", "digest_C", "sql3", "70000000", "20000000", "5", "5", "0", "0", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
		}

		mockRows := sqlmock.NewRows([]string{
			"statements.CURRENT_SCHEMA", "statements.THREAD_ID", "statements.EVENT_ID",
			"statements.END_EVENT_ID", "statements.DIGEST", "statements.SQL_TEXT", "statements.TIMER_END",
			"statements.TIMER_WAIT", "statements.ROWS_EXAMINED", "statements.ROWS_SENT",
			"statements.ROWS_AFFECTED", "statements.ERRORS", "statements.MYSQL_ERRNO", "statements.RETURNED_SQLSTATE", "statements.MESSAGE_TEXT",
			"waits.event_id", "waits.end_event_id", "waits.event_name",
			"waits.object_name", "waits.object_type", "waits.timer_wait",
			"nested_waits.event_id", "nested_waits.end_event_id", "nested_waits.event_name",
			"nested_waits.object_name", "nested_waits.object_type", "nested_waits.timer_wait",
			"threads.PROCESSLIST_USER", "threads.PROCESSLIST_HOST",
			"statements.CPU_TIME", "statements.MAX_CONTROLLED_MEMORY", "statements.MAX_TOTAL_MEMORY",
		})
		for _, r := range rows {
			mockRows = mockRows.AddRow(r...)
		}

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, 2e12,
		).RowsWillBeClosed().WillReturnRows(mockRows)

		require.NoError(t, collector.Start(t.Context()))

		// 6 query samples (digest_D's two rows dedup to one) + 6 wait events = 12 entries.
		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 12
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()
		require.Eventually(t, collector.Stopped, 5*time.Second, 100*time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())

		// Build expected counter values by parsing the emitted log lines so
		// the assertion drifts with the emitter rather than with hard-coded sums.
		digestRe := regexp.MustCompile(`digest="([^"]+)"`)
		schemaRe := regexp.MustCompile(`schema="([^"]+)"`)
		waitTimeRe := regexp.MustCompile(`wait_time="([^"]+)"`)
		expected := map[[2]string]float64{}
		waitEventEntries := 0
		for _, e := range lokiClient.Received() {
			if e.Labels["op"] != model.LabelValue(waitOp) {
				continue
			}
			waitEventEntries++
			d := digestRe.FindStringSubmatch(e.Line)
			s := schemaRe.FindStringSubmatch(e.Line)
			w := waitTimeRe.FindStringSubmatch(e.Line)
			require.Len(t, d, 2, "digest not found in line: %s", e.Line)
			require.Len(t, s, 2, "schema not found in line: %s", e.Line)
			require.Len(t, w, 2, "wait_time not found in line: %s", e.Line)
			dur, err := time.ParseDuration(w[1])
			require.NoError(t, err)
			expected[[2]string{d[1], s[1]}] += dur.Seconds()
		}
		require.Equal(t, 6, waitEventEntries, "should emit one wait-event log per row with a valid wait event")
		require.Len(t, expected, 4, "digest_C has no wait event, so only 4 (digest, schema) groups should be seen")

		for key, expSec := range expected {
			counter, err := collector.waitEventCounter.GetMetricWith(prometheus.Labels{
				"digest": key[0],
				"schema": key[1],
			})
			require.NoError(t, err)
			var m dto.Metric
			require.NoError(t, counter.Write(&m))
			assert.InDelta(t, expSec, m.Counter.GetValue(), 1e-9,
				"counter for digest=%s schema=%s does not match sum of logged wait_time", key[0], key[1])
		}
	})
}

func TestQuerySamples_WaitEvents_PreClassified(t *testing.T) {
	t.Run("flag OFF emits only OP_WAIT_EVENT without wait_event_type", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                            db,
			EngineVersion:                 latestCompatibleVersion,
			CollectInterval:               time.Second,
			EntryHandler:                  lokiClient,
			Logger:                        util.TestAlloyLogger(t).Slog(),
			EnablePreClassifiedWaitEvents: false,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, 2e12,
		).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows([]string{
				"statements.CURRENT_SCHEMA", "statements.THREAD_ID", "statements.EVENT_ID",
				"statements.END_EVENT_ID", "statements.DIGEST", "statements.SQL_TEXT", "statements.TIMER_END",
				"statements.TIMER_WAIT", "statements.ROWS_EXAMINED", "statements.ROWS_SENT",
				"statements.ROWS_AFFECTED", "statements.ERRORS", "statements.MYSQL_ERRNO", "statements.RETURNED_SQLSTATE", "statements.MESSAGE_TEXT",
				"waits.event_id", "waits.end_event_id", "waits.event_name",
				"waits.object_name", "waits.object_type", "waits.timer_wait",
				"nested_waits.event_id", "nested_waits.end_event_id", "nested_waits.event_name",
				"nested_waits.object_name", "nested_waits.object_type", "nested_waits.timer_wait",
				"threads.PROCESSLIST_USER", "threads.PROCESSLIST_HOST",
				"statements.CPU_TIME", "statements.MAX_CONTROLLED_MEMORY", "statements.MAX_TOTAL_MEMORY",
			}).AddRow(
				"some_schema", "890", "123", "234", "some_digest", "some_sql_text",
				"70000000", "20000000", "5", "5", "0", "0",
				nil, nil, nil,
				"124", "124", "wait/io/file/innodb/innodb_data_file",
				"wait_object_name", "wait_object_type", "100000000",
				nil, nil, nil, nil, nil, nil,
				"some_user", "some_host",
				"10000000", "456", "457",
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

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.NotContains(t, lokiEntries[1].Line, "wait_event_type=")
	})

	t.Run("flag ON emits only OP_WAIT_EVENT_V2 with wait_event_type classified", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                            db,
			EngineVersion:                 latestCompatibleVersion,
			CollectInterval:               time.Second,
			EntryHandler:                  lokiClient,
			Logger:                        util.TestAlloyLogger(t).Slog(),
			EnablePreClassifiedWaitEvents: true,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		expectLatestTimerEnd(mock, float64(0), 11e12, 1e12) // the initial timerBookmark
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		expectLatestTimerEnd(mock, 1e12, 11e12, 2e12)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, 2e12,
		).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows([]string{
				"statements.CURRENT_SCHEMA", "statements.THREAD_ID", "statements.EVENT_ID",
				"statements.END_EVENT_ID", "statements.DIGEST", "statements.SQL_TEXT", "statements.TIMER_END",
				"statements.TIMER_WAIT", "statements.ROWS_EXAMINED", "statements.ROWS_SENT",
				"statements.ROWS_AFFECTED", "statements.ERRORS", "statements.MYSQL_ERRNO", "statements.RETURNED_SQLSTATE", "statements.MESSAGE_TEXT",
				"waits.event_id", "waits.end_event_id", "waits.event_name",
				"waits.object_name", "waits.object_type", "waits.timer_wait",
				"nested_waits.event_id", "nested_waits.end_event_id", "nested_waits.event_name",
				"nested_waits.object_name", "nested_waits.object_type", "nested_waits.timer_wait",
				"threads.PROCESSLIST_USER", "threads.PROCESSLIST_HOST",
				"statements.CPU_TIME", "statements.MAX_CONTROLLED_MEMORY", "statements.MAX_TOTAL_MEMORY",
			}).AddRow(
				"some_schema", "890", "123", "234", "some_digest", "some_sql_text",
				"70000000", "20000000", "5", "5", "0", "0",
				nil, nil, nil,
				"124", "124", "wait/io/file/innodb/innodb_data_file",
				"wait_object_name", "wait_object_type", "100000000",
				nil, nil, nil, nil, nil, nil,
				"some_user", "some_host",
				"10000000", "456", "457",
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

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		assert.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT_V2}, lokiEntries[1].Labels)
		assert.Contains(t, lokiEntries[1].Line, `wait_event_type="IO Wait"`)
		// structured metadata labels should not contain wait_event_type
		assert.NotContains(t, string(lokiEntries[1].Labels["wait_event_type"]), "IO Wait")
	})
}

func TestClassifyMySQLWaitEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// IO
		{"wait/io/file/innodb/innodb_data_file", "IO Wait"},
		{"wait/io/file/sql/binlog", "IO Wait"},
		{"wait/io/file/sql/io_cache", "IO Wait"},
		{"wait/io/file/sql/slow_log", "IO Wait"},
		{"wait/io/table/sql/handler", "IO Wait"},

		// Network
		{"wait/io/socket/sql/client_connection", "Network Wait"},

		// Lock (cascade)
		{"wait/io/lock/table/handler", "Lock Wait"},
		{"wait/lock/table/sql/handler", "Lock Wait"},
		{"wait/lock/metadata/sql/mdl", "Lock Wait"},

		// Engine
		{"wait/synch/mutex/sql/LOCK_open", "Engine Wait"},
		{"wait/synch/mutex/sql/LOCK_table_cache", "Engine Wait"},
		{"wait/synch/mutex/sql/LOG::LOCK_log", "Engine Wait"},
		{"wait/synch/mutex/sql/MYSQL_BIN_LOG::LOCK_done", "Engine Wait"},
		{"wait/synch/mutex/sql/LOCK_global_system_variables", "Engine Wait"},
		{"wait/synch/cond/sql/MYSQL_BIN_LOG::COND_done", "Engine Wait"},
		{"wait/synch/rwlock/sql/LOCK_system_variables_hash", "Engine Wait"},
		{"wait/synch/prlock/sql/MDL_lock::rwlock", "Engine Wait"},
		{"wait/synch/mutex/innodb/trx_mutex", "Engine Wait"},
		{"wait/synch/mutex/innodb/dict_table_mutex", "Engine Wait"},

		// Replication
		{"wait/io/file/sql/relaylog", "Replication Wait"},
		{"wait/io/file/sql/relaylog_index", "Replication Wait"},
		{"wait/synch/mutex/sql/Slave_jobs_lock", "Replication Wait"},
		{"wait/synch/mutex/sql/Slave_worker::jobs_lock", "Replication Wait"},
		{"wait/synch/cond/sql/Slave_worker::jobs_cond", "Replication Wait"},
		{"wait/synch/mutex/sql/Relay_log_info::pending_jobs_lock", "Replication Wait"},
		{"wait/synch/mutex/sql/Relay_log_info::log_space_lock", "Replication Wait"},
		// MySQL 8.0.22+ renamed Slave_* to Replica_*.
		{"wait/synch/mutex/sql/Replica_jobs_lock", "Replication Wait"},
		{"wait/synch/mutex/sql/Replica_committed_queue_lock", "Replication Wait"},
		// MySQL 5.7 used Master_info, MySQL 8.0+ uses Source_info.
		{"wait/synch/mutex/sql/Master_info::data_lock", "Replication Wait"},
		{"wait/synch/mutex/sql/Source_info::data_lock", "Replication Wait"},
		// Relay-log internal mutexes and multi-threaded replica coordination.
		{"wait/synch/mutex/sql/MYSQL_RELAY_LOG::LOCK_log", "Replication Wait"},
		{"wait/synch/mutex/sql/Mts_submode_logical_clock::data_lock", "Replication Wait"},

		// Other / unknown
		{"wait/unknown/something", "Other Wait"},
		{"not_a_wait_event", "Other Wait"},
		{"", "Other Wait"},
		{"idle", "Other Wait"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := classifyMySQLWaitEventType(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
