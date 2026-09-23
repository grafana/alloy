---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/custom-builds/
description: Learn how to build a customized Alloy binary with the OpenTelemetry Collector Builder
menuTitle: Custom builds
title: Custom builds with the OpenTelemetry Collector Builder (OCB)
weight: 500
---

# Custom builds with the OpenTelemetry Collector Builder (OCB)

Grafana builds the {{< param "OTEL_ENGINE" >}} from a declarative [OpenTelemetry Collector Builder (OCB)][OCB] manifest.
If you need additional components or want to remove bundled components, edit the manifest and build a customized {{< param "PRODUCT_NAME" >}} binary.

{{< admonition type="caution" >}}
Grafana doesn't offer commercial support for custom builds.
{{< /admonition >}}

## Before you begin

Make sure you have the following tools:

- Go 1.26.7 or later
- Node.js
- Docker, if you want to build the container image

## Build a custom {{% param "PRODUCT_NAME" %}} binary

1. Clone the {{< param "PRODUCT_NAME" >}} repository and change to the repository root.
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

1. Add or remove components by editing the OCB manifest in [`collector/builder-config.yaml`][BuilderConfig].

   To remove a component, delete its entire entry from the appropriate section.
   To add a component, append an entry that points at the module path and version you want.
   Follow the same `- gomod:` pattern as the other entries.

1. Build the full {{< param "PRODUCT_NAME" >}} binary.

   ```shell
   make alloy
   ```

   The binary in `build/` behaves like a standard `alloy` build.
   To skip the UI build when the UI assets already exist, run `SKIP_UI_BUILD=1 make alloy`.
   Use [`alloy otel`][OTelCommand] to run collector YAML against your custom bundle.

1. Confirm that your components are in the build.

   ```shell
   ./build/alloy otel components
   ```

   The command lists each bundled component with its module path and version.

1. Build the {{< param "PRODUCT_NAME" >}} Docker image.

   ```shell
   make alloy-image <ALLOY_IMAGE>=<REGISTRY>/<IMAGE_NAME>:<TAG>
   ```

   Replace the following:

   - _`<ALLOY_IMAGE>`_: Your image repository and image name. If you don't set _`<ALLOY_IMAGE>`_, the build defaults to `grafana/alloy:latest`.
   - _`<REGISTRY>`_: Your container registry.
   - _`<IMAGE_NAME>`_: Your image name.
   - _`<TAG>`_: Your image tag.

[BuilderConfig]: https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml
[OCB]: https://opentelemetry.io/docs/collector/custom-collector/
[OTelCommand]: ../../../reference/cli/otel/
[ReleaseTag]: https://github.com/grafana/alloy/releases
