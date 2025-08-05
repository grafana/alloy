package collector

import (
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/lib/pq"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func getWaitEventType(name string) sql.NullString {
	if name == "wait event" {
		return sql.NullString{String: "Lock", Valid: true}
	}
	return sql.NullString{Valid: false}
}

func getWaitEvent(name string) sql.NullString {
	if name == "wait event" {
		return sql.NullString{String: "relation", Valid: true}
	}
	return sql.NullString{Valid: false}
}

func getState(name string) sql.NullString {
	if name == "wait event" {
		return sql.NullString{String: "waiting", Valid: true}
	}
	return sql.NullString{String: "active", Valid: true}
}

func getBlockedByPids(name string) pq.Int64Array {
	if name == "wait event" {
		return pq.Int64Array{103, 104}
	}
	return nil
}

func TestActivity_QueryRedaction(t *testing.T) {
	defer goleak.VerifyNone(t)

	now := time.Now()

	testCases := []struct {
		name                  string
		disableQueryRedaction bool
		query                 string
		queryID               int64
		expectedLabels        []model.LabelSet
		expectedLines         []string
	}{
		{
			name:                  "redaction enabled",
			disableQueryRedaction: false,
			query:                 "SELECT * FROM users WHERE id = 123",
			queryID:               123,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" pid="100" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="0" xmin="0" query_time="%s" queryid="123" query="SELECT * FROM users WHERE id = ?" engine="postgres" cpu_time="%s"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
			},
		},
		{
			name:                  "redaction disabled",
			disableQueryRedaction: true,
			query:                 "SELECT * FROM users WHERE id = 123",
			queryID:               124,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" pid="100" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="0" xmin="0" query_time="%s" queryid="124" query="SELECT * FROM users WHERE id = 123" engine="postgres" cpu_time="%s"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
			},
		},
		{
			name:                  "truncated query with redaction enabled",
			disableQueryRedaction: false,
			query:                 "SELECT * FROM users WHERE id = 123 /* comment ...",
			queryID:               125,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" pid="100" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="0" xmin="0" query_time="%s" queryid="125" query="SELECT * FROM users WHERE id = ? /* comment ..." engine="postgres" cpu_time="%s"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
			},
		},
		{
			name:                  "truncated query with redaction disabled",
			disableQueryRedaction: true,
			query:                 "SELECT * FROM users WHERE id = 123 /* comment ...",
			queryID:               126,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" pid="100" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="0" xmin="0" query_time="%s" queryid="126" query="SELECT * FROM users WHERE id = 123 /* comment ..." engine="postgres" cpu_time="%s"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
			},
		},
		{
			name:                  "complex query with redaction enabled",
			disableQueryRedaction: false,
			query:                 "SELECT u.id, u.name, p.role FROM users u JOIN permissions p ON u.id = p.user_id WHERE u.id IN (123, 456) AND p.role = 'admin'",
			queryID:               127,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" pid="100" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="0" xmin="0" query_time="%s" queryid="127" query="SELECT u.id, u.name, p.role FROM users u JOIN permissions p ON u.id = p.user_id WHERE u.id IN (?, ?) AND p.role = ?" engine="postgres" cpu_time="%s"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
			},
		},
		{
			name:                  "insert query with redaction enabled",
			disableQueryRedaction: false,
			query:                 "INSERT INTO users (id, name, email) VALUES (123, 'John Doe', 'john@example.com')",
			queryID:               128,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" pid="100" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="0" xmin="0" query_time="%s" queryid="128" query="INSERT INTO users (id, name, email) VALUES (?, ?, ?)" engine="postgres" cpu_time="%s"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
			},
		},
		{
			name:                  "wait event",
			disableQueryRedaction: false,
			query:                 "SELECT * FROM users WHERE id = 123",
			queryID:               125,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
				{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" pid="100" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="0" xmin="0" query_time="%s" queryid="125" query="SELECT * FROM users WHERE id = ?" engine="postgres"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" user="testuser" userid="1000" datname="testdb" datid="1" backend_type="client backend" state="waiting" wait_time="%s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" queryid="125" query="SELECT * FROM users WHERE id = ?" blocked_by_pids="[103 104]" engine="postgres"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
				),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			lokiClient := loki_fake.NewClient(func() {})

			activity, err := NewActivity(ActivityArguments{
				DB:                    db,
				InstanceKey:           "test",
				CollectInterval:       time.Second,
				DisableQueryRedaction: tc.disableQueryRedaction,
				EntryHandler:          lokiClient,
				Logger:                log.NewLogfmtLogger(os.Stderr),
			})
			require.NoError(t, err)
			require.NotNil(t, activity)

			// Setup mock expectations
			mock.ExpectQuery(selectPgStatActivity).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{
					"datname", "datid", "pid", "leader_pid", "usesysid",
					"usename", "application_name", "client_addr", "client_port",
					"state_change", "now", "backend_start", "xact_start",
					"query_start", "wait_event_type", "wait_event", "state",
					"backend_type", "backend_xid", "backend_xmin", "query_id",
					"query", "blocked_by_pids",
				}).AddRow(
					sql.NullString{String: "testdb", Valid: true}, 1, 100, sql.NullInt64{}, 1000,
					"testuser", "testapp", "127.0.0.1", 5432,
					now, now, now, now,
					now, getWaitEventType(tc.name),
					getWaitEvent(tc.name),
					getState(tc.name),
					"client backend", sql.NullInt32{}, sql.NullInt32{}, sql.NullInt64{Int64: tc.queryID, Valid: true},
					sql.NullString{String: tc.query, Valid: true},
					getBlockedByPids(tc.name),
				))

			err = activity.Start(t.Context())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return len(lokiClient.Received()) == len(tc.expectedLines)
			}, 5*time.Second, 100*time.Millisecond)

			activity.Stop()
			lokiClient.Stop()

			require.Eventually(t, func() bool {
				return activity.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

			err = mock.ExpectationsWereMet()
			require.NoError(t, err)

			lokiEntries := lokiClient.Received()
			require.Equal(t, len(tc.expectedLines), len(lokiEntries))
			for i, entry := range lokiEntries {
				require.Equal(t, tc.expectedLabels[i], entry.Labels)
				require.Equal(t, tc.expectedLines[i], entry.Line)
			}
		})
	}
}

func TestActivity_FetchActivity(t *testing.T) {
	defer goleak.VerifyNone(t)

	now := time.Now()

	testCases := []struct {
		name           string
		setupMock      func(mock sqlmock.Sqlmock)
		expectedError  bool
		expectedLabels []model.LabelSet
		expectedLines  []string
	}{
		{
			name: "active query without wait event",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).WithoutArgs().RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"datname", "datid", "pid", "leader_pid", "usesysid",
						"usename", "application_name", "client_addr", "client_port",
						"state_change", "now", "backend_start", "xact_start",
						"query_start", "wait_event_type", "wait_event", "state",
						"backend_type", "backend_xid", "backend_xmin", "query_id",
						"query", "blocked_by_pids",
					}).AddRow(
						"testdb", 1, 100, sql.NullInt64{}, 1000,
						"testuser", "testapp", "127.0.0.1", 5432,
						now, now, now, now,
						now, sql.NullString{}, sql.NullString{}, "active",
						"client backend", sql.NullInt32{Int32: 500, Valid: true}, sql.NullInt32{Int32: 400, Valid: true}, sql.NullInt64{Int64: 123, Valid: true},
						"SELECT * FROM users", nil,
					))
			},
			expectedError: false,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" pid="100" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="500" xmin="400" query_time="%s" queryid="123" query="SELECT * FROM users" engine="postgres" cpu_time="%s"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
			},
		},
		{
			name: "parallel query with leader PID",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).WithoutArgs().RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"datname", "datid", "pid", "leader_pid", "usesysid",
						"usename", "application_name", "client_addr", "client_port",
						"state_change", "now", "backend_start", "xact_start",
						"query_start", "wait_event_type", "wait_event", "state",
						"backend_type", "backend_xid", "backend_xmin", "query_id",
						"query", "blocked_by_pids",
					}).AddRow(
						"testdb", 1, 101, sql.NullInt64{Int64: 100, Valid: true}, 1000,
						"testuser", "testapp", "127.0.0.1", 5432,
						now, now, now, now,
						now, sql.NullString{}, sql.NullString{}, "active",
						"parallel worker", sql.NullInt32{}, sql.NullInt32{}, sql.NullInt64{Int64: 123, Valid: true},
						"SELECT * FROM large_table", nil,
					))
			},
			expectedError: false,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="parallel worker" backend_time="%s" pid="100" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="0" xmin="0" query_time="%s" queryid="123" query="SELECT * FROM large_table" engine="postgres" cpu_time="%s"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
			},
		},
		{
			name: "query with wait event",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).WithoutArgs().RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"datname", "datid", "pid", "leader_pid", "usesysid",
						"usename", "application_name", "client_addr", "client_port",
						"state_change", "now", "backend_start", "xact_start",
						"query_start", "wait_event_type", "wait_event", "state",
						"backend_type", "backend_xid", "backend_xmin", "query_id",
						"query", "blocked_by_pids",
					}).AddRow(
						"testdb", 1, 102, sql.NullInt64{}, 1000,
						"testuser", "testapp", "127.0.0.1", 5432,
						now, now, now, now,
						now, sql.NullString{String: "Lock", Valid: true}, sql.NullString{String: "relation", Valid: true}, "waiting",
						"client backend", sql.NullInt32{}, sql.NullInt32{}, sql.NullInt64{Int64: 124, Valid: true},
						"UPDATE users SET status = 'active'", pq.Int64Array{103, 104},
					))
			},
			expectedError: false,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
				{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" pid="102" user="testuser" userid="1000" datname="testdb" datid="1" xact_time="%s" xid="0" xmin="0" query_time="%s" queryid="124" query="UPDATE users SET status = 'active'" engine="postgres"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
				fmt.Sprintf(`level="info" clock_timestamp="%s" instance="test" user="testuser" userid="1000" datname="testdb" datid="1" backend_type="client backend" state="waiting" wait_time="%s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" queryid="124" query="UPDATE users SET status = 'active'" blocked_by_pids="[103 104]" engine="postgres"`,
					now.Format(time.RFC3339Nano),
					time.Duration(0).String(),
				),
			},
		},
		{
			name: "query with insufficient privilege",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).WithoutArgs().RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"datname", "datid", "pid", "leader_pid", "usesysid",
						"usename", "application_name", "client_addr", "client_port",
						"state_change", "now", "backend_start", "xact_start",
						"query_start", "wait_event_type", "wait_event", "state",
						"backend_type", "backend_xid", "backend_xmin", "query_id",
						"query", "blocked_by_pids",
					}).AddRow(
						"testdb", 1, 105, sql.NullInt64{}, 1000,
						"testuser", "testapp", "127.0.0.1", 5432,
						now, now, now, now,
						now, sql.NullString{}, sql.NullString{}, "active",
						"client backend", sql.NullInt32{}, sql.NullInt32{}, sql.NullInt64{},
						"<insufficient privilege>", nil,
					))
			},
			expectedError: false,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "level": "debug", "op": "log"},
			},
			expectedLines: []string{
				`level=debug msg="invalid pg_stat_activity set" err="insufficient privilege to access query. activity set: {DatabaseName:{String:testdb Valid:true} DatabaseID:1 PID:105 LeaderPID:{Int64:0 Valid:false} UserSysID:1000 Username:{String:testuser Valid:true} ApplicationName:{String:testapp Valid:true} ClientAddr:{String:127.0.0.1 Valid:true} ClientPort:{Int32:5432 Valid:true} StateChange:{Time:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 Valid:true} Now:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 BackendStart:{Time:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 Valid:true} XactStart:{Time:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 Valid:true} QueryStart:{Time:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 Valid:true} WaitEventType:{String: Valid:false} WaitEvent:{String: Valid:false} State:{String:active Valid:true} BackendType:{String:client backend Valid:true} BackendXID:{Int32:0 Valid:false} BackendXmin:{Int32:0 Valid:false} QueryID:{Int64:0 Valid:false} Query:{String:<insufficient privilege> Valid:true} BlockedByPids:[]}"`,
			},
		},
		{
			name: "query with null database name",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).WithoutArgs().RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"datname", "datid", "pid", "leader_pid", "usesysid",
						"usename", "application_name", "client_addr", "client_port",
						"state_change", "now", "backend_start", "xact_start",
						"query_start", "wait_event_type", "wait_event", "state",
						"backend_type", "backend_xid", "backend_xmin", "query_id",
						"query", "blocked_by_pids",
					}).AddRow(
						sql.NullString{}, 1, 106, sql.NullInt64{}, 1000,
						"testuser", "testapp", "127.0.0.1", 5432,
						now, now, now, now,
						now, sql.NullString{}, sql.NullString{}, "active",
						"client backend", sql.NullInt32{}, sql.NullInt32{}, sql.NullInt64{},
						"SELECT 1", nil,
					))
			},
			expectedError: false,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "level": "debug", "op": "log"},
			},
			expectedLines: []string{
				`level=debug msg="invalid pg_stat_activity set" err="database name is not valid. activity set: {DatabaseName:{String: Valid:false} DatabaseID:1 PID:106 LeaderPID:{Int64:0 Valid:false} UserSysID:1000 Username:{String:testuser Valid:true} ApplicationName:{String:testapp Valid:true} ClientAddr:{String:127.0.0.1 Valid:true} ClientPort:{Int32:5432 Valid:true} StateChange:{Time:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 Valid:true} Now:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 Valid:true} BackendStart:{Time:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 Valid:true} XactStart:{Time:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 Valid:true} QueryStart:{Time:2025-08-05 19:12:03.709146048 -0300 -03 m=+0.210592076 Valid:true} WaitEventType:{String: Valid:false} WaitEvent:{String: Valid:false} State:{String:active Valid:true} BackendType:{String:client backend Valid:true} BackendXID:{Int32:0 Valid:false} BackendXmin:{Int32:0 Valid:false} QueryID:{Int64:0 Valid:false} Query:{String:SELECT 1 Valid:true} BlockedByPids:[]}"`,
			},
		},
		{
			name: "query scan error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).WithoutArgs().RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"datname", "datid", "pid", "leader_pid", "usesysid",
						"usename", "application_name", "client_addr", "client_port",
						"state_change", "now", "backend_start", "xact_start",
						"query_start", "wait_event_type", "wait_event", "state",
						"backend_type", "backend_xid", "backend_xmin", "query_id",
						"query", "blocked_by_pids",
					}).AddRow(
						"testdb", "invalid_datid", 100, sql.NullInt64{}, 1000, // datid should be int
						"testuser", "testapp", "127.0.0.1", 5432,
						now, now, now, now,
						now, sql.NullString{}, sql.NullString{}, "active",
						"client backend", sql.NullInt32{}, sql.NullInt32{}, sql.NullInt64{},
						"SELECT * FROM users", nil,
					))
			},
			expectedError: false, // We don't return error for individual row scan failures
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "level": "error", "op": "log"},
			},
			expectedLines: []string{
				`level=info msg="failed to scan pg_stat_activity set" err="sql: Scan error on column index 1, name \"datid\": converting driver.Value type string (\"invalid_datid\") to a int: invalid syntax"`,
			},
		},
		{
			name: "query execution error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).WillReturnError(sql.ErrConnDone)
			},
			expectedError: true,
			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "level": "error", "op": "log"},
			},
			expectedLines: []string{
				`level=error msg="failed to query pg_stat_activity" err="sql: connection is already closed"`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			var logBuf strings.Builder
			logger := log.NewLogfmtLogger(&logBuf)
			lokiClient := loki_fake.NewClient(func() {})

			activity, err := NewActivity(ActivityArguments{
				DB:                    db,
				InstanceKey:           "test",
				CollectInterval:       time.Second,
				DisableQueryRedaction: tc.name == "query with wait event", // Disable redaction for this test
				EntryHandler:          lokiClient,
				Logger:                logger,
			})
			require.NoError(t, err)
			require.NotNil(t, activity)

			// Setup mock expectations
			tc.setupMock(mock)

			err = activity.Start(t.Context())
			require.NoError(t, err)

			if tc.name == "query with insufficient privilege" || tc.name == "query with null database name" || tc.name == "query scan error" || tc.name == "query execution error" {
				// For error/debug/info log test cases, assert on logger output
				require.Eventually(t, func() bool {
					logOutput := logBuf.String()
					for _, expected := range tc.expectedLines {
						if !strings.Contains(logOutput, expected) {
							return false
						}
					}
					activity.Stop()
					return true
				}, 5*time.Second, 100*time.Millisecond)
			} else {
				// For successful cases, assert on lokiClient entries as before
				require.Eventually(t, func() bool {
					entries := lokiClient.Received()
					if len(entries) != len(tc.expectedLines) {
						return false
					}
					for i, entry := range entries {
						if !reflect.DeepEqual(entry.Labels, tc.expectedLabels[i]) {
							return false
						}
						if !strings.Contains(entry.Line, tc.expectedLines[i]) {
							return false
						}
					}
					activity.Stop()
					return true
				}, 5*time.Second, 100*time.Millisecond)
			}

			activity.Stop()
			lokiClient.Stop()

			// Wait for the collector to stop
			require.Eventually(t, func() bool {
				return activity.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

			// Verify mock expectations
			err = mock.ExpectationsWereMet()
			require.NoError(t, err)

			// Verify log entries
			lokiEntries := lokiClient.Received()
			require.Equal(t, len(tc.expectedLines), len(lokiEntries))
			for i, entry := range lokiEntries {
				require.Equal(t, tc.expectedLabels[i], entry.Labels)
				require.Contains(t, entry.Line, tc.expectedLines[i])
			}
		})
	}
}
