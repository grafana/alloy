# PostgreSQL Semantic Query Fingerprinting — 3-PR Stack

**Goal:** Introduce a stable, client-side semantic identifier for every observed query on `database_observability.postgres`, then attach it to every observability surface (errors, samples, wait events, parsed tables, EXPLAIN output) and on cross-surface join metrics. The identifier is computed from the parsed SQL AST via `libpg_query` (`pg_query_go/v6`) and is invariant across comments, whitespace, and literal-vs-placeholder differences — so the same value emerges from `pg_stat_statements.query`, raw `pg_stat_activity.query`, and server-log `STATEMENT:` continuations. This matters most on managed services (RDS, Aurora) that don't expose `pg_stat_statements.queryid` in `log_line_prefix`.

The work is split into three PRs that stack:

| # | Branch | What it ships |
|---|---|---|
| [#6180](https://github.com/grafana/alloy/pull/6180) | `gaantunes/pg-errors-by-fingerprint-counter` | New `internal/.../postgres/fingerprint` package. `op="error"` Loki entries pairing `ERROR`/`FATAL`/`PANIC` with their `STATEMENT:` continuations. New `logs_processing { enable_error_logs = true }` block. |
| [#6200](https://github.com/grafana/alloy/pull/6200) | `gaantunes/postgres-samples-fingerprint` | Versioned ops (`query_association_v2`, `query_sample_v2`, `wait_event_v3`, `wait_event_v4`) carrying `query_fingerprint`. New top-level `enable_query_fingerprint` argument. Pipeline observability counters. |
| [#6214](https://github.com/grafana/alloy/pull/6214) | `gaantunes/postgres-wait-event-seconds-total` | `database_observability_wait_event_seconds_total{query_fingerprint, datname}` counter (mirrors MySQL). `database_observability_pg_query_fingerprint_info` join gauge. `query_parsed_table_name_v2` and `explain_plan_output_v2` ops. |

Each layer is gated independently. The bottom PR is the only one that introduces the fingerprint package; the upper PRs reuse it.

---

## PR #6180 — `op="error"` Loki entries

### What it adds

- **`fingerprint` package** (`internal/component/database_observability/postgres/fingerprint/`). Three-stage pipeline: parse-as-is → quote/paren repair → sentinel hash. Two well-known sentinels (`<truncated query>` and `<unparsable query>`) for queries the parser can't accept.
- **`op="error"` Loki entry shape.** Each PostgreSQL `ERROR`/`FATAL`/`PANIC` log line is paired with the matching `STATEMENT:` continuation, fingerprinted, and emitted as a single logfmt-bodied entry on the component's `forward_to` receiver.
- **`logs_processing` block** with `enable_error_logs` flag, gating the whole pipeline.

### Configuration

```alloy
database_observability.postgres "x" {
  ...
  logs_processing {
    enable_error_logs = true
  }
}
```

`enable_error_logs` defaults to `false`. With the flag off the STATEMENT-pairing pipeline (timeout ticker, pending-error map, statement buffer, prefix-tail parser) does not run at all — the hot path stays at "increment `pg_errors_total`, return."

### Architecture

- PID-keyed `pendingErrors` map captures each ERROR/FATAL/PANIC at receive time, including prefix metadata (`backend_start`, `xid`, `session_id`, `application_name`, `client_addr`/`port`, `error_message`).
- `currentStatement` buffer accumulates the `STATEMENT:` continuation across the keyword line and the TAB-prefixed lines that follow.
- A 5s timeout ticker drops pending entries whose STATEMENT never arrived.
- **Pinned ordering invariant:** the ticker flushes the in-progress STATEMENT buffer BEFORE expiring pendings, so an ERROR + STATEMENT pair arriving right before the expiration deadline still emits.
- Displaced pendings (new ERROR for a PID while one's still pending) are dropped — only `pg_errors_total` counts them. This keeps the op count strictly equal to "errors with captured SQL."

### Loki line shape

```
op="error"
body: level="info" severity=ERROR sqlstate=40001 sqlstate_class=40 xid=42 datname=books_store \
      query_fingerprint=<16-hex> pid=12345 backend_start="2026-05-20 10:43:44 GMT" \
      application_name=[unknown] client_addr=127.0.0.1 client_port=60228 \
      session_id=69fa58c6.30 user=app-user error_message="could not serialize access ..."
```

The SQL text is **not** emitted on the `op="error"` line — consumers join to the `query_samples`/`query_details` streams (added in #6200) on `query_fingerprint` to recover the SQL.

---

## PR #6200 — versioned ops carrying `query_fingerprint`

### What it adds

- **Top-level `enable_query_fingerprint` argument** that gates the whole fingerprint pipeline on `query_samples` and `query_details`.
- **Versioned ops** that carry `query_fingerprint` alongside their pre-fingerprint payload. A given component instance emits exactly one variant per op family, never both:
  - `op="query_association"` → `op="query_association_v2"` (adds `query_fingerprint` between `queryid` and `querytext`)
  - `op="query_sample"` → `op="query_sample_v2"` (adds trailing `query_fingerprint`)
  - `op="wait_event"` → either `op="wait_event_v3"` (raw `wait_event_type`) or `op="wait_event_v4"` (pre-classified type) when `enable_query_fingerprint = true`
- **`s.query` always selected** from `pg_stat_activity` so the fingerprint can be computed regardless of `disable_query_redaction`. Only the *emission* of raw SQL on log lines is still gated by that flag.
- **Pipeline observability counters** (both emitted only when `enable_query_fingerprint = true`):
  - `database_observability_query_fingerprint_repaired_total` — counts queries whose fingerprint required the quote/paren repair heuristic.
  - `database_observability_query_fingerprint_sentinel_total{sentinel="truncated"|"unparsable"}` — counts queries that fell through to a sentinel hash.
- **`track_activity_query_size` read** moved from `component.go` into the `query_samples` collector's `Start` method — keeps `component.go` clean and concentrates the dependency where it's used.

### Fingerprint caching

The fingerprint is computed once per sample (cached on `SampleState.QueryFingerprint`) and reused for every wait-event line emitted under that same `SampleKey`. All entries belonging to one logical query execution share the same `query_fingerprint`.

### Wait-event op matrix

| `enable_query_fingerprint` | `enable_pre_classified_wait_events` | sample `op` | wait-event `op` |
|---|---|---|---|
| `false` (default) | `false` (default) | `query_sample` | `wait_event` |
| `false` | `true` | `query_sample` | `wait_event_v2` |
| `true` | `false` | `query_sample_v2` | `wait_event_v3` |
| `true` | `true` | `query_sample_v2` | `wait_event_v4` |

---

## PR #6214 — wait-event seconds counter + join gauge + remaining versioned ops

### What it adds

- **`database_observability_wait_event_seconds_total{query_fingerprint, datname}`** Prometheus counter. Sum of observed wait-event seconds per logical query, mirroring MySQL's existing `database_observability_wait_event_seconds_total{digest, schema}`.
  - **Alloy-observed time only.** The counter increments by `LastTimestamp - FirstObservedAt` for each emitted wait-event occurrence (a `FirstObservedAt` field added to `WaitEventOccurrence`), not by PG's `state_change`-anchored wait time. This bounds the value to time elapsed since the collector first saw the wait and prevents (a) crediting pre-collector time on startup and (b) multi-counting across wait events within a single query observation.
- **`database_observability_pg_query_fingerprint_info{queryid, query_fingerprint, datname}`** gauge with value `1`, refreshed each `query_details` scrape (`Reset()` per scrape). Exposes the same `queryid` ↔ `query_fingerprint` mapping already emitted on `op="query_association_v2"` as a PromQL-joinable series, so dashboards can pivot `pg_stat_statements_*` counters onto the fingerprint without LogQL round-trips. Uses the `_info` suffix per Prometheus convention for label-carrying gauges.
- **`op="query_parsed_table_name_v2"`** versioned op. Each (queryid, table) row emitted alongside `query_association_v2` carries the parent's `query_fingerprint` as a trailing field.
- **`op="explain_plan_output_v2"`** versioned op. Each EXPLAIN attempt carries `query_fingerprint` between `digest` and the base64-encoded plan output. Fingerprint is computed once in `newQueryInfo` and reused across every emission tied to that cached query (success, skipped, denylisted, or error).

### Why gate everything on `enable_query_fingerprint`?

The whole point of each new surface is the fingerprint label. With the flag off there is no fingerprint resolved on `SampleState` / `queryInfo` / the parsed-table loop, so a registered counter / gauge / `_v2` op would either not populate or carry an empty `query_fingerprint=""` value — useless either way. Gating keeps these out of `/metrics` and the Loki stream entirely until they can carry meaningful labels.

---

## Cross-surface joins

`query_fingerprint` is the canonical identity for "one logical query" across every PostgreSQL observability surface this stack introduces:

- `op="error"` (from #6180)
- `op="query_association_v2"` / `op="query_parsed_table_name_v2"` (from #6200 + #6214)
- `op="query_sample_v2"` / `op="wait_event_v3"` / `op="wait_event_v4"` (from #6200)
- `op="explain_plan_output_v2"` (from #6214)
- `database_observability_wait_event_seconds_total` (from #6214)
- `database_observability_pg_query_fingerprint_info` (from #6214 — join key gauge)

### Recipe — error rate per logical query

`pg_stat_statements.calls` counts only **successful** executions, so:

```
error_rate = errors / (errors + successful_calls)
```

Two-step query because the fingerprint isn't a direct Prometheus label on `pg_stat_statements_*`:

1. **LogQL** — get the queryids belonging to a fingerprint:
   ```logql
   {op="query_association_v2"} | logfmt | query_fingerprint="<fp>"
   ```
2. **PromQL** — sum the per-queryid call counts using `max_over_time` (queryids may rotate before `rate` sees enough delta):
   ```promql
   sum(max_over_time(pg_stat_statements_calls_total{queryid=~"<id1>|<id2>|..."}[12h]))
   ```

After #6214 lands, the LogQL step collapses to a PromQL join through `pg_query_fingerprint_info`:

```promql
sum by (query_fingerprint) (
  pg_stat_statements_calls_total
    * on(queryid, datname) group_left(query_fingerprint)
    database_observability_pg_query_fingerprint_info
)
```

### Recipe — wait time per logical query (from #6214)

```promql
sum by (query_fingerprint) (rate(database_observability_wait_event_seconds_total{datname="<db>"}[5m]))
```

---

## Backward compatibility

- `enable_error_logs = false` AND `enable_query_fingerprint = false` (both defaults) preserve every pre-existing behavior of the component byte-for-byte.
- Tenants that flip either flag opt into additive emission only — no existing op shape, label set, or metric is removed or renamed.
- A given component instance emits exactly one variant of each op family. Mixed legacy and `_v2`/`_v3`/`_v4` ops in the same tenant indicate a partial rollout across multiple `alloy` instances.

---

## Files touched (cumulative across the stack)

**New:**
- `internal/component/database_observability/postgres/fingerprint/fingerprint.go` (+ tests)

**Modified:**
- `internal/component/database_observability/postgres/component.go` — new flag(s), arg plumbing, registry threading
- `internal/component/database_observability/postgres/collector/logs.go` — STATEMENT pairing + `op="error"` emission
- `internal/component/database_observability/postgres/collector/query_samples.go` — fingerprint caching, versioned ops, wait-event-seconds counter, pipeline counters, `track_activity_query_size` read
- `internal/component/database_observability/postgres/collector/query_details.go` — `query_association_v2`, `query_parsed_table_name_v2`, `pg_query_fingerprint_info` gauge
- `internal/component/database_observability/postgres/collector/explain_plans.go` — `explain_plan_output_v2`
- `docs/sources/reference/components/database_observability/database_observability.postgres.md` — argument and block additions

**Unchanged invariants:**
- `database_observability_pg_errors_total` Prometheus counter (low-cardinality, both bases keep it).
- `op="query_association"` / `op="query_sample"` / `op="wait_event"` / `op="wait_event_v2"` / `op="query_parsed_table_name"` / `op="explain_plan_output"` line shapes when their respective flags are off.

---

## Test coverage

Per PR:
- **#6180** — happy path, displaced pending, no-STATEMENT expiration, statement-survives-timeout-flush (10x stress under parallel contention), flag-off path, bare-keyword continuations.
- **#6200** — versioned op shapes for each of the matrix cells, fingerprint caching across sample/wait events, truncation-sentinel and unparsable-sentinel paths firing the right pipeline counter, no-fingerprint-field-when-disabled invariants for each op family.
- **#6214** — wait-event-seconds counter increments by alloy-observed duration (not state_change-anchored), counter not registered when flag off, info gauge populates and resets across scrapes, `query_parsed_table_name_v2` and `explain_plan_output_v2` carry the parent's fingerprint, info gauge unregister on Stop.

Cross-PR manual verification on the `alloy-exp` playground deployment after each image bump.
