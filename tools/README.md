# Tools cli

A unified CLI for Alloy's build and release tooling. All subcommands live under
one Go module so they share dependencies and helpers.

Run from the repository root:

```bash
go run -C tools ./cmd <command> [args]
```

## Commands

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
