package collector

import (
	"fmt"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

func TestIndexStats(t *testing.T) {
	t.Run("below minIndexSizeEngineVersion, size query is skipped", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		registry := prometheus.NewRegistry()

		c, err := NewIndexStats(IndexStatsArguments{
			DB:            db,
			EngineVersion: semver.MustParse("5.5.62"),
			Registry:      registry,
			Logger:        util.TestAlloyLogger(t).Slog(),
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
		# HELP mysql_index_stats_idx_scan_total Number of row fetches against this index
		# TYPE mysql_index_stats_idx_scan_total counter
		mysql_index_stats_idx_scan_total{index="idx_books_title",schema="books_store",table="books"} 0
`

		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("at or above minIndexSizeEngineVersion, size is collected from mysql.innodb_index_stats", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		registry := prometheus.NewRegistry()

		c, err := NewIndexStats(IndexStatsArguments{
			DB:            db,
			EngineVersion: semver.MustParse("8.0.36"),
			Registry:      registry,
			Logger:        util.TestAlloyLogger(t).Slog(),
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
		mock.ExpectQuery(fmt.Sprintf(selectIndexSizeBytes, sqlPlaceholders(len(args)))).
			WithArgs(toDriverValues(args)...).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{"database_name", "table_name", "index_name", "size_bytes"}).
					AddRow("books_store", "books", "idx_books_title", 14196736),
			)

		expected := `
		# HELP mysql_index_stats_idx_scan_total Number of row fetches against this index
		# TYPE mysql_index_stats_idx_scan_total counter
		mysql_index_stats_idx_scan_total{index="idx_books_title",schema="books_store",table="books"} 0
		# HELP mysql_index_stats_size_bytes Total disk space used by this index, in bytes
		# TYPE mysql_index_stats_size_bytes gauge
		mysql_index_stats_size_bytes{index="idx_books_title",schema="books_store",table="books"} 1.4196736e+07
`

		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
