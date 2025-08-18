package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-kit/log"
	"github.com/lib/pq"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	OP_QUERY_SAMPLE = "query_sample"
	OP_WAIT_EVENT   = "wait_event"
	ActivityName    = "activity"
)

const selectPgStatActivity = `
	SELECT 
		clock_timestamp() as now,
		d.datname,		
		s.pid,
		s.leader_pid,
		s.usename,		
		s.application_name,
		s.client_addr,
		s.client_port,
		s.backend_type,
		s.backend_start,
		s.backend_xid,
		s.backend_xmin,		
		s.xact_start,
		s.state,
		s.state_change,		
		s.wait_event_type,
		s.wait_event,
		pg_blocking_pids(s.pid) as blocked_by_pids,
		s.query_start,
		s.query_id,
		s.query
	FROM pg_stat_activity s 
		JOIN pg_database d ON s.datid = d.oid AND NOT d.datistemplate AND d.datallowconn
	WHERE 
		s.pid <> pg_backend_pid() AND
		(
			s.backend_type != 'client backend' OR
			(
				coalesce(TRIM(s.query), '') != '' AND s.query_start IS NOT NULL AND 
				(
					s.state != 'idle' OR 
					(s.state = 'idle' AND s.state_change > $1)
				) AND 
				coalesce(TRIM(s.state), '') != ''
			)
		)
		AND query_id > 0
`

type ActivityInfo struct {
	DatabaseName    sql.NullString
	DatabaseID      int
	PID             int
	LeaderPID       sql.NullInt64
	UserSysID       int
	Username        sql.NullString
	ApplicationName sql.NullString
	ClientAddr      sql.NullString
	ClientPort      sql.NullInt32
	StateChange     sql.NullTime
	Now             time.Time
	BackendStart    sql.NullTime
	XactStart       sql.NullTime
	QueryStart      sql.NullTime
	WaitEventType   sql.NullString
	WaitEvent       sql.NullString
	State           sql.NullString
	BackendType     sql.NullString
	BackendXID      sql.NullInt32
	BackendXmin     sql.NullInt32
	QueryID         sql.NullInt64
	Query           sql.NullString
	BlockedByPIDs   pq.Int64Array
}

type ActivityArguments struct {
	DB              *sql.DB
	InstanceKey     string
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler
	Logger          log.Logger
}

type Activity struct {
	dbConnection    *sql.DB
	instanceKey     string
	collectInterval time.Duration
	entryHandler    loki.EntryHandler

	logger     log.Logger
	running    *atomic.Bool
	ctx        context.Context
	cancel     context.CancelFunc
	lastScrape time.Time
}

func NewActivity(args ActivityArguments) (*Activity, error) {
	return &Activity{
		dbConnection:    args.DB,
		instanceKey:     args.InstanceKey,
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		logger:          log.With(args.Logger, "collector", ActivityName),
		running:         &atomic.Bool{},
	}, nil
}

func (c *Activity) Name() string {
	return ActivityName
}

func (c *Activity) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", ActivityName+" collector started")

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
			if err := c.fetchActivity(c.ctx); err != nil {
				level.Error(c.logger).Log("msg", "collector error", "err", err)
			}

			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	}()

	return nil
}

func (c *Activity) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *Activity) Stop() {
	c.cancel()
}

// calculateDuration returns a formatted duration string between a nullable time and current time
func calculateDuration(nullableTime sql.NullTime, currentTime time.Time) string {
	if nullableTime.Valid {
		return currentTime.Sub(nullableTime.Time).Round(time.Millisecond).String()
	}
	return ""
}

