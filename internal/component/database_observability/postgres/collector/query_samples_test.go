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
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "")).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
					}).AddRow(
						now, "testdb", 100, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", backendStartTime, sql.NullInt32{Int32: 500, Valid: true}, sql.NullInt32{Int32: 400, Valid: true},
						xactStartTime, "active", stateChangeTime, sql.NullString{},
						sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 123, Valid: true},
					))
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "")).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
					}))
			},

			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			expectedLines: []string{
				`level="info" datname="testdb" pid="100" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="active" xid="500" xmin="400" xact_time="2m0s" query_time="30s" queryid="123" cpu_time="10s"`,
			},
		},
		{
			name: "parallel query with leader PID",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "")).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
					}).AddRow(
						now, "testdb", 101, sql.NullInt64{Int64: 100, Valid: true},
						"testuser", "testapp", "127.0.0.1", 5432,
						"parallel worker", now, sql.NullInt32{}, sql.NullInt32{},
						now, "active", now, sql.NullString{},
						sql.NullString{}, nil, now, sql.NullInt64{Int64: 123, Valid: true},
					))
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "")).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
					}))
			},

			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
			},
			expectedLines: []string{
				`level="info" datname="testdb" pid="101" leader_pid="100" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="parallel worker" state="active" xid="0" xmin="0" xact_time="0s" query_time="0s" queryid="123" cpu_time="0s"`, // time.Duration(0).String(),
			},
		},
		{
			name: "query with wait event",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "")).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
					}).AddRow(
						now, "testdb", 102, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
						xactStartTime, "waiting", stateChangeTime, sql.NullString{String: "Lock", Valid: true},
						sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 124, Valid: true},
					))
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "")).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
					}))
			},

			expectedLabels: []model.LabelSet{
				{"op": OP_QUERY_SAMPLE},
				{"op": OP_WAIT_EVENT},
			},
			expectedLines: []string{
				`level="info" datname="testdb" pid="102" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="waiting" xid="0" xmin="0" xact_time="2m0s" query_time="0s" queryid="124"`,
				`level="info" datname="testdb" pid="102" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="10s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="124"`,
			},
		},
		{
			name: "insufficient privilege query - no loki entries expected",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
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
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
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
			expectedErrorLine:     `err="insufficient privilege to access query`,
			expectedLabels:        []model.LabelSet{}, // No Loki entries expected
			expectedLines:         []string{},         // No Loki entries expected
		},
		{
			name: "null database name - no loki entries expected",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "")).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
					}).AddRow(
						now, sql.NullString{Valid: false}, 104, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", now, sql.NullInt32{}, sql.NullInt32{},
						now, "active", now, sql.NullString{},
						sql.NullString{}, nil, now, sql.NullInt64{Int64: 126, Valid: true},
					))
				// Second scrape: empty to complete cycle
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "")).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows([]string{
						"now", "datname", "pid", "leader_pid",
						"usename", "application_name", "client_addr", "client_port",
						"backend_type", "backend_start", "backend_xid", "backend_xmin",
						"xact_start", "state", "state_change", "wait_event_type",
						"wait_event", "blocked_by_pids", "query_start", "query_id",
					}))
			},
			expectedErrorLine: `err="database name is not valid`,
			expectedLabels:    []model.LabelSet{}, // No Loki entries expected
			expectedLines:     []string{},         // No Loki entries expected
		},
		{
			name: "query with redaction disabled",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
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
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
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
				`level="info" datname="testdb" pid="106" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="active" xid="0" xmin="0" xact_time="2m0s" query_time="30s" queryid="128" cpu_time="10s" query="SELECT * FROM users WHERE id = 123 AND email = 'test@example.com'"`,
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
			defer lokiClient.Stop()

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
				require.Equal(t, tc.expectedLines[i], entry.Line)
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
			DB:                    db,
			CollectInterval:       10 * time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)

		// First scrape: active row
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 1000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 10, Valid: true}, sql.NullInt32{Int32: 20, Valid: true},
				xactStartTime, "active", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 999, Valid: true},
				"SELECT * FROM t",
			))
		// Second scrape: no rows -> finalize
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 1)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Equal(t, `level="info" datname="testdb" pid="1000" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="active" xid="10" xmin="20" xact_time="2m0s" query_time="30s" queryid="999" cpu_time="10s" query="SELECT * FROM t"`, entries[0].Line)
			expectedTimestamp := time.Unix(0, now.UnixNano())
			require.True(t, entries[0].Timestamp.Equal(expectedTimestamp))
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
			DB:                    db,
			CollectInterval:       10 * time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
		})

		require.NoError(t, err)
		// Scrape 1: wait event with unordered/dup PIDs
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 300, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-10*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{104, 103}, now, sql.NullInt64{Int64: 124, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 2: same wait event with normalized PIDs
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 300, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-12*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 124, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 3: disappear
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 2)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
			require.Equal(t, `level="info" datname="testdb" pid="300" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="12s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="124"`, entries[1].Line)
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
			DB:                    db,
			CollectInterval:       10 * time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)

		// Scrape 1: wait event
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 301, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", stateChangeTime, sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 555, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 2: active with no wait -> close occurrence
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 301, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "active", now, sql.NullString{},
				sql.NullString{}, nil, now, sql.NullInt64{Int64: 555, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 3: disappear
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 2)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Equal(t, `level="info" datname="testdb" pid="301" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="active" xid="0" xmin="0" xact_time="2m0s" query_time="0s" queryid="555" cpu_time="0s" query="UPDATE users SET status = 'active'"`, entries[0].Line)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
			require.Equal(t, `level="info" datname="testdb" pid="301" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="10s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="555"`, entries[1].Line)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("cpu persists across waits", func(t *testing.T) {
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
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)

		// Scrape 1: active CPU snapshot (10s)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 402, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "active", now.Add(-10*time.Second), sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 9002, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 2: waiting with wait_event; state_change 7s ago
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 402, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-7*time.Second), sql.NullString{String: "IO", Valid: true},
				sql.NullString{String: "DataFileRead", Valid: true}, pq.Int64Array{501}, queryStartTime, sql.NullInt64{Int64: 9002, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 3: disappear -> finalize
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 2)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Equal(t, `level="info" datname="testdb" pid="402" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="waiting" xid="0" xmin="0" xact_time="2m0s" query_time="30s" queryid="9002" cpu_time="10s" query="SELECT * FROM t"`, entries[0].Line)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
			require.Equal(t, `level="info" datname="testdb" pid="402" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="7s" wait_event_type="IO" wait_event="DataFileRead" wait_event_name="IO:DataFileRead" blocked_by_pids="[501]" queryid="9002"`, entries[1].Line)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("wait-event starts new occurrence on set change", func(t *testing.T) {
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
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)

		// Scrape 1: wait event set A
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 403, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-5*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103}, queryStartTime, sql.NullInt64{Int64: 9003, Valid: true},
				"UPDATE t SET c=1",
			))
		// Scrape 2: same event, set changes -> new occurrence
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 403, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-8*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, queryStartTime, sql.NullInt64{Int64: 9003, Valid: true},
				"UPDATE t SET c=1",
			))
		// Scrape 3: disappear -> finalize
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 3)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Equal(t, `level="info" datname="testdb" pid="403" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="waiting" xid="0" xmin="0" xact_time="2m0s" query_time="30s" queryid="9003" query="UPDATE t SET c=1"`, entries[0].Line)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
			require.Equal(t, `level="info" datname="testdb" pid="403" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="5s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103]" queryid="9003"`, entries[1].Line)
			require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[2].Labels)
			require.Equal(t, `level="info" datname="testdb" pid="403" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="8s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="9003"`, entries[2].Line)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestQuerySamples_IdleScenarios(t *testing.T) {
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

	t.Run("emit at idle state with end at state_change", func(t *testing.T) {
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
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)

		// Scrape 1: active row
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 11, Valid: true}, sql.NullInt32{Int32: 22, Valid: true},
				xactStartTime, "active", now.Add(-10*time.Second), sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 20002, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 2: same key turns idle; state_change denotes end
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 11, Valid: true}, sql.NullInt32{Int32: 22, Valid: true},
				xactStartTime, "idle", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 20002, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 3: still idle -> must not emit again
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 11, Valid: true}, sql.NullInt32{Int32: 22, Valid: true},
				xactStartTime, "idle", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 20002, Valid: true},
				"SELECT * FROM t",
			))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 1)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			// query duration should be state_change - query_start = 20s
			require.Contains(t, entries[0].Line, `query_time="20s"`)
			// cpu_time captured from active snapshot
			require.Contains(t, entries[0].Line, `cpu_time="10s"`)
			// timestamp equals state_change
			expectedTs := time.Unix(0, stateChangeTime.UnixNano())
			require.True(t, entries[0].Timestamp.Equal(expectedTs))
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("idle-only emitted once and deduped across scrapes", func(t *testing.T) {
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
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)

		// Scrape 1: only idle row
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2001, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 0, Valid: false}, sql.NullInt32{Int32: 0, Valid: false},
				xactStartTime, "idle", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 20003, Valid: true},
				"SELECT * FROM users",
			))
		// Scrape 2: same idle row again -> should not re-emit
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2001, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 0, Valid: false}, sql.NullInt32{Int32: 0, Valid: false},
				xactStartTime, "idle", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 20003, Valid: true},
				"SELECT * FROM users",
			))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			require.Len(t, entries, 1)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			// query duration should be state_change - query_start = 20s
			require.Contains(t, entries[0].Line, `query_time="20s"`)
			// timestamp equals state_change
			expectedTs := time.Unix(0, stateChangeTime.UnixNano())
			require.True(t, entries[0].Timestamp.Equal(expectedTs))
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestQuerySamples_Throttling(t *testing.T) {
	defer goleak.VerifyNone(t)

	now := time.Now()
	stateChangeTime := now.Add(-10 * time.Second)
	queryStartTime1 := now.Add(-30 * time.Second)
	backendStartTime := now.Add(-1 * time.Hour)

	columns := []string{
		"now", "datname", "pid", "leader_pid",
		"usename", "application_name", "client_addr", "client_port",
		"backend_type", "backend_start", "backend_xid", "backend_xmin",
		"xact_start", "state", "state_change", "wait_event_type",
		"wait_event", "blocked_by_pids", "query_start", "query_id",
		"query",
	}

	t.Run("suppresses second emission within window for same queryid", func(t *testing.T) {
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
			DisableQueryRedaction: true,
			ThrottleInterval:      500 * time.Millisecond,
		})
		require.NoError(t, err)

		// First occurrence: active -> idle (emits)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "active", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime1, sql.NullInt64{Int64: 777, Valid: true},
				"SELECT 1",
			))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "idle", now, sql.NullString{},
				sql.NullString{}, nil, queryStartTime1, sql.NullInt64{Int64: 777, Valid: true},
				"SELECT 1",
			))

		// Second occurrence (same queryid): active -> idle within throttle window (should be suppressed)
		queryStartTime2 := now.Add(100 * time.Millisecond)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "active", now.Add(-50*time.Millisecond), sql.NullString{},
				sql.NullString{}, nil, queryStartTime2, sql.NullInt64{Int64: 777, Valid: true},
				"SELECT 1",
			))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "idle", now, sql.NullString{},
				sql.NullString{}, nil, queryStartTime2, sql.NullInt64{Int64: 777, Valid: true},
				"SELECT 1",
			))

		// Allow subsequent scrapes to proceed without new rows
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			// Only the first emission should pass
			require.Len(t, entries, 1)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("does not throttle idle in transaction emissions", func(t *testing.T) {
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
			DisableQueryRedaction: true,
			ThrottleInterval:      500 * time.Millisecond,
		})
		require.NoError(t, err)

		// First occurrence: active -> idle (emits)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8100, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "active", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime1, sql.NullInt64{Int64: 888, Valid: true},
				"SELECT 1",
			))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8100, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "idle", now, sql.NullString{},
				sql.NullString{}, nil, queryStartTime1, sql.NullInt64{Int64: 888, Valid: true},
				"SELECT 1",
			))

		// Second occurrence (same queryid): active -> idle in transaction -> disappear (should emit despite throttle)
		queryStartTime3 := now.Add(100 * time.Millisecond)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8100, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "active", now.Add(-50*time.Millisecond), sql.NullString{},
				sql.NullString{}, nil, queryStartTime3, sql.NullInt64{Int64: 888, Valid: true},
				"SELECT 1",
			))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8100, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "idle in transaction", now, sql.NullString{},
				sql.NullString{}, nil, queryStartTime3, sql.NullInt64{Int64: 888, Valid: true},
				"SELECT 1",
			))
		// Disappear to finalize
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			// Both emissions should be present
			require.Len(t, entries, 2)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[1].Labels)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("does not throttle when no CPU captured (waiting-only occurrences)", func(t *testing.T) {
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
			DisableQueryRedaction: true,
			ThrottleInterval:      500 * time.Millisecond,
		})
		require.NoError(t, err)

		// Occurrence 1: waiting only
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8200, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "waiting", now.Add(-5*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{1}, queryStartTime1, sql.NullInt64{Int64: 999, Valid: true},
				"UPDATE t SET c=1",
			))
		// Disappear
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		// Occurrence 2: waiting only with different query_start but same queryid (within throttle window)
		queryStartTime4 := now.Add(100 * time.Millisecond)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8200, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				now.Add(-2*time.Minute), "waiting", now.Add(-2*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{1}, queryStartTime4, sql.NullInt64{Int64: 999, Valid: true},
				"UPDATE t SET c=1",
			))
		// Disappear
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			entries := lokiClient.Received()
			// Expect two query sample emissions (throttle bypass due to no CPU captured)
			numSamples := 0
			for _, e := range entries {
				if reflect.DeepEqual(e.Labels, model.LabelSet{"op": OP_QUERY_SAMPLE}) {
					numSamples++
				}
			}
			require.Equal(t, 2, numSamples)
		}, 5*time.Second, 50*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		lokiClient.Stop()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
