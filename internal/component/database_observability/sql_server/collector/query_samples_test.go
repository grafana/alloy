package collector

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/util"
)

var querySampleColumns = []string{
	"now",
	"database_name",
	"session_id",
	"request_id",
	"start_time",
	"login_name",
	"original_login_name",
	"host_name",
	"program_name",
	"client_net_address",
	"client_tcp_port",
	"query_hash",
	"cpu_time",
	"total_elapsed_time",
	"reads",
	"writes",
	"logical_reads",
	"row_count",
	"statement_text",
	"exec_context_id",
	"wait_type",
	"wait_duration_ms",
	"blocking_session_id",
	"resource_description",
}

var (
	querySampleNow   = time.Date(2026, 9, 9, 13, 0, 10, 0, time.UTC)
	querySampleStart = querySampleNow.Add(-10 * time.Second)
)

func querySampleValues(overrides ...func([]driver.Value)) []driver.Value {
	values := []driver.Value{
		querySampleNow,
		testDatabase,
		int64(51),
		int64(0),
		querySampleStart,
		"app_user",
		"app_login",
		"app-host",
		"orders-api",
		"10.0.0.2",
		int64(54321),
		queryHashBytes,
		int64(90_000),
		int64(10_000),
		int64(7),
		int64(3),
		int64(42),
		int64(5),
		"SELECT * FROM orders /*traceparent='00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01'*/",
		int64(2),
		"LCK_M_S",
		int64(250),
		int64(77),
		"keylock hobtid=42 dbid=5",
	}
	for _, override := range overrides {
		override(values)
	}
	return values
}

func querySampleRows(rows ...[]driver.Value) *sqlmock.Rows {
	result := sqlmock.NewRows(querySampleColumns)
	for _, row := range rows {
		result.AddRow(row...)
	}
	return result
}

func newQuerySamplesForTest(t *testing.T, db *sql.DB, handler loki.EntryHandler, tracker QueryTracker, opts ...func(*QuerySamplesArguments)) *QuerySamples {
	t.Helper()
	args := QuerySamplesArguments{
		DB:              db,
		CollectInterval: time.Minute,
		Tracker:         tracker,
		EntryHandler:    handler,
		Logger:          util.TestAlloyLogger(t).Slog(),
	}
	for _, opt := range opts {
		opt(&args)
	}
	collector, err := NewQuerySamples(args)
	require.NoError(t, err)
	return collector
}

func expectQuerySamples(t *testing.T, mock sqlmock.Sqlmock, hashes, users []string, rows *sqlmock.Rows) {
	t.Helper()
	query, args, err := buildQuerySamplesStatement(hashes, users)
	require.NoError(t, err)
	mockSelectQueryStoreState(mock, "READ_WRITE")
	mock.ExpectQuery(query).
		WithArgs(namedArgs(args)...).
		RowsWillBeClosed().
		WillReturnRows(rows)
}

func TestBuildQuerySamplesStatement(t *testing.T) {
	query, args, err := buildQuerySamplesStatement(
		[]string{"ffeeddccbbaa9988", testHash, testHash},
		[]string{"admin", "monitor"},
	)
	require.NoError(t, err)
	require.Contains(t, query, `r.query_hash IN (@h0, @h1)`)
	require.Contains(t, query, `s.original_login_name NOT IN (@u0, @u1)`)
	require.Equal(t, []driver.Value{
		sql.Named("h0", queryHashBytes),
		sql.Named("h1", []byte{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88}),
		sql.Named("u0", "admin"),
		sql.Named("u1", "monitor"),
	}, namedArgs(args))

	query, args, err = buildQuerySamplesStatement([]string{testHash}, nil)
	require.NoError(t, err)
	require.NotContains(t, query, "original_login_name NOT IN")
	require.Len(t, args, 1)

	_, _, err = buildQuerySamplesStatement(nil, nil)
	require.ErrorContains(t, err, "without query hashes")
	_, _, err = buildQuerySamplesStatement([]string{"not-hex"}, nil)
	require.ErrorContains(t, err, "failed to decode")
	_, _, err = buildQuerySamplesStatement([]string{"0011"}, nil)
	require.ErrorContains(t, err, "expected 8 bytes")
}

