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

	mock.ExpectQuery(selectIndexUsageStats).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(
			sqlmock.NewRows([]string{"schemaname", "relname", "indexrelname", "idx_scan", "indisprimary", "indisunique", "is_partial", "index_size_bytes"}).
				AddRow("public", "books", "books_pkey", 184000000, true, true, false, 65536).
				AddRow("public", "books", "idx_books_title", 0, false, false, true, 32768),
		)

	expected := `
	# HELP database_observability_pg_index_stats_idx_scan_total Number of index scans initiated on this index
	# TYPE database_observability_pg_index_stats_idx_scan_total counter
	database_observability_pg_index_stats_idx_scan_total{datname="books_store",indexrelname="books_pkey",relname="books",schemaname="public"} 1.84e+08
	database_observability_pg_index_stats_idx_scan_total{datname="books_store",indexrelname="idx_books_title",relname="books",schemaname="public"} 0
	# HELP database_observability_pg_index_stats_size_bytes Total disk space used by this index, in bytes, labeled with whether it backs the primary key or a unique constraint, or is partial
	# TYPE database_observability_pg_index_stats_size_bytes gauge
	database_observability_pg_index_stats_size_bytes{datname="books_store",indexrelname="books_pkey",is_partial="false",is_primary="true",is_unique="true",relname="books",schemaname="public"} 65536
	database_observability_pg_index_stats_size_bytes{datname="books_store",indexrelname="idx_books_title",is_partial="true",is_primary="false",is_unique="false",relname="books",schemaname="public"} 32768
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}
