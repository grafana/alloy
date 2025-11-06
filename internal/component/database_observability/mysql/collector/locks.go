package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	LocksCollector  = "locks"
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
				AND blocking_stmt_current.EVENT_ID < blocking_lock.EVENT_ID`
)

type LocksArguments struct {
	DB                *sql.DB
	CollectInterval   time.Duration
	LockWaitThreshold time.Duration
	EntryHandler      loki.EntryHandler

	Logger log.Logger
}

type Locks struct {
	mySQLClient     *sql.DB
	collectInterval time.Duration
	logger          log.Logger
	entryHandler    loki.EntryHandler

	// The minimum amount of time elapsed waiting due to a lock
	// to be selected for scrape
	lockTimeThreshold time.Duration
	running           *atomic.Bool
	ctx               context.Context
	cancel            context.CancelFunc
}

func (c *Locks) Name() string {
	return LocksCollector
}

func NewLocks(args LocksArguments) (*Locks, error) {
	if args.DB == nil {
		return nil, errors.New("nil DB connection")
	}

	if err := args.DB.Ping(); err != nil {
		return nil, err
	}

	return &Locks{
		mySQLClient:       args.DB,
		collectInterval:   args.CollectInterval,
		lockTimeThreshold: args.LockWaitThreshold,
		entryHandler:      args.EntryHandler,
		logger:            log.With(args.Logger, "collector", LocksCollector),
		running:           &atomic.Bool{},
	}, nil
}

func (c *Locks) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "collector started")

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	go func() {
		defer func() {
			c.Stop()
			c.running.Store(false)
		}()

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

func (c *Locks) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *Locks) Stop() {
	c.cancel()
}

func (c *Locks) fetchLocks(ctx context.Context) error {
	rsdl, err := c.mySQLClient.QueryContext(ctx, selectDataLocks)
	if err != nil {
		return fmt.Errorf("failed to query data locks: %w", err)
	}
	defer rsdl.Close()

	for rsdl.Next() {
		var waitingTimerWait, waitingLockTime, blockingTimerWait, blockingLockTime float64
		var waitingDigest, waitingDigestText, blockingDigest, blockingDigestText sql.NullString

		err := rsdl.Scan(&waitingTimerWait, &waitingLockTime, &waitingDigest, &waitingDigestText,
			&blockingTimerWait, &blockingLockTime, &blockingDigest, &blockingDigestText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan data locks", "err", err)
			continue
		}

		// only log if the lock_time is longer than the threshold
		if waitingLockTime > secondsToPicoseconds(c.lockTimeThreshold.Seconds()) {
			lockMsg := fmt.Sprintf(
				`waiting_digest="%s" waiting_digest_text="%s" blocking_digest="%s" blocking_digest_text="%s" waiting_timer_wait="%fms" waiting_lock_time="%fms" blocking_timer_wait="%fms" blocking_lock_time="%fms"`,
				waitingDigest.String,
				waitingDigestText.String,
				blockingDigest.String,
				blockingDigestText.String,
				picosecondsToMilliseconds(waitingTimerWait),
				picosecondsToMilliseconds(waitingLockTime),
				picosecondsToMilliseconds(blockingTimerWait),
				picosecondsToMilliseconds(blockingLockTime),
			)

			c.entryHandler.Chan() <- database_observability.BuildLokiEntry(logging.LevelInfo, OP_DATA_LOCKS, lockMsg)
		}
	}

	if err := rsdl.Err(); err != nil {
		return fmt.Errorf("failed to iterate over locks result set: %w", err)
	}

	return nil
}
