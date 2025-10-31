package collector

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"slices"
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
	queryTextClause     = ", s.query"
	stateActive         = "active"
	stateIdle           = "idle"
	stateIdleTxnAborted = "idle in transaction (aborted)"
	stateIdleTxn        = "idle in transaction"
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
		s.query_id
		%s
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
	ThrottleInterval      time.Duration
}

type QuerySamples struct {
	dbConnection          *sql.DB
	collectInterval       time.Duration
	entryHandler          loki.EntryHandler
	disableQueryRedaction bool
	throttleInterval      time.Duration
	lastEmittedByQueryID  map[int64]time.Time
	adptiveShortInterval  time.Duration
	adaptiveRemaining     int
	adaptiveCooldown      int
	adaptiveCycles        int

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc

	// in-memory state of running samples
	samples map[SampleKey]*SampleState
	// keep track of keys that were already emitted to avoid duplicates
	emitted map[SampleKey]time.Time
}

func NewQuerySamples(args QuerySamplesArguments) (*QuerySamples, error) {
	// Compute adaptive short-polling parameters based on the collection interval.
	shortInterval, cycles := computeAdaptiveShortPolling(args.CollectInterval)
	return &QuerySamples{
		dbConnection:          args.DB,
		collectInterval:       args.CollectInterval,
		entryHandler:          args.EntryHandler,
		disableQueryRedaction: args.DisableQueryRedaction,
		logger:                log.With(args.Logger, "collector", QuerySamplesCollector),
		running:               &atomic.Bool{},
		samples:               map[SampleKey]*SampleState{},
		emitted:               map[SampleKey]time.Time{},
		throttleInterval:      args.ThrottleInterval,
		lastEmittedByQueryID:  map[int64]time.Time{},
		adptiveShortInterval:  shortInterval,
		adaptiveCooldown:      0,
		adaptiveCycles:        cycles,
	}, nil
}

func (c *QuerySamples) Name() string {
	return QuerySamplesCollector
}

