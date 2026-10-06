package collector

import (
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

func indexStatsExpected(idxFetch int) string {
	return fmt.Sprintf(`
	# HELP database_observability_mysql_index_stats_idx_fetch_total Count of index I/O wait events for fetch operations
	# TYPE database_observability_mysql_index_stats_idx_fetch_total counter
	database_observability_mysql_index_stats_idx_fetch_total{index="idx_books_title",schema="books_store",table="books"} %d
	# HELP database_observability_mysql_index_stats_size_bytes Total disk space used by this index, in bytes, labeled with whether it backs the primary key or a unique constraint
	# TYPE database_observability_mysql_index_stats_size_bytes gauge
	database_observability_mysql_index_stats_size_bytes{index="PRIMARY",is_primary="true",is_unique="true",schema="books_store",table="books"} 65536
	database_observability_mysql_index_stats_size_bytes{index="idx_books_title",is_primary="false",is_unique="false",schema="books_store",table="books"} 1.4196736e+07
`, idxFetch)
}

func expectIndexStatsRun(mock sqlmock.Sqlmock, idxFetch int) {
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}).
				AddRow("books_store", "books", nil, 39).
				AddRow("books_store", "books", "idx_books_title", idxFetch),
		)
	mock.ExpectQuery(fmt.Sprintf(selectIndexSizeBytes, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"database_name", "table_name", "index_name", "size_bytes", "non_unique"}).
				AddRow("books_store", "books", "PRIMARY", 65536, 0).
				AddRow("books_store", "books", "idx_books_title", 14196736, 1),
		)
}

func newTestIndexStats(t *testing.T, interval time.Duration) (*IndexStats, sqlmock.Sqlmock, *prometheus.Registry) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	registry := prometheus.NewRegistry()

	c, err := NewIndexStats(IndexStatsArguments{
		DB:              db,
		Registry:        registry,
		CollectInterval: interval,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)
	return c, mock, registry
}

func requireIndexStatsEventually(t *testing.T, registry *prometheus.Registry, idxFetch int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return testutil.CollectAndCompare(registry, strings.NewReader(indexStatsExpected(idxFetch))) == nil
	}, 5*time.Second, 10*time.Millisecond)
}

// requireIndexStatsConsistently fails if the exposed metrics ever differ from
// the expected ones during the given duration.
func requireIndexStatsConsistently(t *testing.T, registry *prometheus.Registry, idxFetch int, d time.Duration) {
	t.Helper()
	require.Never(t, func() bool {
		return testutil.CollectAndCompare(registry, strings.NewReader(indexStatsExpected(idxFetch))) != nil
	}, d, 10*time.Millisecond)
}

func TestIndexStats(t *testing.T) {
	c, mock, registry := newTestIndexStats(t, time.Hour)
	expectIndexStatsRun(mock, 39)

	require.NoError(t, c.Start(t.Context()))
	requireIndexStatsEventually(t, registry, 39)

	// Stop waits for the collector goroutine, so the mock is safe to inspect.
	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexStats_ScrapeDoesNotQueryDatabase(t *testing.T) {
	c, mock, registry := newTestIndexStats(t, time.Hour)
	expectIndexStatsRun(mock, 39)

	require.NoError(t, c.Start(t.Context()))
	requireIndexStatsEventually(t, registry, 39)

	// Any further query would fail as an unexpected call to the mock.
	for range 5 {
		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(indexStatsExpected(39))))
	}

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexStats_KeepsLastResultWhenQueriesFail(t *testing.T) {
	c, mock, registry := newTestIndexStats(t, 50*time.Millisecond)
	expectIndexStatsRun(mock, 39)
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillReturnError(errors.New("boom"))
	mock.ExpectQuery(fmt.Sprintf(selectIndexSizeBytes, exclusionClause)).WillReturnError(errors.New("boom"))

	require.NoError(t, c.Start(t.Context()))
	requireIndexStatsEventually(t, registry, 39)

	// Several failing runs happen in this window; the last result stays exposed.
	requireIndexStatsConsistently(t, registry, 39, 500*time.Millisecond)

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexStats_QueryIsBoundedByCollectInterval(t *testing.T) {
	c, mock, registry := newTestIndexStats(t, 100*time.Millisecond)
	expectIndexStatsRun(mock, 39)
	// A query slower than the collect interval must be cancelled, not waited for.
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillDelayFor(time.Hour).WillReturnRows(sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}))
	// The run after it succeeds with new data, proving the collector moved on
	// by itself rather than being unblocked by Stop.
	expectIndexStatsRun(mock, 40)

	require.NoError(t, c.Start(t.Context()))
	requireIndexStatsEventually(t, registry, 40)

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
