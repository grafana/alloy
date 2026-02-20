package collector

import (
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestStatStatementsRegistry_NoRateBeforeFirstDelta(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry, err := NewStatStatementsRegistry(StatStatementsRegistryArguments{
		DB:     db,
		Logger: log.NewNopLogger(),
	})
	require.NoError(t, err)

	expectedQuery := buildStatStatementsQuery(nil)

	// First refresh: provides the initial snapshot (no previous to diff against).
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(111), "testdb", int64(50), "SELECT 1", float64(100.0), float64(2.0)).
			AddRow(int64(222), "testdb", int64(100), "SELECT 2", float64(200.0), float64(2.0)),
	)

	require.NoError(t, registry.refresh(t.Context()))

	// No rates available after only one snapshot.
	_, ok := registry.GetExecutionRate(111, "testdb")
	require.False(t, ok, "expected no rate before first delta")
	_, ok = registry.GetExecutionRate(222, "testdb")
	require.False(t, ok, "expected no rate before first delta")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStatStatementsRegistry_RateComputedAfterTwoSnapshots(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry, err := NewStatStatementsRegistry(StatStatementsRegistryArguments{
		DB:     db,
		Logger: log.NewNopLogger(),
	})
	require.NoError(t, err)

	expectedQuery := buildStatStatementsQuery(nil)

	// First snapshot: 50 calls for qid 111, 100 calls for qid 222.
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(111), "testdb", int64(50), "SELECT 1", float64(100.0), float64(2.0)).
			AddRow(int64(222), "testdb", int64(100), "SELECT 2", float64(200.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	// Force prevTime to be exactly 1 minute ago so rate = delta/1.
	registry.mu.Lock()
	registry.prevTime = registry.prevTime.Add(-1 * time.Minute)
	registry.mu.Unlock()

	// Second snapshot: 110 calls for qid 111 (delta=60), 100 calls for qid 222 (delta=0).
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(111), "testdb", int64(110), "SELECT 1", float64(220.0), float64(2.0)).
			AddRow(int64(222), "testdb", int64(100), "SELECT 2", float64(200.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	rate, ok := registry.GetExecutionRate(111, "testdb")
	require.True(t, ok)
	require.InDelta(t, 60.0, rate, 1.0, "expected ~60/min for qid 111")

	rate, ok = registry.GetExecutionRate(222, "testdb")
	require.True(t, ok)
	require.InDelta(t, 0.0, rate, 0.1, "expected ~0/min for qid 222 (no new calls)")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStatStatementsRegistry_ResetDetected(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry, err := NewStatStatementsRegistry(StatStatementsRegistryArguments{
		DB:     db,
		Logger: log.NewNopLogger(),
	})
	require.NoError(t, err)

	expectedQuery := buildStatStatementsQuery(nil)

	// First snapshot: 500 calls.
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(333), "testdb", int64(500), "SELECT 3", float64(1000.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	registry.mu.Lock()
	registry.prevTime = registry.prevTime.Add(-1 * time.Minute)
	registry.mu.Unlock()

	// Second snapshot: calls dropped to 10 (stats reset happened).
	// Expected rate = 10 / 1 minute = 10/min (treats current calls as the delta).
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(333), "testdb", int64(10), "SELECT 3", float64(20.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	rate, ok := registry.GetExecutionRate(333, "testdb")
	require.True(t, ok)
	require.InDelta(t, 10.0, rate, 1.0, "expected ~10/min after reset")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStatStatementsRegistry_NewQueryidAppearsAfterFirstSnapshot(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry, err := NewStatStatementsRegistry(StatStatementsRegistryArguments{
		DB:     db,
		Logger: log.NewNopLogger(),
	})
	require.NoError(t, err)

	expectedQuery := buildStatStatementsQuery(nil)

	// First snapshot: only qid 100.
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(100), "testdb", int64(20), "SELECT 100", float64(40.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	registry.mu.Lock()
	registry.prevTime = registry.prevTime.Add(-1 * time.Minute)
	registry.mu.Unlock()

	// Second snapshot: qid 100 grows and new qid 200 appears.
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(100), "testdb", int64(40), "SELECT 100", float64(80.0), float64(2.0)).
			AddRow(int64(200), "testdb", int64(15), "SELECT 200", float64(30.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	// qid 100 has a delta.
	rate, ok := registry.GetExecutionRate(100, "testdb")
	require.True(t, ok)
	require.InDelta(t, 20.0, rate, 1.0)

	// qid 200 appeared for the first time — no rate yet (no prev snapshot).
	_, ok = registry.GetExecutionRate(200, "testdb")
	require.False(t, ok, "expected no rate for brand-new queryid")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStatStatementsRegistry_ExcludeDatabases(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry, err := NewStatStatementsRegistry(StatStatementsRegistryArguments{
		DB:               db,
		ExcludeDatabases: []string{"excluded_db"},
		Logger:           log.NewNopLogger(),
	})
	require.NoError(t, err)

	// The query should use the exclusion clause including "excluded_db".
	expectedQuery := buildStatStatementsQuery([]string{"excluded_db"})

	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(555), "included_db", int64(30), "SELECT 555", float64(60.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStatStatementsRegistry_DBError(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry, err := NewStatStatementsRegistry(StatStatementsRegistryArguments{
		DB:     db,
		Logger: log.NewNopLogger(),
	})
	require.NoError(t, err)

	expectedQuery := buildStatStatementsQuery(nil)
	mock.ExpectQuery(expectedQuery).WillReturnError(errMockQuerySamplesFailed)

	err = registry.refresh(t.Context())
	require.ErrorContains(t, err, "failed to query pg_stat_statements")

	// After an error, no rates should be available.
	_, ok := registry.GetExecutionRate(999, "testdb")
	require.False(t, ok)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStatStatementsRegistry_StartStop(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry, err := NewStatStatementsRegistry(StatStatementsRegistryArguments{
		DB:     db,
		Logger: log.NewNopLogger(),
	})
	require.NoError(t, err)

	require.NoError(t, registry.Start(t.Context()))
	require.False(t, registry.Stopped())

	registry.Stop()
	require.Eventually(t, func() bool {
		return registry.Stopped()
	}, 5*time.Second, 10*time.Millisecond)
}

func TestStatStatementsRegistry_SameQueryIDMultipleDatabases(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry, err := NewStatStatementsRegistry(StatStatementsRegistryArguments{
		DB:     db,
		Logger: log.NewNopLogger(),
	})
	require.NoError(t, err)

	expectedQuery := buildStatStatementsQuery(nil)

	// First snapshot: qid 100 runs in two databases.
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(100), "db_a", int64(10), "SELECT 1", float64(20.0), float64(2.0)).
			AddRow(int64(100), "db_b", int64(200), "SELECT 1", float64(400.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	registry.mu.Lock()
	registry.prevTime = registry.prevTime.Add(-1 * time.Minute)
	registry.mu.Unlock()

	// Second snapshot: db_a grew by 5, db_b grew by 50 — very different rates.
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(100), "db_a", int64(15), "SELECT 1", float64(30.0), float64(2.0)).
			AddRow(int64(100), "db_b", int64(250), "SELECT 1", float64(500.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	rateA, ok := registry.GetExecutionRate(100, "db_a")
	require.True(t, ok)
	require.InDelta(t, 5.0, rateA, 0.5, "expected ~5/min for qid 100 in db_a")

	rateB, ok := registry.GetExecutionRate(100, "db_b")
	require.True(t, ok)
	require.InDelta(t, 50.0, rateB, 1.0, "expected ~50/min for qid 100 in db_b")

	require.NotEqual(t, rateA, rateB, "rates for the same queryid in different databases must be independent")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStatStatementsRegistry_Snapshot(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry, err := NewStatStatementsRegistry(StatStatementsRegistryArguments{
		DB:     db,
		Logger: log.NewNopLogger(),
	})
	require.NoError(t, err)

	// Before any refresh the snapshot is empty.
	require.Empty(t, registry.Snapshot())

	expectedQuery := buildStatStatementsQuery(nil)
	mock.ExpectQuery(expectedQuery).WillReturnRows(
		sqlmock.NewRows([]string{"queryid", "datname", "calls", "query", "total_exec_time", "mean_exec_time"}).
			AddRow(int64(777), "mydb", int64(42), "SELECT 777", float64(84.0), float64(2.0)),
	)
	require.NoError(t, registry.refresh(t.Context()))

	snap := registry.Snapshot()
	require.Len(t, snap, 1)

	row, ok := snap[StatStatementsKey{QueryID: 777, DBName: "mydb"}]
	require.True(t, ok)
	require.Equal(t, int64(777), row.QueryID)
	require.Equal(t, "mydb", row.DBName)
	require.Equal(t, int64(42), row.Calls)
	require.Equal(t, "SELECT 777", row.Query)
	require.InDelta(t, 84.0, row.TotalExecMs, 0.01)
	require.InDelta(t, 2.0, row.MeanExecMs, 0.01)

	// Mutating the returned map must not affect the registry's internal snapshot.
	delete(snap, StatStatementsKey{QueryID: 777, DBName: "mydb"})
	require.Len(t, registry.Snapshot(), 1, "internal snapshot must be unaffected by external mutation")

	require.NoError(t, mock.ExpectationsWereMet())
}

// buildStatStatementsQuery returns the exact SQL string that the registry will
// execute for the given list of excluded databases, for use in sqlmock expectations.
func buildStatStatementsQuery(excludeDatabases []string) string {
	return fmt.Sprintf(selectStatStatements, buildExcludedDatabasesClause(excludeDatabases))
}
