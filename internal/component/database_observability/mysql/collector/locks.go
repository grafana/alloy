package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	LocksName       = "locks"
	OP_DATA_LOCKS   = "query_data_locks"
	selectDataLocks = `
		SELECT waiting_thread.PROCESSLIST_ID waitingPid, waiting_lock.LOCK_TYPE waitingLockType, waiting_lock.LOCK_STATUS waitingLockStatus, waiting_lock.LOCK_MODE waitingLockMode,
			waiting_stmt_current.TIMER_WAIT waitingTimerWait, waiting_stmt_current.LOCK_TIME waitingLockTime, waiting_stmt_current.DIGEST waitingDigest, waiting_stmt_current.DIGEST_TEXT waitingDigestText,
			blocking_thread.PROCESSLIST_ID blockingPid, blocking_lock.LOCK_TYPE blockingLockType, blocking_lock.LOCK_STATUS blockingLockStatus, blocking_lock.LOCK_MODE blockingLockMode,
			blocking_stmt_current.TIMER_WAIT blockingTimerWait, blocking_stmt_current.LOCK_TIME blockingLockTime, blocking_stmt_current.DIGEST blockingDigest, blocking_stmt_current.DIGEST_TEXT blockingDigestText,
			waiting_lock.OBJECT_SCHEMA, waiting_lock.OBJECT_NAME, waiting_lock.PARTITION_NAME, waiting_lock.SUBPARTITION_NAME, waiting_lock.INDEX_NAME
		FROM performance_schema.data_lock_waits lock_waits
		JOIN performance_schema.data_locks waiting_lock
			ON lock_waits.REQUESTING_ENGINE_LOCK_ID = waiting_lock.ENGINE_LOCK_ID
				AND lock_waits.ENGINE = waiting_lock.ENGINE
		JOIN performance_schema.events_statements_current waiting_stmt_current
			ON waiting_lock.thread_id = waiting_stmt_current.thread_id
				AND waiting_stmt_current.EVENT_ID < waiting_lock.EVENT_ID
		JOIN performance_schema.threads waiting_thread
			ON waiting_stmt_current.thread_id = waiting_thread.thread_id
		JOIN performance_schema.data_locks blocking_lock
			ON lock_waits.BLOCKING_ENGINE_LOCK_ID = blocking_lock.ENGINE_LOCK_ID
				AND lock_waits.ENGINE = blocking_lock.ENGINE
		JOIN performance_schema.events_statements_current blocking_stmt_current
			ON blocking_lock.thread_id = blocking_stmt_current.thread_id
				AND blocking_stmt_current.EVENT_ID < blocking_lock.EVENT_ID
		JOIN performance_schema.threads blocking_thread
			ON blocking_stmt_current.thread_id = blocking_thread.thread_id;`
)

type dataLockInfo struct {
	WaitingPid         string
	WaitingLockType    string
	WaitingLockStatus  string
	WaitingLockMode    string
	WaitingTimerWait   int64
	WaitingLockTime    int64
	WaitingDigest      string
	WaitingDigestText  string
	BlockingPid        string
	BlockingLockType   string
	BlockingLockStatus string
	BlockingLockMode   string
	BlockingTimerWait  int64
	BlockingLockTime   int64
	BlockingDigest     string
	BlockingDigestText string
	ObjectSchema       string
	ObjectName         string
	PartitionName      string
	SubpartitionName   string
	IndexName          string
}

type LockArguments struct {
	DB                *sql.DB
	InstanceKey       string
	CollectInterval   time.Duration
	LockWaitThreshold time.Duration // TODO: be sure to specify the units necessary
	EntryHandler      loki.EntryHandler

	running *atomic.Bool
	cancel  context.CancelFunc
	Logger  log.Logger
}

type Lock struct {
	mySQLClient     *sql.DB
	instanceKey     string
	collectInterval time.Duration
	logger          log.Logger
	entryHandler    loki.EntryHandler

	// The minimum amount of time elapsed waiting due to a lock
	// to be selected for scrape
	lockTimerWaitThreshold time.Duration
	running                *atomic.Bool
	cancel                 context.CancelFunc
}

func (c *Lock) Name() string {
	return LocksName
}

func (c *Lock) Stopped() bool {
	return !c.running.Load()
}

func NewLock(args LockArguments) (*Lock, error) {
	if args.DB == nil {
		return nil, errors.New("nil DB connection")
	}

	if err := args.DB.Ping(); err != nil {
		return nil, err
	}

	return &Lock{
		mySQLClient:            args.DB,
		instanceKey:            args.InstanceKey,
		collectInterval:        args.CollectInterval,
		lockTimerWaitThreshold: args.LockWaitThreshold,
		entryHandler:           args.EntryHandler,
		logger:                 log.With(args.Logger, "collector", LocksName),
		running:                &atomic.Bool{},
	}, nil
}

