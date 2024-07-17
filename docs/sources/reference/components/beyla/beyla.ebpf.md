---
canonical: https://grafana.com/docs/alloy/latest/reference/components/beyla/beyla.ebpf/
aliases:
  - ../beyla.ebpf/ # /docs/alloy/latest/reference/components/beyla.ebpf/
description: Learn about beyla.ebpf
title: beyla.ebpf
---

<span class="badge docs-labels__stage docs-labels__item">Public preview</span>

# beyla.ebpf

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `beyla.ebpf` component is used as a wrapper for [Grafana Beyla][] which uses [eBPF][] to automatically inspect application executables and the OS networking layer, and capture trace spans related to web transactions and Rate Errors Duration (RED) metrics for Linux HTTP/S and gRPC services.
You can configure the component to collect telemetry data from a specific port or executable path, and other criteria from Kubernetes metadata.
The component exposes metrics that can be collected by a Prometheus scrape component, and traces that can be forwarded to an OTEL exporter component.

{{< admonition type="note" >}}
To run this component, {{< param "PRODUCT_NAME" >}} requires administrative (`sudo`) privileges, or at least it needs to be granted the `CAP_SYS_ADMIN` and `CAP_SYS_PTRACE` capability.
In Kubernetes environments, the [AppArmor profile must be `Unconfined`](https://kubernetes.io/docs/tutorials/security/apparmor/#securing-a-pod) for the Deployment or DaemonSet running {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}


## Usage

```alloy
beyla.ebpf "<LABEL>" {

}
```

## Arguments

`beyla.ebpf` supports the following arguments:

Name              | Type     | Description                                                                         | Default | Required
------------------|----------|-------------------------------------------------------------------------------------|---------|---------
`open_port`       | `string` | The port of the running service for Beyla automatically instrumented with eBPF.     | `""`    | no
`executable_name` | `string` | The name of the executable to match for Beyla automatically instrumented with eBPF. | `""`    | no

`open_port` accepts a comma-separated list of ports (for example, `80,443`), and port ranges (for example, `8000-8999`).
If the executable matches only one of the ports in the list, it is considered to match the selection criteria.

`executable_name` accepts a regular expression to be matched against the full executable command line, including the directory where the executable resides on the file system.

## Blocks

The following blocks are supported inside the definition of `beyla.ebpf`:

Hierarchy               | Block          | Description                                                                                        | Required
------------------------|----------------|----------------------------------------------------------------------------------------------------|---------
routes                  | [routes][]     | Configures the routes to match HTTP paths into user-provided HTTP routes.                          | no
attributes              | [attributes][] | Configures the Beyla attributes for the component.                                                 | no
attributes > kubernetes | [kubernetes attributes][] | Configures decorating of the metrics and traces with Kubernetes metadata of the instrumented Pods. | no
discovery               | [discovery][]  | Configures the discovery for instrumentable processes matching a given criteria.                   | no
discovery > services    | [services][]   | Configures the discovery for the component.                                                        | no
discovery > services > kubernetes    | [kubernetes services][]   | Configures the discovery for the component.                                | no
output                  | [output][]     | Configures where to send received telemetry data.                                                  | yes

The `>` symbol indicates deeper levels of nesting.
For example,`attributes > kubernetes` refers to a `kubernetes` block defined inside an `attributes` block.

### attributes block

This block allows you to configure how some attributes for metrics and traces are decorated.

It contains the following blocks:

#### kubernetes attributes block

Name     | Type     | Description                                | Default | Required
---------|----------|--------------------------------------------|---------|---------
`enable` | `string` | Enable the Kubernetes metadata decoration. | `false` | no

If set to `true`, Beyla will decorate the metrics and traces with Kubernetes metadata. The following labels will be added:

- `k8s.namespace.name`
- `k8s.deployment.name`
- `k8s.statefulset.name`
- `k8s.replicaset.name`
- `k8s.daemonset.name`
- `k8s.node.name`
- `k8s.pod.name`
- `k8s.pod.uid`
- `k8s.pod.start_time`

If set to `false`, the Kubernetes metadata decorator will be disabled.

If set to `autodetect`, Beyla will try to automatically detect if it is running inside Kubernetes, and enable the metadata decoration if that's the case.

### routes block

This block is used to configure the routes to match HTTP paths into user-provided HTTP routes.

Name              | Type           | Description                                                                               | Default | Required
------------------|----------------|-------------------------------------------------------------------------------------------|---------|---------
`patterns`        | `list(string)` | List of provided URL path patterns to set the `http.route` trace/metric property          | `[]`    | no
`ignore_patterns` | `list(string)` | List of provided URL path patterns to ignore from `http.route` trace/metric property.     | `[]`    | no
`ignore_mode`     | `string`       | The mode to use when ignoring patterns.                                                   | `""`    | no
`unmatched`       | `string`       | Specifies what to do when a trace HTTP path does not match any of the `patterns` entries. | `""`    | no

`patterns` and `ignored_patterns` are a list of patterns which a URL path with specific tags which allow for grouping path segments (or ignored them).
The matcher tags can be in the `:name` or `{name}` format.
`ignore_mode` properties are:
- `all` discards metrics and traces matching the `ignored_patterns`.
- `traces` discards only the traces that match the `ignored_patterns`. No metric events are ignored.
- `metrics` discards only the metrics that match the `ignored_patterns`. No trace events are ignored.
`unmatched` properties are:
- `unset` leaves the `http.route` property as unset.
- `path` copies the `http.route` field property to the path value.
  - Caution: This option could lead to a cardinality explosion on the ingester side.
- `wildcard` sets the `http.route` field property to a generic asterisk-based `/**` value.
- `heuristic` automatically derives the `http.route` field property from the path value based on the following rules:
  - Any path components that have numbers or characters outside of the ASCII alphabet (or `-` and _), are replaced by an asterisk `*`.
  - Any alphabetical components that don’t look like words are replaced by an asterisk `*`.

### discovery block

This block is used to configure the discovery for instrumentable processes matching a given criteria.

It contains the following blocks:

### services block

In some scenarios, Beyla will instrument a wide variety of services, such as a Kubernetes DaemonSet that instruments all the services in a node.
This block allows you to filter the services to instrument based on their metadata. If you specify other selectors in the same services entry,
the instrumented processes need to match all the selector properties.

Name         | Type     | Description                                                                     | Default | Required
-------------|----------|---------------------------------------------------------------------------------|---------|---------
`name `      | `string` | The name of the service to match.                                               | `""`    | no
`namespace`  | `string` | The namespace of the service to match.                                          | `""`    | no
`open_ports` | `string` | The port of the running service for Beyla automatically instrumented with eBPF. | `""`    | no
`exe_path`   | `string` | The path of the running service for Beyla automatically instrumented with eBPF. | `""`    | no

`name` defines a name for the matching instrumented service.
It is used to populate the `service.name` OTEL property and/or the `service_name` Prometheus property in the exported metrics/traces.
`open_port` accepts a comma-separated list of ports (for example, `80,443`), and port ranges (for example, `8000-8999`).
If the executable matches only one of the ports in the list, it is considered to match the selection criteria.
`exe_path` accepts a regular expression to be matched against the full executable command line, including the directory where the executable resides on the file system.

### kubernetes services block

This block allows you to filter the services to instrument based on their Kubernetes metadata. If you specify other selectors in the same services entry,
the instrumented processes need to match all the selector properties.

Name               | Type           | Description                                                                                                | Default | Required
-------------------|----------------|-------------------------------------------------------------------------------------------------------------|---------|---------
`namespace`        | `string`       | Regular expression of Kubernetes Namespaces to match.                                                       | `""`    | no
`pod_name`         | `string`       | Regular expression of Kubernetes Pods to match.                                                             | `""`    | no
`deployment_name`  | `string`       | Regular expression of Kubernetes Deployments to match.                                                      | `""`    | no
`statefulset_name` | `string`       | Regular expression of Kubernetes StatefulSets to match.                                                     | `""`    | no
`replicaset_name`  | `string`       | Regular expression of Kubernetes ReplicaSets to match.                                                      | `""`    | no
`daemonset_name`   | `string`       | Regular expression of Kubernetes DaemonSets to match.                                                       | `""`    | no
`owner_name`       | `string`       | Regular expression of Kubernetes owners of running Pods to match.                                           | `""`    | no
`pod_labels`       | `map(string)`  | Key-value pairs of labels with keys matching Kubernetes Pods with the provided value as regular expression. |  `{}`   | no


### output block

The `output` block configures a set of components to forward the resulting telemetry data to.

The following arguments are supported:

Name      | Type                     | Description                           | Default | Required
----------|--------------------------|---------------------------------------|---------|---------
`traces`  | `list(otelcol.Consumer)` | List of consumers to send traces to.  | `[]`    | no

You must specify the `output` block, but all its arguments are optional.
By default, telemetry data is dropped.
Configure the `traces` argument to send traces data to other components.

## Exported fields

The following fields are exported and can be referenced by other components.

Name      | Type                | Description
----------|---------------------|------------------------------------------------------------------------------------
`targets` | `list(map(string))` | The targets that can be used to collect metrics of instrumented services with eBPF.

For example, the `targets` can either be passed to a `discovery.relabel` component to rewrite the targets' label sets or to a `prometheus.scrape` component that collects the exposed metrics.

The exported targets use the configured [in-memory traffic][] address specified by the [run command][].

## Component health

`beyla.ebpf` is only reported as unhealthy if given an invalid configuration.

## Debug information

`beyla.ebpf` does not expose any component-specific debug information.

## Example

### Metrics

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `beyla.ebpf` of the specified port:

```alloy
beyla.ebpf "default" {
    open_port = <OPEN_PORT>
}

prometheus.scrape "beyla" {
  targets = beyla.ebpf.default.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = <PROMETHEUS_REMOTE_WRITE_URL>

    basic_auth {
      username = <USERNAME>
      password = <PASSWORD>
    }
  }
}
```

Replace the following:

- _`<OPEN_PORT>`_: The port of the running service for Beyla automatically instrumented with eBPF.
- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the remote_write API.
- _`<PASSWORD>`_: The password to use for authentication to the remote_write API.

### Traces

This example gets traces from `beyla.ebpf` and forwards them to `otlp`:

```alloy
beyla.ebpf "default" {
    open_port = <OPEN_PORT>
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
        endpoint = env("<OTLP_ENDPOINT>")
    }
}
```

Replace the following:

- _`<OPEN_PORT>`_: The port of the running service for Beyla automatically instrumented with eBPF.
- _`<OTLP_ENDPOINT>`_: The endpoint of the OpenTelemetry Collector to send traces to.

[Grafana Beyla]: https://github.com/grafana/beyla
[eBPF]: https://ebpf.io/
[routes]: #routes-block
[attributes]: #attributes-block
[kubernetes attributes]: #kubernetes-attributes-block
[kubernetes services]: #kubernetes-services-block
[discovery]: #discovery-block
[services]: #services-block
[output]: #output-block
[in-memory traffic]: ../../../../get-started/component_controller#in-memory-traffic
[run command]: ../../../cli/run/
[scrape]: ../../prometheus/prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`beyla.ebpf` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`beyla.ebpf` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
