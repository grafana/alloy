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

func indexStatsExpected(idxScan int) string {
	return fmt.Sprintf(`
	# HELP database_observability_pg_index_stats_idx_scan_total Number of index scans initiated on this index
	# TYPE database_observability_pg_index_stats_idx_scan_total counter
	database_observability_pg_index_stats_idx_scan_total{datname="books_store",indexrelname="gen_adjectives_pkey",relname="gen_adjectives",schemaname="public"} %d
	# HELP database_observability_pg_index_stats_size_bytes Total disk space used by this index, in bytes, labeled with whether it backs the primary key or a unique constraint, or is partial
	# TYPE database_observability_pg_index_stats_size_bytes gauge
	database_observability_pg_index_stats_size_bytes{datname="books_store",indexrelname="gen_adjectives_pkey",is_partial="false",is_primary="true",is_unique="true",relname="gen_adjectives",schemaname="public"} 16384
`, idxScan)
}

func expectIndexStatsRun(mock sqlmock.Sqlmock, interval time.Duration, idxScan int) {
	expectDiscoverDatabases(mock, "books_store")
	mock.ExpectBegin()
	expectSetTimeouts(mock, interval)
	mock.ExpectQuery(selectIndexUsageStats).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"schemaname", "relname", "indexrelname", "idx_scan", "indisprimary", "indisunique", "is_partial", "index_size_bytes"}).
				AddRow("public", "gen_adjectives", "gen_adjectives_pkey", idxScan, true, true, false, 16384),
		)
	mock.ExpectCommit()
}

func newTestIndexStats(t *testing.T, interval time.Duration) (*IndexStats, sqlmock.Sqlmock, *prometheus.Registry) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	registry := prometheus.NewRegistry()
	c, err := NewIndexStats(IndexStatsArguments{
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

func requireIndexStatsEventually(t *testing.T, registry *prometheus.Registry, seqScan int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return testutil.CollectAndCompare(registry, strings.NewReader(indexStatsExpected(seqScan))) == nil
	}, 5*time.Second, 10*time.Millisecond)
}

func TestIndexStats(t *testing.T) {
	c, mock, registry := newTestIndexStats(t, time.Hour)
	expectIndexStatsRun(mock, time.Hour, 4242)

	require.NoError(t, c.Start(t.Context()))
	requireIndexStatsEventually(t, registry, 4242)

	// Stop waits for the collector goroutine, so the mock is safe to inspect.
	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexStats_ScrapeDoesNotQueryDatabase(t *testing.T) {
	c, mock, registry := newTestIndexStats(t, time.Hour)
	expectIndexStatsRun(mock, time.Hour, 4242)

	require.NoError(t, c.Start(t.Context()))
	requireIndexStatsEventually(t, registry, 4242)

	// Any further query would fail as an unexpected call to the mock.
	for range 5 {
		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(indexStatsExpected(4242))))
	}

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexStats_KeepsLastResultWhenQueryFails(t *testing.T) {
	c, mock, registry := newTestIndexStats(t, 50*time.Millisecond)
	expectIndexStatsRun(mock, 50*time.Millisecond, 4242)
	// The next run fails on the database query and rolls back.
	expectDiscoverDatabases(mock, "books_store")
	mock.ExpectBegin()
	expectSetTimeouts(mock, 50*time.Millisecond)
	mock.ExpectQuery(selectIndexUsageStats).WillReturnError(errors.New("boom"))
	mock.ExpectRollback()

	require.NoError(t, c.Start(t.Context()))
	requireIndexStatsEventually(t, registry, 4242)

	// The failing run happens in this window; the last result stays exposed.
	require.Never(t, func() bool {
		return testutil.CollectAndCompare(registry, strings.NewReader(indexStatsExpected(4242))) != nil
	}, 500*time.Millisecond, 10*time.Millisecond)

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexStats_DropsDatabasesThatDisappear(t *testing.T) {
	c, mock, registry := newTestIndexStats(t, 50*time.Millisecond)

	// First run sees two databases; the second database is served by the
	// connection factory, which in this test returns the same mock.
	expectDiscoverDatabases(mock, "books_store", "gone_soon")
	for _, name := range []string{"books_store", "gone_soon"} {
		mock.ExpectBegin()
		expectSetTimeouts(mock, 50*time.Millisecond)
		mock.ExpectQuery(selectIndexUsageStats).WillReturnRows(
			sqlmock.NewRows([]string{"schemaname", "relname", "indexrelname", "idx_scan", "indisprimary", "indisunique", "is_partial", "index_size_bytes"}).
				AddRow("public", "t_"+name, "t_"+name+"_pkey", 1, true, true, false, 8192))
		mock.ExpectCommit()
	}
	// Later runs only see books_store.
	for range 20 {
		expectIndexStatsRun(mock, 50*time.Millisecond, 4242)
	}

	require.NoError(t, c.Start(t.Context()))
	require.Eventually(t, func() bool {
		return testutil.CollectAndCount(registry, "database_observability_pg_index_stats_idx_scan_total") == 1
	}, 5*time.Second, 10*time.Millisecond)
	c.Stop()
}

func TestIndexStats_QueryIsBoundedByCollectInterval(t *testing.T) {
	c, mock, registry := newTestIndexStats(t, 100*time.Millisecond)
	expectIndexStatsRun(mock, 100*time.Millisecond, 4242)
	// A discovery slower than the collect interval must be cancelled, not waited for.
	mock.ExpectQuery(fmt.Sprintf(selectAllDatabases, exclusionClause)).WillDelayFor(time.Hour).
		WillReturnRows(sqlmock.NewRows([]string{"datname"}))
	// The run after it succeeds with new data, proving the collector moved on
	// by itself rather than being unblocked by Stop.
	expectIndexStatsRun(mock, 100*time.Millisecond, 5000)

	require.NoError(t, c.Start(t.Context()))
	requireIndexStatsEventually(t, registry, 5000)

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexStats_RequiresPositiveCollectInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		_, err := NewIndexStats(IndexStatsArguments{
			Registry:        prometheus.NewRegistry(),
			CollectInterval: interval,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.Error(t, err)
	}
}
