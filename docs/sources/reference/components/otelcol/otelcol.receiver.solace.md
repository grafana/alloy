---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.solace/
description: Learn about otelcol.receiver.solace
title: otelcol.receiver.solace
---

# otelcol.receiver.solace

`otelcol.receiver.solace` accepts traces from a [Solace PubSub+ Event Broker](https://solace.com/products/event-broker/) and
forwards it to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.solace` is a wrapper over the upstream OpenTelemetry Collector `solace` receiver from the `otelcol-contrib` distribution.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

Multiple `otelcol.receiver.solace` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.receiver.solace "LABEL" {
  queue = "QUEUE"
  auth {
    // sasl_plain or sasl_xauth2 or sasl_external block
  }
  output {
    traces  = [...]
  }
}
```

## Arguments

The following arguments are supported:

| Name                 | Type     | Description                                                               | Default          | Required |
| -------------------- | -------- | ------------------------------------------------------------------------- | ---------------- | -------- |
| `queue`              | `string` | Name of the Solace telemetry queue to get span trace messages from.       |                  | yes      |
| `broker`             | `string` | Name of the Solace broker using amqp over tls.                            | `localhost:5671` | no       |
| `max_unacknowledged` | `int`    | Maximum number of unacknowledged messages the Solace broker can transmit. | 10               | no       |

`queue` should have the format `queue://#telemetry-myTelemetryProfile`.

## Blocks

The following blocks are supported inside the definition of
`otelcol.receiver.solace`:

| Hierarchy                      | Block              | Description                                                                                                                      | Required |
| ------------------------------ | ------------------ | -------------------------------------------------------------------------------------------------------------------------------- | -------- |
| authentication                 | [authentication][] | Configures authentication for connecting to the Solace broker.                                                                   | yes      |
| authentication > sasl_plain    | [sasl_plain][]     | Authenticates against the Solace broker with SASL PLAIN.                                                                         | no       |
| authentication > sasl_xauth2   | [sasl_xauth2][]    | Authenticates against the Solace broker with SASL XOauth2.                                                                       | no       |
| authentication > sasl_external | [sasl_external][]  | Authenticates against the Solace broker with SASL External.                                                                      | no       |
| flow                           | [flow][]           | Configures the behaviour to use when temporary errors are encountered from the next component.                                   | no       |
| flow > delayed_retry           | [delayed_retry][]  | Sets the flow control strategy to `delayed retry` which will wait before trying to push the message to the next component again. | no       |
| tls                            | [tls][]            | Configures TLS for connecting to the Solace broker.                                                                              | no       |
| debug_metrics                  | [debug_metrics][]  | Configures the metrics which this component generates to monitor its state.                                                      | no       |
| output                         | [output][]         | Configures where to send received telemetry data.                                                                                | yes      |

One SASL authentication block is required in the `authentication` block.

`sasl_external` must be used together with the `tls` block.

The `>` symbol indicates deeper levels of nesting. For example,
`authentication > tls` refers to a `tls` block defined inside an
`authentication` block.

[authentication]: #authentication-block
[sasl_plain]: #sasl_plain-block
[sasl_xauth2]: #sasl_xauth2-block
[sasl_external]: #sasl_external-block
[tls]: #tls-block
[flow]: #flow-block
[delayed_retry]: #delayed_retry-block
[debug_metrics]: #debug_metrics-block
[output]: #output-block

### authentication block

The `authentication` block configures how to authenticate for connecting to the Solace broker.
It doesn't support any arguments and is configured fully through inner blocks.

### sasl_plain block

The `sasl_plain` block configures how to authenticate to the Solace broker with SASL PLAIN.

The following arguments are supported:

| Name       | Type     | Description          | Default | Required |
| ---------- | -------- | -------------------- | ------- | -------- |
| `username` | `string` | The username to use. |         | yes      |
| `password` | `string` | The password to use. |         | yes      |

### sasl_xauth2 block

The `sasl_xauth2` block configures how to authenticate to the Solace broker with SASL XOauth2.

The following arguments are supported:

| Name       | Type     | Description               | Default | Required |
| ---------- | -------- | ------------------------- | ------- | -------- |
| `username` | `string` | The username to use.      |         | yes      |
| `bearer`   | `string` | The bearer in plain text. |         | yes      |

### sasl_external block

The `sasl_xauth2` block configures how to authenticate to the Solace broker with SASL External.
It doesn't support any arguments or blocks. It must be used with the [tls][] block.

### flow block

The `flow` block configures the behaviour to use when temporary errors are encountered from the next component.
It doesn't support any arguments and is configured fully through inner blocks.

### delayed_retry block

The `delayed_retry` block sets the flow control strategy to `delayed retry` which will wait before trying to push the message to the next component again.

The following arguments are supported:

| Name    | Type     | Description                       | Default  | Required |
| ------- | -------- | --------------------------------- | -------- | -------- |
| `delay` | `string` | The time to wait before retrying. | `"10ms"` | no       |

### tls block

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< admonition type="warning" >}}
Having multiple consumers may result in duplicated traces in case of errors because of the retry strategy.
It is recommended to only set one consumer for this component.
{{< /admonition >}}

## Exported fields

`otelcol.receiver.solace` does not export any fields.

## Component health

`otelcol.receiver.solace` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.receiver.solace` does not expose any component-specific debug
information.

## Example

This example forwards read telemetry data through a batch processor before
finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.solace "default" {
  queue = "queue://#telemetry-testprofile"
  broker = "localhost:5672"
  auth {
    sasl_plain {
      username = "alloy"
      password = "password"
    }
  }
  tls {
    insecure             = true
    insecure_skip_verify = true
  }
  output {
    traces = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    traces  = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = sys.env("OTLP_ENDPOINT")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.solace` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
