package collector

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	QuerySamplesCollector = "query_samples"
	QUERY_HASH_BYTE_LEN   = 8

	// maxWaitOccurrencesPerRequest bounds the number of distinct wait episodes
	// retained per tracked request. Long-lived, heavily-contended requests can
	// otherwise accumulate one occurrence per poll and flush them as a single
	// synchronous burst of Loki entries when the request finishes.
	maxWaitOccurrencesPerRequest = 1000
)

const selectQuerySamplesTemplate = `
WITH tracked_requests AS (
	SELECT
		SYSDATETIMEOFFSET() AS now,
		DB_NAME(r.database_id) AS database_name,
		r.session_id,
		r.request_id,
		TODATETIMEOFFSET(r.start_time, DATEPART(TZOFFSET, SYSDATETIMEOFFSET())) AS start_time,
		s.login_name,
		s.original_login_name,
		s.host_name,
		s.program_name,
		c.client_net_address,
		c.client_tcp_port,
		r.query_hash,
		r.cpu_time,
		r.total_elapsed_time,
		r.reads,
		r.writes,
		r.logical_reads,
		r.row_count,
		SUBSTRING(
			st.text,
			(r.statement_start_offset / 2) + 1,
			(
				CASE r.statement_end_offset
					WHEN -1 THEN DATALENGTH(st.text)
					ELSE r.statement_end_offset
				END - r.statement_start_offset
			) / 2 + 1
		) AS statement_text
	FROM sys.dm_exec_requests AS r
	INNER JOIN sys.dm_exec_sessions AS s
		ON s.session_id = r.session_id
	LEFT JOIN sys.dm_exec_connections AS c
		ON c.connection_id = r.connection_id
	OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) AS st
	WHERE
		r.session_id <> @@SPID
		AND s.is_user_process = 1
		AND r.database_id = DB_ID()
		AND r.query_hash IN (%s)
		%s
),
current_waits AS (
	SELECT
		t.session_id,
		t.request_id,
		w.exec_context_id,
		w.wait_type,
		w.wait_duration_ms,
		w.blocking_session_id,
		w.resource_description
	FROM sys.dm_os_tasks AS t
	INNER JOIN sys.dm_os_waiting_tasks AS w
		ON w.waiting_task_address = t.task_address
)
SELECT
	r.now,
	r.database_name,
	r.session_id,
	r.request_id,
	r.start_time,
	r.login_name,
	r.original_login_name,
	r.host_name,
	r.program_name,
	r.client_net_address,
	r.client_tcp_port,
	r.query_hash,
	r.cpu_time,
	r.total_elapsed_time,
	r.reads,
	r.writes,
	r.logical_reads,
	r.row_count,
	r.statement_text,
	w.exec_context_id,
	w.wait_type,
	w.wait_duration_ms,
	w.blocking_session_id,
	w.resource_description
FROM tracked_requests AS r
LEFT JOIN current_waits AS w
	ON w.session_id = r.session_id
	AND w.request_id = r.request_id
ORDER BY r.session_id, r.request_id, w.exec_context_id`

type QuerySamplesArguments struct {
	DB                    *sql.DB
	CollectInterval       time.Duration
	QueryTimeout          time.Duration
	Tracker               QueryTracker
	EntryHandler          loki.EntryHandler
	Logger                *slog.Logger
	DisableQueryRedaction bool
	ExcludeDatabases      []string
	ExcludeUsers          []string
}

type QuerySamples struct {
	dbConnection          *sql.DB
	collectInterval       time.Duration
	queryTimeout          time.Duration
	tracker               QueryTracker
	entryHandler          loki.EntryHandler
	disableQueryRedaction bool
	excludeDatabases      []string
	excludeUsers          []string

	logger  *slog.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	samples map[querySampleKey]*querySampleState
}

type querySampleRow struct {
	Now               time.Time
	DatabaseName      string
	SessionID         int64
	RequestID         int64
	StartTime         time.Time
	LoginName         sql.NullString
	OriginalLoginName sql.NullString
	HostName          sql.NullString
	ProgramName       sql.NullString
	ClientAddress     sql.NullString
	ClientPort        sql.NullInt64
	QueryHash         string
	CPUTimeMillis     int64
	ElapsedTimeMillis int64
	Reads             int64
	Writes            int64
	LogicalReads      int64
	RowCount          int64
	StatementText     sql.NullString
	ExecContextID     sql.NullInt64
	WaitType          sql.NullString
	WaitDurationMs    sql.NullInt64
	BlockingSessionID sql.NullInt64
	Resource          sql.NullString
}

