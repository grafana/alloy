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
				mock.ExpectQuery(selectPgStatActivity).WithArgs(sqlmock.AnyArg()).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}).AddRow(
						now, "testdb", 100, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", now, sql.NullInt32{Int32: 500, Valid: true}, sql.NullInt32{Int32: 400, Valid: true},
						now, "active", now, sql.NullString{},
						sql.NullString{}, nil, now, sql.NullInt64{Int64: 123, Valid: true},
						"SELECT * FROM users",
					))
			},
			expectedError: false,

			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" instance="test" datname="testdb" pid="100" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" xid="500" xmin="400" xact_time="%s" state="active" query_time="%s" queryid="123" query="SELECT * FROM users" engine="postgres" cpu_time="%s"`,
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
				mock.ExpectQuery(selectPgStatActivity).WithArgs(sqlmock.AnyArg()).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}).AddRow(
						now, "testdb", 101, sql.NullInt64{Int64: 100, Valid: true},
						"testuser", "testapp", "127.0.0.1", 5432,
						"parallel worker", now, sql.NullInt32{}, sql.NullInt32{},
						now, "active", now, sql.NullString{},
						sql.NullString{}, nil, now, sql.NullInt64{Int64: 123, Valid: true},
						"SELECT * FROM large_table",
					))
			},
			expectedError: false,

			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" instance="test" datname="testdb" pid="101" leader_pid="100" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="parallel worker" backend_time="%s" xid="0" xmin="0" xact_time="%s" state="active" query_time="%s" queryid="123" query="SELECT * FROM large_table" engine="postgres" cpu_time="%s"`,
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
				mock.ExpectQuery(selectPgStatActivity).WithArgs(sqlmock.AnyArg()).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}).AddRow(
						now, "testdb", 102, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", now, sql.NullInt32{}, sql.NullInt32{},
						now, "waiting", now, sql.NullString{String: "Lock", Valid: true},
						sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 124, Valid: true},
						"UPDATE users SET status = 'active'",
					))
			},
			expectedError: false,

			expectedLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "test"},
				{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "test"},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" instance="test" datname="testdb" pid="102" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="%s" xid="0" xmin="0" xact_time="%s" state="waiting" query_time="%s" queryid="124" query="UPDATE users SET status = 'active'" engine="postgres"`,
					time.Duration(0).String(),
					time.Duration(0).String(),
					time.Duration(0).String(),
				),
				fmt.Sprintf(`level="info" instance="test" datname="testdb" backend_type="client backend" state="waiting" wait_time="%s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="124" query="UPDATE users SET status = 'active'" engine="postgres"`,
					time.Duration(0).String(),
				),
			},
		},
		{
			name: "insufficient privilege query - no loki entries expected",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).WithArgs(sqlmock.AnyArg()).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}).AddRow(
						now, "testdb", 103, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", now, sql.NullInt32{}, sql.NullInt32{},
						now, "active", now, sql.NullString{},
						sql.NullString{}, nil, now, sql.NullInt64{Int64: 125, Valid: true},
						"<insufficient privilege>",
					))
			},
			expectedError:  false,
			expectedLabels: []model.LabelSet{}, // No Loki entries expected
			expectedLines:  []string{},         // No Loki entries expected
		},
		{
			name: "null database name - no loki entries expected",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).WithArgs(sqlmock.AnyArg()).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}).AddRow(
						now, sql.NullString{Valid: false}, 104, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", now, sql.NullInt32{}, sql.NullInt32{},
						now, "active", now, sql.NullString{},
						sql.NullString{}, nil, now, sql.NullInt64{Int64: 126, Valid: true},
						"SELECT * FROM users",
					))
			},
			expectedError:  false,
			expectedLabels: []model.LabelSet{}, // No Loki entries expected
			expectedLines:  []string{},         // No Loki entries expected
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			logger := log.NewLogfmtLogger(os.Stderr)
			lokiClient := loki_fake.NewClient(func() {})

			activity, err := NewActivity(ActivityArguments{
				DB:              db,
				InstanceKey:     "test",
				CollectInterval: time.Second * 5,
				EntryHandler:    lokiClient,
				Logger:          logger,
			})
			require.NoError(t, err)
			require.NotNil(t, activity)

			// Setup mock expectations
			tc.setupMock(mock)

			err = activity.Start(t.Context())
			require.NoError(t, err)

			// Wait for Loki entries to be generated
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
				return true
			}, 5*time.Second, 100*time.Millisecond)

			activity.Stop()

			// Wait for the collector to stop
			require.Eventually(t, func() bool {
				return activity.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

			lokiClient.Stop()

			// Give time for goroutines to clean up
			time.Sleep(100 * time.Millisecond)

			// Verify mock expectations and Loki entries
			err = mock.ExpectationsWereMet()
			require.NoError(t, err)

			lokiEntries := lokiClient.Received()
			require.Equal(t, len(tc.expectedLines), len(lokiEntries))
			for i, entry := range lokiEntries {
				require.Equal(t, tc.expectedLabels[i], entry.Labels)
				require.Contains(t, entry.Line, tc.expectedLines[i])
			}
		})
	}
}
