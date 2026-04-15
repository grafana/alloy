# wait_event v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the multi-version wait event experiment (v1–v6) with a clean two-state design: `wait_event` (v1 baseline) or `wait_event_v2` (pre-classified `wait_event_type` in log body), controlled by the renamed flag `enable_pre_classified_wait_events`; add a `database_observability_wait_event_seconds_total` Prometheus counter to MySQL only.

**Architecture:** Work is done on branch `gaantunes/structured-metadata-poc`. Changes are split across Postgres (flag rename + op cleanup) and MySQL (same + Prometheus counter). TDD throughout — write the failing test first, then implement.

**Tech Stack:** Go, `github.com/prometheus/client_golang/prometheus`, `github.com/grafana/alloy/internal/component/loki`, `sqlmock` for collector tests.

**Spec:** `docs/superpowers/specs/2026-04-15-wait-event-v2-design.md`

---

## File Map

| File | Change |
|---|---|
| `internal/component/database_observability/postgres/component.go` | Rename flag |
| `internal/component/database_observability/postgres/collector/query_samples.go` | Rename flag, remove v2/v3/v5/v6, rename v4→v2, mutual exclusion |
| `internal/component/database_observability/postgres/collector/query_samples_test.go` | Update tests |
| `internal/component/database_observability/mysql/component.go` | Rename flag, create + register counter, pass to QuerySamples |
| `internal/component/database_observability/mysql/collector/query_details.go` | Rename flag field |
| `internal/component/database_observability/mysql/collector/query_samples.go` | Rename flag, remove v2/v3/v5/v6, rename v4→v2, mutual exclusion, increment counter |
| `internal/component/database_observability/mysql/collector/query_samples_test.go` | Update tests, add counter tests |
| `docs/sources/reference/components/database_observability/database_observability.mysql.md` | Update flag name |
| `docs/sources/reference/components/database_observability/database_observability.postgres.md` | Update flag name |

---

## Task 1: Rename flag in Postgres

**Files:**
- Modify: `internal/component/database_observability/postgres/component.go`
- Modify: `internal/component/database_observability/postgres/collector/query_samples.go`
- Modify: `internal/component/database_observability/postgres/component_test.go`

- [ ] **Step 1: Rename in postgres/component.go Arguments struct**

Find and replace all occurrences of `EnableStructuredMetadata` → `EnablePreClassifiedWaitEvents` and `enable_structured_metadata` → `enable_pre_classified_wait_events` in this file.

The field in the Arguments struct changes from:
```go
EnableStructuredMetadata bool `alloy:"enable_structured_metadata,attr,optional"`
```
to:
```go
EnablePreClassifiedWaitEvents bool `alloy:"enable_pre_classified_wait_events,attr,optional"`
```

And in the default args and any place it's passed down (grep for `EnableStructuredMetadata` in this file and update all occurrences).

- [ ] **Step 2: Rename in postgres/collector/query_samples.go**

In `QuerySamplesArguments` struct:
```go
// before
EnableStructuredMetadata bool
// after
EnablePreClassifiedWaitEvents bool
```

In `QuerySamples` struct:
```go
// before
enableStructuredMetadata bool
// after
enablePreClassifiedWaitEvents bool
```

In `NewQuerySamples`:
```go
// before
enableStructuredMetadata: args.EnableStructuredMetadata,
// after
enablePreClassifiedWaitEvents: args.EnablePreClassifiedWaitEvents,
```

In the emit logic, all references to `c.enableStructuredMetadata` → `c.enablePreClassifiedWaitEvents`.

- [ ] **Step 3: Update postgres/component_test.go**

Replace all `EnableStructuredMetadata` → `EnablePreClassifiedWaitEvents` in test setup.

- [ ] **Step 4: Run tests to confirm rename compiles and passes**

```bash
cd /home/gaantunes/git/alloy
git checkout gaantunes/structured-metadata-poc
go test ./internal/component/database_observability/postgres/... -count=1 -timeout 60s
```

