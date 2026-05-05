# PostgreSQL Error → Loki `op="error"` Op Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Emit one Loki entry per PostgreSQL ERROR+STATEMENT pair, labelled `op="error"`, carrying every high-cardinality field — `query_fingerprint`, `pid`, `xid`, `application_name`, `error_message`, the SQL itself, etc. — as logfmt key/value pairs in the line body. No Loki structured metadata is used. This replaces the previously planned high-cardinality `database_observability_pg_errors_by_fingerprint_total` counter — `query_fingerprint` is unbounded and unsafe as a Prometheus label.

**Cardinality model:**
- **Stable Loki labels** (low cardinality): `op="error"`, `severity`, `sqlstate_class`, `datname`.
- **No structured metadata.** Everything else goes into the log line body as logfmt key/value pairs. Downstream consumers parse with `| logfmt`.
- **Log line body** (logfmt): `query_fingerprint`, `pid`, `backend_start`, `application_name`, `sqlstate`, `xid` (omitted when `0`), `client_addr`, `client_port`, `session_id`, `user`, `error_message`, `statement`. The `statement` field carries the assembled SQL with newlines collapsed to single spaces so the body stays a single logfmt line.

**LogQL replacement for the dropped metric:**
```
sum by (datname, query_fingerprint)
  (count_over_time({op="error"} | logfmt [5m]))
```

**Architecture:**
1. **Keep** the freestanding `fingerprint` package (already shipped on this branch — three-stage parse/repair/sentinel pipeline using `pg_query_go/v6`).
2. **Keep** all the log-parsing state machine in the logs collector (already shipped on this branch): TAB-continuation accumulation, prefixed-STATEMENT detection, statement-flush-before-pending-expiration ordering. None of that changes.
3. **Drop** the `database_observability_pg_errors_by_fingerprint_total` counter — its registration, its increment paths, its tests, its docs entry.
4. **Add** a Loki entry emission path. When a STATEMENT flushes (via `flushStatementLocked` or `processBareContinuation`), build a `loki.Entry` with the stable labels + a logfmt body and forward via the existing `entryHandler` field on `Logs` (currently wired but unused).
5. **Keep** `database_observability_pg_errors_total` exactly as-is — low-cardinality parent counter, useful as a denominator and as a "did we see the error at all" indicator.

**Tech stack unchanged:** Go (CGo via `pg_query_go/v6` for libpg_query); `prometheus/client_golang`; the existing `loki.LogsReceiver` plumbing already used by the logs collector; `loki.EntryHandler` for fanout.

**Recommendation followed:** This is the deliverable the broader semantic-query-fingerprint design pointed at — an op-shaped Loki entry that downstream LogQL/dashboards can group by `query_fingerprint`, and that joins to `pg_stat_activity` samples on `(pid, query_fingerprint, time-window)` (or on `(pid, xid)` when a write transaction was in flight).

---

## Branch state going into this plan

This plan executes on top of branch `gaantunes/pg-errors-by-fingerprint-counter`, which already has the following commits:

| Commit | What landed |
|---|---|
| `feat(deps): add pg_query_go for postgres semantic query fingerprinting` | `go.mod`/`go.sum` for pg_query_go in root + collector module |
| `feat(database_observability/postgres): add semantic query fingerprint package` | `internal/component/database_observability/postgres/fingerprint/{fingerprint.go,fingerprint_test.go}` |
| `feat(database_observability/postgres): register pg_errors_by_fingerprint_total counter` | **TO BE REVERTED** — adds CounterVec field |
| `feat(database_observability/postgres): increment pg_errors_by_fingerprint_total on ERROR+STATEMENT pairs` | mixes parser state machine (KEEP) with counter increments (REPLACE) |
| `test(database_observability/postgres): pin no-statement→no-fingerprint-counter behavior` | **TO BE REPLACED** — counter assertion → entry assertion |
| `test(database_observability/postgres): pin displaced-pending semantics for fingerprint counter` | **TO BE REPLACED** |
| `docs(database_observability/postgres): document pg_errors_by_fingerprint_total` | **TO BE REPLACED** with op docs |
| `feat(database_observability/postgres): handle prefixed STATEMENT continuations` | KEEP — this is the production-format support |
| `fix(database_observability/postgres): flush STATEMENT before pending expiration` | KEEP — race fix |

> Worker note: do **not** rebase or rewrite history; instead make the changes as new commits and let reviewers see the evolution. The fingerprint package + the parser state machine + the timeout fix are all still load-bearing for this plan.

