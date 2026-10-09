package collector

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/util"
)

// newTestLogsCollector builds a Logs collector (no DB) with a collecting
// Loki handler and a fresh prometheus registry, ready to receive entries
// via the returned receiver.
func newTestLogsCollector(t *testing.T, _ *sqlmock.Sqlmock) (*Logs, loki.LogsReceiver, *loki.CollectingHandler) {
	t.Helper()

	receiver := loki.NewLogsReceiver()
	lokiClient := loki.NewCollectingHandler()
	reg := prometheus.NewRegistry()

	l, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: lokiClient,
		Logger:       util.TestAlloyLogger(t).Slog(),
		Registry:     reg,
	})
	require.NoError(t, err)
	require.NoError(t, l.Start(t.Context()))

	t.Cleanup(func() {
		l.Stop()
		lokiClient.Stop()
	})

	return l, receiver, lokiClient
}

func send(t *testing.T, receiver loki.LogsReceiver, labels model.LabelSet, line string) {
	t.Helper()
	select {
	case receiver.Chan() <- loki.NewEntry(labels, push.Entry{Timestamp: time.Now(), Line: line}):
	case <-time.After(5 * time.Second):
		t.Fatal("timed out sending log entry")
	}
}

func waitForEntries(t *testing.T, lokiClient *loki.CollectingHandler) {
	t.Helper()
	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) >= 1
	}, 5*time.Second, 10*time.Millisecond)
}

func TestLogsCollector_StartStop(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	l, _, _ := newTestLogsCollector(t, nil)
	require.False(t, l.Stopped())
	l.Stop()
	require.True(t, l.Stopped())
}

func TestLogsCollector_ErrorLog_ParseStructuredFormat_UnmodeledCode(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	l, receiver, lokiClient := newTestLogsCollector(t, nil)

	// MY-012487 is a real InnoDB code but not one of our modeled categories.
	send(t, receiver, model.LabelSet{"filename": "/var/log/mysql/error.log"},
		`2020-08-06T14:25:02.835618Z 0 [Note] [MY-012487] [InnoDB] DDL log recovery : begin`)

	// Give the collector a moment to process; nothing should be emitted.
	time.Sleep(50 * time.Millisecond)
	require.Empty(t, lokiClient.Received())

	send(t, receiver, model.LabelSet{"filename": "/var/log/mysql/error.log"},
		`2024-01-15T10:23:45.123456Z 7 [Warning] [MY-099999] [Server] some unmodeled warning`)
	time.Sleep(50 * time.Millisecond)
	require.Empty(t, lokiClient.Received(), "unmodeled codes must never emit an entry")

	require.Equal(t, float64(1), testutil.ToFloat64(l.errorsByCode.WithLabelValues("Warning", "MY-099999", "Server", "false")))
}

func TestLogsCollector_ErrorLog_InvalidFormat(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	l, receiver, lokiClient := newTestLogsCollector(t, nil)

	send(t, receiver, model.LabelSet{"filename": "/var/log/mysql/error.log"}, `this is not a recognizable log line at all`)
	time.Sleep(50 * time.Millisecond)

	require.Empty(t, lokiClient.Received())
	require.Equal(t, float64(1), testutil.ToFloat64(l.parseFailures.WithLabelValues("unknown")))
}

