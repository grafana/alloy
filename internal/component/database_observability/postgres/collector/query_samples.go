package collector

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
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

const (
	stateActive = "active"
	stateIdle   = "idle"
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
		s.pid != pg_backend_pid() AND
		(
			s.backend_type != 'client backend' OR
			(				
				coalesce(TRIM(s.query), '') != '' AND
				s.query_id != 0
			)
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
	// keys that were already emitted (e.g., idle-only or after idle transition), to avoid duplicates
	emitted map[SampleKey]struct{}
}

// SampleKey uses (PID, QueryID, QueryStartNs) so concurrent executions of the same
// query across backends/transactions are uniquely tracked between scrapes.
type SampleKey struct {
	PID          int
	QueryID      int64
	QueryStartNs int64
}

func newSampleKey(pid int, queryID int64, queryStart sql.NullTime) SampleKey {
	key := SampleKey{PID: pid, QueryID: queryID, QueryStartNs: 0}
	if queryStart.Valid {
		key.QueryStartNs = queryStart.Time.UnixNano()
	}
	return key
}

// SampleState buffers state across scrapes and is emitted once the query
// turns idle or disappears, avoiding partial/duplicate emissions.
type SampleState struct {
	LastRow     QuerySamplesInfo
	LastSeenAt  time.Time
	LastCpuTime string // last cpu_time observed under CPU condition
	tracker     WaitEventTracker
}

// WaitEventTracker coalesces consecutive identical wait events
// to reduce log volume while preserving timing.
type WaitEventTracker struct {
	waitEvents []WaitEventOccurrence
	openIdx    int // -1 means none open
}

func newWaitEventTracker() WaitEventTracker {
	return WaitEventTracker{waitEvents: []WaitEventOccurrence{}, openIdx: -1}
}

func (t *WaitEventTracker) CloseOpen()                        { t.openIdx = -1 }
func (t *WaitEventTracker) WaitEvents() []WaitEventOccurrence { return t.waitEvents }

// WaitEventOccurrence tracks a continuous occurrence of the same wait event
// with the same blocked_by_pids set.
type WaitEventOccurrence struct {
	WaitEventType string
	WaitEvent     string
	BlockedByPIDs []int64 // normalized set (sorted, unique)
	LastWaitTime  string  // last stateDuration seen for this wait event
	LastState     string
	LastTimestamp time.Time
}

// WaitEventIdentity defines the identity of a wait-event occurrence (type, event, blocked_by set)
type WaitEventIdentity struct {
	eventType string
	event     string
	blockedBy []int64 // normalized
}

func (w WaitEventIdentity) Equal(other WaitEventIdentity) bool {
	if w.eventType != other.eventType || w.event != other.event {
		return false
	}
	return equalPIDSets(w.blockedBy, other.blockedBy)
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
		emitted:               map[SampleKey]struct{}{},
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

func (c *QuerySamples) fetchQuerySample(ctx context.Context) error {
	rows, err := c.dbConnection.QueryContext(ctx, selectPgStatActivity)
	if err != nil {
		return fmt.Errorf("failed to query pg_stat_activity: %w", err)
	}

	defer rows.Close()

	activeKeys := map[SampleKey]struct{}{}

	for rows.Next() {
		sample, scanErr := c.scanRow(rows)
		if scanErr != nil {
			level.Error(c.logger).Log("msg", "failed to scan pg_stat_activity", "err", scanErr)
			continue
		}

		key, procErr := c.processRow(sample)
		if procErr != nil {
			level.Debug(c.logger).Log("msg", "invalid pg_stat_activity set", "queryid", sample.QueryID.Int64, "err", procErr)
			continue
		}

		if !isIdleState(sample.State.String) {
			c.upsertActiveSample(key, sample)
			activeKeys[key] = struct{}{}
			continue
		}

		if _, hadActive := c.samples[key]; hadActive {
			c.emitAtIdleTransition(key, sample)
			activeKeys[key] = struct{}{}
			continue
		}

		if _, already := c.emitted[key]; !already {
			c.emitIdleOnlySample(key, sample)
			c.emitted[key] = struct{}{}
		}
	}

	if err := rows.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate pg_stat_activity rows", "err", err)
		return err
	}

	// finalize samples that are no longer active
	for key := range c.samples {
		if _, stillActive := activeKeys[key]; stillActive {
			continue
		}
		c.emitAndDeleteSample(key)
	}
	return nil
}

func (c *QuerySamples) scanRow(rows *sql.Rows) (QuerySamplesInfo, error) {
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
	return sample, err
}

func (c *QuerySamples) processRow(sample QuerySamplesInfo) (SampleKey, error) {
	if err := c.validateQuerySample(sample); err != nil {
		return SampleKey{}, err
	}
	key := newSampleKey(sample.PID, sample.QueryID.Int64, sample.QueryStart)
	return key, nil
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

func (c *QuerySamples) upsertActiveSample(key SampleKey, sample QuerySamplesInfo) {
	state, ok := c.samples[key]
	if !ok {
		state = &SampleState{tracker: newWaitEventTracker()}
		c.samples[key] = state
	}
	state.LastRow = sample
	state.LastSeenAt = sample.Now
	state.updateCpuTimeIfActive(sample)
	state.tracker.upsertWaitEvent(sample, sample.Now)
}

func (t *WaitEventTracker) upsertWaitEvent(sample QuerySamplesInfo, now time.Time) {
	if sample.WaitEventType.Valid && sample.WaitEvent.Valid {
		current := WaitEventIdentity{
			eventType: sample.WaitEventType.String,
			event:     sample.WaitEvent.String,
			blockedBy: normalizePIDs(sample.BlockedByPIDs),
		}
		if t.openIdx >= 0 {
			we := t.waitEvents[t.openIdx]
			existing := WaitEventIdentity{eventType: we.WaitEventType, event: we.WaitEvent, blockedBy: we.BlockedByPIDs}
			if existing.Equal(current) {
				we.LastWaitTime = calculateDuration(sample.StateChange, now)
				we.LastState = sample.State.String
				we.LastTimestamp = now
				t.waitEvents[t.openIdx] = we
				return
			}
			t.openIdx = -1
		}

		newOcc := WaitEventOccurrence{
			WaitEventType: current.eventType,
			WaitEvent:     current.event,
			BlockedByPIDs: current.blockedBy,
			LastWaitTime:  calculateDuration(sample.StateChange, now),
			LastState:     sample.State.String,
			LastTimestamp: now,
		}
		t.waitEvents = append(t.waitEvents, newOcc)
		t.openIdx = len(t.waitEvents) - 1
		return
	}

	if t.openIdx >= 0 {
		t.openIdx = -1
	}
}

func (c *QuerySamples) emitAndDeleteSample(key SampleKey) {
	state, ok := c.samples[key]
	if !ok {
		return
	}
	sampleLabels := c.buildQuerySampleLabels(state, nil)
	c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		OP_QUERY_SAMPLE,
		sampleLabels,
		state.LastSeenAt.UnixNano(),
	)

	for _, we := range state.tracker.WaitEvents() {
		if we.WaitEventType == "" || we.WaitEvent == "" {
			continue
		}
		waitEventLabels := c.buildWaitEventLabels(state, we)
		c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
			logging.LevelInfo,
			OP_WAIT_EVENT,
			waitEventLabels,
			we.LastTimestamp.UnixNano(),
		)
	}

	delete(c.samples, key)
}

