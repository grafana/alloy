package collector

import (
	"database/sql/driver"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector/parser"
)

func TestQuerySample(t *testing.T) {
	defer goleak.VerifyNone(t)

	testcases := []struct {
		name       string
		rows       [][]driver.Value
		logsLabels []model.LabelSet
		logsLines  []string
	}{
		{
			name: "select query without wait event",
			rows: [][]driver.Value{{
				"some_schema",
				"123",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "truncated query",
			rows: [][]driver.Value{{
				"some_schema",
				"123",
				"some_digest",
				"insert into some_table (`id1`, `id2`, `id3`, `id...",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}, {
				"some_schema",
				"124",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level="info" schema="some_schema" event_id="124" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "truncated in multi-line comment",
			rows: [][]driver.Value{{
				"some_schema",
				"123",
				"some_digest",
				"select * from some_table where id = 1 /*traceparent='00-abc...",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "truncated with properly closed comment",
			rows: [][]driver.Value{{
				"some_schema",
				"123",
				"some_digest",
				"select * from some_table where id = 1 /* comment that's closed */ and name = 'test...",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}},
			logsLabels: []model.LabelSet{},
			logsLines:  []string{},
		},
		{
			name: "start transaction",
			rows: [][]driver.Value{{
				"some_schema",
				"123",
				"some_digest",
				"START TRANSACTION",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="begin" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "sql parse error",
			rows: [][]driver.Value{{
				"some_schema",
				"123",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}, {
				"some_schema",
				"124",
				"some_digest",
				"not valid sql",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "multiple schemas",
			rows: [][]driver.Value{{
				"some_schema",
				"123",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}, {
				"some_other_schema",
				"124",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
				`level="info" schema="some_other_schema" event_id="124" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "subquery and union",
			rows: [][]driver.Value{{
				"some_schema",
				"123",
				"some_digest",
				"SELECT * FROM (SELECT id, name FROM employees_us_east UNION SELECT id, name FROM employees_us_west) as employees_us UNION SELECT id, name FROM employees_emea",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from (select id, name from employees_us_east union select id, name from employees_us_west) as employees_us union select id, name from employees_emea" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "show create table (table name is not parsed)",
			rows: [][]driver.Value{{
				"some_schema",
				"123",
				"some_digest",
				"SHOW CREATE TABLE some_table",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
				nil,
				nil,
				nil,
				nil,
				nil,
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="show create table" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			lokiClient := loki_fake.NewClient(func() {})

			collector, err := NewQuerySample(QuerySampleArguments{
				DB:              db,
				InstanceKey:     "mysql-db",
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

			mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
				1e12, // initial timerBookmark
				1e12,
			).RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"statements.CURRENT_SCHEMA",
						"statements.EVENT_ID",
						"statements.DIGEST",
						"statements.DIGEST_TEXT",
						"statements.TIMER_END",
						"statements.TIMER_WAIT",
						"statements.CPU_TIME",
						"statements.ROWS_EXAMINED",
						"statements.ROWS_SENT",
						"statements.ROWS_AFFECTED",
						"statements.ERRORS",
						"statements.MAX_CONTROLLED_MEMORY",
						"statements.MAX_TOTAL_MEMORY",
						"waits.event_id",
						"waits.event_name",
						"waits.object_name",
						"waits.object_type",
						"waits.timer_wait",
					}).AddRows(
						tc.rows...,
					),
				)

			err = collector.Start(t.Context())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return len(lokiClient.Received()) == len(tc.logsLines)
			}, 5*time.Second, 100*time.Millisecond)

			collector.Stop()
			lokiClient.Stop()

			require.Eventually(t, func() bool {
				return collector.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

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

func TestQuerySample_WaitEvents(t *testing.T) {
	t.Run("both query sample and associated wait event is collected", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
			1e12,
			1e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.EVENT_ID",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"waits.event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
				}).AddRows(
					[]driver.Value{
						"some_schema",
						"123",
						"some_digest",
						"select * from some_table where id = 1",
						"70000000",
						"20000000",
						"10000000",
						"5",
						"5",
						"0",
						"0",
						"456",
						"457",
						"124",
						"wait/io/file/innodb/innodb_data_file",
						"wait_object_name",
						"wait_object_type",
						"100000000",
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
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "mysql-db"}, lokiEntries[1].Labels)
		assert.Equal(t, `level="info" schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = 1" event_id="123" wait_event_id="124" wait_event_name="wait/io/file/innodb/innodb_data_file" wait_object_name="wait_object_name" wait_object_type="wait_object_type" wait_time="0.100000ms"`, lokiEntries[1].Line)
	})

	t.Run("query sample and multiple wait events are collected", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
			1e12,
			1e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.EVENT_ID",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"waits.event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
				}).AddRows(
					[]driver.Value{
						"books_store",
						"123",
						"some_digest",
						"select * from some_table where id = 1",
						"70000000",
						"20000000",
						"10000000",
						"5",
						"5",
						"0",
						"0",
						"456",
						"457",
						"124",
						"wait/lock/table/sql/handler",
						"books",
						"TABLE",
						"150000",
					},
					[]driver.Value{
						"books_store",
						"123",
						"some_digest",
						"select * from some_table where id = 1",
						"70000000",
						"20000000",
						"10000000",
						"5",
						"5",
						"0",
						"0",
						"456",
						"457",
						"125",
						"wait/lock/table/sql/handler",
						"categories",
						"TABLE",
						"350000",
					},
					[]driver.Value{
						"books_store",
						"123",
						"some_digest",
						"select * from some_table where id = 1",
						"70000000",
						"20000000",
						"10000000",
						"5",
						"5",
						"0",
						"0",
						"456",
						"457",
						"126",
						"wait/io/table/sql/handler",
						"books",
						"TABLE",
						"500000",
					},
					[]driver.Value{
						"books_store",
						"123",
						"some_digest",
						"select * from some_table where id = 1",
						"70000000",
						"20000000",
						"10000000",
						"5",
						"5",
						"0",
						"0",
						"456",
						"457",
						"127",
						"wait/io/table/sql/handler",
						"categories",
						"TABLE",
						"700000",
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
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" schema="books_store" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "mysql-db"}, lokiEntries[1].Labels)
		assert.Equal(t, `level="info" schema="books_store" digest="some_digest" digest_text="select * from some_table where id = 1" event_id="123" wait_event_id="124" wait_event_name="wait/lock/table/sql/handler" wait_object_name="books" wait_object_type="TABLE" wait_time="0.000150ms"`, lokiEntries[1].Line)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "mysql-db"}, lokiEntries[2].Labels)
		assert.Equal(t, `level="info" schema="books_store" digest="some_digest" digest_text="select * from some_table where id = 1" event_id="123" wait_event_id="125" wait_event_name="wait/lock/table/sql/handler" wait_object_name="categories" wait_object_type="TABLE" wait_time="0.000350ms"`, lokiEntries[2].Line)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "mysql-db"}, lokiEntries[3].Labels)
		assert.Equal(t, `level="info" schema="books_store" digest="some_digest" digest_text="select * from some_table where id = 1" event_id="123" wait_event_id="126" wait_event_name="wait/io/table/sql/handler" wait_object_name="books" wait_object_type="TABLE" wait_time="0.000500ms"`, lokiEntries[3].Line)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "mysql-db"}, lokiEntries[4].Labels)
		assert.Equal(t, `level="info" schema="books_store" digest="some_digest" digest_text="select * from some_table where id = 1" event_id="123" wait_event_id="127" wait_event_name="wait/io/table/sql/handler" wait_object_name="categories" wait_object_type="TABLE" wait_time="0.000700ms"`, lokiEntries[4].Line)
	})

	t.Run("query sample and its wait event and another query sample are collected", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"uptime"}).AddRow("1"))
		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{"now", "uptime"}).AddRow(5, 1))
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
			1e12,
			1e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.EVENT_ID",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"waits.event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
				}).AddRows(
					[]driver.Value{
						"books_store",
						"123",
						"some_digest",
						"select * from some_table where id = 1",
						"70000000",
						"20000000",
						"10000000",
						"5",
						"5",
						"0",
						"0",
						"456",
						"457",
						"124",
						"wait/lock/table/sql/handler",
						"books",
						"TABLE",
						"150000",
					},
					[]driver.Value{
						"books_store",
						"126",
						"another_digest",
						"select * from another_table where id = 1",
						"70000000",
						"20000000",
						"10000000",
						"5",
						"5",
						"0",
						"0",
						"456",
						"457",
						nil,
						nil,
						nil,
						nil,
						nil,
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
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" schema="books_store" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "mysql-db"}, lokiEntries[1].Labels)
		assert.Equal(t, `level="info" schema="books_store" digest="some_digest" digest_text="select * from some_table where id = 1" event_id="123" wait_event_id="124" wait_event_name="wait/lock/table/sql/handler" wait_object_name="books" wait_object_type="TABLE" wait_time="0.000150ms"`, lokiEntries[1].Line)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[2].Labels)
		assert.Equal(t, `level="info" schema="books_store" event_id="126" digest="another_digest" digest_text="select * from another_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[2].Line)
	})

	t.Run("wait event with disabled sql redaction", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:                    db,
			InstanceKey:           "mysql-db",
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, sqlTextField, sqlTextNotNullClause, endOfTimeline)).WithArgs(
			1e12, // initial timerBookmark
			1e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.EVENT_ID",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"waits.event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"statements.SQL_TEXT",
				}).AddRows(
					[]driver.Value{
						"some_schema",
						"123",
						"some_digest",
						"select * from some_table where id = ?",
						"70000000",
						"20000000",
						"10000000",
						"5",
						"5",
						"0",
						"0",
						"456",
						"457",
						"124",
						"wait/io/file/innodb/innodb_data_file",
						"wait_object_name",
						"wait_object_type",
						"100000000",
						"select * from some_table where id = 1",
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
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :v1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms" sql_text="select * from some_table where id = 1"`, lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "mysql-db"}, lokiEntries[1].Labels)
		assert.Equal(t, `level="info" schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = ?" event_id="123" wait_event_id="124" wait_event_name="wait/io/file/innodb/innodb_data_file" wait_object_name="wait_object_name" wait_object_type="wait_object_type" wait_time="0.100000ms" sql_text="select * from some_table where id = 1"`, lokiEntries[1].Line)
	})
}

