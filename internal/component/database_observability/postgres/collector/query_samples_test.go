package collector

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/lib/pq"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/util/syncbuffer"
)

func TestQuerySamples_FetchQuerySamples(t *testing.T) {
	defer goleak.VerifyNone(t)

	now := time.Now()
	// Define different timestamps for testing durations
	stateChangeTime := now.Add(-10 * time.Second) // 10 seconds ago
	queryStartTime := now.Add(-30 * time.Second)  // 30 seconds ago
	xactStartTime := now.Add(-2 * time.Minute)    // 2 minutes ago
	backendStartTime := now.Add(-1 * time.Hour)   // 1 hour ago

	testCases := []struct {
		name                  string
		setupMock             func(mock sqlmock.Sqlmock)
		disableQueryRedaction bool
		expectedErrorLine     string
		expectedLabels        []model.LabelSet
		expectedLines         []string
	}{
		{
			name: "active query without wait event",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
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
						"client backend", backendStartTime, sql.NullInt32{Int32: 500, Valid: true}, sql.NullInt32{Int32: 400, Valid: true},
						xactStartTime, "active", stateChangeTime, sql.NullString{},
						sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 123, Valid: true},
						"SELECT * FROM users",
					))
			},

			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			expectedLines: []string{
				`level="info" datname="testdb" pid="100" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="1h0m0s" xid="500" xmin="400" xact_time="2m0s" state="active" query_time="30s" queryid="123" query="SELECT * FROM users" engine="postgres" cpu_time="10s"`,
			},
		},
		{
			name: "parallel query with leader PID",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
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

			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" datname="testdb" pid="101" leader_pid="100" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="parallel worker" backend_time="%s" xid="0" xmin="0" xact_time="%s" state="active" query_time="%s" queryid="123" query="SELECT * FROM large_table" engine="postgres" cpu_time="%s"`,
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
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
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
						"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
						xactStartTime, "waiting", stateChangeTime, sql.NullString{String: "Lock", Valid: true},
						sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 124, Valid: true},
						"UPDATE users SET status = 'active'",
					))
			},

			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
				{"op": OP_WAIT_EVENT},
			},
			expectedLines: []string{
				`level="info" datname="testdb" pid="102" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="1h0m0s" xid="0" xmin="0" xact_time="2m0s" state="waiting" query_time="0s" queryid="124" query="UPDATE users SET status = ?" engine="postgres"`,
				`level="info" datname="testdb" user="testuser" backend_type="client backend" state="waiting" wait_time="10s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="124" query="UPDATE users SET status = ?" engine="postgres"`,
			},
		},
		{
			name: "insufficient privilege query - no loki entries expected",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
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
			expectedErrorLine: `err="insufficient privilege to access query`,
			expectedLabels:    []model.LabelSet{}, // No Loki entries expected
			expectedLines:     []string{},         // No Loki entries expected
		},
		{
			name: "null database name - no loki entries expected",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
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
			expectedErrorLine: `err="database name is not valid`,
			expectedLabels:    []model.LabelSet{}, // No Loki entries expected
			expectedLines:     []string{},         // No Loki entries expected
		},
		{
			name: "query with redaction disabled",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}).AddRow(
						now, "testdb", 106, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
						xactStartTime, "active", stateChangeTime, sql.NullString{},
						sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 128, Valid: true},
						"SELECT * FROM users WHERE id = 123 AND email = 'test@example.com'",
					))
			},
			disableQueryRedaction: true,
			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			expectedLines: []string{
				`level="info" datname="testdb" pid="106" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" backend_time="1h0m0s" xid="0" xmin="0" xact_time="2m0s" state="active" query_time="30s" queryid="128" query="SELECT * FROM users WHERE id = 123 AND email = 'test@example.com'" engine="postgres" cpu_time="10s"`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			logBuffer := syncbuffer.Buffer{}
			lokiClient := loki_fake.NewClient(func() {})

			sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
				DB:                    db,
				CollectInterval:       time.Second * 5,
				EntryHandler:          lokiClient,
				Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
				DisableQueryRedaction: tc.disableQueryRedaction,
			})
			require.NoError(t, err)
			require.NotNil(t, sampleCollector)

			// Setup mock expectations
			tc.setupMock(mock)

			err = sampleCollector.Start(t.Context())
			require.NoError(t, err)

			// Wait for Loki entries to be generated and verify their content, labels, and timestamps.
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				entries := lokiClient.Received()
				require.Len(t, entries, len(tc.expectedLines))

				require.Contains(t, logBuffer.String(), tc.expectedErrorLine)

				for i, entry := range entries {
					if !reflect.DeepEqual(entry.Labels, tc.expectedLabels[i]) {
						t.Errorf("expected label %v, got %v", tc.expectedLabels[i], entry.Labels)
					}
					require.Contains(t, entry.Line, tc.expectedLines[i])
					// Verify that BuildLokiEntryWithTimestamp is setting the timestamp correctly
					expectedTimestamp := time.Unix(0, now.UnixNano())
					require.True(t, entry.Timestamp.Equal(expectedTimestamp))
				}
			}, 5*time.Second, 100*time.Millisecond)

			sampleCollector.Stop()

			// Wait for the collector to stop
			require.Eventually(t, func() bool {
				return sampleCollector.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

			lokiClient.Stop()

			// Give time for goroutines to clean up
			time.Sleep(100 * time.Millisecond)
			// Run this after Stop() to avoid race conditions
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
