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
