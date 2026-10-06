package collector

import (
	"context"
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

func ioWaitsRows(fetch int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}).
		AddRow("books_store", "books", nil, fetch)
}

func TestIOWaitsScan_ReusesResultWithinMaxAge(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	scan := NewIOWaitsScan(db, nil)
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillReturnRows(ioWaitsRows(1))
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillReturnRows(ioWaitsRows(2))

	first, err := scan.get(t.Context(), time.Hour)
	require.NoError(t, err)
	again, err := scan.get(t.Context(), time.Hour)
	require.NoError(t, err)
	require.Equal(t, first, again, "a second call within maxAge must not scan again")
	require.EqualValues(t, 1, first[0].countFetch)

	// Too old for a zero maxAge: scans again.
	fresh, err := scan.get(t.Context(), 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, fresh[0].countFetch)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIOWaitsScan_WaitingCallerGivesUpWhenContextIsDone(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	scan := NewIOWaitsScan(db, nil)
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillDelayFor(time.Hour).WillReturnRows(sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}))

	// The first caller holds the scan until its own context is cancelled.
	holderCtx, cancelHolder := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { _, _ = scan.get(holderCtx, 0); close(done) }()
	time.Sleep(100 * time.Millisecond)

	waiterCtx, cancelWaiter := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancelWaiter()
	_, err = scan.get(waiterCtx, 0)
	require.ErrorIs(t, err, waiterCtx.Err(), "a waiting caller must stop waiting when its context is done")

	cancelHolder()
	<-done
}

// TestTableStatsAndIndexStatsShareOneScan guards the point of IOWaitsScan: with
// both collectors enabled, performance_schema is scanned once per cycle.
func TestTableStatsAndIndexStatsShareOneScan(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	// The collectors run concurrently, so their queries arrive in either order.
	mock.MatchExpectationsInOrder(false)

	// A single scan query is expected; a second one would be an unexpected call.
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}).
			AddRow("books_store", "books", nil, 39).
			AddRow("books_store", "books", "idx_books_title", 7))
	mock.ExpectQuery(fmt.Sprintf(selectTableRowCount, exclusionClause)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"database_name", "table_name", "n_rows"}).AddRow("books_store", "books", 500))
	mock.ExpectQuery(fmt.Sprintf(selectIndexSizeBytes, exclusionClause)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"database_name", "table_name", "index_name", "size_bytes", "non_unique"}).
			AddRow("books_store", "books", "idx_books_title", 100, 1))

	registry := prometheus.NewRegistry()
	scan := NewIOWaitsScan(db, nil)
	logger := util.TestAlloyLogger(t).Slog()

	ts, err := NewTableStats(TableStatsArguments{DB: db, Registry: registry, CollectInterval: time.Hour, IOWaits: scan, Logger: logger})
	require.NoError(t, err)
	is, err := NewIndexStats(IndexStatsArguments{DB: db, Registry: registry, CollectInterval: time.Hour, IOWaits: scan, Logger: logger})
	require.NoError(t, err)
	require.NoError(t, ts.Start(t.Context()))
	require.NoError(t, is.Start(t.Context()))

	require.Eventually(t, func() bool {
		return testutil.CollectAndCount(registry, "database_observability_mysql_table_stats_no_idx_fetch_total") == 1 &&
			testutil.CollectAndCount(registry, "database_observability_mysql_index_stats_idx_fetch_total") == 1 &&
			testutil.CollectAndCount(registry, "database_observability_mysql_table_stats_row_count") == 1 &&
			testutil.CollectAndCount(registry, "database_observability_mysql_index_stats_size_bytes") == 1
	}, 5*time.Second, 10*time.Millisecond)

	// Each collector only exposes its own rows of the shared scan.
	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(`
	# HELP database_observability_mysql_table_stats_no_idx_fetch_total Count of index I/O wait events for fetch operations that did not use an index
	# TYPE database_observability_mysql_table_stats_no_idx_fetch_total counter
	database_observability_mysql_table_stats_no_idx_fetch_total{schema="books_store",table="books"} 39
	`), "database_observability_mysql_table_stats_no_idx_fetch_total"))
	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(`
	# HELP database_observability_mysql_index_stats_idx_fetch_total Count of index I/O wait events for fetch operations
	# TYPE database_observability_mysql_index_stats_idx_fetch_total counter
	database_observability_mysql_index_stats_idx_fetch_total{index="idx_books_title",schema="books_store",table="books"} 7
	`), "database_observability_mysql_index_stats_idx_fetch_total"))

	ts.Stop()
	is.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}
