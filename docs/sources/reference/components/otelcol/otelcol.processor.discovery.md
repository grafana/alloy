---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.discovery/
aliases:
  - ../otelcol.processor.discovery/ # /docs/alloy/latest/reference/otelcol.processor.discovery/
description: Learn about otelcol.processor.discovery
title: otelcol.processor.discovery
---

# otelcol.processor.discovery

`otelcol.processor.discovery` accepts traces telemetry data from other `otelcol` components.
It can be paired with `discovery.*` components, which supply a list of labels for each discovered target.
`otelcol.processor.discovery` adds resource attributes to spans which have a hostname matching the one in the `__address__` label provided by the `discovery.*` component.

{{< admonition type="note" >}}
`otelcol.processor.discovery` is a custom component unrelated to any processors from the OpenTelemetry Collector.
{{< /admonition >}}

Multiple `otelcol.processor.discovery` components can be specified by giving them different labels.

{{< admonition type="note" >}}
It can be difficult to follow [OpenTelemetry semantic conventions][OTEL sem conv] when
adding resource attributes via `otelcol.processor.discovery`:
* `discovery.relabel` and most `discovery.*` processes such as `discovery.kubernetes`
  can only emit [Prometheus-compatible labels][Prometheus data model].
* Prometheus labels use underscores (`_`) in labels names, whereas
  [OpenTelemetry semantic conventions][OTEL sem conv] use dots (`.`).
* Although `otelcol.processor.discovery` is able to work with non-Prometheus labels
  such as ones containing dots, the fact that `discovery.*` components are generally
  only compatible with Prometheus naming conventions makes it hard to follow OpenTelemetry
  semantic conventions in `otelcol.processor.discovery`.

If your use case is to add resource attributes which contain Kubernetes metadata, consider using `otelcol.processor.k8sattributes` instead.

------
The main use case for `otelcol.processor.discovery` is for users who migrate to {{< param "PRODUCT_NAME" >}}
from Grafana Agent Static mode `prom_sd_operation_type`/`prom_sd_pod_associations` [configuration options][Traces].

[Prometheus data model]: https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
[OTEL sem conv]: https://github.com/open-telemetry/semantic-conventions/blob/main/docs/README.md
[Traces]: https://grafana.com/docs/agent/latest/static/configuration/traces-config/
{{< /admonition >}}

## Usage

```alloy
otelcol.processor.discovery "LABEL" {
  targets = [...]
  output {
    traces  = [...]
  }
}
```

## Arguments

`otelcol.processor.discovery` supports the following arguments:

Name               | Type                | Description                                                           | Default                                                         | Required
-------------------|---------------------|-----------------------------------------------------------------------|-----------------------------------------------------------------|---------
`targets`          | `list(map(string))` | List of target labels to apply to the spans.                          |                                                                 | yes
`operation_type`   | `string`            | Configures whether to update a span's attribute if it already exists. | `upsert`                                                        | no
`pod_associations` | `list(string)`      | Configures how to decide the hostname of the span.                    | `["ip", "net.host.ip", "k8s.pod.ip", "hostname", "connection"]` | no

`targets` could come from `discovery.*` components:
1. The `__address__` label will be matched against the IP address of incoming spans.
   * If `__address__` contains a port, it is ignored.
2. If a match is found, then relabeling rules are applied.
   * Note that labels starting with `__` will not be added to the spans.

The supported values for `operation_type` are:
* `insert`: Inserts a new resource attribute if the key does not already exist.
* `update`: Updates a resource attribute if the key already exists.
* `upsert`: Either inserts a new resource attribute if the key does not already exist,
   or updates a resource attribute if the key does exist.

The supported values for `pod_associations` are:
* `ip`: The hostname will be sourced from an `ip` resource attribute.
* `net.host.ip`: The hostname will be sourced from a `net.host.ip` resource attribute.
* `k8s.pod.ip`: The hostname will be sourced from a `k8s.pod.ip` resource attribute.
* `hostname`: The hostname will be sourced from a `host.name` resource attribute.
* `connection`: The hostname will be sourced from the context from the incoming requests (gRPC and HTTP).

If multiple `pod_associations` methods are enabled, the order of evaluation is honored.
For example, when `pod_associations` is `["ip", "net.host.ip"]`, `"net.host.ip"` may be matched
only if `"ip"` hasn't already matched.

## Blocks

The following blocks are supported inside the definition of
`otelcol.processor.discovery`:

Hierarchy | Block      | Description                                       | Required
----------|------------|---------------------------------------------------|---------
output    | [output][] | Configures where to send received telemetry data. | yes

[output]: #output-block

### output block

{{< docs/shared lookup="reference/components/output-block-traces.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` OTLP-formatted data for telemetry signals of these types:
* traces

## Component health

`otelcol.processor.discovery` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.discovery` doesn't expose any component-specific debug
information.

## Examples

### Basic usage
```alloy
discovery.http "dynamic_targets" {
    url              = "https://example.com/scrape_targets"
    refresh_interval = "15s"
}

otelcol.processor.discovery "default" {
    targets = discovery.http.dynamic_targets.targets

    output {
        traces = [otelcol.exporter.otlp.default.input]
    }
}
```

### Using more than one discovery process

Outputs from more than one discovery process can be combined via the `concat` function.

```alloy
discovery.http "dynamic_targets" {
    url              = "https://example.com/scrape_targets"
    refresh_interval = "15s"
}

discovery.kubelet "k8s_pods" {
  bearer_token_file = "/var/run/secrets/kubernetes.io/serviceaccount/token"
  namespaces        = ["default", "kube-system"]
}

otelcol.processor.discovery "default" {
    targets = concat(discovery.http.dynamic_targets.targets, discovery.kubelet.k8s_pods.targets)

    output {
        traces = [otelcol.exporter.otlp.default.input]
    }
}
```

### Using a preconfigured list of attributes

It's not necessary to use a discovery component.
In the example below, both a `test_label` and a `test.label.with.dots` resource attributes will be added to a span if its IP address is "1.2.2.2".
The `__internal_label__` will be not be added to the span, because it begins with a double underscore (`__`).

```alloy
otelcol.processor.discovery "default" {
    targets = [{
        "__address__"          = "1.2.2.2",
        "__internal_label__"   = "test_val",
        "test_label"           = "test_val2",
        "test.label.with.dots" = "test.val2.with.dots"}]

    output {
        traces = [otelcol.exporter.otlp.default.input]
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.discovery` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.discovery` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
