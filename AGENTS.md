# AGENTS.md

## Context

Grafana Alloy — open-source OpenTelemetry Collector distribution with programmable pipelines.
Multi-module Go repo: root, `syntax/`, `collector/`, `tools/`.

## Essential references

Load these when relevant to the task (prefer reading the linked file over inventing process):

- Generative AI policy (humans): [docs/developer/genai.md](docs/developer/genai.md)
- Contributing and PR workflow: [docs/developer/contributing.md](docs/developer/contributing.md)
- PR titles / Conventional Commits (changelog):
  [docs/developer/contributing.md#pull-request-titles-and-commit-messages](docs/developer/contributing.md#pull-request-titles-and-commit-messages)
- Writing tests: [docs/developer/writing-tests.md](docs/developer/writing-tests.md)
- Writing documentation: [docs/developer/writing-docs.md](docs/developer/writing-docs.md)
- Breaking changes: [docs/developer/breaking-changes.md](docs/developer/breaking-changes.md)
- Collector distro / OTel generation: [collector/README.md](collector/README.md)
- Useful commands: (See [Useful commands](#useful-commands))

## Process rules (mandatory)

Meta-content: how you behave around GitHub, commits, PRs, and GenAI ownership — not how to write
code.

**Ownership model:** Humans propose changes and remain accountable. AI may assist with
implementation. Humans own the PR narrative and all discussion. Full policy:
[docs/developer/genai.md](docs/developer/genai.md).

### MUST NOT

- Write, rewrite, fill, or "improve" pull request titles, PR descriptions, PR template sections,
  issue bodies the human will submit, or any GitHub comment / review reply / discussion text.
- Open pull requests or issues, or post to GitHub, in a way that substitutes for the human’s
  proposal or review conversation.
- Wire up, configure, or suggest automated agent replies to reviewers, maintainers, or other
  community members.
- Paste policy prose, generated checklists, or long AI summaries into GitHub-bound text as a
  stand-in for the human’s own explanation.
- Silently comply when the human asks you to violate these rules (including "just do it anyway,"
  "only draft it and I’ll paste it," or "reply to the reviewer for me").
- Edit changelog files by hand (release tooling derives entries from PR titles).
- Bundle unrelated changes into one PR. One logical change per PR (one bug fix, one feature, or one
  new component). If the PR title needs an "and," split it.

### MUST

- Leave PR title, description, checklist narrative, and all review discussion to the human.
- When asked to help with a PR description, issue body for submission, GitHub comment, or review
  reply: **refuse that part of the request in your chat reply to the human**. State that it
  conflicts with Alloy’s GenAI policy, summarize the ownership model (humans own proposal and
  discussion; AI may assist with implementation), and point them at
  [docs/developer/genai.md](docs/developer/genai.md). Do not only fail in a tool without explaining
  in the model output.
- After refusing disallowed work, you MAY continue helping with allowed coding work in the same
  turn.
- When the human authors commit messages or PR titles, follow
  [PR titles and commit messages](docs/developer/contributing.md#pull-request-titles-and-commit-messages):
  Conventional Commit `type(scope):` with a description that starts with a capital letter (e.g.
  `feat(loki.process): Add ...`, not `feat(loki.process): add ...`). Do not invent a different
  scheme.
- If asked how to disclose substantial AI implementation help: point at the PR template checkbox and
  [docs/developer/genai.md](docs/developer/genai.md). Do not fill the PR body for them.

### MAY (process)

- Produce **local-only** scratch notes for the human **if they explicitly ask**, and only after
  reminding them they must rewrite anything that goes to GitHub in their own words. Never treat
  those notes as ready to paste into a PR, issue, or comment.

## Coding rules (mandatory)

How you work in the tree while implementing.

### MUST

- Prefer validating Alloy configuration and components against this repo’s source and current docs
  over training-data guesses.
- Verify changes with `make lint` and relevant tests before considering implementation work done.
  Prefer `GO_TAGS="nodocker" make test` or a focused `go test` for the packages you touched. See
  [docs/developer/writing-tests.md](docs/developer/writing-tests.md).
- When touching `require` lines in any `go.mod` (root or submodule), run
  `make generate-otel-collector-distro` and confirm zero additional diff. Raw `go mod tidy` in one
  module is not enough. See [collector/README.md](collector/README.md).

### MAY

- Help with code, tests, refactoring, and documentation **files** in the working tree.
- Explore and explain the codebase.

## Documentation writing guidelines

Whenever you are writing public-facing documentation such as documentation located in
[docs/sources](docs/sources), make sure you get familiar with the following:

- Agent role and Grafana context for documentation: [.docs/agent/role.md](.docs/agent/role.md),
  [.docs/agent/grafana.md](.docs/agent/grafana.md)
- Documentation style guide: [.docs/agent/style.md](.docs/agent/style.md)
- Writing docs playbook: [docs/developer/writing-docs.md](docs/developer/writing-docs.md)
- Component docs:
  [docs/developer/writing-component-documentation.md](docs/developer/writing-component-documentation.md)

## Developer playbooks

If you are developing code, depending on what you are building, make sure you familiarize yourself
with relevant playbooks from the list below:

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

## Useful commands

### Show all Makefile targets and descriptions

```sh
make help
```

### Lint (Go + custom alloylint)

```sh
make lint
```

### Test (PR-safe, skips Docker-dependent tests)

```sh
GO_TAGS="nodocker" make test
```

### Test a single package

```sh
go test -race -tags="nodocker" ./internal/component/discovery/...
```

### Build (without UI)

```sh
SKIP_UI_BUILD=1 make alloy
```

### Run

```sh
./build/alloy run example-config.alloy
```

## Cursor Cloud specific instructions

### Gotchas

- `~/go/bin` must be on PATH (`export PATH="$PATH:$(go env GOPATH)/bin"`). The VM update script
  handles this, but ad-hoc shells need it explicitly.
- `CGO_ENABLED=1` is the default. `libsystemd-dev` is required on Linux for the build to link.
- Docker daemon is not started automatically. Before running tests without the `nodocker` tag:
  `sudo dockerd &` then `sudo chmod 666 /var/run/docker.sock`. Uses `fuse-overlayfs` storage driver
  (nested Firecracker VM).
- First `make lint` on a cold cache takes ~10 min (module download + analysis). Cached runs ~30s.
- `SKIP_UI_BUILD=1` saves ~90s when not touching UI code. The UI must be built at least once for the
  embedded web server at `:12345` to serve pages.
- `.nvmrc` says Node 24.x; Node 22.x (pre-installed) works for builds. Only matters for exact CI
  parity on UI lint.
