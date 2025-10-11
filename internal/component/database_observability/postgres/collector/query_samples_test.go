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
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}))
			},

			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			expectedLines: []string{
				`level="info" datname="testdb" pid="100" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="active" xid="500" xmin="400" xact_time="2m0s" query_time="30s" queryid="123" query="SELECT * FROM users" engine="postgres" cpu_time="10s"`,
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
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}))
			},

			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			expectedLines: []string{
				fmt.Sprintf(`level="info" datname="testdb" pid="101" leader_pid="100" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="parallel worker" state="active" xid="0" xmin="0" xact_time="%s" query_time="%s" queryid="123" query="SELECT * FROM large_table" engine="postgres" cpu_time="%s"`,
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
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}))
			},

			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
				{"op": OP_WAIT_EVENT},
			},
			expectedLines: []string{
				`level="info" datname="testdb" pid="102" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="waiting" xid="0" xmin="0" xact_time="2m0s" query_time="0s" queryid="124" query="UPDATE users SET status = ?" engine="postgres"`,
				`level="info" datname="testdb" pid="102" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="10s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="124" query="UPDATE users SET status = ?" engine="postgres"`,
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
				// Second scrape: empty to complete cycle
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}))
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
				// Second scrape: empty to complete cycle
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}))
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
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
						"query",
					}))
			},
			disableQueryRedaction: true,
			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			expectedLines: []string{
				`level="info" datname="testdb" pid="106" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="active" xid="0" xmin="0" xact_time="2m0s" query_time="30s" queryid="128" query="SELECT * FROM users WHERE id = 123 AND email = 'test@example.com'" engine="postgres" cpu_time="10s"`,
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
				CollectInterval:       10 * time.Millisecond,
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

func TestQuerySamples_FinalizationScenarios(t *testing.T) {
	defer goleak.VerifyNone(t)

	now := time.Now()
	stateChangeTime := now.Add(-10 * time.Second)
	queryStartTime := now.Add(-30 * time.Second)
	xactStartTime := now.Add(-2 * time.Minute)
	backendStartTime := now.Add(-1 * time.Hour)

	columns := []string{
		"now", "datname", "pid", "leader_pid",
		"usename", "application_name", "client_addr", "client_port",
		"backend_type", "backend_start", "backend_xid", "backend_xmin",
		"xact_start", "state", "state_change", "wait_event_type",
		"wait_event", "blocked_by_pids", "query_start", "query_id",
		"query",
	}

	t.Run("finalize on disappear after active scrape", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki_fake.NewClient(func() {})

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			CollectInterval: 10 * time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
		})
		require.NoError(t, err)

		// First scrape: active row
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 1000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 10, Valid: true}, sql.NullInt32{Int32: 20, Valid: true},
				xactStartTime, "active", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 999, Valid: true},
				"SELECT * FROM t",
			))
		// Second scrape: no rows -> finalize
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 1)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Contains(t, entries[0].Line, `datname="testdb" pid="1000" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="active" xid="10" xmin="20" xact_time="2m0s" query_time="30s" queryid="999" query="SELECT * FROM t" engine="postgres" cpu_time="10s"`)
			expectedTimestamp := time.Unix(0, now.UnixNano())
			require.True(t, entries[0].Timestamp.Equal(expectedTimestamp))
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("xid change finalizes previous sample and starts new", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki_fake.NewClient(func() {})

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			CollectInterval: 10 * time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
		})
		require.NoError(t, err)

		// Scrape 1: xid=1
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 200, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", now.Add(-1*time.Minute), sql.NullInt32{Int32: 1, Valid: true}, sql.NullInt32{},
				now.Add(-30*time.Second), "active", now.Add(-10*time.Second), sql.NullString{},
				sql.NullString{}, nil, now.Add(-10*time.Second), sql.NullInt64{Int64: 777, Valid: true},
				"SELECT 1",
			))
		// Scrape 2: xid=2 (same pid/queryid)
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 200, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", now, sql.NullInt32{Int32: 2, Valid: true}, sql.NullInt32{},
				now, "active", now, sql.NullString{},
				sql.NullString{}, nil, now, sql.NullInt64{Int64: 777, Valid: true},
				"SELECT 1",
			))
		// Scrape 3: disappear -> finalize xid=2
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 2)
			// First emitted: xid=1
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Contains(t, entries[0].Line, `xid="1"`)
			require.Contains(t, entries[0].Line, `queryid="777"`)
			require.Contains(t, entries[0].Line, `cpu_time="10s"`)
			// Second emitted: xid=2
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[1].Labels)
			require.Contains(t, entries[1].Line, `xid="2"`)
			require.Contains(t, entries[1].Line, `queryid="777"`)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("wait-event merges across scrapes with normalized PID set", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki_fake.NewClient(func() {})

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			CollectInterval: 10 * time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
		})
		require.NoError(t, err)

		// Scrape 1: wait event with unordered/dup PIDs
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 300, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-10*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{104, 103}, now, sql.NullInt64{Int64: 124, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 2: same wait event with normalized PIDs
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 300, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-12*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 124, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 3: disappear
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 2)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
			require.Contains(t, entries[1].Line, `wait_time="12s"`)
			require.Contains(t, entries[1].Line, `blocked_by_pids="[103 104]"`)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("wait-event closes on no-wait row; single occurrence emitted", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki_fake.NewClient(func() {})

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			CollectInterval: 10 * time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
		})
		require.NoError(t, err)

		// Scrape 1: wait event
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 301, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", stateChangeTime, sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 555, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 2: active with no wait -> close occurrence
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 301, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "active", now, sql.NullString{},
				sql.NullString{}, nil, now, sql.NullInt64{Int64: 555, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 3: disappear
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 2)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
			require.Contains(t, entries[1].Line, `wait_time="10s"`)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// Finalize when row turns idle (non-client backend)
	t.Run("finalize when row turns idle", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki_fake.NewClient(func() {})

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			CollectInterval: 10 * time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
		})
		require.NoError(t, err)

		// Scrape 1: parallel worker active
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 401, sql.NullInt64{Int64: 400, Valid: true},
				"testuser", "testapp", "127.0.0.1", 5432,
				"parallel worker", backendStartTime, sql.NullInt32{Int32: 42, Valid: true}, sql.NullInt32{},
				xactStartTime, "active", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 9001, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 2: same row turns idle (allowed for non-client backend)
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 401, sql.NullInt64{Int64: 400, Valid: true},
				"testuser", "testapp", "127.0.0.1", 5432,
				"parallel worker", backendStartTime, sql.NullInt32{Int32: 42, Valid: true}, sql.NullInt32{},
				xactStartTime, "idle", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 9001, Valid: true},
				"SELECT * FROM t",
			))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 1)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Contains(t, entries[0].Line, `leader_pid="400"`)
			require.Contains(t, entries[0].Line, `backend_type="parallel worker"`)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// CPU persists across later waits
	t.Run("cpu persists across waits", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki_fake.NewClient(func() {})

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			CollectInterval: 10 * time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
		})
		require.NoError(t, err)

		// Scrape 1: active CPU snapshot (10s)
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 402, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "active", now.Add(-10*time.Second), sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 9002, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 2: waiting with wait_event; state_change 7s ago
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 402, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-7*time.Second), sql.NullString{String: "IO", Valid: true},
				sql.NullString{String: "DataFileRead", Valid: true}, pq.Int64Array{501}, queryStartTime, sql.NullInt64{Int64: 9002, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 3: disappear -> finalize
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 2)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Contains(t, entries[0].Line, `cpu_time="10s"`)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
			require.Contains(t, entries[1].Line, `state="waiting"`)
			require.Contains(t, entries[1].Line, `wait_time="7s"`)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// New occurrence when blocked_by_pids set changes
	t.Run("wait-event starts new occurrence on set change", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki_fake.NewClient(func() {})

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:              db,
			CollectInterval: 10 * time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
		})
		require.NoError(t, err)

		// Scrape 1: wait event set A
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 403, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-5*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103}, queryStartTime, sql.NullInt64{Int64: 9003, Valid: true},
				"UPDATE t SET c=1",
			))
		// Scrape 2: same event, set changes -> new occurrence
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 403, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-8*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, queryStartTime, sql.NullInt64{Int64: 9003, Valid: true},
				"UPDATE t SET c=1",
			))
		// Scrape 3: disappear -> finalize
		mock.ExpectQuery(selectPgStatActivity).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 3)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
			require.Contains(t, entries[1].Line, `state="waiting"`)
			require.Contains(t, entries[1].Line, `blocked_by_pids="[103]"`)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[2].Labels)
			require.Contains(t, entries[2].Line, `blocked_by_pids="[103 104]"`)
			require.Contains(t, entries[2].Line, `wait_time="8s"`)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
