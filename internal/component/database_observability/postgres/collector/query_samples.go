package collector

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
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

// SampleKey uniquely identifies a running sample (query execution instance)
// while it is in a non-idle state.
type SampleKey struct {
	PID     int
	QueryID int64
	XID     int32
}

// WaitEventOccurrence tracks a continuous occurrence of the same wait event
// with the same blocked_by_pids set.
type WaitEventOccurrence struct {
	WaitEventType string
	WaitEvent     string
	BlockedByPIDs []int64 // normalized set (sorted, unique)
	LastWaitTime  string  // last stateDuration seen for this occurrence
	LastState     string
	LastTimestamp time.Time
}

// SampleState holds the in-memory buffered state for a running sample.
type SampleState struct {
	LastRow        QuerySamplesInfo
	LastSeenAt     time.Time
	LastCpuTime    string // last cpu_time observed under CPU condition
	WaitEvents     []WaitEventOccurrence
	OpenOccurrence *int // index into WaitEvents slice; nil if none open
}

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

	// in-memory state of running samples
	samples map[SampleKey]*SampleState
}

func NewQuerySamples(args QuerySamplesArguments) (*QuerySamples, error) {
	return &QuerySamples{
		dbConnection:          args.DB,
		collectInterval:       args.CollectInterval,
		entryHandler:          args.EntryHandler,
		disableQueryRedaction: args.DisableQueryRedaction,
		logger:                log.With(args.Logger, "collector", QuerySamplesCollector),
		running:               &atomic.Bool{},
		samples:               map[SampleKey]*SampleState{},
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

	activeKeys := map[SampleKey]struct{}{}
	idleKeys := map[SampleKey]struct{}{}

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

		key := SampleKey{PID: sample.PID, QueryID: sample.QueryID.Int64, XID: sample.BackendXID.Int32}
		if sample.State.Valid && sample.State.String == "idle" {
			// Update last snapshot and mark for finalization
			c.upsertSampleIdle(key, sample)
			idleKeys[key] = struct{}{}
			continue
		}

		// Process active row (state != 'idle')
		c.upsertSampleActive(key, sample)
		activeKeys[key] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate pg_stat_activity rows", "err", err)
		return err
	}

	// Finalize samples that turned idle in this scrape
	for key := range idleKeys {
		if state, ok := c.samples[key]; ok {
			c.emitAndDeleteSample(key, state)
		}
	}

	// Finalize samples that have disappeared (not seen as active this scrape)
	for key, state := range c.samples {
		if _, stillActive := activeKeys[key]; stillActive {
			continue
		}
		// Skip ones already finalized due to idle
		if _, wasIdle := idleKeys[key]; wasIdle {
			continue
		}
		c.emitAndDeleteSample(key, state)
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

// --- In-memory buffering helpers ---

// normalizePIDs returns a sorted unique slice for stable set comparisons/printing.
func normalizePIDs(pids pq.Int64Array) []int64 {
	set := map[int64]struct{}{}
	for _, pid := range pids {
		set[int64(pid)] = struct{}{}
	}
	out := make([]int64, 0, len(set))
	for pid := range set {
		out = append(out, pid)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func equalPIDSets(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (c *QuerySamples) upsertSampleActive(key SampleKey, sample QuerySamplesInfo) {
	// Upsert state
	state, ok := c.samples[key]
	if !ok {
		state = &SampleState{}
		c.samples[key] = state
	}
	state.LastRow = sample
	state.LastSeenAt = sample.Now

	// CPU condition: no wait event and state == "active"
	if !sample.WaitEventType.Valid && !sample.WaitEvent.Valid && sample.State.String == "active" {
		state.LastCpuTime = calculateDuration(sample.StateChange, sample.Now)
	}

	// Wait event occurrences
	if sample.WaitEventType.Valid && sample.WaitEvent.Valid {
		currentType := sample.WaitEventType.String
		currentEvent := sample.WaitEvent.String
		normalized := normalizePIDs(sample.BlockedByPIDs)

		if state.OpenOccurrence != nil {
			idx := *state.OpenOccurrence
			occ := state.WaitEvents[idx]
			if occ.WaitEventType == currentType && occ.WaitEvent == currentEvent && equalPIDSets(occ.BlockedByPIDs, normalized) {
				// continue same occurrence; update last values
				occ.LastWaitTime = calculateDuration(sample.StateChange, sample.Now)
				occ.LastState = sample.State.String
				occ.LastTimestamp = sample.Now
				state.WaitEvents[idx] = occ
				return
			}
			// close current occurrence
			state.OpenOccurrence = nil
		}

		// start new occurrence
		newOcc := WaitEventOccurrence{
			WaitEventType: currentType,
			WaitEvent:     currentEvent,
			BlockedByPIDs: normalized,
			LastWaitTime:  calculateDuration(sample.StateChange, sample.Now),
			LastState:     sample.State.String,
			LastTimestamp: sample.Now,
		}
		state.WaitEvents = append(state.WaitEvents, newOcc)
		idx := len(state.WaitEvents) - 1
		state.OpenOccurrence = &idx
		return
	}

	// No wait event on this row; close any open occurrence
	if state.OpenOccurrence != nil {
		state.OpenOccurrence = nil
	}
}

func (c *QuerySamples) upsertSampleIdle(key SampleKey, sample QuerySamplesInfo) {
	state, ok := c.samples[key]
	if !ok {
		state = &SampleState{}
		c.samples[key] = state
	}
	state.LastRow = sample
	state.LastSeenAt = sample.Now
	// Close any open occurrence
	if state.OpenOccurrence != nil {
		state.OpenOccurrence = nil
	}
}

// emitAndDeleteSample builds final entries for a sample and removes it from memory.
func (c *QuerySamples) emitAndDeleteSample(key SampleKey, state *SampleState) {
	// Build OP_QUERY_SAMPLE labels using the last snapshot
	leaderPID := ""
	if state.LastRow.LeaderPID.Valid {
		leaderPID = fmt.Sprintf(`%d`, state.LastRow.LeaderPID.Int64)
	}
	backendDuration := calculateDuration(state.LastRow.BackendStart, state.LastRow.Now)
	xactDuration := calculateDuration(state.LastRow.XactStart, state.LastRow.Now)
	queryDuration := calculateDuration(state.LastRow.QueryStart, state.LastRow.Now)

	clientAddr := ""
	if state.LastRow.ClientAddr.Valid {
		clientAddr = state.LastRow.ClientAddr.String
		if state.LastRow.ClientPort.Valid {
			clientAddr = fmt.Sprintf("%s:%d", clientAddr, state.LastRow.ClientPort.Int32)
		}
	}

	queryText := state.LastRow.Query.String
	if !c.disableQueryRedaction {
		queryText = redact(queryText)
	}

	sampleLabels := fmt.Sprintf(
		`datname="%s" pid="%d" leader_pid="%s" user="%s" app="%s" client="%s" backend_type="%s" backend_time="%s" xid="%d" xmin="%d" xact_time="%s" state="%s" query_time="%s" queryid="%d" query="%s" engine="postgres"`,
		state.LastRow.DatabaseName.String,
		state.LastRow.PID,
		leaderPID,
		state.LastRow.Username.String,
		state.LastRow.ApplicationName.String,
		clientAddr,
		state.LastRow.BackendType.String,
		backendDuration,
		state.LastRow.BackendXID.Int32,
		state.LastRow.BackendXmin.Int32,
		xactDuration,
		state.LastRow.State.String,
		queryDuration,
		state.LastRow.QueryID.Int64,
		queryText,
	)

	// Append cpu_time if observed at least once during this sample
	if state.LastCpuTime != "" {
		sampleLabels = fmt.Sprintf(`%s cpu_time="%s"`, sampleLabels, state.LastCpuTime)
	}

	c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		OP_QUERY_SAMPLE,
		sampleLabels,
		state.LastSeenAt.UnixNano(),
	)

	// Emit OP_WAIT_EVENT entries for each occurrence
	for _, occ := range state.WaitEvents {
		if occ.WaitEventType == "" || occ.WaitEvent == "" {
			continue
		}
		waitEventFullName := fmt.Sprintf("%s:%s", occ.WaitEventType, occ.WaitEvent)
		waitEventLabels := fmt.Sprintf(
			`datname="%s" user="%s" backend_type="%s" state="%s" wait_time="%s" wait_event_type="%s" wait_event="%s" wait_event_name="%s" blocked_by_pids="%v" queryid="%d" query="%s" engine="postgres"`,
			state.LastRow.DatabaseName.String,
			state.LastRow.Username.String,
			state.LastRow.BackendType.String,
			occ.LastState,
			occ.LastWaitTime,
			occ.WaitEventType,
			occ.WaitEvent,
			waitEventFullName,
			occ.BlockedByPIDs,
			state.LastRow.QueryID.Int64,
			queryText,
		)

		c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
			logging.LevelInfo,
			OP_WAIT_EVENT,
			waitEventLabels,
			occ.LastTimestamp.UnixNano(),
		)
	}

	delete(c.samples, key)
}
