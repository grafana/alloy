## Overview

Checks the upstream stability shown in every `otelcol` component doc against the `status.stability` block in the upstream component's `metadata.yaml`, at the version Alloy's `go.mod` selects. It also checks that each component's Alloy stage matches the upstream level, using development/alpha → experimental, beta → public preview, stable → generally available, and the highest upstream level among the signals the doc lists.

Components in `allowlist.txt` are grandfathered: their Alloy stage is allowed to differ from upstream as long as upstream stays at the level recorded there.

## Usage

```bash
go run main.go [--repo <path to Alloy root>]
```

**Flags:**

- `--repo` (optional): Path to the Alloy repository root. Defaults to the git top-level of the current directory.

Run it after `go.mod` is updated, so it reads the new upstream versions. The first run may download modules.

## Output

A Markdown report grouped by what needs doing. Empty groups are left out. Each item names the component and, where it applies, the module and version it read, and the exact line to add.

```
## Docs disagree with upstream (fix the docs line) (1)

- otelcol.exporter.loadbalancing (github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter@v0.165.0): docs say "beta for traces and logs, alpha for metrics", upstream says "beta for traces, metrics, and logs"

## Grandfathered component whose upstream level moved (needs a decision) (1)

- otelcol.connector.spanmetrics: allowlisted as general-availability/alpha, now general-availability/beta (Alloy/upstream). Upstream moved, so review the Alloy stage.
```

Groups:

- **Docs disagree with upstream**: update the docs line to what upstream says.
- **Upstream deprecated or unmaintained**: user-facing, so it needs a release note.
- **Alloy stage doesn't match upstream**: needs a decision from a maintainer. Promote the Alloy stage in a follow-up, or add the printed allowlist line with approval. Never downgrade a GA component.
- **Grandfathered component whose upstream level moved**: review the Alloy stage, then update or remove the allowlist entry.
- **Missing upstream stability line**: add the printed line, dropping any signal the Alloy component doesn't expose.
- **Can't resolve upstream component**: the doc's upstream link is missing or points at something without `metadata.yaml`. Fix the link.
- **Stale allowlist entries**: remove them.
- **Upstream signals not listed in the docs**: confirm the Alloy component doesn't expose them.
- **Skipped**: community components, which have no stage label.

When nothing needs doing, it prints `All otelcol docs match upstream stability.`