Expected: all tests pass (behaviour unchanged — this is a pure rename).

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/
git commit -m "refactor(database_observability): rename enable_structured_metadata to enable_pre_classified_wait_events (postgres)"
```

---

## Task 2: Postgres op cleanup + mutual exclusion

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/query_samples.go`
- Modify: `internal/component/database_observability/postgres/collector/query_samples_test.go`

- [ ] **Step 1: Write failing tests**

Open `postgres/collector/query_samples_test.go`. Find `TestQuerySamples_WaitEvents`.

Add two sub-tests after the existing ones (or replace the existing structural assertions — see Step 3 for what the new assertions look like):

```go
t.Run("flag OFF emits only wait_event (v1)", func(t *testing.T) {
    // same DB mock setup as existing test
    // EnablePreClassifiedWaitEvents: false (default)
    // assert: entries contain OP_WAIT_EVENT only (no OP_WAIT_EVENT_V2)
    // assert: wait_event_type field is NOT in the log line
    collector, err := NewQuerySamples(QuerySamplesArguments{
        DB:                           db,
        EngineVersion:                latestCompatibleVersion,
        CollectInterval:              time.Second,
        EntryHandler:                 lokiClient,
        Logger:                       log.NewLogfmtLogger(os.Stderr),
        EnablePreClassifiedWaitEvents: false,
    })
    // ... run and collect entries
    // There should be 2 entries: one OP_QUERY_SAMPLE, one OP_WAIT_EVENT
    require.Len(t, lokiEntries, 2)
    require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
    require.NotContains(t, lokiEntries[1].Line, "wait_event_type=")
})

t.Run("flag ON emits only wait_event_v2", func(t *testing.T) {
    // same DB mock setup
    // EnablePreClassifiedWaitEvents: true
    // assert: entries contain OP_WAIT_EVENT_V2 only (no OP_WAIT_EVENT)
    // assert: wait_event_type="IO Wait" in the log line
    collector, err := NewQuerySamples(QuerySamplesArguments{
        DB:                           db,
        EngineVersion:                latestCompatibleVersion,
        CollectInterval:              time.Second,
        EntryHandler:                 lokiClient,
        Logger:                       log.NewLogfmtLogger(os.Stderr),
        EnablePreClassifiedWaitEvents: true,
    })
    // ... run and collect entries
    // There should be 2 entries: one OP_QUERY_SAMPLE, one OP_WAIT_EVENT_V2
    require.Len(t, lokiEntries, 2)
    require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT_V2}, lokiEntries[1].Labels)
    require.Contains(t, lokiEntries[1].Line, `wait_event_type="IO Wait"`)
    require.Empty(t, lokiEntries[1].StructuredMetadata)
})
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/component/database_observability/postgres/collector/... -run TestQuerySamples_WaitEvents -v -count=1
```

Expected: compile error (OP_WAIT_EVENT_V2 constant will be reused from old version, wrong semantics) or assertion failures showing 3–4 entries instead of 2.

- [ ] **Step 3: Update constants in query_samples.go**

Remove these constants:
```go
OP_WAIT_EVENT_V2 = "wait_event_v2"  // old SM version — DELETE
OP_WAIT_EVENT_V3 = "wait_event_v3"  // DELETE
OP_WAIT_EVENT_V5 = "wait_event_v5"  // DELETE
OP_WAIT_EVENT_V6 = "wait_event_v6"  // DELETE
```

Rename:
```go
// before
OP_WAIT_EVENT_V4 = "wait_event_v4"
// after
OP_WAIT_EVENT_V2 = "wait_event_v2"
```

- [ ] **Step 4: Remove helper functions for deleted ops**

Delete from `query_samples.go`:
- `buildWaitEventLabelsV2` (served old V2/V3/V5 SM ops)
- `buildWaitEventV5Labels`
- `buildWaitEventV6Labels`

