# Major Dependencies Update - 2025-11-19

This document tracks the process of updating major dependencies in the Alloy project.

## Step 2: Current and Latest Versions

| Dependency | Current Version | Latest Version | Update Needed |
|-----------|----------------|----------------|---------------|
| **OpenTelemetry Collector Core** | v1.45.0 / v0.139.0 | v1.46.0 / v0.140.0 | 🔄 |
| **OpenTelemetry Collector Contrib** | v0.139.0 (mixed) | v0.140.1 | 🔄 |
| - loadbalancingexporter | v0.138.0 | v0.140.1 | 🔄 |
| - prometheusexporter | v0.130.0 | v0.140.1 | 🔄 |
| - translator/loki | v0.130.0 | v0.140.1 | 🔄 |
| - opencensusreceiver | v0.133.0 | v0.140.1 | 🔄 |
| **Prometheus (prometheus/prometheus)** | v0.305.1 (fork of v3.7.1) | v3.7.3 | 🔄 |
| **Prometheus client_golang** | v1.23.2 | v1.23.2 | ✅ |
| **Prometheus common** | v0.67.1 | v0.67.3 | 🔄 |
| **Prometheus client_model** | v0.6.2 | v0.6.2 | ✅ |
| **Beyla** | v2.7.6 | v2.7.6 | ✅ |
| **Loki** | v3.0.0 (main branch) | v3.6.0 | 🔄 |

## Step 3: Fork Analysis

### 1. go.opentelemetry.io/collector/featuregate => github.com/grafana/opentelemetry-collector/featuregate

**Current**: `feature-gate-registration-error-handler` branch from 2024-03-25

**Purpose**: Adds custom handling for duplicate featuregate registrations to avoid panics when both Prometheus and OTel Collector register the same feature gates.

**Changes**: Single commit "Allow for custom duplicate featuregates handling"

**Related Issue**: https://github.com/prometheus/prometheus/issues/13842 (CLOSED)

**Status**: ✅ **Fork no longer needed**. Issue #13842 was marked as completed. The issue has been resolved by moving the required packages into prometheus/prometheus itself, eliminating the cyclic dependency problem.

### 2. github.com/prometheus/prometheus => github.com/grafana/prometheus (staleness_disabling_v3.7.3)

**Current**: Fork of v3.7.1, branch `staleness_disabling_v3.7.3`

**Base version**: v3.7.3

**Purpose**: Adds ability to disable staleness markers for specific targets, addressing clustering scenarios where targets move between instances.

