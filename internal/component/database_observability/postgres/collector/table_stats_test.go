package collector

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

func tableStatsExpected(seqScan int) string {
	return fmt.Sprintf(`
	# HELP database_observability_pg_table_stats_idx_scan_total Number of index scans initiated on this table
	# TYPE database_observability_pg_table_stats_idx_scan_total counter
	database_observability_pg_table_stats_idx_scan_total{datname="books_store",relname="gen_adjectives",schemaname="public"} 0
	# HELP database_observability_pg_table_stats_row_count Estimated number of live rows in this table
	# TYPE database_observability_pg_table_stats_row_count gauge
	database_observability_pg_table_stats_row_count{datname="books_store",relname="gen_adjectives",schemaname="public"} 500
	# HELP database_observability_pg_table_stats_seq_scan_total Number of sequential scans initiated on this table
	# TYPE database_observability_pg_table_stats_seq_scan_total counter
	database_observability_pg_table_stats_seq_scan_total{datname="books_store",relname="gen_adjectives",schemaname="public"} %d
`, seqScan)
}

// expectSetTimeouts expects the server-side timeouts every query sets inside
// its transaction.
func expectSetTimeouts(mock sqlmock.Sqlmock, interval time.Duration) {
	mock.ExpectExec(fmt.Sprintf("SET LOCAL statement_timeout = %d; SET LOCAL lock_timeout = %d",
		interval.Milliseconds(), min(interval, maxLockTimeout).Milliseconds())).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func expectDiscoverDatabases(mock sqlmock.Sqlmock, databases ...string) {
	rows := sqlmock.NewRows([]string{"datname"})
	for _, d := range databases {
		rows.AddRow(d)
	}
	mock.ExpectQuery(fmt.Sprintf(selectAllDatabases, exclusionClause)).WithoutArgs().RowsWillBeClosed().WillReturnRows(rows)
}

func expectTableStatsRun(mock sqlmock.Sqlmock, interval time.Duration, seqScan int) {
	expectDiscoverDatabases(mock, "books_store")
	mock.ExpectBegin()
	expectSetTimeouts(mock, interval)
	mock.ExpectQuery(selectTableScanStats).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"schemaname", "relname", "seq_scan", "idx_scan", "n_live_tup"}).
				AddRow("public", "gen_adjectives", seqScan, 0, 500),
		)
	mock.ExpectCommit()
}

func newTestTableStats(t *testing.T, interval time.Duration) (*TableStats, sqlmock.Sqlmock, *prometheus.Registry) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	registry := prometheus.NewRegistry()
	c, err := NewTableStats(TableStatsArguments{
		DB:               db,
		DSN:              "postgres://user:pass@localhost:5432/books_store",
		ExcludeDatabases: nil,
		Registry:         registry,
		CollectInterval:  interval,
		Logger:           util.TestAlloyLogger(t).Slog(),
		dbConnectionFactory: func(dsn string) (*sql.DB, error) {
			return db, nil
		},
	})
	require.NoError(t, err)
	return c, mock, registry
}

func requireTableStatsEventually(t *testing.T, registry *prometheus.Registry, seqScan int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return testutil.CollectAndCompare(registry, strings.NewReader(tableStatsExpected(seqScan))) == nil
	}, 5*time.Second, 10*time.Millisecond)
}

func TestTableStats(t *testing.T) {
	c, mock, registry := newTestTableStats(t, time.Hour)
	expectTableStatsRun(mock, time.Hour, 37154)

	require.NoError(t, c.Start(t.Context()))
	requireTableStatsEventually(t, registry, 37154)

	// Stop waits for the collector goroutine, so the mock is safe to inspect.
	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTableStats_ScrapeDoesNotQueryDatabase(t *testing.T) {
	c, mock, registry := newTestTableStats(t, time.Hour)
	expectTableStatsRun(mock, time.Hour, 37154)

	require.NoError(t, c.Start(t.Context()))
	requireTableStatsEventually(t, registry, 37154)

	// Any further query would fail as an unexpected call to the mock.
	for range 5 {
		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(tableStatsExpected(37154))))
	}

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTableStats_KeepsLastResultWhenQueryFails(t *testing.T) {
	c, mock, registry := newTestTableStats(t, 50*time.Millisecond)
	expectTableStatsRun(mock, 50*time.Millisecond, 37154)
	// The next run fails on the database query and rolls back.
	expectDiscoverDatabases(mock, "books_store")
	mock.ExpectBegin()
	expectSetTimeouts(mock, 50*time.Millisecond)
	mock.ExpectQuery(selectTableScanStats).WillReturnError(errors.New("boom"))
	mock.ExpectRollback()

	require.NoError(t, c.Start(t.Context()))
	requireTableStatsEventually(t, registry, 37154)

	// The failing run happens in this window; the last result stays exposed.
	require.Never(t, func() bool {
		return testutil.CollectAndCompare(registry, strings.NewReader(tableStatsExpected(37154))) != nil
	}, 500*time.Millisecond, 10*time.Millisecond)

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTableStats_DropsDatabasesThatDisappear(t *testing.T) {
	c, mock, registry := newTestTableStats(t, 50*time.Millisecond)

	// First run sees two databases; the second database is served by the
	// connection factory, which in this test returns the same mock.
	expectDiscoverDatabases(mock, "books_store", "gone_soon")
	for _, name := range []string{"books_store", "gone_soon"} {
		mock.ExpectBegin()
		expectSetTimeouts(mock, 50*time.Millisecond)
		mock.ExpectQuery(selectTableScanStats).WillReturnRows(
			sqlmock.NewRows([]string{"schemaname", "relname", "seq_scan", "idx_scan", "n_live_tup"}).
				AddRow("public", "t_"+name, 1, 0, 10))
		mock.ExpectCommit()
	}
	// Later runs only see books_store.
	for range 20 {
		expectTableStatsRun(mock, 50*time.Millisecond, 37154)
	}

	require.NoError(t, c.Start(t.Context()))
	require.Eventually(t, func() bool {
		return testutil.CollectAndCount(registry, "database_observability_pg_table_stats_seq_scan_total") == 1
	}, 5*time.Second, 10*time.Millisecond)
	c.Stop()
}

func TestTableStats_QueryIsBoundedByCollectInterval(t *testing.T) {
	c, mock, registry := newTestTableStats(t, 100*time.Millisecond)
	expectTableStatsRun(mock, 100*time.Millisecond, 37154)
	// A discovery slower than the collect interval must be cancelled, not waited for.
	mock.ExpectQuery(fmt.Sprintf(selectAllDatabases, exclusionClause)).WillDelayFor(time.Hour).
		WillReturnRows(sqlmock.NewRows([]string{"datname"}))
	// The run after it succeeds with new data, proving the collector moved on
	// by itself rather than being unblocked by Stop.
	expectTableStatsRun(mock, 100*time.Millisecond, 40000)

	require.NoError(t, c.Start(t.Context()))
	requireTableStatsEventually(t, registry, 40000)

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTableStats_RequiresPositiveCollectInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		_, err := NewTableStats(TableStatsArguments{
			Registry:        prometheus.NewRegistry(),
			CollectInterval: interval,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.Error(t, err)
	}
}