---

## File Structure

**Modified:**
- `internal/component/database_observability/postgres/collector/logs.go` — extend metadata captured into `pendingError`, replace counter increment with Loki entry emission, drop the CounterVec lifecycle.
- `internal/component/database_observability/postgres/collector/logs_test.go` — replace counter assertions with `EntryHandler.Chan()` assertions; add metadata-field assertions; keep all the structural tests (timeout, displacement, multi-line, statement-flush ordering) but updated to inspect emitted entries instead of counter values.
- `docs/sources/reference/components/database_observability/database_observability.postgres.md` — drop the metric block, add an "Emitted Loki entries" section describing the `op="error"` shape and showing the LogQL replacement.

**Unchanged (keep):**
- `internal/component/database_observability/postgres/fingerprint/*` — already shipped.
- The `pg_errors_total` counter and its tests.
- Existing TAB-continuation accumulation, prefixed-STATEMENT detection, and the `flushExpiredPending` ordering fix.

**File responsibilities:**
- `fingerprint/` keeps its single concern: text → stable identifier. No changes.
- `logs.go` grows by ~80 lines net: new metadata fields on `pendingError`, ~50 lines of `loki.Entry` build + send, minus the counter wiring it removes (~30 lines).
- The reference doc's metric block is replaced by an op block. Net length similar.

---

## Phase 0 — Pre-flight

### Task 1: Confirm shipped state and clean up the deprecated metric

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs.go`

This task removes the metric without disturbing the parser state machine that the next phase will reuse.

- [ ] **Step 1: Verify branch state**

Run:
```bash
git log --oneline main..HEAD | head -10
git diff --stat main -- internal/component/database_observability/postgres/collector/logs.go
```
Expected: see all the commits listed in *Branch state* above, and `logs.go` shows the buffering / state-machine / fingerprint-counter additions.

- [ ] **Step 2: Remove the `errorsByFingerprint` field**

In `logs.go`, in the `Logs` struct, delete the line:
```go
errorsByFingerprint *prometheus.CounterVec
```

- [ ] **Step 3: Remove the CounterVec construction in `initMetrics`**

Delete the block:
```go
l.errorsByFingerprint = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Namespace: "database_observability",
        Name:      "pg_errors_by_fingerprint_total",
        Help:      "...",
    },
    []string{"severity", "sqlstate", "sqlstate_class", "datname", "query_fingerprint"},
)
```
And remove `l.errorsByFingerprint,` from the `l.registry.MustRegister(...)` argument list. The block becomes:
```go
l.registry.MustRegister(
    l.errorsBySQLState,
    l.parseErrors,
)
```

- [ ] **Step 4: Remove the unregister call in `Stop`**

Delete the line:
```go
l.registry.Unregister(l.errorsByFingerprint)
```

- [ ] **Step 5: Remove `incrementByFingerprint` and its callers (preserving the parser state machine)**

`incrementByFingerprint` is only meaningful when the counter exists. Replace it with a stub that keeps the call sites compiling — they will be re-pointed at the Loki emission path in Task 3. For now, make it a no-op that drops the entry but logs at debug:
```go
func (l *Logs) incrementByFingerprint(entry *pendingError, stmt string) {
    // Replaced in a follow-up task with Loki op emission.
    // This stub keeps the call sites in flushStatementLocked and
    // processBareContinuation building while Task 3 wires in the entry path.
    _ = entry
    _ = stmt
}
```

> Worker note: do not delete the call sites — `flushStatementLocked` and `processBareContinuation` already drive into `incrementByFingerprint` at the right moment. Reusing those call sites is what keeps the multi-line accumulation, displacement, and timeout-flush fixes load-bearing.

- [ ] **Step 6: Run all postgres tests; expect counter-assertion failures**

Run:
```bash
go test -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector
```
Expected: tests that assert on `c.errorsByFingerprint` no longer compile. Capture the failing test names — Task 6 will replace them.

- [ ] **Step 7: Comment out (do not delete yet) the failing assertions**

In `logs_test.go`, mark each affected test with `t.Skip("Replaced by Loki entry emission in Task 6")` at the top. Tests to skip (exhaustive list — derived from the shipped state):
- `TestLogsCollector_IncrementsErrorsByFingerprint_OnErrorPlusStatement`
- `TestLogsCollector_TimedOutPendingDoesNotIncrementFingerprintCounter`
- `TestLogsCollector_DisplacedPendingDoesNotDoubleCount`
- `TestLogsCollector_PrefixedStatement_MultiLine`
- `TestLogsCollector_StatementSurvivesTimeoutFlush`

> Worker note: this `t.Skip` is short-lived. Task 6 deletes the skip and rewrites each test against the entry handler.

- [ ] **Step 8: Verify build and runs cleanly**

Run:
```bash
go test -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector
go vet ./internal/component/database_observability/postgres/...
CGO_ENABLED=1 go build ./internal/component/...
```
Expected: pass / clean.

- [ ] **Step 9: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs.go internal/component/database_observability/postgres/collector/logs_test.go
git commit -m "refactor(database_observability/postgres): drop pg_errors_by_fingerprint counter (preparing for Loki op)"
```

