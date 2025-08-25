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
	QuerySampleName = "query_sample"
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

type QuerySampleInfo struct {
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

type QuerySampleArguments struct {
	DB                    *sql.DB
	CollectInterval       time.Duration
	EntryHandler          loki.EntryHandler
	Logger                log.Logger
	DisableQueryRedaction bool
}

type QuerySample struct {
	dbConnection          *sql.DB
	collectInterval       time.Duration
	entryHandler          loki.EntryHandler
	disableQueryRedaction bool

	logger     log.Logger
	running    *atomic.Bool
	ctx        context.Context
	cancel     context.CancelFunc
	lastScrape time.Time
}

func NewQuerySample(args QuerySampleArguments) (*QuerySample, error) {
	return &QuerySample{
		dbConnection:          args.DB,
		collectInterval:       args.CollectInterval,
		entryHandler:          args.EntryHandler,
		disableQueryRedaction: args.DisableQueryRedaction,
		logger:                log.With(args.Logger, "collector", QuerySampleName),
		running:               &atomic.Bool{},
	}, nil
}

func (c *QuerySample) Name() string {
	return QuerySampleName
}

func (c *QuerySample) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", QuerySampleName+" collector started")

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
			if err := c.fetchQuerySample(c.ctx); err != nil {
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

func (c *QuerySample) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *QuerySample) Stop() {
	c.cancel()
}

// calculateDuration returns a formatted duration string between a nullable time and current time
func calculateDuration(nullableTime sql.NullTime, currentTime time.Time) string {
	if nullableTime.Valid {
		return currentTime.Sub(nullableTime.Time).Round(time.Millisecond).String()
	}
	return ""
}

func (c *QuerySample) fetchQuerySample(ctx context.Context) error {
	slog.Debug("Fetching sample")
	scrapeTime := time.Now()
	rows, err := c.dbConnection.QueryContext(ctx, selectPgStatActivity, c.lastScrape)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query pg_stat_activity", "err", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		sample := QuerySampleInfo{}
		err := rows.Scan(
			&sample.Now,
			&sample.DatabaseName,
			&sample.PID,
			&sample.LeaderPID,
			&sample.Username,
			&sample.ApplicationName,
			&sample.ClientAddr,
			&sample.ClientPort,
			&sample.BackendType,
			&sample.BackendStart,
			&sample.BackendXID,
			&sample.BackendXmin,
			&sample.XactStart,
			&sample.State,
			&sample.StateChange,
			&sample.WaitEventType,
			&sample.WaitEvent,
			&sample.BlockedByPIDs,
			&sample.QueryStart,
			&sample.QueryID,
			&sample.Query,
		)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to scan pg_stat_activity", "err", err)
			continue
		}

		err = c.validateQuerySample(sample)
		if err != nil {
			level.Debug(c.logger).Log("msg", "invalid pg_stat_activity set", "queryid", sample.QueryID.Int64, "err", err)
			continue
		}

		leaderPID := ""
		if sample.LeaderPID.Valid {
			leaderPID = fmt.Sprintf(`%d`, sample.LeaderPID.Int64)
		}

		stateDuration := calculateDuration(sample.StateChange, sample.Now)
		queryDuration := calculateDuration(sample.QueryStart, sample.Now)
		xactDuration := calculateDuration(sample.XactStart, sample.Now)
		backendDuration := calculateDuration(sample.BackendStart, sample.Now)

		clientAddr := ""
		if sample.ClientAddr.Valid {
			clientAddr = sample.ClientAddr.String
			if sample.ClientPort.Valid {
				clientAddr = fmt.Sprintf("%s:%d", clientAddr, sample.ClientPort.Int32)
			}
		}

		waitEventFullName := ""
		waitEvent := sample.WaitEvent.String
		waitEventType := sample.WaitEventType.String
		if sample.WaitEventType.Valid && sample.WaitEvent.Valid {
			waitEventFullName = fmt.Sprintf("%s:%s", sample.WaitEventType.String, sample.WaitEvent.String)
		}

		// Get query string and redact if needed
		queryText := sample.Query.String
		if !c.disableQueryRedaction {
			queryText = redact(queryText)
		}

		// Build query sample entry
		sampleLabels := fmt.Sprintf(
			`datname="%s" pid="%d" leader_pid="%s" user="%s" app="%s" client="%s" backend_type="%s" backend_time="%s" xid="%d" xmin="%d" xact_time="%s" state="%s" query_time="%s" queryid="%d" query="%s" engine="postgres"`,
			sample.DatabaseName.String,
			sample.PID,
			leaderPID,
			sample.Username.String,
			sample.ApplicationName.String,
			clientAddr,
			sample.BackendType.String,
			backendDuration,
			sample.BackendXID.Int32,
			sample.BackendXmin.Int32,
			xactDuration,
			sample.State.String,
			queryDuration,
			sample.QueryID.Int64,
			queryText,
		)

		if !sample.WaitEventType.Valid && !sample.WaitEvent.Valid && sample.State.String == "active" {
			// If the wait event is null and the state is active, it means the query is executing on CPU
			// Log it as a cpu_time within the query sample op
			sampleLabels = fmt.Sprintf(`%s cpu_time="%s"`, sampleLabels, stateDuration)
		}

		c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
			logging.LevelInfo,
			OP_QUERY_SAMPLE,
			sampleLabels,
			sample.Now.UnixNano(),
		)

		if waitEvent != "" {
			waitEventLabels := fmt.Sprintf(
				`datname="%s" backend_type="%s" state="%s" wait_time="%s" wait_event_type="%s" wait_event="%s" wait_event_name="%s" blocked_by_pids="%v" queryid="%d" query="%s" engine="postgres"`,
				sample.DatabaseName.String,
				sample.BackendType.String,
				sample.State.String,
				stateDuration,
				waitEventType,
				waitEvent,
				waitEventFullName,
				sample.BlockedByPIDs,
				sample.QueryID.Int64,
				queryText,
			)

			c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
				logging.LevelInfo,
				OP_WAIT_EVENT,
				waitEventLabels,
				sample.Now.UnixNano(),
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

func (c QuerySample) validateQuerySample(sample QuerySampleInfo) error {
	if sample.Query.Valid && sample.Query.String == "<insufficient privilege>" {
		return fmt.Errorf("insufficient privilege to access query. sample set: %+v", sample)
	}

	if !sample.DatabaseName.Valid {
		return fmt.Errorf("database name is not valid. sample set: %+v", sample)
	}

	return nil
}
