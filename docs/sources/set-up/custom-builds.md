---
canonical: https://grafana.com/docs/alloy/latest/set-up/custom-builds/
description: Learn how to build a custom Grafana Alloy binary that includes only the components you need
menuTitle: Custom builds
title: Build a custom Grafana Alloy binary
weight: 400
---

# Build a custom {{% param "FULL_PRODUCT_NAME" %}} binary

You can build {{< param "PRODUCT_NAME" >}} from source with only the components you need.
A custom build can add OpenTelemetry Collector components that {{< param "PRODUCT_NAME" >}} doesn't bundle, or leave out components you don't use to reduce the binary size.
For example, you can build a {{< param "PRODUCT_NAME" >}} binary that only collects logs.

Grafana doesn't offer commercial support for custom builds.

A single [OpenTelemetry Collector Builder (OCB)](https://opentelemetry.io/docs/collector/custom-collector/) manifest, [`collector/builder-config.yaml`](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml), controls what a build includes:

- **OCB sections:** The `receivers`, `processors`, `exporters`, `connectors`, and `extensions` sections select the Collector components available to the {{< param "OTEL_ENGINE" >}}.
- **The `alloy` section:** This section selects the native {{< param "PRODUCT_NAME" >}} components available to the {{< param "DEFAULT_ENGINE" >}}, and whether the build can convert configuration files from other formats.
  OCB ignores this section.

## Before you begin

Before you begin, ensure you have the following:

- Git.
- The Go version listed in the {{< param "PRODUCT_NAME" >}} repository's [`go.mod`](https://github.com/grafana/alloy/blob/main/go.mod) file.
- Make.
- A C compiler.
  The {{< param "PRODUCT_NAME" >}} build calls C code through Go's `cgo` feature by default.
  On Linux, you also need the system development libraries, such as `libsystemd-dev`.
- Docker, if you want to build a Docker image.

## Clone the repository

Clone the {{< param "PRODUCT_NAME" >}} repository and change to the repository root.
The rest of this page assumes you run commands from this directory.

```shell
git clone https://github.com/grafana/alloy.git
cd alloy
```

To build from a specific release, fetch the tags and check out the release tag:

```shell
git fetch --tags
git checkout <RELEASE_TAG>
```

Replace _`<RELEASE_TAG>`_ with the [release tag](https://github.com/grafana/alloy/releases) you want.

## Select OpenTelemetry Collector components

The OCB sections of `collector/builder-config.yaml` list the Collector components that the {{< param "OTEL_ENGINE" >}} can use.

- **Remove a component:** Delete its `- gomod: ...` line from the appropriate section.
- **Add a component:** Append a `- gomod:` line that points at the module path and version you want.
  Follow the same pattern as the other entries.

## Select native components

The `components` field of the `alloy` section lists the native {{< param "PRODUCT_NAME" >}} components to include in the build.
Set it to `all` to include every component, or to a list of component names.

The following example includes only the components needed to read log files and send them to Loki:

```yaml
alloy:
  components: [local.file_match, loki.source.file, loki.process, loki.write]
```

Use the component names listed in the [components reference](../../reference/components/).
If the list contains a name that doesn't exist, the build fails and lists the components in the same namespace, for example all `loki.*` components.

If you omit the `components` field, the build includes every component.

The native `otelcol.*` components are separate from the Collector components in the OCB sections.
For example, the `otlpreceiver` entry in the `receivers` section doesn't make `otelcol.receiver.otlp` available to the {{< param "DEFAULT_ENGINE" >}}.
List `otelcol.receiver.otlp` in the `components` field to include it.

{{< param "PRODUCT_NAME" >}} rejects configurations that use components which aren't included in the build.
For example, `alloy run` and `alloy validate` report that they can't find the definition of the component.

## Remove configuration conversion

The `converters` field of the `alloy` section controls whether the build can convert configuration files from other formats, such as Prometheus, Promtail, and OpenTelemetry Collector configuration files.
It defaults to `true`.

The converters depend on most native components.
If you include only some components, set `converters` to `false` to reduce the binary size:

```yaml
alloy:
  components: [local.file_match, loki.source.file, loki.process, loki.write]
  converters: false
```

A build without converters returns an error when you run [`alloy convert`](../../reference/cli/convert/) or start {{< param "PRODUCT_NAME" >}} with a [`--config.format`](../../reference/cli/run/) other than `alloy`.

## Build without the OpenTelemetry Engine

If you only use the {{< param "DEFAULT_ENGINE" >}}, you can leave the {{< param "OTEL_ENGINE" >}} out of the build.
To do this, keep the `alloy` section and remove the `receivers`, `processors`, `exporters`, `connectors`, and `extensions` sections.

The following manifest builds a {{< param "PRODUCT_NAME" >}} binary that only collects logs:

```yaml
dist:
  module: github.com/grafana/alloy/otel_engine
  name: alloy
  description: Alloy OTel Collector distribution.
  version: <ALLOY_VERSION>
  output_path: .

alloy:
  components: [local.file_match, loki.source.file, loki.process, loki.write]
  converters: false

providers:
  # Keep the providers from the original manifest.

excludes:
  # Keep the excludes from the original manifest.

replaces:
  # Keep the replace directives from the original manifest.
```

Keep the `dist`, `providers`, `excludes`, and `replaces` sections from the original `collector/builder-config.yaml`.
_`<ALLOY_VERSION>`_ is the version already set in the `dist` section of the original manifest.

A build without the {{< param "OTEL_ENGINE" >}} doesn't include the [`alloy otel`](../../reference/cli/otel/) and [`alloy otel-supervisor`](../../reference/cli/otel-supervisor/) commands, or any Collector components.

## Build the binary

After you edit `collector/builder-config.yaml`, build {{< param "PRODUCT_NAME" >}}:

```shell
make alloy
```

The build regenerates the code for the components you selected and writes the binary to `build/alloy`.
Run it the same way as a standard `alloy` binary.

To strip debug information from the binary like the official releases do, set `RELEASE_BUILD=1`:

```shell
RELEASE_BUILD=1 make alloy
```

## Build a Docker image

To build a Docker image from your custom manifest, run the following command:

```shell
make alloy-image ALLOY_IMAGE=<REGISTRY>/<IMAGE_NAME>
```

Replace the following:

- _`<REGISTRY>`_: Your image registry.
- _`<IMAGE_NAME>`_: The name and tag of your image, for example `alloy-logs:v1`.

If you don't set `ALLOY_IMAGE`, the build names the image `grafana/alloy:latest`.

## Embed the Alloy Engine extension in your own Collector distribution

You can also embed the {{< param "DEFAULT_ENGINE" >}} in your own OCB distribution with the `alloyengine` extension.
Refer to the [`alloyengine` extension README](https://github.com/grafana/alloy/blob/main/extension/alloyengine/README.md#include-alloyengine-extension-in-an-ocb-distribution) for instructions.

The extension always includes every native {{< param "PRODUCT_NAME" >}} component and the converters.
OCB ignores the `alloy` section, so you can't select native components in your own distribution.
To select native components, build {{< param "PRODUCT_NAME" >}} from its repository as described on this page.

## Next steps

- [Run the {{< param "OTEL_ENGINE" >}}](../otel_engine/) with your custom build.
- [Components reference](../../reference/components/) for the names of native components.
- [OpenTelemetry Collector Builder documentation](https://opentelemetry.io/docs/collector/custom-collector/) for the OCB manifest format.