---

## Phase 1 — Capture richer metadata

### Task 2: Extend `pendingError` with all fields needed for the Loki entry

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs.go`

This task only widens the data captured at ERROR-line time. The downstream emission path is added in Task 3.

- [ ] **Step 1: Add fields to `pendingError`**

The new struct shape:
```go
type pendingError struct {
    receivedAt    time.Time

    // Existing label-shaped fields.
    severity      string
    sqlstate      string
    sqlstateClass string
    datname       string

    // Existing user field (already kept on errors_total label).
    user          string

    // New fields populated from the prefix and message tail. All are
    // strings; emit "" when the source field is absent or empty.
    timestamp        time.Time // from %m, used as the Loki entry timestamp
    clientAddr       string    // host portion of %r
    clientPort       string    // port portion of %r
    pid              string    // %p
    backendStart     string    // %s, kept as the original RFC-ish text
    xid              string    // %x; emit "" when "0"
    sessionID        string    // %c
    applicationName  string    // %a; "[unknown]" pass-through
    errorMessage     string    // text after "<severity>:  " on the ERROR line
}
```

- [ ] **Step 2: Parse `%r` host and port**

`%r` formats:
- TCP: `host(port)` e.g. `172.18.0.3(34492)`
- Unix domain: `[local]`

Add a helper in `logs.go`:
```go
// parseRemote extracts the host and port from a %r value. Returns
// ("[local]", "") for unix-domain connections and ("", "") if the value
// can't be parsed.
func parseRemote(s string) (host, port string) {
    if s == "[local]" || s == "" {
        return s, ""
    }
    open := strings.LastIndex(s, "(")
    close := strings.LastIndex(s, ")")
    if open == -1 || close == -1 || close <= open {
        return s, ""
    }
    return s[:open], s[open+1 : close]
}
```

- [ ] **Step 3: Parse the application name from the prefix tail**

The line up through `%c` is delimited by `:`. After `%c` the literal sequence is `:%q%a:` — `%q` produces nothing for session-bound logs and `%a` is the application name (often `[unknown]`). The keyword (`ERROR:` / `STATEMENT:` / etc.) immediately follows the trailing `:` of `%a`.

So in `parseTextLog`, after the existing `pidEndIdx` extraction:
```go
// %s:%v:%x:%c:%q%a:KEYWORD body
parts := strings.SplitN(afterPid, ":", 8)
// parts[0] = "" (leading colon after "[<pid>]")
// parts[1] = %l
// parts[2] = %e (sqlstate)
// parts[3] = first half of %s (date portion before its inner ':')
// parts[4] = rest of %s + %v + %x + %c + %a, joined by remaining colons
//
// %s contains its own ':' (timestamps), so SplitN on a small N is fragile.
// Use the keyword position as the right anchor instead.
```

Better approach: anchor from the *right*. Find the FIRST `:KEYWORD:` in the line (where `KEYWORD` ∈ {`ERROR`, `FATAL`, `PANIC`, `STATEMENT`, `DETAIL`, `HINT`, `CONTEXT`, `QUERY`, `LOCATION`}). The byte slice immediately to the left of that match, minus the leading `:`, is `%a`. The slice to the left of `%a:` is `…:%c`.

Add helper:
```go
// findKeywordPos returns the index in line where the keyword (preceded by
// `:` and followed by `:`) begins, or -1.
func findKeywordPos(line string) (kwStart int, keyword string) {
    keywords := []string{"ERROR", "FATAL", "PANIC", "STATEMENT", "DETAIL", "HINT", "CONTEXT", "QUERY", "LOCATION"}
    best := -1
    bestKw := ""
    for _, kw := range keywords {
        if idx := strings.Index(line, ":"+kw+":"); idx != -1 {
            if best == -1 || idx < best {
                best = idx
                bestKw = kw
            }
        }
    }
    return best, bestKw
}
```

Then in `parseTextLog` (after the existing prefix parsing):
```go
kwStart, _ := findKeywordPos(line)
if kwStart != -1 {
    // The %a field is what sits between the last `:` before kwStart and the
    // colon at kwStart. Walk backwards from kwStart to find that colon.
    appStart := strings.LastIndex(line[:kwStart], ":") + 1
    applicationName := line[appStart:kwStart]
    // …
}
```

- [ ] **Step 4: Parse `%s`, `%v`, `%x`, `%c` from the segment between `[%p]:` and the keyword**

The segment after `]:` and before `:%a:KEYWORD:` looks like:
```
4:40001:2026-05-05 20:55:11 GMT:191/347:11054:69fa592f.136
```
that is: `%l:%e:%s:%v:%x:%c`. Split on `:` from the right (since `%s` contains internal `:` separators in its time-of-day component). Use a custom backwards splitter:

```go
// splitFromRight splits s on sep into n fields, taking the rightmost n-1
// separators. The first field is whatever remains and may itself contain
// separators.
func splitFromRight(s, sep string, n int) []string {
    if n <= 1 {
        return []string{s}
    }
    out := make([]string, n)
    for i := n - 1; i > 0; i-- {
        idx := strings.LastIndex(s, sep)
        if idx == -1 {
            return nil
        }
        out[i] = s[idx+len(sep):]
        s = s[:idx]
    }
    out[0] = s
    return out
}
```

Then with the `%l:%e:%s:%v:%x:%c` segment:
```go
fields := splitFromRight(prefixSeg, ":", 6) // [%l, %e, %s, %v, %x, %c]
if len(fields) == 6 {
    backendStart := fields[2] // contains internal spaces, no internal ':'
                              // wait — %s does contain ':' in HH:MM:SS.
                              // splitFromRight collapses those into fields[0].
}
```

> **Worker note:** The naive `splitFromRight(seg, ":", 6)` will fail because `%s` (`2026-05-05 20:55:11 GMT`) contains `:` characters. Instead, anchor on stable shapes from the right:
> - `%c` is `XXXXXXXX.XXXX` (hex.hex), no colons.
> - `%x` is `\d+`, no colons.
> - `%v` is `\d+/\d+`, no colons.
> - `%e` is `[A-Z0-9]{5}`, no colons.
> - `%l` is `\d+`, no colons.
>
> So splitting from the right by `:` six times consumes six tokens that have no internal `:` — except that `%s` is **inside** the segment between `%e` and `%v` and contains `:`. The reliable form is to splitFromRight by 5 (giving you `%c, %x, %v, %s, %e`) and let the leftmost remainder be `%l`. **Also note:** because `%s` contains `:`, we must accept its raw form (with the colons) as the backend_start string and not try to split it further.

A safer recipe:

```go
// segment looks like:  %l:%e:%s:%v:%x:%c
//
// Right-anchored extraction:
sessionID, segNoSession := popRight(segment, ":")    // %c
xid, segNoXid           := popRight(segNoSession, ":")
vxid, segNoVxid         := popRight(segNoXid, ":")
// What remains is "%l:%e:%s". %s has ':' inside, so split by the next ':'
// from the LEFT to get %l, then by the next ':' to get %e, and the rest
// is %s.
ll, rest := popLeft(segNoVxid, ":")                  // %l
sqlstate, backendStart := popLeft(rest, ":")
```

Add `popLeft` and `popRight` helpers (or inline them). Document that `backendStart` is the raw `%s` value (e.g., `"2026-05-05 20:55:11 GMT"`) and is forwarded to Loki metadata verbatim.

- [ ] **Step 5: Capture the error message body**

After the existing severity detection:
```go
// messageStart is the index of the severity token (e.g. "ERROR")
errorMessage := strings.TrimSpace(line[messageStart+len(severity)+1:]) // skip "<sev>:"
// Trim the leading "  " left by `<sev>:  message`.
errorMessage = strings.TrimLeft(errorMessage, " ")
```

- [ ] **Step 6: Populate all the new fields when writing into `pendingErrors`**

Replace the current `&pendingError{...}` literal with the full-fields version:
```go
l.pendingErrors[pid] = &pendingError{
    receivedAt:      time.Now(),
    severity:        severity,
    sqlstate:        sqlstateCode,
    sqlstateClass:   sqlstateClass,
    datname:         database,
    user:            user,
    timestamp:       parsedTimestamp,            // from the existing timestamp parse
    clientAddr:      clientAddr,                 // parseRemote(...)
    clientPort:      clientPort,
    pid:             pid,
    backendStart:    backendStart,
    xid:             normalizedXid(xidRaw),      // helper that returns "" if "0"
    sessionID:       sessionID,
    applicationName: applicationName,
    errorMessage:    errorMessage,
}
```

Add `normalizedXid`:
```go
func normalizedXid(s string) string {
    if s == "0" {
        return ""
    }
    return s
}
```

- [ ] **Step 7: Tests for parsing helpers**

Add `TestParseRemote`, `TestSplitFromRight`/`TestPopRight`, and `TestNormalizedXid` to `logs_test.go`. Table-driven, exhaustive over the input shapes:

```go
func TestParseRemote(t *testing.T) {
    cases := []struct {
        in           string
        wantHost     string
        wantPort     string
    }{
        {"172.18.0.3(34492)", "172.18.0.3", "34492"},
        {"[local]", "[local]", ""},
        {"", "", ""},
        {"weird-no-parens", "weird-no-parens", ""},
        {"::1(5432)", "::1", "5432"},
    }
    for _, c := range cases {
        gotH, gotP := parseRemote(c.in)
        require.Equal(t, c.wantHost, gotH)
        require.Equal(t, c.wantPort, gotP)
    }
}
```

- [ ] **Step 8: Run helper tests**

Run:
```bash
go test -count=1 -v ./internal/component/database_observability/postgres/collector/ -run "TestParseRemote|TestPopRight|TestNormalizedXid"
```
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs.go internal/component/database_observability/postgres/collector/logs_test.go
git commit -m "feat(database_observability/postgres): capture full prefix + error_message metadata"
```

