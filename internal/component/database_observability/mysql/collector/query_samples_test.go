package collector

import (
	"database/sql/driver"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
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
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
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
				"10000000",
				"456",
				"457",
			}, {
				"some_other_schema",
				"891",
				"124",
				"235",
				"some_digest",
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
				"10000000",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
				{"op": OP_QUERY_SAMPLE},
			},
			logsLines: []string{
				"level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
				"level=\"info\" schema=\"some_other_schema\" thread_id=\"891\" event_id=\"124\" end_event_id=\"235\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"",
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

			mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
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
			}, 10*time.Second, 200*time.Millisecond)

			collector.Stop()
			lokiClient.Stop()

			require.Eventually(t, func() bool {
				return collector.Stopped()
			}, 10*time.Second, 200*time.Millisecond)

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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
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
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"124\" wait_event_name=\"wait/io/file/innodb/innodb_data_file\" wait_object_name=\"wait_object_name\" wait_object_type=\"wait_object_type\" wait_time=\"0.100000ms\"", lokiEntries[1].Line)
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
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
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"125\" wait_event_name=\"wait/lock/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"0.000150ms\"", lokiEntries[1].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[2].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"126\" wait_end_event_id=\"126\" wait_event_name=\"wait/lock/table/sql/handler\" wait_object_name=\"categories\" wait_object_type=\"TABLE\" wait_time=\"0.000350ms\"", lokiEntries[2].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[3].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"127\" wait_end_event_id=\"127\" wait_event_name=\"wait/io/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"0.000500ms\"", lokiEntries[3].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[4].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"128\" wait_end_event_id=\"128\" wait_event_name=\"wait/io/table/sql/handler\" wait_object_name=\"categories\" wait_object_type=\"TABLE\" wait_time=\"0.000700ms\"", lokiEntries[4].Line)
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
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
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"125\" wait_event_name=\"wait/lock/table/sql/handler\" wait_object_name=\"books\" wait_object_type=\"TABLE\" wait_time=\"0.000150ms\"", lokiEntries[1].Line)
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[2].Labels)
		assert.Equal(t, "level=\"info\" schema=\"books_store\" thread_id=\"890\" event_id=\"126\" end_event_id=\"234\" digest=\"another_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[2].Line)
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField+sqlTextField, sqlTextNotNullClause, endOfTimeline)).WithArgs(
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
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"statements.SQL_TEXT",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
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
					"10000000",
					"456",
					"457",
					"select * from some_table where id = 1",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\" sql_text=\"select * from some_table where id = 1\"", lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
		assert.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" digest=\"some_digest\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"125\" wait_event_name=\"wait/io/file/innodb/innodb_data_file\" wait_object_name=\"wait_object_name\" wait_object_type=\"wait_object_type\" wait_time=\"0.100000ms\" sql_text=\"select * from some_table where id = 1\"", lokiEntries[1].Line)
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField+sqlTextField, sqlTextNotNullClause, endOfTimeline)).WithArgs(1e12, 1e12).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
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
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"statements.SQL_TEXT",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
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
					"10000000",
					"456",
					"457",
					"select * from some_table where id = 1",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\" sql_text=\"select * from some_table where id = 1\"", lokiEntries[0].Line)
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(1e12, 1e12).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
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
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
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
					"10000000",
					"456",
					"457",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
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
			},
			expectedLogOutput: `level="info" schema="test_schema" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="0b" max_total_memory="0b" cpu_time="0.000000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			scanValues: []driver.Value{
				"test_schema",
				"890",
				"123",
				"234",
				"some_digest",
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
				"statements.CPU_TIME",
			},
			expectedLogOutput: `level="info" schema="test_schema" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="0b" max_total_memory="0b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			scanValues: []driver.Value{
				"test_schema",
				"890",
				"123",
				"234",
				"some_digest",
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
				"statements.CPU_TIME",
				"statements.MAX_CONTROLLED_MEMORY",
				"statements.MAX_TOTAL_MEMORY",
			},
			expectedLogOutput: `level="info" schema="test_schema" thread_id="890" event_id="123" end_event_id="234" digest="some_digest" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="1024b" max_total_memory="2048b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			scanValues: []driver.Value{
				"test_schema",
				"890",
				"123",
				"234",
				"some_digest",
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

			mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, tc.expectedFields, digestTextNotNullClause, endOfTimeline)).WithArgs(1e12, 1e12).RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows(tc.expectedColumns).AddRow(tc.scanValues...),
				)

			err = collector.Start(t.Context())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return len(lokiClient.Received()) == 1
			}, 10*time.Second, 200*time.Millisecond)

			collector.Stop()
			lokiClient.Stop()

			require.Eventually(t, func() bool {
				return collector.Stopped()
			}, 10*time.Second, 200*time.Millisecond)

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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
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
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
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
					"10000000",
					"456",
					"457",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(1e12, 2e12).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.THREAD_ID",
					"statements.EVENT_ID",
					"statements.END_EVENT_ID",
					"statements.DIGEST",
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
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
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
					"10000000",
					"456",
					"457",
				).AddRow(
					"some_schema",
					"891",
					"124",
					"235",
					"some_digest",
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
					"10000000",
					"456",
					"457",
				).RowError(1, fmt.Errorf("rs error")), // error on second row
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
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
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
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
					"10000000",
					"456",
					"457",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, lokiEntries[0].Labels)
		require.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
			1e12, // initial timerBookmark
			5e12, // uptime of 5 seconds in picoseconds (modulo 0 overflows)
		).WillReturnRows(sqlmock.NewRows([]string{
			"statements.CURRENT_SCHEMA",
			"statements.THREAD_ID",
			"statements.EVENT_ID",
			"statements.END_EVENT_ID",
			"statements.DIGEST",
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
				555555,        // cpu_time
				1048576,       // max_controlled_memory (1MB)
				2097152,       // max_total_memory (2MB)
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
		}, 10*time.Second, 200*time.Millisecond)
		require.Len(t, lokiClient.Received(), 1)
		assert.Equal(t, model.LabelSet{
			"op": OP_QUERY_SAMPLE,
		}, lokiClient.Received()[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"test_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some digest\" rows_examined=\"1000\" rows_sent=\"100\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"1048576b\" max_total_memory=\"2097152b\" cpu_time=\"0.000556ms\" elapsed_time=\"2000.000000ms\" elapsed_time_ms=\"2000.000000ms\"", lokiClient.Received()[0].Line)
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
			1e12, // initial timerBookmark
			5e12, // uptime of 5 seconds in picoseconds (modulo 0 overflows)
		).WillReturnRows(sqlmock.NewRows([]string{
			"current_schema",
			"thread_id",
			"event_id",
			"end_event_id",
			"digest",
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, beginningAndEndOfTimeline)).WithArgs( // asserts that beginningAndEndOfTimeline clause is used
			3e12,
			10e12, // uptimeLimit is calculated as uptime "modulo" overflows: (uptime - 1 overflow) in picoseconds
		).WillReturnRows(sqlmock.NewRows([]string{
			"current_schema",
			"thread_id",
			"event_id",
			"end_event_id",
			"digest",
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, beginningAndEndOfTimeline)).WithArgs(
			3e12,
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"thread_id",
			"event_id",
			"end_event_id",
			"current_schema",
			"digest",
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs( // asserts revert to endOfTimeline clause
			10e12, // asserts timerBookmark has been updated to the previous uptimeLimit
			13e12, // asserts uptimeLimit is now updated to the current uptime "modulo" overflows
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"event_id",
			"thread_id",
			"current_schema",
			"digest",
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
			float64(0),
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"event_id",
			"thread_id",
			"current_schema",
			"digest",
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
			float64(0),
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"event_id",
			"thread_id",
			"current_schema",
			"digest",
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(3e12, 10e12).WillReturnError(fmt.Errorf("some error"))

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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
			2e12,
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"current_schema",
			"digest",
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
			"cpu_time",
			"max_controlled_memory",
			"max_total_memory",
		}).
			AddRow(
				"test_schema", // current_schema
				"some digest", // digest
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
				555555,        // cpu_time
				1048576,       // max_controlled_memory (1MB)
				2097152,       // max_total_memory (2MB)
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, cpuTimeField+maxControlledMemoryField+maxTotalMemoryField, digestTextNotNullClause, endOfTimeline)).WithArgs(
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
					"statements.CPU_TIME",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"890",
					"123",
					"234",
					"some_digest",
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
					"10000000",
					"456",
					"457",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 10*time.Second, 200*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 10*time.Second, 200*time.Millisecond)

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
		}, 10*time.Second, 200*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)
	})
}
