---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-compression-params-client-block/
description: Shared content, otelcol compression params client block
headless: true
---

The following arguments are supported:

| Name    | Type  | Description                  | Default | Required |
| ------- | ----- | ---------------------------- | ------- | -------- |
| `level` | `int` | Configure compression level. |         | yes      |

For valid combinations of `client.compression` and `client.compression_params.level`, refer to the [upstream documentation][confighttp].

[confighttp]: https://github.com/open-telemetry/opentelemetry-collector/blob/<OTEL_VERSION>/config/confighttp/README.md
