# PostgreSQL Semantic Query Fingerprint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a semantic query fingerprint to `database_observability.postgres` so the same logical SQL produces the same identifier across `pg_stat_statements`, `pg_stat_activity`, and PostgreSQL log lines — including on managed services (RDS, Aurora) where the native `queryid` is not available in logs.

**Architecture:**
1. A small `fingerprint` package wraps [pg_query_go](https://github.com/pganalyze/pg_query_go) with the pganalyze-style three-stage pipeline: parse-as-is → repair-and-retry → sentinel fallback.
2. A `QueryHashRegistry` (queryid → fingerprint+datname, LRU+TTL) is populated by the `query_details` collector while it scrapes `pg_stat_statements`. A `QueryHashMetricsCollector` exposes the registry as a Prometheus join metric `database_observability_query_hash_info{queryid, query_fingerprint, server_id, datname} = 1`. Dashboards do `pg_stat_statements_X * on(queryid, datname) group_left(query_fingerprint) database_observability_query_hash_info` to attach the fingerprint to existing series with no cardinality bump on the source metrics.
3. The `query_samples` collector consults the registry first (registry fingerprint comes from un-truncated text), falling back to fingerprinting the truncated `pg_stat_activity.query` text using the same pipeline with the truncation sentinel enabled.
4. The `logs` collector is extended to (a) buffer ERROR + STATEMENT continuation pairs so the SQL behind an error is captured, and (b) parse slow-query `LOG: duration: N ms statement: ...` lines. Both emit Loki entries with `query_fingerprint` carried as Loki structured metadata so cardinality stays out of the index.

**Recommendation followed:** Option 1 + Option B from `docs/design/XXXX-semantic-query-fingerprint.md` (compute via pg_query AST, augment alongside `queryid` rather than replace it).

**Tech Stack:** Go, CGo (libpg_query via pg_query_go/v6), `prometheus/client_golang`, `grafana/loki/pkg/push`, `hashicorp/golang-lru/v2/expirable`.

---

## File Structure

**New files:**
- `internal/component/database_observability/postgres/fingerprint/fingerprint.go` — three-stage `Fingerprint` function, sentinel constants, `Source` enum.
- `internal/component/database_observability/postgres/fingerprint/fingerprint_test.go` — table-driven unit tests.
- `internal/component/database_observability/postgres/collector/query_hash_registry.go` — LRU+TTL `queryid → fingerprint, datname` map.
- `internal/component/database_observability/postgres/collector/query_hash_registry_test.go`
- `internal/component/database_observability/postgres/collector/query_hash_metrics.go` — `prometheus.Collector` exposing `query_hash_info` from the registry.
- `internal/component/database_observability/postgres/collector/query_hash_metrics_test.go`

**Modified files:**
- `go.mod`, `go.sum`, `collector/go.mod`, `collector/go.sum`, `extension/alloyengine/go.mod`, `extension/alloyengine/go.sum` — add `github.com/pganalyze/pg_query_go/v6`.
- `internal/component/database_observability/loki_entry.go` — add `BuildLokiEntryWithStructuredMetadata` variant.
- `internal/component/database_observability/loki_entry_test.go` — cover the new variant.
- `internal/component/database_observability/postgres/component.go` — construct registry + metrics collector, plumb into collectors, fetch `track_activity_query_size`.
- `internal/component/database_observability/postgres/collector/query_details.go` — replace ad-hoc normalization with the fingerprint pipeline; populate the registry; emit `query_fingerprint=` on `OP_QUERY_ASSOCIATION` entries.
- `internal/component/database_observability/postgres/collector/query_details_test.go` — fingerprint assertions.
- `internal/component/database_observability/postgres/collector/query_samples.go` — registry lookup + fallback fingerprinting; emit `query_fingerprint=` on `OP_QUERY_SAMPLE`, `OP_WAIT_EVENT`, `OP_WAIT_EVENT_V2` entries.
- `internal/component/database_observability/postgres/collector/query_samples_test.go` — assertions for fingerprint label.
- `internal/component/database_observability/postgres/collector/logs.go` — multi-line ERROR/STATEMENT buffering, slow-query log parsing, Loki emission with `query_fingerprint` structured metadata, `pg_errors_total` gains a `query_fingerprint` label.
- `internal/component/database_observability/postgres/collector/logs_test.go` — error+STATEMENT and slow-query test cases.
- `docs/sources/reference/components/database_observability/database_observability.postgres.md` — document the new metric and structured-metadata field.

**File responsibilities:**
- `fingerprint/` is *only* about turning text into a stable identifier; it must not import anything from `database_observability` so it can be reused later (e.g. for MySQL via a different parser).
- `query_hash_registry.go` is shared state; `query_details.go` writes, `query_samples.go` and `query_hash_metrics.go` read.
- `query_hash_metrics.go` is the only place that talks to `prometheus.Collector` for the join metric — keep it small (mirrors the pattern of `connection_info.go`).
- The logs collector is already on the larger side (~340 lines); the new buffering logic goes into a new private struct/method on the same file rather than creating a new file, since it's tightly coupled to `parseTextLog`.

---

## Phase 0 — Dependency and fingerprint package

### Task 1: Add `pg_query_go/v6` dependency

**Files:**
- Modify: `go.mod`, `collector/go.mod`, `extension/alloyengine/go.mod`
- Modify: `go.sum`, `collector/go.sum`, `extension/alloyengine/go.sum`

- [ ] **Step 1: Add the require entry to the top-level `go.mod`**

In `go.mod`, locate the existing `require ( ... )` block that already has named direct deps such as `buf.build/gen/...`, and add:

```go
require (
    github.com/pganalyze/pg_query_go/v6 v6.1.0
)
```

(Use a separate `require` block — matches the style already used for `buf.build/...` per PR #5354.)

- [ ] **Step 2: Run `go mod tidy` from the repo root**

Run: `cd /home/gaantunes/git/alloy && go mod tidy`
Expected: `go.sum` is updated with `github.com/pganalyze/pg_query_go/v6 v6.1.0` lines and the protobuf transitive bump.

- [ ] **Step 3: Tidy the sub-modules**

Run:
```bash
cd /home/gaantunes/git/alloy/collector && go mod tidy
cd /home/gaantunes/git/alloy/extension/alloyengine && go mod tidy
```
Expected: `pg_query_go/v6` appears as `// indirect` in those go.mod files.

- [ ] **Step 4: Verify the binary still builds (CGo path)**

Run: `cd /home/gaantunes/git/alloy && CGO_ENABLED=1 go build ./...`
Expected: Build succeeds. If a CGo toolchain is missing, install `build-essential` (Linux) and re-run.

- [ ] **Step 5: Smoke-check that `pg_query.Fingerprint` is usable**

Add a temporary `cmd/_smoke/fingerprint_smoke/main.go`:
```go
package main

import (
    "fmt"

    pg_query "github.com/pganalyze/pg_query_go/v6"
)

func main() {
    fp, err := pg_query.Fingerprint("SELECT 1")
    if err != nil {
        panic(err)
    }
    fmt.Println(fp)
}
```

Run: `cd /home/gaantunes/git/alloy && go run ./cmd/_smoke/fingerprint_smoke`
Expected: A 16-character hex fingerprint is printed.

- [ ] **Step 6: Delete the smoke binary and commit**

Run:
```bash
rm -rf /home/gaantunes/git/alloy/cmd/_smoke
git add go.mod go.sum collector/go.mod collector/go.sum extension/alloyengine/go.mod extension/alloyengine/go.sum
git commit -m "feat(deps): add pg_query_go for postgres semantic query fingerprinting"
```

---

### Task 2: Create the `fingerprint` package

**Files:**
- Create: `internal/component/database_observability/postgres/fingerprint/fingerprint.go`
- Create: `internal/component/database_observability/postgres/fingerprint/fingerprint_test.go`

- [ ] **Step 1: Write the failing test file**

Create `internal/component/database_observability/postgres/fingerprint/fingerprint_test.go`:

```go
package fingerprint

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFingerprint_StableAcrossCommentsAndWhitespace(t *testing.T) {
    a, _, errA := Fingerprint("SELECT * FROM users WHERE id = $1 -- foo", SourcePgStatStatements, 0)
    require.NoError(t, errA)
    b, _, errB := Fingerprint("SELECT *\nFROM users\nWHERE id = $1 /* bar */", SourcePgStatStatements, 0)
    require.NoError(t, errB)
    assert.Equal(t, a, b)
}

func TestFingerprint_DifferentForDifferentQueries(t *testing.T) {
    a, _, _ := Fingerprint("SELECT 1", SourcePgStatStatements, 0)
    b, _, _ := Fingerprint("SELECT 2", SourcePgStatStatements, 0)
    assert.NotEqual(t, a, b)
}

func TestFingerprint_RepairUnclosedQuotes(t *testing.T) {
    fp, repaired, err := Fingerprint("SELECT * FROM t WHERE name = 'oh no", SourceLog, 0)
    require.NoError(t, err)
    assert.True(t, repaired, "should report that repair was used")
    assert.NotEqual(t, "", fp)
}

func TestFingerprint_RepairUnclosedParens(t *testing.T) {
    fp, repaired, err := Fingerprint("SELECT * FROM t WHERE id IN (1, 2, 3", SourceLog, 0)
    require.NoError(t, err)
    assert.True(t, repaired)
    assert.NotEqual(t, "", fp)
}

func TestFingerprint_TruncatedSentinelOnPgStatActivity(t *testing.T) {
    // Build a malformed query whose length equals trackActivityQuerySize-1.
    const trackSize = 1024
    bad := makeUnparsableOfLen(trackSize - 1)

    fp, _, err := Fingerprint(bad, SourcePgStatActivity, trackSize)
    require.NoError(t, err)
    assert.Equal(t, FingerprintOf(SentinelTruncated), fp)
}

func TestFingerprint_UnparsableSentinel(t *testing.T) {
    // Garbage that cannot be repaired and is not at the activity buffer limit.
    fp, _, err := Fingerprint("$$$ not sql at all $$$", SourcePgStatStatements, 0)
    require.NoError(t, err)
    assert.Equal(t, FingerprintOf(SentinelUnparsable), fp)
}

func TestFingerprint_EmptyAndNullInputs(t *testing.T) {
    _, _, err := Fingerprint("", SourcePgStatStatements, 0)
    assert.Error(t, err, "empty input should error so callers can skip emitting")
}

// makeUnparsableOfLen returns an unparsable string of exactly n bytes (helps
// exercise the trackActivityQuerySize-1 sentinel path).
func makeUnparsableOfLen(n int) string {
    s := "SELECT * FROM t WHERE x IN ("
    for len(s) < n {
        s += "1,"
    }
    return s[:n]
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/fingerprint/...`
Expected: FAIL — package has no implementation yet.

- [ ] **Step 3: Implement the fingerprint package**

Create `internal/component/database_observability/postgres/fingerprint/fingerprint.go`:

```go
// Package fingerprint computes stable, semantic SQL fingerprints for PostgreSQL
// query text using the libpg_query parser (via pg_query_go).
//
// The same fingerprint is produced by Alloy regardless of comments, whitespace,
// or literal-vs-placeholder differences, allowing pg_stat_statements metrics,
// pg_stat_activity samples, and server-log query text to be correlated by a
// single client-side identifier — including on managed services (RDS, Aurora)
// that do not expose pg_stat_statements.queryid in log_line_prefix.
package fingerprint

import (
    "errors"
    "strings"

    pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Source describes which PostgreSQL surface the query text was read from.
// It only affects the sentinel chosen when both parse and repair fail.
type Source int

const (
    SourcePgStatStatements Source = iota
    SourcePgStatActivity
    SourceLog
)

// Sentinel strings used when query text cannot be parsed even after repair.
// These match the sentinels used by pganalyze's collector so existing
// dashboards built around their values port over without changes.
const (
    SentinelTruncated  = "<truncated query>"
    SentinelUnparsable = "<unparsable query>"
)

// ErrEmpty is returned when the input is empty or whitespace-only. Callers
// should skip emitting fingerprints for these (don't even use a sentinel).
var ErrEmpty = errors.New("fingerprint: empty query text")

// Fingerprint runs the three-stage pipeline:
//  1. Parse the input as-is.
//  2. If parsing fails, balance unclosed quotes and parentheses and retry.
//  3. If parsing still fails, return a sentinel fingerprint.
//
// trackActivityQuerySize is the value of the postgres setting
// `track_activity_query_size` and is only consulted when source ==
// SourcePgStatActivity. Pass 0 for other sources.
//
// The returned `repaired` flag reports whether stage 2 was needed; callers may
// log this as a metric to detect upstream truncation issues.
func Fingerprint(query string, source Source, trackActivityQuerySize int) (fp string, repaired bool, err error) {
    if strings.TrimSpace(query) == "" {
        return "", false, ErrEmpty
    }

    if fp, perr := pg_query.Fingerprint(query); perr == nil {
        return fp, false, nil
    }

    fixed := repair(query)
    if fp, perr := pg_query.Fingerprint(fixed); perr == nil {
        return fp, true, nil
    }

    return sentinelFingerprint(query, source, trackActivityQuerySize), true, nil
}

// FingerprintOf hashes a known sentinel string deterministically. Exported so
// tests and callers can compare against the values produced for sentinels.
func FingerprintOf(text string) string {
    fp, _ := pg_query.Fingerprint(text)
    if fp != "" {
        return fp
    }
    // pg_query.Fingerprint may not parse a non-SQL sentinel; fall back to a
    // deterministic hash of the text. pg_query.HashXXH3_64 is what pganalyze
    // uses for static texts (util/fingerprint.go).
    return formatHash(pg_query.HashXXH3_64([]byte(text), 0xee))
}

func sentinelFingerprint(query string, source Source, trackActivityQuerySize int) string {
    if source == SourcePgStatActivity && trackActivityQuerySize > 0 && len(query) == trackActivityQuerySize-1 {
        return FingerprintOf(SentinelTruncated)
    }
    return FingerprintOf(SentinelUnparsable)
}

// repair closes unclosed single/double quotes and balances unclosed
// parentheses, mirroring pganalyze's `fixTruncatedQuery`. The repaired text
// is only used for fingerprint computation — it is not emitted anywhere.
func repair(query string) string {
    if strings.Count(query, "'")%2 == 1 {
        query += "'"
    }
    if strings.Count(query, "\"")%2 == 1 {
        query += "\""
    }
    open := strings.Count(query, "(") - strings.Count(query, ")")
    for i := 0; i < open; i++ {
        query += ")"
    }
    return query
}

func formatHash(h uint64) string {
    const hexChars = "0123456789abcdef"
    out := make([]byte, 16)
    for i := 15; i >= 0; i-- {
        out[i] = hexChars[h&0xF]
        h >>= 4
    }
    return string(out)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/fingerprint/... -v`
Expected: PASS for all 7 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/fingerprint/
git commit -m "feat(database_observability/postgres): add semantic query fingerprint package"
```

---

## Phase 1 — Metrics side: query_hash_info join metric

### Task 3: Implement `QueryHashRegistry`

**Files:**
- Create: `internal/component/database_observability/postgres/collector/query_hash_registry.go`
- Create: `internal/component/database_observability/postgres/collector/query_hash_registry_test.go`

- [ ] **Step 1: Write the failing test**

Create `query_hash_registry_test.go`:

```go
package collector

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestQueryHashRegistry_SetAndGet(t *testing.T) {
    r := NewQueryHashRegistry(100, time.Hour)
    r.Set("123", "fp_a", "books")

    info, ok := r.Get("123")
    require.True(t, ok)
    assert.Equal(t, "fp_a", info.Fingerprint)
    assert.Equal(t, "books", info.DatabaseName)

    _, ok = r.Get("missing")
    assert.False(t, ok)
}

func TestQueryHashRegistry_Snapshot(t *testing.T) {
    r := NewQueryHashRegistry(100, time.Hour)
    r.Set("1", "fp_a", "db1")
    r.Set("2", "fp_b", "db2")

    snap := r.Snapshot()
    assert.Len(t, snap, 2)
    assert.Equal(t, "fp_a", snap["1"].Fingerprint)
    assert.Equal(t, "fp_b", snap["2"].Fingerprint)
}

func TestQueryHashRegistry_TTLEviction(t *testing.T) {
    r := NewQueryHashRegistry(100, 50*time.Millisecond)
    r.Set("1", "fp_a", "db")
    time.Sleep(80 * time.Millisecond)
    _, ok := r.Get("1")
    assert.False(t, ok, "entry should have expired")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestQueryHashRegistry`
Expected: FAIL — `NewQueryHashRegistry` undefined.

- [ ] **Step 3: Implement the registry**

Create `query_hash_registry.go`:

```go
package collector

import (
    "sync"
    "time"

    "github.com/hashicorp/golang-lru/v2/expirable"
)

// QueryHashInfo holds the per-queryid data the metrics collector needs at
// scrape time. The fields are intentionally narrow: anything more goes on the
// emitted Loki entry, not on the join metric (cardinality control).
type QueryHashInfo struct {
    Fingerprint  string
    DatabaseName string
    LastSeen     time.Time
}

// QueryHashRegistry is a small LRU+TTL cache mapping the native PostgreSQL
// queryid (string form) to the semantic fingerprint Alloy computed from that
// query's text. The query_details collector populates it on each scrape; the
// query_samples collector and the query_hash_info Prometheus collector read
// from it.
//
// The size cap matches the pg_stat_statements.max default (5000 → 1000 hot
// entries is plenty for typical fleets); TTL ensures stale queryids don't
// keep being exported after the database evicts them.
type QueryHashRegistry struct {
    mu    sync.RWMutex
    cache *expirable.LRU[string, QueryHashInfo]
}

func NewQueryHashRegistry(size int, ttl time.Duration) *QueryHashRegistry {
    return &QueryHashRegistry{
        cache: expirable.NewLRU[string, QueryHashInfo](size, nil, ttl),
    }
}

func (r *QueryHashRegistry) Set(queryID, fingerprint, databaseName string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.cache.Add(queryID, QueryHashInfo{
        Fingerprint:  fingerprint,
        DatabaseName: databaseName,
        LastSeen:     time.Now(),
    })
}

func (r *QueryHashRegistry) Get(queryID string) (QueryHashInfo, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.cache.Get(queryID)
}

// Snapshot returns a shallow copy of all live entries. Used by the metrics
// collector at scrape time; not on the hot path of any collector.
func (r *QueryHashRegistry) Snapshot() map[string]QueryHashInfo {
    r.mu.RLock()
    defer r.mu.RUnlock()

    out := make(map[string]QueryHashInfo, r.cache.Len())
    for _, k := range r.cache.Keys() {
        if v, ok := r.cache.Peek(k); ok {
            out[k] = v
        }
    }
    return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestQueryHashRegistry -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/collector/query_hash_registry.go internal/component/database_observability/postgres/collector/query_hash_registry_test.go
git commit -m "feat(database_observability/postgres): add QueryHashRegistry"
```

---

### Task 4: Implement `QueryHashMetricsCollector`

**Files:**
- Create: `internal/component/database_observability/postgres/collector/query_hash_metrics.go`
- Create: `internal/component/database_observability/postgres/collector/query_hash_metrics_test.go`

- [ ] **Step 1: Write the failing test**

Create `query_hash_metrics_test.go`:

```go
package collector

import (
    "strings"
    "testing"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/testutil"
    "github.com/stretchr/testify/require"
)

func TestQueryHashMetricsCollector_ExposesRegistryAsJoinMetric(t *testing.T) {
    reg := prometheus.NewRegistry()
    qhr := NewQueryHashRegistry(100, time.Hour)
    col := NewQueryHashMetricsCollector(qhr, "server-A")
    require.NoError(t, reg.Register(col))

    qhr.Set("12345", "fpAAAA", "books_store")
    qhr.Set("67890", "fpBBBB", "library")

    expected := `
# HELP database_observability_query_hash_info Mapping of PostgreSQL queryid to semantic query fingerprint
# TYPE database_observability_query_hash_info gauge
database_observability_query_hash_info{datname="books_store",query_fingerprint="fpAAAA",queryid="12345",server_id="server-A"} 1
database_observability_query_hash_info{datname="library",query_fingerprint="fpBBBB",queryid="67890",server_id="server-A"} 1
`

    require.NoError(t, testutil.GatherAndCompare(
        reg,
        strings.NewReader(expected),
        "database_observability_query_hash_info",
    ))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestQueryHashMetricsCollector`
Expected: FAIL — `NewQueryHashMetricsCollector` undefined.

- [ ] **Step 3: Implement the collector**

Create `query_hash_metrics.go`:

```go
package collector

import (
    "github.com/prometheus/client_golang/prometheus"
)

// QueryHashMetricsCollector exposes the QueryHashRegistry as a Prometheus
// "info" gauge metric. The metric is intended to be join-merged into existing
// pg_stat_statements series via PromQL:
//
//   pg_stat_statements_calls_total
//     * on(queryid, datname) group_left(query_fingerprint)
//       database_observability_query_hash_info
//
// We deliberately keep the fingerprint *off* the existing pg_stat_statements
// series labels so we don't bump cardinality on every scrape.
type QueryHashMetricsCollector struct {
    registry *QueryHashRegistry
    serverID string
    desc     *prometheus.Desc
}

func NewQueryHashMetricsCollector(registry *QueryHashRegistry, serverID string) *QueryHashMetricsCollector {
    return &QueryHashMetricsCollector{
        registry: registry,
        serverID: serverID,
        desc: prometheus.NewDesc(
            prometheus.BuildFQName("database_observability", "", "query_hash_info"),
            "Mapping of PostgreSQL queryid to semantic query fingerprint",
            []string{"queryid", "query_fingerprint", "server_id", "datname"},
            nil,
        ),
    }
}

func (c *QueryHashMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- c.desc
}

func (c *QueryHashMetricsCollector) Collect(ch chan<- prometheus.Metric) {
    for queryID, info := range c.registry.Snapshot() {
        ch <- prometheus.MustNewConstMetric(
            c.desc,
            prometheus.GaugeValue,
            1,
            queryID,
            info.Fingerprint,
            c.serverID,
            info.DatabaseName,
        )
    }
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestQueryHashMetricsCollector -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/collector/query_hash_metrics.go internal/component/database_observability/postgres/collector/query_hash_metrics_test.go
git commit -m "feat(database_observability/postgres): expose query_hash_info join metric"
```

---

### Task 5: Wire fingerprinting into `query_details` collector

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/query_details.go`
- Modify: `internal/component/database_observability/postgres/collector/query_details_test.go`

- [ ] **Step 1: Write the failing test**

Append to `query_details_test.go` (read the existing file to find the right helper to reuse):

```go
func TestQueryDetails_PopulatesRegistryAndEmitsFingerprint(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()

    mock.ExpectQuery(`pg_stat_statements`).WillReturnRows(
        sqlmock.NewRows([]string{"queryid", "query", "datname"}).
            AddRow("9876", "SELECT * FROM books WHERE id = $1", "books_store"),
    )

    entries := loki_fake.NewClient(func() {})
    registry := NewQueryHashRegistry(100, time.Hour)

    qd, err := NewQueryDetails(QueryDetailsArguments{
        DB:                db,
        CollectInterval:   time.Hour,
        StatementsLimit:   100,
        EntryHandler:      entries,
        QueryHashRegistry: registry,
        Logger:            log.NewNopLogger(),
    })
    require.NoError(t, err)

    require.NoError(t, qd.fetchAndAssociate(context.Background()))

    info, ok := registry.Get("9876")
    require.True(t, ok)
    assert.NotEmpty(t, info.Fingerprint)
    assert.Equal(t, "books_store", info.DatabaseName)

    require.Eventually(t, func() bool { return len(entries.Received()) > 0 }, time.Second, 10*time.Millisecond)
    line := entries.Received()[0].Line
    assert.Contains(t, line, `queryid="9876"`)
    assert.Contains(t, line, `query_fingerprint="`+info.Fingerprint+`"`)
}
```

(Use whichever fake-loki helper the file already imports; the existing tests in this package tell you which.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestQueryDetails_PopulatesRegistry`
Expected: FAIL — `QueryHashRegistry` field missing on `QueryDetailsArguments`, line missing fingerprint.

- [ ] **Step 3: Modify `QueryDetailsArguments` and the struct to accept the registry**

In `query_details.go`, add the field to both the args and the struct:

```go
type QueryDetailsArguments struct {
    // ...existing fields...
    QueryHashRegistry *QueryHashRegistry
    // ...
}

type QueryDetails struct {
    // ...existing fields...
    queryHashRegistry *QueryHashRegistry
    // ...
}
```

Wire it through `NewQueryDetails` (alongside the other fields).

- [ ] **Step 4: Update `fetchAndAssociate` to compute the fingerprint and emit it**

Locate the loop body in `fetchAndAssociate` that builds the `OP_QUERY_ASSOCIATION` log line and replace the relevant block (around `queryText, err = removeComments(...)`) with:

```go
fp, _, fpErr := fingerprint.Fingerprint(queryText, fingerprint.SourcePgStatStatements, 0)
if fpErr != nil {
    level.Debug(c.logger).Log("msg", "skip fingerprint", "queryid", queryID, "err", fpErr)
}

if fp != "" && c.queryHashRegistry != nil {
    c.queryHashRegistry.Set(queryID, fp, string(databaseName))
}

queryText, err = removeComments(c.normalizer, queryText)
if err != nil {
    level.Error(c.logger).Log("msg", "failed to remove comments", "err", err)
    continue
}

c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
    logging.LevelInfo,
    OP_QUERY_ASSOCIATION,
    fmt.Sprintf(`queryid="%s" query_fingerprint="%s" querytext=%q datname="%s"`,
        queryID, fp, queryText, databaseName),
)
```

Add the import `"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"`.

- [ ] **Step 5: Run tests**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestQueryDetails -v`
Expected: PASS for both the new test and the existing `TestQueryDetails_*` cases (you may need to update existing tests' expected log lines to include `query_fingerprint=`).

- [ ] **Step 6: Commit**

```bash
git add internal/component/database_observability/postgres/collector/query_details.go internal/component/database_observability/postgres/collector/query_details_test.go
git commit -m "feat(database_observability/postgres): populate query hash registry from pg_stat_statements"
```

---

### Task 6: Wire metrics collector into `component.go`

**Files:**
- Modify: `internal/component/database_observability/postgres/component.go`

- [ ] **Step 1: Add the registry to the component struct**

In `component.go`, add the field to the `Component` struct (next to `dbConnection`):

```go
queryHashRegistry *collector.QueryHashRegistry
```

- [ ] **Step 2: Construct the registry and metrics collector during reconnection**

In `connectAndStartCollectors` (just before the `if len(c.args.Targets) == 0 {` block), construct it once per reconnect:

```go
const queryHashRegistrySize = 5000          // matches pg_stat_statements.max default ceiling
const queryHashRegistryTTL = 30 * time.Minute
c.queryHashRegistry = collector.NewQueryHashRegistry(queryHashRegistrySize, queryHashRegistryTTL)

queryHashMetrics := collector.NewQueryHashMetricsCollector(c.queryHashRegistry, generatedSystemID)
if err := c.registry.Register(queryHashMetrics); err != nil {
    return fmt.Errorf("failed to register query_hash_info metrics: %w", err)
}
c.exporterCollectors = append(c.exporterCollectors, queryHashMetrics)
```

(`exporterCollectors` is already cleaned up by `cleanupExporterCollectors`; the metrics collector implements `prometheus.Collector` so it fits the slice without changes.)

- [ ] **Step 3: Pass the registry to `query_details`**

In the `if collectors[collector.QueryDetailsCollector] {` block, add `QueryHashRegistry: c.queryHashRegistry,` to the `collector.QueryDetailsArguments{...}` literal.

- [ ] **Step 4: Build and run the existing component tests**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/...`
Expected: All existing tests pass; if `component_test.go` instantiates `Component` directly, you may need to expose a constructor accepting a custom registry (skip if not required).

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/component.go
git commit -m "feat(database_observability/postgres): register query_hash_info collector"
```

---

## Phase 2 — query_samples integration

### Task 7: Fetch `track_activity_query_size` once per reconnect

**Files:**
- Modify: `internal/component/database_observability/postgres/component.go`
- Modify: `internal/component/database_observability/postgres/collector/query_samples.go`

- [ ] **Step 1: Pull `track_activity_query_size` during reconnect**

In `connectAndStartCollectors` (right after the `selectServerInfo` query), add:

```go
var trackActivityQuerySize int
{
    var raw sql.NullString
    if err := dbConnection.QueryRowContext(ctx, "SELECT setting FROM pg_settings WHERE name = 'track_activity_query_size'").Scan(&raw); err != nil {
        level.Warn(c.opts.Logger).Log("msg", "failed to read track_activity_query_size; truncation sentinel will not fire", "err", err)
    } else if raw.Valid {
        if v, err := strconv.Atoi(raw.String); err == nil {
            trackActivityQuerySize = v
        }
    }
}
```

(Add `"strconv"` to the imports.) Pass `trackActivityQuerySize` into `collector.QuerySamplesArguments`.

- [ ] **Step 2: Add the field to `QuerySamplesArguments` and the struct**

In `query_samples.go`:

```go
type QuerySamplesArguments struct {
    // ...existing...
    QueryHashRegistry        *QueryHashRegistry
    TrackActivityQuerySize   int
}

type QuerySamples struct {
    // ...existing...
    queryHashRegistry      *QueryHashRegistry
    trackActivityQuerySize int
}
```

Wire through `NewQuerySamples`.

- [ ] **Step 3: Pass the registry from `component.go`**

In the `if collectors[collector.QuerySamplesCollector] {` block, add:
```go
QueryHashRegistry:      c.queryHashRegistry,
TrackActivityQuerySize: trackActivityQuerySize,
```

- [ ] **Step 4: Run existing tests**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/...`
Expected: PASS — no behavior change yet, just new args.

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/component.go internal/component/database_observability/postgres/collector/query_samples.go
git commit -m "feat(database_observability/postgres): plumb track_activity_query_size to query_samples"
```

---

### Task 8: Emit `query_fingerprint` on query_sample / wait_event entries

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/query_samples.go`
- Modify: `internal/component/database_observability/postgres/collector/query_samples_test.go`

- [ ] **Step 1: Write the failing test**

In `query_samples_test.go`, add a new case (or extend an existing assertion) that drives a row through `fetchQuerySample` with `disableQueryRedaction=true` and a known query, then asserts the emitted log line contains `query_fingerprint="..."`. Use the existing test scaffolding for setting up `sqlmock`, the entry handler, and the registry. Compute the expected fingerprint via `fingerprint.Fingerprint`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestQuerySamples`
Expected: FAIL — emitted line does not contain `query_fingerprint=`.

- [ ] **Step 3: Add a fingerprint helper on `QuerySamples`**

In `query_samples.go`, add:

```go
// resolveQueryFingerprint returns the fingerprint to attach to a sample's
// emitted Loki entry. Preference order:
//  1. Registry hit on queryid (computed from un-truncated pg_stat_statements text).
//  2. Direct fingerprint of the sample's (possibly truncated) query text — only
//     when query redaction is disabled, since that's the only time we have text.
// Empty string is returned when neither path is available; callers should
// simply omit the label.
func (c *QuerySamples) resolveQueryFingerprint(sample QuerySamplesInfo) string {
    if c.queryHashRegistry != nil && sample.QueryID.Valid {
        if info, ok := c.queryHashRegistry.Get(strconv.FormatInt(sample.QueryID.Int64, 10)); ok {
            return info.Fingerprint
        }
    }
    if c.disableQueryRedaction && sample.Query.Valid {
        fp, _, err := fingerprint.Fingerprint(
            sample.Query.String,
            fingerprint.SourcePgStatActivity,
            c.trackActivityQuerySize,
        )
        if err == nil {
            return fp
        }
    }
    return ""
}
```

Add the imports `"strconv"` and `"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"`.

- [ ] **Step 4: Append `query_fingerprint=` to the three label-builder methods**

For `buildQuerySampleLabelsWithEnd`, `buildWaitEventLabels`, `buildWaitEventV2Labels`:
- Pass the fingerprint into the function (or read it from a field on `SampleState` set when the active sample is upserted; either is fine — the field approach avoids re-resolving on every wait-event emission).
- Append `query_fingerprint="<fp>"` to the format string only when non-empty.

For example, in `upsertActiveSample`:

```go
state.LastRow = sample
state.LastSeenAt = sample.Now
if state.QueryFingerprint == "" {
    state.QueryFingerprint = c.resolveQueryFingerprint(sample)
}
```

(Add `QueryFingerprint string` to `SampleState`.)

In each label builder, after the existing format string, do:

```go
if state.QueryFingerprint != "" {
    labels = fmt.Sprintf(`%s query_fingerprint="%s"`, labels, state.QueryFingerprint)
}
```

- [ ] **Step 5: Run tests**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestQuerySamples -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/component/database_observability/postgres/collector/query_samples.go internal/component/database_observability/postgres/collector/query_samples_test.go
git commit -m "feat(database_observability/postgres): emit query_fingerprint on query_sample and wait_event entries"
```

---

## Phase 3 — Logs integration

### Task 9: Add structured-metadata variant to `BuildLokiEntry`

**Files:**
- Modify: `internal/component/database_observability/loki_entry.go`
- Modify: `internal/component/database_observability/loki_entry_test.go`

- [ ] **Step 1: Write the failing test**

Append to `loki_entry_test.go`:

```go
func TestBuildLokiEntryWithStructuredMetadata(t *testing.T) {
    e := BuildLokiEntryWithStructuredMetadata(
        logging.LevelInfo,
        "op_test",
        `key="value"`,
        push.LabelsAdapter{
            push.LabelAdapter{Name: "query_fingerprint", Value: "abc123"},
        },
    )
    require.Equal(t, model.LabelValue("op_test"), e.Labels["op"])
    require.Equal(t, `level="info" key="value"`, e.Line)
    require.Len(t, e.StructuredMetadata, 1)
    require.Equal(t, "query_fingerprint", e.StructuredMetadata[0].Name)
    require.Equal(t, "abc123", e.StructuredMetadata[0].Value)
}
```

(Add the necessary imports.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/ -run TestBuildLokiEntryWithStructuredMetadata`
Expected: FAIL — function undefined.

- [ ] **Step 3: Add the variant**

In `loki_entry.go`, add:

```go
import "github.com/grafana/loki/pkg/push"

func BuildLokiEntryWithStructuredMetadata(level logging.Level, op, line string, metadata push.LabelsAdapter) loki.Entry {
    e := BuildLokiEntry(level, op, line)
    e.Entry.StructuredMetadata = metadata
    return e
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/ -run TestBuildLokiEntryWithStructuredMetadata -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/loki_entry.go internal/component/database_observability/loki_entry_test.go
git commit -m "feat(database_observability): add BuildLokiEntryWithStructuredMetadata variant"
```

---

### Task 10: Buffer ERROR + STATEMENT pairs in the logs collector

The current `parseTextLog` skips continuation lines (`STATEMENT:`, `QUERY:`, etc.) entirely. To attach a fingerprint to errors, we need to remember the prior ERROR row and stitch the SQL onto it when the matching `STATEMENT:` line arrives.

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs.go`
- Modify: `internal/component/database_observability/postgres/collector/logs_test.go`

- [ ] **Step 1: Write the failing test**

In `logs_test.go`, add:

```go
func TestLogs_AttachesQueryFingerprintToError(t *testing.T) {
    var (
        receiver  = loki.NewLogsReceiver()
        emitter   = loki_fake.NewClient(func() {})
        registry  = prometheus.NewRegistry()
    )
    l, err := NewLogs(LogsArguments{
        Receiver:     receiver,
        EntryHandler: emitter,
        Logger:       log.NewNopLogger(),
        Registry:     registry,
    })
    require.NoError(t, err)
    require.NoError(t, l.Start(context.Background()))
    t.Cleanup(l.Stop)

    pid := "12345"
    receiver.Chan() <- loki.NewEntry(model.LabelSet{}, push.Entry{
        Timestamp: time.Now(),
        Line:      `2026-04-28 12:00:00.000 UTC:127.0.0.1:user@books_store:[` + pid + `]:1:42P01::1:0:0:c1::ERROR:  relation "missing" does not exist`,
    })
    receiver.Chan() <- loki.NewEntry(model.LabelSet{}, push.Entry{
        Timestamp: time.Now(),
        Line:      `2026-04-28 12:00:00.000 UTC:127.0.0.1:user@books_store:[` + pid + `]:2:42P01::1:0:0:c1::STATEMENT:  SELECT * FROM missing WHERE id = $1`,
    })

    require.Eventually(t, func() bool { return len(emitter.Received()) >= 1 }, time.Second, 10*time.Millisecond)
    e := emitter.Received()[0]
    var fp string
    for _, m := range e.StructuredMetadata {
        if m.Name == "query_fingerprint" {
            fp = m.Value
        }
    }
    assert.NotEmpty(t, fp, "expected query_fingerprint structured metadata on the error entry")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestLogs_AttachesQueryFingerprintToError`
Expected: FAIL — current logs collector does not emit Loki entries for errors.

- [ ] **Step 3: Add a small in-memory buffer keyed by PID**

In `logs.go`, add to the `Logs` struct:

```go
pendingErrors map[string]*pendingError
pendingMu     sync.Mutex
```

```go
type pendingError struct {
    receivedAt time.Time
    severity   string
    sqlstate   string
    database   string
    user       string
    timestamp  time.Time
    // populated when the matching STATEMENT continuation arrives
    statement  string
}
```

Initialize the map in `NewLogs`. Drop pending entries older than 5 seconds in `run` (a ticker, not on the hot path of every line). The existing `parseTextLog` becomes the writer; add a new `processContinuation(line)` for `STATEMENT:` lines.

- [ ] **Step 4: Refactor `parseTextLog`**

When a matching ERROR/FATAL/PANIC line is detected, build the `pendingError` and store it under the line's PID instead of *only* incrementing `pg_errors_total`. Continuation lines starting with `STATEMENT:` look up the pending entry by PID, attach the SQL, and call a new `emitErrorEntry(*pendingError)` that:
- computes `fp, _, _ := fingerprint.Fingerprint(stmt, fingerprint.SourceLog, 0)`
- emits a Loki entry via `BuildLokiEntryWithStructuredMetadata` with `op="pg_error"`, line `severity="..." sqlstate="..." datname="..." user="..." statement_preview="..."`, and structured metadata `[{query_fingerprint, fp}]`
- still increments `pg_errors_total` (now also gains a `query_fingerprint` label — see step 5).

Important: the existing logic that early-returns on continuation lines (`if isContinuationLine(line) { return nil }`) needs to be replaced with a routing call so we don't drop `STATEMENT:` lines.

- [ ] **Step 5: Add `query_fingerprint` to the `pg_errors_total` counter**

In `initMetrics`, change the label list of `errorsBySQLState` from
`{"severity", "sqlstate", "sqlstate_class", "datname", "user"}`
to
`{"severity", "sqlstate", "sqlstate_class", "datname", "user", "query_fingerprint"}`.

In `emitErrorEntry`, increment with the fingerprint included. Pending entries that time out without a STATEMENT continuation (e.g. internal-server errors) are still counted, with `query_fingerprint=""`.

- [ ] **Step 6: Run tests**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestLogs -v`
Expected: New test PASSES; existing tests may need their expected label sets updated.

- [ ] **Step 7: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs.go internal/component/database_observability/postgres/collector/logs_test.go
git commit -m "feat(database_observability/postgres): emit query_fingerprint on error log entries"
```

---

### Task 11: Parse slow-query `LOG: duration: ... ms statement: ...` lines

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs.go`
- Modify: `internal/component/database_observability/postgres/collector/logs_test.go`

- [ ] **Step 1: Write the failing test**

Append to `logs_test.go`:

```go
func TestLogs_EmitsSlowQueryWithFingerprint(t *testing.T) {
    var (
        receiver = loki.NewLogsReceiver()
        emitter  = loki_fake.NewClient(func() {})
        registry = prometheus.NewRegistry()
    )
    l, err := NewLogs(LogsArguments{
        Receiver: receiver, EntryHandler: emitter,
        Logger: log.NewNopLogger(), Registry: registry,
    })
    require.NoError(t, err)
    require.NoError(t, l.Start(context.Background()))
    t.Cleanup(l.Stop)

    receiver.Chan() <- loki.NewEntry(model.LabelSet{}, push.Entry{
        Timestamp: time.Now(),
        Line:      `2026-04-28 12:00:00.000 UTC:127.0.0.1:user@books_store:[12345]:1:00000::1:0:0:c1::LOG:  duration: 1234.567 ms  statement: SELECT pg_sleep(1)`,
    })

    require.Eventually(t, func() bool {
        for _, e := range emitter.Received() {
            if e.Labels["op"] == "pg_slow_query" {
                return true
            }
        }
        return false
    }, time.Second, 10*time.Millisecond)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestLogs_EmitsSlowQuery`
Expected: FAIL.

- [ ] **Step 3: Add slow-query parsing**

In `logs.go`, add a regex anchored to the message-suffix portion of the log line:

```go
// LOG:  duration: <ms> ms  statement: <sql>
var slowQueryRegex = regexp.MustCompile(`LOG:\s+duration:\s+([\d.]+)\s+ms\s+statement:\s+(.+)$`)
```

In `parseTextLog`, after the prefix match but before the ERROR keyword check, run the slow-query regex against the message portion. On match:
- parse the duration (ms float),
- compute the fingerprint via `fingerprint.Fingerprint(stmt, fingerprint.SourceLog, 0)`,
- emit a Loki entry with `op="pg_slow_query"`, line `datname="..." user="..." duration_ms="..." statement_preview="..."`, structured metadata `[{query_fingerprint, fp}]`.

- [ ] **Step 4: Run tests**

Run: `cd /home/gaantunes/git/alloy && go test ./internal/component/database_observability/postgres/collector/ -run TestLogs -v`
Expected: PASS for all logs tests.

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs.go internal/component/database_observability/postgres/collector/logs_test.go
git commit -m "feat(database_observability/postgres): emit slow-query log entries with fingerprint"
```

---

## Phase 4 — Documentation and integration

### Task 12: Update component reference docs

**Files:**
- Modify: `docs/sources/reference/components/database_observability/database_observability.postgres.md`

- [ ] **Step 1: Document the new metric**

In the "Exported metrics" / "Metrics" section, add a row:

```markdown
| `database_observability_query_hash_info` | gauge | Maps each `pg_stat_statements.queryid` to a stable, version-independent `query_fingerprint` computed by Alloy from the query's parsed AST. Join against any `pg_stat_statements_*` series with `* on(queryid, datname) group_left(query_fingerprint)` to attach the fingerprint without bumping cardinality on the source series. |
```

List its labels: `queryid`, `query_fingerprint`, `server_id`, `datname`.

- [ ] **Step 2: Document the new structured metadata fields**

Add a "Loki structured metadata" subsection (or extend the existing Loki section) listing:
- `query_fingerprint` on `op="query_sample"`, `op="wait_event"`, `op="wait_event_v2"`, `op="query_association"`, `op="pg_error"`, and `op="pg_slow_query"` entries.
- A note that the same fingerprint value appears on the metric side (`query_hash_info`), enabling log↔metric correlation on managed services.

- [ ] **Step 3: Document the new `pg_error` and `pg_slow_query` ops**

If the doc lists Loki ops produced by the component, add the two new ones with sample lines.

- [ ] **Step 4: Run docs lints if any**

Run (if the repo's Makefile defines this): `make docs/lint`
Expected: PASS, or skip if the target doesn't exist.

- [ ] **Step 5: Commit**

```bash
git add docs/sources/reference/components/database_observability/database_observability.postgres.md
git commit -m "docs(database_observability/postgres): document query_fingerprint metric and structured metadata"
```

---

### Task 13: Add a CHANGELOG entry

**Files:**
- Modify: `CHANGELOG.md` (root)

- [ ] **Step 1: Add an "Unreleased" enhancement entry**

```markdown
### Enhancements

- (_Public preview_) Add semantic query fingerprinting to `database_observability.postgres`: a new `database_observability_query_hash_info` join metric maps `pg_stat_statements.queryid` to a stable `query_fingerprint`, and the same fingerprint is attached as Loki structured metadata to `query_sample`, `wait_event`, `pg_error`, and `pg_slow_query` entries. This enables log↔metric correlation on managed PostgreSQL (RDS, Aurora) where `queryid` is not available in server logs. (@gaantunes)
```

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): add semantic query fingerprint entry"
```

---

### Task 14: End-to-end smoke test against a local PostgreSQL

**Files:** none (manual verification step)

- [ ] **Step 1: Bring up a postgres test container with `pg_stat_statements`**

```bash
docker run --rm -d --name alloy-fp-test \
  -e POSTGRES_PASSWORD=test \
  -p 5432:5432 \
  postgres:18 \
  -c shared_preload_libraries=pg_stat_statements \
  -c log_min_duration_statement=0 \
  -c log_line_prefix='%m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a '
```

Run: `psql "postgres://postgres:test@localhost/postgres" -c "CREATE EXTENSION pg_stat_statements;"`
Expected: `CREATE EXTENSION`.

- [ ] **Step 2: Generate some traffic with comments**

```bash
psql "postgres://postgres:test@localhost/postgres" <<'SQL'
SELECT 1 -- run 1;
SELECT 1 /* run 2 */;
SELECT 1;
SQL
```

- [ ] **Step 3: Run alloy with a minimal config**

```alloy
database_observability.postgres "test" {
  data_source_name = "postgres://postgres:test@localhost:5432/postgres?sslmode=disable"
  forward_to       = [loki.write.local.receiver]
}
loki.write "local" { endpoint { url = "http://localhost:3100/loki/api/v1/push" } }
```

Run: `./build/alloy run config.alloy`
Expected: starts up, no panic on the CGo path.

- [ ] **Step 4: Curl the metrics endpoint**

```bash
curl -s http://127.0.0.1:12345/api/v0/component/database_observability.postgres.test/metrics | grep query_hash_info
```

Expected: One `database_observability_query_hash_info{...} 1` line per logical query — note the three `SELECT 1` variants must collapse to a single fingerprint value.

- [ ] **Step 5: Tear down**

```bash
docker rm -f alloy-fp-test
```

No commit for this task — it's verification only.

---

## Self-Review Checklist

**Spec coverage:**
- ✅ Problem 1 (non-uniqueness): Tasks 2, 5 — pg_query AST collapses comment/whitespace/literal differences.
- ✅ Problem 2 (version instability): Task 4 — `query_fingerprint` is computed by Alloy and is independent of the PG-version-specific `queryid`.
- ✅ Problem 3 (log↔metric correlation): Tasks 10, 11 — fingerprint emitted as structured metadata on log entries that carry SQL text; same value on the `query_hash_info` join metric.
- ✅ Truncation/unparsable handling: Task 2 implements both sentinels; Task 7 plumbs `track_activity_query_size` so the truncation sentinel can fire.
- ✅ Recommendation (Option 1 + Option B, additive): Tasks 4, 6 keep `queryid` intact and add a new join metric; existing dashboards keep working.

**Placeholder scan:** No "TBD", "implement later", or "appropriate error handling" placeholders. All steps include either exact code or exact commands.

**Type consistency:**
- `QueryHashRegistry`, `QueryHashInfo`, `QueryHashMetricsCollector`, `QueryHashRegistry.Set/Get/Snapshot` are referenced consistently in tasks 3, 4, 5, 6, 8.
- `fingerprint.Fingerprint(text, source, trackSize) (string, bool, error)` and `fingerprint.SourcePgStatStatements/PgStatActivity/Log` are used consistently in tasks 2, 5, 8, 10, 11.
- `BuildLokiEntryWithStructuredMetadata(level, op, line, metadata)` is defined in task 9 and used in tasks 10, 11.
- The metric name `database_observability_query_hash_info` and label names `queryid, query_fingerprint, server_id, datname` are stable across tasks 4, 6, 12.

**Open follow-ups (out of scope of this plan):**
- Migrating `grafana-dbo11y-app` PromQL/LogQL to consume `query_fingerprint`.
- Eventual deprecation of native `queryid` (Option A) once the app migration completes.
- Adding the same fingerprint pipeline to `database_observability.mysql` (would need a different parser; design doc only covers PostgreSQL).
