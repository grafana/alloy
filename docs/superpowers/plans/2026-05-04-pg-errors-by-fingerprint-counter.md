# PostgreSQL Errors-by-Fingerprint Counter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a `database_observability_pg_errors_by_fingerprint_total{severity, sqlstate, sqlstate_class, datname, query_fingerprint}` Prometheus counter populated by parsing PostgreSQL ERROR + STATEMENT log line pairs, so users can compute error rate per logical query (`sum by (query_fingerprint, datname) (rate(...[5m]))`).

**Architecture:**
1. Add `pg_query_go/v6` as a dependency.
2. Add a freestanding `fingerprint` package (3-stage pipeline: parse-as-is → repair → sentinel) — same package the larger semantic-fingerprint design proposes for use in `query_details`/`query_samples`/Loki later, but introduced here as the minimum surface needed.
3. Extend the existing logs collector to buffer ERROR/FATAL/PANIC log rows by PID, stitch the matching `STATEMENT:` continuation, fingerprint the SQL text, and increment a new counter. Keep the existing `pg_errors_total` counter untouched.
4. **No Loki entries are emitted in this plan.** That deliberately keeps scope tiny and avoids the `EntryHandler`/fanout wiring concerns that come up later.

**Tech Stack:** Go (CGo via `pg_query_go/v6` for libpg_query), `prometheus/client_golang`, the existing `loki.LogsReceiver` plumbing already used by the logs collector.

**Recommendation followed:** This is the first deliverable from the broader semantic-query-fingerprint design (`docs/design/XXXX-semantic-query-fingerprint.md`). It implements only the error-rate-per-fingerprint surface so dashboards can answer "errors per logical query" without waiting on the full join-metric / Loki integration.

---

## File Structure

**New files:**
- `internal/component/database_observability/postgres/fingerprint/fingerprint.go` — three-stage `Fingerprint` function, sentinel constants, `Source` enum.
- `internal/component/database_observability/postgres/fingerprint/fingerprint_test.go` — table-driven unit tests.

**Modified files:**
- `go.mod`, `go.sum`, `collector/go.mod`, `collector/go.sum`, `extension/alloyengine/go.mod`, `extension/alloyengine/go.sum` — add `github.com/pganalyze/pg_query_go/v6`.
- `internal/component/database_observability/postgres/collector/logs.go` — add `pendingError` buffering keyed by PID, a timeout ticker that flushes stale pendings, a `processContinuation` helper that fingerprints `STATEMENT:` lines and increments the new counter, and the new counter itself.
- `internal/component/database_observability/postgres/collector/logs_test.go` — happy-path, timeout, and displaced-pending coverage.
- `docs/sources/reference/components/database_observability/database_observability.postgres.md` — document the new metric.

**File responsibilities:**
- `fingerprint/` is *only* about turning text into a stable identifier; it must not import anything from `database_observability` so it can be reused for other surfaces (samples, query_details, slow-query logs) without dragging in component dependencies.
- `logs.go` already houses `parseTextLog` and `isContinuationLine`; the new buffering and fingerprint-emission logic goes alongside them. The file grows by ~100 lines but stays single-purpose.

---

## Phase 0 — Dependency and fingerprint package

### Task 1: Add `pg_query_go/v6` dependency

**Files:**
- Modify: `go.mod`, `collector/go.mod`, `extension/alloyengine/go.mod`
- Modify: `go.sum`, `collector/go.sum`, `extension/alloyengine/go.sum`

- [ ] **Step 1: Add the require entry to the top-level `go.mod`**

In `go.mod`, locate the `require ( ... )` block that already contains `github.com/percona/mongodb_exporter` (around line 184), and add a new entry in alphabetical order between `mongodb_exporter` and `phayes/freeport`:

```go
github.com/pganalyze/pg_query_go/v6 v6.1.0
```

- [ ] **Step 2: Run `go mod tidy` from the repo root**

Run: `go mod tidy`
Expected: `go.sum` is updated with `github.com/pganalyze/pg_query_go/v6 v6.1.0` lines and one transitive `google.golang.org/protobuf v1.31.0/go.mod` hash.

- [ ] **Step 3: Tidy the sub-modules**