// Stop should be kept idempotent
func (c *Lock) Stop() {
	c.cancel()
}

func (c *Lock) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	go func() {
		defer func() {
			c.Stop()
			c.running.Store(false)
		}()
		level.Info(c.logger).Log("msg", "Lock: starting")
		ticker := time.NewTicker(time.Second)

		for {
			if err := c.fetchLocks(ctx); err != nil {
				level.Error(c.logger).Log("msg", "locks collector error", "err", err)

			}

			select {
			case <-ctx.Done():
				level.Info(c.logger).Log("msg", "Lock: stopping")
				return
			case <-ticker.C:
				// continue loop
			}
		}
	}()

	return nil
}

func (c *Lock) fetchLocks(ctx context.Context) error {
	rsdl, err := c.mySQLClient.QueryContext(ctx, selectDataLocks)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query data locks", "err", err)
		return err
	}
	defer rsdl.Close()

	dataLocks := c.parseDataLocks(rsdl)

	for _, lock := range dataLocks {
		// only log if the lock has been waiting for more than the threshold
		if lock.WaitingTimerWait > c.convertToPicoSeconds(int64(c.lockTimerWaitThreshold.Seconds())) || lock.WaitingLockTime > c.convertToPicoSeconds(int64(c.lockTimerWaitThreshold.Seconds())) {
			lockMsg := fmt.Sprintf(
				`waiting_digest="%s" waiting_digest_text="%s" blocking_digest="%s" blocking_digest_text="%s" waiting_timer_wait="%d ps" blocking_timer_wait="%d ps"`,
				lock.WaitingDigest, lock.WaitingDigestText, lock.BlockingDigest, lock.BlockingDigestText, lock.WaitingTimerWait, lock.BlockingTimerWait)

			c.entryHandler.Chan() <- buildLokiEntry(logging.LevelInfo, OP_DATA_LOCKS, c.instanceKey, lockMsg)
		}
	}

	return nil
}

func (c *Lock) parseDataLocks(rows *sql.Rows) []dataLockInfo {
	dataLocks := []dataLockInfo{}

	for rows.Next() {
		if err := rows.Err(); err != nil {
			level.Error(c.logger).Log("msg", "failed to iterate rows", "err", err)
			break
		}

		var waitingPid, waitingLockType, waitingLockStatus, waitingLockMode, waitingDigest, waitingDigestText,
			blockingPid, blockingLockType, blockingLockStatus, blockingLockMode, blockingDigest, blockingDigestText,
			objectSchema, objectName string
		var waitingTimeWait, blockingTimerWait, blockingLockTime, waitingLockTime int64
		var partitionName, subpartitionName, indexName sql.NullString

		err := rows.Scan(&waitingPid, &waitingLockType, &waitingLockStatus, &waitingLockMode,
			&waitingTimeWait, &waitingLockTime, &waitingDigest, &waitingDigestText,
			&blockingPid, &blockingLockType, &blockingLockStatus, &blockingLockMode,
			&blockingTimerWait, &blockingLockTime, &blockingDigest, &blockingDigestText,
			&objectSchema, &objectName, &partitionName, &subpartitionName, &indexName)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan data locks", "err", err)
			break
		}

		lock := dataLockInfo{
			WaitingPid: waitingPid, WaitingLockType: waitingLockType, WaitingLockStatus: waitingLockStatus, WaitingLockMode: waitingLockMode,
			WaitingTimerWait: waitingTimeWait, WaitingLockTime: waitingLockTime, WaitingDigest: waitingDigest, WaitingDigestText: waitingDigestText,
			BlockingPid: blockingPid, BlockingLockType: blockingLockType, BlockingLockStatus: blockingLockStatus, BlockingLockMode: blockingLockMode,
			BlockingTimerWait: blockingTimerWait, BlockingLockTime: blockingLockTime, BlockingDigest: blockingDigest, BlockingDigestText: blockingDigestText,
			ObjectSchema: objectSchema, ObjectName: objectName, PartitionName: partitionName.String, SubpartitionName: subpartitionName.String, IndexName: indexName.String,
		}
		dataLocks = append(dataLocks, lock)
	}

	return dataLocks
}

func (c *Lock) convertToPicoSeconds(s int64) int64 {
	return s * 1000000000000 // Convert seconds to picoseconds
}
