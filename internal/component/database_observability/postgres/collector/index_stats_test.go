package collector

import (
	"database/sql"
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
		DB:               db,
		DSN:              "postgres://user:pass@localhost:5432/books_store",
		ExcludeDatabases: nil,
		Registry:         registry,
		Logger:           util.TestAlloyLogger(t).Slog(),
		dbConnectionFactory: func(dsn string) (*sql.DB, error) {
			return db, nil
		},
	})
	require.NoError(t, err)

	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	mock.ExpectQuery(fmt.Sprintf(selectAllDatabases, exclusionClause)).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"datname"}).AddRow("books_store"))

	mock.ExpectQuery(selectTableScanStats).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"schemaname", "relname", "seq_scan", "idx_scan", "n_live_tup"}).
				AddRow("public", "gen_adjectives", 37154, 0, 500),
		)

	mock.ExpectQuery(selectIndexUsageStats).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"schemaname", "relname", "indexrelname", "idx_scan", "indisprimary", "index_size_bytes"}).
				AddRow("public", "books", "books_pkey", 184000000, true, 65536).
				AddRow("public", "books", "idx_books_title", 0, false, 32768),
		)

	expected := `
	# HELP pg_stat_user_indexes_idx_scan_total Number of index scans initiated on this index
	# TYPE pg_stat_user_indexes_idx_scan_total counter
	pg_stat_user_indexes_idx_scan_total{datname="books_store",indexrelname="books_pkey",relname="books",schemaname="public"} 1.84e+08
	pg_stat_user_indexes_idx_scan_total{datname="books_store",indexrelname="idx_books_title",relname="books",schemaname="public"} 0
	# HELP pg_stat_user_tables_idx_scan Number of index scans initiated on this table
	# TYPE pg_stat_user_tables_idx_scan counter
	pg_stat_user_tables_idx_scan{datname="books_store",relname="gen_adjectives",schemaname="public"} 0
	# HELP pg_stat_user_tables_n_live_tup Estimated number of live rows
	# TYPE pg_stat_user_tables_n_live_tup gauge
	pg_stat_user_tables_n_live_tup{datname="books_store",relname="gen_adjectives",schemaname="public"} 500
	# HELP pg_stat_user_tables_seq_scan Number of sequential scans initiated on this table
	# TYPE pg_stat_user_tables_seq_scan counter
	pg_stat_user_tables_seq_scan{datname="books_store",relname="gen_adjectives",schemaname="public"} 37154
	# HELP pg_index_properties Properties of an index; a constant 1 with is_primary set to whether the index backs a primary key
	# TYPE pg_index_properties gauge
	pg_index_properties{datname="books_store",indexrelname="books_pkey",is_primary="true",relname="books",schemaname="public"} 1
	pg_index_properties{datname="books_store",indexrelname="idx_books_title",is_primary="false",relname="books",schemaname="public"} 1
	# HELP pg_index_size_bytes Total disk space used by this index, in bytes
	# TYPE pg_index_size_bytes gauge
	pg_index_size_bytes{datname="books_store",indexrelname="books_pkey",relname="books",schemaname="public"} 65536
	pg_index_size_bytes{datname="books_store",indexrelname="idx_books_title",relname="books",schemaname="public"} 32768
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}
