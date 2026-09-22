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

func TestTableStats(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()

	c, err := NewTableStats(TableStatsArguments{
		DB:       db,
		Registry: registry,
		Logger:   util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)

	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	mock.ExpectQuery(fmt.Sprintf(selectTableIOWaitsNoIndex, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "COUNT_FETCH"}).
				AddRow("books_store", "books", 39),
		)
	mock.ExpectQuery(fmt.Sprintf(selectTableRowCount, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"database_name", "table_name", "n_rows"}).
				AddRow("books_store", "books", 500),
		)

	expected := `
	# HELP database_observability_mysql_table_stats_no_idx_fetch_total Count of index I/O wait events for fetch operations that did not use an index
	# TYPE database_observability_mysql_table_stats_no_idx_fetch_total counter
	database_observability_mysql_table_stats_no_idx_fetch_total{schema="books_store",table="books"} 39
	# HELP database_observability_mysql_table_stats_row_count Estimated number of rows in this table
	# TYPE database_observability_mysql_table_stats_row_count gauge
	database_observability_mysql_table_stats_row_count{schema="books_store",table="books"} 500
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}