Rename:
- `buildWaitEventV4Labels` → `buildWaitEventV2Labels`

- [ ] **Step 5: Replace emit block with mutual exclusion**

Find the wait event emit loop in `emitAndDeleteSample`. Replace the entire block (currently emits v1, then if-SM emits v2/v3/v5, then always emits v4 and v6) with:

```go
for _, we := range state.tracker.WaitEvents() {
    if we.WaitEventType == "" || we.WaitEvent == "" {
        continue
    }
    if c.enablePreClassifiedWaitEvents {
        c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
            logging.LevelInfo,
            OP_WAIT_EVENT_V2,
            c.buildWaitEventV2Labels(state, we),
            we.LastTimestamp.UnixNano(),
        )
    } else {
        c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
            logging.LevelInfo,
            OP_WAIT_EVENT,
            c.buildWaitEventLabels(state, we),
            we.LastTimestamp.UnixNano(),
        )
    }
}
```

`buildWaitEventV2Labels` (renamed from `buildWaitEventV4Labels`) produces:
```
datname="%s" pid="%d" leader_pid="%s" user="%s" backend_type="%s" state="%s"
xid="%d" xmin="%d" wait_time="%s" wait_event_type="%s" wait_event="%s"
wait_event_name="%s" blocked_by_pids="%v" queryid="%d"
```
where `wait_event_type` is `classifyPostgresWaitEventType(we.WaitEventType)`.

- [ ] **Step 6: Update existing test assertions**

In `TestQuerySamples_WaitEvents`, all existing assertions that reference `OP_WAIT_EVENT_V4` → `OP_WAIT_EVENT_V2`. All assertions that reference `OP_WAIT_EVENT_V6` → remove (v6 is gone). Adjust `require.Len` counts accordingly (3 entries per wait event → 2 entries: one sample + one wait event op).

Also update the structured-metadata enabled test (around line 1301) to use `EnablePreClassifiedWaitEvents` and assert `OP_WAIT_EVENT_V2` only.

- [ ] **Step 7: Run tests**

```bash
go test ./internal/component/database_observability/postgres/collector/... -run TestQuerySamples_WaitEvents -v -count=1
```

Expected: all pass.

- [ ] **Step 8: Run full Postgres suite**

```bash
go test ./internal/component/database_observability/postgres/... -count=1 -timeout 60s
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add internal/component/database_observability/postgres/
git commit -m "feat(database_observability): replace wait_event v1-v6 with clean v1/v2 mutual exclusion (postgres)"
```

---

## Task 3: Rename flag in MySQL

**Files:**
- Modify: `internal/component/database_observability/mysql/component.go`
- Modify: `internal/component/database_observability/mysql/collector/query_details.go`
- Modify: `internal/component/database_observability/mysql/collector/query_samples.go`
- Modify: `internal/component/database_observability/mysql/component_test.go`

- [ ] **Step 1: Rename in mysql/component.go**

In `Arguments` struct:
```go
// before
EnableStructuredMetadata bool `alloy:"enable_structured_metadata,attr,optional"`
// after
EnablePreClassifiedWaitEvents bool `alloy:"enable_pre_classified_wait_events,attr,optional"`
```

In default args (search for `EnableStructuredMetadata: false`) and every place the flag is passed to collectors (two places: `NewQueryDetails` call and `NewQuerySamples` call):
```go
// before
EnableStructuredMetadata: c.args.EnableStructuredMetadata,
// after
EnablePreClassifiedWaitEvents: c.args.EnablePreClassifiedWaitEvents,
```

- [ ] **Step 2: Rename in mysql/collector/query_details.go**

In `QueryDetailsArguments`:
```go
EnablePreClassifiedWaitEvents bool
```

In `QueryDetails` struct:
```go
enablePreClassifiedWaitEvents bool
```

