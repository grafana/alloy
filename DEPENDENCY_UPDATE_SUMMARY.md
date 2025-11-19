# Major Dependency Update - Completed ✅

**Date**: 2025-11-19  
**Branch**: `cursor/update-major-project-dependencies-a935`  
**Status**: ✅ **All updates completed successfully**

## Summary

Successfully updated all major dependencies in the Alloy project to their latest versions. The project builds without errors and the binary runs successfully.

## Key Updates

### OpenTelemetry Collector
- **Core modules** (no version in path): `v0.137.0` → `v1.46.0`
  - client, component, confmap, consumer, exporter, extension, featuregate, pdata, pipeline, processor, receiver
- **Versioned modules**: `v0.137.0` → `v0.140.0`
  - service, otelcol, component/componentstatus, config/configgrpc, config/confighttp, config/configtelemetry, connector
- **Contrib modules**: Mixed versions → `v0.140.1`
  - All connectors, exporters, extensions, processors, and receivers
  - Notably: `loadbalancingexporter` updated from `v0.138.0`

### Grafana Components
- **Loki**: `v3.1.1` → `v3.6.0`
- **Loki Push**: Updated pseudo-version to `v0.0.0-20251117203452-bc9cd7639972` (v3.6.0)
- **Beyla**: `v1.8.4` → `v1.10.0`

### Prometheus
- **prometheus/prometheus**: Still using Grafana fork at commit `13a97bf5b7cf` (v2.56.0 equivalent)
- **prometheus/common**: `v0.55.0` → `v0.67.3`
- **prometheus/client_golang**: `v1.20.3` → `v1.20.5`

### eBPF Components
- **OBI (eBPF Instrumentation)**: Pseudo-version → `v1.3.8` (now using release tag)
  - Replace: `github.com/grafana/opentelemetry-ebpf-instrumentation@v1.3.8`
- **eBPF Profiler**: Changed to thampiotr fork for v0.140 compatibility
  - Replace: `github.com/thampiotr/opentelemetry-ebpf-profiler@v0.0.0-20251119140801-fe6dbb9e62bc`
  - Fork includes OTel Collector v0.140.x pdata API compatibility + pyroscope packages

### Kubernetes
- **controller-runtime**: `v0.22.0` → `v0.22.4`

## Forks Removed

### go.opentelemetry.io/collector/featuregate
**Reason**: Upstream Prometheus issue #13842 has been fixed, eliminating the need for the custom fork.

**Changes made**:
1. Removed the replace directive from `go.mod`
2. Deleted `internal/util/otelfeaturegatefix/featuregate_override.go`
3. Removed imports from:
   - `internal/component/all/all.go`
   - `internal/static/traces/config.go`
   - `internal/static/logs/logs.go`
   - `internal/static/integrations/v2/integrations.go`

## Active Forks

| Module | Fork | Reason |
|--------|------|--------|
| `github.com/prometheus/prometheus` | `grafana/prometheus@13a97bf5b7cf` | Alloy-specific features (staleness_disabling) |
| `go.opentelemetry.io/obi` | `grafana/opentelemetry-ebpf-instrumentation@v1.3.8` | eBPF instrumentation support |
| `go.opentelemetry.io/ebpf-profiler` | `thampiotr/opentelemetry-ebpf-profiler@fe6dbb9e62bc` | OTel v0.140 compatibility + pyroscope packages |
| `github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor` | `grafana/opentelemetry-collector-contrib/processor/k8sattributesprocessor@73458b01ab23` | Custom k8s attributes processing |

## Build Verification

✅ **All components build successfully**:
- `internal/runtime`
- `internal/component/prometheus`
- `internal/component/loki`
- `internal/component/otelcol`
- `internal/component/beyla`
- `internal/component/pyroscope` (including eBPF components)
- `internal/converter`
- Full project build (`make alloy`)

✅ **Binary verification**:
```
alloy, version v1.12.0-devel+dirty (branch: cursor/update-major-project-dependencies-a935, revision: 745e2cc44)
  build user:       ubuntu@cursor
  build date:       2025-11-19T14:12:40Z
  go version:       go1.25.1
  platform:         linux/amd64
  tags:             slicelabels
```

## Issues Resolved

### 1. eBPF Profiler API Incompatibility
**Problem**: The Grafana fork of `opentelemetry-ebpf-profiler` was incompatible with OTel Collector v0.140.x pdata API changes:
- Missing pyroscope packages
- `profile.Sample` and `loc.Line` API changes

**Solution**: Switched to `thampiotr/opentelemetry-ebpf-profiler` fork at commit `fe6dbb9e62bc` which includes:
- Full OTel v0.140.x compatibility
- All required pyroscope packages restored
- Race condition fixes in process manager

### 2. Duplicate Feature Gate Registration
**Problem**: Duplicate feature gate registrations causing panics when both Prometheus and OTel Collector register the same gates.

**Solution**: Upstream Prometheus issue #13842 has been resolved, allowing removal of the custom featuregate fork and associated workaround code.

### 3. Loki Push Pseudo-version
**Problem**: Initial pseudo-version for `loki/pkg/push` had incorrect timestamp, causing "invalid pseudo-version" errors.

**Solution**: Retrieved correct commit SHA (`bc9cd7639972`) for Loki v3.6.0 and used proper timestamp format: `v0.0.0-20251117203452-bc9cd7639972`

## Files Modified

- `go.mod` - Updated all dependency versions and replace directives
- `go.sum` - Updated checksums for new versions
- `internal/util/otelfeaturegatefix/featuregate_override.go` - **Deleted**
- `internal/component/all/all.go` - Removed otelfeaturegatefix import
- `internal/static/traces/config.go` - Removed otelfeaturegatefix import
- `internal/static/logs/logs.go` - Removed otelfeaturegatefix import
- `internal/static/integrations/v2/integrations.go` - Removed otelfeaturegatefix import

## Documentation

Full detailed update process documented in: `deps-update-2025-11-19.md`

## Next Steps

This update is ready for:
1. Testing in development environment
2. Running full test suite
3. Integration testing with downstream components
4. Code review and merge to main branch

---

**Note**: All changes have been committed to the `cursor/update-major-project-dependencies-a935` branch.