func TestLogsCollector_ErrorLog_ModeledCategories(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	testCases := []struct {
		name         string
		line         string
		wantCategory string
		wantOp       string
		wantFields   map[string]string
	}{
		{
			name:         "aborted_connection",
			line:         `2024-01-15T10:23:45.123456Z 42 [Warning] [MY-010914] [Server] Aborted connection 42 to db: 'employees' user: 'josh' host: 'localhost' (Got an error reading communication packets).`,
			wantCategory: "aborted_connection",
			wantOp:       database_observability.OP_ERROR_MESSAGE,
			wantFields:   map[string]string{"db": "employees", "user": "josh", "reason": "Got an error reading communication packets"},
		},
		{
			name:         "access_denied",
			line:         `2024-01-15T10:23:45.123456Z 9 [Warning] [MY-010926] [Server] Access denied for user 'root'@'localhost' (using password: YES)`,
			wantCategory: "access_denied",
			wantOp:       database_observability.OP_ERROR_MESSAGE,
			wantFields:   map[string]string{"user": "root", "using_password": "YES"},
		},
		{
			name:         "replication_io_error",
			line:         `2024-01-15T10:23:45.123456Z 5 [Error] [MY-013114] [Repl] Got fatal error 1236 from source when reading data from binary log: 'could not find first log file name in binary log index file'`,
			wantCategory: "replication_io_error",
			wantOp:       database_observability.OP_ERROR_MESSAGE,
			wantFields:   map[string]string{"replication_errno": "1236"},
		},
		{
			name:         "replication_sql_error",
			line:         `2024-01-15T10:23:45.123456Z 6 [Error] [MY-010586] [Repl] Error running query, replica SQL thread aborted. Fix the problem, and restart the replica SQL thread with "START REPLICA". We stopped at log 'binlog.000123' position 456`,
			wantCategory: "replication_sql_error",
			wantOp:       database_observability.OP_ERROR_MESSAGE,
			wantFields:   map[string]string{"binlog_file": "binlog.000123", "binlog_position": "456"},
		},
		{
			name:         "server_startup_starting",
			line:         `2024-01-15T10:23:45.123456Z 0 [System] [MY-010116] [Server] /usr/sbin/mysqld (mysqld 8.0.36) starting as process 1234`,
			wantCategory: "server_startup",
			wantOp:       database_observability.OP_SERVER_LOG,
			wantFields:   map[string]string{"pid": "1234"},
		},
		{
			name:         "server_startup_ready",
			line:         `2024-01-15T10:23:45.123456Z 0 [System] [MY-010931] [Server] /usr/sbin/mysqld: ready for connections. Version: '8.0.36'  socket: '/var/run/mysqld.sock'  port: 3306  MySQL Community Server - GPL.`,
			wantCategory: "server_startup",
			wantOp:       database_observability.OP_SERVER_LOG,
			wantFields:   map[string]string{"version": "8.0.36", "port": "3306"},
		},
		{
			name:         "server_shutdown",
			line:         `2024-01-15T10:23:45.123456Z 0 [System] [MY-013105] [Server] /usr/sbin/mysqld: Normal shutdown.`,
			wantCategory: "server_shutdown",
			wantOp:       database_observability.OP_SERVER_LOG,
			wantFields:   map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l, receiver, lokiClient := newTestLogsCollector(t, nil)

			send(t, receiver, model.LabelSet{"filename": "/var/log/mysql/error.log"}, tc.line)
			waitForEntries(t, lokiClient)

			entries := lokiClient.Received()
			require.Len(t, entries, 1)
			require.Equal(t, model.LabelSet{"op": model.LabelValue(tc.wantOp)}, entries[0].Labels)

			require.Contains(t, entries[0].Line, `category="`+tc.wantCategory+`"`)
			for k, v := range tc.wantFields {
				if _, numeric := numericLogFields[k]; numeric {
					require.Contains(t, entries[0].Line, k+"="+v, "field %s", k)
				} else {
					require.Contains(t, entries[0].Line, k+`="`+v+`"`, "field %s", k)
				}
			}
			require.NotContains(t, entries[0].Line, "host=", "client host must never be captured")
			require.NotContains(t, entries[0].Line, "localhost", "client host value must never be captured")
			_ = l
		})
	}
}

func TestLogsCollector_ErrorLog_ExcludeUsersAndSchemas(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	receiver := loki.NewLogsReceiver()
	lokiClient := loki.NewCollectingHandler()
	reg := prometheus.NewRegistry()

	l, err := NewLogs(LogsArguments{
		Receiver:       receiver,
		EntryHandler:   lokiClient,
		Logger:         util.TestAlloyLogger(t).Slog(),
		Registry:       reg,
		ExcludeUsers:   []string{"josh"},
		ExcludeSchemas: []string{"employees"},
	})
	require.NoError(t, err)
	require.NoError(t, l.Start(t.Context()))
	t.Cleanup(func() {
		l.Stop()
		lokiClient.Stop()
	})

	send(t, receiver, model.LabelSet{"filename": "/var/log/mysql/error.log"},
		`2024-01-15T10:23:45.123456Z 42 [Warning] [MY-010914] [Server] Aborted connection 42 to db: 'employees' user: 'josh' host: 'localhost' (Got an error reading communication packets).`)
	time.Sleep(50 * time.Millisecond)
	require.Empty(t, lokiClient.Received(), "excluded user/schema must suppress emission")
}

