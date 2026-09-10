package collector

import (
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestConnectToDatabaseReusesInitialConnection guards against connectToDatabase
// opening a redundant connection for the database initial already points to;
// see the comment on connectToDatabase for why a bare "conn != initial" check
// can't catch this.
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