func (c *QuerySamples) Start(ctx context.Context) error {
	if c.disableQueryRedaction {
		level.Warn(c.logger).Log("msg", "collector started with query redaction disabled. SQL text in query samples may include query parameters.")
	} else {
		level.Debug(c.logger).Log("msg", "collector started")
	}

	if c.throttleInterval < 30*time.Second {
		level.Warn(c.logger).Log("msg", fmt.Sprintf("collector configured with throttle interval below 30 seconds: %s. This may result in excessive samples volume.", c.throttleInterval))
	}

	level.Debug(c.logger).Log("msg", fmt.Sprintf("collector started with throttle interval: %s", c.throttleInterval))

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	go func() {
		defer func() {
			c.Stop()
			c.running.Store(false)
		}()

		for {
			if err := c.fetchQuerySample(c.ctx); err != nil {
				level.Error(c.logger).Log("msg", "collector error", "err", err)
			}

			// Decide next interval (adaptive short polling when active)
			interval := c.collectInterval
			if c.adaptiveRemaining > 0 {
				interval = c.adptiveShortInterval
				c.adaptiveRemaining--
				// Arm a cooldown once the last short cycle completes
				if c.adaptiveRemaining == 0 && c.adaptiveCooldown == 0 {
					c.adaptiveCooldown = 1
				}
			} else if c.adaptiveCooldown > 0 {
				// Enforce at least one base interval before re-arming
				interval = c.collectInterval
				c.adaptiveCooldown--
			}
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(interval):
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
	// EndOverride is used to compute durations and timestamps when a query
	// transitioned to idle or was only observed as idle.
	EndOverride sql.NullTime
}

func (s *SampleState) updateCpuTimeIfActive(sample QuerySamplesInfo) {
	if !sample.WaitEventType.Valid && !sample.WaitEvent.Valid && sample.State.String == stateActive {
		s.LastCpuTime = calculateDuration(sample.StateChange, sample.Now)
	}
}

// markEndedAt sets EndOverride and LastSeenAt based on the sample's state change or clock_timestamp if not available
func (s *SampleState) markEndedAt(sample QuerySamplesInfo) {
	if sample.StateChange.Valid {
		s.EndOverride = sample.StateChange
		s.LastSeenAt = sample.StateChange.Time
		return
	}
	s.EndOverride = sql.NullTime{Time: sample.Now, Valid: true}
	s.LastSeenAt = sample.Now
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

func (c *QuerySamples) fetchQuerySample(ctx context.Context) error {
	queryTextField := ""
	if c.disableQueryRedaction {
		queryTextField = queryTextClause
	}

	query := fmt.Sprintf(selectPgStatActivity, queryTextField)
	rows, err := c.dbConnection.QueryContext(ctx, query)
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

		if st, hadActive := c.samples[key]; hadActive {
			st.markEndedAt(sample)
			st.LastRow.State = sample.State
			c.emitted[key] = sample.Now
		} else if _, already := c.emitted[key]; !already {
			// new idle sample not yet seen -> create a new sample state to track and emit it
			newIdleState := &SampleState{LastRow: sample, tracker: newWaitEventTracker()}
			newIdleState.markEndedAt(sample)
			newIdleState.LastRow.State = sample.State
			c.samples[key] = newIdleState
			c.emitted[key] = sample.Now
		}
	}

	if err := rows.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate pg_stat_activity rows", "err", err)
		return err
	}

	// finalize samples that are no longer active or have EndOverride set (idle finalized or one off idle sample)
	for key, st := range c.samples {
		if _, stillActive := activeKeys[key]; stillActive && !st.EndOverride.Valid {
			continue
		}
		c.emitAndDeleteSample(key)
	}

	if len(activeKeys) > 0 && c.adaptiveCooldown == 0 {
		if c.adaptiveRemaining < c.adaptiveCycles {
			c.adaptiveRemaining = c.adaptiveCycles
		}
	}
	c.cleanupEmitted(time.Now())
	return nil
}

func (c *QuerySamples) scanRow(rows *sql.Rows) (QuerySamplesInfo, error) {
	sample := QuerySamplesInfo{}
	scanArgs := []interface{}{
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
	}
	if c.disableQueryRedaction {
		scanArgs = append(scanArgs, &sample.Query)
	}
	err := rows.Scan(scanArgs...)
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
	if c.disableQueryRedaction {
		if sample.Query.Valid && sample.Query.String == "<insufficient privilege>" {
			return fmt.Errorf("insufficient privilege to access query sample set: %+v", sample)
		}
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
	state.LastRow.State = sample.State
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

	var endOverride *time.Time
	if state.EndOverride.Valid {
		t := state.EndOverride.Time
		endOverride = &t
	}
	sampleLabels := c.buildQuerySampleLabels(state, endOverride)
	ts := state.LastSeenAt.UnixNano()
	if endOverride != nil {
		ts = endOverride.UnixNano()
	}

	shouldEmit := true
	if !isThrottleExempt(state) && c.throttleInterval > 0 {
		qid := state.LastRow.QueryID.Int64
		if last, ok := c.lastEmittedByQueryID[qid]; ok {
			if time.Since(last) < c.throttleInterval {
				shouldEmit = false
			}
		}
		if shouldEmit {
			c.lastEmittedByQueryID[qid] = time.Now()
		}
	}

	if shouldEmit {
		c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
			logging.LevelInfo,
			OP_QUERY_SAMPLE,
			sampleLabels,
			ts,
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
	}

	delete(c.samples, key)
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

	labels := fmt.Sprintf(
		`datname="%s" pid="%d" leader_pid="%s" user="%s" app="%s" client="%s" backend_type="%s" state="%s" xid="%d" xmin="%d" xact_time="%s" query_time="%s" queryid="%d"`,
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
	)

	if state.LastCpuTime != "" {
		labels = fmt.Sprintf(`%s cpu_time="%s"`, labels, state.LastCpuTime)
	}
	if c.disableQueryRedaction && state.LastRow.Query.Valid {
		labels = fmt.Sprintf(`%s query="%s"`, labels, state.LastRow.Query.String)
	}
	return labels
}

func (c *QuerySamples) buildWaitEventLabels(state *SampleState, we WaitEventOccurrence) string {
	waitEventFullName := fmt.Sprintf("%s:%s", we.WaitEventType, we.WaitEvent)
	leaderPID := ""
	if state.LastRow.LeaderPID.Valid {
		leaderPID = fmt.Sprintf(`%d`, state.LastRow.LeaderPID.Int64)
	}
	return fmt.Sprintf(
		`datname="%s" pid="%d" leader_pid="%s" user="%s" backend_type="%s" state="%s" xid="%d" xmin="%d" wait_time="%s" wait_event_type="%s" wait_event="%s" wait_event_name="%s" blocked_by_pids="%v" queryid="%d"`,
		state.LastRow.DatabaseName.String,
		state.LastRow.PID,
		leaderPID,
		state.LastRow.Username.String,
		state.LastRow.BackendType.String,
		we.LastState,
		state.LastRow.BackendXID.Int32,
		state.LastRow.BackendXmin.Int32,
		we.LastWaitTime,
		we.WaitEventType,
		we.WaitEvent,
		waitEventFullName,
		we.BlockedByPIDs,
		state.LastRow.QueryID.Int64,
	)
}

func (c *QuerySamples) cleanupEmitted(now time.Time) {
	const ttl = 10 * time.Minute
	for k, lastSeen := range c.emitted {
		if now.Sub(lastSeen) > ttl {
			delete(c.emitted, k)
		}
	}
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
	if state == stateIdle || state == stateIdleTxnAborted {
		return true
	}
	return false
}

func isThrottleExempt(sample *SampleState) bool {
	if sample.LastRow.State.String == stateIdleTxn || sample.LastRow.State.String == stateIdleTxnAborted || len(sample.tracker.WaitEvents()) > 0 || sample.LastCpuTime != "" {
		return true
	}
	return false
}

// computeAdaptiveShortPolling derives the short-interval (s) and number of
// short cycles (r) for adaptive polling, given the base collection interval (CI).
//
// Goals:
//   - Adaptively increase sampling density when there is activity, to better
//     approximate query end times that occur between scrapes.
//   - Keep overhead bounded: total short-poll window T = r*s must be < CI/2 to
//     avoid race conditions with concurrent collectors and to limit DB load.
//   - Ensure the short interval does not become too aggressive: floor at 100ms,
//     and cap at 300ms for stability across large CIs.
//
// Heuristic (amplified root/log scaling):
//
//	s = clamp(CI/35, 100ms, 300ms)
//	r_cap   = floor((CI/2 - 50ms) / s)
//	r_shape = floor(2.5*sqrt(CI_seconds) + 1.2*log2(1 + CI_seconds) + 1)
//	r       = min(r_cap, r_shape)
//
// Properties:
//   - r*s is guaranteed < CI/2 via r_cap (with a small 50ms guard band).
//   - s never increases as CI decreases due to the 100ms floor.
//   - r grows sublinearly with CI (sqrt+log), which increases coverage for larger
//     CIs without exploding the number of short cycles.

// Examples:

// CI=1s  → s=100ms, r=4  → T=400ms (40%)
// CI=2s  → s=100ms, r=6  → T=600ms (30%)
// CI=5s  → s≈143ms, r=9  → T≈1.29s (25.8%)
// CI=10s → s≈286ms, r=13 → T≈3.72s (37.2%)
// CI=30s → s=300ms, r=20 → T=6.0s (20%)

func computeAdaptiveShortPolling(collectInterval time.Duration) (time.Duration, int) {
	if collectInterval <= 0 {
		return 150 * time.Millisecond, 0
	}

	ciSeconds := float64(collectInterval) / float64(time.Second)

	// Short interval: clamp(CI/35, 100ms, 300ms)
	s := time.Duration(float64(collectInterval) / 35.0)
	if s < 100*time.Millisecond {
		s = 100 * time.Millisecond
	} else if s > 300*time.Millisecond {
		s = 300 * time.Millisecond
	}

	// Maximum cycles such that r*s < CI/2 (with 50ms guard)
	guard := 50 * time.Millisecond
	capWindow := collectInterval/2 - guard
	if capWindow < 0 {
		capWindow = 0
	}
	rCap := int(math.Floor(float64(capWindow) / float64(s)))
	if rCap < 0 {
		rCap = 0
	}

	// Shape-driven cycles: 2.5*sqrt(CI_s) + 1.2*log2(1+CI_s) + 1
	rShape := int(math.Floor(2.5*math.Sqrt(ciSeconds) + 1.2*math.Log2(1.0+ciSeconds) + 1.0))
	if rShape < 0 {
		rShape = 0
	}

	r := rShape
	if r > rCap {
		r = rCap
	}

	return s, r
}
