package collector

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
)

func expectPermissions(mock sqlmock.Sqlmock, permissions ...string) {
	rows := sqlmock.NewRows([]string{"permission_name"})
	for _, p := range permissions {
		rows.AddRow(p)
	}
	mock.ExpectQuery(selectMyPermissionsQuery).WithoutArgs().RowsWillBeClosed().WillReturnRows(rows)
}

func expectQueryStoreState(mock sqlmock.Sqlmock, dbName, actualState string) {
	mock.ExpectQuery(selectQueryStoreState).WithoutArgs().RowsWillBeClosed().WillReturnRows(
		sqlmock.NewRows([]string{"", "actual_state_desc", "query_capture_mode_desc", "readonly_reason"}).
			AddRow(dbName, actualState, "ALL", nil),
	)
}

func expectQueryStoreHasRows(mock sqlmock.Sqlmock, hasRows bool) {
	q := mock.ExpectQuery(selectQueryStoreHasRowsQuery).WithoutArgs()
	if hasRows {
		q.RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{""}).AddRow(1))
	} else {
		q.WillReturnError(sql.ErrNoRows)
	}
}

func TestHealthCheck(t *testing.T) {
	defer goleak.VerifyNone(t)

	newCollector := func(t *testing.T, db *sql.DB) *HealthCheck {
		lokiClient := loki.NewCollectingHandler()
		t.Cleanup(lokiClient.Stop)
		c, err := NewHealthCheck(HealthCheckArguments{
			DB:              db,
			CollectInterval: time.Millisecond,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)
		require.NotNil(t, c)
		return c
	}

	t.Run("all checks pass", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		c := newCollector(t, db)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")
		expectPermissions(mock, "VIEW DEFINITION", "VIEW DATABASE STATE")
		expectQueryStoreState(mock, "some_db", "READ_WRITE")
		expectQueryStoreHasRows(mock, true)

		results, err := c.runPerDatabaseChecks(t.Context())
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())

		byName := resultsByName(results)
		require.True(t, byName["RequiredGrantsPresent"].result)
		require.True(t, byName["QueryStoreEnabled"].result)
		require.True(t, byName["QueryStoreHasRows"].result)
	})

	t.Run("missing grant on one database", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		c := newCollector(t, db)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")
		expectPermissions(mock, "VIEW DEFINITION") // missing VIEW DATABASE STATE
		expectQueryStoreState(mock, "some_db", "READ_WRITE")
		expectQueryStoreHasRows(mock, true)

		results, err := c.runPerDatabaseChecks(t.Context())
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())

		byName := resultsByName(results)
		require.False(t, byName["RequiredGrantsPresent"].result)
		require.Contains(t, byName["RequiredGrantsPresent"].value, "some_db")
		require.True(t, byName["QueryStoreEnabled"].result)
	})

	t.Run("query store disabled on one database", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		c := newCollector(t, db)

		expectListDatabases(mock, "db_a", "db_b")
		expectUseDatabase(mock, "db_a")
		expectPermissions(mock, "VIEW DEFINITION", "VIEW DATABASE STATE")
		expectQueryStoreState(mock, "db_a", "OFF")
		expectQueryStoreHasRows(mock, false)
		expectUseDatabase(mock, "db_b")
		expectPermissions(mock, "VIEW DEFINITION", "VIEW DATABASE STATE")
		expectQueryStoreState(mock, "db_b", "READ_WRITE")
		expectQueryStoreHasRows(mock, true)

		results, err := c.runPerDatabaseChecks(t.Context())
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())

		byName := resultsByName(results)
		require.True(t, byName["RequiredGrantsPresent"].result)
		require.False(t, byName["QueryStoreEnabled"].result)
		require.Contains(t, byName["QueryStoreEnabled"].value, "db_a")
		require.NotContains(t, byName["QueryStoreEnabled"].value, "db_b")
		// db_b has rows, so the aggregate has-rows check passes even though db_a doesn't.
		require.True(t, byName["QueryStoreHasRows"].result)
	})

	t.Run("USE failure is noted separately, not reported as missing grants or disabled query store", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		c := newCollector(t, db)

		expectListDatabases(mock, "db_a", "db_b")
		mock.ExpectExec("USE [db_a]").WillReturnError(fmt.Errorf("database is currently restoring"))
		expectUseDatabase(mock, "db_b")
		expectPermissions(mock, "VIEW DEFINITION", "VIEW DATABASE STATE")
		expectQueryStoreState(mock, "db_b", "READ_WRITE")
		expectQueryStoreHasRows(mock, true)

		results, err := c.runPerDatabaseChecks(t.Context())
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())

		byName := resultsByName(results)
		// db_a couldn't be checked at all, but db_b is fully compliant, so the
		// aggregate checks pass rather than falsely reporting db_a as
		// non-compliant with grants or Query Store.
		require.True(t, byName["RequiredGrantsPresent"].result)
		require.NotContains(t, byName["RequiredGrantsPresent"].value, "missing grants")
		require.Contains(t, byName["RequiredGrantsPresent"].value, "could not check: db_a")
		require.True(t, byName["QueryStoreEnabled"].result)
		require.NotContains(t, byName["QueryStoreEnabled"].value, "not enabled")
		require.Contains(t, byName["QueryStoreEnabled"].value, "could not check: db_a")
		require.True(t, byName["QueryStoreHasRows"].result)
		require.Contains(t, byName["QueryStoreHasRows"].value, "could not check: db_a")
	})

	t.Run("no rows anywhere", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		c := newCollector(t, db)

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")
		expectPermissions(mock, "VIEW DEFINITION", "VIEW DATABASE STATE")
		expectQueryStoreState(mock, "some_db", "READ_WRITE")
		expectQueryStoreHasRows(mock, false)

		results, err := c.runPerDatabaseChecks(t.Context())
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())

		byName := resultsByName(results)
		require.False(t, byName["QueryStoreHasRows"].result)
	})

	t.Run("no accessible databases", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		c := newCollector(t, db)

		expectListDatabases(mock)

		results, err := c.runPerDatabaseChecks(t.Context())
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())

		byName := resultsByName(results)
		require.Equal(t, "no accessible databases", byName["RequiredGrantsPresent"].value)
		require.Equal(t, "no accessible databases", byName["QueryStoreEnabled"].value)
		require.False(t, byName["QueryStoreHasRows"].result)
	})

	t.Run("Start/Stop lifecycle emits AlloyVersion and per-database checks", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		expectListDatabases(mock, "some_db")
		expectUseDatabase(mock, "some_db")
		expectPermissions(mock, "VIEW DEFINITION", "VIEW DATABASE STATE")
		expectQueryStoreState(mock, "some_db", "READ_WRITE")
		expectQueryStoreHasRows(mock, true)
		mock.MatchExpectationsInOrder(false)

		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()
		c, err := NewHealthCheck(HealthCheckArguments{
			DB:              db,
			CollectInterval: time.Hour,
			EntryHandler:    lokiClient,
			Logger:          util.TestAlloyLogger(t).Slog(),
		})
		require.NoError(t, err)

		require.NoError(t, c.Start(t.Context()))
		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) >= 4
		}, time.Second, time.Millisecond, "expected 4 health_status log lines (AlloyVersion + 3 per-database checks)")

		c.Stop()
		require.True(t, c.Stopped())

		var checks []string
		for _, entry := range lokiClient.Received() {
			checks = append(checks, entry.Line)
		}
		require.Contains(t, fmt.Sprint(checks), "AlloyVersion")
	})
}

func resultsByName(results []healthCheckResult) map[string]healthCheckResult {
	m := make(map[string]healthCheckResult, len(results))
	for _, r := range results {
		m[r.name] = r
	}
	return m
}