type querySampleKey struct {
	sessionID  int64
	requestID  int64
	startNanos int64
	queryHash  string
}

type querySampleState struct {
	lastRow    querySampleRow
	lastSeenAt time.Time
	waits      queryWaitTracker
	capWarned  bool
}

type queryWaitIdentity struct {
	execContextID     int64
	waitType          string
	resource          string
	blockingSessionID sql.NullInt64
}

type queryWaitOccurrence struct {
	identity queryWaitIdentity
	duration time.Duration
}

type queryWaitTracker struct {
	occurrences []queryWaitOccurrence
	openByTask  map[int64]int
	dropped     int
}

var (
	replicationWaitEventPrefixes = []string{
		"DBMIRROR_",
		"HADR_",
		"PWAIT_HADR",
		"REDO_",
		"REPL_",
		"SE_REPL_",
	}
	replicationWaitEventNames = []string{
		"FCB_REPLICA_READ",
		"FCB_REPLICA_WRITE",
		"REPLICA_WRITE",
		"REPLICA_WRITES",
	}
	networkWaitEventNames = []string{
		"ASYNC_NETWORK_IO",
		"EXTERNAL_SCRIPT_NETWORK_IO",
		"NET_WAITFOR_PACKET",
		"PROXY_NETWORK_IO",
	}
	ioWaitEventPrefixes = []string{
		"ASYNC_IO_COMPLETION",
		"BACKUPBUFFER",
		"BACKUPIO",
		"IO_COMPLETION",
		"PAGEIOLATCH_",
		"WRITE_COMPLETION",
	}
	ioWaitEventNames = []string{
		"DISKIO_SUSPEND",
		"LOGBUFFER",
		"LOGMGR",
		"LOGMGR_FLUSH",
		"WRITELOG",
	}
	engineWaitEventPrefixes = []string{
		"CXSYNC_",
		"LATCH_",
		"PAGELATCH_",
		"RESOURCE_SEMAPHORE",
	}
	engineWaitEventNames = []string{
		"CXCONSUMER",
		"CXPACKET",
		"EXCHANGE",
		"SOS_SCHEDULER_YIELD",
		"THREADPOOL",
	}
)

func newQueryWaitTracker() queryWaitTracker {
	return queryWaitTracker{openByTask: make(map[int64]int)}
}

func NewQuerySamples(args QuerySamplesArguments) (*QuerySamples, error) {
	return &QuerySamples{
		dbConnection:          args.DB,
		collectInterval:       args.CollectInterval,
		queryTimeout:          queryTimeoutOrDefault(args.QueryTimeout),
		tracker:               args.Tracker,
		entryHandler:          args.EntryHandler,
		disableQueryRedaction: args.DisableQueryRedaction,
		excludeDatabases:      append(slices.Clone(excludedDatabases), args.ExcludeDatabases...),
		excludeUsers:          uniqueSorted(args.ExcludeUsers),
		logger:                args.Logger.With("collector", QuerySamplesCollector),
		running:               atomic.NewBool(false),
		samples:               make(map[querySampleKey]*querySampleState),
	}, nil
}

func (c *QuerySamples) Name() string {
	return QuerySamplesCollector
}

func (c *QuerySamples) Start(ctx context.Context) error {
	if c.disableQueryRedaction {
		c.logger.Warn("collector started with query redaction disabled. SQL text in query samples may include query parameters.")
	} else {
		c.logger.Debug("collector started")
	}

	c.running.Store(true)
	c.ctx, c.cancel = context.WithCancel(ctx)

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			if err := c.collect(c.ctx); err != nil {
				c.logger.Error("collector error", "err", err)
			}

			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})

	return nil
}

func (c *QuerySamples) Stopped() bool {
	return !c.running.Load()
}

