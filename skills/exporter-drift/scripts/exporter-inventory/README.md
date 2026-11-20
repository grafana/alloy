## Overview

Lists every `prometheus.exporter.*` component with the exporter module it embeds, what Alloy actually builds from (after `replace`), whether that's a fork and of what, the latest upstream release, and how far the pin is behind upstream's default branch. It decides which exporters need an audit and writes a brief for each one.

It reads `go.mod` and the component code locally, and calls the GitHub API through `gh` (repo metadata, latest release, compare, branches-where-head). It doesn't clone anything.

The component-to-module mapping is a table in `main.go` (`exporterModules`). The tool reports a mapping problem when a component directory has no entry, or when neither the component nor its static integration imports the mapped module. Fix those before trusting the rest of the output.

## Usage

Run from the root of an Alloy checkout:

```bash
go run ./skills/exporter-drift/scripts/exporter-inventory [flags]
```

**Flags:**

- `--repo` (optional): Path to the root of an Alloy checkout (default: `.`)
- `--exporter` (optional): Comma-separated component names to include, such as `kafka,unix` (default: all)

`gh` must be installed and authenticated.

## Output

Markdown on stdout with these sections:

- **Mapping problems**, only when there are any.
- **Summary**: one row per component.
- **Briefs**: one per exporter that needs an audit. A brief lists the audit reason, the Alloy code paths, the go.mod require and replace (with the replace's comment), the pinned commit, the upstream and fork repos (flagging archived repos and stale fork parents), whether the pin is the head of a fork branch, the latest release, and the merge base and commit counts against upstream's default branch.
- **Current, no audit needed**: components that are up to date.

An exporter needs an audit when it's a fork, when it's behind upstream's latest release, when it's pinned to a commit that's older than the latest release or behind the default branch, or when a lookup failed.

Commit counts come from the GitHub compare API and include merge commits. Upstreams that tag without publishing GitHub releases are compared by commits only.

Example:

```
| Component | Builds from | Fork of | Latest upstream release | Upstream commits not in pin | Audit |
|---|---|---|---|---|---|
| kafka | `github.com/grafana/kafka_exporter v0.0.0-20261016145052-44e4679ca78b` | danielqsj/kafka_exporter | v1.10.0 (2026-09-08) | 105 | **yes**: fork |
| process | `github.com/ncabatoff/process-exporter v0.8.7` | - | v0.8.7 (2025-04-21) | 0 | no |

### kafka

- Audit reason: fork
- Alloy code: `internal/component/prometheus/exporter/kafka`, `internal/static/integrations/kafka_exporter`
- go.mod require: `github.com/grafana/kafka_exporter v0.0.0-20261016145052-44e4679ca78b`
- Pinned commit: `44e4679ca78b` (2026-10-16)
- Upstream: danielqsj/kafka_exporter (default branch `master`, last push 2026-10-19)
- Fork: grafana/kafka_exporter (default branch `master`, last push 2026-10-16)
- The pin is the head of fork branch(es): `master`
- Latest upstream release: v1.10.0 (2026-09-08)
- Against upstream `master`: merge base `2e60c05` (2024-03-15), 29 commits only in the pin, 105 only upstream (counts include merges)
```