func (c *Activity) fetchActivity(ctx context.Context) error {
	slog.Debug("Fetching activity")
	scrapeTime := time.Now()
	rows, err := c.dbConnection.QueryContext(ctx, selectPgStatActivity, c.lastScrape)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query pg_stat_activity", "err", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		activity := ActivityInfo{}
		err := rows.Scan(
			&activity.Now,
			&activity.DatabaseName,
			&activity.PID,
			&activity.LeaderPID,
			&activity.Username,
			&activity.ApplicationName,
			&activity.ClientAddr,
			&activity.ClientPort,
			&activity.BackendType,
			&activity.BackendStart,
			&activity.BackendXID,
			&activity.BackendXmin,
			&activity.XactStart,
			&activity.State,
			&activity.StateChange,
			&activity.WaitEventType,
			&activity.WaitEvent,
			&activity.BlockedByPIDs,
			&activity.QueryStart,
			&activity.QueryID,
			&activity.Query,
		)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan pg_stat_activity", "err", err)
			continue
		}

		err = c.validateActivity(activity)
		if err != nil {
			level.Debug(c.logger).Log("msg", "invalid pg_stat_activity set", "queryid", activity.QueryID.Int64, "err", err)
			continue
		}

		leaderPID := ""
		if activity.LeaderPID.Valid {
			leaderPID = fmt.Sprintf(`%d`, activity.LeaderPID.Int64)
		}

		stateDuration := calculateDuration(activity.StateChange, activity.Now)
		queryDuration := calculateDuration(activity.QueryStart, activity.Now)
		xactDuration := calculateDuration(activity.XactStart, activity.Now)
		backendDuration := calculateDuration(activity.BackendStart, activity.Now)

		clientAddr := ""
		if activity.ClientAddr.Valid {
			clientAddr = activity.ClientAddr.String
			if activity.ClientPort.Valid {
				clientAddr = fmt.Sprintf("%s:%d", clientAddr, activity.ClientPort.Int32)
			}
		}

		waitEventFullName := ""
		waitEvent := activity.WaitEvent.String
		waitEventType := activity.WaitEventType.String
		if activity.WaitEventType.Valid && activity.WaitEvent.Valid {
			waitEventFullName = fmt.Sprintf("%s:%s", activity.WaitEventType.String, activity.WaitEvent.String)
		}

		// Build query sample entry
		sampleLabels := fmt.Sprintf(
			`instance="%s" datname="%s" pid="%d" leader_pid="%s" user="%s" app="%s" client="%s" backend_type="%s" backend_time="%s" xid="%d" xmin="%d" xact_time="%s" state="%s" query_time="%s" queryid="%d" query="%s" engine="postgres"`,
			c.instanceKey,
			activity.DatabaseName.String,
			activity.PID,
			leaderPID,
			activity.Username.String,
			activity.ApplicationName.String,
			clientAddr,
			activity.BackendType.String,
			backendDuration,
			activity.BackendXID.Int32,
			activity.BackendXmin.Int32,
			xactDuration,
			activity.State.String,
			queryDuration,
			activity.QueryID.Int64,
			activity.Query.String,
		)

		if !activity.WaitEventType.Valid && !activity.WaitEvent.Valid && activity.State.String == "active" {
			// If the wait event is null and the state is active, it means the query is executing on CPU
			// Log it as a cpu_time within the query sample op
			sampleLabels = fmt.Sprintf(`%s cpu_time="%s"`, sampleLabels, stateDuration)
		}

		c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
			logging.LevelInfo,
			OP_QUERY_SAMPLE,
			c.instanceKey,
			sampleLabels,
			activity.Now.UnixNano(),
		)

		if waitEvent != "" {
			waitEventLabels := fmt.Sprintf(
				`instance="%s" datname="%s" backend_type="%s" state="%s" wait_time="%s" wait_event_type="%s" wait_event="%s" wait_event_name="%s" blocked_by_pids="%v" queryid="%d" query="%s" engine="postgres"`,
				c.instanceKey,
				activity.DatabaseName.String,
				activity.BackendType.String,
				activity.State.String,
				stateDuration,
				waitEventType,
				waitEvent,
				waitEventFullName,
				activity.BlockedByPIDs,
				activity.QueryID.Int64,
				activity.Query.String,
			)

			c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
				logging.LevelInfo,
				OP_WAIT_EVENT,
				c.instanceKey,
				waitEventLabels,
				activity.Now.UnixNano(),
			)
		}
	}

	if err := rows.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate pg_stat_activity rows", "err", err)
		return err
	}

	// Update last scrape time after successful scrape
	c.lastScrape = scrapeTime

	return nil
}

func (c Activity) validateActivity(activity ActivityInfo) error {
	if activity.Query.Valid && activity.Query.String == "<insufficient privilege>" {
		return fmt.Errorf("insufficient privilege to access query. activity set: %+v", activity)
	}

	if !activity.DatabaseName.Valid {
		return fmt.Errorf("database name is not valid. activity set: %+v", activity)
	}

	return nil
}