Run:
```bash
(cd collector && go mod tidy)
(cd extension/alloyengine && go mod tidy)
```
Expected: nothing changes in those go.mod/go.sum files. Nothing in those sub-modules imports `pg_query_go` yet, so tidy correctly leaves them alone — that's fine. **Do not** manually add a `require` if tidy doesn't.

- [ ] **Step 4: Verify the binary still builds (CGo path)**

Run: `CGO_ENABLED=1 go build ./internal/component/...`
Expected: Build succeeds. If a CGo toolchain is missing, install `build-essential` (Linux) and re-run. (`./...` would also work but the root has a pre-existing alloy-service Linux build issue that's unrelated to this change.)

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

Run: `go run ./cmd/_smoke/fingerprint_smoke`
Expected: a 16-character hex fingerprint is printed.

- [ ] **Step 6: Delete the smoke binary and commit**

```bash
rm -rf cmd/_smoke
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
	a, _, errA := Fingerprint("SELECT * FROM users WHERE id = $1 -- foo", SourceLog, 0)
	require.NoError(t, errA)
	b, _, errB := Fingerprint("SELECT *\nFROM users\nWHERE id = $1 /* bar */", SourceLog, 0)
	require.NoError(t, errB)
	assert.Equal(t, a, b)
}

func TestFingerprint_DifferentForDifferentQueries(t *testing.T) {
	a, _, _ := Fingerprint("SELECT * FROM users", SourceLog, 0)
	b, _, _ := Fingerprint("SELECT * FROM products", SourceLog, 0)
	assert.NotEqual(t, a, b)
}

func TestFingerprint_RepairUnclosedQuotes(t *testing.T) {
	want, _, errWant := Fingerprint("SELECT * FROM t WHERE name = 'oh no'", SourceLog, 0)
	require.NoError(t, errWant)

	fp, repaired, err := Fingerprint("SELECT * FROM t WHERE name = 'oh no", SourceLog, 0)
	require.NoError(t, err)
	assert.True(t, repaired, "should report that repair was used")
	assert.Equal(t, want, fp, "repaired fingerprint must match the closed-quote form")
}

func TestFingerprint_RepairUnclosedParens(t *testing.T) {
	want, _, errWant := Fingerprint("SELECT * FROM t WHERE id IN (1, 2, 3)", SourceLog, 0)
	require.NoError(t, errWant)

	fp, repaired, err := Fingerprint("SELECT * FROM t WHERE id IN (1, 2, 3", SourceLog, 0)
	require.NoError(t, err)
	assert.True(t, repaired)
	assert.Equal(t, want, fp, "repaired fingerprint must match the closed-paren form")
}

func TestFingerprint_TruncatedSentinelOnPgStatActivity(t *testing.T) {
	const trackSize = 1024
	bad := makeUnparsableOfLen(trackSize - 1)

	fp, _, err := Fingerprint(bad, SourcePgStatActivity, trackSize)
	require.NoError(t, err)
	assert.Equal(t, FingerprintOf(SentinelTruncated), fp)
}

func TestFingerprint_UnparsableSentinel(t *testing.T) {
	fp, _, err := Fingerprint("$$$ not sql at all $$$", SourceLog, 0)
	require.NoError(t, err)
	assert.Equal(t, FingerprintOf(SentinelUnparsable), fp)
}

func TestFingerprint_EmptyAndNullInputs(t *testing.T) {
	_, _, err := Fingerprint("", SourceLog, 0)
	assert.Error(t, err, "empty input should error so callers can skip emitting")
}

func TestFingerprint_SentinelStability(t *testing.T) {
	t.Run("truncated sentinel is stable", func(t *testing.T) {
		const trackSize = 1024
		bad := makeUnparsableOfLen(trackSize - 1)

		first, _, err1 := Fingerprint(bad, SourcePgStatActivity, trackSize)
		require.NoError(t, err1)
		second, _, err2 := Fingerprint(bad, SourcePgStatActivity, trackSize)
		require.NoError(t, err2)
		assert.Equal(t, first, second)
		assert.Equal(t, FingerprintOf(SentinelTruncated), first)
	})

	t.Run("unparsable sentinel is stable", func(t *testing.T) {
		first, _, _ := Fingerprint("$$$ not sql at all $$$", SourceLog, 0)
		second, _, _ := Fingerprint("$$$ not sql at all $$$", SourceLog, 0)
		assert.Equal(t, first, second)
		assert.Equal(t, FingerprintOf(SentinelUnparsable), first)
	})
}

// makeUnparsableOfLen returns a string of exactly n bytes that is unparsable
// even after `repair()` runs (no quotes or parens to balance).
func makeUnparsableOfLen(n int) string {
	const seed = "NOT VALID SQL !!! "
	out := seed
	for len(out) < n {
		out += "x "
	}
	return out[:n]
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/component/database_observability/postgres/fingerprint/...`
Expected: FAIL — package has no implementation yet.

- [ ] **Step 3: Implement the fingerprint package**

Create `internal/component/database_observability/postgres/fingerprint/fingerprint.go`:

```go
// Package fingerprint computes stable, semantic SQL fingerprints for PostgreSQL
// query text using the libpg_query parser (via pg_query_go).
//
// The same fingerprint is produced regardless of comments, whitespace, or
// literal-vs-placeholder differences, allowing pg_stat_statements metrics,
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

var (
	sentinelTruncatedFp  = FingerprintOf(SentinelTruncated)
	sentinelUnparsableFp = FingerprintOf(SentinelUnparsable)
)

// Fingerprint runs the three-stage pipeline:
//  1. Parse the input as-is.
//  2. If parsing fails, balance unclosed quotes and parentheses and retry.
//  3. If parsing still fails, return a sentinel fingerprint.
//
// trackActivityQuerySize is the value of the postgres setting
// `track_activity_query_size` and is only consulted when source ==
// SourcePgStatActivity. Pass 0 for other sources.
//
// The returned `repaired` flag is true when the input did NOT parse as-is
// (i.e. either stage 2 succeeded, or both stage 2 and stage 3 ran). To
// distinguish a successful repair from a sentinel fallback, compare the
// returned fingerprint against FingerprintOf(SentinelTruncated) /
// FingerprintOf(SentinelUnparsable).
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
	if fp, err := pg_query.Fingerprint(text); err == nil && fp != "" {
		return fp
	}
	// pg_query.Fingerprint may not parse a non-SQL sentinel; fall back to a
	// deterministic hash of the text (matches pganalyze's util/fingerprint.go).
	return formatHash(pg_query.HashXXH3_64([]byte(text), 0xee))
}

func sentinelFingerprint(query string, source Source, trackActivityQuerySize int) string {
	if source == SourcePgStatActivity && trackActivityQuerySize > 0 && len(query) == trackActivityQuerySize-1 {
		return sentinelTruncatedFp
	}
	return sentinelUnparsableFp
}

// repair closes unclosed single/double quotes and balances unclosed
// parentheses, mirroring pganalyze's `fixTruncatedQuery`. The repaired text
// is only used for fingerprint computation — it is not emitted anywhere.
//
// This is a heuristic and has known false positives:
//   - Doubled-apostrophe escapes inside string literals (`'O''Brien'`) are
//     counted as four separate `'` characters.
//   - Dollar-quoted strings (`$body$ ... $body$`) are not understood at all.
//   - Backslash-escaped quotes (with `standard_conforming_strings = off`) are
//     similarly miscounted.
//
// Quote-balancing must run before paren-balancing — a string ending in
// `'(` should have the quote closed first.
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

Run: `go test -v ./internal/component/database_observability/postgres/fingerprint/...`
Expected: PASS for all 8 tests (the 7 directly named plus the `SentinelStability` umbrella with 2 subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/fingerprint/
git commit -m "feat(database_observability/postgres): add semantic query fingerprint package"
```

---

## Phase 1 — Logs collector: errors-by-fingerprint counter

### Task 3: Register the new counter

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs.go`

This task introduces the metric and its lifecycle without any incrementing logic yet. Splitting it off makes the diff easy to review.

- [ ] **Step 1: Read the existing struct and `initMetrics`**

Open `internal/component/database_observability/postgres/collector/logs.go` and locate:
- `Logs` struct around line 50 (has `errorsBySQLState *prometheus.CounterVec` field).
- `initMetrics` around line 96 (constructs and registers existing metrics).
- `Stop` around line 137 (unregisters existing metrics).

These are your anchors.

- [ ] **Step 2: Add the field to `Logs`**

Add a field next to the existing `errorsBySQLState`:

```go
errorsByFingerprint *prometheus.CounterVec
```

- [ ] **Step 3: Construct it in `initMetrics`**

After the existing `l.errorsBySQLState = prometheus.NewCounterVec(...)` block (and before `l.parseErrors = ...`), add:

```go
l.errorsByFingerprint = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "database_observability",
		Name:      "pg_errors_by_fingerprint_total",
		Help:      "Number of PostgreSQL log errors with a captured query fingerprint, partitioned by severity, SQL state, and the originating query's fingerprint. Counts a subset of pg_errors_total (only errors for which Alloy successfully observed the matching STATEMENT continuation).",
	},
	[]string{"severity", "sqlstate", "sqlstate_class", "datname", "query_fingerprint"},
)
```

In the existing `l.registry.MustRegister(...)` call below, add `l.errorsByFingerprint` to the argument list. The block becomes:

```go
l.registry.MustRegister(
	l.errorsBySQLState,
	l.errorsByFingerprint,
	l.parseErrors,
)
```

- [ ] **Step 4: Unregister it in `Stop`**

In `Stop`, alongside the existing `l.registry.Unregister(l.errorsBySQLState)`, add:

```go
l.registry.Unregister(l.errorsByFingerprint)
```

- [ ] **Step 5: Run all logs tests to confirm no regression**

Run: `go test -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector`
Expected: All existing tests still pass (the metric exists but never increments yet, so no test cares).

- [ ] **Step 6: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs.go
git commit -m "feat(database_observability/postgres): register pg_errors_by_fingerprint_total counter"
```

---

### Task 4: Buffer pending errors, fingerprint STATEMENT continuations, increment the counter

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs.go`
- Modify: `internal/component/database_observability/postgres/collector/logs_test.go`

This is the single biggest task — but it's still bite-sized in steps. It introduces the buffering map, the timeout ticker, and the STATEMENT-arrives → fingerprint → increment path. Errors with no STATEMENT are not counted by the new metric (they remain counted by the existing `pg_errors_total`).

- [ ] **Step 1: Write the failing test (happy path)**

Append to `internal/component/database_observability/postgres/collector/logs_test.go`:

```go
func TestLogsCollector_IncrementsErrorsByFingerprint_OnErrorPlusStatement(t *testing.T) {
	registry := prometheus.NewRegistry()
	receiver := loki.NewLogsReceiver()

	c, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: loki.NewEntryHandler(make(chan loki.Entry, 8), func() {}),
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(c.Stop)

	ts := c.startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	// ERROR row.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{
		Timestamp: time.Now(),
		Line:      ts1 + ":127.0.0.1:5432:user@books_store:[12345]:1:42P01:" + ts2 + ":1/0:0:c1::psqlERROR:  relation \"missing\" does not exist",
	}}
	// STATEMENT continuation.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{
		Timestamp: time.Now(),
		Line:      "STATEMENT:  SELECT * FROM missing WHERE id = $1",
	}}

	expectedFP, _, fpErr := fingerprint.Fingerprint("SELECT * FROM missing WHERE id = $1", fingerprint.SourceLog, 0)
	require.NoError(t, fpErr)

	require.Eventually(t, func() bool {
		return testutil.CollectAndCount(c.errorsByFingerprint, "database_observability_pg_errors_by_fingerprint_total") >= 1
	}, 2*time.Second, 50*time.Millisecond)

	got := testutil.ToFloat64(c.errorsByFingerprint.WithLabelValues("ERROR", "42P01", "42", "books_store", expectedFP))
	require.Equal(t, float64(1), got)
}
```

Add necessary imports to the test file (most are already present — verify by reading the existing import block):
- `"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"`
- `"github.com/prometheus/client_golang/prometheus/testutil"`

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector_IncrementsErrorsByFingerprint_OnErrorPlusStatement`
Expected: FAIL — `c.errorsByFingerprint.WithLabelValues(...)` returns 0; nothing increments yet. (The test compiles because the field was added in Task 3.)

- [ ] **Step 3: Add the `pendingError` struct and the buffering fields on `Logs`**

In `logs.go`, near the existing `Logs` struct (around line 50), add:

```go
type pendingError struct {
	receivedAt    time.Time
	severity      string
	sqlstate      string
	sqlstateClass string
	datname       string
}
```

Inside the `Logs` struct, add three new fields (next to the existing `errorsByFingerprint`):

```go
pendingErrors       map[string]*pendingError
pendingMu           sync.Mutex
pendingErrorTimeout time.Duration
```

In `NewLogs`, after the `Logs{...}` literal, initialize them:

```go
l.pendingErrors = make(map[string]*pendingError)
l.pendingErrorTimeout = 5 * time.Second
```

(Defaulting to 5s mirrors the buffer window pganalyze uses for the same shape of stitching. Tests can override this field directly.)

- [ ] **Step 4: Add a timeout ticker to `run`**

Locate `run` (around line 150). Replace its body with:

```go
func (l *Logs) run() {
	level.Debug(l.logger).Log("msg", "collector running, waiting for log entries")

	tickPeriod := l.pendingErrorTimeout / 2
	if tickPeriod < 50*time.Millisecond {
		tickPeriod = 50 * time.Millisecond
	}
	timeoutTicker := time.NewTicker(tickPeriod)
	defer timeoutTicker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			level.Debug(l.logger).Log("msg", "collector stopping")
			return
		case entry := <-l.receiver.Chan():
			if err := l.parseTextLog(entry); err != nil {
				level.Warn(l.logger).Log(
					"msg", "failed to process log line",
					"error", err,
					"line_preview", truncateString(entry.Entry.Line, 100),
				)
			}
		case <-timeoutTicker.C:
			l.flushExpiredPending()
		}
	}
}
```

- [ ] **Step 5: Refactor the continuation-routing in `parseTextLog`**

Locate the existing guard (around line 173):

```go
if isContinuationLine(line) {
	return nil
}
```

Replace it with:

```go
if isContinuationLine(line) {
	l.processContinuation(line)
	return nil
}
```

- [ ] **Step 6: Capture pending errors in `parseTextLog`**

In `parseTextLog`, after the existing `l.errorsBySQLState.WithLabelValues(...).Inc()` call (around line 261-267), add the buffering write:

```go
// Buffer the error so a matching STATEMENT continuation can stitch the
// SQL text and we can record per-fingerprint cardinality.
pidStart := strings.Index(afterAt, "[")
pidEnd := strings.Index(afterAt, "]")
pid := ""
if pidStart != -1 && pidEnd > pidStart {
	pid = afterAt[pidStart+1 : pidEnd]
}

