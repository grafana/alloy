package collector

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/lib/pq"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util/syncbuffer"
)

func TestQuerySamples_FetchQuerySamples(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	now := time.Now()
	// Define different timestamps for testing durations
	stateChangeTime := now.Add(-10 * time.Second) // 10 seconds ago
	queryStartTime := now.Add(-30 * time.Second)  // 30 seconds ago
	xactStartTime := now.Add(-2 * time.Minute)    // 2 minutes ago
	backendStartTime := now.Add(-1 * time.Hour)   // 1 hour ago

	columns := []string{
		"now", "datname", "pid", "leader_pid",
		"usename", "application_name", "client_addr", "client_port",
		"backend_type", "backend_start", "backend_xid", "backend_xmin",
		"xact_start", "state", "state_change", "wait_event_type",
		"wait_event", "blocked_by_pids", "query_start", "query_id",
	}

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
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "", excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(columns).AddRow(
						now, "testdb", 100, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", backendStartTime, sql.NullInt32{Int32: 500, Valid: true}, sql.NullInt32{Int32: 400, Valid: true},
						xactStartTime, "active", stateChangeTime, sql.NullString{},
						sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 123, Valid: true},
					))
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "", excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(columns))
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
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "", excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(columns).AddRow(
						now, "testdb", 101, sql.NullInt64{Int64: 100, Valid: true},
						"testuser", "testapp", "127.0.0.1", 5432,
						"parallel worker", now, sql.NullInt32{}, sql.NullInt32{},
						now, "active", now, sql.NullString{},
						sql.NullString{}, nil, now, sql.NullInt64{Int64: 123, Valid: true},
					))
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "", excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(columns))
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
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "", excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(columns).AddRow(
						now, "testdb", 102, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
						xactStartTime, "waiting", stateChangeTime, sql.NullString{String: "Lock", Valid: true},
						sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 124, Valid: true},
					))
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "", excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(columns))
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
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(append(columns, "query")).AddRow(
						now, "testdb", 103, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", now, sql.NullInt32{}, sql.NullInt32{},
						now, "active", now, sql.NullString{},
						sql.NullString{}, nil, now, sql.NullInt64{Int64: 125, Valid: true},
						"<insufficient privilege>",
					))
				// Second scrape: empty to complete cycle
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(append(columns, "query")))
			},
			disableQueryRedaction: true,
			expectedErrorLine:     `err="insufficient privilege to access query`,
			expectedLabels:        []model.LabelSet{}, // No Loki entries expected
			expectedLines:         []string{},         // No Loki entries expected
		},
		{
			name: "null database name - no loki entries expected",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "", excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(columns).AddRow(
						now, sql.NullString{Valid: false}, 104, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", now, sql.NullInt32{}, sql.NullInt32{},
						now, "active", now, sql.NullString{},
						sql.NullString{}, nil, now, sql.NullInt64{Int64: 126, Valid: true},
					))
				// Second scrape: empty to complete cycle
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, "", excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(columns))
			},
			expectedErrorLine: `err="database name is not valid`,
			expectedLabels:    []model.LabelSet{}, // No Loki entries expected
			expectedLines:     []string{},         // No Loki entries expected
		},
		{
			name: "query with redaction disabled",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(append(columns, "query")).AddRow(
						now, "testdb", 106, sql.NullInt64{},
						"testuser", "testapp", "127.0.0.1", 5432,
						"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
						xactStartTime, "active", stateChangeTime, sql.NullString{},
						sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 128, Valid: true},
						"SELECT * FROM users WHERE id = 123 AND email = 'test@example.com'",
					))
				// Second scrape: empty to trigger finalization
				mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
					WillReturnRows(sqlmock.NewRows(append(columns, "query")))
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
			lokiClient := loki.NewCollectingHandler()
			defer lokiClient.Stop()

			sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
				DB:                    db,
				CollectInterval:       time.Millisecond,
				EntryHandler:          lokiClient,
				Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
				DisableQueryRedaction: tc.disableQueryRedaction,
				ExcludeCurrentUser:    true,
			})
			require.NoError(t, err)
			require.NotNil(t, sampleCollector)

			tc.setupMock(mock)

			require.NoError(t, sampleCollector.Start(t.Context()))

			// For error cases, wait for error message in logs
			if tc.expectedErrorLine != "" {
				require.Eventually(t, func() bool {
					return strings.Contains(logBuffer.String(), tc.expectedErrorLine)
				}, 5*time.Second, 100*time.Millisecond)
			}

			require.Eventually(t, func() bool {
				return len(lokiClient.Received()) == len(tc.expectedLines)
			}, 5*time.Second, 100*time.Millisecond)

			entries := lokiClient.Received()
			for i, entry := range entries {
				if !reflect.DeepEqual(entry.Labels, tc.expectedLabels[i]) {
					t.Errorf("expected label %v, got %v", tc.expectedLabels[i], entry.Labels)
				}
				require.Equal(t, entry.Line, tc.expectedLines[i])
				// Verify that BuildLokiEntryWithTimestamp is setting the timestamp correctly
				expectedTimestamp := time.Unix(0, now.UnixNano())
				require.True(t, entry.Timestamp.Equal(expectedTimestamp))
			}

			sampleCollector.Stop()
			require.Eventually(t, func() bool {
				return sampleCollector.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

			require.Eventually(t, func() bool {
				return mock.ExpectationsWereMet() == nil
			}, 5*time.Second, 100*time.Millisecond)
		})
	}
}

