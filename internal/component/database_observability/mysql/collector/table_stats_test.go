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

func tableStatsExpected(noIdxFetch int) string {
	return fmt.Sprintf(`
	# HELP database_observability_mysql_table_stats_no_idx_fetch_total Count of index I/O wait events for fetch operations that did not use an index
	# TYPE database_observability_mysql_table_stats_no_idx_fetch_total counter
	database_observability_mysql_table_stats_no_idx_fetch_total{schema="books_store",table="books"} %d
	# HELP database_observability_mysql_table_stats_row_count Estimated number of rows in this table
	# TYPE database_observability_mysql_table_stats_row_count gauge
	database_observability_mysql_table_stats_row_count{schema="books_store",table="books"} 500
`, noIdxFetch)
}

func expectTableStatsRun(mock sqlmock.Sqlmock, noIdxFetch int) {
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}).
				AddRow("books_store", "books", nil, noIdxFetch).
				AddRow("books_store", "books", "idx_books_title", 7),
		)
	mock.ExpectQuery(fmt.Sprintf(selectTableRowCount, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"database_name", "table_name", "n_rows"}).
				AddRow("books_store", "books", 500),
		)
}

func newTestTableStats(t *testing.T, interval time.Duration) (*TableStats, sqlmock.Sqlmock, *prometheus.Registry) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	registry := prometheus.NewRegistry()

	c, err := NewTableStats(TableStatsArguments{
		DB:              db,
		Registry:        registry,
		CollectInterval: interval,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)
	return c, mock, registry
}

func requireTableStatsEventually(t *testing.T, registry *prometheus.Registry, noIdxFetch int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return testutil.CollectAndCompare(registry, strings.NewReader(tableStatsExpected(noIdxFetch))) == nil
	}, 5*time.Second, 10*time.Millisecond)
}

// requireTableStatsConsistently fails if the exposed metrics ever differ from
// the expected ones during the given duration.
func requireTableStatsConsistently(t *testing.T, registry *prometheus.Registry, noIdxFetch int, d time.Duration) {
	t.Helper()
	require.Never(t, func() bool {
		return testutil.CollectAndCompare(registry, strings.NewReader(tableStatsExpected(noIdxFetch))) != nil
	}, d, 10*time.Millisecond)
}

func TestTableStats(t *testing.T) {
	c, mock, registry := newTestTableStats(t, time.Hour)
	expectTableStatsRun(mock, 39)

	require.NoError(t, c.Start(t.Context()))
	requireTableStatsEventually(t, registry, 39)

	// Stop waits for the collector goroutine, so the mock is safe to inspect.
	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTableStats_ScrapeDoesNotQueryDatabase(t *testing.T) {
	c, mock, registry := newTestTableStats(t, time.Hour)
	expectTableStatsRun(mock, 39)

	require.NoError(t, c.Start(t.Context()))
	requireTableStatsEventually(t, registry, 39)

	// Any further query would fail as an unexpected call to the mock.
	for range 5 {
		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(tableStatsExpected(39))))
	}

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTableStats_KeepsLastResultWhenQueriesFail(t *testing.T) {
	c, mock, registry := newTestTableStats(t, 50*time.Millisecond)
	expectTableStatsRun(mock, 39)
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillReturnError(errors.New("boom"))
	mock.ExpectQuery(fmt.Sprintf(selectTableRowCount, exclusionClause)).WillReturnError(errors.New("boom"))

	require.NoError(t, c.Start(t.Context()))
	requireTableStatsEventually(t, registry, 39)

	// Several failing runs happen in this window; the last result stays exposed.
	requireTableStatsConsistently(t, registry, 39, 500*time.Millisecond)

	c.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTableStats_QueryIsBoundedByCollectInterval(t *testing.T) {
	c, mock, registry := newTestTableStats(t, 100*time.Millisecond)
	expectTableStatsRun(mock, 39)
	// A query slower than the collect interval must be cancelled, not waited for.
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillDelayFor(time.Hour).WillReturnRows(sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}))
	// The run after it succeeds with new data, proving the collector moved on
	// by itself rather than being unblocked by Stop.
	expectTableStatsRun(mock, 40)

	require.NoError(t, c.Start(t.Context()))
	requireTableStatsEventually(t, registry, 40)

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
