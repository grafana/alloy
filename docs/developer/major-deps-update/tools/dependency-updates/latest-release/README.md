## Overview

Finds the latest release version of a Go module by combining information from two sources:

1. Go module versions from Go dependency management system
2. GitHub releases from the repository

This tool helps identify the most recent version across both sources, which is useful when updating dependencies or checking for discrepancies between Go modules and GitHub releases.

## Usage

`go run main.go <go-module-path>`

**Positional Arguments:**

- `go-module-path` (required): The Go module path (e.g., `github.com/prometheus/common`)

**Flags:**

- `--limit` (optional): Number of recent GitHub releases to fetch (default: 20)

## Output

Displays a comparison of latest versions from both sources:

Example:

```bash
Go Module: github.com/prometheus/common

Latest from Go modules: v0.55.0
Latest from GitHub releases: v0.55.0

All Go module versions (last 10):
  v0.55.0
  v0.54.0
  v0.53.0
  v0.52.3
  v0.52.2
  ...

Recent GitHub releases:
  v0.55.0  2024-06-15  Latest Release
  v0.54.0  2024-05-20  Bug fixes and improvements
  v0.53.0  2024-04-10  New features
  ...
```

The tool will also highlight any discrepancies between the two sources.
