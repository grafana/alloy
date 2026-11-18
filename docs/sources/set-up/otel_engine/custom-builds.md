---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/custom-builds/
description: Learn how to build a customized Grafana Alloy binary with only the components you need
menuTitle: Custom builds
review_date: 2026-10-06
title: Build a custom Grafana Alloy binary
weight: 500
---

# Build a custom {{% param "FULL_PRODUCT_NAME" %}} binary

You can build a {{< param "PRODUCT_NAME" >}} binary that includes only the components you need, or that adds components the official release doesn't bundle.
For example, you can build a binary that only collects logs.

{{< admonition type="caution" >}}
Grafana doesn't offer commercial support for custom builds.
{{< /admonition >}}

{{< docs/shared lookup="stability/public_preview_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Before you begin

Make sure you have the following tools:

- Git
- Go {{< param "GO_VERSION" >}} or later
- Node.js
- On Linux, the systemd headers, for example `libsystemd-dev` on Debian-based distributions
- On Windows, a 64-bit [mingw-w64][MinGW] gcc toolchain, such as the [WinLibs][WinLibs] build

## Build a custom binary

Two files in the `collector` directory of the {{< param "PRODUCT_NAME" >}} repository control what goes into the build:

- **[`alloy-components.yaml`][AlloyComponents]:** The {{< param "DEFAULT_ENGINE" >}} components.
- **[`builder-config.yaml`][BuilderConfig]:** The OpenTelemetry Collector components for the {{< param "OTEL_ENGINE" >}}, in the [OpenTelemetry Collector Builder (OCB)][OCB] manifest format.

To build a custom binary:

1. Clone the repository and check out the [release][ReleaseTag] you want to build:

   ```shell
   git clone https://github.com/grafana/alloy.git
   cd alloy
   git checkout <RELEASE_TAG>
   ```

1. In `collector/alloy-components.yaml`, keep only the {{< param "DEFAULT_ENGINE" >}} components you need, and set `converters` to `false`.
   For example, to read log files and send them to Loki:

   ```yaml
   components:
     - local.file_match
     - loki.source.file
     - loki.process
     - loki.write
   converters: false
   ```

1. Optional: In `collector/builder-config.yaml`, delete or add `- gomod:` entries to change the Collector components.

1. Build {{< param "PRODUCT_NAME" >}}:

   ```shell
   make alloy
   ```

   The binary is in `build/alloy`.
   To build a container image instead, run `make alloy-image ALLOY_IMAGE=<REGISTRY>/<IMAGE_NAME>`.

Use the component names from the [components reference](../../../reference/components/).
If a name doesn't exist, the build fails.
The following rules also apply:

- `otelcol.*` components, such as `otelcol.receiver.otlp`, belong in `alloy-components.yaml`, not `builder-config.yaml`.
- The configuration converters depend on most components.
  With `converters: false`, the binary is much smaller, but [`alloy convert`](../../../reference/cli/convert/) and [`--config.format`](../../../reference/cli/run/) values other than `alloy` don't work.

## Check the build

Run `./build/alloy validate <CONFIG_FILE>` against your configuration.
It fails if the configuration uses a component that isn't in the build.

To list the Collector components in the build, run `./build/alloy otel components`.

## Embed the {{% param "DEFAULT_ENGINE" %}} in your own Collector distribution

If you build your own Collector distribution with OCB, you can add the {{< param "DEFAULT_ENGINE" >}} to it with the [`alloyengine` extension](https://github.com/grafana/alloy/blob/main/extension/alloyengine/README.md#include-alloyengine-extension-in-an-ocb-distribution).
The extension always includes every {{< param "DEFAULT_ENGINE" >}} component.

## Next steps

- [Run the {{< param "OTEL_ENGINE" >}}](../) with your custom build.
- Use [`alloy otel`][OTelCommand] to run Collector configuration files.

[AlloyComponents]: https://github.com/grafana/alloy/blob/main/collector/alloy-components.yaml
[BuilderConfig]: https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml
[MinGW]: https://www.mingw-w64.org/
[OCB]: https://opentelemetry.io/docs/collector/custom-collector/
[OTelCommand]: ../../../reference/cli/otel/
[ReleaseTag]: https://github.com/grafana/alloy/releases
[WinLibs]: https://winlibs.com/