---

## Phase 2 — Emit the `op="error"` Loki entry

### Task 3: Wire the `entryHandler` to emit on every successful flush

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs.go`

- [ ] **Step 1: Replace the `incrementByFingerprint` stub with the entry-emitting body**

The function name stays for compatibility with the call sites (or rename to `emitErrorEntry` and update both `flushStatementLocked` and `processBareContinuation` — preferred). Final shape:

```go
func (l *Logs) emitErrorEntry(entry *pendingError, stmt string) {
    if entry == nil {
        return
    }
    fp, _, err := fingerprint.Fingerprint(stmt, fingerprint.SourceLog, 0)
    if err != nil {
        // fingerprint.ErrEmpty — skip; the SQL was empty whitespace.
        return
    }

    // Stable labels: low cardinality. These ride on the Loki stream.
    labels := model.LabelSet{
        "op":             "error",
        "severity":       model.LabelValue(entry.severity),
        "sqlstate_class": model.LabelValue(entry.sqlstateClass),
        "datname":        model.LabelValue(entry.datname),
    }

    // Body is a single logfmt line. No structured metadata — every
    // high-cardinality field lives here so that `| logfmt` in LogQL
    // surfaces them as parsed labels at query time.
    line := buildErrorLine(entry, fp, stmt)

    ts := entry.timestamp
    if ts.IsZero() {
        ts = time.Now()
    }

    l.entryHandler.Chan() <- loki.Entry{
        Labels: labels,
        Entry: push.Entry{
            Timestamp: ts,
            Line:      line,
        },
    }
}

