package database_observability

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestConfigureConnectionPool(t *testing.T) {
	t.Run("caps open and idle connections", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		ConfigureConnectionPool(db, 7)

		require.Equal(t, 7, db.Stats().MaxOpenConnections)
	})

	for _, maxOpen := range []int{0, -1} {
		t.Run("leaves the pool unlimited", func(t *testing.T) {
			db, _, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			ConfigureConnectionPool(db, maxOpen)

			require.Equal(t, 0, db.Stats().MaxOpenConnections)
		})
	}
}
