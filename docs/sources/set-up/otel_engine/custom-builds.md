---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/custom-builds/
description: Learn how to build a customized Grafana Alloy binary with the OpenTelemetry Collector Builder
menuTitle: Custom builds
review_date: 2026-10-06
title: Build a custom Grafana Alloy binary with the OpenTelemetry Collector Builder (OCB)
weight: 500
---

# Build a custom {{% param "FULL_PRODUCT_NAME" %}} binary with the OpenTelemetry Collector Builder (OCB)

Grafana builds {{< param "PRODUCT_NAME" >}} from a declarative [OpenTelemetry Collector Builder (OCB)][OCB] manifest, [`collector/builder-config.yaml`][BuilderConfig], and a list of native components, [`collector/alloy-components.yaml`][AlloyComponents].
If you need additional components or want to remove bundled components, edit the manifest and build a customized {{< param "PRODUCT_NAME" >}} binary.
For example, you can build a {{< param "PRODUCT_NAME" >}} binary that only collects logs.

Two files control what a build includes:

- **`builder-config.yaml`:** The `receivers`, `processors`, `exporters`, `connectors`, and `extensions` sections select the Collector components available to the {{< param "OTEL_ENGINE" >}}.
- **`alloy-components.yaml`:** This file selects the native {{< param "PRODUCT_NAME" >}} components available to the {{< param "DEFAULT_ENGINE" >}}, and whether the build can convert configuration files from other formats.
  OCB doesn't read this file.

{{< admonition type="caution" >}}
Grafana doesn't offer commercial support for custom builds.
{{< /admonition >}}