func TestQuerySampleDisableQueryRedaction(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Run("collects sql text when enabled", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:                    db,
			InstanceKey:           "mysql-db",
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, sqlTextField, sqlTextNotNullClause, endOfTimeline)).WithArgs(1e12, 1e12).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.EVENT_ID",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"waits.event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
					"statements.SQL_TEXT",
				}).AddRow(
					"some_schema",
					"123",
					"some_digest",
					"select * from some_table where id = ?",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
					nil,
					nil,
					nil,
					nil,
					nil,
					"select * from some_table where id = 1",
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :v1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms" sql_text="select * from some_table where id = 1"`, lokiEntries[0].Line)
	})

	t.Run("does not collect sql text when disabled", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:                    db,
			InstanceKey:           "mysql-db",
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(1e12, 1e12).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.EVENT_ID",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"waits.event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
				}).AddRow(
					"some_schema",
					"123",
					"some_digest",
					"select * from some_table where id = ?",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
					nil,
					nil,
					nil,
					nil,
					nil,
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :v1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
	})
}

func TestQuerySampleSQLDriverErrors(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("recoverable sql error in result set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
			1e12,
			1e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.EVENT_ID",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"waits.event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
				}).AddRow(
					"some_schema",
					"123",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
					nil,
					nil,
					nil,
					nil,
					nil,
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
	})

	t.Run("result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(1e12, 2e12).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.EVENT_ID",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"waits.event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
				}).AddRow(
					"some_schema",
					"123",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
					nil,
					nil,
					nil,
					nil,
					nil,
				).AddRow(
					"some_schema",
					"124",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
					nil,
					nil,
					nil,
					nil,
					nil,
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
	})

	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
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

		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.EVENT_ID",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
					"waits.event_id",
					"waits.event_name",
					"waits.object_name",
					"waits.object_type",
					"waits.timer_wait",
				}).AddRow(
					"some_schema",
					"123",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
					nil,
					nil,
					nil,
					nil,
					nil,
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
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" schema="some_schema" event_id="123" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
	})
}