if pid != "" {
	l.pendingMu.Lock()
	l.pendingErrors[pid] = &pendingError{
		receivedAt:    time.Now(),
		severity:      severity,
		sqlstate:      sqlstateCode,
		sqlstateClass: sqlstateClass,
		datname:       database,
	}
	l.pendingMu.Unlock()
}
```

- [ ] **Step 7: Implement `processContinuation`, `flushExpiredPending`, and the increment path**

Add three new methods, e.g. just after the existing `isContinuationLine` function:

```go
// processContinuation handles continuation lines (DETAIL/HINT/STATEMENT/...).
// We only act on STATEMENT lines today: the SQL text is fingerprinted and
// the matching pending error increments database_observability_pg_errors_by_fingerprint_total.
//
// PostgreSQL does not include the log_line_prefix on continuation lines, so
// we cannot extract the originating PID from the line itself. We match the
// most recently buffered pending error instead — PG's ereport mutex emits
// ERROR + STATEMENT contiguously per backend, so per-PID interleaving in
// the upstream tailer is the only failure mode (rare in practice).
func (l *Logs) processContinuation(line string) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "STATEMENT:") {
		return
	}
	stmt := strings.TrimSpace(strings.TrimPrefix(trimmed, "STATEMENT:"))

	l.pendingMu.Lock()
	var bestPID string
	var bestEntry *pendingError
	for pid, p := range l.pendingErrors {
		if bestEntry == nil || p.receivedAt.After(bestEntry.receivedAt) {
			bestPID = pid
			bestEntry = p
		}
	}
	if bestEntry != nil {
		delete(l.pendingErrors, bestPID)
	}
	l.pendingMu.Unlock()

	if bestEntry == nil {
		return
	}

	fp, _, err := fingerprint.Fingerprint(stmt, fingerprint.SourceLog, 0)
	if err != nil {
		// fingerprint.ErrEmpty — caller should skip; do not increment.
		return
	}

	l.errorsByFingerprint.WithLabelValues(
		bestEntry.severity,
		bestEntry.sqlstate,
		bestEntry.sqlstateClass,
		bestEntry.datname,
		fp,
	).Inc()
}