// emitAtIdleTransition finalizes a running sample when its state becomes idle.
func (c *QuerySamples) emitAtIdleTransition(key SampleKey, idleRow QuerySamplesInfo) {
	state, ok := c.samples[key]
	if !ok {
		return
	}

	finalState := *state
	finalState.LastRow.State = sql.NullString{String: stateIdle, Valid: true}
	endTs := idleRow.StateChange
	if endTs.Valid {
		finalState.LastSeenAt = endTs.Time
	}

	var endOverride *time.Time
	if endTs.Valid {
		t := endTs.Time
		endOverride = &t
	}
	sampleLabels := c.buildQuerySampleLabels(&finalState, endOverride)
	c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		OP_QUERY_SAMPLE,
		sampleLabels,
		finalState.LastSeenAt.UnixNano(),
	)

	for _, we := range state.tracker.WaitEvents() {
		if we.WaitEventType == "" || we.WaitEvent == "" {
			continue
		}
		waitEventLabels := c.buildWaitEventLabels(state, we)
		c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
			logging.LevelInfo,
			OP_WAIT_EVENT,
			waitEventLabels,
			we.LastTimestamp.UnixNano(),
		)
	}

	delete(c.samples, key)
	c.emitted[key] = struct{}{}
}

// emitIdleOnlySample emits a one-off sample for a query that is only observed
// as idle (we didn't catch it while active).
func (c *QuerySamples) emitIdleOnlySample(key SampleKey, idleRow QuerySamplesInfo) {
	dummy := &SampleState{LastRow: idleRow, LastSeenAt: idleRow.Now, tracker: newWaitEventTracker()}
	endTs := idleRow.StateChange
	if endTs.Valid {
		dummy.LastSeenAt = endTs.Time
	}
	var endOverride *time.Time
	if endTs.Valid {
		t := endTs.Time
		endOverride = &t
	}
	labels := c.buildQuerySampleLabels(dummy, endOverride)
	c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		OP_QUERY_SAMPLE,
		labels,
		dummy.LastSeenAt.UnixNano(),
	)
}

