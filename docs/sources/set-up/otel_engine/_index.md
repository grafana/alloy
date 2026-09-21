---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/
aliases:
  - ../opentelemetry/get-started/ # /docs/alloy/latest/opentelemetry/get-started/
description: Learn how to run the OpenTelemetry Engine using the CLI, Helm chart, or service installation
menuTitle: OpenTelemetry Engine
title: The OpenTelemetry Engine
weight: 390
---

# The {{% param "FULL_OTEL_ENGINE" %}}

You can run the {{< param "OTEL_ENGINE" >}} using the CLI, the {{< param "PRODUCT_NAME" >}} Engine extension, the OpenTelemetry Collector Helm chart, or a service installation.

## Prerequisites

There are no additional prerequisites.
The tools needed to run the {{< param "OTEL_ENGINE" >}} are shipped within {{< param "PRODUCT_NAME" >}}.

Before you start, validate your OpenTelemetry YAML configuration with the `validate` command:

```bash
alloy otel validate --config=<CONFIG_FILE>
```

While this is an experimental feature, it isn't hidden behind an `experimental` feature flag like regular components.
This maintains compatibility with the OpenTelemetry Collector.

## Run the {{% param "OTEL_ENGINE" %}}

Use the following pages to learn how to run the {{< param "OTEL_ENGINE" >}}, and how to build a customized {{< param "PRODUCT_NAME" >}} binary that bundles different OpenTelemetry Collector components.

{{< section >}}

## Considerations

1. **Storage configuration**: The {{< param "DEFAULT_ENGINE" >}} accepts the `--storage.path` flag to set a base directory for components to store data on disk.
   The {{< param "OTEL_ENGINE" >}} uses the `filestorage` extension instead of a CLI flag.
   Refer to the [upstream documentation](https://opentelemetry.io/docs/collector/resiliency/#persistent-storage-write-ahead-log---wal) for more information.
1. **Server ports**: The {{< param "DEFAULT_ENGINE" >}} exposes its HTTP server on port `12345`.
   The {{< param "OTEL_ENGINE" >}} exposes its HTTP server on port `8888`.
   The {{< param "OTEL_ENGINE" >}} HTTP server doesn't expose a UI, support bundles, or reload endpoint functionality like the {{< param "DEFAULT_ENGINE" >}} does.
1. **Inline configuration module path**: If `config.inline.module_path` isn't defined, the `module_path` {{< param "PRODUCT_NAME" >}} configuration keyword resolves to the process current working directory.

## Next steps

- Refer to [OpenTelemetry in {{< param "PRODUCT_NAME" >}}](../../introduction/otel_alloy/) to learn when to use each engine.
- Refer to the [`otel` command reference](../../reference/cli/otel/) for the command options and included components.
