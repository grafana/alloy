package collector

import (
	"testing"

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
