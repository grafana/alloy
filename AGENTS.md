# AGENTS.md

## Context

Grafana Alloy — open-source OpenTelemetry Collector distribution with programmable pipelines.
Multi-module Go repo: root, `syntax/`, `collector/`, `extension/alloyengine/`, `tools/`.

## Essential references

- Contributing and PR workflow: [docs/developer/contributing.md](docs/developer/contributing.md)
  - Specifically, make sure you use the conventional commit formats and PR titles as described in the contributing guide.
  - Verify the changes with `make lint` and run relevant tests before opening the PR.
  - Note that we have some code generation steps that need to be run when the generated output is changed. See the contributing guide and Makefile help commands for more details.

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
- [Writing documentation](docs/developer/writing-docs.md)
- [Write component docs](docs/developer/writing-component-documentation.md)
- [Create exporter components](docs/developer/writing-exporter-components.md)
- [Key dependency updates](docs/developer/key-deps-update/key-dep-updates.md)

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
