package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	OP_DATA_LOCKS     = "query_data_locks"
	OP_METADATA_LOCKS = "query_metadata_locks"
	selectDataLocks   = `
		SELECT wt.PROCESSLIST_ID waitingPid, wl.LOCK_TYPE waitingLockType, wl.LOCK_STATUS waitingLockStatus, wl.LOCK_MODE waitingLockMode,
			ws.TIMER_WAIT waitingTimeWait, ws.LOCK_TIME waitingLockTime, ws.DIGEST waitingDigest, ws.DIGEST_TEXT waitingDigestText,
			bt.PROCESSLIST_ID blockingPid, bl.LOCK_TYPE blockingLockType, bl.LOCK_STATUS blockingLockStatus, bl.LOCK_MODE blockingLockMode,
			bs.TIMER_WAIT blockingTimerWait, bs.LOCK_TIME blockingLockTime, bs.DIGEST blockingDigest, bs.DIGEST_TEXT blockingDigestText,
			wl.OBJECT_SCHEMA, wl.OBJECT_NAME, wl.PARTITION_NAME, wl.SUBPARTITION_NAME, wl.INDEX_NAME
		FROM performance_schema.data_lock_waits w
		JOIN performance_schema.data_locks wl
			ON w.REQUESTING_ENGINE_LOCK_ID = wl.ENGINE_LOCK_ID
				AND w.ENGINE = wl.ENGINE
		JOIN performance_schema.events_statements_current ws
			ON wl.thread_id = ws.thread_id
				AND ws.EVENT_ID < wl.EVENT_ID
		JOIN performance_schema.threads wt
			ON ws.thread_id = wt.thread_id
		JOIN performance_schema.data_locks bl
			ON w.BLOCKING_ENGINE_LOCK_ID = bl.ENGINE_LOCK_ID
				AND w.ENGINE = bl.ENGINE
		JOIN performance_schema.events_statements_current bs
			ON bl.thread_id = bs.thread_id
				AND bs.EVENT_ID < bl.EVENT_ID
		JOIN performance_schema.threads bt
			ON bs.thread_id = bt.thread_id;`
)

type dataLockInfo struct {
	WaitingPid         string `json:"waitingPid,omitempty"`
	WaitingLockType    string `json:"waitingLockType,omitempty"`
	WaitingLockStatus  string `json:"waitingLockStatus,omitempty"`
	WaitingLockMode    string `json:"waitingLockMode,omitempty"`
	WaitingTimeWait    int64  `json:"waitingTimeWait,omitempty"`
	WaitingLockTime    int64  `json:"waitingLockTime,omitempty"`
	WaitingDigest      string `json:"waitingDigest,omitempty"`
	WaitingDigestText  string `json:"waitingDigestText,omitempty"`
	BlockingPid        string `json:"blockingPid,omitempty"`
	BlockingLockType   string `json:"blockingLockType,omitempty"`
	BlockingLockStatus string `json:"blockingLockStatus,omitempty"`
	BlockingLockMode   string `json:"blockingLockMode,omitempty"`
	BlockingTimerWait  int64  `json:"blockingTimerWait,omitempty"`
	BlockingLockTime   int64  `json:"blockingLockTime,omitempty"`
	BlockingDigest     string `json:"blockingDigest,omitempty"`
	BlockingDigestText string `json:"blockingDigestText,omitempty"`
	ObjectSchema       string `json:"objectSchema,omitempty"`
	ObjectName         string `json:"objectName,omitempty"`
	PartitionName      string `json:"partitionName,omitempty"`
	SubpartitionName   string `json:"subpartitionName,omitempty"`
	IndexName          string `json:"indexName,omitempty"`
}

type LockArguments struct {
	DSN               string
	InstanceKey       string
	ScrapeInterval    time.Duration
	LockWaitThreshold time.Duration
}

type Lock struct {
	mySQLClient    *sql.DB
	instanceKey    string
	scrapeInterval time.Duration
	logger         log.Logger
	entryHandler   loki.EntryHandler

	// The minimum amount of time elapsed waiting due to a lock
	// to be selected for scrape
	lockWaitThreshold time.Duration
}

