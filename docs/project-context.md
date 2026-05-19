# Project context (local)

<!-- Copied from docs-ai/skills/context-templates/project-context.template.md -->
<!-- Location: docs/project-context.md -->

## Identity

- **Product name (short):** Alloy
- **Product name (first mention in prose):** Grafana Alloy
- **GitHub org/repo:** grafana/alloy

## Branches and releases

- **Default development branch:** `main`
- **Release branch pattern:** `release-X.Y`
- **Docs version mapping:** Doc site version maps directly to product version (e.g. `latest` → current release, versioned URLs like `/alloy/v1.16/`)

## Documentation paths

- **Documentation root (filesystem):** `docs/sources/`
- **Generated pages (do not hand-edit):** Check individual component directories for auto-generated reference pages
- **Configuration reference index:** `docs/sources/reference/components/`
- **Changelog:** `CHANGELOG.md` at repo root
- **Architecture / "start here" page:** `docs/sources/introduction/`

## Code ↔ documentation mapping

| Code area             | Documentation area                                        |
| --------------------- | --------------------------------------------------------- |
| `internal/component/` | `docs/sources/reference/components/`                      |
| `syntax/`             | `docs/sources/reference/` (config-blocks, stdlib)         |
| `collector/`          | `docs/sources/introduction/otel_alloy/` (OTel components) |
| `extension/`          | `docs/sources/reference/`                                 |

## Code validation paths

Paths the agent should check when validating documentation claims against code.

| What to validate                           | Where to look                          |
| ------------------------------------------ | -------------------------------------- |
| Component configuration structs / defaults | `internal/component/`                  |
| Syntax / expression language               | `syntax/`                              |
| CLI flags and options                      | `internal/cmd/` or root-level Go files |
| OTel collector components                  | `collector/`                           |

## Frontmatter and site conventions

- Uses Grafana Writers' Toolkit conventions: https://grafana.com/docs/writers-toolkit/
- Topic types: Introduction, Get started, Concepts, Tutorials, Tasks, Reference, Release notes
- Internal link style: relative paths, no trailing slash
- Front matter variables defined in `docs/sources/_index.md` cascade (e.g. `ALLOY_RELEASE`, `PRODUCT_NAME`, `FULL_PRODUCT_NAME`)
- Use `{{< param "PRODUCT_NAME" >}}` and `{{< param "FULL_PRODUCT_NAME" >}}` shortcodes rather than hardcoding product names

## Conventions for agents

- **Product naming:** Use `{{< param "PRODUCT_NAME" >}}` (renders as "Alloy") and `{{< param "FULL_PRODUCT_NAME" >}}` (renders as "Grafana Alloy") in docs; first mention on a page uses full name
- **Configuration language:** Called "Alloy configuration syntax" or just "Alloy syntax" — not HCL or River
- **Vale linter config:** `.vale.ini` at repo root; style `Grafana`
- **Component naming convention:** `namespace.component_name` format (e.g. `prometheus.scrape`, `otelcol.receiver.otlp`)

## Subsystem knowledge

- `AGENTS.md` at repo root covers overall agent context, build commands, and gotchas
- `docs/developer/writing-docs.md` — documentation organisation and topic type guidelines
- `docs/developer/writing-component-documentation.md` — reference page structure for components
- `.docs/agent/role.md`, `.docs/agent/grafana.md`, `.docs/agent/style.md` — agent role and style context

## Optional: shared features across sub-products

Alloy is part of the Grafana observability stack. Documentation changes that touch integrations with the following may require coordinated updates:
- Grafana Loki (log collection components)
- Grafana Mimir / Prometheus (metrics components)
- Grafana Tempo (trace components)
- Grafana Pyroscope (profiling components)
- OpenTelemetry Collector (OTel components under `otelcol.*`)
