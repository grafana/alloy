# Windows CI investigation — September 2026

Handoff notes. Written to be picked up cold by another engineer or agent.

**Status: shelved.** Two fixes are committed on this branch, one is on a sibling
branch, and two problems are unaddressed. Nothing has been opened as a PR.

This file is a working note, not product documentation. It is not intended to
merge to `main` as-is.

---

## 1. Summary

The `Test (Windows)` job ([`.github/workflows/test_windows.yml`](.github/workflows/test_windows.yml))
has been red continuously. Across the **last 50 commits on `main`** (2026-09-17
→ 2026-09-21) there were **0 successes**: 47 failures, 1 cancelled, and 2 with no
conclusion. 46 of the 47 failing logs were classified (one had already expired).

Most runs fail for two or three reasons at once, so the counts below overlap.

| # | Cause | Runs | Kind | Status |
|---|-------|-----:|------|--------|
| 1 | `TestSyncServicesDocPreservesPermissions` asserts POSIX mode bits | 39/47 | Deterministic test bug | **Fixed** on `ptodev/windows-ci-fix` |
| 2 | Two `positions` tests rely on "a directory can't be read as a file" | 39/47 | Deterministic test bug | **Fixed** on this branch |
| 3 | `TestTracker/last_marked_segment_is_updated_when_sends_complete` | 21/47 | **Real product bug** | **Fixed** on this branch |
| 4 | Runner disk exhaustion | 7/47 | Infrastructure | Not addressed |
| 5 | Assorted single-occurrence flakes | 16/47 | Flake tail | Not addressed |

