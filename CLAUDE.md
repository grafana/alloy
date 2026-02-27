# Claude

## Documentation

@.docs/agent/role.md
@.docs/agent/grafana.md
@.docs/agent/style.md

## Pull Requests

### Title

Use Conventional Commits format: `type(scope): description`

Common types: `feat`, `fix`, `chore`, `ci`, `docs`

Scope is optional — use the component name in dot-notation when relevant (e.g. `loki.write`, `otelcol.processor.tail_sampling`).

### Description

For `feat` and significant `fix` PRs, use the PR template. For `docs`, `ci`, and `chore` PRs, a freeform paragraph is the norm.

Include links to CI failures, upstream PRs, or screenshots when they help explain the change.

## Before Opening a PR

When changing Go code, run the following before pushing:

```sh
make lint test
```

If you changed generated files, regenerate them:

- `dependency-replacements.yaml`: `make generate-module-dependencies`
- `collector/`: `make generate-otel-collector-distro`
- `operations/helm`: `make docs rebuild-tests` from `operations/helm`

Rebase on `main` before opening the PR to resolve any merge conflicts.
