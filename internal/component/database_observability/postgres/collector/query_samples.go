package collector

import (
	"context"
	"database/sql"
	"fmt"
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
	QuerySamplesCollector = "query_samples"
	OP_QUERY_SAMPLE       = "query_sample"
	OP_WAIT_EVENT         = "wait_event"
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
		s.backend_type != 'client backend' OR
		(
			s.pid != pg_backend_pid() AND
			coalesce(TRIM(s.query), '') != '' AND
			s.query_id != 0 AND
			s.state != 'idle'
		)
`

type QuerySamplesInfo struct {
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

type QuerySamplesArguments struct {
	DB                    *sql.DB
	CollectInterval       time.Duration
	EntryHandler          loki.EntryHandler
	Logger                log.Logger
	DisableQueryRedaction bool
}

type QuerySamples struct {
	dbConnection          *sql.DB
	collectInterval       time.Duration
	entryHandler          loki.EntryHandler
	disableQueryRedaction bool

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewQuerySamples(args QuerySamplesArguments) (*QuerySamples, error) {
	return &QuerySamples{
		dbConnection:          args.DB,
		collectInterval:       args.CollectInterval,
		entryHandler:          args.EntryHandler,
		disableQueryRedaction: args.DisableQueryRedaction,
		logger:                log.With(args.Logger, "collector", QuerySamplesCollector),
		running:               &atomic.Bool{},
	}, nil
}

func (c *QuerySamples) Name() string {
	return QuerySamplesCollector
}

func (c *QuerySamples) Start(ctx context.Context) error {
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

func (c *QuerySamples) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *QuerySamples) Stop() {
	c.cancel()
}

// calculateDuration returns a formatted duration string between a nullable time and current time
func calculateDuration(nullableTime sql.NullTime, currentTime time.Time) string {
	if nullableTime.Valid {
		return currentTime.Sub(nullableTime.Time).Round(time.Millisecond).String()
	}
	return ""
}

func (c *QuerySamples) fetchQuerySample(ctx context.Context) error {
	rows, err := c.dbConnection.QueryContext(ctx, selectPgStatActivity)
	if err != nil {
		return fmt.Errorf("failed to query pg_stat_activity: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		sample := QuerySamplesInfo{}
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
				`datname="%s" user="%s" backend_type="%s" state="%s" wait_time="%s" wait_event_type="%s" wait_event="%s" wait_event_name="%s" blocked_by_pids="%v" queryid="%d" query="%s" engine="postgres"`,
				sample.DatabaseName.String,
				sample.Username.String,
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

	return nil
}

func (c QuerySamples) validateQuerySample(sample QuerySamplesInfo) error {
	if sample.Query.Valid && sample.Query.String == "<insufficient privilege>" {
		return fmt.Errorf("insufficient privilege to access query. sample set: %+v", sample)
	}

	if !sample.DatabaseName.Valid {
		return fmt.Errorf("database name is not valid. sample set: %+v", sample)
	}

	return nil
}
