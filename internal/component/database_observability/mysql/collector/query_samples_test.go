package collector

import (
	"database/sql/driver"
	"fmt"
	"math"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
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
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
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
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\" traceparent=\"00-00bd5199fe2a4c8506368b55ef212cf1-d49c5e2fb232379b-01\"",
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
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
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
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
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
				"some_user",
				"some_host",
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
				{"op": OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
				"level=\"info\" schema=\"some_other_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"891\" event_id=\"124\" end_event_id=\"235\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
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
			lokiClient := loki.NewCollectingHandler()

			collector, err := NewQuerySamples(QuerySamplesArguments{
				DB:              db,
				EngineVersion:   latestCompatibleVersion,
				CollectInterval: time.Second,
				EntryHandler:    lokiClient,
				Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
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

			mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
				sqlmock.NewRows([]string{
					"now",
					"uptime",
				}).AddRow(
					5,
					1,
				))

			mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
				1e12, // initial timerBookmark
				1e12,
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
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
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
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
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
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"125\" wait_event_name=\"wait/lock/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"0.000150ms\"", lokiEntries[1].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[2].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"126\" wait_end_event_id=\"126\" wait_event_name=\"wait/lock/table/sql/handler\" wait_object_name=\"categories\" wait_object_type=\"TABLE\" wait_time=\"0.000350ms\"", lokiEntries[2].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[3].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"127\" wait_end_event_id=\"127\" wait_event_name=\"wait/io/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"0.000500ms\"", lokiEntries[3].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[4].Labels)
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
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"125\" wait_event_name=\"wait/lock/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"0.000150ms\"", lokiEntries[1].Line)
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[2].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"126\" end_event_id=\"234\" digest=\"another_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[2].Line)
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
			Logger:                log.NewLogfmtLogger(os.Stderr),
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

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, // initial timerBookmark
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\" sql_text=\"select * from some_table where id = 1\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
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
			Logger:               log.NewLogfmtLogger(os.Stderr),
			WaitEventMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "AND waits.timer_wait >= 1000000000", "AND nested_waits.timer_wait >= 1000000000", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
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
			Logger:               log.NewLogfmtLogger(os.Stderr),
			WaitEventMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "AND waits.timer_wait >= 1000000000", "AND nested_waits.timer_wait >= 1000000000", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
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
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
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
			Logger:                        log.NewLogfmtLogger(os.Stderr),
			EnablePreClassifiedWaitEvents: true,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT_V2}, lokiEntries[1].Labels)
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
			Logger:               log.NewLogfmtLogger(os.Stderr),
			WaitEventMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "AND waits.timer_wait >= 1000000000", "AND nested_waits.timer_wait >= 1000000000", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
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
			Logger:            log.NewLogfmtLogger(os.Stderr),
			SampleMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "AND statements.TIMER_WAIT >= 1000000000", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
			Logger:            log.NewLogfmtLogger(os.Stderr),
			SampleMinDuration: 1 * time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "AND statements.TIMER_WAIT >= 1000000000", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
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
			Logger:                log.NewLogfmtLogger(os.Stderr),
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

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(1e12, 1e12).RowsWillBeClosed().
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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\" sql_text=\"select * from some_table where id = 1\"", lokiEntries[0].Line)
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
			Logger:                log.NewLogfmtLogger(os.Stderr),
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

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(1e12, 1e12).RowsWillBeClosed().
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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
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
			expectedLogOutput: `level="info" schema="test_schema" user="some_user" client_host="some_host" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="0b" max_total_memory="0b" cpu_time="0.000000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
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
			expectedLogOutput: `level="info" schema="test_schema" user="some_user" client_host="some_host" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="0b" max_total_memory="0b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
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
			expectedLogOutput: `level="info" schema="test_schema" user="some_user" client_host="some_host" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="1024b" max_total_memory="2048b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
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
				Logger:          log.NewLogfmtLogger(os.Stderr),
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

			mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
				sqlmock.NewRows([]string{
					"now",
					"uptime",
				}).AddRow(
					5,
					1,
				))

			mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, tc.expectedFields, "", "", exclusionClause, "", endOfTimeline)).WithArgs(1e12, 1e12).RowsWillBeClosed().
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
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
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
			Logger:          log.NewLogfmtLogger(os.Stderr),
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

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
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
			Logger:          log.NewLogfmtLogger(os.Stderr),
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

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				2,
			))

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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
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
			Logger:          log.NewLogfmtLogger(os.Stderr),
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

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				2,
			))

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
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
	})
}

func TestQuerySamples_initializeTimer(t *testing.T) {
	t.Run("selects uptime, sets timerBookmark", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{
			"uptime",
		}).AddRow(
			5,
		))

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db})
		require.NoError(t, err)

		require.NoError(t, c.initializeBookmark(t.Context()))

		assert.Equal(t, 5e12, c.timerBookmark)
	})

	t.Run("sets timerBookmark as uptime modulo overflows (uptime is comprised of 1 overflow)", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{
			"uptime",
		}).AddRow(
			picosecondsToSeconds(math.MaxUint64) + 5,
		))

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db})
		require.NoError(t, err)

		require.NoError(t, c.initializeBookmark(t.Context()))

		assert.Equal(t, 5e12, c.timerBookmark)
	})
}

func TestQuerySamples_handles_timer_overflows(t *testing.T) {
	t.Run("selects query sample summary: first run uses initialized timerBookmark and uptime limit", func(t *testing.T) {
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, // initial timerBookmark
			5e12, // uptime of 5 seconds in picoseconds (modulo 0 overflows)
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
			logger:        log.NewLogfmtLogger(os.Stderr),
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))

		assert.Equal(t, 5e12, c.timerBookmark) // timerBookmark is updated to the uptime in picoseconds
		assert.EqualValues(t, 5, c.lastUptime) // lastUptime is updated to the uptime in seconds

		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)
		require.Len(t, lokiClient.Received(), 1)
		assert.Equal(t, model.LabelSet{
			"op": OP_QUERY_SAMPLE,
		}, lokiClient.Received()[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"test_schema\" user=\"some_user\" client_host=\"some_host\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some digest\" rows_examined=\"1000\" rows_sent=\"100\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"1048576b\" max_total_memory=\"2097152b\" cpu_time=\"0.000556ms\" elapsed_time=\"2000.000000ms\" elapsed_time_ms=\"2000.000000ms\"", lokiClient.Received()[0].Line)
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, // initial timerBookmark
			5e12, // uptime of 5 seconds in picoseconds (modulo 0 overflows)
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
			logger:        log.NewLogfmtLogger(os.Stderr),
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))
	})

	t.Run("overflow has just happened: select with beginningAndEndOfTimeline clause, uptimeLimit is modulo overflows", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				picosecondsToSeconds(math.MaxUint64)+15,
				picosecondsToSeconds(math.MaxUint64)+10,
			),
		)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", beginningAndEndOfTimeline)).WithArgs( // asserts that beginningAndEndOfTimeline clause is used
			3e12,
			10e12, // uptimeLimit is calculated as uptime "modulo" overflows: (uptime - 1 overflow) in picoseconds
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
		}

		require.NoError(t, c.fetchQuerySamples(t.Context()))

		assert.EqualValues(t, picosecondsToSeconds(math.MaxUint64)+10, c.lastUptime)
		assert.Equal(t, 10e12, c.timerBookmark)
	})

	t.Run("overflow just happened, next query reverts back to endOfTimeline clause", func(t *testing.T) {
		// Below is the first query after an overflow just happened. The special beginningAndEndOfTimeline clause is used.
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				picosecondsToSeconds(math.MaxUint64)+15,
				picosecondsToSeconds(math.MaxUint64)+10,
			),
		)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", beginningAndEndOfTimeline)).WithArgs(
			3e12,
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"thread_id",
			"event_id",
			"end_event_id",
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
		}
		require.NoError(t, c.fetchQuerySamples(t.Context()))

		// Below, we want to assert that the subsequent query reverts back to the endOfTimeline clause.
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				picosecondsToSeconds(math.MaxUint64)+18,
				picosecondsToSeconds(math.MaxUint64)+13,
			),
		)
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs( // asserts revert to endOfTimeline clause
			10e12, // asserts timerBookmark has been updated to the previous uptimeLimit
			13e12, // asserts uptimeLimit is now updated to the current uptime "modulo" overflows
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
		require.NoError(t, c.fetchQuerySamples(t.Context()))

		assert.Equal(t, picosecondsToSeconds(math.MaxUint64)+13, c.lastUptime)
		assert.Equal(t, 13e12, c.timerBookmark)
	})

	t.Run("server restarts, timer bookmark is reset", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				picosecondsToSeconds(math.MaxUint64)+15,
				10,
			),
		)
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
			lastUptime:    11,
		}
		require.NoError(t, c.fetchQuerySamples(t.Context()))

		assert.EqualValues(t, 10, c.lastUptime)
		assert.Equal(t, 10e12, c.timerBookmark)
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

		c, err := NewQuerySamples(QuerySamplesArguments{DB: db})
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
			logger:        log.NewLogfmtLogger(os.Stderr),
		}

		mockParser.On("CleanTruncatedText", "SELECT * FROM users").Return("SELECT * FROM users", nil)
		mockParser.On("Redact", "SELECT * FROM users").Return("", fmt.Errorf("some error"))

		err = c.fetchQuerySamples(t.Context())

		assert.NoError(t, err)
	})
}

func TestQuerySamples_calculateTimerClauseAndLimit(t *testing.T) {
	tests := map[string]struct {
		lastUptime          float64
		uptime              float64
		expectedTimerClause string
		expectedLimit       float64
	}{
		"no overflows yet": {
			lastUptime:          99,
			uptime:              1000,
			expectedTimerClause: endOfTimeline,
			expectedLimit:       secondsToPicoseconds(1000),
		},
		"just overflowed": {
			lastUptime:          99,
			uptime:              picosecondsToSeconds(float64(math.MaxUint64)) + 10,
			expectedTimerClause: beginningAndEndOfTimeline, // switches clause
			expectedLimit:       secondsToPicoseconds(10),  // uptime "modulo" overflows
		},
		"already overflowed once": {
			lastUptime:          picosecondsToSeconds(float64(math.MaxUint64)) + 5,
			uptime:              picosecondsToSeconds(float64(math.MaxUint64)) + 10,
			expectedTimerClause: endOfTimeline,
			expectedLimit:       secondsToPicoseconds(10),
		},
		"second overflow occurs": {
			lastUptime:          picosecondsToSeconds(float64(math.MaxUint64)) + 5,
			uptime:              picosecondsToSeconds(float64(math.MaxUint64)*2) + 50.0,
			expectedTimerClause: beginningAndEndOfTimeline,
			expectedLimit:       secondsToPicoseconds(50.0), // uptime "modulo" overflows
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &QuerySamples{
				lastUptime: tc.lastUptime,
			}

			actualTimerClause, actualLimit := c.determineTimerClauseAndLimit(tc.uptime)

			assert.Equal(t, tc.expectedTimerClause, actualTimerClause)
			assert.Equal(t, tc.expectedLimit, actualLimit)
		})
	}
}

func Test_TryExtractTraceParent(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid traceparent with single quotes",
			input:    "SELECT * FROM users /*traceparent='00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01'*/",
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "valid traceparent with double quotes",
			input:    `SELECT * FROM users /*traceparent="00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"*/`,
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "valid traceparent without quotes",
			input:    "SELECT * FROM users /*traceparent=00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01*/",
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "traceparent with mixed case keyword",
			input:    "SELECT * FROM users /*TraceParent='00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01'*/",
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "traceparent among other comment fields",
			input:    "SELECT * FROM users /*controller='index',traceparent='00-abc123-def456-01',framework='django'*/",
			expected: "00-abc123-def456-01",
		},
		{
			name:     "no traceparent in SQL",
			input:    "SELECT * FROM users WHERE id = 1",
			expected: "",
		},
		{
			name:     "truncated SQL ending with ...",
			input:    "SELECT * FROM users WHERE id = 1 /*traceparent='00-abc...",
			expected: "",
		},
		{
			name:     "truncated as traceparent=... ",
			input:    "SELECT * FROM users WHERE id = 1 /*traceparent=...",
			expected: "",
		},
		{
			name:     "truncated as traceparent=",
			input:    "SELECT * FROM users WHERE id = 1 /*traceparent=",
			expected: "",
		},
		{
			name:     "traceparent without closing quote",
			input:    "SELECT * FROM users /*traceparent='00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			expected: "",
		},
		{
			name:     "empty traceparent value",
			input:    "SELECT * FROM users /*traceparent=''*/",
			expected: "",
		},
		{
			name:     "traceparent with whitespace",
			input:    "SELECT * FROM users /*traceparent='  00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01  '*/",
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "multiple traceparent occurrences - last one wins",
			input:    "SELECT * FROM users /*traceparent='00-first-first-01'*/ /*traceparent='00-second-second-02'*/",
			expected: "00-second-second-02",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name: "SQLCommenter exhibit",
			// Note that traceparent and value (W3C trace context) cannot have meta characters nor URL to decode, so they are effectively inert to tryExtractTraceParent
			input: `SELECT * FROM FOO /*action='%2Fparam*\'d',controller='index,'framework='spring',` +
				"\n" + `traceparent='00-5bd66ef5095369c7b0d1f8f4bd33716a-c532cb4098ac3dd2-01',` +
				"\n" + `tracestate='congo%3Dt61rcWkgMzE%2Crojo%3D00f067aa0ba902b7'*/`,
			expected: "00-5bd66ef5095369c7b0d1f8f4bd33716a-c532cb4098ac3dd2-01",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tryExtractTraceParent(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
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
			Logger:                      log.NewLogfmtLogger(os.Stderr),
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

		mock.ExpectExec(updateSetupConsumers).WithoutArgs().WillReturnResult(sqlmock.NewResult(0, 3))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				5,
				1,
			))

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12,
			1e12,
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
			Logger:                      log.NewLogfmtLogger(os.Stderr),
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
		Logger:          log.NewLogfmtLogger(os.Stderr),
	})
	require.NoError(t, err)

	// Initialize the timerBookmark as Start() would do
	c.timerBookmark = 1e12

	mock.ExpectQuery(selectNowAndUptime).WithoutArgs().
		WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))

	// Verify the query uses the custom exclusion clause
	customClause := buildExcludedSchemasClause([]string{"excluded_schema"})
	mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", customClause, "", endOfTimeline)).
		WithArgs(1e12, 1e12).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{
		"current_schema", "thread_id", "event_id", "end_event_id", "digest",
		"timer_end", "timer_wait", "rows_examined", "rows_sent", "rows_affected",
		"errors", "object_schema", "object_name", "object_type", "index_name",
		"lock_time", "digest_text", "threads.PROCESSLIST_USER", "threads.PROCESSLIST_HOST", "cpu_time", "max_controlled_memory", "max_total_memory",
	}))

	c.fetchQuerySamples(t.Context())
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestQuerySamples_WaitEventCounter_MatchesLogLines pins the invariant that
// the counter delta for (digest, schema) equals the sum of wait_time on
// every emitted wait_event_v2 line for that key.
func TestQuerySamples_WaitEventCounter_MatchesLogLines(t *testing.T) {
	waitOp := OP_WAIT_EVENT_V2
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
			Logger:                        log.NewLogfmtLogger(os.Stderr),
			EnablePreClassifiedWaitEvents: true,
		})
		require.NoError(t, err)
		require.NotNil(t, collector.waitEventCounter)

		rows := [][]driver.Value{
			// (digest_A, schema_X): two rows, 0.1ms + 0.2ms = 0.3ms
			{"schema_X", "1", "10", "11", "digest_A", "sql1", "70000000", "20000000", "5", "5", "0", "0", "100", "101", "wait/io/file/x", "obj", "typ", "100000000", nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
			{"schema_X", "1", "20", "21", "digest_A", "sql1", "70000000", "20000000", "5", "5", "0", "0", "200", "201", "wait/io/file/y", "obj", "typ", "200000000", nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
			// (digest_B, schema_X): 0.05ms
			{"schema_X", "1", "30", "31", "digest_B", "sql2", "70000000", "20000000", "5", "5", "0", "0", "300", "301", "wait/lock/metadata", "obj", "typ", "50000000", nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
			// (digest_A, schema_Y): 0.075ms — same digest, different schema
			{"schema_Y", "1", "40", "41", "digest_A", "sql1", "70000000", "20000000", "5", "5", "0", "0", "400", "401", "wait/synch/mutex", "obj", "typ", "75000000", nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
			// (digest_D, schema_X): wait/io/table/sql/handler molecule with two
			// nested atoms (same outer event_id, two rows from the JOIN). Pre-fix,
			// the counter added outer 5ms twice (over-count by N). Post-fix, each
			// row contributes its own nested timer, matching the log line.
			// Expected counter contribution: 0.4ms + 0.6ms = 1.0ms total.
			{"schema_X", "1", "60", "61", "digest_D", "sql4", "70000000", "20000000", "5", "5", "0", "0", "500", "501", "wait/io/table/sql/handler", "books", "TABLE", "5000000000", "510", "511", "wait/io/file/innodb/innodb_data_file", "ibdata1", "FILE", "400000000", "u", "h", "1000", "1", "1"},
			{"schema_X", "1", "60", "61", "digest_D", "sql4", "70000000", "20000000", "5", "5", "0", "0", "500", "501", "wait/io/table/sql/handler", "books", "TABLE", "5000000000", "520", "521", "wait/io/file/innodb/innodb_data_file", "ibdata1", "FILE", "600000000", "u", "h", "1000", "1", "1"},
			// digest_C: no wait event (nil wait fields) — must not increment any counter
			{"schema_X", "1", "50", "51", "digest_C", "sql3", "70000000", "20000000", "5", "5", "0", "0", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "u", "h", "1000", "1", "1"},
		}

		mockRows := sqlmock.NewRows([]string{
			"statements.CURRENT_SCHEMA", "statements.THREAD_ID", "statements.EVENT_ID",
			"statements.END_EVENT_ID", "statements.DIGEST", "statements.SQL_TEXT", "statements.TIMER_END",
			"statements.TIMER_WAIT", "statements.ROWS_EXAMINED", "statements.ROWS_SENT",
			"statements.ROWS_AFFECTED", "statements.ERRORS",
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
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, 1e12,
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
			Logger:                        log.NewLogfmtLogger(os.Stderr),
			EnablePreClassifiedWaitEvents: false,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, 1e12,
		).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows([]string{
				"statements.CURRENT_SCHEMA", "statements.THREAD_ID", "statements.EVENT_ID",
				"statements.END_EVENT_ID", "statements.DIGEST", "statements.SQL_TEXT", "statements.TIMER_END",
				"statements.TIMER_WAIT", "statements.ROWS_EXAMINED", "statements.ROWS_SENT",
				"statements.ROWS_AFFECTED", "statements.ERRORS",
				"waits.event_id", "waits.end_event_id", "waits.event_name",
				"waits.object_name", "waits.object_type", "waits.timer_wait",
				"nested_waits.event_id", "nested_waits.end_event_id", "nested_waits.event_name",
				"nested_waits.object_name", "nested_waits.object_type", "nested_waits.timer_wait",
				"threads.PROCESSLIST_USER", "threads.PROCESSLIST_HOST",
				"statements.CPU_TIME", "statements.MAX_CONTROLLED_MEMORY", "statements.MAX_TOTAL_MEMORY",
			}).AddRow(
				"some_schema", "890", "123", "234", "some_digest", "some_sql_text",
				"70000000", "20000000", "5", "5", "0", "0",
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
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
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
			Logger:                        log.NewLogfmtLogger(os.Stderr),
			EnablePreClassifiedWaitEvents: true,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, "", "", exclusionClause, "", endOfTimeline)).WithArgs(
			1e12, 1e12,
		).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows([]string{
				"statements.CURRENT_SCHEMA", "statements.THREAD_ID", "statements.EVENT_ID",
				"statements.END_EVENT_ID", "statements.DIGEST", "statements.SQL_TEXT", "statements.TIMER_END",
				"statements.TIMER_WAIT", "statements.ROWS_EXAMINED", "statements.ROWS_SENT",
				"statements.ROWS_AFFECTED", "statements.ERRORS",
				"waits.event_id", "waits.end_event_id", "waits.event_name",
				"waits.object_name", "waits.object_type", "waits.timer_wait",
				"nested_waits.event_id", "nested_waits.end_event_id", "nested_waits.event_name",
				"nested_waits.object_name", "nested_waits.object_type", "nested_waits.timer_wait",
				"threads.PROCESSLIST_USER", "threads.PROCESSLIST_HOST",
				"statements.CPU_TIME", "statements.MAX_CONTROLLED_MEMORY", "statements.MAX_TOTAL_MEMORY",
			}).AddRow(
				"some_schema", "890", "123", "234", "some_digest", "some_sql_text",
				"70000000", "20000000", "5", "5", "0", "0",
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
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT_V2}, lokiEntries[1].Labels)
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
