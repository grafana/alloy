package collector

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestDetectExtensions(t *testing.T) {
	const probeQuery = `SELECT extname FROM pg_extension WHERE extname IN ('timescaledb')`
	const probeSchemaQuery = `SELECT nspname FROM pg_namespace WHERE nspname IN ('_timescaledb_cache', '_timescaledb_catalog', '_timescaledb_config', '_timescaledb_debug', '_timescaledb_functions', '_timescaledb_internal')`

	t.Run("timescaledb present", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(probeQuery).WillReturnRows(
			sqlmock.NewRows([]string{"extname"}).AddRow("timescaledb"),
		)

		got := DetectExtensions(t.Context(), db, log.NewLogfmtLogger(os.Stderr))
		require.True(t, got.TimescaleDB)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("timescaledb absent", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(probeQuery).WillReturnRows(sqlmock.NewRows([]string{"extname"}))

		got := DetectExtensions(t.Context(), db, log.NewLogfmtLogger(os.Stderr))
		require.False(t, got.TimescaleDB)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query error falls back to schema probe", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(probeQuery).WillReturnError(errors.New("permission denied for table pg_extension"))
		mock.ExpectQuery(probeSchemaQuery).WillReturnRows(
			sqlmock.NewRows([]string{"nspname"}).AddRow("_timescaledb_catalog"),
		)

		got := DetectExtensions(t.Context(), db, log.NewLogfmtLogger(os.Stderr))
		require.True(t, got.TimescaleDB)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("schema probe preserves detected extension on row iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(probeQuery).WillReturnError(errors.New("permission denied for table pg_extension"))
		mock.ExpectQuery(probeSchemaQuery).WillReturnRows(
			sqlmock.NewRows([]string{"nspname"}).
				AddRow("_timescaledb_catalog").
				AddRow("_timescaledb_internal").
				RowError(1, errors.New("iteration failed after first row")),
		)

		got := DetectExtensions(t.Context(), db, log.NewLogfmtLogger(os.Stderr))
		require.True(t, got.TimescaleDB)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDetectExtensionsAcrossDatabases(t *testing.T) {
	t.Parallel()

	const probeQuery = `SELECT extname FROM pg_extension WHERE extname IN ('timescaledb')`

	rootDB, rootMock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer rootDB.Close()

	appDB, appMock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer appDB.Close()

	metricsDB, metricsMock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer metricsDB.Close()

	rootMock.ExpectQuery(fmt.Sprintf(selectDatabasesForExtensionDetection, exclusionClause)).
		WillReturnRows(sqlmock.NewRows([]string{"datname"}).
			AddRow("appdb").
			AddRow("metricsdb"))
	appMock.ExpectQuery(probeQuery).WillReturnRows(sqlmock.NewRows([]string{"extname"}))
	metricsMock.ExpectQuery(probeQuery).WillReturnRows(
		sqlmock.NewRows([]string{"extname"}).AddRow("timescaledb"),
	)

	dsns := map[string]*sql.DB{
		"postgres://user:pass@localhost:5432/appdb?sslmode=disable":     appDB,
		"postgres://user:pass@localhost:5432/metricsdb?sslmode=disable": metricsDB,
	}
	factory := func(dsn string) (*sql.DB, error) {
		db, ok := dsns[dsn]
		if !ok {
			return nil, fmt.Errorf("unexpected DSN: %s", dsn)
		}
		return db, nil
	}

	got := DetectExtensionsAcrossDatabases(
		t.Context(),
		rootDB,
		"postgres://user:pass@localhost:5432/appdb?sslmode=disable",
		nil,
		factory,
		log.NewLogfmtLogger(os.Stderr),
	)
	require.True(t, got.TimescaleDB)
	require.NoError(t, rootMock.ExpectationsWereMet())
	require.NoError(t, appMock.ExpectationsWereMet())
	require.NoError(t, metricsMock.ExpectationsWereMet())
}

func TestBuildExtensionExplainFilterClause(t *testing.T) {
	t.Parallel()

	require.Empty(t, buildExtensionExplainFilterClause(false, DetectedExtensions{TimescaleDB: true}))
	require.Empty(t, buildExtensionExplainFilterClause(true, DetectedExtensions{TimescaleDB: false}))

	got := buildExtensionExplainFilterClause(true, DetectedExtensions{TimescaleDB: true})
	require.Equal(t, `AND s.query !~ '_timescaledb_(cache|catalog|config|debug|functions|internal)\.'`, got)
}

func TestTimescaleDBSchemaListsAreIsolated(t *testing.T) {
	t.Parallel()

	a := TimescaleDBInternalSchemas()
	a[0] = "mutated"
	b := TimescaleDBInternalSchemas()
	require.NotEqual(t, "mutated", b[0], "returned slice must not share backing array")
}
