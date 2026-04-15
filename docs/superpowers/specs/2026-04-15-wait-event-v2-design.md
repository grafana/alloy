# Design: wait_event v2 — Pre-Classified Wait Events + Prometheus Counter

**Date:** 2026-04-15
**Branch:** gaantunes/structured-metadata-poc (PR #5518)
**Status:** Approved

---

## Background

PR #5518 introduced an experiment with multiple parallel wait event op versions (v1–v6) to benchmark
structured metadata and pre-classification strategies for `wait_event_type`. Benchmarking concluded:

- **v4** (pre-classified `wait_event_type` in log body, no structured metadata) delivers −83 to −90%
  p95 improvement on WaitEventsPanel duration by eliminating the Loki 500-series fallback retry loop.
- All SM-based versions (v2, v3, v5) introduce 25× cardinality increases that negate their benefits
  at wide unfiltered windows.
- **v6** is superseded by v4 for this scope.
- A Prometheus counter benchmarked externally shows −52 to −65% p50 on TotalWaitEventsTime at
  windows ≥1h30 unfiltered, by bypassing Loki series limits entirely.

This design formalises the outcome: adopt v4 as the production "v2", remove all other experiment
versions, and implement the Prometheus counter for MySQL.

---

## Goals

1. Rename the v4 op to `wait_event_v2` and make it the single alternative to v1.
2. Enforce mutual exclusion: when v2 is enabled, v1 is not emitted.
3. Clean up all intermediate experiment ops (old v2, v3, v5, v6) from both MySQL and Postgres.
4. Rename the feature flag from `enable_structured_metadata` to `enable_pre_classified_wait_events`.
5. Implement `database_observability_wait_event_seconds_total` Prometheus counter in MySQL only.

---

## Non-Goals

- No changes to query_association, query_sample, or any other op type.
- No structured metadata usage in the new v2.
- No Prometheus counter for Postgres.
- No changes to the Grafana app queries (handled separately as a follow-up).

---

## Design

### 1. Flag Rename

**Files:** `mysql/component.go`, `postgres/component.go`, and both `collector/query_samples.go`

`enable_structured_metadata` → `enable_pre_classified_wait_events` everywhere:

```go
// Arguments struct (both components)
EnablePreClassifiedWaitEvents bool `alloy:"enable_pre_classified_wait_events,attr,optional"`

// QuerySamples args/struct
EnablePreClassifiedWaitEvents bool
enablePreClassifiedWaitEvents bool
```

The Alloy config attribute name changes accordingly. Users who had `enable_structured_metadata = true`
will need to update their config to `enable_pre_classified_wait_events = true`.

---

### 2. Op Cleanup and Rename

**Constants (both MySQL and Postgres `query_samples.go`):**

Remove:
- `OP_WAIT_EVENT_V2` (`"wait_event_v2"` — old SM version)
- `OP_WAIT_EVENT_V3` (`"wait_event_v3"`)
- `OP_WAIT_EVENT_V5` (`"wait_event_v5"`)
- `OP_WAIT_EVENT_V6` (`"wait_event_v6"`)

Rename:
- `OP_WAIT_EVENT_V4` → `OP_WAIT_EVENT_V2` with string `"wait_event_v2"`

**Log message format for new v2** (unchanged from v4 — includes `wait_event_type` pre-classified
in the log body):

MySQL:
```
schema="%s" user="%s" client_host="%s" thread_id="%s" digest="%s" event_id="%s"
wait_event_id="%s" wait_end_event_id="%s" wait_event_name="%s" wait_event_type="%s"
wait_object_name="%s" wait_object_type="%s" wait_time="%fms"
```

Postgres:
```
datname="%s" pid="%d" leader_pid="%s" user="%s" backend_type="%s" state="%s"
xid="%d" xmin="%d" wait_time="%s" wait_event="%s" wait_event_name="%s"
wait_event_type="%s" blocked_by_pids="%v" queryid="%d"
```

Also remove all helper functions that only served removed ops:
- `classifyMySQLWaitEventType` — keep (used by new v2)
- `classifyPostgresWaitEventType` — keep (used by new v2)
- `buildWaitEventV3Labels`, `buildWaitEventV5Labels`, etc. — remove

---

### 3. Mutual Exclusion

Emission logic in both MySQL and Postgres `fetchQuerySamples` / `emitAndDeleteSample`:

```go
if c.enablePreClassifiedWaitEvents {
    // emit wait_event_v2 (pre-classified wait_event_type in log body)
} else {
    // emit wait_event (v1 baseline)
}
```

v1 and v2 are never emitted in the same run. No other emission paths change.

---

### 4. Prometheus Counter (MySQL only)

**Metric definition:**

```go
waitEventSecondsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "database_observability_wait_event_seconds_total",
        Help: "Total wait time in seconds per query, aggregated by server, query digest, and database.",
    },
    []string{"server_id", "digest", "schema"},
)
```

**Labels:**
- `server_id` — from the component's target label
- `digest` — MySQL `digest` value
- `schema` — MySQL `schema` value

**Registration:** at MySQL component startup, registered into the existing `c.registry`
(`prometheus.NewRegistry()` already present in `mysql/component.go`).

**Passing to collector:** add `WaitEventCounter *prometheus.CounterVec` to `QuerySamplesArguments`
in `mysql/component.go`. The component creates, registers, and passes the counter down. The
`QuerySamples` struct holds a reference.

**Increment:** in the wait event emit loop in `mysql/collector/query_samples.go`, unconditionally
(regardless of v1/v2 flag), after processing each wait event row:

```go
if c.waitEventCounter != nil {
    waitTimeSeconds := waitTime / 1000.0 // waitTime is in ms (from picosecondsToMilliseconds)
    c.waitEventCounter.WithLabelValues(c.serverID, row.Digest.String, row.Schema.String).Add(waitTimeSeconds) // labels: server_id, digest, schema
}
```

The counter accumulates independently of which Loki op is being emitted. Postgres does not receive
this counter.

---

## File Changelist

| File | Change |
|---|---|
| `mysql/component.go` | Rename flag; create + register counter; pass to QuerySamples |
| `mysql/collector/query_samples.go` | Rename flag + op; remove old v2/v3/v5/v6 blocks; mutual exclusion; increment counter |
| `mysql/collector/query_samples_test.go` | Update tests for renamed flag/op; remove old version tests; add counter tests |
| `postgres/component.go` | Rename flag only |
| `postgres/collector/query_samples.go` | Rename flag + op; remove old v2/v3/v5/v6 blocks; mutual exclusion |
| `postgres/collector/query_samples_test.go` | Update tests for renamed flag/op; remove old version tests |
| `docs/sources/reference/components/database_observability/database_observability.mysql.md` | Update flag name and description |
| `docs/sources/reference/components/database_observability/database_observability.postgres.md` | Update flag name and description |

---

## Test Plan

- `TestQuerySamples_WaitEvents` (MySQL + Postgres): assert that with flag OFF only `wait_event` is
  emitted; with flag ON only `wait_event_v2` is emitted; never both.
- `TestWaitEventCounter` (MySQL): assert counter increments with correct `server_id`, `queryid`,
  `dbname` labels and correct seconds value; assert it increments regardless of v1/v2 flag state.
- Remove all test assertions for old v2/v3/v5/v6 ops.