func NewLock(args LockArguments) (*Lock, error) {
	dbConnection, err := sql.Open("mysql", args.DSN)
	if err != nil {
		return nil, err
	}
	if dbConnection == nil {
		return nil, errors.New("nil DB connection")
	}

	if err = dbConnection.Ping(); err != nil {
		return nil, err
	}

	return &Lock{
		mySQLClient:       dbConnection,
		instanceKey:       args.InstanceKey,
		scrapeInterval:    args.ScrapeInterval,
		lockWaitThreshold: args.LockWaitThreshold,
	}, nil
}

func (c Lock) Run(ctx context.Context, wg *sync.WaitGroup) error {
	go func() {
		defer wg.Done()

		level.Info(c.logger).Log("Lock: starting")
		ticker := time.NewTicker(c.scrapeInterval)

		for {
			if err := c.fetchLocks(ctx); err != nil {
				break
			}

			select {
			case <-ctx.Done():
				level.Info(c.logger).Log("Lock: stopping")
				return
			case <-ticker.C:
				// continue loop
			}
		}
	}()

	return nil
}

func (c Lock) fetchLocks(ctx context.Context) error {
	rsdl, err := c.mySQLClient.QueryContext(ctx, selectDataLocks)
	if err != nil {
		level.Error(c.logger).Log("failed to query data locks", "err", err)
		return err
	}
	defer rsdl.Close()

	dataLocks := c.parseDataLocks(rsdl)

	for _, lock := range dataLocks {
		mLock, err := json.Marshal(lock)
		if err != nil {
			level.Error(c.logger).Log("failed to marshal data lock", "err", err)
		}

		// only log if the lock has been waiting for more than the threshold
		if lock.WaitingTimeWait > c.convertToPicoSeconds(int64(c.lockWaitThreshold.Seconds())) {
			lockMsg := fmt.Sprintf(
				`waiting_digest="%s" waiting_digest_text="%s" blocking_digest="%s" blocking_digest_text="%s" lock="%s"`,
				lock.WaitingDigest, lock.WaitingDigestText, lock.BlockingDigest, lock.BlockingDigestText, mLock)

			c.entryHandler.Chan() <- buildLokiEntry(logging.LevelInfo, // TODO: what could the timestamp be?
				OP_DATA_LOCKS, c.instanceKey, lockMsg)
		}
	}

	return nil
}

func (c Lock) parseDataLocks(rows *sql.Rows) []dataLockInfo {
	dataLocks := []dataLockInfo{}

	for rows.Next() {
		if err := rows.Err(); err != nil {
			level.Error(c.logger).Log("failed to iterate rows", "err", err)
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
			level.Error(c.logger).Log("failed to scan data locks", "err", err)
			break
		}

		lock := dataLockInfo{
			WaitingPid: waitingPid, WaitingLockType: waitingLockType, WaitingLockStatus: waitingLockStatus, WaitingLockMode: waitingLockMode,
			WaitingTimeWait: c.convertFromPicoSeconds(waitingTimeWait), WaitingLockTime: c.convertFromPicoSeconds(waitingLockTime), WaitingDigest: waitingDigest, WaitingDigestText: waitingDigestText,
			BlockingPid: blockingPid, BlockingLockType: blockingLockType, BlockingLockStatus: blockingLockStatus, BlockingLockMode: blockingLockMode,
			BlockingTimerWait: c.convertFromPicoSeconds(blockingTimerWait), BlockingLockTime: c.convertFromPicoSeconds(blockingLockTime), BlockingDigest: blockingDigest, BlockingDigestText: blockingDigestText,
			ObjectSchema: objectSchema, ObjectName: objectName, PartitionName: partitionName.String, SubpartitionName: subpartitionName.String, IndexName: indexName.String,
		}
		dataLocks = append(dataLocks, lock)
	}

	return dataLocks
}

func (c Lock) convertToPicoSeconds(s int64) int64 {
	return s * 1000000000000
}

func (c Lock) convertFromPicoSeconds(p int64) int64 {
	return p / 1000000000000
}
