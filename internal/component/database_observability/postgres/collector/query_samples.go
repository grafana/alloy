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
		s.backend_type = 'client backend' AND
		s.pid != pg_backend_pid() AND
		coalesce(TRIM(s.query), '') != '' AND
		s.query_id != 0 AND
		s.state != 'idle'
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

// SampleKey uniquely identifies a running sample (query execution instance)
// while it is in a non-idle state.
type SampleKey struct {
	PID     int
	QueryID int64
	XID     int32
}

// SampleState holds the in-memory buffered state for a running sample.
type SampleState struct {
	LastRow     QuerySamplesInfo
	LastSeenAt  time.Time
	LastCpuTime string // last cpu_time observed under CPU condition
	tracker     WaitEventTracker
}

// WaitEventTracker manages a sequence of wait-event occurrences for a sample
type WaitEventTracker struct {
	waitEvents []WaitEventOccurrence
	openIdx    int // -1 means none open
}

func newWaitEventTracker() WaitEventTracker {
	return WaitEventTracker{waitEvents: []WaitEventOccurrence{}, openIdx: -1}
}

// CloseOpen closes any open wait event
func (t *WaitEventTracker) CloseOpen() { t.openIdx = -1 }

// WaitEvents returns the tracked wait events
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

// Normalize ensures the blockedBy set is sorted unique
func (w *WaitEventIdentity) Normalize() {
	w.blockedBy = normalizePIDs(pq.Int64Array(w.blockedBy))
}

// Equal compares identity ignoring order/duplicates of blocked_by set
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
	rows, err := c.queryPgStatActivity(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()

	activeKeys := map[SampleKey]struct{}{}
	idleKeys := map[SampleKey]struct{}{}

	for rows.Next() {
		sample, scanErr := c.scanRow(rows)
		if scanErr != nil {
			level.Error(c.logger).Log("msg", "failed to scan pg_stat_activity", "err", scanErr)
			continue
		}

		key, isIdle, procErr := c.processRow(sample)
		if procErr != nil {
			level.Debug(c.logger).Log("msg", "invalid pg_stat_activity set", "queryid", sample.QueryID.Int64, "err", procErr)
			continue
		}

		if isIdle {
			c.upsertIdleSample(key, sample)
			idleKeys[key] = struct{}{}
			continue
		}

		c.upsertActiveSample(key, sample)
		activeKeys[key] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate pg_stat_activity rows", "err", err)
		return err
	}

	c.finalizeSamples(activeKeys, idleKeys)
	return nil
}

// queryPgStatActivity executes the query against pg_stat_activity
func (c *QuerySamples) queryPgStatActivity(ctx context.Context) (*sql.Rows, error) {
	rows, err := c.dbConnection.QueryContext(ctx, selectPgStatActivity)
	if err != nil {
		return nil, fmt.Errorf("failed to query pg_stat_activity: %w", err)
	}
	return rows, nil
}

// scanRow reads a single row into QuerySamplesInfo
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

// processRow validates and classifies a row, returning its sample key and idle flag
func (c *QuerySamples) processRow(sample QuerySamplesInfo) (SampleKey, bool, error) {
	if err := c.validateQuerySample(sample); err != nil {
		return SampleKey{}, false, err
	}
	key := SampleKey{PID: sample.PID, QueryID: sample.QueryID.Int64, XID: sample.BackendXID.Int32}
	if sample.State.Valid && sample.State.String == stateIdle {
		return key, true, nil
	}
	return key, false, nil
}

// finalizeSamples emits samples that turned idle or disappeared
func (c *QuerySamples) finalizeSamples(activeKeys, idleKeys map[SampleKey]struct{}) {
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

// upsertActiveSample upserts the state for an active sample
func (c *QuerySamples) upsertActiveSample(key SampleKey, sample QuerySamplesInfo) {
	// Upsert state
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

// upsertWaitEvent ingests a new row and updates or opens/closes wait event occurrences accordingly
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
				// continue same wait event; update last values
				we.LastWaitTime = calculateDuration(sample.StateChange, now)
				we.LastState = sample.State.String
				we.LastTimestamp = now
				t.waitEvents[t.openIdx] = we
				return
			}
			// close current wait event
			t.openIdx = -1
		}

		// start new wait event
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

	// No wait event on this row; close any open wait event
	if t.openIdx >= 0 {
		t.openIdx = -1
	}
}

// upsertIdleSample upserts the state for an idle sample
func (c *QuerySamples) upsertIdleSample(key SampleKey, sample QuerySamplesInfo) {
	state, ok := c.samples[key]
	if !ok {
		state = &SampleState{tracker: newWaitEventTracker()}
		c.samples[key] = state
	}
	state.LastRow = sample
	state.LastSeenAt = sample.Now
	// Close any open wait event
	state.tracker.CloseOpen()
}

// emitAndDeleteSample builds final entries for a sample and removes it from memory.
func (c *QuerySamples) emitAndDeleteSample(key SampleKey, state *SampleState) {
	// Build and emit OP_QUERY_SAMPLE
	sampleLabels := c.buildQuerySampleLabels(state)
	c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		OP_QUERY_SAMPLE,
		sampleLabels,
		state.LastSeenAt.UnixNano(),
	)

	// Emit OP_WAIT_EVENT entries for each wait event
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

// updateCpuTimeIfActive applies CPU sampling rule and stores last observed cpu_time
func (s *SampleState) updateCpuTimeIfActive(sample QuerySamplesInfo) {
	if !sample.WaitEventType.Valid && !sample.WaitEvent.Valid && sample.State.String == stateActive {
		s.LastCpuTime = calculateDuration(sample.StateChange, sample.Now)
	}
}

// buildQuerySampleLabels constructs the labels string for OP_QUERY_SAMPLE
func (c *QuerySamples) buildQuerySampleLabels(state *SampleState) string {
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

	labels := fmt.Sprintf(
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
	if state.LastCpuTime != "" {
		labels = fmt.Sprintf(`%s cpu_time="%s"`, labels, state.LastCpuTime)
	}
	return labels
}

// buildWaitEventLabels constructs the labels string for OP_WAIT_EVENT
func (c *QuerySamples) buildWaitEventLabels(state *SampleState, we WaitEventOccurrence) string {
	waitEventFullName := fmt.Sprintf("%s:%s", we.WaitEventType, we.WaitEvent)
	queryText := state.LastRow.Query.String
	if !c.disableQueryRedaction {
		queryText = redact(queryText)
	}
	return fmt.Sprintf(
		`datname="%s" user="%s" backend_type="%s" state="%s" wait_time="%s" wait_event_type="%s" wait_event="%s" wait_event_name="%s" blocked_by_pids="%v" queryid="%d" query="%s" engine="postgres"`,
		state.LastRow.DatabaseName.String,
		state.LastRow.Username.String,
		state.LastRow.BackendType.String,
		we.LastState,
		we.LastWaitTime,
		we.WaitEventType,
		we.WaitEvent,
		waitEventFullName,
		we.BlockedByPIDs,
		state.LastRow.QueryID.Int64,
		queryText,
	)
}

// calculateDuration returns a formatted duration string between a nullable time and current time
func calculateDuration(nullableTime sql.NullTime, currentTime time.Time) string {
	if nullableTime.Valid {
		return currentTime.Sub(nullableTime.Time).String()
	}
	return ""
}

// normalizePIDs returns a sorted unique slice for stable set comparisons/printing.
func normalizePIDs(pids pq.Int64Array) []int64 {
	set := map[int64]struct{}{}
	for _, pid := range pids {
		set[pid] = struct{}{}
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