func TestLogsCollector_SlowQueryLog_EmitsDigestNeverRawSQL(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	const sqlText = `SELECT * FROM big_table WHERE secret_column = 'super-secret-value'`
	const wantDigest = "deadbeefdeadbeefdeadbeefdeadbeef"

	mock.ExpectQuery(selectStatementDigest).WithArgs(sqlText).
		WillReturnRows(sqlmock.NewRows([]string{"digest"}).AddRow(wantDigest))

	receiver := loki.NewLogsReceiver()
	lokiClient := loki.NewCollectingHandler()
	reg := prometheus.NewRegistry()

	l, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: lokiClient,
		Logger:       util.TestAlloyLogger(t).Slog(),
		Registry:     reg,
		DB:           db,
	})
	require.NoError(t, err)
	require.NoError(t, l.Start(t.Context()))
	t.Cleanup(func() {
		l.Stop()
		lokiClient.Stop()
	})

	labels := model.LabelSet{"filename": "/var/log/mysql/slow.log"}
	send(t, receiver, labels, `# Time: 2024-01-15T10:23:45.123456Z`)
	send(t, receiver, labels, `# User@Host: root[root] @ localhost []  Id: 55`)
	send(t, receiver, labels, `# Query_time: 1.234567  Lock_time: 0.000123  Rows_sent: 10  Rows_examined: 1000`)
	send(t, receiver, labels, `SET timestamp=1705315425;`)
	send(t, receiver, labels, sqlText+`;`)

	waitForEntries(t, lokiClient)
	entries := lokiClient.Received()
	require.Len(t, entries, 1)
	require.Equal(t, model.LabelSet{"op": database_observability.OP_SLOW_QUERY}, entries[0].Labels)

	require.Contains(t, entries[0].Line, `digest="`+wantDigest+`"`)
	require.Contains(t, entries[0].Line, `thread_id=55`)
	require.Contains(t, entries[0].Line, `user="root"`)
	require.Contains(t, entries[0].Line, `query_time=1.234567`)
	require.Contains(t, entries[0].Line, `rows_examined=1000`)

	require.NotContains(t, entries[0].Line, "secret_column")
	require.NotContains(t, entries[0].Line, "super-secret-value")
	require.NotContains(t, entries[0].Line, "SELECT")
	require.NotContains(t, entries[0].Line, "localhost")

	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, float64(1), testutil.ToFloat64(l.slowQueries))
}

func TestLogsCollector_SlowQueryLog_ExcludeUser(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	// No ExpectQuery: computeDigest must never be called for an excluded user.

	receiver := loki.NewLogsReceiver()
	lokiClient := loki.NewCollectingHandler()
	reg := prometheus.NewRegistry()

	l, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: lokiClient,
		Logger:       util.TestAlloyLogger(t).Slog(),
		Registry:     reg,
		DB:           db,
		ExcludeUsers: []string{"root"},
	})
	require.NoError(t, err)
	require.NoError(t, l.Start(t.Context()))
	t.Cleanup(func() {
		l.Stop()
		lokiClient.Stop()
	})

	labels := model.LabelSet{"filename": "/var/log/mysql/slow.log"}
	send(t, receiver, labels, `# Time: 2024-01-15T10:23:45.123456Z`)
	send(t, receiver, labels, `# User@Host: root[root] @ localhost []  Id: 55`)
	send(t, receiver, labels, `# Query_time: 1.234567  Lock_time: 0.000123  Rows_sent: 10  Rows_examined: 1000`)
	send(t, receiver, labels, `SET timestamp=1705315425;`)
	send(t, receiver, labels, `SELECT 1;`)

	time.Sleep(50 * time.Millisecond)
	require.Empty(t, lokiClient.Received())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLogsCollector_SlowQueryLog_FlushesOnNextTimeLine(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(selectStatementDigest).WithArgs("SELECT 1").
		WillReturnRows(sqlmock.NewRows([]string{"digest"}).AddRow("digest1"))

	receiver := loki.NewLogsReceiver()
	lokiClient := loki.NewCollectingHandler()
	reg := prometheus.NewRegistry()

	l, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: lokiClient,
		Logger:       util.TestAlloyLogger(t).Slog(),
		Registry:     reg,
		DB:           db,
	})
	require.NoError(t, err)
	require.NoError(t, l.Start(t.Context()))
	t.Cleanup(func() {
		l.Stop()
		lokiClient.Stop()
	})

	labels := model.LabelSet{"filename": "/var/log/mysql/slow.log"}
	// First block has no trailing ';' line captured as its own statement
	// (e.g. a multi-line statement lacking a clean terminator) — it must
	// still flush once the next block's Time line arrives.
	send(t, receiver, labels, `# Time: 2024-01-15T10:23:45.123456Z`)
	send(t, receiver, labels, `# User@Host: root[root] @ localhost []  Id: 1`)
	send(t, receiver, labels, `# Query_time: 0.1  Lock_time: 0.0  Rows_sent: 1  Rows_examined: 1`)
	send(t, receiver, labels, `SET timestamp=1705315425;`)
	send(t, receiver, labels, `SELECT 1`) // no trailing ';'

	send(t, receiver, labels, `# Time: 2024-01-15T10:23:46.000000Z`)

	waitForEntries(t, lokiClient)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLogsCollector_GeneralLog_ModeledCommands(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	testCases := []struct {
		name         string
		line         string
		wantCategory string
		wantFields   map[string]string
	}{
		{
			name:         "connect",
			line:         "2024-01-15T10:23:45.000000Z\t5\tConnect\troot@localhost on employees using SSL/TLS",
			wantCategory: "connection",
			wantFields:   map[string]string{"user": "root", "db": "employees"},
		},
		{
			name:         "quit",
			line:         "\t5\tQuit\t",
			wantCategory: "disconnection",
			wantFields:   map[string]string{},
		},
		{
			name:         "change_user",
			line:         "\t5\tChange user\tnewuser@localhost on employees",
			wantCategory: "change_user",
			wantFields:   map[string]string{"user": "newuser"},
		},
		{
			name:         "init_db",
			line:         "\t5\tInit DB\temployees",
			wantCategory: "init_db",
			wantFields:   map[string]string{"db": "employees"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l, receiver, lokiClient := newTestLogsCollector(t, nil)

			send(t, receiver, model.LabelSet{"filename": "/var/log/mysql/general.log"}, tc.line)
			waitForEntries(t, lokiClient)

			entries := lokiClient.Received()
			require.Len(t, entries, 1)
			require.Equal(t, model.LabelSet{"op": database_observability.OP_SERVER_LOG}, entries[0].Labels)
			require.Contains(t, entries[0].Line, `category="`+tc.wantCategory+`"`)
			for k, v := range tc.wantFields {
				require.Contains(t, entries[0].Line, k+`="`+v+`"`)
			}
			require.NotContains(t, entries[0].Line, "localhost")
			_ = l
		})
	}
}