// flushExpiredPending drops pending entries older than pendingErrorTimeout.
// Errors without a matching STATEMENT continuation never increment the
// pg_errors_by_fingerprint_total counter — they remain counted only on
// pg_errors_total.
func (l *Logs) flushExpiredPending() {
	deadline := time.Now().Add(-l.pendingErrorTimeout)
	l.pendingMu.Lock()
	for pid, p := range l.pendingErrors {
		if p.receivedAt.Before(deadline) {
			delete(l.pendingErrors, pid)
		}
	}
	l.pendingMu.Unlock()
}
```

Add these imports to the top of `logs.go` (group with the existing `grafana/alloy` imports):

```go
"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
```

- [ ] **Step 8: Run the new test to verify it passes**

Run: `go test -v -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector_IncrementsErrorsByFingerprint_OnErrorPlusStatement`
Expected: PASS.

- [ ] **Step 9: Run all logs tests**

Run: `go test -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector`
Expected: All pass. The pre-existing tests don't pass a STATEMENT continuation, so the new metric stays at zero for them; no expectations need updating.

- [ ] **Step 10: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs.go internal/component/database_observability/postgres/collector/logs_test.go
git commit -m "feat(database_observability/postgres): increment pg_errors_by_fingerprint_total on ERROR+STATEMENT pairs"
```

