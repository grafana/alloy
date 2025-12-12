# dependency-changelog

Fetch and print upstream changelogs (typically GitHub release notes) for Alloy key dependencies between two versions.

This tool is intended to be used during the key dependency update process to inspect upstream changes and identify:
- breaking changes (to be documented in Alloy)
- new features relevant to Alloy (to be summarized in the deps update output)

## Usage

```bash
go run -C tools ./dependency-changelog \
  --dep <dependency> \
  --from <version-or-ref> \
  --to <version-or-ref>
```

Where:
- `--dep` can be a key dependency alias (recommended) or a module path.
- `--from` / `--to` are the versions/refs you are updating from/to.
  - For most deps these are tags like `v0.139.0`, `v1.23.2`, `v3.6.2`.
  - For `github.com/prometheus/prometheus`, pass the Go module version (e.g. `v0.308.0`); the tool will map it to the upstream release tag (e.g. `v3.8.0`).
  - For pseudo-versions, the tool will try to extract the commit SHA and fall back to a GitHub compare view.

Common aliases:
- `otelcol` (OpenTelemetry Collector core)
- `otelcol-contrib` (OpenTelemetry Collector contrib)
- `prometheus` (prometheus/prometheus)
- `prom-common` (prometheus/common)
- `prom-client-golang` (prometheus/client_golang)
- `prom-client-model` (prometheus/client_model)
- `beyla` (grafana/beyla)
- `loki` (grafana/loki)
- `obi` (Grafana fork used by Alloy)
- `ebpf-profiler` (Grafana fork used by Alloy)

## Output

The tool prints:
- A short summary (repo + resolved from/to refs)
- Any GitHub releases found in the requested range (tag, date, title, body)
- If releases can’t be determined, it falls back to a commit comparison summary
