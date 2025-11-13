# Major Dependency Update - 2025-11-13

## Step 1: Establish the latest and current versions of all the major dependencies

| Dependency | Current Version | Latest Version | Status |
|------------|----------------|----------------|--------|
| Prometheus client_golang | v1.23.2 | v1.23.2 | Already latest |
| Prometheus client_model | v0.6.2 | v0.6.2 | Already latest |
| Prometheus common | v0.67.1 | v0.67.2 | Needs update |
| OpenTelemetry Collector Core | v1.44.0 | v1.45.0 | Needs update |
| OpenTelemetry Collector Contrib | v0.138.0 | v0.139.0 | Needs update |
| Prometheus (prometheus/prometheus) | v0.305.1 (forked from v3.7.1) | v3.7.3 | Needs update |
| Beyla | v2.7.4 | v2.7.6 | Needs update |
| Loki | v3.0.0-20251021174646 | v2.9.17 (latest stable) | Using pre-release |
| OBI (go.opentelemetry.io/obi) | v1.2.2 (forked to v1.3.2) | v0.2.0 (upstream) | Forked version |

## Step 2: List the current forks and what changes have been added to them

### Fork: github.com/grafana/prometheus (staleness_disabling_v3.7.3 branch)

**Changes:**
- d73e188 (2025-06-30) - Piotr - Add staleness disabling
- c9e0b31 (2025-10-20) - Piotr - fix: Fix slicelabels corruption when used with proto decoding

**Summary:** This fork adds staleness disabling functionality and fixes a slicelabels corruption bug. The fork is based on Prometheus v3.7.3. We need to continue maintaining this fork as it contains custom functionality required by Alloy.

### Fork: github.com/grafana/opentelemetry-collector/featuregate (feature-gate-registration-error-handler branch)

**Changes:**
- 2fd1623 (2024-03-25) - Piotr Gwizdala - Allow for custom duplicate featuregates handling

**Summary:** This fork adds custom duplicate featuregates handling. We should continue maintaining this fork until upstream accepts the changes.

### Fork: go.opentelemetry.io/obi => github.com/grafana/opentelemetry-ebpf-instrumentation v1.3.2

**Summary:** This is a fork of the OBI package. The versioning scheme differs from upstream (v1.3.2 vs v0.2.0). We need to continue using the fork.

## Step 3: Update the major dependencies in the recommended order

### Update Order:
1. ✅ Prometheus Client Libraries (client_golang, client_model, common)
   - Updated `github.com/prometheus/common` from v0.67.1 to v0.67.2
   - `go mod tidy` completed successfully

2. ✅ OpenTelemetry Collector Core
   - Updated all v1.44.0 packages to v1.45.0
   - Updated all v0.138.0 packages to v0.139.0
   - Updated processor packages (batchprocessor, memorylimiterprocessor) to v0.139.0
   - Updated scraperhelper to v0.139.0
   - Note: opencensusreceiver kept at v0.133.0 (v0.134.0 doesn't exist, package is deprecated)
   - `go mod tidy` completed successfully

3. ✅ OpenTelemetry Collector Contrib
   - Updated most packages from v0.138.0 to v0.139.0
   - Updated some packages from v0.134.0 to v0.135.0
   - Updated some packages from v0.130.0 to v0.131.0
   - `go mod tidy` completed successfully

4. ✅ Prometheus (prometheus/prometheus)
   - Updated from v0.305.1 to v0.307.3 (v3.7.3)
   - Fork (grafana/prometheus staleness_disabling_v3.7.3 branch) is already based on v3.7.3
   - Replace directive remains unchanged as fork is compatible
   - `go mod tidy` completed successfully

5. ✅ Beyla (grafana/beyla/v2)
   - Updated from v2.7.4 to v2.7.6
   - `go mod tidy` completed successfully

6. ⏸️ Loki (grafana/loki/v3)
   - Current: v3.0.0-20251021174646 (pre-release from main branch)
   - Latest stable: v2.9.17
   - Note: Using pre-release version as indicated in go.mod comment. No action needed unless a new v3 release is available.

## Summary

All major dependencies have been successfully updated:

- ✅ **Prometheus Client Libraries**: Updated `common` from v0.67.1 to v0.67.2
- ✅ **OpenTelemetry Collector Core**: Updated from v1.44.0/v0.138.0 to v1.45.0/v0.139.0
- ✅ **OpenTelemetry Collector Contrib**: Updated from v0.138.0 to v0.139.0 (with some packages at v0.135.0)
- ✅ **Prometheus**: Updated from v0.305.1 to v0.307.3 (v3.7.3)
- ✅ **Beyla**: Updated from v2.7.4 to v2.7.6
- ⏸️ **Loki**: No update needed (using pre-release v3.0.0 as intended)

All updates completed successfully with `go mod tidy` passing. The go.mod file is in a consistent state and ready for building and testing.

### Notes:
- The Prometheus fork (grafana/prometheus staleness_disabling_v3.7.3) is already based on v3.7.3 and remains compatible
- The opencensusreceiver package is deprecated and kept at v0.133.0 (v0.134.0 doesn't exist)
- All forks remain in place as they contain necessary customizations