func TestNewQuerySamplesNormalizesExcludedUsers(t *testing.T) {
	collector, err := NewQuerySamples(QuerySamplesArguments{
		Logger:       util.TestAlloyLogger(t).Slog(),
		ExcludeUsers: []string{"monitor", "admin", "monitor"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"admin", "monitor"}, collector.excludeUsers)
}

func TestQuerySamples_NoTrackerIsNoOp(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	collector := newQuerySamplesForTest(t, db, handler, nil)
	require.NoError(t, collector.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, handler.Received())
}

func TestQuerySamples_SkipsUnavailableQueryStore(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	collector := newQuerySamplesForTest(t, db, handler, trackerFor(testHash))
	mockSelectQueryStoreState(mock, "OFF")

	require.NoError(t, collector.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, handler.Received())
}

func TestQuerySamples_SkipsExcludedDatabase(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	collector := newQuerySamplesForTest(t, db, handler, trackerFor(testHash), func(args *QuerySamplesArguments) {
		args.ExcludeDatabases = []string{"BOOKS_STORE"}
	})
	mockSelectQueryStoreState(mock, "READ_WRITE")

	require.NoError(t, collector.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, handler.Received())
}

func TestQuerySamples_CollectAndFinalize(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	users := []string{"alloy_monitor"}
	collector := newQuerySamplesForTest(t, db, handler, trackerFor(testHash), func(args *QuerySamplesArguments) {
		args.ExcludeUsers = users
	})

	expectQuerySamples(t, mock, []string{testHash}, users, querySampleRows(querySampleValues()))
	require.NoError(t, collector.collect(context.Background()))
	require.Len(t, collector.samples, 1)
	require.Empty(t, handler.Received())

	expectQuerySamples(t, mock, []string{testHash}, users, querySampleRows())
	require.NoError(t, collector.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Eventually(t, func() bool { return len(handler.Received()) == 2 }, 5*time.Second, 20*time.Millisecond)
	entries := handler.Received()
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, entries[0].Labels)
	require.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT_V2}, entries[1].Labels)
	require.True(t, entries[0].Timestamp.Equal(querySampleStart))
	require.Equal(t, entries[0].Timestamp, entries[1].Timestamp)

	require.Contains(t, entries[0].Line, `database="books_store"`)
	require.Contains(t, entries[0].Line, `query_hash="0011223344556677"`)
	require.Contains(t, entries[0].Line, `client_address="10.0.0.2"`)
	require.Contains(t, entries[0].Line, `client_port="54321"`)
	require.Contains(t, entries[0].Line, `session_id="51" request_id="0"`)
	require.Contains(t, entries[0].Line, `cpu_time="90000ms"`)
	require.Contains(t, entries[0].Line, `elapsed_time="10s" elapsed_time_ms="10000"`)
	require.Contains(t, entries[0].Line, `traceparent="00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01"`)
	require.NotContains(t, entries[0].Line, ` query=`)

	require.Contains(t, entries[1].Line, `wait_event_type="Lock Wait"`)
	require.Contains(t, entries[1].Line, `wait_event_name="LCK_M_S"`)
	require.Contains(t, entries[1].Line, `exec_context_id="2"`)
	require.Contains(t, entries[1].Line, `blocking_session_id="77"`)
	require.Contains(t, entries[1].Line, `wait_time="250ms"`)
}