In `NewQueryDetails`:
```go
enablePreClassifiedWaitEvents: args.EnablePreClassifiedWaitEvents,
```

In the emit logic:
```go
if c.enablePreClassifiedWaitEvents {
```

- [ ] **Step 3: Rename in mysql/collector/query_samples.go**

Same pattern as Postgres Task 1 Step 2:
- `QuerySamplesArguments.EnableStructuredMetadata` → `EnablePreClassifiedWaitEvents`
- `QuerySamples.enableStructuredMetadata` → `enablePreClassifiedWaitEvents`
- Constructor assignment
- All `c.enableStructuredMetadata` references in the emit loop

- [ ] **Step 4: Update mysql/component_test.go**

Replace all `EnableStructuredMetadata` → `EnablePreClassifiedWaitEvents`.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/component/database_observability/mysql/... -count=1 -timeout 60s
```

Expected: all pass (pure rename, no behaviour change).

- [ ] **Step 6: Commit**

```bash
git add internal/component/database_observability/mysql/
git commit -m "refactor(database_observability): rename enable_structured_metadata to enable_pre_classified_wait_events (mysql)"
```

---

## Task 4: MySQL op cleanup + mutual exclusion

**Files:**
- Modify: `internal/component/database_observability/mysql/collector/query_samples.go`
- Modify: `internal/component/database_observability/mysql/collector/query_samples_test.go`

- [ ] **Step 1: Write failing tests**

In `TestQuerySamples_WaitEvents`, add (or replace existing structural tests with) two sub-tests:

```go
t.Run("flag OFF emits only wait_event (v1)", func(t *testing.T) {
    // use existing sqlmock setup (schema="some_schema", digest="some_digest",
    // wait_event_name="wait/io/file/innodb/innodb_data_file", wait_time=100000000ps)
    collector, err := NewQuerySamples(QuerySamplesArguments{
        DB:                           db,
        EngineVersion:                latestCompatibleVersion,
        CollectInterval:              time.Second,
        EntryHandler:                 lokiClient,
        Logger:                       log.NewLogfmtLogger(os.Stderr),
        EnablePreClassifiedWaitEvents: false,
    })
    require.NoError(t, err)
    // ... start, wait for 2 entries, stop
    require.Len(t, lokiEntries, 2) // OP_QUERY_SAMPLE + OP_WAIT_EVENT
    require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT}, lokiEntries[1].Labels)
    require.NotContains(t, lokiEntries[1].Line, "wait_event_type=")
})

t.Run("flag ON emits only wait_event_v2", func(t *testing.T) {
    collector, err := NewQuerySamples(QuerySamplesArguments{
        DB:                           db,
        EngineVersion:                latestCompatibleVersion,
        CollectInterval:              time.Second,
        EntryHandler:                 lokiClient,
        Logger:                       log.NewLogfmtLogger(os.Stderr),
        EnablePreClassifiedWaitEvents: true,
    })
    require.NoError(t, err)
    // ... start, wait for 2 entries, stop
    require.Len(t, lokiEntries, 2) // OP_QUERY_SAMPLE + OP_WAIT_EVENT_V2
    require.Equal(t, model.LabelSet{"op": OP_WAIT_EVENT_V2}, lokiEntries[1].Labels)
    require.Contains(t, lokiEntries[1].Line, `wait_event_type="IO Wait"`)
    require.Empty(t, lokiEntries[1].StructuredMetadata)
    require.NotContains(t, lokiEntries[1].Line, "queryid=")  // no v6-style extra fields
})
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/component/database_observability/mysql/collector/... -run TestQuerySamples_WaitEvents -v -count=1
```

Expected: assertion failures (currently 4 entries, not 2).

- [ ] **Step 3: Remove constants for deleted ops in query_samples.go**

```go
// DELETE these:
OP_WAIT_EVENT_V2 = "wait_event_v2"  // old SM version
OP_WAIT_EVENT_V3 = "wait_event_v3"
OP_WAIT_EVENT_V5 = "wait_event_v5"
OP_WAIT_EVENT_V6 = "wait_event_v6"