---

### Task 5: Confirm timeout-without-STATEMENT does not increment

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs_test.go`

This is a small test-only task that pins the documented behavior: errors without a STATEMENT continuation are NOT counted on the new metric.

- [ ] **Step 1: Write the test**

Append to `logs_test.go`:

```go
func TestLogsCollector_TimedOutPendingDoesNotIncrementFingerprintCounter(t *testing.T) {
	registry := prometheus.NewRegistry()
	receiver := loki.NewLogsReceiver()

	c, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: loki.NewEntryHandler(make(chan loki.Entry, 8), func() {}),
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)
	c.pendingErrorTimeout = 100 * time.Millisecond

	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(c.Stop)

	ts := c.startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	// ERROR with no following STATEMENT.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{
		Timestamp: time.Now(),
		Line:      ts1 + ":127.0.0.1:5432:user@books_store:[99999]:1:53300:" + ts2 + ":1/0:0:c1::psqlFATAL:  too many connections",
	}}

	// pg_errors_total increments immediately (existing behavior).
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(c.errorsBySQLState.WithLabelValues("FATAL", "53300", "53", "books_store", "user")) == 1
	}, 1*time.Second, 50*time.Millisecond)

	// Wait past the pending timeout window plus a tick.
	time.Sleep(300 * time.Millisecond)

	// pg_errors_by_fingerprint_total stays at 0 — there was no STATEMENT.
	require.Equal(t, 0, testutil.CollectAndCount(c.errorsByFingerprint, "database_observability_pg_errors_by_fingerprint_total"))
}
```

- [ ] **Step 2: Run the test**

Run: `go test -v -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector_TimedOutPendingDoesNotIncrementFingerprintCounter`
Expected: PASS (no implementation change required; this test pins existing behavior).

- [ ] **Step 3: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs_test.go
git commit -m "test(database_observability/postgres): pin no-statement→no-fingerprint-counter behavior"
```