// buildErrorLine assembles a logfmt line containing every high-cardinality
// field for one ERROR+STATEMENT pair. The `statement` field carries the
// assembled SQL; newlines and tabs are collapsed to single spaces so the
// whole entry stays a single logfmt line.
func buildErrorLine(entry *pendingError, fp, stmt string) string {
    fields := []struct{ k, v string }{
        {"sqlstate", entry.sqlstate},
        {"query_fingerprint", fp},
        {"pid", entry.pid},
        {"backend_start", entry.backendStart},
        {"application_name", entry.applicationName},
        {"client_addr", entry.clientAddr},
        {"client_port", entry.clientPort},
        {"session_id", entry.sessionID},
        {"user", entry.user},
        {"error_message", entry.errorMessage},
        {"statement", collapseWhitespace(stmt)},
    }
    if entry.xid != "" {
        // Insert xid right after sqlstate to keep ordering stable for tests.
        fields = append(fields[:1],
            append([]struct{ k, v string }{{"xid", entry.xid}}, fields[1:]...)...)
    }

    var b strings.Builder
    for i, f := range fields {
        if f.v == "" && f.k != "statement" && f.k != "error_message" {
            // Skip empty optional fields except those that are always present.
            continue
        }
        if i > 0 && b.Len() > 0 {
            b.WriteByte(' ')
        }
        b.WriteString(f.k)
        b.WriteByte('=')
        b.WriteString(logfmtQuote(f.v))
    }
    return b.String()
}

