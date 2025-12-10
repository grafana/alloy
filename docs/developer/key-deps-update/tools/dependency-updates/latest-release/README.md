## Overview

Finds the latest version of a Go module by querying the most authoritative source for that dependency. Automatically selects between GitHub releases, Git tags, or Go modules based on the dependency type. Handles special cases like Prometheus-style versioning (v3.8.0 ↔ v0.308.0) and Grafana forks that use Git tags.

## Usage

```bash
go run main.go [flags] <module_path>
```

**Positional Arguments:**

- `module_path` (required): The Go module path (e.g., `github.com/prometheus/prometheus`)

**Flags:**

- `--limit N`: Number of recent releases/tags to fetch (default: 20)

## Output

Shows the lookup method used, latest version, and recent version history. For Prometheus, displays both GitHub release format (v3.8.0) and Go module format (v0.308.0).

**Example:**

```
Go Module: github.com/prometheus/prometheus

Lookup method: GitHub releases
Latest version:  v3.8.0 (Go module: v0.308.0)

Recent versions (last 10, newest first):
  v3.8.0          2025-12-02   (Go: v0.308.0    ) Latest - 3.8.0 / 2025-11-28
  v3.7.3          2025-10-30   (Go: v0.307.3    ) 3.7.3 / 2025-10-29
  v3.7.2          2025-10-22   (Go: v0.307.2    ) 3.7.2 / 2025-10-22
  ...
```
