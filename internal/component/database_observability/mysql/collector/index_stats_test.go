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

	args := excludedSchemasArgs(nil)
	mock.ExpectQuery(fmt.Sprintf(selectIndexIOWaits, sqlPlaceholders(len(args)))).
		WithArgs(toDriverValues(args)...).RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}).
				AddRow("books_store", "books", "idx_books_title", 0),
		)

	expected := `
	# HELP mysql_perf_schema_index_io_waits_total The total number of index I/O wait events for each index and operation.
	# TYPE mysql_perf_schema_index_io_waits_total counter
	mysql_perf_schema_index_io_waits_total{index="idx_books_title",name="books",operation="fetch",schema="books_store"} 0
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}