// collapseWhitespace reduces all runs of whitespace (including tabs and
// newlines) to a single space, then trims. Used so multi-line SQL fits in
// a single-line logfmt body.
func collapseWhitespace(s string) string {
    var b strings.Builder
    b.Grow(len(s))
    inSpace := false
    for _, r := range s {
        if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
            if !inSpace {
                b.WriteByte(' ')
                inSpace = true
            }
            continue
        }
        b.WriteRune(r)
        inSpace = false
    }
    return strings.TrimSpace(b.String())
}

// logfmtQuote returns v as a logfmt value: bare if safe, otherwise wrapped
// in double quotes with internal `"` and `\` backslash-escaped.
func logfmtQuote(v string) string {
    needsQuote := strings.ContainsAny(v, " =\"")
    if !needsQuote {
        return v
    }
    var b strings.Builder
    b.Grow(len(v) + 2)
    b.WriteByte('"')
    for _, r := range v {
        if r == '"' || r == '\\' {
            b.WriteByte('\\')
        }
        b.WriteRune(r)
    }
    b.WriteByte('"')
    return b.String()
}
```

Imports to add at the top:
```go
"github.com/grafana/loki/pkg/push"
"github.com/prometheus/common/model"
```

- [ ] **Step 2: Update the call sites**

Rename the calls in `flushStatementLocked` and `processBareContinuation` from `l.incrementByFingerprint(entry, stmt)` to `l.emitErrorEntry(entry, stmt)`. Behavioural semantics are identical — the function only differs in what it does after looking up `entry`.

- [ ] **Step 3: Refactor — remove `incrementByFingerprint` if you renamed**

If the rename happened in Step 1, ensure no leftovers. Keep the new helper alongside `flushExpiredPending` for locality.

- [ ] **Step 4: Compile**

Run:
```bash
CGO_ENABLED=1 go build ./internal/component/...
go vet ./internal/component/database_observability/postgres/...
```
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs.go
git commit -m "feat(database_observability/postgres): emit op=error Loki entry on each ERROR+STATEMENT pair"
```

---

### Task 4: Replace counter tests with entry assertions

**Files:**
- Modify: `internal/component/database_observability/postgres/collector/logs_test.go`

This task is repetitive but mechanical. For each test that was `t.Skip`-ped in Task 1 Step 7, drop the skip and rewrite the assertion against `entryHandler.Chan()` instead of the counter.

A reusable helper at the top of the test file:
```go
// drainEntries reads all currently-buffered Loki entries from the handler,
// up to a deadline. Returns them in arrival order.
func drainEntries(t *testing.T, handler loki.EntryHandler, want int, timeout time.Duration) []loki.Entry {
    t.Helper()
    out := make([]loki.Entry, 0, want)
    deadline := time.Now().Add(timeout)
    for len(out) < want && time.Now().Before(deadline) {
        select {
        case e := <-handler.Chan():
            out = append(out, e)
        case <-time.After(25 * time.Millisecond):
        }
    }
    return out
}
```

> Worker note: when constructing `LogsArguments` in tests, you must pass an `EntryHandler` with a real channel that you control, e.g.:
> ```go
> entryCh := make(chan loki.Entry, 16)
> handler := loki.NewEntryHandler(entryCh, func() {})
> // ... pass `handler` as EntryHandler in LogsArguments ...
> // assert via drainEntries(t, handler, ...)
> ```

- [ ] **Step 1: Rewrite `TestLogsCollector_IncrementsErrorsByFingerprint_OnErrorPlusStatement` → `TestLogsCollector_EmitsErrorEntry_OnErrorPlusStatement`**