func TestQuerySample_initializeTimer(t *testing.T) {
	t.Run("selects uptime, sets timerBookmark", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{
			"uptime",
		}).AddRow(
			5,
		))

		c, err := NewQuerySample(QuerySampleArguments{DB: db})
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

		c, err := NewQuerySample(QuerySampleArguments{DB: db})
		require.NoError(t, err)

		require.NoError(t, c.initializeBookmark(t.Context()))

		assert.Equal(t, 5e12, c.timerBookmark)
	})
}

func TestQuerySample_handles_timer_overflows(t *testing.T) {
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
			1e12, // initial timerBookmark
			5e12, // uptime of 5 seconds in picoseconds (modulo 0 overflows)
		).WillReturnRows(sqlmock.NewRows([]string{
			"statements.CURRENT_SCHEMA",
			"statements.EVENT_ID",
			"statements.DIGEST",
			"statements.DIGEST_TEXT",
			"statements.TIMER_END",
			"statements.TIMER_WAIT",
			"statements.CPU_TIME",
			"statements.ROWS_EXAMINED",
			"statements.ROWS_SENT",
			"statements.ROWS_AFFECTED",
			"statements.ERRORS",
			"statements.MAX_CONTROLLED_MEMORY",
			"statements.MAX_TOTAL_MEMORY",
			"waits.event_id",
			"waits.event_name",
			"waits.object_name",
			"waits.object_type",
			"waits.timer_wait",
		}).
			AddRow(
				"test_schema",         // current_schema
				123,                   // EVENT_ID
				"some digest",         // digest
				"SELECT * FROM users", // digest_text
				2e12,                  // timer_end
				2e12,                  // timer_wait
				555555,                // cpu_time
				1000,                  // rows_examined
				100,                   // rows_sent
				0,                     // rows_affected
				0,                     // errors
				1048576,               // max_controlled_memory (1MB)
				2097152,               // max_total_memory (2MB)
				nil,                   // WAIT_EVENT_ID
				nil,                   // WAIT_EVENT_NAME
				nil,                   // WAIT_OBJECT_NAME
				nil,                   // WAIT_OBJECT_TYPE
				nil,                   // WAIT_TIME
			),
		)

		lokiClient := loki_fake.NewClient(func() {})
		c := &QuerySample{
			sqlParser:     &parser.TiDBSqlParser{},
			instanceKey:   "some instance key",
			dbConnection:  db,
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
			"job":      database_observability.JobName,
			"op":       OP_QUERY_SAMPLE,
			"instance": "some instance key",
		}, lokiClient.Received()[0].Labels)
		assert.Equal(t, "level=\"info\" schema=\"test_schema\" event_id=\"123\" digest=\"some digest\" digest_text=\"select * from `users`\" "+
			"rows_examined=\"1000\" rows_sent=\"100\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"1048576b\" "+
			"max_total_memory=\"2097152b\" cpu_time=\"0.000556ms\" elapsed_time=\"2000.000000ms\" elapsed_time_ms=\"2000.000000ms\"",
			lokiClient.Received()[0].Line)
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
			SELECT
				statements.CURRENT_SCHEMA,
				statements.EVENT_ID,
				statements.DIGEST,
				statements.DIGEST_TEXT,
				statements.TIMER_END,
				statements.TIMER_WAIT,
				statements.CPU_TIME,
				statements.ROWS_EXAMINED,
				statements.ROWS_SENT,
				statements.ROWS_AFFECTED,
				statements.ERRORS,
				statements.MAX_CONTROLLED_MEMORY,
				statements.MAX_TOTAL_MEMORY,
				waits.event_id as WAIT_EVENT_ID,
				waits.event_name as WAIT_EVENT_NAME,
				waits.object_name as WAIT_OBJECT_NAME,
				waits.object_type as WAIT_OBJECT_TYPE,
				waits.timer_wait as WAIT_TIMER_WAIT
			FROM
				performance_schema.events_statements_history AS statements
			LEFT JOIN
				performance_schema.events_waits_history waits
				ON statements.thread_id = waits.thread_id
				AND statements.EVENT_ID = waits.NESTING_EVENT_ID
			WHERE statements.DIGEST IS NOT NULL
			  AND statements.CURRENT_SCHEMA NOT IN ('mysql', 'performance_schema', 'sys', 'information_schema')
			  AND statements.DIGEST_TEXT IS NOT NULL
			  AND statements.TIMER_END > ? AND statements.TIMER_END <= ?`).WithArgs(
			1e12, // initial timerBookmark
			5e12, // uptime of 5 seconds in picoseconds (modulo 0 overflows)
		).WillReturnRows(sqlmock.NewRows([]string{
			"current_schema",
			"digest",
			"digest_text",
			"timer_end",
			"timer_wait",
			"cpu_time",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"errors",
			"max_controlled_memory",
			"max_total_memory",
		}))

		c := &QuerySample{
			sqlParser:     &parser.TiDBSqlParser{},
			instanceKey:   "some instance key",
			dbConnection:  db,
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, beginningAndEndOfTimeline)).WithArgs( // asserts that beginningAndEndOfTimeline clause is used
			3e12,
			10e12, // uptimeLimit is calculated as uptime "modulo" overflows: (uptime - 1 overflow) in picoseconds
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"event_id",
			"thread_id",
			"current_schema",
			"digest_text",
			"digest",
			"timer_wait",
			"cpu_time",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"max_controlled_memory",
			"max_total_memory",
			"errors",
		}))
		c := &QuerySample{
			sqlParser:     &parser.TiDBSqlParser{},
			dbConnection:  db,
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, beginningAndEndOfTimeline)).WithArgs(
			3e12,
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"event_id",
			"thread_id",
			"current_schema",
			"digest_text",
			"digest",
			"timer_wait",
			"cpu_time",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"max_controlled_memory",
			"max_total_memory",
			"errors",
		}))
		c := &QuerySample{
			sqlParser:     &parser.TiDBSqlParser{},
			dbConnection:  db,
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs( // asserts revert to endOfTimeline clause
			10e12, // asserts timerBookmark has been updated to the previous uptimeLimit
			13e12, // asserts uptimeLimit is now updated to the current uptime "modulo" overflows
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"event_id",
			"thread_id",
			"current_schema",
			"digest_text",
			"digest",
			"timer_wait",
			"cpu_time",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"max_controlled_memory",
			"max_total_memory",
			"errors",
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
			float64(0),
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"event_id",
			"thread_id",
			"current_schema",
			"digest_text",
			"digest",
			"timer_wait",
			"cpu_time",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"max_controlled_memory",
			"max_total_memory",
			"errors",
		}))
		c := &QuerySample{
			dbConnection:  db,
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
			float64(0),
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"timer_end",
			"event_id",
			"thread_id",
			"current_schema",
			"digest_text",
			"digest",
			"timer_wait",
			"cpu_time",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"max_controlled_memory",
			"max_total_memory",
			"errors",
		}))
		c := &QuerySample{
			dbConnection:  db,
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

		c, err := NewQuerySample(QuerySampleArguments{DB: db})
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

		c := &QuerySample{
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
		mock.ExpectQuery(fmt.Sprintf(selectQuerySamples, "", digestTextNotNullClause, endOfTimeline)).WithArgs(
			2e12,
			10e12,
		).WillReturnRows(sqlmock.NewRows([]string{
			"current_schema",
			"digest",
			"digest_text",
			"timer_end",
			"timer_wait",
			"cpu_time",
			"rows_examined",
			"rows_sent",
			"rows_affected",
			"errors",
			"max_controlled_memory",
			"max_total_memory",
		}).
			AddRow(
				"test_schema",         // current_schema
				"some digest",         // digest
				"SELECT * FROM users", // digest_text
				2e12,                  // timer_end
				2e12,                  // timer_wait
				555555,                // cpu_time
				1000,                  // rows_examined
				100,                   // rows_sent
				0,                     // rows_affected
				0,                     // errors
				1048576,               // max_controlled_memory (1MB)
				2097152,               // max_total_memory (2MB)
			),
		)
		mockParser := &parser.MockParser{}
		c := &QuerySample{
			dbConnection:  db,
			sqlParser:     mockParser,
			timerBookmark: 2e12,
			logger:        log.NewLogfmtLogger(os.Stderr),
		}

		mockParser.On("CleanTruncatedText", "SELECT * FROM users").Return("SELECT * FROM users", nil)
		mockParser.On("Redact", "SELECT * FROM users").Return("", fmt.Errorf("some error"))

		err = c.fetchQuerySamples(t.Context())

		assert.NoError(t, err)
	})
}

func TestQuerySample_calculateWallTime(t *testing.T) {
	t.Run("calculates the timestamp at which an event happened", func(t *testing.T) {
		c := &QuerySample{}
		serverStartTime := float64(2)
		timer := 2e12 // Timer indicates event timing, counted since server startup. 2 seconds in picoseconds

		result := c.calculateWallTime(serverStartTime, timer)
		assert.Equalf(t, float64(4000), result, "got %f, want 4000", result)
	})

	t.Run("calculates the timestamp, taking into account the overflows", func(t *testing.T) {
		c := &QuerySample{lastUptime: picosecondsToSeconds(math.MaxUint64) + 1}
		serverStartTime := float64(3)
		timer := 2e12 // 2 seconds in picoseconds

		result := c.calculateWallTime(serverStartTime, timer)

		assert.Equalf(t, 18446749073.709553, result, "got %f, want 18446749073.709553", result)
	})

	t.Run("calculates another timestamp when timer approaches overflow", func(t *testing.T) {
		c := &QuerySample{lastUptime: picosecondsToSeconds(math.MaxUint64) + 1}
		serverStartTime := float64(3)
		timer := float64(math.MaxUint64 - 5)

		result := c.calculateWallTime(serverStartTime, timer)

		assert.Equalf(t, 3.6893491147419106e+10, result, "got %f, want 3.6893491147419106e+10", result)
	})
}

func TestQuerySample_calculateNumberOfOverflows(t *testing.T) {
	testCases := map[string]struct {
		expected uint64
		uptime   float64
	}{
		"0 overflows": {0, 5},
		"1 overflow":  {1, picosecondsToSeconds(math.MaxUint64) + 5},
		"2 overflows": {2, picosecondsToSeconds(math.MaxUint64)*2 + 5},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.EqualValues(t, tc.expected, calculateNumberOfOverflows(tc.uptime))
		})
	}
}

func TestQuerySample_calculateTimerClauseAndLimit(t *testing.T) {
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
			c := &QuerySample{
				lastUptime: tc.lastUptime,
			}

			actualTimerClause, actualLimit := c.determineTimerClauseAndLimit(tc.uptime)

			assert.Equal(t, tc.expectedTimerClause, actualTimerClause)
			assert.Equal(t, tc.expectedLimit, actualLimit)
		})
	}
}
