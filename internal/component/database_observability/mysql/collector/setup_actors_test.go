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
)

func Test_checkSetupActors(t *testing.T) {
	t.Run("setup actors properly configured", func(t *testing.T) {
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.NoError(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("setup actors needs update with auto-update enabled", func(t *testing.T) {
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.NoError(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("setup actors needs update with auto-update disabled", func(t *testing.T) {
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: false,
		})
		require.NoError(t, err)

		assert.NoError(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("setup actors row missing with auto-update enabled", func(t *testing.T) {
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.NoError(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("setup actors row missing with auto-update disabled", func(t *testing.T) {
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: false,
		})
		require.NoError(t, err)

		assert.NoError(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("error getting current user", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUserQuery).WithoutArgs().
			WillReturnError(fmt.Errorf("connection error"))

		c, err := NewSetupActors(SetupActorsArguments{
			DB:                    db,
			Logger:                log.NewLogfmtLogger(os.Stderr),
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.Error(t, c.checkSetupActors(context.Background()))
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.Error(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.Error(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.NoError(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.NoError(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.NoError(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.Error(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
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
			CollectInterval:       1 * time.Second,
			AutoUpdateSetupActors: true,
		})
		require.NoError(t, err)

		assert.Error(t, c.checkSetupActors(context.Background()))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
