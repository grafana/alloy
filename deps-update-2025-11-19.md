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