func TestQuerySamples_FinalizationScenarios(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

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
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
			ExcludeCurrentUser:    true,
		})
		require.NoError(t, err)

		// First scrape: active row
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 1000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 10, Valid: true}, sql.NullInt32{Int32: 20, Valid: true},
				xactStartTime, "active", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 999, Valid: true},
				"SELECT * FROM t",
			))
		// Second scrape: no rows -> finalize
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		entries := lokiClient.Received()
		require.Len(t, entries, 1)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
		require.Equal(t, `level="info" datname="testdb" pid="1000" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="active" xid="10" xmin="20" xact_time="2m0s" query_time="30s" queryid="999" cpu_time="10s" query="SELECT * FROM t"`, entries[0].Line)
		expectedTimestamp := time.Unix(0, now.UnixNano())
		require.True(t, entries[0].Timestamp.Equal(expectedTimestamp))

		sampleCollector.Stop()
		require.Eventually(t, func() bool {
			return sampleCollector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("wait-event merges across scrapes with normalized PID set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
			ExcludeCurrentUser:    true,
		})

		require.NoError(t, err)
		// Scrape 1: wait event with unordered/dup PIDs
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 300, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-10*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{104, 103}, now, sql.NullInt64{Int64: 124, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 2: same wait event with normalized PIDs
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 300, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-12*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 124, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 3: disappear
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 5*time.Second, 100*time.Millisecond)

		entries := lokiClient.Received()
		require.Len(t, entries, 2)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
		require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
		require.Equal(t, `level="info" datname="testdb" pid="300" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="12s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="124"`, entries[1].Line)

		sampleCollector.Stop()
		require.Eventually(t, func() bool {
			return sampleCollector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("wait-event closes on no-wait row; single occurrence emitted", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
			ExcludeCurrentUser:    true,
		})
		require.NoError(t, err)

		// Scrape 1: wait event
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 301, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "active", stateChangeTime, sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, now, sql.NullInt64{Int64: 555, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 2: active with no wait -> close occurrence
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 301, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "active", now, sql.NullString{},
				sql.NullString{}, nil, now, sql.NullInt64{Int64: 555, Valid: true},
				"UPDATE users SET status = 'active'",
			))
		// Scrape 3: disappear
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 5*time.Second, 100*time.Millisecond)

		entries := lokiClient.Received()
		require.Len(t, entries, 2)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
		require.Equal(t, `level="info" datname="testdb" pid="301" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="active" xid="0" xmin="0" xact_time="2m0s" query_time="0s" queryid="555" cpu_time="0s" query="UPDATE users SET status = 'active'"`, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
		require.Equal(t, `level="info" datname="testdb" pid="301" leader_pid="" user="testuser" backend_type="client backend" state="active" xid="0" xmin="0" wait_time="10s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="555"`, entries[1].Line)

		sampleCollector.Stop()
		require.Eventually(t, func() bool {
			return sampleCollector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("cpu persists across waits", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
			ExcludeCurrentUser:    true,
		})
		require.NoError(t, err)

		// Scrape 1: active CPU snapshot (10s)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 402, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "active", now.Add(-10*time.Second), sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 9002, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 2: waiting with wait_event; state_change 7s ago
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 402, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-7*time.Second), sql.NullString{String: "IO", Valid: true},
				sql.NullString{String: "DataFileRead", Valid: true}, pq.Int64Array{501}, queryStartTime, sql.NullInt64{Int64: 9002, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 3: disappear -> finalize
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 5*time.Second, 100*time.Millisecond)

		entries := lokiClient.Received()
		require.Len(t, entries, 2)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
		require.Equal(t, `level="info" datname="testdb" pid="402" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="waiting" xid="0" xmin="0" xact_time="2m0s" query_time="30s" queryid="9002" cpu_time="10s" query="SELECT * FROM t"`, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
		require.Equal(t, `level="info" datname="testdb" pid="402" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="7s" wait_event_type="IO" wait_event="DataFileRead" wait_event_name="IO:DataFileRead" blocked_by_pids="[501]" queryid="9002"`, entries[1].Line)

		sampleCollector.Stop()
		require.Eventually(t, func() bool {
			return sampleCollector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("wait-event starts new occurrence on set change", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
			ExcludeCurrentUser:    true,
		})
		require.NoError(t, err)

		// Scrape 1: wait event set A
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 403, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-5*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103}, queryStartTime, sql.NullInt64{Int64: 9003, Valid: true},
				"UPDATE t SET c=1",
			))
		// Scrape 2: same event, set changes -> new occurrence
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 403, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "waiting", now.Add(-8*time.Second), sql.NullString{String: "Lock", Valid: true},
				sql.NullString{String: "relation", Valid: true}, pq.Int64Array{103, 104}, queryStartTime, sql.NullInt64{Int64: 9003, Valid: true},
				"UPDATE t SET c=1",
			))
		// Scrape 3: disappear -> finalize
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		entries := lokiClient.Received()
		require.Len(t, entries, 3)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
		require.Equal(t, `level="info" datname="testdb" pid="403" leader_pid="" user="testuser" app="testapp" client="127.0.0.1:5432" backend_type="client backend" state="waiting" xid="0" xmin="0" xact_time="2m0s" query_time="30s" queryid="9003" query="UPDATE t SET c=1"`, entries[0].Line)
		require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[1].Labels)
		require.Equal(t, `level="info" datname="testdb" pid="403" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="5s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103]" queryid="9003"`, entries[1].Line)
		require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, entries[2].Labels)
		require.Equal(t, `level="info" datname="testdb" pid="403" leader_pid="" user="testuser" backend_type="client backend" state="waiting" xid="0" xmin="0" wait_time="8s" wait_event_type="Lock" wait_event="relation" wait_event_name="Lock:relation" blocked_by_pids="[103 104]" queryid="9003"`, entries[2].Line)

		sampleCollector.Stop()
		require.Eventually(t, func() bool {
			return sampleCollector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestQuerySamples_IdleScenarios(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

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
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
			ExcludeCurrentUser:    true,
		})
		require.NoError(t, err)

		// Scrape 1: active row
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 11, Valid: true}, sql.NullInt32{Int32: 22, Valid: true},
				xactStartTime, "active", now.Add(-10*time.Second), sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 20002, Valid: true},
				"SELECT * FROM t",
			))
		// Scrape 2: same key turns idle; state_change denotes end
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2000, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 11, Valid: true}, sql.NullInt32{Int32: 22, Valid: true},
				xactStartTime, "idle", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 20002, Valid: true},
				"SELECT * FROM t",
			))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		entries := lokiClient.Received()
		require.Len(t, entries, 1)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
		require.Contains(t, entries[0].Line, `query_time="20s"`)
		require.Contains(t, entries[0].Line, `cpu_time="10s"`)
		expectedTs := time.Unix(0, stateChangeTime.UnixNano())
		require.True(t, entries[0].Timestamp.Equal(expectedTs))

		sampleCollector.Stop()
		require.Eventually(t, func() bool {
			return sampleCollector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("idle-only emitted once and deduped across scrapes", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
			ExcludeCurrentUser:    true,
		})
		require.NoError(t, err)

		// Scrape 1: only idle row
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2001, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 0, Valid: false}, sql.NullInt32{Int32: 0, Valid: false},
				xactStartTime, "idle", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 20003, Valid: true},
				"SELECT * FROM users",
			))
		// Scrape 2: same idle row again -> should not re-emit
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2001, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{Int32: 0, Valid: false}, sql.NullInt32{Int32: 0, Valid: false},
				xactStartTime, "idle", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 20003, Valid: true},
				"SELECT * FROM users",
			))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		entries := lokiClient.Received()
		require.Len(t, entries, 1)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
		require.Contains(t, entries[0].Line, `query_time="20s"`)
		expectedTs := time.Unix(0, stateChangeTime.UnixNano())
		require.True(t, entries[0].Timestamp.Equal(expectedTs))

		sampleCollector.Stop()
		require.Eventually(t, func() bool {
			return sampleCollector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("idle in transaction (aborted) emitted once and deduped across scrapes", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
			ExcludeCurrentUser:    true,
		})
		require.NoError(t, err)

		// Scrape 1: idle in transaction (aborted)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2100, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "idle in transaction (aborted)", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 21002, Valid: true},
				"SELECT 1",
			))
		// Scrape 2: same idle row again -> should not re-emit
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 2100, sql.NullInt64{},
				"testuser", "testapp", "127.0.0.1", 5432,
				"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
				xactStartTime, "idle in transaction (aborted)", stateChangeTime, sql.NullString{},
				sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 21002, Valid: true},
				"SELECT 1",
			))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		entries := lokiClient.Received()
		require.Len(t, entries, 1)
		require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, entries[0].Labels)
		// End timestamp should match state_change
		expectedTs := time.Unix(0, stateChangeTime.UnixNano())
		require.True(t, entries[0].Timestamp.Equal(expectedTs))

		sampleCollector.Stop()
		require.Eventually(t, func() bool {
			return sampleCollector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("two idle-only keys emit separately and dedup individually", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
			ExcludeCurrentUser:    true,
		})
		require.NoError(t, err)

		// Scrape 1: two idle-only rows with different keys (PID/QueryID)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).
				AddRow(
					now, "testdb", 2200, sql.NullInt64{},
					"testuser", "testapp", "127.0.0.1", 5432,
					"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
					xactStartTime, "idle", stateChangeTime, sql.NullString{},
					sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 22002, Valid: true},
					"SELECT * FROM a",
				).
				AddRow(
					now, "testdb", 2300, sql.NullInt64{},
					"testuser", "testapp", "127.0.0.1", 5432,
					"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
					xactStartTime, "idle", stateChangeTime, sql.NullString{},
					sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 23002, Valid: true},
					"SELECT * FROM b",
				))
		// Scrape 2: same idle rows again -> should not re-emit
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, excludeCurrentUserClause)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(columns).
				AddRow(
					now, "testdb", 2200, sql.NullInt64{},
					"testuser", "testapp", "127.0.0.1", 5432,
					"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
					xactStartTime, "idle", stateChangeTime, sql.NullString{},
					sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 22002, Valid: true},
					"SELECT * FROM a",
				).
				AddRow(
					now, "testdb", 2300, sql.NullInt64{},
					"testuser", "testapp", "127.0.0.1", 5432,
					"client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{},
					xactStartTime, "idle", stateChangeTime, sql.NullString{},
					sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 23002, Valid: true},
					"SELECT * FROM b",
				))

		require.NoError(t, sampleCollector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 5*time.Second, 100*time.Millisecond)

		entries := lokiClient.Received()
		require.Len(t, entries, 2)
		// Both entries should be OP_QUERY_SAMPLE
		for _, e := range entries {
			require.Equal(t, model.LabelSet{"op": OP_QUERY_SAMPLE}, e.Labels)
		}
		// Ensure both queryids are present among the two entries
		var seen22002, seen23002 bool
		for _, e := range entries {
			if strings.Contains(e.Line, `queryid="22002"`) {
				seen22002 = true
			}
			if strings.Contains(e.Line, `queryid="23002"`) {
				seen23002 = true
			}
		}
		require.True(t, seen22002 && seen23002)

		sampleCollector.Stop()
		require.Eventually(t, func() bool {
			return sampleCollector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestQuerySamples_ExcludeCurrentUser(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

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
	}

	testCases := []struct {
		name               string
		excludeCurrentUser bool
		expectedQuery      string
	}{
		{
			name:               "ExcludeCurrentUser enabled",
			excludeCurrentUser: true,
			expectedQuery:      fmt.Sprintf(selectPgStatActivity, "", excludeCurrentUserClause),
		},
		{
			name:               "ExcludeCurrentUser disabled",
			excludeCurrentUser: false,
			expectedQuery:      fmt.Sprintf(selectPgStatActivity, "", ""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			logBuffer := syncbuffer.Buffer{}
			lokiClient := loki.NewCollectingHandler()
			defer lokiClient.Stop()

			sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
				DB:                 db,
				CollectInterval:    time.Millisecond,
				EntryHandler:       lokiClient,
				Logger:             log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
				ExcludeCurrentUser: tc.excludeCurrentUser,
			})
			require.NoError(t, err)
			require.NotNil(t, sampleCollector)

			// First scrape: expect query with correct SQL format
			mock.ExpectQuery(tc.expectedQuery).RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows(columns).AddRow(
					now, "testdb", 100, sql.NullInt64{},
					"testuser", "testapp", "127.0.0.1", 5432,
					"client backend", backendStartTime, sql.NullInt32{Int32: 500, Valid: true}, sql.NullInt32{Int32: 400, Valid: true},
					xactStartTime, "active", stateChangeTime, sql.NullString{},
					sql.NullString{}, nil, queryStartTime, sql.NullInt64{Int64: 123, Valid: true},
				))

			// Second scrape: empty to trigger finalization
			mock.ExpectQuery(tc.expectedQuery).RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows(columns))

			err = sampleCollector.Start(t.Context())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return len(lokiClient.Received()) == 1
			}, 5*time.Second, 100*time.Millisecond)

			entries := lokiClient.Received()
			require.Len(t, entries, 1)

			sampleCollector.Stop()
			require.Eventually(t, func() bool {
				return sampleCollector.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

			require.Eventually(t, func() bool {
				return mock.ExpectationsWereMet() == nil
			}, 5*time.Second, 100*time.Millisecond)
		})
	}
}