func (s *SampleState) updateCpuTimeIfActive(sample QuerySamplesInfo) {
	if !sample.WaitEventType.Valid && !sample.WaitEvent.Valid && sample.State.String == stateActive {
		s.LastCpuTime = calculateDuration(sample.StateChange, sample.Now)
	}
}

func (c *QuerySamples) buildQuerySampleLabels(state *SampleState, endOverride *time.Time) string {
	leaderPID := ""
	if state.LastRow.LeaderPID.Valid {
		leaderPID = fmt.Sprintf(`%d`, state.LastRow.LeaderPID.Int64)
	}

	end := state.LastRow.Now
	if endOverride != nil {
		end = *endOverride
	}

	xactDuration := calculateDuration(state.LastRow.XactStart, end)
	queryDuration := calculateDuration(state.LastRow.QueryStart, end)

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

	labels := fmt.Sprintf(
		`datname="%s" pid="%d" leader_pid="%s" user="%s" app="%s" client="%s" backend_type="%s" state="%s" xid="%d" xmin="%d" xact_time="%s" query_time="%s" queryid="%d" query="%s" engine="postgres"`,
		state.LastRow.DatabaseName.String,
		state.LastRow.PID,
		leaderPID,
		state.LastRow.Username.String,
		state.LastRow.ApplicationName.String,
		clientAddr,
		state.LastRow.BackendType.String,
		state.LastRow.State.String,
		state.LastRow.BackendXID.Int32,
		state.LastRow.BackendXmin.Int32,
		xactDuration,
		queryDuration,
		state.LastRow.QueryID.Int64,
		queryText,
	)
	if state.LastCpuTime != "" {
		labels = fmt.Sprintf(`%s cpu_time="%s"`, labels, state.LastCpuTime)
	}
	return labels
}

func (c *QuerySamples) buildWaitEventLabels(state *SampleState, we WaitEventOccurrence) string {
	waitEventFullName := fmt.Sprintf("%s:%s", we.WaitEventType, we.WaitEvent)
	leaderPID := ""
	if state.LastRow.LeaderPID.Valid {
		leaderPID = fmt.Sprintf(`%d`, state.LastRow.LeaderPID.Int64)
	}
	queryText := state.LastRow.Query.String
	if !c.disableQueryRedaction {
		queryText = redact(queryText)
	}
	return fmt.Sprintf(
		`datname="%s" pid="%d" leader_pid="%s" user="%s" backend_type="%s" state="%s" xid="%d" xmin="%d" wait_time="%s" wait_event_type="%s" wait_event="%s" wait_event_name="%s" blocked_by_pids="%v" queryid="%d" query="%s" engine="postgres"`,
		state.LastRow.DatabaseName.String,
		state.LastRow.PID,
		leaderPID,
		state.LastRow.Username.String,
		state.LastRow.BackendType.String,
		state.LastRow.State.String,
		state.LastRow.BackendXID.Int32,
		state.LastRow.BackendXmin.Int32,
		we.LastWaitTime,
		we.WaitEventType,
		we.WaitEvent,
		waitEventFullName,
		we.BlockedByPIDs,
		state.LastRow.QueryID.Int64,
		queryText,
	)
}

func calculateDuration(nullableTime sql.NullTime, currentTime time.Time) string {
	if !nullableTime.Valid {
		return ""
	}
	return currentTime.Sub(nullableTime.Time).Round(time.Nanosecond).String()
}

func normalizePIDs(pids pq.Int64Array) []int64 {
	seen := make(map[int64]struct{}, len(pids))
	out := make([]int64, 0, len(pids))
	for _, pid := range pids {
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		out = append(out, pid)
	}
	slices.Sort(out)
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

func isIdleState(state string) bool {
	if state == stateIdle {
		return true
	}
	return strings.HasPrefix(strings.ToLower(state), "idle")
}
