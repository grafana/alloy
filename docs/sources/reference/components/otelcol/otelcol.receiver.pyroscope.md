---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.pyroscope/
aliases:
  - ../otelcol.receiver.pyroscope/ # /docs/alloy/latest/reference/components/otelcol.receiver.pyroscope/
description: Learn about otelcol.receiver.pyroscope
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.pyroscope
---

# `otelcol.receiver.pyroscope`

`otelcol.receiver.pyroscope` receives pprof profiles from `pyroscope.*` components, converts them to the OpenTelemetry profiles format, and forwards them to profiles-capable `otelcol.*` components.

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

You can specify multiple `otelcol.receiver.pyroscope` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.pyroscope "<LABEL>" {
  output {
    profiles = <PROFILE_CONSUMER_LIST>
  }
}
```

## Arguments

The `otelcol.receiver.pyroscope` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.receiver.pyroscope`:

{{< docs/alloy-config >}}

| Block              | Description                                        | Required |
| ------------------ | -------------------------------------------------- | -------- |
| [`output`][output] | Configures where to send converted profile data.   | yes      |

[output]: #output

{{< /docs/alloy-config >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.pyroscope` only forwards data to the consumers listed in the `profiles` argument.
The `profiles` argument must contain at least one consumer.
Connect the `profiles` argument only to components that support the profiles signal.
Currently, [`otelcol.exporter.otlp`][otelcol.exporter.otlp] is the only `otelcol.*` component that accepts profiles.

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/

The component maps the `service_name` Pyroscope label to the `service.name` OpenTelemetry resource attribute.
It maps the `otel.scope.name` and `otel.scope.version` labels to the OpenTelemetry instrumentation scope.
It preserves the `__name__` label as an OpenTelemetry resource attribute because the label identifies the profile type.
The component maps all other non-reserved Pyroscope labels to OpenTelemetry resource attributes.
It drops other reserved labels, including `__delta__`.
Labels embedded in individual pprof samples remain sample attributes and are separate from these external resource labels.

The component supports pprof payloads received through the standard Pyroscope append path.
Through the Pyroscope ingest path, the component only supports non-multipart requests that explicitly set the `format` query parameter to `pprof`.
Multipart pprof, JFR, folded, speedscope, and other ingest formats return an unsupported-format error.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type               | Description                                                               |
| ---------- | ------------------ | ------------------------------------------------------------------------- |
| `receiver` | `ProfilesReceiver` | A value that other components can use to send pprof profile data to.      |

## Component health

`otelcol.receiver.pyroscope` is only reported as unhealthy if given an invalid configuration.
Profile conversion and downstream consumer errors are returned to the component that sends the profile.

## Debug information

`otelcol.receiver.pyroscope` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.receiver.pyroscope` doesn't expose any component-specific debug metrics.

{{< admonition type="note" >}}
The OpenTelemetry profiles protocol doesn't transport Pyroscope debug information.
When you use `otelcol.receiver.pyroscope`, Pyroscope components can't upload debug information through this pipeline.
{{< /admonition >}}

## Example

This example uses `otelcol.receiver.pyroscope` as a bridge between the Pyroscope and OpenTelemetry ecosystems.
The component converts profiles collected by `pyroscope.ebpf` and forwards them to an OpenTelemetry Collector over OTLP gRPC:

```alloy
pyroscope.ebpf "default" {
  forward_to = [otelcol.receiver.pyroscope.default.receiver]
}

otelcol.receiver.pyroscope "default" {
  output {
    profiles = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = "otel-collector:4317"
    tls {
      insecure = true
    }
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.pyroscope` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.receiver.pyroscope` has exports that can be consumed by the following components:

- Components that consume [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