func TestComputeBurstWindow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		ci       time.Duration
		observed time.Duration
		wantS    time.Duration
		wantW    time.Duration
	}{
		{
			name:     "CI=60s, observed=0 -> s=300ms, W=min(29.9s, 6s)=6s",
			ci:       60 * time.Second,
			observed: 0,
			wantS:    300 * time.Millisecond,
			wantW:    6 * time.Second,
		},
		{
			name:     "CI=3s, observed=0 -> s=100ms, W=min(1.4s, 2s)=1.4s",
			ci:       3 * time.Second,
			observed: 0,
			wantS:    100 * time.Millisecond,
			wantW:    1400 * time.Millisecond,
		},
		{
			name:     "CI=300ms, observed=0 -> s=50ms (clamped), W=min(50ms, 1s)=50ms",
			ci:       300 * time.Millisecond,
			observed: 0,
			wantS:    50 * time.Millisecond,
			wantW:    50 * time.Millisecond,
		},
		{
			name:     "CI=9s, observed=450ms -> s=450ms, W=min(4.4s, 9s)=4.4s",
			ci:       9 * time.Second,
			observed: 450 * time.Millisecond,
			wantS:    450 * time.Millisecond,
			wantW:    4400 * time.Millisecond,
		},
		{
			name:     "CI=100ms very small -> s=50ms (clamped), W=0",
			ci:       100 * time.Millisecond,
			observed: 0,
			wantS:    50 * time.Millisecond,
			wantW:    0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, w := computeBurstWindow(tc.ci, tc.observed)
			require.Equal(t, tc.wantS, s)
			require.Equal(t, tc.wantW, w)
		})
	}
}

