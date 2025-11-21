---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.faro/
description: Learn about otelcol.exporter.faro
labels:
  stage: experimental
  products:
    - oss
title: otelcol.exporter.faro
---

# `otelcol.exporter.faro`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.faro` accepts logs and traces telemetry data from other `otelcol` components and sends it to [Faro][Faro] endpoint.
Use this exporter to send telemetry data to Grafana Cloud Collector Endpoint for [Frontend Observability][Frontend Observability] or to any backend that supports Faro format, allowing you to gain insights into the end user experience of your web application.

{{< docs/shared lookup="reference/components/otelcol-faro-component-note.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< admonition type="note" >}}
`otelcol.exporter.faro` is a wrapper over the upstream OpenTelemetry Collector [`faro`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`faro`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/faroexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.faro` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.faro "<LABEL>" {
    client {
        endpoint = "<HOST>:<PORT>"
    }
}
```

## Arguments

The `otelcol.exporter.faro` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.exporter.faro`:

| Block                                                 | Description                                                                    | Required |
|-------------------------------------------------------|--------------------------------------------------------------------------------|----------|
| [`client`][client]                                    | Configures the HTTP client to send telemetry data to.                          | yes      |
| `client` > [`compression_params`][compression_params] | Configure advanced compression options.                                        | no       |
| `client` > [`cookies`][cookies]                       | Store cookies from server responses and reuse them in subsequent requests.     | no       |
| `client` > [`tls`][tls]                               | Configures TLS for the HTTP client.                                            | no       |
| `client` > `tls` > [`tpm`][tpm]                       | Configures TPM settings for the TLS key_file.                                  | no       |
| [`debug_metrics`][debug_metrics]                      | Configures the metrics that this component generates to monitor its state.     | no       |
| [`retry_on_failure`][retry_on_failure]                | Configures retry mechanism for failed requests.                                | no       |
| [`sending_queue`][sending_queue]                      | Configures batching of data before sending.                                    | no       |
| `sending_queue` > [`batch`][batch]                    | Configures batching requests based on a timeout and a minimum number of items. | no       |

The > symbol indicates deeper levels of nesting.
For example, `client` > `tls` refers to a `tls` block defined inside a `client` block.

[client]: #client
[tls]: #tls
[tpm]: #tpm
[cookies]: #cookies
[compression_params]: #compression_params
[sending_queue]: #sending_queue
[batch]: #batch
[retry_on_failure]: #retry_on_failure
[debug_metrics]: #debug_metrics

### `client`

{{< badge text="Required" >}}

The `client` block configures the HTTP client used by the component.

{{< docs/shared lookup="reference/components/otelcol-http-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `compression_params`

The `compression_params` block configures the advanced compression options.

{{< docs/shared lookup="reference/components/otelcol-compression-params-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `cookies`

The `cookies` block allows the HTTP client to store cookies from server responses and reuse them in subsequent requests.

This could be useful in situations such as load balancers relying on cookies for sticky sessions and enforcing a maximum session age.

{{< docs/shared lookup="reference/components/otelcol-cookies-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS settings used for the connection to the HTTP server.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to the HTTP server are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures queueing and batching for the exporter.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for logs and traces.

## Component health

`otelcol.exporter.faro` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.faro` doesn't expose any component-specific debug information.

## Example

This example creates an exporter to send data to a [Faro][Faro] endpoint.

```alloy
otelcol.exporter.faro "default" {
    client {
        endpoint = "<FARO_COLLECTOR_ADDRESS>"
    }
}
```

Replace the following:

* _`<FARO_COLLECTOR_ADDRESS>`_: The address of the Faro-compatible server to send data to.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.faro` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

[Faro]: https://grafana.com/oss/faro/
[Frontend Observability]: https://grafana.com/products/cloud/frontend-observability-for-real-user-monitoring/