---

### Task 6: Handle displaced pending entries cleanly

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs.go`
- Modify: `internal/component/database_observability/postgres/collector/logs_test.go`

When the same backend (PID) issues a second ERROR before the first's STATEMENT arrives, the buffer's `pendingErrors[pid] = ...` overwrites the first. The first error's `pg_errors_total` was already incremented, but the new fingerprint counter would silently skip it — making the new counter under-count compared to its parent counter. Fix: drop the displaced entry like a timeout (no fingerprint counter increment), but make the dropping explicit and tested.

- [ ] **Step 1: Write the failing test**

Append to `logs_test.go`:

```go
func TestLogsCollector_DisplacedPendingDoesNotDoubleCount(t *testing.T) {
	registry := prometheus.NewRegistry()
	receiver := loki.NewLogsReceiver()

	c, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: loki.NewEntryHandler(make(chan loki.Entry, 8), func() {}),
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(c.Stop)

	ts := c.startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	pid := "55555"
	// ERROR #1 (no STATEMENT yet).
	receiver.Chan() <- loki.Entry{Entry: push.Entry{
		Timestamp: time.Now(),
		Line:      ts1 + ":127.0.0.1:5432:user@books_store:[" + pid + "]:1:42P01:" + ts2 + ":1/0:0:c1::psqlERROR:  err one",
	}}
	// ERROR #2 from same PID (displaces #1).
	receiver.Chan() <- loki.Entry{Entry: push.Entry{
		Timestamp: time.Now(),
		Line:      ts1 + ":127.0.0.1:5432:user@books_store:[" + pid + "]:1:42P02:" + ts2 + ":1/0:0:c1::psqlERROR:  err two",
	}}
	// STATEMENT for #2.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{
		Timestamp: time.Now(),
		Line:      "STATEMENT:  SELECT 2",
	}}

	expectedFP, _, _ := fingerprint.Fingerprint("SELECT 2", fingerprint.SourceLog, 0)

	// Exactly one increment, with err two's labels and SELECT 2's fingerprint.
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(c.errorsByFingerprint.WithLabelValues("ERROR", "42P02", "42", "books_store", expectedFP)) == 1
	}, 2*time.Second, 50*time.Millisecond)
	// And nothing under err one's labels.
	require.Equal(t, float64(0), testutil.ToFloat64(c.errorsByFingerprint.WithLabelValues("ERROR", "42P01", "42", "books_store", expectedFP)))
}
```

- [ ] **Step 2: Run the test**

Run: `go test -v -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector_DisplacedPendingDoesNotDoubleCount`
Expected: PASS already. The current implementation overwrites silently, which is the desired behavior for *this* metric — the displaced entry simply doesn't contribute. The test pins that behavior.

(If you preferred to *attribute* the displaced entry to the second error's STATEMENT, you'd see a `2` instead of `1` and the test would fail. The current single-increment behavior is correct.)

- [ ] **Step 3: Add a documentation comment to `parseTextLog`'s buffering write**

Find the `if pid != "" {` block added in Task 4, Step 6. Add a comment explaining the displacement semantics:

```go
if pid != "" {
	l.pendingMu.Lock()
	// If a previous error from this PID is still pending (no STATEMENT
	// arrived within the timeout window), the new error displaces it.
	// The displaced error is NOT credited to the fingerprint counter —
	// only pg_errors_total counts it. This keeps the new metric strictly
	// equal to "errors with successfully captured SQL".
	l.pendingErrors[pid] = &pendingError{...}
	l.pendingMu.Unlock()
}
```

(Reuse the existing `pendingError{...}` literal from Task 4; only the comment is new.)

- [ ] **Step 4: Run all logs tests**

Run: `go test -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs.go internal/component/database_observability/postgres/collector/logs_test.go
git commit -m "test(database_observability/postgres): pin displaced-pending semantics for fingerprint counter"
```

---

## Phase 2 — Documentation

### Task 7: Document the new metric

**Files:**
- Modify: `docs/sources/reference/components/database_observability/database_observability.postgres.md`

- [ ] **Step 1: Locate the existing exported-metrics section**

Open `docs/sources/reference/components/database_observability/database_observability.postgres.md` and find the section that documents `database_observability_pg_errors_total` (search for `pg_errors_total` — if there's no dedicated metrics section, add one between `## Exports` and `## logs collector` following the pattern of other Grafana Alloy component docs).

- [ ] **Step 2: Add a row for the new metric**

Add this entry (or a fresh table row, depending on how the section is currently structured):

```markdown
### `database_observability_pg_errors_by_fingerprint_total`

| Property    | Value                                                                                                                                                                                                                                                                                          |
|-------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Type**    | counter                                                                                                                                                                                                                                                                                        |
| **Labels**  | `severity`, `sqlstate`, `sqlstate_class`, `datname`, `query_fingerprint`                                                                                                                                                                                                                       |
| **Subset of** | `database_observability_pg_errors_total` — increments only when Alloy successfully observes the matching `STATEMENT:` continuation line and computes a fingerprint from the SQL text.                                                                                                       |
| **Use**     | Compute error rate per logical query: `sum by (datname, query_fingerprint) (rate(database_observability_pg_errors_by_fingerprint_total[5m]))`                                                                                                                                                  |

The `query_fingerprint` value is computed client-side by Alloy from the parsed AST (libpg_query). Two queries that differ only in comments, whitespace, or literal values produce the same fingerprint — so an error rate keyed by fingerprint groups every variant of one logical query together. Errors without a captured `STATEMENT:` continuation (e.g. connection failures, internal server errors) contribute only to `pg_errors_total`, not to this metric.
```

(Match the existing doc's table style. If the existing metric is rendered as a single bulleted list rather than a table, mirror that.)

- [ ] **Step 3: Verify rendering**

If the repo has a docs preview workflow (e.g. `make generate-docs`), run it and skim the output. Otherwise rely on a Markdown preview (VS Code, GitHub).

- [ ] **Step 4: Commit**

```bash
git add docs/sources/reference/components/database_observability/database_observability.postgres.md
git commit -m "docs(database_observability/postgres): document pg_errors_by_fingerprint_total"
```

---

## Self-Review Checklist

**Spec coverage:**
- ✅ User's ask ("error rate metric that contains the fingerprint") → Tasks 3 + 4 register and populate `pg_errors_by_fingerprint_total{severity, sqlstate, sqlstate_class, datname, query_fingerprint}`.
- ✅ Cardinality boundedness → Task 4 only increments on successful STATEMENT match; Task 5 pins the "no-statement → no-increment" behavior; Task 6 pins the displacement semantics.
- ✅ Existing `pg_errors_total` unchanged → no task modifies its label set or its increment site.
- ✅ Documentation → Task 7.

**Placeholder scan:** No "TBD", "implement later", or "appropriate error handling" placeholders. Every code step has the exact code; every command step has the exact command and expected output.

**Type consistency:**
- `Fingerprint(text, source, trackActivityQuerySize) (string, bool, error)` — defined in Task 2, used in Task 4 (via `fingerprint.Fingerprint(...)` in `processContinuation`).
- `pendingError` struct fields (`receivedAt, severity, sqlstate, sqlstateClass, datname`) — defined in Task 4 Step 3, read in Task 4 Step 7's `processContinuation` and `flushExpiredPending`.
- `errorsByFingerprint *prometheus.CounterVec` field — defined in Task 3 Step 2, written in Task 4 Step 7, read in Tasks 4–6 tests via `testutil.ToFloat64` / `testutil.CollectAndCount`.
- Metric label order on the CounterVec definition (`severity, sqlstate, sqlstate_class, datname, query_fingerprint`) matches the order passed to `WithLabelValues` in `processContinuation` and in every test assertion. **Get this right; positional label arguments are easy to swap by accident.**

**Open follow-ups (out of scope of this plan):**
- Adding `query_fingerprint` Loki structured metadata to a future `pg_error` op.
- The full join metric `database_observability_query_hash_info` populated by `query_details` — separate plan/PR.
- `pg_slow_query` log entries — separate plan/PR.
- Surfacing `repaired=true` and sentinel hits as their own observability counters — useful for detecting upstream truncation but not required for this deliverable.

---

## Implementation handoff

After saving this plan, two execution options:

**1. Subagent-Driven (recommended)** — Dispatch a fresh subagent per task with a two-stage review (spec compliance, then code quality) before moving on. Best for keeping context tight and catching issues early.

**2. Inline Execution** — Execute tasks sequentially in the same session using `superpowers:executing-plans`, with checkpoints between phases.

The TDD discipline is identical either way: every behavior task starts with a failing test, then minimal implementation, then green run, then commit.
