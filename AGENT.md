# Agent

This file is the entry point for AI coding assistants in this repository, including Claude, Cursor, and Codex.
It explains what each linked document is for and when to use it.

## Agent behavior and terminology

- [Agent role](.docs/agent/role.md): Defines the expected role and output style for the assistant (experienced engineer + technical writer, practical and implementation-focused).
- [Grafana instructions](.docs/agent/grafana.md): Product and terminology rules for Grafana, OpenTelemetry, Kubernetes, and signal naming.

## Documentation writing

- [Documentation style guide](.docs/agent/style.md): How to structure docs and write clear copy (headings, voice, tense, lists, code blocks, API docs, and shortcodes).

## Code and contribution workflow

- [Contributing guide](docs/developer/contributing.md): Development workflow for code changes (build/test commands, linting, PR title rules, dependency/fork policy, and contribution process).

## Useful developer playbooks

- [Handling breaking changes](docs/developer/breaking-changes.md): How to evaluate, mitigate, communicate, and track user-facing breakage.
- [Shepherding releases](docs/developer/shepherding-releases.md): Release branch, RC, release, and backport process.
- [Managing issues](docs/developer/issue-triage.md): Triage states, labels, and expected maintainer actions for issues.
- [Updating OpenTelemetry dependencies](docs/developer/updating-otel/README.md): Detailed OpenTelemetry dependency update workflow and pitfalls.
- [Add OpenTelemetry components](docs/developer/add-otel-component.md): How to wrap and register upstream OpenTelemetry Collector components in Alloy.
- [Adding community components](docs/developer/adding-community-components.md): Proposal and maintenance process for community-owned components.
- [Writing documentation](docs/developer/writing-docs.md): Documentation organization and writing best practices in this repo.
- [Write component docs](docs/developer/writing-component-documentation.md): Required structure and standards for component reference pages.
- [Create exporter components](docs/developer/writing-exporter-components.md): Implementation guidance for Prometheus exporter components.

## Key dependency updates

- [key-deps-updates.md](docs/developer/key-deps-update/key-dep-updates.md): Step-by-step guide for updating Alloy's key dependencies, including fork checks, module updates, and follow-up build/test investigation.
