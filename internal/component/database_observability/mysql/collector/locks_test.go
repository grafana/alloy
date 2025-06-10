package collector

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/database_observability"
)

func Test_QueryLocks(t *testing.T) {
	t.Run("no logs with no lock events", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := fake.NewClient(func() {})

		collector, err := NewLock(LockArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
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
					"blockingDigest",
					"blockingDigestText",
				},
			),
		)

		require.NoError(t, collector.Start(context.Background()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 0
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.NoError(t, mock.ExpectationsWereMet())
		lokiEntries := lokiClient.Received()
		assert.Empty(t, lokiEntries, "Expected no log entries for no lock events")
	})

	t.Run("data lock is logged", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := fake.NewClient(func() {})

		collector, err := NewLock(LockArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
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
					"blockingDigest",
					"blockingDigestText",
				},
			).AddRow(
				1500000000000,
				1000000000000,
				"abc123",
				"SELECT * FROM users WHERE id = ?",
				2000000000000,
				"def456",
				"UPDATE users SET name = ? WHERE id = ?",
			),
		)

		require.NoError(t, collector.Start(context.Background()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.NoError(t, mock.ExpectationsWereMet())
		lokiEntries := lokiClient.Received()
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATA_LOCKS, "instance": "mysql-db"}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" waiting_digest="abc123" waiting_digest_text="SELECT * FROM users WHERE id = ?" blocking_digest="def456" blocking_digest_text="UPDATE users SET name = ? WHERE id = ?" waiting_timer_wait="1500.000000 ms" blocking_timer_wait="2000.000000 ms"`, lokiEntries[0].Line)
	})

	t.Run("multiple data locks are logged", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := fake.NewClient(func() {})

		collector, err := NewLock(LockArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
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
					"blockingDigest",
					"blockingDigestText",
				},
			).AddRow(
				1500000000000,
				1000000000000,
				"abc123",
				"SELECT * FROM users WHERE id = ?",
				2000000000000,
				"def456",
				"UPDATE users SET name = ? WHERE id = ?",
			).AddRow(
				2500000000000,
				2000000000000,
				"xyz789",
				"SELECT * FROM orders WHERE user_id = ?",
				3000000000000,
				"ghi012",
				"DELETE FROM sessions WHERE expired = ?",
			),
		)

		require.NoError(t, collector.Start(context.Background()))

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 2*time.Second, 50*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.NoError(t, mock.ExpectationsWereMet())
		lokiEntries := lokiClient.Received()
		require.Len(t, lokiEntries, 2)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATA_LOCKS, "instance": "mysql-db"}, lokiEntries[0].Labels)
		assert.Equal(t, `level="info" waiting_digest="abc123" waiting_digest_text="SELECT * FROM users WHERE id = ?" blocking_digest="def456" blocking_digest_text="UPDATE users SET name = ? WHERE id = ?" waiting_timer_wait="1500.000000 ms" blocking_timer_wait="2000.000000 ms"`, lokiEntries[0].Line)
		assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATA_LOCKS, "instance": "mysql-db"}, lokiEntries[1].Labels)
		assert.Equal(t, `level="info" waiting_digest="xyz789" waiting_digest_text="SELECT * FROM orders WHERE user_id = ?" blocking_digest="ghi012" blocking_digest_text="DELETE FROM sessions WHERE expired = ?" waiting_timer_wait="2500.000000 ms" blocking_timer_wait="3000.000000 ms"`, lokiEntries[1].Line)
	})
}