{{< docs/shared lookup="stability/public_preview_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Before you begin

Make sure you have the following tools:

- Git
- Go {{< param "GO_VERSION" >}} or later
- Node.js
- Docker, if you want to build the container image

The build compiles C dependencies, so you also need the following for your platform:

- On Linux, the systemd headers.
  On Debian-based distributions, run `sudo apt-get install libsystemd-dev`.
- On Windows, a 64-bit [mingw-w64][MinGW] gcc toolchain.
  Grafana recommends the [WinLibs][WinLibs] build.

## Clone the repository

Clone the {{< param "PRODUCT_NAME" >}} repository and change to the repository root.
The following steps assume you run commands from this directory.

```shell
git clone https://github.com/grafana/alloy.git
cd alloy
```

To build from a specific release, fetch tags and check out the tag after you clone:

```shell
git fetch --tags
git checkout <RELEASE_TAG>
```

Replace _`<RELEASE_TAG>`_ with the [release tag][ReleaseTag] you want.

## Select OpenTelemetry Collector components

The OCB sections of the manifest list the Collector components that the {{< param "OTEL_ENGINE" >}} can use.

- **Remove a component:** Delete its `- gomod: ...` line from the appropriate section.
- **Add a component:** Append a `- gomod:` line that points at the module path and version you want.
  Follow the same pattern as the other entries.

## Select native components

The `components` field of `collector/alloy-components.yaml` lists the native {{< param "PRODUCT_NAME" >}} components to include in the build.
Set it to `all` to include every component, or to a list of component names.

The following example includes only the components needed to read log files and send them to Loki:

```yaml
components: [local.file_match, loki.source.file, loki.process, loki.write]
```

Use the component names listed in the [components reference](../../../reference/components/).
The build includes the whole package that registers each component you list, and one package can register several components.
For example, listing `discovery.ec2` also includes `discovery.aws` and `discovery.lightsail`.
The list selects packages, so it's not always an exact allowlist of components.
If the list contains a name that doesn't exist, the build fails and lists the components in the same namespace, for example all `loki.*` components.

If you omit the `components` field, the build includes every component.

The native `otelcol.*` components are separate from the Collector components in the OCB sections.
For example, the `otlpreceiver` entry in the `receivers` section doesn't make `otelcol.receiver.otlp` available to the {{< param "DEFAULT_ENGINE" >}}.
List `otelcol.receiver.otlp` in the `components` field to include it.

{{< param "PRODUCT_NAME" >}} rejects configurations that use components which aren't included in the build.
For example, `alloy run` and `alloy validate` report that they can't find the definition of the component.

## Remove configuration conversion

The `converters` field of `collector/alloy-components.yaml` controls whether the build can convert configuration files from other formats, such as Prometheus, Promtail, and OpenTelemetry Collector configuration files.
It defaults to `true`.

The converters depend on most native components.
If you include only some components, set `converters` to `false` to reduce the binary size:

```yaml
components: [local.file_match, loki.source.file, loki.process, loki.write]
converters: false
```

A build without converters returns an error when you run [`alloy convert`](../../../reference/cli/convert/) or start {{< param "PRODUCT_NAME" >}} with a [`--config.format`](../../../reference/cli/run/) other than `alloy`.

## Build without the OpenTelemetry Engine

If you only use the {{< param "DEFAULT_ENGINE" >}}, you can leave the {{< param "OTEL_ENGINE" >}} out of the build.
To do this, remove the `receivers`, `processors`, `exporters`, `connectors`, and `extensions` sections from `collector/builder-config.yaml`.

The following `builder-config.yaml` and `alloy-components.yaml` build a {{< param "PRODUCT_NAME" >}} binary that only collects logs.

In `collector/builder-config.yaml`:

```yaml
dist:
  module: github.com/grafana/alloy/otel_engine
  name: alloy
  description: Alloy OTel Collector distribution.
  version: <ALLOY_VERSION>
  output_path: .

providers:
  # Keep the providers from the original manifest.

excludes:
  # Keep the excludes from the original manifest.

replaces:
  # Keep the replace directives from the original manifest.
```

Keep the `dist`, `providers`, `excludes`, and `replaces` sections from the original `collector/builder-config.yaml`.
_`<ALLOY_VERSION>`_ is the version already set in the `dist` section of the original manifest.

In `collector/alloy-components.yaml`:

```yaml
components: [local.file_match, loki.source.file, loki.process, loki.write]
converters: false
```

A build without the {{< param "OTEL_ENGINE" >}} doesn't include the [`alloy otel`](../../../reference/cli/otel/) and [`alloy otel-supervisor`](../../../reference/cli/otel-supervisor/) commands, or any Collector components.

## Build the binary

After you edit `collector/builder-config.yaml` and `collector/alloy-components.yaml`, build {{< param "PRODUCT_NAME" >}}:

```shell
make alloy
```

This command regenerates the collector distribution and the native component registration from the manifest before it builds, so your manifest edits take effect.
The binary in `build/` behaves like a standard `alloy` build.
To skip the UI build when the UI assets already exist, run `SKIP_UI_BUILD=1 make alloy`.
To strip debug information from the binary like the official releases do, set `RELEASE_BUILD=1`.

If your build includes the {{< param "OTEL_ENGINE" >}}, confirm that your components are in the build:

```shell
./build/alloy otel components
```

The command lists each bundled component with its module path, version, and stability level.
Use [`alloy otel`][OTelCommand] to run collector YAML against your custom bundle.

## Build a Docker image

To build a Docker image from your custom manifest, run the following command:

```shell
make alloy-image ALLOY_IMAGE=<REGISTRY>/<IMAGE_NAME>
```

Replace the following:

- _`<REGISTRY>`_: Your container registry.
- _`<IMAGE_NAME>`_: Your image name and tag, for example `alloy-logs:v1`.

If you don't set `ALLOY_IMAGE`, the build names the image `grafana/alloy:latest`.

## Embed the Alloy Engine extension in your own Collector distribution

You can also embed the {{< param "DEFAULT_ENGINE" >}} in your own OCB distribution with the `alloyengine` extension.
Refer to the [`alloyengine` extension README](https://github.com/grafana/alloy/blob/main/extension/alloyengine/README.md#include-alloyengine-extension-in-an-ocb-distribution) for instructions.

The extension always includes every native {{< param "PRODUCT_NAME" >}} component and the converters.
OCB doesn't read `alloy-components.yaml`, so you can't select native components in your own distribution.
To select native components, build {{< param "PRODUCT_NAME" >}} from its repository as described on this page.

## Next steps

- [Run the {{< param "OTEL_ENGINE" >}}](../) with your custom build.
- [Components reference](../../../reference/components/) for the names of native components.
- [OpenTelemetry Collector Builder documentation](https://opentelemetry.io/docs/collector/custom-collector/) for the OCB manifest format.

[AlloyComponents]: https://github.com/grafana/alloy/blob/main/collector/alloy-components.yaml
[BuilderConfig]: https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml
[MinGW]: https://www.mingw-w64.org/
[OCB]: https://opentelemetry.io/docs/collector/custom-collector/
[OTelCommand]: ../../../reference/cli/otel/
[ReleaseTag]: https://github.com/grafana/alloy/releases
[WinLibs]: https://winlibs.com/