func TestQuerySamples_OmitsClientPortWhenUnavailable(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector, err := NewQuerySamples(QuerySamplesArguments{
		EntryHandler: handler,
		Logger:       util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	registered := map[string]struct{}{testHash: {}}
	row := querySampleRow{
		Now:           querySampleNow,
		DatabaseName:  testDatabase,
		SessionID:     51,
		RequestID:     0,
		StartTime:     querySampleStart,
		QueryHash:     testHash,
		ClientAddress: sql.NullString{String: "10.0.0.2", Valid: true},
	}

	collector.applySnapshot([]querySampleRow{row}, registered)
	collector.applySnapshot(nil, registered)
	require.Eventually(t, func() bool { return len(handler.Received()) == 1 }, 5*time.Second, 20*time.Millisecond)
	require.Contains(t, handler.Received()[0].Line, `client_address="10.0.0.2"`)
	require.NotContains(t, handler.Received()[0].Line, `client_port=`)

	row.ClientPort = sql.NullInt64{Valid: true}
	collector.applySnapshot([]querySampleRow{row}, registered)
	collector.applySnapshot(nil, registered)
	require.Eventually(t, func() bool { return len(handler.Received()) == 2 }, 5*time.Second, 20*time.Millisecond)
	require.Contains(t, handler.Received()[1].Line, `client_port="0"`)
}

func TestQuerySamples_OmitsBlockingSessionIDWhenUnavailable(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector, err := NewQuerySamples(QuerySamplesArguments{
		EntryHandler: handler,
		Logger:       util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	registered := map[string]struct{}{testHash: {}}
	row := querySampleRow{
		Now:            querySampleNow,
		DatabaseName:   testDatabase,
		SessionID:      51,
		RequestID:      0,
		StartTime:      querySampleStart,
		QueryHash:      testHash,
		ExecContextID:  sql.NullInt64{Int64: 2, Valid: true},
		WaitType:       sql.NullString{String: "LCK_M_S", Valid: true},
		WaitDurationMs: sql.NullInt64{Int64: 100, Valid: true},
	}

	collector.applySnapshot([]querySampleRow{row}, registered)
	collector.applySnapshot(nil, registered)
	require.Eventually(t, func() bool { return len(handler.Received()) == 2 }, 5*time.Second, 20*time.Millisecond)
	require.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT_V2}, handler.Received()[1].Labels)
	require.NotContains(t, handler.Received()[1].Line, `blocking_session_id=`)

	row.BlockingSessionID = sql.NullInt64{Valid: true}
	collector.applySnapshot([]querySampleRow{row}, registered)
	collector.applySnapshot(nil, registered)
	require.Eventually(t, func() bool { return len(handler.Received()) == 4 }, 5*time.Second, 20*time.Millisecond)
	require.Contains(t, handler.Received()[3].Line, `blocking_session_id="0"`)
}

func TestQuerySamples_RawQueryIsOptInAndQuoted(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector, err := NewQuerySamples(QuerySamplesArguments{
		EntryHandler:          handler,
		Logger:                util.TestAlloyLogger(t).Slog(),
		DisableQueryRedaction: true,
	})
	require.NoError(t, err)

	row := querySampleRow{
		Now:           querySampleNow,
		DatabaseName:  testDatabase,
		SessionID:     51,
		RequestID:     0,
		StartTime:     querySampleStart,
		QueryHash:     testHash,
		StatementText: sql.NullString{String: "SELECT \"secret\"\nFROM t", Valid: true},
	}
	collector.applySnapshot([]querySampleRow{row}, map[string]struct{}{testHash: {}})
	collector.applySnapshot(nil, map[string]struct{}{testHash: {}})

	require.Eventually(t, func() bool { return len(handler.Received()) == 1 }, 5*time.Second, 20*time.Millisecond)
	require.Contains(t, handler.Received()[0].Line, `query="SELECT \"secret\"\nFROM t"`)
}

func TestQuerySamples_TaskWaitTracking(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector, err := NewQuerySamples(QuerySamplesArguments{
		EntryHandler: handler,
		Logger:       util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	base := querySampleRow{
		Now:           querySampleNow,
		DatabaseName:  testDatabase,
		SessionID:     51,
		RequestID:     0,
		StartTime:     querySampleStart,
		QueryHash:     testHash,
		ExecContextID: sql.NullInt64{Int64: 2, Valid: true},
		WaitType:      sql.NullString{String: "LCK_M_S", Valid: true},
		WaitDurationMs: sql.NullInt64{
			Int64: 100,
			Valid: true,
		},
		Resource: sql.NullString{String: "keylock", Valid: true},
	}
	registered := map[string]struct{}{testHash: {}}
	collector.applySnapshot([]querySampleRow{base}, registered)

	extended := base
	extended.Now = extended.Now.Add(time.Second)
	extended.WaitDurationMs.Int64 = 250
	collector.applySnapshot([]querySampleRow{extended}, registered)

	reset := extended
	reset.Now = reset.Now.Add(time.Second)
	reset.WaitDurationMs.Int64 = 50
	collector.applySnapshot([]querySampleRow{reset}, registered)

	notWaiting := reset
	notWaiting.Now = notWaiting.Now.Add(time.Second)
	notWaiting.ExecContextID = sql.NullInt64{}
	notWaiting.WaitType = sql.NullString{}
	notWaiting.WaitDurationMs = sql.NullInt64{}
	collector.applySnapshot([]querySampleRow{notWaiting}, registered)

	waitAgain := reset
	waitAgain.Now = waitAgain.Now.Add(2 * time.Second)
	waitAgain.WaitDurationMs.Int64 = 20
	collector.applySnapshot([]querySampleRow{waitAgain}, registered)
	collector.applySnapshot(nil, registered)

	require.Eventually(t, func() bool { return len(handler.Received()) == 4 }, 5*time.Second, 20*time.Millisecond)
	entries := handler.Received()
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, entries[0].Labels)
	for _, entry := range entries[1:] {
		require.Equal(t, model.LabelSet{"op": database_observability.OP_WAIT_EVENT_V2}, entry.Labels)
	}
	require.Contains(t, entries[1].Line, `wait_time="250ms"`)
	require.Contains(t, entries[2].Line, `wait_time="50ms"`)
	require.Contains(t, entries[3].Line, `wait_time="20ms"`)
}

func TestQuerySamples_WaitOccurrencesAreCapped(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector, err := NewQuerySamples(QuerySamplesArguments{
		EntryHandler: handler,
		Logger:       util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	registered := map[string]struct{}{testHash: {}}
	base := querySampleRow{
		Now:            querySampleNow,
		DatabaseName:   testDatabase,
		SessionID:      51,
		RequestID:      0,
		StartTime:      querySampleStart,
		QueryHash:      testHash,
		ExecContextID:  sql.NullInt64{Int64: 2, Valid: true},
		WaitType:       sql.NullString{String: "LCK_M_S", Valid: true},
		WaitDurationMs: sql.NullInt64{Int64: 100, Valid: true},
	}

	const extra = 5
	for i := 0; i < maxWaitOccurrencesPerRequest+extra; i++ {
		row := base
		row.Now = querySampleNow.Add(time.Duration(i) * time.Second)
		// A unique resource each poll changes the wait identity, forcing a new
		// occurrence attempt on every snapshot.
		row.Resource = sql.NullString{String: fmt.Sprintf("keylock-%d", i), Valid: true}
		collector.applySnapshot([]querySampleRow{row}, registered)
	}

	key := newQuerySampleKey(base)
	state, ok := collector.samples[key]
	require.True(t, ok, "request should still be tracked")
	require.Len(t, state.waits.occurrences, maxWaitOccurrencesPerRequest, "occurrences are capped")
	require.Equal(t, extra, state.waits.dropped, "occurrence attempts beyond the cap are counted as dropped")
	require.True(t, state.capWarned, "the cap crossing is recorded once")

	// Tearing down the request flushes at most one wait event per retained
	// occurrence (plus the query sample), so the emission burst is bounded by the cap.
	collector.applySnapshot(nil, registered)
	require.Eventually(t, func() bool {
		return len(handler.Received()) == maxWaitOccurrencesPerRequest+1
	}, 5*time.Second, 20*time.Millisecond)
}

func TestQuerySamples_ParallelWaitsProduceOneSample(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector, err := NewQuerySamples(QuerySamplesArguments{
		EntryHandler: handler,
		Logger:       util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	first := querySampleRow{
		Now:           querySampleNow,
		DatabaseName:  testDatabase,
		SessionID:     51,
		RequestID:     0,
		StartTime:     querySampleStart,
		QueryHash:     testHash,
		ExecContextID: sql.NullInt64{Int64: 2, Valid: true},
		WaitType:      sql.NullString{String: "PAGEIOLATCH_SH", Valid: true},
		WaitDurationMs: sql.NullInt64{
			Int64: 20,
			Valid: true,
		},
	}
	second := first
	second.ExecContextID.Int64 = 3
	second.WaitType.String = "CXPACKET"
	second.WaitDurationMs.Int64 = 30

	collector.applySnapshot([]querySampleRow{first, second}, map[string]struct{}{testHash: {}})
	collector.applySnapshot(nil, map[string]struct{}{testHash: {}})

	require.Eventually(t, func() bool { return len(handler.Received()) == 3 }, 5*time.Second, 20*time.Millisecond)
	entries := handler.Received()
	require.Equal(t, model.LabelSet{"op": database_observability.OP_QUERY_SAMPLE}, entries[0].Labels)
	require.Contains(t, entries[1].Line, `wait_event_type="IO Wait"`)
	require.Contains(t, entries[2].Line, `wait_event_type="Engine Wait"`)
}

func TestQuerySamples_RegistryControlsAdmissionAndPinsExistingRequests(t *testing.T) {
	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector, err := NewQuerySamples(QuerySamplesArguments{
		EntryHandler: handler,
		Logger:       util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	admitted := querySampleRow{
		Now:          querySampleNow,
		DatabaseName: testDatabase,
		SessionID:    51,
		RequestID:    0,
		StartTime:    querySampleStart,
		QueryHash:    testHash,
	}
	collector.applySnapshot([]querySampleRow{admitted}, map[string]struct{}{testHash: {}})

	updated := admitted
	updated.Now = updated.Now.Add(time.Second)
	newRequest := updated
	newRequest.SessionID = 52
	collector.applySnapshot([]querySampleRow{updated, newRequest}, map[string]struct{}{})

	require.Len(t, collector.samples, 1, "the admitted request remains pinned while a new request is rejected")
	collector.applySnapshot(nil, map[string]struct{}{})
	require.Eventually(t, func() bool { return len(handler.Received()) == 1 }, 5*time.Second, 20*time.Millisecond)
}

func TestQuerySamples_CollectKeepsAdmittedHashPinned(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	tracker := fakeTracker{hashes: map[string][]string{testDatabase: {testHash}}}
	collector := newQuerySamplesForTest(t, db, handler, tracker)

	expectQuerySamples(t, mock, []string{testHash}, nil, querySampleRows(querySampleValues()))
	require.NoError(t, collector.collect(context.Background()))
	require.Len(t, collector.samples, 1)

	tracker.hashes[testDatabase] = nil
	expectQuerySamples(t, mock, []string{testHash}, nil, querySampleRows(querySampleValues()))
	require.NoError(t, collector.collect(context.Background()))
	require.Len(t, collector.samples, 1)

	expectQuerySamples(t, mock, []string{testHash}, nil, querySampleRows())
	require.NoError(t, collector.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Eventually(t, func() bool { return len(handler.Received()) == 2 }, 5*time.Second, 20*time.Millisecond)
}

func TestQuerySamples_ConcurrentRequestsAndRequestIDReuse(t *testing.T) {
	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector, err := NewQuerySamples(QuerySamplesArguments{
		EntryHandler: handler,
		Logger:       util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	first := querySampleRow{
		Now:          querySampleNow,
		DatabaseName: testDatabase,
		SessionID:    51,
		RequestID:    0,
		StartTime:    querySampleStart,
		QueryHash:    testHash,
	}
	second := first
	second.SessionID = 52
	reused := first
	reused.StartTime = first.StartTime.Add(time.Second)

	registered := map[string]struct{}{testHash: {}}
	collector.applySnapshot([]querySampleRow{first, second, reused}, registered)
	require.Len(t, collector.samples, 3)
	collector.applySnapshot(nil, registered)
	require.Eventually(t, func() bool { return len(handler.Received()) == 3 }, 5*time.Second, 20*time.Millisecond)
}

func TestQuerySamples_QueryAndRowErrorsPreserveState(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector := newQuerySamplesForTest(t, db, handler, trackerFor(testHash))

	collector.applySnapshot([]querySampleRow{{
		Now:          querySampleNow,
		DatabaseName: testDatabase,
		SessionID:    51,
		RequestID:    0,
		StartTime:    querySampleStart,
		QueryHash:    testHash,
	}}, map[string]struct{}{testHash: {}})

	query, args, err := buildQuerySamplesStatement([]string{testHash}, nil)
	require.NoError(t, err)
	mockSelectQueryStoreState(mock, "READ_WRITE")
	mock.ExpectQuery(query).WithArgs(namedArgs(args)...).WillReturnError(errors.New("permission denied"))
	require.ErrorContains(t, collector.collect(context.Background()), "permission denied")
	require.Len(t, collector.samples, 1)
	require.Empty(t, handler.Received())

	rows := querySampleRows(querySampleValues(), querySampleValues(func(values []driver.Value) {
		values[2] = int64(52)
	})).RowError(1, errors.New("row failed"))
	mockSelectQueryStoreState(mock, "READ_WRITE")
	mock.ExpectQuery(query).
		WithArgs(namedArgs(args)...).
		RowsWillBeClosed().
		WillReturnRows(rows)
	require.ErrorContains(t, collector.collect(context.Background()), "row failed")
	require.Len(t, collector.samples, 1)
	require.Empty(t, handler.Received())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuerySamples_StartStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	handler := loki.NewCollectingHandler()
	defer handler.Stop()
	collector := newQuerySamplesForTest(t, db, handler, nil)

	require.NoError(t, collector.Start(t.Context()))
	require.Eventually(t, func() bool { return !collector.Stopped() }, 5*time.Second, 20*time.Millisecond)
	collector.Stop()
	require.True(t, collector.Stopped())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClassifySQLServerWaitEventType(t *testing.T) {
	testCases := []struct {
		waitType string
		expected string
	}{
		{waitType: "HADR_FILESTREAM_IOMGR_IOCOMPLETION", expected: "Replication Wait"},
		{waitType: "DBMIRROR_SEND", expected: "Replication Wait"},
		{waitType: "REPL_SCHEMA_ACCESS", expected: "Replication Wait"},
		{waitType: "FCB_REPLICA_READ", expected: "Replication Wait"},
		{waitType: "LCK_M_S", expected: "Lock Wait"},
		{waitType: "PAGEIOLATCH_SH", expected: "IO Wait"},
		{waitType: "WRITELOG", expected: "IO Wait"},
		{waitType: "ASYNC_NETWORK_IO", expected: "Network Wait"},
		{waitType: "PAGELATCH_EX", expected: "Engine Wait"},
		{waitType: "CXPACKET", expected: "Engine Wait"},
		{waitType: "RESOURCE_SEMAPHORE_QUERY_COMPILE", expected: "Engine Wait"},
		{waitType: "WAITFOR", expected: "Other Wait"},
	}

	for _, tc := range testCases {
		t.Run(tc.waitType, func(t *testing.T) {
			require.Equal(t, tc.expected, classifySQLServerWaitEventType(tc.waitType))
		})
	}
}
