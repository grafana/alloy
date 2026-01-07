package collector

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func Test_SetupActors(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("setup_actors properly configured", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("NO", "NO"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("setup_actors needs update with auto-update enabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("YES", "YES"))
		mock.ExpectExec(updateQuery).WithArgs("test_user").
			WillReturnResult(sqlmock.NewResult(0, 1))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("setup_actors needs update with auto-update disabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("YES", "YES"))
		// No ExpectExec for update since auto-update is disabled

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: false,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("setup_actors row missing with auto-update enabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(insertQuery).WithArgs("test_user").
			WillReturnResult(sqlmock.NewResult(1, 1))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("setup_actors row missing with auto-update disabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnError(sql.ErrNoRows)
		// No ExpectExec for insert since auto-update is disabled

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: false,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("error getting current user prevents start", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnError(fmt.Errorf("connection error"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection error")

		// Verify the collector is stopped after error
		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("error querying setup_actors table", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnError(fmt.Errorf("database error"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		// Wait for the first check to run (which will error)
		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		// Collector should still be running despite the error
		assert.False(t, c.Stopped())

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("update affects no rows", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("YES", "YES"))
		mock.ExpectExec(updateQuery).WithArgs("test_user").
			WillReturnResult(sqlmock.NewResult(0, 0))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		// Wait for the first check to run (which will error)
		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		// Collector should still be running despite the error
		assert.False(t, c.Stopped())

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("enabled needs update but history is correct", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("YES", "NO"))
		mock.ExpectExec(updateQuery).WithArgs("test_user").
			WillReturnResult(sqlmock.NewResult(0, 1))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("history needs update but enabled is correct", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("NO", "YES"))
		mock.ExpectExec(updateQuery).WithArgs("test_user").
			WillReturnResult(sqlmock.NewResult(0, 1))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("case insensitive check for enabled and history", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("no", "no"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("insert query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(insertQuery).WithArgs("test_user").
			WillReturnError(fmt.Errorf("insert failed"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		// Wait for the first check to run (which will error)
		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		// Collector should still be running despite the error
		assert.False(t, c.Stopped())

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("update query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("YES", "YES"))
		mock.ExpectExec(updateQuery).WithArgs("test_user").
			WillReturnError(fmt.Errorf("update failed"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		// Wait for the first check to run (which will error)
		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		// Collector should still be running despite the error
		assert.False(t, c.Stopped())

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("continues running even when checks fail", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		// First check fails
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnError(fmt.Errorf("database error"))
		// Second check succeeds
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("NO", "NO"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		// Wait for both checks to run
		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		// Collector should still be running despite the error
		assert.False(t, c.Stopped())

		c.Stop()

		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("context cancellation stops the collector", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("NO", "NO"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		err = c.Start(ctx)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		// Cancel the context
		cancel()

		// Verify the collector stopped
		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Stop() can be called multiple times safely", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"user"}).AddRow("test_user"))
		mock.ExpectQuery(selectQuery).WithArgs("test_user").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "history"}).
				AddRow("NO", "NO"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       time.Millisecond,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		err = c.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, 5*time.Second, 10*time.Millisecond)

		// Call Stop() multiple times - should not panic
		c.Stop()
		c.Stop()
		c.Stop()

		// Verify the collector stopped
		require.Eventually(t, func() bool {
			return c.Stopped()
		}, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}