Assert:
- exactly 1 entry arrived,
- Labels: `op=error severity=ERROR sqlstate_class=42 datname=books_store`,
- Line body parses as logfmt and contains the expected key/value pairs:
  - `query_fingerprint` matches `fingerprint.Fingerprint(SQL, SourceLog, 0)`,
  - `pid`, `application_name`, `session_id`, `user`,
  - `sqlstate=42P01`,
  - `error_message="..."` containing `relation "missing" does not exist`,
  - `statement="..."` containing the SQL with whitespace collapsed,
  - `xid` is **absent** from the body (input had `xid=0`),
- `Timestamp` matches the parsed `%m`.

Use a small helper:
```go
// parseLogfmt is a minimal logfmt parser for tests — supports bare and
// double-quoted values with `\"` and `\\` escapes. Returns map[k]v.
func parseLogfmt(t *testing.T, line string) map[string]string { ... }
```

- [ ] **Step 2: Rewrite `TestLogsCollector_TimedOutPendingDoesNotIncrementFingerprintCounter` → `TestLogsCollector_TimedOutPendingDoesNotEmitErrorEntry`**

Assert: zero entries in `entryHandler.Chan()` after `2 × pendingErrorTimeout` has elapsed.

- [ ] **Step 3: Rewrite `TestLogsCollector_DisplacedPendingDoesNotDoubleCount` → `TestLogsCollector_DisplacedPendingEmitsExactlyOneEntry`**

Same scenario as before: same PID, two ERROR lines, then one STATEMENT. Assert exactly one entry, with the *second* error's labels/metadata and the STATEMENT body's fingerprint.

- [ ] **Step 4: Rewrite `TestLogsCollector_PrefixedStatement_MultiLine` → `TestLogsCollector_EmitsErrorEntry_PrefixedMultiLineStatement`**

Same scenario; assert the entry's body is logfmt where the `statement` field equals the **whitespace-collapsed** assembled SQL, and `query_fingerprint` matches `fingerprint.Fingerprint(assembledSQL, fingerprint.SourceLog, 0)` (fingerprint is computed on the original multi-line text, not the collapsed form — they parse to the same AST).

- [ ] **Step 5: Rewrite `TestLogsCollector_StatementSurvivesTimeoutFlush` → `TestLogsCollector_StatementSurvivesTimeoutFlush_EmitsEntry`**

Same scenario; assert the entry arrives within `2 × pendingErrorTimeout` from a quiet log channel.

- [ ] **Step 6: Add a new test asserting `xid` field presence/absence in the body**

```go
func TestLogsCollector_OmitsXidFieldWhenZero(t *testing.T) { ... }
func TestLogsCollector_IncludesXidFieldWhenNonZero(t *testing.T) { ... }
```

- [ ] **Step 7: Add a new test asserting `application_name` round-trips**

Use `[unknown]` (the default) and a real value (e.g. `psql`); assert the parsed body's `application_name` field reflects each. Confirm that values containing characters that need quoting (`[unknown]` has `[`/`]`, fine; values with spaces would be quoted) round-trip correctly through `parseLogfmt`.

- [ ] **Step 8: Run all logs tests**

Run:
```bash
go test -count=1 ./internal/component/database_observability/postgres/collector/ -run TestLogsCollector
```
Expected: green.

- [ ] **Step 9: Commit**

```bash
git add internal/component/database_observability/postgres/collector/logs_test.go
git commit -m "test(database_observability/postgres): replace fingerprint-counter tests with op=error entry assertions"
```

---

## Phase 3 — Documentation

### Task 5: Replace metric docs with op docs

**Files:**
- Modify: `docs/sources/reference/components/database_observability/database_observability.postgres.md`

- [ ] **Step 1: Remove the `database_observability_pg_errors_by_fingerprint_total` block**

Delete the table block introduced in the previous shipping; the parent metric `pg_errors_total` stays.

- [ ] **Step 2: Add a "Emitted Loki entries" subsection under `## logs collector`**

Section copy:

