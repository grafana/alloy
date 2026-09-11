package collector

import (
	"fmt"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

func TestIndexStats(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()

	c, err := NewIndexStats(IndexStatsArguments{
		DB:       db,
		Registry: registry,
		Logger:   util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	mock.ExpectQuery(fmt.Sprintf(selectIndexIOWaits, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}).
				AddRow("books_store", "books", "idx_books_title", 0),
		)
	mock.ExpectQuery(fmt.Sprintf(selectIndexSizeBytes, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"database_name", "table_name", "index_name", "size_bytes"}).
				AddRow("books_store", "books", "idx_books_title", 14196736),
		)

	expected := `
	# HELP database_observability_mysql_index_stats_idx_fetch_total Count of index I/O wait events for fetch operations
	# TYPE database_observability_mysql_index_stats_idx_fetch_total counter
	database_observability_mysql_index_stats_idx_fetch_total{index="idx_books_title",schema="books_store",table="books"} 0
	# HELP database_observability_mysql_index_stats_size_bytes Total disk space used by this index, in bytes
	# TYPE database_observability_mysql_index_stats_size_bytes gauge
	database_observability_mysql_index_stats_size_bytes{index="idx_books_title",schema="books_store",table="books"} 1.4196736e+07
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}
