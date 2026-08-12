package collector

import (
	"context"
	"database/sql/driver"
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

type fakeTracker struct {
	hashes map[string][]string
}

func (f fakeTracker) GetQueryHashes(database string) []string {
	return f.hashes[database]
}

func trackerFor(hashes ...string) QueryTracker {
	return fakeTracker{hashes: map[string][]string{testDatabase: hashes}}
}

func mockQueryTextRows(rows ...[]driver.Value) *sqlmock.Rows {
	r := sqlmock.NewRows([]string{"query_hash", "query_sql_text"})
	for _, row := range rows {
		r.AddRow(row...)
	}
	return r
}

// namedArgs converts the collector's []any of sql.Named parameters into the
// []driver.Value that sqlmock's WithArgs expects.
func namedArgs(args []any) []driver.Value {
	out := make([]driver.Value, len(args))
	for i, a := range args {
		out[i] = a
	}
	return out
}

// expectQueryText sets up the two queries one collect cycle runs: the Query
// Store state preflight and the query text fetch filtered to hashes.
func expectQueryText(t *testing.T, mock sqlmock.Sqlmock, hashes []string, rows ...[]driver.Value) {
	t.Helper()

	query, args, err := buildQueryTextStatement(hashes)
	require.NoError(t, err)

	mockSelectQueryStoreState(mock, "READ_WRITE")
	mock.ExpectQuery(query).
		WithArgs(namedArgs(args)...).
		RowsWillBeClosed().
		WillReturnRows(mockQueryTextRows(rows...))
}

func TestQueryDetails_NoTrackerIsNoOp(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Minute,
		EntryHandler:    lokiClient,
		Tracker:         nil,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, lokiClient.Received())
}

func TestQueryDetails_EmptyTrackerEmitsNothing(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Minute,
		EntryHandler:    lokiClient,
		// Tracker is present but tracks nothing yet: no text query must run.
		Tracker: fakeTracker{hashes: map[string][]string{}},
		Logger:  util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	mockSelectQueryStoreState(mock, "READ_WRITE")

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, lokiClient.Received())
}

func TestQueryDetails_SkipsWhenQueryStoreUnavailable(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Minute,
		EntryHandler:    lokiClient,
		Tracker:         trackerFor(testHash),
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	// Query Store is OFF: no text query must run and nothing is emitted.
	mockSelectQueryStoreState(mock, "OFF")

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, lokiClient.Received())
}

func TestQueryDetails_NormalizesQueryText(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Minute,
		EntryHandler:    lokiClient,
		Tracker:         trackerFor(testHash),
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	queryText := "SELECT *\n\tFROM [dbo].[orders] o /* pick up the big ones */\n\tJOIN customers c ON c.id = o.customer_id\n\tWHERE o.total > 100"

	expectQueryText(t, mock, []string{testHash}, []driver.Value{queryHashBytes, queryText})

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) == 3
	}, 5*time.Second, 20*time.Millisecond)

	entries := lokiClient.Received()
	require.Equal(t, model.LabelSet{"op": OP_QUERY_ASSOCIATION}, entries[0].Labels)
	require.Equal(t, `level="info" database="books_store" query_hash="0011223344556677" querytext="SELECT * FROM [dbo].[orders] o JOIN customers c ON c.id = o.customer_id WHERE o.total > ?"`, entries[0].Line)
	require.NotContains(t, entries[0].Line, `\n`)
	require.NotContains(t, entries[0].Line, `\t`)

	require.Equal(t, model.LabelSet{"op": OP_QUERY_PARSED_TABLE_NAME}, entries[1].Labels)
	require.Equal(t, `level="info" database="books_store" query_hash="0011223344556677" table="dbo.orders"`, entries[1].Line)
	require.Equal(t, model.LabelSet{"op": OP_QUERY_PARSED_TABLE_NAME}, entries[2].Labels)
	require.Equal(t, `level="info" database="books_store" query_hash="0011223344556677" table="customers"`, entries[2].Line)
}

func TestQueryDetails_DedupesRepeatedHashWithinCycle(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Minute,
		EntryHandler:    lokiClient,
		Tracker:         trackerFor(testHash),
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	// The same query_hash maps to two query_text_id rows; only the first is emitted.
	expectQueryText(
		t, mock, []string{testHash},
		[]driver.Value{queryHashBytes, "SELECT * FROM foo WHERE id = 1"},
		[]driver.Value{queryHashBytes, "SELECT * FROM foo WHERE id = 2"},
	)

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) == 2
	}, 5*time.Second, 20*time.Millisecond)
	require.Len(t, lokiClient.Received(), 2)
}

func TestQueryDetails_ThrottlesWithinEmitInterval(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c, err := NewQueryDetails(QueryDetailsArguments{
		DB:              db,
		CollectInterval: time.Minute,
		EntryHandler:    lokiClient,
		Tracker:         trackerFor(testHash),
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	base := time.Now()
	c.now = func() time.Time { return base }
	key := queryMetricsKey{database: testDatabase, queryHash: testHash}

	poll := func() {
		t.Helper()
		expectQueryText(t, mock, []string{testHash},
			[]driver.Value{queryHashBytes, "SELECT * FROM foo WHERE id = 1"})
		require.NoError(t, c.collect(context.Background()))
	}

	// First cycle emits the association + parsed table (2 entries).
	poll()
	require.Equal(t, base, c.lastEmittedAt[key])

	// Second cycle within EmitInterval: the query still runs but nothing new is emitted.
	c.now = func() time.Time { return base.Add(database_observability.EmitInterval / 2) }
	poll()
	require.Equal(t, base, c.lastEmittedAt[key])

	// Once EmitInterval has elapsed, the query is emitted again (2 more entries).
	afterInterval := base.Add(database_observability.EmitInterval + time.Minute)
	c.now = func() time.Time { return afterInterval }
	poll()
	require.Equal(t, afterInterval, c.lastEmittedAt[key])

	require.NoError(t, mock.ExpectationsWereMet())
	lokiClient.Stop()
	require.Len(t, lokiClient.Received(), 4)
}

func TestBuildQueryTextStatement(t *testing.T) {
	t.Parallel()

	t.Run("hashes fill the IN clause with binary params", func(t *testing.T) {
		query, args, err := buildQueryTextStatement([]string{testHash, "aabbccddeeff0011"})
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf(selectQueryTextTemplate, "@h0, @h1"), query)
		require.Len(t, args, 2)
	})

	t.Run("invalid hex hash errors", func(t *testing.T) {
		_, _, err := buildQueryTextStatement([]string{"not_hex_string"})
		require.Error(t, err)
	})
}
