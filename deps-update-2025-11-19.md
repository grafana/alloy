# Major Dependency Update - 2025-11-19

## Step 1: Tools Familiarization

Tools to be used:
- `gh release list` - Finding latest releases on GitHub
- `go list -m -versions` - Finding latest releases via Go package manager
- `go mod download` and `go mod graph` - Viewing dependencies
- `gh api repos/.../compare` - Comparing versions and forks
- `gh pr view` and `gh issue view` - Getting PR/issue details
- `diff` - Comparing code changes between versions

## Step 2: Current and Latest Versions

| Dependency | Current Version | Latest Version | Update Needed |
|------------|----------------|----------------|---------------|
| OpenTelemetry Collector Core | v1.45.0/v0.139.0 | v1.46.0/v0.140.0 | ✅ |
| OpenTelemetry Collector Contrib | v0.139.0 | v0.140.1 | ✅ |
| Prometheus | v3.7.1 (fork) | v3.7.3 | ✅ |
| Prometheus client_golang | v1.23.2 | v1.23.2 | ❌ |
| Prometheus client_model | v0.6.2 | v0.6.2 | ❌ |
| Prometheus common | v0.67.1 | v0.67.3 | ✅ |
| Beyla | v2.7.6 | v2.7.6 | ❌ |
| Loki | commit 053429db2124 (Oct 21) | v3.6.0 | ✅ |
| OBI (grafana fork) | v1.3.7 | v1.3.8 | ✅ |
| ebpf-profiler (grafana fork) | commit a00a0ef | commit a00a0ef | ❌ |

## Step 3: Fork Status

### Prometheus Fork

**Fork:** `github.com/grafana/prometheus` branch `staleness_disabling_v3.7.3`
**Base version:** v3.7.3
**Current fork commit:** c9e0b31e9aeb (Oct 30, 2025)
**Changes in fork:**
- Commit d73e188: "Add staleness disabling" - Adds ability to disable end-of-run staleness markers
- Commit c9e0b31: "fix: Fix slicelabels corruption when used with proto decoding"

**Upstream issue:** https://github.com/prometheus/prometheus/issues/14049 (CLOSED on 2025-11-04)
**Upstream PR:** https://github.com/prometheus/prometheus/pull/17431 (MERGED on 2025-11-04)

**Status:** ✅ The fork exists for v3.7.3. PR #17431 was merged upstream but is NOT included in v3.7.3 release (merged after release). The fork is still needed and is ready to use with v3.7.3 base.

### OBI Fork

**Fork:** `github.com/grafana/opentelemetry-ebpf-instrumentation`
**Current version:** v1.3.7
**Latest version:** v1.3.8 (released 2025-11-18)

**Status:** ✅ Latest version v1.3.8 is available and should be used.

### ebpf-profiler Fork

**Fork:** `github.com/grafana/opentelemetry-ebpf-profiler`
**Current commit:** a00a0ef (Nov 6, 2025)
**Latest commit:** a00a0ef (same)

**Status:** ✅ Using latest commit. No update needed.

### Other Forks

The following forks are not major dependencies and will be kept unchanged:
- `github.com/grafana/cadvisor` - cadvisor fork
- `github.com/grafana/postgres_exporter` - postgres_exporter fork
- `github.com/grafana/mysqld_exporter` - mysqld_exporter fork
- `github.com/grafana/node_exporter` - node_exporter fork
- `github.com/grafana/opentelemetry-collector/featuregate` - featuregate fork
- Various Loki-related forks

**Note:** `exporter/loadbalancingexporter` is currently pinned to v0.138.0 due to issue #43950. PR #43960 was merged on 2025-11-06 and is included in v0.140.0+, so we can upgrade it.

## Step 4: Update Go Modules to Desired Versions

Target versions:
- OpenTelemetry Collector Core: v1.46.0/v0.140.0
- OpenTelemetry Collector Contrib: v0.140.1 (most packages), v0.140.0 for loadbalancingexporter (fix included)
- Prometheus: v3.7.3 (via fork staleness_disabling_v3.7.3)
- Prometheus common: v0.67.3
- Loki: v3.6.0
- OBI: v1.3.8
- ebpf-profiler: keep current commit

**Update Status:** ✅ Successfully updated go.mod. All major dependencies updated to target versions. `go mod tidy` completed successfully.

## Step 5: Organize go.mod

The go.mod file is already well-organized with:
- Module declaration and Go version at the top
- Direct dependencies in the first `require()` block
- Indirect dependencies in subsequent `require()` blocks
- Replace directives at the bottom with comments

No reorganization needed. ✅

## Step 6: Fix Compilation Errors

### Issue Found: ebpf-profiler API Compatibility

**Error:** The `github.com/grafana/opentelemetry-ebpf-profiler` fork uses the old pprofile API that was changed in OTel v0.140.0:
- `profile.Sample()` → `profile.Samples()` (returns SampleSlice)
- `loc.Line()` → `loc.Lines()` (returns LineSlice)

**Fix Applied:** Updated the fork code locally to use the new API:
- Line 159: `profile.Sample().AppendEmpty()` → `profile.Samples().AppendEmpty()`
- Line 226: `loc.Line().AppendEmpty()` → `loc.Lines().AppendEmpty()`
- Line 285: `profile.Sample().Len()` → `profile.Samples().Len()`

**Status:** ✅ Build successful with local fix. The fork at `github.com/grafana/opentelemetry-ebpf-profiler` needs to be updated with this fix. Currently using a local replace directive pointing to `/tmp/ebpf-profiler-fix`.

**Next Steps:** 
1. Create a PR to the grafana/opentelemetry-ebpf-profiler fork with this fix
2. Once merged, update the replace directive to use the new commit
3. Remove the local path replace directive
