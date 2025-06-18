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
		SELECT 
			waiting_stmt_current.TIMER_WAIT waitingTimerWait,
			waiting_stmt_current.LOCK_TIME waitingLockTime,
			waiting_stmt_current.DIGEST waitingDigest,
			waiting_stmt_current.DIGEST_TEXT waitingDigestText,
			blocking_stmt_current.TIMER_WAIT blockingTimerWait,
			blocking_stmt_current.LOCK_TIME blockingLockTime,
			blocking_stmt_current.DIGEST blockingDigest,
			blocking_stmt_current.DIGEST_TEXT blockingDigestText
		FROM performance_schema.data_lock_waits lock_waits
		JOIN performance_schema.data_locks waiting_lock
			ON lock_waits.REQUESTING_ENGINE_LOCK_ID = waiting_lock.ENGINE_LOCK_ID
				AND lock_waits.ENGINE = waiting_lock.ENGINE
		JOIN performance_schema.events_statements_current waiting_stmt_current
			ON waiting_lock.thread_id = waiting_stmt_current.thread_id
				AND waiting_stmt_current.EVENT_ID < waiting_lock.EVENT_ID
		JOIN performance_schema.data_locks blocking_lock
			ON lock_waits.BLOCKING_ENGINE_LOCK_ID = blocking_lock.ENGINE_LOCK_ID
				AND lock_waits.ENGINE = blocking_lock.ENGINE
		JOIN performance_schema.events_statements_current blocking_stmt_current
			ON blocking_lock.thread_id = blocking_stmt_current.thread_id
				AND blocking_stmt_current.EVENT_ID < blocking_lock.EVENT_ID;`
)

type LockArguments struct {
	DB                *sql.DB
	InstanceKey       string
	CollectInterval   time.Duration
	LockWaitThreshold time.Duration
	EntryHandler      loki.EntryHandler

	running *atomic.Bool
	cancel  context.CancelFunc
	Logger  log.Logger
}

type LockCollector struct {
	mySQLClient     *sql.DB
	instanceKey     string
	collectInterval time.Duration
	logger          log.Logger
	entryHandler    loki.EntryHandler

	// The minimum amount of time elapsed waiting due to a lock
	// to be selected for scrape
	lockTimeThreshold time.Duration
	running           *atomic.Bool
	cancel            context.CancelFunc
}

func (c *LockCollector) Name() string {
	return LocksName
}

func (c *LockCollector) Stopped() bool {
	return !c.running.Load()
}

func NewLock(args LockArguments) (*LockCollector, error) {
	if args.DB == nil {
		return nil, errors.New("nil DB connection")
	}

	if err := args.DB.Ping(); err != nil {
		return nil, err
	}

	return &LockCollector{
		mySQLClient:       args.DB,
		instanceKey:       args.InstanceKey,
		collectInterval:   args.CollectInterval,
		lockTimeThreshold: args.LockWaitThreshold,
		entryHandler:      args.EntryHandler,
		logger:            log.With(args.Logger, "collector", LocksName),
		running:           &atomic.Bool{},
	}, nil
}

// Stop should be kept idempotent
func (c *LockCollector) Stop() {
	c.cancel()
}

func (c *LockCollector) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	go func() {
		defer func() {
			c.Stop()
			c.running.Store(false)
		}()
		level.Debug(c.logger).Log("msg", "collector started")
		ticker := time.NewTicker(c.collectInterval)

		for {
			if err := c.fetchLocks(ctx); err != nil {
				level.Error(c.logger).Log("msg", "collector error", "err", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	}()

	return nil
}

type dataLock struct {
	WaitingTimerWait   float64
	WaitingLockTime    float64
	WaitingDigest      string
	WaitingDigestText  string
	BlockingTimerWait  float64
	BlockingLockTime   float64
	BlockingDigest     string
	BlockingDigestText string
}

func (c *LockCollector) fetchLocks(ctx context.Context) error {
	rsdl, err := c.mySQLClient.QueryContext(ctx, selectDataLocks)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query data locks", "err", err)
		return err
	}
	defer rsdl.Close()

	for rsdl.Next() {
		var waitingTimerWait, waitingLockTime, blockingTimerWait, blockingLockTime float64
		var waitingDigest, waitingDigestText, blockingDigest, blockingDigestText string

		err := rsdl.Scan(&waitingTimerWait, &waitingLockTime, &waitingDigest, &waitingDigestText,
			&blockingTimerWait, &blockingLockTime, &blockingDigest, &blockingDigestText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan data locks", "err", err)
			continue
		}

		// only log if the lock_time is longer than the threshold
		if waitingLockTime > secondsToPicoseconds(c.lockTimeThreshold.Seconds()) {
			lockMsg := fmt.Sprintf(
				`waiting_digest="%s" waiting_digest_text="%s" blocking_digest="%s" blocking_digest_text="%s" waiting_timer_wait="%f ms" waiting_lock_time="%f ms" blocking_timer_wait="%f ms" blocking_lock_time="%f ms"`,
				waitingDigest,
				waitingDigestText,
				blockingDigest,
				blockingDigestText,
				picosecondsToMilliseconds(waitingTimerWait),
				picosecondsToMilliseconds(waitingLockTime),
				picosecondsToMilliseconds(blockingTimerWait),
				picosecondsToMilliseconds(blockingLockTime),
			)

			c.entryHandler.Chan() <- buildLokiEntry(logging.LevelInfo, OP_DATA_LOCKS, c.instanceKey, lockMsg)
		}
	}

	if err := rsdl.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over locks result set", "err", err)
		return err
	}

	return nil
}