func TestQuerySamples_TestBurstWindow(t *testing.T) {
	t.Parallel()

	collectInterval := 10 * time.Millisecond
	observedLatency := 100 * time.Millisecond
	burstInterval, burstWindow := computeBurstWindow(collectInterval, observedLatency)
	require.Equal(t, 100*time.Millisecond, burstInterval)
	require.Equal(t, 0*time.Millisecond, burstWindow)

	t.Run("ends_burst_window_when_inactive", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		// Use CI = 500ms so burst interval is ~50ms, then verify finalizations spacing reflects CI when inactive
		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       500 * time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)

		now := time.Now()
		backendStartTime := now.Add(-1 * time.Hour)
		columns := []string{
			"now", "datname", "pid", "leader_pid",
			"usename", "application_name", "client_addr", "client_port",
			"backend_type", "backend_start", "backend_xid", "backend_xmin",
			"xact_start", "state", "state_change", "wait_event_type",
			"wait_event", "query_start", "query_id",
			"query",
		}

		// Active → Active → Empty (emit #1) → Empty (wait CI) → Active → Empty (emit #2)
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(20 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns).AddRow(
			now, "testdb", 7000, sql.NullInt64{}, "testuser", "testapp", "127.0.0.1", 5432, "client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{}, now.Add(-2*time.Minute), "active", now, sql.NullString{}, sql.NullString{}, now, sql.NullInt64{Int64: 7001, Valid: true}, "SELECT 1",
		))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(20 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns).AddRow(
			now, "testdb", 7000, sql.NullInt64{}, "testuser", "testapp", "127.0.0.1", 5432, "client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{}, now.Add(-2*time.Minute), "active", now, sql.NullString{}, sql.NullString{}, now, sql.NullInt64{Int64: 7001, Valid: true}, "SELECT 1",
		))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(20 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(20 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(10 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns).AddRow(
			now, "testdb", 7000, sql.NullInt64{}, "testuser", "testapp", "127.0.0.1", 5432, "client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{}, now.Add(-2*time.Minute), "active", now, sql.NullString{}, sql.NullString{}, now, sql.NullInt64{Int64: 7001, Valid: true}, "SELECT 1",
		))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(10 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		var t1, t2 time.Time
		require.Eventually(t, func() bool {
			if len(lokiClient.Received()) >= 1 {
				t1 = time.Now()
				return true
			}
			return false
		}, 3*time.Second, 20*time.Millisecond)
		require.Eventually(t, func() bool {
			if len(lokiClient.Received()) >= 2 {
				t2 = time.Now()
				return true
			}
			return false
		}, 3*time.Second, 20*time.Millisecond)

		delta := t2.Sub(t1)
		require.GreaterOrEqual(t, delta, 900*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// 2) When observed latency > CI, no burst; next interval equals observed
	t.Run("respects_delay_greater_than_collect_interval", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       100 * time.Millisecond,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)

		now := time.Now()
		backendStartTime := now.Add(-1 * time.Hour)
		columns := []string{
			"now", "datname", "pid", "leader_pid",
			"usename", "application_name", "client_addr", "client_port",
			"backend_type", "backend_start", "backend_xid", "backend_xmin",
			"xact_start", "state", "state_change", "wait_event_type",
			"wait_event", "query_start", "query_id",
			"query",
		}

		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(250 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns).AddRow(
			now, "testdb", 9100, sql.NullInt64{}, "testuser", "testapp", "127.0.0.1", 5432, "client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{}, now.Add(-2*time.Minute), "active", now, sql.NullString{}, sql.NullString{}, now, sql.NullInt64{Int64: 5001, Valid: true}, "SELECT 1",
		))
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(10 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		start := time.Now()
		require.Eventually(t, func() bool { return len(lokiClient.Received()) >= 1 }, 2*time.Second, 20*time.Millisecond)
		elapsed := time.Since(start)
		require.GreaterOrEqual(t, elapsed, 500*time.Millisecond)
		require.Less(t, elapsed, 600*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// 3) Multiple polls occur within burst window
	t.Run("multiple_polls_within_window", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		logBuffer := syncbuffer.Buffer{}
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()

		sampleCollector, err := NewQuerySamples(QuerySamplesArguments{
			DB:                    db,
			CollectInterval:       3 * time.Second,
			EntryHandler:          lokiClient,
			Logger:                log.NewLogfmtLogger(log.NewSyncWriter(&logBuffer)),
			DisableQueryRedaction: true,
		})
		require.NoError(t, err)

		now := time.Now()
		backendStartTime := now.Add(-1 * time.Hour)
		columns := []string{
			"now", "datname", "pid", "leader_pid",
			"usename", "application_name", "client_addr", "client_port",
			"backend_type", "backend_start", "backend_xid", "backend_xmin",
			"xact_start", "state", "state_change", "wait_event_type",
			"wait_event", "query_start", "query_id",
			"query",
		}

		for i := 0; i < 7; i++ {
			mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(5 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns).AddRow(
				now, "testdb", 8100, sql.NullInt64{}, "testuser", "testapp", "127.0.0.1", 5432, "client backend", backendStartTime, sql.NullInt32{}, sql.NullInt32{}, now.Add(-2*time.Minute), "active", now, sql.NullString{}, sql.NullString{}, now, sql.NullInt64{Int64: 6001, Valid: true}, "SELECT 1",
			))
		}
		mock.ExpectQuery(fmt.Sprintf(selectPgStatActivity, queryTextClause, "")).WillDelayFor(5 * time.Millisecond).RowsWillBeClosed().WillReturnRows(sqlmock.NewRows(columns))

		require.NoError(t, sampleCollector.Start(t.Context()))

		start := time.Now()
		require.Eventually(t, func() bool { return len(lokiClient.Received()) >= 1 }, 3*time.Second, 20*time.Millisecond)
		elapsed := time.Since(start)
		require.GreaterOrEqual(t, elapsed, 700*time.Millisecond)
		require.Less(t, elapsed, 1000*time.Millisecond)

		sampleCollector.Stop()
		require.Eventually(t, func() bool { return sampleCollector.Stopped() }, 5*time.Second, 100*time.Millisecond)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
