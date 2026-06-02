# Tools cli

A unified CLI for Alloy's build and release tooling. All subcommands live under
one Go module so they share dependencies and helpers.

Run from the repository root:

```bash
go run -C tools ./cmd <command> [args]
```

## Commands

### `aireview`

Analyzes a PR diff with OpenAI and posts the result as a PR comment.

**GitHub mode** — fetch diff from a PR and post a comment:

```bash
go run -C tools ./cmd aireview \
  --slug="owner/repo" \
  --pr-number=123 \
  --prompt-file=".github/ai-review-prompts/dependency-review.md" \
  --marker="<!-- ai-review -->"
```

**GitHub mode, no comment** — fetch diff but print to stdout:

```bash
go run -C tools ./cmd aireview \
  --slug="owner/repo" \
  --pr-number=123 \
  --prompt-file=".github/ai-review-prompts/dependency-review.md" \
  --no-comment
```

**Stdin mode** — pipe a diff in and print the result:

```bash
git diff main | go run -C tools ./cmd aireview \
  --prompt-file=".github/ai-review-prompts/dependency-review.md"
```

Requires `OPENAI_API_KEY`. GitHub mode additionally requires `GITHUB_TOKEN`.

Each workflow that uses `aireview` should pass a unique `--marker` (e.g.
`<!-- ai-deps -->`, `<!-- ai-security -->`) so its comments stay separate from
other AI-review bots on the same PR. See
`.github/workflows/ai-dependency-review.yml` for a working example.

### `release`

Four subcommands that automate the release flow. Each is driven by a GitHub
Actions workflow and supports `--dry-run` so you can verify what would happen
without making changes.

**`release create-release-branch --tag=<v1.X.0>`**

Creates the `release/v1.X` branch from a finalized release tag and ensures the
matching `backport/v1.X` label exists. Idempotent — re-running on an existing
branch is a no-op. Driven by `release-create-branch.yml` (fires on `v*.*.0`
tag pushes).

**`release create-rc --branch=main|release/v1.X`**

Tags the next RC and creates a draft prerelease from the open release-please
PR for the given branch. Use `main` for a new minor RC, `release/v1.X` for a
patch RC. Driven by `release-create-rc.yml` (manual `workflow_dispatch`).

**`release backport --pr=<N> --label=backport/v1.X`**

Cherry-picks the commit from a merged PR onto the `release/v1.X` branch, pushes
the backport branch, and opens a backport PR. Skips cleanly when the target
branch doesn't exist yet (release still in RC) or when the backport is already
merged. On failure it comments on the source PR with manual instructions.
Driven by `release-backport.yml` after the trigger workflow.

**`release enrich-release-notes --tag=<v1.X.Y> [--footer=<path>]`**

Adds contributor attribution (`@user1, @user2`) to each changelog entry in a
published release's body, and optionally appends a footer template (with
`${RELEASE_DOC_TAG}` substituted to `vX.Y`). Component release tags (those
with a `/` like `syntax/v0.1.2`) skip the footer. Driven by
`release-enrich-release-notes.yml` (fires on release publish).

### `generate module-dependencies`

Keeps Go module `replace` directives consistent across the repository from a
single source of truth (`dependency-replacements.yaml` at the repo root).

The tool reads the replacements, renders them through a template, and injects
the rendered block into each target file between marker comments:

```
BEGIN GENERATED REPLACES - DO NOT EDIT MANUALLY ... END GENERATED REPLACES
```

**Do not edit anything between the markers** — update
`dependency-replacements.yaml` instead; the next run overwrites manual changes.
Local `replace` directives (pointing a dependency at a local path) belong
outside the markers in the individual `go.mod` files, not in
`dependency-replacements.yaml`.

Supported `file_type` values in the config:

- `mod` — Go module files (`go.mod`); `go mod tidy` runs after the update.
- `ocb` — OpenTelemetry Collector Builder config YAML files.

Run via the Make wrapper (preferred):

```bash
make generate-module-dependencies
```

Or invoke the CLI directly:

```bash
go run -C tools ./cmd generate module-dependencies \
  --dependency-yaml="$PWD/dependency-replacements.yaml"
```

`--root` defaults to the git repo root, so it works from any subdirectory; pass
`--root=<path>` to override. `--dependency-yaml` is resolved relative to the
binary's working directory — `go run -C tools` runs from `tools/`, so use an
absolute path or one relative to `tools/`.

CI checks that the generated output matches what's committed. If you change
`dependency-replacements.yaml`, run the Make target and commit the resulting
diff.

### `goversion`

Bumps the Go toolchain version across the repository. Split into two steps that
are intended to land as separate PRs.

```bash
# PR 1: bump Go in the build images.
go run -C tools ./cmd goversion pr-1 <version>

# PR 2: bump Go in go.mod files, Dockerfiles, and the build image pin.
go run -C tools ./cmd goversion pr-2 <version>
```

### `govulncheck`

Runs `govulncheck` across every `go.mod` module in the repo and fails when reachable, non-ignored findings remain.
The command resolves the repository root via `git`, so it works from any subdirectory in the repo, and reads ignore rules from `.govulncheck.yaml` by default.

### `lint go`

Runs `golangci-lint` across every `go.mod` module in the repo. Every module is
linted regardless of earlier failures, and the command exits non-zero if any
module reports findings. The repository root is resolved via `git`, so it works
from any subdirectory.

Run via the Make wrapper (preferred):

```bash
make lint-go
```

Or invoke the CLI directly:

```bash
go run -C tools ./cmd lint go
```

Pass `--binary=<path>` to use a specific `golangci-lint` binary (defaults to
`golangci-lint` on `PATH`), and `--root=<path>` to override the repository root.

### `lint shell`

Runs `shellcheck` over the repo's shell scripts. It discovers candidate files by
extension (`.sh`, `.bash`, or no extension) and then keeps only those whose first
line is a `sh`/`bash` shebang, so extension-less scripts are covered. The
repository root is resolved via `git`, so it works from any subdirectory, and the
command exits non-zero if `shellcheck` reports findings.

```bash
go run -C tools ./cmd lint shell
```

Pass `--root=<path>` to override the repository root. Requires `shellcheck` on
`PATH`.

## Adding a new command

1. Create a package under `tools/<area>/` (e.g. `tools/release/`).
2. Export a `Command() *cobra.Command` that returns the area's root command,
   wiring in any subcommands.
3. Register it in `tools/cmd/main.go`.

Shared helpers go in `tools/internal/`.
