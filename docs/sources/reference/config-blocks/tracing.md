---
aliases:
- /docs/grafana-cloud/agent/flow/reference/config-blocks/tracing/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/config-blocks/tracing/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/config-blocks/tracing/
- /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/tracing/
canonical: https://grafana.com/docs/agent/latest/flow/reference/config-blocks/tracing/
description: Learn about the tracing configuration block
menuTitle: tracing
title: tracing block
---

# tracing block

`tracing` is an optional configuration block used to customize how {{< param "PRODUCT_NAME" >}} produces traces.
`tracing` is specified without a label and can only be provided once per configuration file.

## Example

```river
tracing {
  sampling_fraction = 0.1

  write_to = [otelcol.exporter.otlp.tempo.input]
}

otelcol.exporter.otlp "tempo" {
  // Send traces to a locally running Tempo without TLS enabled.
  client {
    endpoint = env("TEMPO_OTLP_ENDPOINT")

    tls {
      insecure = true
    }
  }
}
```

## Arguments

The following arguments are supported:

Name                | Type                     | Description                                         | Default | Required
--------------------|--------------------------|-----------------------------------------------------|---------|---------
`sampling_fraction` | `number`                 | Fraction of traces to keep.                         | `0.1`   | no
`write_to`          | `list(otelcol.Consumer)` | Inputs from `otelcol` components to send traces to. | `[]`    | no

The `write_to` argument controls which components to send traces to for
processing. The elements in the array can be any `otelcol` component that
accept traces, including processors and exporters. When `write_to` is set
to an empty array `[]`, all traces are dropped.

> **NOTE**: Any traces generated before the `tracing` block has been evaluated,
> such as at the early start of the process' lifetime, are dropped.

The `sampling_fraction` argument controls what percentage of generated traces
should be sent to the consumers specified by `write_to`. When set to `1` or
greater, 100% of traces are kept. When set to `0` or lower, 0% of traces are
kept.

## Blocks

The following blocks are supported inside the definition of `tracing`:

Hierarchy               | Block             | Description                                                  | Required
------------------------|-------------------|--------------------------------------------------------------|---------
sampler                 | [sampler][]       | Define custom sampling on top of the base sampling fraction. | no
sampler > jaeger_remote | [jaeger_remote][] | Retrieve sampling information via a Jaeger remote sampler.   | no

The `>` symbol indicates deeper levels of nesting. For example, `sampler >
jaeger_remote` refers to a `jaeger_remote` block defined inside an `sampler`
block.

[sampler]: #sampler-block
[jaeger_remote]: #jaeger_remote-block

### sampler block

The `sampler` block contains a definition of a custom sampler to use. The
`sampler` block supports no arguments and is controlled fully through inner
blocks.

It is invalid to define more than one sampler to use in the `sampler` block.

### jaeger_remote block

The `jaeger_remote` block configures the retrieval of sampling information
through a remote server that exposes Jaeger sampling strategies.

Name               | Type       | Description                                                | Default                            | Required
-------------------|------------|------------------------------------------------------------|------------------------------------|---------
`url`              | `string`   | URL to retrieve sampling strategies from.                  | `"http://127.0.0.1:5778/sampling"` | no
`max_operations`   | `number`   | Limit number of operations which can have custom sampling. | `256`                              | no
`refresh_interval` | `duration` | Frequency to poll the URL for new sampling strategies.     | `"1m"`                             | no

The remote sampling strategies are retrieved from the URL specified by the
`url` argument, and polled for updates on a timer. The frequency for how oftenName | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`names` | `list(string)` | DNS names to look up. | | yes
`port` | `number` | Port to use for collecting metrics. Not used for SRV records. | `0` | no
`refresh_interval` | `duration` | How often to query DNS for updates. | `"30s"` | no
`type` | `string` | Type of DNS record to query. Must be one of SRV, A, AAAA, or MX. | `"SRV"` | no
polling occurs is controlled by the `refresh_interval` argument.

Requests to the remote sampling strategies server are made through an HTTP
`GET` request to the configured `url` argument. A `service=grafana-agent` query
parameter is always added to the URL to allow the server to respond with
service-specific strategies. The HTTP response body is read as JSON matching
the schema specified by Jaeger's [`strategies.json` file][Jaeger sampling
strategies].

The `max_operations` limits the amount of custom span names that can have
custom sampling rules. If the remote sampling strategy exceeds the limit,
sampling decisions fall back to the default sampler.

[Jaeger sampling strategies]: https://www.jaegertracing.io/docs/1.22/sampling/#collector-sampling-configuration