func TestLogsCollector_GeneralLog_QueryCommandSkipped(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	l, receiver, lokiClient := newTestLogsCollector(t, nil)

	send(t, receiver, model.LabelSet{"filename": "/var/log/mysql/general.log"},
		"2024-01-15T10:23:45.000000Z\t5\tQuery\tSELECT * FROM employees")
	time.Sleep(50 * time.Millisecond)

	require.Empty(t, lokiClient.Received(), "Query commands are intentionally out of scope")
	require.Equal(t, float64(0), testutil.ToFloat64(l.parseFailures.WithLabelValues("general")), "a skipped command is not a parse failure")
}

func TestLogsCollector_Dispatch_InterleavedSourcesDoNotCorruptSlowQueryBlock(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(selectStatementDigest).WithArgs("SELECT 1").
		WillReturnRows(sqlmock.NewRows([]string{"digest"}).AddRow("digest1"))

	receiver := loki.NewLogsReceiver()
	lokiClient := loki.NewCollectingHandler()
	reg := prometheus.NewRegistry()

	l, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: lokiClient,
		Logger:       util.TestAlloyLogger(t).Slog(),
		Registry:     reg,
		DB:           db,
	})
	require.NoError(t, err)
	require.NoError(t, l.Start(t.Context()))
	t.Cleanup(func() {
		l.Stop()
		lokiClient.Stop()
	})

	slowLabels := model.LabelSet{"filename": "/var/log/mysql/slow.log"}
	errorLabels := model.LabelSet{"filename": "/var/log/mysql/error.log"}

	send(t, receiver, slowLabels, `# Time: 2024-01-15T10:23:45.123456Z`)
	send(t, receiver, slowLabels, `# User@Host: root[root] @ localhost []  Id: 1`)
	send(t, receiver, slowLabels, `# Query_time: 0.1  Lock_time: 0.0  Rows_sent: 1  Rows_examined: 1`)
	send(t, receiver, slowLabels, `SET timestamp=1705315425;`)
	// An unrelated error-log line arrives from a different source mid-block.
	send(t, receiver, errorLabels, `2024-01-15T10:23:45.123456Z 7 [Warning] [MY-099999] [Server] unrelated warning`)
	send(t, receiver, slowLabels, `SELECT 1;`)

	waitForEntries(t, lokiClient)
	entries := lokiClient.Received()
	require.Len(t, entries, 1)
	require.Equal(t, model.LabelSet{"op": database_observability.OP_SLOW_QUERY}, entries[0].Labels)
	require.Contains(t, entries[0].Line, `digest="digest1"`)
	require.NoError(t, mock.ExpectationsWereMet())
}
