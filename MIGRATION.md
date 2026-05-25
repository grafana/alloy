# Mise migration progress

Tracks the migration from `make` to `mise` as the project's task runner / toolchain manager. The `Makefile` is still authoritative; this work is additive until parity is reached.

## Done

### Tooling
- [x] Tools pinned in `mise.toml`: `go`, `node`, `shellcheck`, `golangci-lint`
- [x] `[env]` defaults with shell-overridable templates (e.g. `CGO_ENABLED`)

### Build
- [x] `build` — composite: frontend + binary
- [x] `build:binary` — Go build with `--release`, `--tags`, `--output` flags; ldflags via `_build/ldflags`
- [x] `build:frontend` — vite build, cached via `sources`/`outputs`
- [x] `install:frontend` — `npm ci`, cached on lockfile

### Lint
- [x] `lint` — composite (scripts + go + alloy + frontend)
- [x] `lint:scripts` — `shellcheck` over project shell scripts (matches CI scope)
- [x] `lint:go` — `golangci-lint` on every `go.mod` (excluding testdata)
- [x] `lint:alloy` — custom `alloylint` analyzer via `go run`
- [x] `lint:frontend` — `npm run lint` + `npm run format`

### Test
- [x] `test` — composite (currently just `test:go`)
- [x] `test:go` — `go test -race` across every `go.mod` (excluding testdata)

## In progress

(nothing actively in flight)

## Todo

| Bucket | Makefile targets |
|---|---|
| Tests | `test-pyroscope` (Pyroscope-specific subset) |
| Integration tests | `integration-test-docker`, `integration-test-k8s`, `integration-test-windows-service` |
| Code generation | `generate-graphql`, `generate-snmp`, `generate-module-dependencies`, `generate-otel-collector-distro`, `generate-winmanifest` |
| Helm artifacts | `generate-helm-docs`, `generate-helm-tests`, `generate-rendered-mixin` |
| Docker images | `alloy-image`, `alloy-image-windows` |
| Release / dist | `dist-alloy-binaries`, `dist-alloy-packages`, `dist-alloy-installer`, `dist-alloy-mixin-zip` |
| Utility | `clean`, `info`, `help` |

## Intentionally not migrated

- `USE_CONTAINER=1` — `mise [tools]` replaces the use case (pinned tool versions without a build container). System libs (`libsystemd-dev` for cgo) remain a user concern.
- `SKIP_UI_BUILD=1` — replaced by per-task caching on `build:frontend`. Dockerfile will need an update when it migrates.

## Known gotchas

- **`#MISE sources=[...]` in script comments doesn't engage caching.** `sources`/`outputs` must live on the TOML task entry, even for `file =`-style tasks.
- **Glob syntax:** `**/*` for recursive file matching, not bare `**`.
- **`lint:alloy` + `install:frontend` race:** `go run` with `./...` walks into `internal/web/ui/node_modules/` while `npm ci` is wiping it. Workaround: `mise run --jobs 1 lint`. Real fix would be making alloylint skip `node_modules` dirs internally.
- **Makefile `make lint` has a latent bug:** `xargs -I __dir__ golangci-lint ...` never substitutes `__dir__`, so it lints the root module N times instead of each module once. Our `lint:go` does the right thing by cd-ing into each module.

## File layout

```
mise.toml                       # task + tool registry
mise-tasks/
  _build/                       # build scripts (hidden auto-discovery; canonical names live in mise.toml)
    build                       # used by both `build` and `build:binary`
    ldflags                     # prints -ldflags string used by binary build
  _lint/
    scripts                     # shellcheck driver
    go                          # golangci-lint multi-module driver
MIGRATION.md                    # this file
```
