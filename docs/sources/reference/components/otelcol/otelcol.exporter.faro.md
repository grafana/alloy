---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.faro/
aliases:
  - ../otelcol.exporter.faro/ # /docs/alloy/latest/reference/components/otelcol.exporter.faro/
description: Learn about otelcol.exporter.faro
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.exporter.faro
---

# `otelcol.exporter.faro`

`otelcol.exporter.faro` accepts logs and traces telemetry data from other `otelcol` components and sends it to [Faro][Faro] endpoint.

{{< admonition type="note" >}}
`otelcol.exporter.faro` is a wrapper over the upstream OpenTelemetry Collector `faro` exporter from the `otelcol-contrib`  distribution.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

Multiple `otelcol.exporter.faro` components can be specified by giving them different labels.

## Usage

```alloy
otelcol.exporter.faro "<LABEL>" {
    client {
        endpoint = "<HOST>:<PORT>"
    }
}
```

## Blocks

You can use the following blocks with `otelcol.exporter.faro`:

| Block                                                 | Description                                                                | Required |
| ----------------------------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`client`][client]                                    | Configures the HTTP client to send telemetry data to.                      | yes      |
| `client` > [`compression_params`][compression_params] | Configure advanced compression options.                                    | no       |
| `client` > [`cookies`][cookies]                       | Store cookies from server responses and reuse them in subsequent requests. | no       |
| `client` > [`tls`][tls]                               | Configures TLS for the HTTP client.                                        | no       |
| [`debug_metrics`][debug_metrics]                      | Configures the metrics that this component generates to monitor its state. | no       |
| [`retry_on_failure`][retry_on_failure]                | Configures retry mechanism for failed requests.                            | no       |
| [`sending_queue`][sending_queue]                      | Configures batching of data before sending.                                | no       |

The > symbol indicates deeper levels of nesting.
For example, `client` > `tls` refers to a `tls` block defined inside a `client` block.

[client]: #client
[tls]: #tls
[cookies]: #cookies
[compression_params]: #compression_params
[sending_queue]: #sending_queue
[retry_on_failure]: #retry_on_failure
[debug_metrics]: #debug_metrics

### `client`

<span class="badge docs-labels__stage docs-labels__item">Required</span>

The `client` block configures the HTTP client used by the component.

{{< docs/shared lookup="reference/components/otelcol-http-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `compression_params`

The `compression_params` block allows for configuration of advanced compression options.

{{< docs/shared lookup="reference/components/otelcol-compression-params-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `cookies`

The `cookies` block allows the HTTP client to store cookies from server responses and reuse them in subsequent requests.

This could be useful in situations such as load balancers relying on cookies for sticky sessions and enforcing a maximum session age.

{{< docs/shared lookup="reference/components/otelcol-cookies-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS settings used for the connection to the HTTP server.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to the HTTP server are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures an in-memory buffer of batches before data is sent to the HTTP server.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

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