Separately, PR-triggered runs hit a sixth problem (mise tool download 504) which
never appeared in the `main` push runs. See [§7](#7-issue-5--mise-504-on-pr-runs).

Causes 1 and 2 are deterministic and land in the **same 39 runs**, which is why
the job never passed. Cause 3 is a genuine bug in `loki.write`'s WAL marker and
is the only finding with production consequences.

---

## 2. Issue 1 — CloudWatch docs permissions test

**Fixed on branch `ptodev/windows-ci-fix`, commit `13f228b29`** (already pushed).
Not on this branch.

```
doc_test.go:69: synced file permissions = -rw-rw-rw-, want -rw-r-----
```

The test wrote a file with `0o640` and asserted `Mode().Perm() == 0o640`. Windows
has no POSIX mode bits — Go synthesises `0666` for a writable file and `0444` for
a read-only one — so the assertion could never hold there.

The deeper point: **the check was testing nothing we own.** `os.WriteFile` is
documented as

> If the file does not exist, WriteFile creates it with permissions perm (before
> umask); otherwise WriteFile truncates it before writing, **without changing
> permissions**.

The stat-and-reuse block in `syncServicesDoc` only ran when the file existed —
exactly the case where `perm` is ignored. So the mode-preservation code was dead,
and the test passed with or without it. Verified empirically: writing with
`0o644` over an existing `0o640` file leaves it `-rw-r-----` and updates the
contents.

Fix: delete the dead mode handling, pass `0o644` directly, and replace the
vacuous test with `TestSyncServicesDocUpdatesExistingFile`, which asserts the
content is rewritten. That path was previously covered *only* by the permissions
test — `TestSyncServicesDoc` starts from a non-existent file, so it exercised
create-then-no-op only. Deleting the old test outright would have silently
dropped overwrite coverage.

---

## 3. Issue 2 — Legacy positions tests

**Fixed on this branch, commit `54674f14c`.**

```
positions_test.go:125: Error: An error is expected but got nil.
```

Both `TestLegacyConversionUnreadableFile` and
`TestConvertLegacyPositionsFileJournalUnreadableFile` `os.Mkdir` the legacy path,
with a comment claiming *"A directory can't be read as a file, so this reliably
forces a read error without relying on chmod/permission bits."* That is a
Unix-only assumption, and the failure is subtler than the comment suggests:

[`positions.go`](internal/component/loki/source/internal/positions/positions.go),
`readLegacyFile`, before the fix:

```go
if oldFile.Size() == 0 {
    l.Info("no legacy positions file found", "path", legacyPath)
    return nil, nil          // ← Windows exits here
}
buf, err := os.ReadFile(clean) // ← never reached on Windows
```

`os.Stat` on a directory reports `Size() == 0` on Windows but a non-zero block
size on Unix (64 on macOS, 4096 on Linux), where `ReadFile` then fails with
`is a directory`. So on Windows `readLegacyFile` short-circuited to `(nil, nil)`
meaning "no legacy file", the conversion treated that as "nothing to do", and the
expected error never materialised.

This was **not only a test problem.** `loki.source.file` and
`loki.source.journal` would silently ignore a directory at
`legacy_positions_file` on Windows while failing loudly on Linux — the opposite
of what [#6750](https://github.com/grafana/alloy/pull/6750) ("Fail loudly on
unreadable legacy positions file") intended.

Fix: reject anything that is not a regular file ahead of the size check.

> **Behaviour change.** On Windows, a directory (or pipe, or device) at
> `legacy_positions_file` now fails startup instead of being ignored. That
> matches Linux today, so it reads as a bug fix rather than a breaking change —
> but it is a product behaviour change and the reviewer should be told.

An alternative that keeps this test-only would be to inject a read error instead,
which needs a larger refactor of `readLegacyFile`'s shape.

---

## 4. Issue 3 — The WAL segment marker bug (the real one)

**Fixed on this branch, commit `d8f28326f`.** This is the finding that matters.

### What the marker is for

The marker file records the last WAL segment fully consumed. On restart the
Watcher resumes *after* that number, so a stale marker means already-delivered
segments get re-read and re-sent.

### The bug: a failed write was treated as a successful one

`MarkSegment` logged its error and returned nothing, and the caller advanced its
in-memory position unconditionally:

```go
// tracker.go, before the fix
if markableSegment > t.lastMarkedSegment {
    t.file.MarkSegment(markableSegment)   // may have failed
    t.lastMarkedSegment = markableSegment // advances anyway
    t.metrics.lastMarkedSegment.WithLabelValues().Set(float64(markableSegment))
}
```

After a failed write the file said 10 while `t.lastMarkedSegment` said 11 —
**permanently diverged**, for three compounding reasons:

1. The guard is `markableSegment > t.lastMarkedSegment`, and 11 is never `> 11`,
   so segment 11 is never attempted again.
2. `findMarkableSegment` *deletes* consumed entries as it scans, so segment 11
   was already gone from the map and could not be rediscovered.
3. `runFindTicker` never triggered a find by itself. It sat in a non-blocking
   `default` select *after* the main one, so it only enabled a find on the
   *next* data update. With no further data, nothing ran — despite the comment
   claiming it forced a run "every second".

### The fix — four parts

All in
[`internal/component/common/loki/client/internal/marker/`](internal/component/common/loki/client/internal/marker/):

| Where | Change |
|-------|--------|
| `file.go`, `MarkSegment` | Returns `error` |
| `tracker.go`, `runUpdatePendingData` | Advance `lastMarkedSegment` and the metric **only on a successful write** |
| `tracker.go`, `runUpdatePendingData` | New `pendingSegment` local, outliving the loop iteration — survives the map deletion in (2) |
| `tracker.go`, `runUpdatePendingData` | Ticker moved **into** the blocking select, so it drives finds and retries with no data flowing |

The last two are what make a retry reachable at all. Without either of them, not
advancing the position would merely stall the marker instead of diverging it.

The same commit also makes `findInterval` injectable via an unexported
`newSegmentTracker`, leaving `NewSegmentTracker` and its `defaultFindInterval =
time.Second` unchanged for production. `TestTracker` drops from **5.42s to
0.16s**. That part is an independent speedup, not part of the bug fix.

### Points a reviewer should push on

- Moving the ticker into the select is the one steady-state behaviour change: a
  find now runs every second even when idle. It is cheap —
  `findMarkableSegment` on an empty map returns `-1` and short-circuits — and it
  matches what the comment always claimed. But it is new.
- `pendingSegment` is a local rather than a struct field because only the
  `runUpdatePendingData` goroutine touches it, so no lock is needed. Worth
  agreeing that invariant is real.

### Evidence from CI — this is observed, not inferred

The mechanism is Windows-specific:

1. `MarkSegment` → `atomic.WriteFile` → on Windows,
   `MoveFileExW(..., MOVEFILE_REPLACE_EXISTING)`
   (`natefinch/atomic@v1.0.1/file_windows.go`).
2. Go's `os.Open` on Windows opens with `FILE_SHARE_READ|FILE_SHARE_WRITE` and
   **not** `FILE_SHARE_DELETE` (`syscall/syscall_windows.go:395` in Go 1.26.7,
   reached via `os.openFileNolog`). An open read handle therefore makes that
   rename fail with a sharing violation.
3. `File.LastMarkedSegment()` does an `os.ReadFile` on **every** call, and
   `require.Eventually` polled it every 100ms — so the assertion was opening the
   very file the tracker was trying to replace. The test manufactured its own
   collision.

All **21 of 21** marker-failing runs contain the matching error, a perfect 1:1
set match with the test failures:

```
level=error msg="could not replace segment marker file"
  file=...\TestTrackerlast_marked_segment_is_updated_when_sends_complete...\001\remote\segment_marker
  err="cannot replace ...\segment_marker with tempfile ...\segment_marker1962904912:
       replace ... : Access is denied."
```

On Linux, `rename(2)` over an open file just works, which is why this never
showed there.

> Quote that log line in the PR description. GitHub expires Actions logs after 90
> days, so the run links in [§8](#8-example-ci-runs) will go dead.

### Not a regression

Both halves of the defect are present verbatim in
[`abdaca048`](https://github.com/grafana/alloy/commit/abdaca048), *"Implement WAL
replay and markers for `loki.write` (#5590)"*, **2023-11-02**. The ticker
placement was original too. Everything since has been moves and renames:

| Commit | Date | What |
|--------|------|------|
| [`abdaca048`](https://github.com/grafana/alloy/commit/abdaca048) | 2023-11-02 | Introduced the feature, with both defects |
| [`0e36aa27c`](https://github.com/grafana/alloy/commit/0e36aa27c) | 2024-02-29 | Bulk move into `internal/` |
| [`ffa64d2a5`](https://github.com/grafana/alloy/commit/ffa64d2a5) | 2026-05-05 | `chore(loki): Cleanup marker code` — "Nothing functional have changed" |
| [`349a82ace`](https://github.com/grafana/alloy/commit/349a82ace) | 2026-06-03 | slog migration; touched only the log call |

So there is no revert to reach for. The fix is a behaviour change to
long-standing code and deserves its own reviewable commit.

---

## 5. Customer impact of Issue 3

**Low real-world risk — materially lower than the 21/47 CI failure rate
suggests.** Four conditions must all hold.

**1. The WAL must be explicitly enabled.** It is off by default
([`write.go`](internal/component/loki/write/write.go), `WalArguments.SetToDefault`
sets `Enabled: false`, with a standing `todo(thepalbi)` about eventually
flipping it), and the `wal` block is marked **experimental** in
[the docs](docs/sources/reference/components/loki/loki.write.md).
[#7030](https://github.com/grafana/alloy/issues/7030) is an open question about a
roadmap to GA. Without `wal { enabled = true }`, `loki.write` uses
`marker.NewNopTracker()` (`consumer_fanout.go`) and none of this code runs. The
real tracker is only built in `consumer_wal.go`.

**2. Windows.** On Linux the rename cannot fail this way.

**3. Something must make the rename fail.** This is the key mitigation: the CI
trigger is **self-inflicted by the test**. Production has no polling reader — the
Watcher reads the marker only once per `run()` attempt, at startup and after an
error (`wal/watcher.go`, `mainLoop`). Alloy never races itself here. In
production the collision needs an *external* handle holder: antivirus, a backup
agent, a file indexer. That is a plausible and well-known hazard for
atomic-rename-on-Windows, but **there is no field evidence for it.**

**4. A restart must land in the stale window.** The bug self-heals: once any
*higher* segment is consumed, that write succeeds and the marker jumps forward to
a correct value. Exposure is only from the failed write until the next segment
completes — typically seconds to minutes under steady traffic.

### If it does hit

**Duplicate delivery, never loss.** A failed write leaves the marker *behind*
reality, so the Watcher resumes earlier and re-sends already-delivered segments.
It cannot end up ahead. The WAL is at-least-once by design, so this widens an
existing duplicate window rather than breaking a guarantee.

The genuinely bad case is a **persistent** failure — wrong permissions on the
marker directory, say. Pre-fix, every new segment attempts one write, fails, and
advances in-memory anyway, so the on-disk marker **never moves** and a restart
replays the entire retained WAL: a large duplicate burst with only an
`error`-level log line as warning. Post-fix that scenario still cannot write, but
`lastMarkedSegment` no longer lies and the `lastMarkedSegment` metric tracks
durable state rather than intent — so it becomes alertable.

### Related issue worth reading first

Nothing in the tracker matches this in the field. But
[#7112](https://github.com/grafana/alloy/issues/7112) (open, 2026-09-16) is in
exactly this code: *"loki.write WAL skips unsent entries appended to an
already-marked open segment after SIGKILL."* That is the **opposite polarity** —
marker ahead of reality, so actual data loss — and a different defect. It is
adjacent enough to read before writing the PR description, and together the two
suggest the marker logic deserves a broader review than either fix alone.

**Recommended framing:** a correctness fix found via CI, with low but non-zero
production exposure confined to Windows users of an experimental opt-in feature.
Not a backport scramble.

---

## 6. Issue 4 — Runner disk exhaustion (not addressed)

7 of 47 runs, all confined to **2026-09-17 → 2026-09-18**. These died during
compilation and never reached a test:

```
compile: writing output: write $WORK\b5404\_pkg_.a: There is not enough space on the disk.
##[error]There is not enough space on the disk. : 'C:\actions-runner\...\_diag\...'
```

The runner filled up mid-build — even the Actions runner's own diagnostic log
writes failed. `make test` builds every package in the repo twice (once with
`-race`), and `windows-latest` has the least free space of the hosted images.

Worth a free-space step or trimming the build cache. Independent of everything
above.

Run IDs: `35220264263`, `35227211979`, `35266912259`, `35320460986`,
`35362030842`, `35364894830`, `35378107912`.

---

## 7. Issue 5 — mise 504 on PR runs (not addressed)

Did **not** appear in any `main` push run — PR-triggered only. Example:
[run 35609717650, job 106391380975](https://github.com/grafana/alloy/actions/runs/35609717650/job/106391380975).

```
mise WARN  HTTP GET .../shellcheck-v0.11.0.zip attempt 1 failed (transient): 504 Gateway Timeout
                                              attempt 2 ... attempt 3 ...
mise ERROR Failed to install aqua:koalaman/shellcheck@0.11.0: 504 Gateway Timeout
make: *** [Makefile:225: test] Error 1
```

The Test step failed after 30s without compiling anything.

**Cause.** `test_windows.yml` correctly scopes setup to `install_args: go`, but
`jdx/mise-action` defaults `add_shims_to_path: true`, so `…/mise/shims` is on
`PATH` for later steps. When something in `make test` resolves through a shim,
mise's command-not-found auto-install handler installs the **entire** `mise.toml`
toolset — the log shows node, task, golangci-lint, kind, helm, helm-docs,
govulncheck and shellcheck all starting. So the test job depends on eight GitHub
release downloads succeeding; any one 5xx kills it. mise's built-in retry gave up
after three attempts in ~4 seconds.

mise-action's cache does not help: the cache key covers `install_args: go`, so
shellcheck is missing on a cache hit too and the auto-install fires every run.

**Proposed fix** — turn off on-demand installs job-wide so the toolset is exactly
what `install_args` asked for:

```yaml
jobs:
  test_windows:
    env:
      # The setup step installs only Go. Without these, a mise shim on PATH
      # triggers an on-demand install of the whole mise.toml toolset, so an
      # unrelated GitHub release outage fails the test job.
      MISE_AUTO_INSTALL: "false"
      MISE_NOT_FOUND_AUTO_INSTALL: "false"
```

Both default to `true`. This is the safe variant — it leaves `GOROOT`/`GOBIN`/
`PATH` exactly as they are and only removes the implicit fetch.
`add_shims_to_path: false` would also work but changes how `go` resolves, so it
needs a trial run.

Applies to every workflow pairing `install_args` with a later build step:
`test_pr.yml`, `test_mac.yml`, `test_full.yml`, `integration-tests*.yml`,
`check-generate-otel-collector-distro.yml`.

As a secondary hedge for downloads that *are* needed (the `lint.yml` shellcheck
job genuinely needs it): `MISE_HTTP_RETRIES: "5"`, `MISE_HTTP_TIMEOUT: "60s"`.

---

## 8. Example CI runs

All `Test (Windows)` → job `Test`, on pushes to `main`. **Logs expire after 90
days.**

### Issue 3 — the marker bug (21 runs, all also carrying Issues 1 and 2)

| Date | Run | Commit |
|------|-----|--------|
| 2026-09-17 07:11 | [35193285349](https://github.com/grafana/alloy/actions/runs/35193285349) | `3df385db6` fix(loki.write): Stop started endpoints when one of them fails (#7115) |
| 2026-09-17 18:41 | [35260341702](https://github.com/grafana/alloy/actions/runs/35260341702) | `15a736cb1` chore(loki.process): Fix flaky tests (#7132) |
| 2026-09-17 18:43 | [35260516240](https://github.com/grafana/alloy/actions/runs/35260516240) | `58a5b9722` fix(remotecfg): Use atomic writes for cache file (#7131) |
| 2026-09-18 11:25 | [35339525107](https://github.com/grafana/alloy/actions/runs/35339525107) | `e11dc7c2a` chore(otelcol.receiver.loki): Implement consumer (#7154) |
| 2026-09-18 11:55 | [35342036329](https://github.com/grafana/alloy/actions/runs/35342036329) | `13ba33586` refactor(otelcol): Seed factory defaults (group 3 of 5) (#7089) |
| 2026-09-18 12:29 | [35344966577](https://github.com/grafana/alloy/actions/runs/35344966577) | `08135d8f7` refactor(otelcol): Seed factory defaults (group 2 of 5) (#7085) |
| 2026-09-18 13:29 | [35350486658](https://github.com/grafana/alloy/actions/runs/35350486658) | `4017d4e38` feat: OTel service installation support for windows service (#7007) |
| 2026-09-18 14:06 | [35354245192](https://github.com/grafana/alloy/actions/runs/35354245192) | `406030b0b` fix(otelcol.processor.memory_limiter)!: Apply upstream GC interval defaults (#7137) |
| 2026-09-18 15:14 | [35361237488](https://github.com/grafana/alloy/actions/runs/35361237488) | `a872c11d5` feat(prometheus.remote_write): Expose sigv4 session_name and tags (#7145) |
| 2026-09-18 15:20 | [35361799193](https://github.com/grafana/alloy/actions/runs/35361799193) | `bb080ea4a` feat(otelcol.receiver.tcplog): Add auth argument (#7148) |
| 2026-09-18 15:22 | [35361997061](https://github.com/grafana/alloy/actions/runs/35361997061) | `9ae05d83a` feat(otelcol.exporter.otlphttp,faro): Add keepalive block (#7152) |
| 2026-09-18 16:18 | [35367697578](https://github.com/grafana/alloy/actions/runs/35367697578) | `89929a2e6` feat(database_observability.sql_server): Add `query_timeout` (#7157) |
| 2026-09-18 21:28 | [35396946659](https://github.com/grafana/alloy/actions/runs/35396946659) | `c9f1806c9` feat(otelcol): Support encoding extensions for otelcol.receiver.awss3 (#7014) |
| 2026-09-21 07:10 | [35571679939](https://github.com/grafana/alloy/actions/runs/35571679939) | `15c698763` chore(loki.process): Implement new stage interface (#6914) |
| 2026-09-21 10:13 | [35587628024](https://github.com/grafana/alloy/actions/runs/35587628024) | `c10b6d5d5` feat(otelcol.receiver.filelog): Add skip_unmodified_files (#7170) |
| 2026-09-21 10:22 | [35588405498](https://github.com/grafana/alloy/actions/runs/35588405498) | `3ecb703f1` feat(otelcol.processor.tail_sampling): Add num_shards (#7144) |
| 2026-09-21 10:30 | [35589102951](https://github.com/grafana/alloy/actions/runs/35589102951) | `857a73f22` feat(otelcol.processor.resourcedetection): Add azureappservice detector (#7150) |
| 2026-09-21 10:31 | [35589174639](https://github.com/grafana/alloy/actions/runs/35589174639) | `077db7da4` feat(otelcol.receiver.kafka): Add partition_processing block (#7163) |
| 2026-09-21 10:34 | [35589421045](https://github.com/grafana/alloy/actions/runs/35589421045) | `20244eb55` feat(otelcol): Add keepalive block to shared HTTP server config (#7166) |
| 2026-09-21 12:19 | [35598850448](https://github.com/grafana/alloy/actions/runs/35598850448) | `5f934a5c1` feat(otelcol.processor.resourcedetection): Expose the retry block (#7141) |
| 2026-09-21 12:48 | [35601677939](https://github.com/grafana/alloy/actions/runs/35601677939) | `08b82b02b` fix(discovery.hetzner)!: Document __meta_hetzner_datacenter (#7169) |

### Issues 1 and 2 — the deterministic failures (39 runs)

Every run in the table above, plus the rest of the 47 that were not disk-space
failures. [35601677939](https://github.com/grafana/alloy/actions/runs/35601677939)
is a good single example showing all three test failures together.

### Issue 5 — mise 504

[run 35609717650, job 106391380975](https://github.com/grafana/alloy/actions/runs/35609717650/job/106391380975)
(PR-triggered, not a `main` push).

---

## 9. Branch and commit state

| Branch | Commit | Contents | Pushed |
|--------|--------|----------|--------|
| `ptodev/windows-ci-fix` | `13f228b29` | Issue 1, CloudWatch docs | yes |
| `ptodev/windows-ci-investigation` (this) | `d8f28326f` | Issue 3, WAL marker fix + test speedup | yes |
| | `54674f14c` | Issue 2, legacy positions | yes |
| | this file | these notes | yes |

Both branches are cut from `main` at `deb039cb4`. The CloudWatch fix is **not**
on this branch — the two branches are independent and neither contains the
other's work.

Per `AGENTS.md`, these are three separate logical changes and should become three
separate PRs. Suggested titles:

```
refactor(cloudwatch_exporter): Remove redundant file mode handling from the docs generator
fix(loki.source.file, loki.source.journal): Reject a non-regular legacy positions file
fix(loki.write): Retry marking a WAL segment when the marker write fails
```

`d8f28326f` bundles the marker bug fix with the `findInterval` test speedup. They
are separable if a reviewer prefers, but the speedup is what makes the new test
fast, so they read naturally together.

There is also a local `git stash` entry (*"Windows fix: reject non-regular legacy
positions file"*) holding the same content as `54674f14c`. It is redundant now
and safe to drop — and note it does **not** travel with the push.

---

## 10. Verification performed

On macOS (darwin/arm64, Go 1.26.7), with `-race` and `-tags="nodocker nonetwork"`:

- `internal/component/common/loki/client/...` passes at `-count=10`.
- `internal/component/common/loki/client/internal/marker/...` passes at
  `-count=25`.
- `internal/component/loki/source/internal/positions/...` and both consumers
  (`loki/source/file`, `loki/source/journal`) pass.
- `internal/static/integrations/cloudwatch_exporter/docs/...` passes (on the
  other branch), and the generator is idempotent against the tracked doc —
  `git status --porcelain` on it is empty, which is the assertion
  `.github/workflows/check-cloudwatch-docs.yml` makes.
- `make lint` exits 0, all six golangci-lint module runs at `0 issues`, plus
  `alloylint` and `shellcheck`.

The new `TestTracker/retries marking a segment after a failed write` subtest was
**mutation-tested**: with the old swallow-the-error code temporarily restored it
fails with *"Condition never satisfied — expected the tracker to retry marking
segment 11"*, and it passes with the fix. So it genuinely pins the bug.

### Not verified

**Nothing has been run on Windows.** Everything above is macOS plus source
reading. The Windows-specific claims that still need confirming:

1. `os.Stat` on a directory reports `IsRegular() == false` on Windows — Issue 2's
   fix depends on this.
2. The sharing-violation mechanism in [§4](#4-issue-3--the-wal-segment-marker-bug-the-real-one).
   The `Access is denied.` log line is real and observed, but the attribution to
   `FILE_SHARE_DELETE` is inferred from the Go and `atomic` sources.
3. Whether the marker subtest still flakes on a Windows runner under whole-repo
   `-race` load. If it does, the retry path needs a closer look rather than a
   longer timeout — raising the timeout was the original wrong turn here, and
   more polling means *more* collisions.

---

## 11. Next steps

1. Run both branches on a Windows VM or via the `os:windows` PR label, which
   gates `test_windows.yml` on PRs.
2. Read [#7112](https://github.com/grafana/alloy/issues/7112) before writing the
   marker PR description — same code, opposite polarity, possible common cause.
3. Split into the three PRs above. Note the Issue 2 behaviour change for
   reviewers.
4. Decide on the disk-exhaustion mitigation ([§6](#6-issue-4--runner-disk-exhaustion-not-addressed)).
5. Apply the mise `env` block ([§7](#7-issue-5--mise-504-on-pr-runs)) across the
   affected workflows.
6. Even with all of the above, expect the tail flakes
   ([§1](#1-summary), cause 5) to keep the job amber. Getting one green run is
   the prerequisite for treating any of them as signal.
