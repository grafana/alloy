package collector

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestLocks(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Run("no logs with no lock events", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewLocks(LocksArguments{
			DB:              db,
			CollectInterval: 10 * time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDataLocks).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows(
				[]string{
					"waitingTimerWait",
					"waitingLockTime",
					"waitingDigest",
					"waitingDigestText",
					"blockingTimerWait",
					"blockingLockTime",
					"blockingDigest",
					"blockingDigestText",
				},
			),
		)

		require.NoError(t, collector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 0
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		lokiEntries := lokiClient.Received()
		assert.Empty(t, lokiEntries, "Expected no log entries for no lock events")
	})

	t.Run("data lock is logged", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewLocks(LocksArguments{
			DB:              db,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDataLocks).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows(
				[]string{
					"waitingTimerWait",
					"waitingLockTime",
					"waitingDigest",
					"waitingDigestText",
					"blockingTimerWait",
					"blockingLockTime",
					"blockingDigest",
					"blockingDigestText",
				},
			).AddRow(
				1500000000000,
				1000000000000,
				"abc123",
				"SELECT * FROM users WHERE id = ?",
				2000000000000,
				1700000000000,
				"def456",
				"UPDATE users SET name = ? WHERE id = ?",
			),
		)

		require.NoError(t, collector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.NoError(t, mock.ExpectationsWereMet())
		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": OP_DATA_LOCKS}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" waiting_digest="abc123" waiting_digest_text="SELECT * FROM users WHERE id = ?" blocking_digest="def456" blocking_digest_text="UPDATE users SET name = ? WHERE id = ?" waiting_timer_wait="1500.000000ms" waiting_lock_time="1000.000000ms" blocking_timer_wait="2000.000000ms" blocking_lock_time="1700.000000ms"`, lokiEntries[0].Line)
	})

	t.Run("multiple data locks are logged", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewLocks(LocksArguments{
			DB:              db,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDataLocks).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows(
				[]string{
					"waitingTimerWait",
					"waitingLockTime",
					"waitingDigest",
					"waitingDigestText",
					"blockingTimerWait",
					"blockingLockTime",
					"blockingDigest",
					"blockingDigestText",
				},
			).AddRow(
				1500000000000,
				1000000000000,
				"abc123",
				"SELECT * FROM users WHERE id = ?",
				2000000000000,
				1700000000000,
				"def456",
				"UPDATE users SET name = ? WHERE id = ?",
			).AddRow(
				2500000000000,
				2000000000000,
				"xyz789",
				"SELECT * FROM orders WHERE user_id = ?",
				3000000000000,
				2700000000000,
				"ghi012",
				"DELETE FROM sessions WHERE expired = ?",
			),
		)

		require.NoError(t, collector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.NoError(t, mock.ExpectationsWereMet())
		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		assert.Equal(t, model.LabelSet{"op": OP_DATA_LOCKS}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" waiting_digest="abc123" waiting_digest_text="SELECT * FROM users WHERE id = ?" blocking_digest="def456" blocking_digest_text="UPDATE users SET name = ? WHERE id = ?" waiting_timer_wait="1500.000000ms" waiting_lock_time="1000.000000ms" blocking_timer_wait="2000.000000ms" blocking_lock_time="1700.000000ms"`, lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"op": OP_DATA_LOCKS}, lokiEntries[1].Labels)
		assert.Equal(t, `level="info" waiting_digest="xyz789" waiting_digest_text="SELECT * FROM orders WHERE user_id = ?" blocking_digest="ghi012" blocking_digest_text="DELETE FROM sessions WHERE expired = ?" waiting_timer_wait="2500.000000ms" waiting_lock_time="2000.000000ms" blocking_timer_wait="3000.000000ms" blocking_lock_time="2700.000000ms"`, lokiEntries[1].Line)
	})

	t.Run("data lock with null digests and digest texts", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewLocks(LocksArguments{
			DB:              db,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDataLocks).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows(
				[]string{
					"waitingTimerWait",
					"waitingLockTime",
					"waitingDigest",
					"waitingDigestText",
					"blockingTimerWait",
					"blockingLockTime",
					"blockingDigest",
					"blockingDigestText",
				},
			).AddRow(
				1500000000000,
				1000000000000,
				nil,
				nil,
				2000000000000,
				1700000000000,
				nil,
				nil,
			),
		)

		require.NoError(t, collector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.NoError(t, mock.ExpectationsWereMet())
		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"op": OP_DATA_LOCKS}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" waiting_digest="" waiting_digest_text="" blocking_digest="" blocking_digest_text="" waiting_timer_wait="1500.000000ms" waiting_lock_time="1000.000000ms" blocking_timer_wait="2000.000000ms" blocking_lock_time="1700.000000ms"`, lokiEntries[0].Line)
	})

	t.Run("recoverable sql error in selectDataLocks result set", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewLocks(LocksArguments{
			DB:              db,
			CollectInterval: 1 * time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDataLocks).WillReturnError(fmt.Errorf("some error"))

		mock.ExpectQuery(selectDataLocks).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows(
				[]string{
					"waitingTimerWait",
					"waitingLockTime",
					"waitingDigest",
					"waitingDigestText",
					"blockingTimerWait",
					"blockingLockTime",
					"blockingDigest",
					"blockingDigestText",
				},
			).AddRow(
				1500000000000,
				1000000000000,
				"abc123",
				"SELECT * FROM users WHERE id = ?",
				2000000000000,
				1700000000000,
				"def456",
				"UPDATE users SET name = ? WHERE id = ?",
			),
		)

		require.NoError(t, collector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 1)
		assert.Equal(t, model.LabelSet{"op": OP_DATA_LOCKS}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" waiting_digest="abc123" waiting_digest_text="SELECT * FROM users WHERE id = ?" blocking_digest="def456" blocking_digest_text="UPDATE users SET name = ? WHERE id = ?" waiting_timer_wait="1500.000000ms" waiting_lock_time="1000.000000ms" blocking_timer_wait="2000.000000ms" blocking_lock_time="1700.000000ms"`, lokiEntries[0].Line)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("recoverable sql error in result set", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewLocks(LocksArguments{
			DB:              db,
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDataLocks).WillReturnRows( // first query returns an error, but loop will continue
			sqlmock.NewRows(
				[]string{
					"some_column", // not enough columns
				},
			).AddRow(
				1500000000000,
			))

		mock.ExpectQuery(selectDataLocks).RowsWillBeClosed().WillReturnRows( // second query returns valid data and its 1 row produces 1 log line
			sqlmock.NewRows(
				[]string{
					"waitingTimerWait",
					"waitingLockTime",
					"waitingDigest",
					"waitingDigestText",
					"blockingTimerWait",
					"blockingLockTime",
					"blockingDigest",
					"blockingDigestText",
				},
			).AddRow(
				1500000000000,
				1000000000000,
				"abc123",
				"SELECT * FROM users WHERE id = ?",
				2000000000000,
				1700000000000,
				"def456",
				"UPDATE users SET name = ? WHERE id = ?",
			))

		require.NoError(t, collector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.NoError(t, mock.ExpectationsWereMet())
		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 1)
		assert.Equal(t, model.LabelSet{"op": OP_DATA_LOCKS}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" waiting_digest="abc123" waiting_digest_text="SELECT * FROM users WHERE id = ?" blocking_digest="def456" blocking_digest_text="UPDATE users SET name = ? WHERE id = ?" waiting_timer_wait="1500.000000ms" waiting_lock_time="1000.000000ms" blocking_timer_wait="2000.000000ms" blocking_lock_time="1700.000000ms"`, lokiEntries[0].Line)
	})

	t.Run("result set iteration error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki.NewCollectingHandler()

		collector, err := NewLocks(LocksArguments{
			DB:              db,
			CollectInterval: 10 * time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDataLocks).RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows(
				[]string{
					"waitingTimerWait",
					"waitingLockTime",
					"waitingDigest",
					"waitingDigestText",
					"blockingTimerWait",
					"blockingLockTime",
					"blockingDigest",
					"blockingDigestText",
				},
			).AddRow(
				1500000000000,
				1000000000000,
				"abc123",
				"SELECT * FROM users WHERE id = ?",
				2000000000000,
				1700000000000,
				"def456",
				"UPDATE users SET name = ? WHERE id = ?",
			).AddRow(
				2500000000000,
				2000000000000,
				"xyz789",
				"SELECT * FROM orders WHERE user_id = ?",
				3000000000000,
				2700000000000,
				"ghi012",
				"DELETE FROM sessions WHERE expired = ?",
			).RowError(1, fmt.Errorf("some error")), // error on second row
		)

		require.NoError(t, collector.Start(t.Context()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 2*time.Second, 50*time.Millisecond)

		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 1)
		assert.Equal(t, model.LabelSet{"op": OP_DATA_LOCKS}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" waiting_digest="abc123" waiting_digest_text="SELECT * FROM users WHERE id = ?" blocking_digest="def456" blocking_digest_text="UPDATE users SET name = ? WHERE id = ?" waiting_timer_wait="1500.000000ms" waiting_lock_time="1000.000000ms" blocking_timer_wait="2000.000000ms" blocking_lock_time="1700.000000ms"`, lokiEntries[0].Line)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
