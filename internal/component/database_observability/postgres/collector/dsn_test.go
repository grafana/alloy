package collector

import (
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestReplaceDatabaseNameInDSN(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		newDBName   string
		expected    string
		expectError bool
	}{
		{
			name:      "basic postgres DSN",
			dsn:       "postgres://user:pass@localhost:5432/mydb",
			newDBName: "newdb",
			expected:  "postgres://user:pass@localhost:5432/newdb",
		},
		{
			name:      "postgres DSN with query parameters",
			dsn:       "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
			newDBName: "newdb",
			expected:  "postgres://user:pass@localhost:5432/newdb?sslmode=disable",
		},
		{
			name:      "postgres DSN with multiple query parameters",
			dsn:       "postgres://user:pass@localhost:5432/mydb?sslmode=disable&connect_timeout=10",
			newDBName: "newdb",
			expected:  "postgres://user:pass@localhost:5432/newdb?sslmode=disable&connect_timeout=10",
		},
		{
			name:      "problematic case - database name is 'postgres'",
			dsn:       "postgres://postgres:password@localhost:5432/postgres",
			newDBName: "testdb",
			expected:  "postgres://postgres:password@localhost:5432/testdb",
		},
		{
			name:      "database name appears in password",
			dsn:       "postgres://user:mydb123@localhost:5432/mydb",
			newDBName: "newdb",
			expected:  "postgres://user:mydb123@localhost:5432/newdb",
		},
		{
			name:      "database name with special characters",
			dsn:       "postgres://user:pass@localhost:5432/my-db_test$1",
			newDBName: "new_db",
			expected:  "postgres://user:pass@localhost:5432/new_db",
		},
		{
			name:      "unix socket - minimum postgres DSN",
			dsn:       "postgres:///mydb?host=/run/postgresql",
			newDBName: "newdb",
			expected:  "postgres:///newdb?host=/run/postgresql",
		},
		{
			name:      "unix socket - general postgres DSN",
			dsn:       "postgres://user:@/mydb?host=/run/postgresql",
			newDBName: "newdb",
			expected:  "postgres://user:@/newdb?host=/run/postgresql",
		},
		{
			name:      "hostname with special characters",
			dsn:       "postgres://user:pass@ex-amp_le.com:5432/mydb",
			newDBName: "new_db",
			expected:  "postgres://user:pass@ex-amp_le.com:5432/new_db",
		},
		{
			name:        "invalid DSN format",
			dsn:         "invalid-dsn-format",
			newDBName:   "newdb",
			expectError: true,
		},
		{
			name:        "DSN without database name",
			dsn:         "postgres://user:pass@localhost:5432/",
			newDBName:   "newdb",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := replaceDatabaseNameInDSN(tt.dsn, tt.newDBName)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

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