```markdown
### Emitted Loki entries

The `logs` collector forwards a Loki entry to its `forward_to` target for every
PostgreSQL `ERROR`/`FATAL`/`PANIC` for which the matching `STATEMENT:`
continuation was successfully observed. The entry uses a small set of stable
labels and encodes everything else as logfmt key/value pairs in the line body
— no Loki structured metadata is used, so the entries are portable across
Loki versions and downstream tooling that expects line-only ingest.

| Field                  | Source                  | Notes                                                       |
|------------------------|-------------------------|-------------------------------------------------------------|
| Label `op`             | constant                | `error`                                                     |
| Label `severity`       | log keyword             | `ERROR`, `FATAL`, or `PANIC`                                |
| Label `sqlstate_class` | first 2 chars of `%e`   | `40`, `42`, `53`, `23`, ...                                 |
| Label `datname`        | `%d`                    | database name                                               |
| Body field `sqlstate`  | `%e`                    | full 5-character SQLSTATE                                   |
| Body field `xid`       | `%x`                    | omitted when `0` (read-only / not yet assigned)             |
| Body field `query_fingerprint` | computed         | `libpg_query` fingerprint of the SQL text (16-char hex)     |
| Body field `pid`       | `%p`                    | backend PID                                                 |
| Body field `backend_start`     | `%s`             | session start timestamp, raw text                           |
| Body field `application_name`  | `%a`             | typically `[unknown]` unless set client-side                |
| Body field `client_addr` | host portion of `%r`  | `[local]` for unix-domain                                   |
| Body field `client_port` | port portion of `%r`  | empty for unix-domain                                       |
| Body field `session_id`  | `%c`                  | unique per backend connection                               |
| Body field `user`        | `%u`                  | also present on `pg_errors_total` as a label                |
| Body field `error_message` | text after `<sev>:` | human-readable error message                                |
| Body field `statement`   | STATEMENT body        | assembled SQL with whitespace collapsed to single spaces    |

Compute error rate per logical query in LogQL:

\`\`\`logql
sum by (datname, query_fingerprint)
  (count_over_time({op="error"} | logfmt [5m]))
\`\`\`

Correlate to `pg_stat_activity` samples emitted by the `query_samples`
collector by joining on `query_fingerprint` AND `pid` AND a small time window
around the entry's timestamp. If the failed query was inside a write
transaction, `xid` is also a deterministic key against
`pg_stat_activity.backend_xid`.
```

- [ ] **Step 3: Commit**

```bash
git add docs/sources/reference/components/database_observability/database_observability.postgres.md
git commit -m "docs(database_observability/postgres): document op=error Loki entries (replace fingerprint metric docs)"
```

---

## Self-Review Checklist

**Spec coverage:**
- ✅ Drop the high-cardinality counter (`pg_errors_by_fingerprint_total`) — Task 1.
- ✅ Capture all fields the user asked for, including `error_message` — Task 2.
- ✅ Emit `op="error"` (no `pg_` prefix) Loki entries — Task 3.
- ✅ `xid` only when `≠ 0` — Task 3 Step 1.
- ✅ Tests cover entry presence, content, displacement, timeout flush, multi-line, and xid presence/absence — Task 4.
- ✅ Documentation updated — Task 5.
- ✅ `pg_errors_total` unchanged — no task touches it.
- ✅ The fingerprint package and the parser state machine (TAB-continuation, prefixed-STATEMENT, flush ordering) are reused, not re-implemented.

**Placeholder scan:** No "TBD", "implement later", or "appropriate error handling" placeholders. Every code step has the exact code or a precise prescription; every command step has the exact command and expected output.

**Type consistency:**
- `pendingError` field set in Task 2 is read by `emitErrorEntry` in Task 3.
- `loki.Entry` shape matches `loki.EntryHandler.Chan()` consumer side already used elsewhere in alloy.
- Stable label keys are quoted as `model.LabelValue`; everything else lives in the logfmt body, no structured metadata.

**Open follow-ups (out of scope of this plan):**
- Adding `virtualtransaction` to `query_samples` (one extra column joined from `pg_locks`) for deterministic `(pid, vxid)` correlation regardless of read/write.
- The full join metric `database_observability_query_hash_info` populated by `query_details` — separate plan/PR.
- `pg_slow_query` log entries — separate plan/PR.
- Surfacing `repaired=true` and sentinel hits as their own observability counters — useful for detecting upstream truncation but not required for this deliverable.
- Switching `log_line_prefix` to include `%Q` (`query_id`) where the customer's PG version + parameter group allow it — would make the correlation key exact.

---

## Implementation handoff

After saving this plan, two execution options:

**1. Subagent-Driven (recommended)** — Dispatch a fresh subagent per task with a two-stage review (spec compliance, then code quality) before moving on. Best for keeping context tight and catching issues early.

**2. Inline Execution** — Execute tasks sequentially in the same session using `superpowers:executing-plans`, with checkpoints between phases.

The TDD discipline is identical either way: every behavior task starts with a failing test (entry-handler-shaped, not counter-shaped), then minimal implementation, then green run, then commit.