// RENAME:
// OP_WAIT_EVENT_V4 = "wait_event_v4"  →
OP_WAIT_EVENT_V2 = "wait_event_v2"
```

- [ ] **Step 4: Remove SM helper functions**

Delete from `query_samples.go`:
- `classifyMySQLWaitEventType` — **keep** (used by new v2)
- Any helper that only served old v2/v3/v5/v6 SM emission (search for functions only referenced in the deleted `if c.enablePreClassifiedWaitEvents` SM blocks)

- [ ] **Step 5: Replace emit block with mutual exclusion**

Find the wait event emit block (currently: always emits v1, if-flag emits old v2/v3/v5, always emits v4/v6). Replace with:

```go
if row.WaitEventID.Valid && row.WaitTime.Valid {
    waitTime := picosecondsToMilliseconds(row.WaitTime.Float64)

    if c.enablePreClassifiedWaitEvents {
        waitV2LogMessage := fmt.Sprintf(
            `schema="%s" user="%s" client_host="%s" thread_id="%s" digest="%s" event_id="%s" wait_event_id="%s" wait_end_event_id="%s" wait_event_name="%s" wait_event_type="%s" wait_object_name="%s" wait_object_type="%s" wait_time="%fms"`,
            row.Schema.String,
            row.User.String,
            row.Host.String,
            row.ThreadID.String,
            row.Digest.String,
            row.StatementEventID.String,
            row.WaitEventID.String,
            row.WaitEndEventID.String,
            row.WaitEventName.String,
            classifyMySQLWaitEventType(row.WaitEventName.String),
            row.WaitObjectName.String,
            row.WaitObjectType.String,
            waitTime,
        )
        if c.disableQueryRedaction && row.SQLText.Valid {
            waitV2LogMessage += fmt.Sprintf(` sql_text="%s"`, row.SQLText.String)
        }
        c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
            logging.LevelInfo,
            OP_WAIT_EVENT_V2,
            waitV2LogMessage,
            int64(millisecondsToNanoseconds(row.TimestampMilliseconds)),
        )
    } else {
        waitLogMessage := fmt.Sprintf(
            `schema="%s" user="%s" client_host="%s" thread_id="%s" digest="%s" event_id="%s" wait_event_id="%s" wait_end_event_id="%s" wait_event_name="%s" wait_object_name="%s" wait_object_type="%s" wait_time="%fms"`,
            row.Schema.String,
            row.User.String,
            row.Host.String,
            row.ThreadID.String,
            row.Digest.String,
            row.StatementEventID.String,
            row.WaitEventID.String,
            row.WaitEndEventID.String,
            row.WaitEventName.String,
            row.WaitObjectName.String,
            row.WaitObjectType.String,
            waitTime,
        )
        if c.disableQueryRedaction && row.SQLText.Valid {
            waitLogMessage += fmt.Sprintf(` sql_text="%s"`, row.SQLText.String)
        }
        c.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
            logging.LevelInfo,
            OP_WAIT_EVENT,
            waitLogMessage,
            int64(millisecondsToNanoseconds(row.TimestampMilliseconds)),
        )
    }
}
```

- [ ] **Step 6: Update existing test assertions**

All assertions referencing `OP_WAIT_EVENT_V4` → `OP_WAIT_EVENT_V2`. All assertions referencing `OP_WAIT_EVENT_V6` → remove. Adjust `require.Len` / `require.Eventually` counts from 4 down to 2 per wait event row.

- [ ] **Step 7: Run tests**

```bash
go test ./internal/component/database_observability/mysql/collector/... -run TestQuerySamples_WaitEvents -v -count=1
```

Expected: all pass.

- [ ] **Step 8: Run full MySQL suite**

```bash
go test ./internal/component/database_observability/mysql/... -count=1 -timeout 60s
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add internal/component/database_observability/mysql/
git commit -m "feat(database_observability): replace wait_event v1-v6 with clean v1/v2 mutual exclusion (mysql)"
```

---

## Task 5: Prometheus counter in MySQL

**Files:**
- Modify: `internal/component/database_observability/mysql/component.go`
- Modify: `internal/component/database_observability/mysql/collector/query_samples.go`
- Modify: `internal/component/database_observability/mysql/collector/query_samples_test.go`

- [ ] **Step 1: Write failing counter test**

In `query_samples_test.go`, add a new test function:

```go
func TestQuerySamples_WaitEventCounter(t *testing.T) {
    db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
    require.NoError(t, err)
    defer db.Close()

    lokiClient := loki.NewCollectingHandler()
    reg := prometheus.NewRegistry()
    counter := prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "database_observability_wait_event_seconds_total",
        Help: "Total wait time in seconds per query, aggregated by server, query digest, and database.",
    }, []string{"server_id", "digest", "schema"})
    require.NoError(t, reg.Register(counter))

    collector, err := NewQuerySamples(QuerySamplesArguments{
        DB:               db,
        EngineVersion:    latestCompatibleVersion,
        CollectInterval:  time.Second,
        EntryHandler:     lokiClient,
        Logger:           log.NewLogfmtLogger(os.Stderr),
        WaitEventCounter: counter,
        ServerID:         "test-server",
    })
    require.NoError(t, err)

    // same mock rows as existing wait event test:
    // wait_event_name="wait/io/file/innodb/innodb_data_file"
    // timer_wait=100000000 (picoseconds) → 0.1ms → 0.0001s
    // schema="some_schema", digest="some_digest"
    mock.ExpectQuery(selectUptime)./* ... same as other tests ... */

    err = collector.Start(t.Context())
    require.NoError(t, err)

    require.Eventually(t, func() bool {
        return len(lokiClient.Received()) >= 2
    }, 5*time.Second, 100*time.Millisecond)

    collector.Stop()
    lokiClient.Stop()

    // Assert counter was incremented with correct labels and value
    mfs, err := reg.Gather()
    require.NoError(t, err)
    require.Len(t, mfs, 1)
    require.Equal(t, "database_observability_wait_event_seconds_total", *mfs[0].Name)
    require.Len(t, mfs[0].Metric, 1)

    m := mfs[0].Metric[0]
    labels := map[string]string{}
    for _, lp := range m.Label {
        labels[*lp.Name] = *lp.Value
    }
    require.Equal(t, "test-server", labels["server_id"])
    require.Equal(t, "some_digest", labels["digest"])
    require.Equal(t, "some_schema", labels["schema"])
    require.InDelta(t, 0.0001, m.Counter.GetValue(), 1e-9) // 100000000ps = 0.1ms = 0.0001s
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/component/database_observability/mysql/collector/... -run TestQuerySamples_WaitEventCounter -v -count=1
```

Expected: compile error — `WaitEventCounter` and `ServerID` fields don't exist yet.

- [ ] **Step 3: Add counter and ServerID to QuerySamplesArguments and QuerySamples**

In `mysql/collector/query_samples.go`, add to `QuerySamplesArguments`:
```go
WaitEventCounter *prometheus.CounterVec
ServerID         string
```

Add to `QuerySamples` struct:
```go
waitEventCounter *prometheus.CounterVec
serverID         string
```

Add to `NewQuerySamples` constructor:
```go
waitEventCounter: args.WaitEventCounter,
serverID:         args.ServerID,
```

Add import if not present: `"github.com/prometheus/client_golang/prometheus"`

- [ ] **Step 4: Increment counter in the emit loop**

At the end of the wait event block (after the if/else that emits v1 or v2), add unconditional counter increment:

```go
if c.waitEventCounter != nil {
    waitTimeSeconds := waitTime / 1000.0 // waitTime is in ms (from picosecondsToMilliseconds)
    c.waitEventCounter.WithLabelValues(c.serverID, row.Digest.String, row.Schema.String).Add(waitTimeSeconds)
}
```

This goes inside the `if row.WaitEventID.Valid && row.WaitTime.Valid` block, after the if/else emission.

- [ ] **Step 5: Run counter test**

```bash
go test ./internal/component/database_observability/mysql/collector/... -run TestQuerySamples_WaitEventCounter -v -count=1
```

Expected: PASS.

- [ ] **Step 6: Wire counter in mysql/component.go**

In `mysql/component.go`, in the `new()` function or `Update()` where `NewQuerySamples` is called, create and register the counter before constructing the collector:

```go
waitEventCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "database_observability_wait_event_seconds_total",
    Help: "Total wait time in seconds per query, aggregated by server, query digest, and database.",
}, []string{"server_id", "digest", "schema"})
if err := c.registry.Register(waitEventCounter); err != nil {
    return fmt.Errorf("failed to register wait event counter: %w", err)
}
```

Then pass it into `NewQuerySamples`:
```go
qsCollector, err := collector.NewQuerySamples(collector.QuerySamplesArguments{
    // ... existing fields ...
    WaitEventCounter: waitEventCounter,
    ServerID:         c.instanceKey,
})
```

- [ ] **Step 7: Run full MySQL suite**

```bash
go test ./internal/component/database_observability/mysql/... -count=1 -timeout 60s
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/component/database_observability/mysql/
git commit -m "feat(database_observability): add database_observability_wait_event_seconds_total counter to mysql"
```

---

## Task 6: Update docs

**Files:**
- Modify: `docs/sources/reference/components/database_observability/database_observability.mysql.md`
- Modify: `docs/sources/reference/components/database_observability/database_observability.postgres.md`

- [ ] **Step 1: Update mysql.md**

Find the `enable_structured_metadata` attribute entry. Replace:
```markdown
`enable_structured_metadata` | `bool` | ...
```
with:
```markdown
`enable_pre_classified_wait_events` | `bool` | When `true`, emits `wait_event_v2` entries with `wait_event_type` pre-classified as `IO Wait`, `Lock Wait`, `Network Wait`, or `Other Wait` in the log body, instead of the baseline `wait_event` entries. Defaults to `false`. | `false`
```

Remove any reference to `enable_structured_metadata` in examples or description blocks.

- [ ] **Step 2: Update postgres.md**

Same change as Step 1 in the Postgres component doc.

- [ ] **Step 3: Run doc build check (optional)**

```bash
# Only if make docs is available and fast:
# make docs
```

- [ ] **Step 4: Commit**

```bash
git add docs/sources/reference/components/database_observability/
git commit -m "docs(database_observability): update flag name to enable_pre_classified_wait_events"
```

---

## Final Verification

- [ ] **Run full database_observability suite**

```bash
go test ./internal/component/database_observability/... -count=1 -timeout 120s
```

Expected: all pass, zero failures.

- [ ] **Confirm no remaining references to old flag or old op names**

```bash
grep -r "enable_structured_metadata\|OP_WAIT_EVENT_V3\|OP_WAIT_EVENT_V5\|OP_WAIT_EVENT_V6\|wait_event_v3\|wait_event_v5\|wait_event_v6" \
  internal/component/database_observability/ --include="*.go"
```

Expected: no output (zero matches).

- [ ] **Confirm old SM emit helpers are gone**

```bash
grep -r "BuildLokiEntryWithStructuredMetadataAndTimestamp\|buildWaitEventLabelsV2\|buildWaitEventV5Labels\|buildWaitEventV6Labels" \
  internal/component/database_observability/ --include="*.go"
```

Expected: no output in query_samples files (the function may still exist in loki_entry.go but should not be called from query_samples).
