package collector

import (
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestConnectToDatabaseReusesInitialConnection guards against a regression
// where connectToDatabase opens (and immediately closes) a redundant
// connection for the one database in a fan-out that's already the database
// initial points to. With the real sql.Open-based factory, a freshly opened
// *sql.DB is never pointer-equal to initial even for an identical DSN, so
// the "same database" case has to be detected up front, before the factory
// is ever called -- a bare `conn != initial` check after the fact can't
// catch it.
func TestConnectToDatabaseReusesInitialConnection(t *testing.T) {
	initial, _, err := sqlmock.New()
	require.NoError(t, err)
	defer initial.Close()

	newDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer newDB.Close()

	t.Run("same database as the DSN: reuses initial, never calls factory", func(t *testing.T) {
		factoryCalls := 0
		factory := func(dsn string) (*sql.DB, error) {
			factoryCalls++
			return newDB, nil
		}

		conn, closeFn, err := connectToDatabase("postgres://user:pass@localhost:5432/books_store", "books_store", factory, initial)
		require.NoError(t, err)
		require.Same(t, initial, conn)
		require.Equal(t, 0, factoryCalls)
		closeFn() // must not close initial

		require.NoError(t, initial.PingContext(t.Context())) // still usable
	})

	t.Run("different database: opens a new connection via factory", func(t *testing.T) {
		factoryCalls := 0
		factory := func(dsn string) (*sql.DB, error) {
			factoryCalls++
			return newDB, nil
		}

		conn, closeFn, err := connectToDatabase("postgres://user:pass@localhost:5432/postgres", "books_store", factory, initial)
		require.NoError(t, err)
		require.Same(t, newDB, conn)
		require.Equal(t, 1, factoryCalls)
		closeFn()
	})
}
