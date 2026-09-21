---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/custom-builds/
description: Learn how to build a customized Alloy binary with the OpenTelemetry Collector Builder
menuTitle: Custom builds
title: Custom builds with the OpenTelemetry Collector Builder (OCB)
weight: 500
---

# Custom builds with the OpenTelemetry Collector Builder (OCB)

The {{< param "OTEL_ENGINE" >}} is generated from a declarative [OpenTelemetry Collector Builder (OCB)](https://opentelemetry.io/docs/collector/custom-collector/) manifest.
If you need additional components or want to remove bundled components, edit the manifest and build a customized {{< param "PRODUCT_NAME" >}} binary.
Grafana doesn't offer commercial support for custom builds.

1. Clone the {{< param "PRODUCT_NAME" >}} repository and change to the repository root.
   The following steps assume you run commands from this directory.

   ```shell
   git clone https://github.com/grafana/alloy.git
   cd alloy
   ```

   To build from a **specific release**, fetch tags and check out the tag after you clone:

   ```shell
   git fetch --tags
   git checkout <RELEASE_TAG>
   ```

   Replace _`<RELEASE_TAG>`_ with the [release tag](https://github.com/grafana/alloy/releases) you want.

1. Add or remove components by editing the OCB manifest in [`collector/builder-config.yaml`](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml).

   To **Remove** a component, delete its `- gomod: ...` line from the appropriate section.
   To **Add** a component, append a line that points at the module path and version you want.
   Follow the same `- gomod:` pattern as the other entries.

1. Build the full {{< param "PRODUCT_NAME" >}} binary.

   ```shell
   make alloy
   ```

   The binary in `build/` behaves like a standard `alloy` build.
   Use [`alloy otel`](../../../reference/cli/otel/) to run collector YAML against your custom bundle.

1. Build the {{< param "PRODUCT_NAME" >}} Docker image.

   ```shell
   make alloy-image <ALLOY_IMAGE>=<REGISTRY>/<IMAGE_NAME>
   ```

   Replace _`<ALLOY_IMAGE>`_ with your image repository and image name.
   If you don't set the image repository and image name, the build defaults to `grafana/alloy`.