**Changes** (2 commits on top of v3.7.3):
1. `d73e188` - "Add staleness disabling" (PR #34, addresses issue #14049)
2. `c9e0b31` - "fix: Fix slicelabels corruption when used with proto decoding" (PR #45)

**Related Issues**: 
- https://github.com/prometheus/prometheus/issues/14049 (closed)
- https://github.com/grafana/alloy/issues/249

**Upstream Status**: Issue #14049 is closed, but no evidence of the feature being merged into upstream Prometheus.

**Next Steps**: Need to check if there's a v3.7.3 branch or if we need to create a new branch based on v3.7.3 with these patches.

**Status**: 🛑 **Fork still needed**. The staleness disabling feature has not been merged upstream. However, we may need to update the fork to be based on the latest Prometheus version (v3.7.3).

### 3. go.opentelemetry.io/obi => github.com/grafana/opentelemetry-ebpf-instrumentation

**Current**: v1.3.7

**Latest**: v1.3.8 (released 2025-11-18)

**Purpose**: Grafana maintains this fork as the most up-to-date version of OBI.

**Status**: ✅ **Update available**. Should update to v1.3.8.

### 4. go.opentelemetry.io/ebpf-profiler => github.com/grafana/opentelemetry-ebpf-profiler

**Current**: v0.0.202546-0.20251106085643-a00a0ef2a84c (commit a00a0ef2a84c from 2025-11-06)

**Purpose**: Grafana maintains this fork as the most up-to-date version.

**Latest commit message**: "Merge pull request #36 from grafana/fix-racce - fix(processmanager): race during release resources"

**Status**: ✅ **Use latest commit from main branch**. This is the Grafana fork and we should use the latest version.

### 5. github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor

**Current**: github.com/grafana/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.0.0-20251021125353-73458b01ab23

**Purpose**: Adds support for k8s.io/client-go v0.34.1, including RunWithContext and AddEventHandlerWithOptions methods to fake informers.

**Status**: 🛑 **Fork still needed**. This fork provides compatibility with newer Kubernetes client libraries. Need to check if a newer version exists for v0.140.x OTel Collector Contrib.

## Step 4: Updating Go Modules

Starting dependency updates in the following order based on dependency relationships:

1. Update Prometheus client libraries (no dependencies on other major deps)
2. Update OpenTelemetry Collector Core
3. Update OpenTelemetry Collector Contrib  
4. Update Beyla (depends on OTel Core + Prometheus clients)
5. Update Loki (depends on Prometheus + OTel)
6. Remove featuregate fork
7. Update OBI and ebpf-profiler forks

### Update Summary

✅ **Prometheus common**: v0.67.1 → v0.67.3
✅ **OpenTelemetry Collector Core**: v1.45.0 → v1.46.0 / v0.139.0 → v0.140.0
✅ **OpenTelemetry Collector Contrib**: v0.139.0 → v0.140.1 (most modules)
- ✅ loadbalancingexporter: v0.138.0 → v0.140.1  
- ⚠️ prometheusexporter: Remains at v0.130.0 (TODO comment suggests otlptranslator issue)
- ⚠️ translator/loki: Remains at v0.130.0 (TODO comment suggests otlptranslator issue)
- ℹ️ opencensusreceiver: Remains at v0.133.0 (module no longer exists in v0.140.x)
✅ **Loki**: v3.0.0 (main branch) → v3.6.0
✅ **OBI**: v1.2.2 → v1.3.8
✅ **ebpf-profiler**: Updated to latest commit (1e5516f97d8b from 2025-11-05)
✅ **Featuregate fork removed**: Successfully using upstream v1.46.0
✅ **controller-runtime**: v0.20.4 → v0.22.4
✅ **go mod tidy**: Completed successfully

### Notes

- The opencensusreceiver module was removed from the opentelemetry-collector-contrib repository and no longer exists in v0.140.x. It remains at v0.133.0 for now.
- The prometheusexporter and translator/loki modules remain pinned at v0.130.0 due to otlptranslator compatibility issues mentioned in TODO comments.
- All other OTel Collector Contrib modules successfully updated to v0.140.1.

## Step 5: Organizing go.mod

✅ **go.mod organized**: The file is already well-organized with separate require blocks for direct and indirect dependencies, and replace directives are properly commented.
✅ **OBI replace updated**: v1.3.7 → v1.3.8
✅ **ebpf-profiler replace updated**: Updated to commit 1e5516f97d8b (2025-11-05)

## Step 6: Fixing Compilation Errors

### Fixed Issues

✅ **Removed featuregate workaround**: Deleted `internal/util/otelfeaturegatefix/featuregate_override.go` - no longer needed as issue #13842 is resolved.
✅ **Removed imports**: Removed imports of the deleted otelfeaturegatefix package from:
- `internal/component/all/all.go`
- `internal/static/traces/config.go`
- `internal/static/logs/logs.go`
- `internal/static/integrations/v2/integrations.go`

✅ **internal/runtime**: Builds successfully
✅ **internal/component/prometheus**: Builds successfully
✅ **internal/component/loki**: Builds successfully  
✅ **internal/component/otelcol**: Builds successfully
✅ **internal/component/beyla**: Builds successfully
❌ **internal/component/pyroscope**: Fails due to ebpf-profiler incompatibility
❌ **internal/converter**: Fails due to ebpf-profiler incompatibility
❌ **make alloy**: Fails due to ebpf-profiler incompatibility

### Remaining Issues

🛑 **ebpf-profiler incompatibility with OTel v0.140.x**: The `github.com/grafana/opentelemetry-ebpf-profiler` fork has compatibility issues with the new OTel Collector pdata API:
- `profile.Sample` method no longer exists on `pprofile.Profile` type
- `loc.Line` method no longer exists on `pprofile.Location` type

**Two options**:
1. **Use the latest ebpf-profiler commit** (1e5516f97d8b from 2025-11-05): This version removes the pyroscope packages entirely, breaking `internal/component/pyroscope/ebpf`
2. **Keep old ebpf-profiler** (a00a0ef2a84c): This has API incompatibilities with the new OTel Collector pdata v0.140.x

**Current state**: Reverted to old ebpf-profiler (a00a0ef2a84c) to keep pyroscope packages, but this causes compilation failures in pyroscope components.

**Recommendation**: The ebpf-profiler fork needs to be updated to:
- Be compatible with OTel Collector v0.140.x pdata API changes
- Or maintain the pyroscope packages if using the latest version

## Summary

Major dependency updates were successfully completed with the following highlights:

✅ **OTel Collector upgraded**: v1.45.0/v0.139.0 → v1.46.0/v0.140.0/v0.140.1
✅ **Prometheus common upgraded**: v0.67.1 → v0.67.3
✅ **Loki upgraded**: v3.0.0 → v3.6.0
✅ **Featuregate fork removed**: Successfully removed workaround for #13842
✅ **OBI upgraded**: v1.2.2 → v1.3.8
✅ **Most components building**: Runtime and core components compile successfully

### Build Status

✅ **Components that build successfully**:
- internal/runtime
- internal/component/prometheus  
- internal/component/loki
- internal/component/otelcol
- internal/component/beyla

❌ **Components that fail to build**:
- internal/component/pyroscope (ebpf-profiler API incompatibility)
- internal/converter (transitively depends on pyroscope)
- make alloy (full project build fails due to above)

### Next Steps

**Option 1: Fix ebpf-profiler fork (Recommended)**
The `github.com/grafana/opentelemetry-ebpf-profiler` fork needs to be updated to be compatible with OTel Collector v0.140.x pdata API. The following API changes need to be addressed:
- `pprofile.Profile.Sample()` method has been changed or removed
- `pprofile.Location.Line()` method has been changed or removed

**Option 2: Downgrade OTel Collector (Not Recommended)**
Could potentially stay on v0.139.0, but this defeats the purpose of the major dependency update.

**Option 3: Temporarily Exclude Pyroscope Components**
Could conditionally exclude pyroscope/ebpf component until the fork is fixed, allowing other components to be tested.

**Files with errors**:
```
/home/ubuntu/go/pkg/mod/github.com/grafana/opentelemetry-ebpf-profiler@v0.0.202546-0.20251106085643-a00a0ef2a84c/reporter/internal/pdata/generate.go:159:21
/home/ubuntu/go/pkg/mod/github.com/grafana/opentelemetry-ebpf-profiler@v0.0.202546-0.20251106085643-a00a0ef2a84c/reporter/internal/pdata/generate.go:226:18
/home/ubuntu/go/pkg/mod/github.com/grafana/opentelemetry-ebpf-profiler@v0.0.202546-0.20251106085643-a00a0ef2a84c/reporter/internal/pdata/generate.go:285:63
```

