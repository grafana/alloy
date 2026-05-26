# tooling

A unified CLI for Alloy's build and release tooling. All subcommands live under
one Go module so they share dependencies and helpers.

Run from the repository root:

```bash
go run -C tools ./cmd <command> [args]
```

## Commands

### `ai-review`

Analyzes a PR diff with OpenAI and posts the result as a PR comment.

**GitHub mode** — fetch diff from a PR and post a comment:

```bash
go run -C tools ./cmd ai-review \
  --slug="owner/repo" \
  --pr-number=123 \
  --prompt-file=".github/ai-review-prompts/dependency-review.md" \
  --marker="<!-- ai-review -->"
```

**GitHub mode, no comment** — fetch diff but print to stdout:

```bash
go run -C tools ./cmd ai-review \
  --slug="owner/repo" \
  --pr-number=123 \
  --prompt-file=".github/ai-review-prompts/dependency-review.md" \
  --no-comment
```

**Stdin mode** — pipe a diff in and print the result:

```bash
git diff main | go run -C tools ./cmd ai-review \
  --prompt-file=".github/ai-review-prompts/dependency-review.md"
```

Requires `OPENAI_API_KEY`. GitHub mode additionally requires `GITHUB_TOKEN`.

Each workflow that uses `ai-review` should pass a unique `--marker` (e.g.
`<!-- ai-deps -->`, `<!-- ai-security -->`) so its comments stay separate from
other AI-review bots on the same PR. See
`.github/workflows/ai-dependency-review.yml` for a working example.

### `generate module-dependencies`

Keeps Go module `replace` directives consistent across the repository from a
single source of truth (`dependency-replacements.yaml` at the repo root).

The tool reads the replacements, renders them through a template, and injects
the rendered block into each target file between marker comments:

```
BEGIN GENERATED REPLACES - DO NOT EDIT MANUALLY ... END GENERATED REPLACES
```

**Do not edit anything between these markers** — update
`dependency-replacements.yaml` instead. Anything changed manually inside the
markers gets overwritten on the next run. Local `replace` directives (pointing
a dependency at a local path) are *not* meant to live in
`dependency-replacements.yaml`; keep those outside the markers in the
individual `go.mod` files.

Supported `file_type` values in the config:

- `mod` — Go module files (`go.mod`); `go mod tidy` is run afterwards.
- `ocb` — OpenTelemetry Collector Builder config YAML files.

Run via the wrapper Make target:

```bash
make generate-module-dependencies
```

Or invoke the CLI directly (what the Make target does):

```bash
go run -C tools ./cmd generate module-dependencies \
  --dependency-yaml="$PWD/dependency-replacements.yaml" \
  --project-root="$PWD"
```

(Paths must be absolute or be relative to `tools/`, because `go run -C tools`
changes the working directory before running the binary.)

CI checks that the generated output matches what's committed. If you change
`dependency-replacements.yaml`, run the Make target and commit the resulting
diff.

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

### `update-go-version`

Bumps the Go toolchain version across the repository. Split into two steps that
are intended to land as separate PRs.

```bash
# PR 1: bump Go in the build images.
go run -C tools ./cmd update-go-version pr-1 <version>

# PR 2: bump Go in go.mod files, Dockerfiles, and the build image pin.
go run -C tools ./cmd update-go-version pr-2 <version>
```

## Adding a new command

1. Create a package under `tools/<area>/` (e.g. `tools/release/`).
2. Export a `Command() *cobra.Command` that returns the area's root command,
   wiring in any subcommands.
3. Register it in `tools/cmd/main.go`.

Shared helpers go in `tools/internal/`.