func (c *QuerySamples) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *QuerySamples) collect(ctx context.Context) error {
	if c.tracker == nil {
		c.logger.Error("no query tracker available, skipping collection")
		return nil
	}

	database, ok := checkQueryStoreState(ctx, c.dbConnection, c.queryTimeout, c.logger)
	if !ok {
		return nil
	}
	if c.databaseExcluded(database) {
		return nil
	}

	registryHashes := c.tracker.GetQueryHashes(database)
	registrySet := make(map[string]struct{}, len(registryHashes))
	for _, queryHash := range registryHashes {
		registrySet[queryHash] = struct{}{}
	}

	// Keep hashes for requests admitted by an earlier registry snapshot pinned
	// until those requests disappear. Rows for new requests with a pinned but no
	// longer registered hash are rejected below.
	candidateHashes := slices.Clone(registryHashes)
	for _, state := range c.samples {
		if state.lastRow.DatabaseName == database {
			candidateHashes = append(candidateHashes, state.lastRow.QueryHash)
		}
	}
	if len(candidateHashes) == 0 {
		return nil
	}

	// TODO(cristian): GetQueryHashes returns TTL-retained hashes and can exceed
	// query_metrics.statements_limit. Introduce an exact latest-top-N snapshot
	// before relying on the tracker as a strict admission bound.
	query, args, err := buildQuerySamplesStatement(candidateHashes, c.excludeUsers)
	if err != nil {
		return err
	}

	var snapshot []querySampleRow
	err = withQueryTimeout(ctx, c.queryTimeout, func(queryCtx context.Context) error {
		rows, err := c.dbConnection.QueryContext(queryCtx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to query SQL Server activity: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			row, err := scanQuerySampleRow(rows)
			if err != nil {
				return fmt.Errorf("failed to scan SQL Server activity: %w", err)
			}
			snapshot = append(snapshot, row)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to read SQL Server activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	c.applySnapshot(snapshot, registrySet)
	return nil
}

func scanQuerySampleRow(rows *sql.Rows) (querySampleRow, error) {
	var row querySampleRow
	var queryHash []byte
	err := rows.Scan(
		&row.Now,
		&row.DatabaseName,
		&row.SessionID,
		&row.RequestID,
		&row.StartTime,
		&row.LoginName,
		&row.OriginalLoginName,
		&row.HostName,
		&row.ProgramName,
		&row.ClientAddress,
		&row.ClientPort,
		&queryHash,
		&row.CPUTimeMillis,
		&row.ElapsedTimeMillis,
		&row.Reads,
		&row.Writes,
		&row.LogicalReads,
		&row.RowCount,
		&row.StatementText,
		&row.ExecContextID,
		&row.WaitType,
		&row.WaitDurationMs,
		&row.BlockingSessionID,
		&row.Resource,
	)
	if err != nil {
		return querySampleRow{}, err
	}

	row.QueryHash, err = formatQueryHash(queryHash)
	if err != nil {
		return querySampleRow{}, err
	}
	return row, nil
}

func (c *QuerySamples) applySnapshot(snapshot []querySampleRow, registrySet map[string]struct{}) {
	activeKeys := make(map[querySampleKey]struct{})
	seenWaitTasks := make(map[querySampleKey]map[int64]struct{})

	for _, row := range snapshot {
		key := newQuerySampleKey(row)
		state, admitted := c.samples[key]
		if !admitted {
			if _, registered := registrySet[row.QueryHash]; !registered {
				continue
			}
			state = &querySampleState{waits: newQueryWaitTracker()}
			c.samples[key] = state
		}

		state.lastRow = row
		state.lastSeenAt = row.Now
		activeKeys[key] = struct{}{}

		if row.ExecContextID.Valid && row.WaitType.Valid && row.WaitDurationMs.Valid {
			seen := seenWaitTasks[key]
			if seen == nil {
				seen = make(map[int64]struct{})
				seenWaitTasks[key] = seen
			}
			seen[row.ExecContextID.Int64] = struct{}{}
			if state.waits.observe(row) && !state.capWarned {
				state.capWarned = true
				c.logger.Warn(
					"wait occurrence cap reached; dropping further wait episodes for request",
					"database", row.DatabaseName,
					"session_id", row.SessionID,
					"request_id", row.RequestID,
					"query_hash", row.QueryHash,
					"cap", maxWaitOccurrencesPerRequest,
				)
			}
		}
	}

	for key := range activeKeys {
		c.samples[key].waits.closeMissing(seenWaitTasks[key])
	}

	for key := range c.samples {
		if _, active := activeKeys[key]; active {
			continue
		}
		c.emitAndDelete(key)
	}
}

func newQuerySampleKey(row querySampleRow) querySampleKey {
	return querySampleKey{
		sessionID:  row.SessionID,
		requestID:  row.RequestID,
		startNanos: row.StartTime.UnixNano(),
		queryHash:  row.QueryHash,
	}
}

func (t *queryWaitTracker) observe(row querySampleRow) bool {
	duration := time.Duration(row.WaitDurationMs.Int64) * time.Millisecond
	if duration < 0 {
		duration = 0
	}
	identity := queryWaitIdentity{
		execContextID:     row.ExecContextID.Int64,
		waitType:          row.WaitType.String,
		resource:          row.Resource.String,
		blockingSessionID: row.BlockingSessionID,
	}

	if occurrenceIdx, ok := t.openByTask[identity.execContextID]; ok {
		occurrence := &t.occurrences[occurrenceIdx]
		if occurrence.identity == identity && duration >= occurrence.duration {
			occurrence.duration = duration
			return false
		}
		delete(t.openByTask, identity.execContextID)
	}

	if len(t.occurrences) >= maxWaitOccurrencesPerRequest {
		t.dropped++
		return true
	}

	t.occurrences = append(t.occurrences, queryWaitOccurrence{
		identity: identity,
		duration: duration,
	})
	t.openByTask[identity.execContextID] = len(t.occurrences) - 1
	return false
}

func (t *queryWaitTracker) closeMissing(seen map[int64]struct{}) {
	for taskID := range t.openByTask {
		if _, ok := seen[taskID]; !ok {
			delete(t.openByTask, taskID)
		}
	}
}

func (c *QuerySamples) emitAndDelete(key querySampleKey) {
	state, ok := c.samples[key]
	if !ok {
		return
	}

	timestamp := state.lastSeenAt.UnixNano()
	if !state.lastRow.StartTime.IsZero() {
		timestamp = state.lastRow.StartTime.UnixNano()
	}
	c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		database_observability.OP_QUERY_SAMPLE,
		c.buildQuerySampleLine(state.lastRow),
		timestamp,
	)

	for _, wait := range state.waits.occurrences {
		if wait.identity.waitType == "" {
			continue
		}
		c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
			logging.LevelInfo,
			database_observability.OP_WAIT_EVENT_V2,
			c.buildWaitEventLine(state.lastRow, wait),
			timestamp,
		)
	}

	delete(c.samples, key)
}

func (c *QuerySamples) buildQuerySampleLine(row querySampleRow) string {
	line := fmt.Sprintf(
		`database=%s user=%s original_login=%s host=%s program=%s client_address=%s`,
		strconv.Quote(row.DatabaseName),
		strconv.Quote(row.LoginName.String),
		strconv.Quote(row.OriginalLoginName.String),
		strconv.Quote(row.HostName.String),
		strconv.Quote(row.ProgramName.String),
		strconv.Quote(row.ClientAddress.String),
	)
	if row.ClientPort.Valid {
		line += fmt.Sprintf(` client_port="%d"`, row.ClientPort.Int64)
	}
	line += fmt.Sprintf(
		` session_id="%d" request_id="%d" query_hash=%s cpu_time="%dms" elapsed_time="%s" elapsed_time_ms="%d" reads="%d" writes="%d" logical_reads="%d" row_count="%d"`,
		row.SessionID,
		row.RequestID,
		strconv.Quote(row.QueryHash),
		row.CPUTimeMillis,
		(time.Duration(row.ElapsedTimeMillis) * time.Millisecond).String(),
		row.ElapsedTimeMillis,
		row.Reads,
		row.Writes,
		row.LogicalReads,
		row.RowCount,
	)

	if row.StatementText.Valid {
		if traceparent := database_observability.TryExtractTraceParent(row.StatementText.String); traceparent != "" {
			line += " traceparent=" + strconv.Quote(traceparent)
		}
		if c.disableQueryRedaction {
			line += " query=" + strconv.Quote(row.StatementText.String)
		}
	}
	return line
}

func (c *QuerySamples) buildWaitEventLine(row querySampleRow, wait queryWaitOccurrence) string {
	line := fmt.Sprintf(
		`database=%s user=%s session_id="%d" request_id="%d" exec_context_id="%d" query_hash=%s wait_event_type=%s wait_event_name=%s wait_object_name=%s`,
		strconv.Quote(row.DatabaseName),
		strconv.Quote(row.LoginName.String),
		row.SessionID,
		row.RequestID,
		wait.identity.execContextID,
		strconv.Quote(row.QueryHash),
		strconv.Quote(classifySQLServerWaitEventType(wait.identity.waitType)),
		strconv.Quote(wait.identity.waitType),
		strconv.Quote(wait.identity.resource),
	)
	if wait.identity.blockingSessionID.Valid {
		line += fmt.Sprintf(` blocking_session_id="%d"`, wait.identity.blockingSessionID.Int64)
	}
	return line + fmt.Sprintf(` wait_time=%s`, strconv.Quote(wait.duration.String()))
}

func buildQuerySamplesStatement(hashes, excludedUsers []string) (string, []any, error) {
	hashes = uniqueSorted(hashes)
	hashPlaceholders := make([]string, 0, len(hashes))
	args := make([]any, 0, len(hashes)+len(excludedUsers))
	for i, queryHash := range hashes {
		raw, err := hex.DecodeString(queryHash)
		if err != nil {
			return "", nil, fmt.Errorf("failed to decode tracked query hash %q: %w", queryHash, err)
		}
		if len(raw) != QUERY_HASH_BYTE_LEN {
			return "", nil, fmt.Errorf("invalid tracked query hash length %d for %q, expected %d bytes", len(raw), queryHash, QUERY_HASH_BYTE_LEN)
		}
		name := fmt.Sprintf("h%d", i)
		hashPlaceholders = append(hashPlaceholders, "@"+name)
		args = append(args, sql.Named(name, raw))
	}
	if len(hashPlaceholders) == 0 {
		return "", nil, fmt.Errorf("cannot build query samples statement without query hashes")
	}

	userClause := ""
	if len(excludedUsers) > 0 {
		userPlaceholders := make([]string, 0, len(excludedUsers))
		for i, user := range excludedUsers {
			name := fmt.Sprintf("u%d", i)
			userPlaceholders = append(userPlaceholders, "@"+name)
			args = append(args, sql.Named(name, user))
		}
		userClause = fmt.Sprintf("AND s.original_login_name NOT IN (%s)", strings.Join(userPlaceholders, ", "))
	}

	return fmt.Sprintf(selectQuerySamplesTemplate, strings.Join(hashPlaceholders, ", "), userClause), args, nil
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	values = slices.Clone(values)
	slices.Sort(values)
	return slices.Compact(values)
}

func (c *QuerySamples) databaseExcluded(database string) bool {
	for _, excluded := range c.excludeDatabases {
		if strings.EqualFold(database, excluded) {
			return true
		}
	}
	return false
}

func classifySQLServerWaitEventType(waitType string) string {
	waitType = strings.ToUpper(waitType)

	if hasAnyPrefix(waitType, replicationWaitEventPrefixes...) || hasAnyValue(waitType, replicationWaitEventNames...) {
		return database_observability.WAIT_EVENT_TYPE_REPLICATION
	}
	if strings.HasPrefix(waitType, "LCK_M_") {
		return database_observability.WAIT_EVENT_TYPE_LOCK
	}
	if hasAnyValue(waitType, networkWaitEventNames...) {
		return database_observability.WAIT_EVENT_TYPE_NETWORK
	}
	if hasAnyPrefix(waitType, ioWaitEventPrefixes...) || hasAnyValue(waitType, ioWaitEventNames...) {
		return database_observability.WAIT_EVENT_TYPE_IO
	}
	if hasAnyPrefix(waitType, engineWaitEventPrefixes...) || hasAnyValue(waitType, engineWaitEventNames...) {
		return database_observability.WAIT_EVENT_TYPE_ENGINE
	}
	return database_observability.WAIT_EVENT_TYPE_OTHER
}

func hasAnyPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func hasAnyValue(value string, values ...string) bool {
	return slices.Contains(values, value)
}
