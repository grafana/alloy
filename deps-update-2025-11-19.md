# Major Dependency Update – 2025-11-19

## Step 2 – Version Snapshot
| Dependency | Current Version | Latest Release | Update? |
| --- | --- | --- | --- |
| OpenTelemetry Collector Core (`go.opentelemetry.io/collector/*`) | Mix of `v1.45.0` + `v0.139.0` | `v1.46.0` / `v0.140.0` | ⚠️ |
| OpenTelemetry Collector Contrib (`github.com/open-telemetry/opentelemetry-collector-contrib/...`) | `v0.139.0` (with a few stuck at `v0.138.0`/`v0.130.0`) | `v0.140.1` | ⚠️ |
| Prometheus Server (`github.com/prometheus/prometheus`) | `github.com/grafana/prometheus` `staleness_disabling_v3.7.3` (base `v3.7.3` / module `v0.307.3`) | Upstream `v3.7.3` (`v0.307.3`) | ✅ |
| Prometheus Common | `v0.67.1` | `v0.67.3` | ⚠️ |
| Prometheus Client Golang | `v1.23.2` | `v1.23.2` | ✅ |
| Prometheus Client Model | `v0.6.2` | `v0.6.2` | ✅ |
| Grafana Beyla (`github.com/grafana/beyla/v2`) | `v2.7.6` | `v2.7.6` | ✅ |
| Grafana Loki (`github.com/grafana/loki/v3`) | `v3.0.0-20251021174646-053429db2124` (main commit) | `v3.6.0` | ⚠️ |
| OBI (`go.opentelemetry.io/obi` ⇒ `grafana/opentelemetry-ebpf-instrumentation`) | `v1.3.7` | `v1.3.8` | ⚠️ |
| eBPF Profiler (`go.opentelemetry.io/ebpf-profiler` ⇒ `grafana/opentelemetry-ebpf-profiler`) | `v0.0.202546-0.20251106085643-a00a0ef2a84c` | Fork head `a00a0ef` (post-`v0.0.202545`, upstream `v0.0.202547`) | ✅ |

## Step 3 – Fork Review

### go.opentelemetry.io/collector/featuregate ⇒ grafana/opentelemetry-collector (`feature-gate-registration-error-handler`) — ✅
- **Base / Deltas:** Branch diverges from upstream `main` at `116c1812` (Mar 2024, ~v1.45). Single commit `2fd1623` (“Allow for custom duplicate featuregates handling”) introduces a configurable duplicate registration handler (`SetAlreadyRegisteredErrHandler`) touching `featuregate/registry.go` and tests.
- **Why:** Alloy installs a handler via `internal/util/otelfeaturegatefix/featuregate_override.go` to prevent Prometheus scrape library from panicking when shared components register feature gates twice (see Grafana Alloy issue #249 referenced in Prom issue #14049).
- **Upstream status:** No upstream issue or PR adding this hook yet; diff vs `v1.46.0` still missing the handler, so the fork remains necessary.
- **Next step:** The existing branch can continue to satisfy Alloy while we update to OTel `v1.46.0`; once practical, cut a refreshed tag (e.g., `feature-gate-registration-error-handler-v1.46.0`) so the fork stays close to upstream.

### github.com/prometheus/prometheus ⇒ grafana/prometheus (`staleness_disabling_v3.7.3`) — ✅
- **Base / Deltas:** Branch tracks upstream `v3.7.3` (`v0.307.3`) with two extra commits: `d73e188` (“Add staleness disabling”) and `c9e0b31` (“Fix slicelabels corruption when used with proto decoding”).
- **Why:** Enables per-target disabling of end-of-run staleness markers for Alloy’s sharded scrape use case (fixes gaps described in upstream issue [#14049](https://github.com/prometheus/prometheus/issues/14049)).
- **Upstream status:** Upstream PR [#17431](https://github.com/prometheus/prometheus/pull/17431) merged on 2025-11-04 but is not yet part of a released tarball; we must keep the fork until a release ≥v3.7.4 ships with that patch.
- **Next step:** When upstream tags the next release containing #17431, cut `staleness_disabling_v<new version>` carrying only the slicelabel hotfix if still needed.

### go.opentelemetry.io/obi ⇒ grafana/opentelemetry-ebpf-instrumentation — ✅
- **Base / Deltas:** Grafana-maintained fork is ahead of upstream; latest tag `v1.3.8` adds commits `be9aeb6…92f3924` (PR [#29](https://github.com/grafana/opentelemetry-ebpf-instrumentation/pull/29)) that vendor the SDK name, sync missing metrics, and stabilize multi-node tests.
- **Status:** Latest fork release exists and should be consumed (`v1.3.8`); no upstream equivalent yet, so fork is ready once we bump.

### go.opentelemetry.io/ebpf-profiler ⇒ grafana/opentelemetry-ebpf-profiler — ✅
- **Base / Deltas:** Currently pinned to commit `a00a0ef` (post-tag pseudo-version) that merges Grafana PR [#36](https://github.com/grafana/opentelemetry-ebpf-profiler/pull/36), fixing a race in `processmanager`’s resource release logic.
- **Why:** Upstream PR [open-telemetry/opentelemetry-ebpf-profiler#899](https://github.com/open-telemetry/opentelemetry-ebpf-profiler/pull/899) carrying the same fix was closed (code being rewritten), so we need the fork for Alloy releases.
- **Status:** Alloy already consumes the fork head with the race fix, so we can proceed; for bookkeeping we should cut a tag (e.g., `v0.0.202546`) or rebase onto upstream `v0.0.202547` when available, but it is not blocking the dependency update.

## Step 4 – Go Module Updates
- Bumped the OpenTelemetry Collector core modules from `v1.45.0/v0.139.0` to `v1.46.0/v0.140.0`, and the contrib components to `v0.140.1`. The legacy `opencensusreceiver` no longer ships after `v0.133.0`, so it remains pinned with an inline note.
- Upgraded Prometheus core dependency to `v0.307.3` (release `v3.7.3`) while continuing to route it through the `staleness_disabling_v3.7.3` Grafana fork.
- Prometheus common moved to `v0.67.3`; client_golang/client_model were already current.
- Updated `go.opentelemetry.io/obi` replace to the latest fork tag `v1.3.8`.
- Adopted the dedicated `thampiotr/opentelemetry-ebpf-profiler` fork (`alloy-fork-v0.140`, pseudo-version `v0.0.0-20251119140801-fe6dbb9e62bc`), which bundles both the Grafana race fix and the new `pprofile` API updates (`Samples()`/`Lines()`). go.mod now replaces `go.opentelemetry.io/ebpf-profiler` with that fork, so no vendored copy is needed.
- Tidied modules (`go mod tidy`) after the upgrades; go.mod now carries only two `require` blocks (direct and indirect), with the former reorganized alphabetically during the update.

## Step 5 – go.mod Organization
- Collapsed the third indirect `require` block into the main indirect block so that go.mod now follows the requested structure: one direct block, one indirect block, followed by replace/exclude sections.
- Documented special cases inline (e.g., opencensusreceiver removal, new profiler fork replace) to make future upgrades easier.

## Step 6 – Build & Validation
- `make alloy` initially failed because the Grafana eBPF profiler fork still relied on the pre-`v0.140` `pprofile` API (`Profile.Sample`, `Location.Line`). The local fork now mirrors upstream’s `Samples()` / `Lines()` helpers, resolving those build breaks.
- After the patch, `make alloy` completes successfully with the new dependency graph. No additional compilation issues surfaced in the prioritized subsystems.
