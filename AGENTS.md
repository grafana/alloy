# AGENTS.md

## Context

Grafana Alloy — open-source OpenTelemetry Collector distribution with programmable pipelines.
Multi-module Go repo: root, `syntax/`, `collector/`, `extension/alloyengine/`, `tools/`.

## Essential references

- Contributing and PR workflow: [docs/developer/contributing.md](docs/developer/contributing.md)
  - Use the conventional commit formats and PR titles as described in the contributing guide. The description after the `type(scope):` prefix **must start with a capital letter** (e.g. `feat(loki.process): Add ...`, not `feat(loki.process): add ...`) — a CI check enforces this on PR titles, and squash-merge means the PR title is what lands in `main`.
  - Don't edit `CHANGELOG.md` by hand. Release tooling generates entries from PR titles; manual edits conflict or get overwritten.
  - Verify the changes with `make lint` and run relevant tests before opening the PR.
  - When touching `require` lines in any `go.mod` (root or submodule), regenerate the cross-module wiring before pushing — raw `go mod tidy` in one module isn't enough. Run `make generate-module-dependencies` and `make generate-otel-collector-distro`, then confirm zero additional diff. CI's `check` job fails otherwise.

## Documentation writing guidelines

Whenever you are writing public-facing documentation such as documentation located in [docs/sources](docs/sources), make sure you get familiar with the following:

- Agent role and Grafana context for documentation: [.docs/agent/role.md](.docs/agent/role.md), [.docs/agent/grafana.md](.docs/agent/grafana.md)
- Documentation style guide: [.docs/agent/style.md](.docs/agent/style.md)

## Developer playbooks

If you are developing code, depending on what you are building, make sure you get familiar with relevant playbooks from the list below:

- [Handling breaking changes](docs/developer/breaking-changes.md)
- [Shepherding releases](docs/developer/shepherding-releases.md)
- [Managing issues](docs/developer/issue-triage.md)
- [Updating OpenTelemetry dependencies](docs/developer/updating-otel/README.md)
- [Add OpenTelemetry components](docs/developer/add-otel-component.md)
- [Adding community components](docs/developer/adding-community-components.md)
- [Writing tests](docs/developer/writing-tests.md)
- [Writing documentation](docs/developer/writing-docs.md)
- [Write component docs](docs/developer/writing-component-documentation.md)
- [Create exporter components](docs/developer/writing-exporter-components.md)
- [Key dependency updates](docs/developer/key-deps-update/key-dep-updates.md)

## GitHub Actions security

When introducing or modifying new github actions, consider the following security requirements:

### Required rules

1. **Secrets and Credentials:** For GitHub App tokens: use GATB (`grafana/shared-workflows/actions/create-github-app-token`). For all other secrets: fetch from Vault via `get-vault-secrets` + OIDC. Never hardcode secrets or store them in GitHub Secrets.
2. **PR / fork workflow safety:** `pull_request` workflows must be read-only (checkout, lint, test etc). Do not add Vault or GATB to PR workflows that fork contributors can trigger. Fork runs only get `id-token: read`, so OIDC will not issue tokens anyway.
3. **Commit SHAs:** — Pin third-party actions to full commit SHAs, for example `uses: org/action@abcdef… # v1.2.3`
4. **Least-privilege permissions** — Scope the workflow `permissions:` key (what `GITHUB_TOKEN` can do), defaulting to `contents: read`; add `id-token: write` or write scopes only when the job actually needs OIDC/Vault or publishing.
5. **`persist-credentials: false` on actions/checkout** — Unless later steps use git to push or commit (then pass a token to checkout and leave persistence enabled).
6. **Avoid dangerous triggers** — Do not use `pull_request_target`. Use `workflow_run` only when documented and gated; never check out or execute untrusted PR code in those jobs. For `issue_comment` workflows, allow only authorized actors to trigger runs and checkout the commit SHA specified in the comment—not the PR head ref by default.
7. **Separate test and publish jobs** — Jobs that build or test PR code must not have write access to the repo, GitHub Packages, or container registries. Keep image pushes, releases, and other publishing in dedicated workflows with elevated permissions.
8. **Sanitize user input** — Do not interpolate user-controlled values (PR titles, comments, issue bodies, label names, etc.) directly into `run` scripts. Pass them through `env:` and read them as environment variables, or validate and quote them before use, to avoid shell and workflow-command injection.
9. **Limit Actions caching** — To lessen cache poisoning attack vectors: avoid caching in jobs that run untrusted PR code. On release/publish jobs, avoid caching or set `lookup-only: true` on `actions/cache`. Do not reuse cache `key` / `restore-keys` between untrusted builds and privileged workflows.

## Useful commands

Show all Makefile targets and descriptions:

```sh
make help
```

Lint (Go + custom alloylint):

```sh
make lint
```

Test (PR-safe, skips Docker-dependent tests):

```sh
GO_TAGS="nodocker" make test
```

Test a single package:

```sh
go test -race -tags="nodocker" ./internal/component/discovery/...
```

Build (without UI):

```sh
SKIP_UI_BUILD=1 make alloy
```

Run:

```sh
./build/alloy run example-config.alloy
```

## Cursor Cloud specific instructions

### Gotchas

- `~/go/bin` must be on PATH (`export PATH="$PATH:$(go env GOPATH)/bin"`). The VM update script handles this, but ad-hoc shells need it explicitly.
- `CGO_ENABLED=1` is the default. `libsystemd-dev` is required on Linux for the build to link.
- Docker daemon is not started automatically. Before running tests without the `nodocker` tag: `sudo dockerd &` then `sudo chmod 666 /var/run/docker.sock`. Uses `fuse-overlayfs` storage driver (nested Firecracker VM).
- First `make lint` on a cold cache takes ~10 min (module download + analysis). Cached runs ~30s.
- `SKIP_UI_BUILD=1` saves ~90s when not touching UI code. The UI must be built at least once for the embedded web server at `:12345` to serve pages.
- `.nvmrc` says Node 24.x; Node 22.x (pre-installed) works for builds. Only matters for exact CI parity on UI lint.
