package collector

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

// toDriverValues converts args built for QueryContext (a mysql query's
// natural argument type is any) into the []driver.Value sqlmock expects.
func toDriverValues(args []any) []driver.Value {
	values := make([]driver.Value, len(args))
	for i, a := range args {
		values[i] = a
	}
	return values
}

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

	args := excludedSchemasArgs(nil)
	mock.ExpectQuery(fmt.Sprintf(selectTableIOWaitsNoIndex, sqlPlaceholders(len(args)))).
		WithArgs(toDriverValues(args)...).RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "COUNT_FETCH"}).
				AddRow("books_store", "books", 39),
		)

	expected := `
	# HELP mysql_table_stats_seq_scan_total Number of row fetches against this table that did not use an index
	# TYPE mysql_table_stats_seq_scan_total counter
	mysql_table_stats_seq_scan_total{schema="books_store",table="books"} 39
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}
