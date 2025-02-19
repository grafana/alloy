---
canonical: https://grafana.com/docs/alloy/latest/reference/components/beyla/beyla.ebpf/
aliases:
  - ../beyla.ebpf/ # /docs/alloy/latest/reference/components/beyla.ebpf/
description: Learn about beyla.ebpf
labels:
  stage: public-preview
title: beyla.ebpf
---

# `beyla.ebpf`

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `beyla.ebpf` component is a wrapper for [Grafana Beyla][] which uses [eBPF][] to automatically inspect application executables and the OS networking layer, and capture trace spans related to web transactions and Rate Errors Duration (RED) metrics for Linux HTTP/S and gRPC services.
You can configure the component to collect telemetry data from a specific port or executable path, and other criteria from Kubernetes metadata.
The component exposes metrics that can be collected by a Prometheus scrape component, and traces that can be forwarded to an OTel exporter component.

{{< admonition type="note" >}}
To run this component, {{< param "PRODUCT_NAME" >}} requires administrative privileges, or at least it needs to be granted the `CAP_SYS_ADMIN` and `CAP_SYS_PTRACE` capability.
In Kubernetes environments, the [AppArmor profile must be `Unconfined`](https://kubernetes.io/docs/tutorials/security/apparmor/#securing-a-pod) for the Deployment or DaemonSet running {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

## Usage

```alloy
beyla.ebpf "<LABEL>" {

}
```

## Arguments

You can use the following arguments with `beyla.ebpf`:

| Name              | Type     | Description                                                                         | Default | Required |
| ----------------- | -------- | ----------------------------------------------------------------------------------- | ------- | -------- |
| `debug`           | `bool`   | Enable debug mode for Beyla.                                                        | `false` | no       |
| `executable_name` | `string` | The name of the executable to match for Beyla automatically instrumented with eBPF. | `""`    | no       |
| `open_port`       | `string` | The port of the running service for Beyla automatically instrumented with eBPF.     | `""`    | no       |
| `enforce_sys_caps`| `bool`   | Enforce system capabilities required for eBPF instrumentation.                      | `false` | no       |

`debug` enables debug mode for Beyla. This mode logs BPF logs, network logs, trace representation logs, and other debug information.

`executable_name` accepts a regular expression to be matched against the full executable command line, including the directory where the executable resides on the file system.

`open_port` accepts a comma-separated list of ports (for example, `80,443`), and port ranges (for example, `8000-8999`).
If the executable matches only one of the ports in the list, it's considered to match the selection criteria.

`enforce_sys_caps` when set to true, if the required system capabilities are not present Beyla aborts its startup and logs a list of the missing capabilities.

## Blocks

You can use the following blocks with `beyla.ebpf`:

| Block                                                                  | Description                                                                                        | Required |
| ---------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | -------- |
| [`output`][output]                                                     | Configures where to send received telemetry data.                                                  | yes      |
| [`attributes`][attributes]                                             | Configures the Beyla attributes for the component.                                                 | no       |
| `attributes` > [`kubernetes`][kubernetes attributes]                   | Configures decorating of the metrics and traces with Kubernetes metadata of the instrumented Pods. | no       |
| `attributes` > [`instance_id`][instance_id]                           | Configures instance ID settings.                                                                   | no       |
| `attributes` > [`select`][select]                                     | Configures which attributes to include or exclude for specific sections.                           | no       |
| [`discovery`][discovery]                                               | Configures the discovery for processes to instrument matching given criteria.                      | no       |
| `discovery` > [`exclude_services`][services]                           | Configures the services to exclude for the component.                                              | no       |
| `discovery` > `exclude_services` > [`kubernetes`][kubernetes services] | Configures the Kubernetes services to exclude for the component.                                   | no       |
| `discovery` > [`services`][services]                                   | Configures the services to discover for the component.                                             | no       |
| `discovery` > `services` > [`kubernetes`][kubernetes services]         | Configures the Kubernetes services to discover for the component.                                  | no       |
| [`metrics`][metrics]                                                   | Configures which metrics Beyla exposes.                                                            | no       |
| `metrics` > [`network`][network]                                       | Configures network metrics options for Beyla.                                                      | no       |
| [`routes`][routes]                                                     | Configures the routes to match HTTP paths into user-provided HTTP routes.                          | no       |
| [`ebpf`][ebpf]                                                         | Configures eBPF-specific settings.                                                                 | no       |
| [`filters`][filters]                                                   | Configures filtering of attributes.                                                                | no       |
| `filters` > [`application`][application filters]                       | Configures filtering of application attributes.                                                    | no       |
| `filters` > [`network`][network filters]                              | Configures filtering of network attributes.                                                        | no       |

The > symbol indicates deeper levels of nesting.
For example,`attributes` > `kubernetes` refers to a `kubernetes` block defined inside an `attributes` block.

### `output`

<span class="badge docs-labels__stage docs-labels__item">Required</span>

The `output` block configures a set of components to forward the resulting telemetry data to.

The following arguments are supported:

| Name     | Type                     | Description                          | Default | Required |
| -------- | ------------------------ | ------------------------------------ | ------- | -------- |
| `traces` | `list(otelcol.Consumer)` | List of consumers to send traces to. | `[]`    | no       |

You must specify the `output` block, but all its arguments are optional.
By default, telemetry data is dropped.
Configure the `traces` argument to send traces data to other components.

### `attributes`

The `attributes` block configures how some attributes for metrics and traces are decorated.

It contains the following blocks:

#### `kubernetes` attributes

This `kubernetes` block configures the decorating of the metrics and traces with Kubernetes metadata from the instrumented Pods.

| Name                     | Type          | Description                                                    | Default        | Required |
| ------------------------ | ------------- | -------------------------------------------------------------- | -------------- | -------- |
| `cluster_name`          | `string`      | The name of the Kubernetes cluster.                           | `""`           | no       |
| `enable`                | `string`      | Enable the Kubernetes metadata decoration.                     | `false`        | no       |
| `informers_sync_timeout`| `duration`    | Timeout for Kubernetes informers synchronization.             | `30s`          | no       |
| `informers_resync_period`| `duration`   | Period for Kubernetes informers resynchronization.            | `30m`          | no       |
| `disable_informers`     | `list(string)`| List of Kubernetes informers to disable.                      | `[]`           | no       |
| `meta_restrict_local_node`| `bool`      | Restrict Kubernetes metadata collection to local node.         | `false`        | no       |

If `cluster_name` isn't set, Beyla tries to detect the cluster name from the Kubernetes API.

If `enable` is set to `true`, Beyla decorates the metrics and traces with Kubernetes metadata. The following labels are added:

* `k8s.namespace.name`
* `k8s.deployment.name`
* `k8s.statefulset.name`
* `k8s.replicaset.name`
* `k8s.daemonset.name`
* `k8s.node.name`
* `k8s.pod.name`
* `k8s.pod.uid`
* `k8s.pod.start_time`

If `enable` is set to `false`, the Kubernetes metadata decorator is disabled.

If `enable` is set to `autodetect`, Beyla tries to detect if it's running inside Kubernetes, and enables the metadata decoration if that's the case.

In `disable_informers`, you can specify the Kubernetes informers to disable. The accepted value is a list that might contain `node` and `service`.

#### `instance_id`

The `instance_id` block configures instance ID settings.

| Name                | Type    | Description                                                | Default | Required |
| ------------------ | ------- | ---------------------------------------------------------- | ------- | -------- |
| `dns`              | `bool`  | Enable DNS resolution for hostname.                        | `true` | no       |
| `override_hostname`| `string`| Override the hostname used for instance identification.     | `""`    | no       |

#### `select`

The `select` block configures which attributes to include or exclude for specific metric/trace sections. Each selected attribute is defined as a labeled block with the attribute name as the label.

| Name        | Type           | Description                                                | Required |
| ----------- | -------------- | ---------------------------------------------------------- | -------- |
| `include`   | `list(string)` | List of attributes to include. Use `*` to include all.    | no       |
| `exclude`   | `list(string)` | List of attributes to exclude.                            | no       |

Example:
```alloy
select "sql_client_duration" {
    include = ["*"]
    exclude = ["db_statement"]
}
```

### `discovery`

The `discovery` block configures the discovery for processes to instrument matching given criteria.

| Name                                  | Type           | Description                                                     | Default | Required |
| ------------------------------------- | -------------- | --------------------------------------------------------------- | ------- | -------- |
| `skip_go_specific_tracers`            | `bool`         | Skip Go-specific tracers during discovery.                      | `false` | no       |
| `exclude_otel_instrumented_services`  | `bool`         | Exclude services that are already instrumented with OpenTelemetry.| `true`  | no       |

It contains the following blocks:

#### `services`

In some scenarios, Beyla instruments a wide variety of services, such as a Kubernetes DaemonSet that instruments all the services in a node.
The `services` block allows you to filter the services to instrument based on their metadata. If you specify other selectors in the same services entry,
the instrumented processes need to match all the selector properties.

| Name         | Type     | Description                                                                              | Default | Required |
| ------------ | -------- | ---------------------------------------------------------------------------------------- | ------- | -------- |
| `name`       | `string` | The name of the service to match.                                                        | `""`    | no       |
| `namespace`  | `string` | The namespace of the service to match.                                                   | `""`    | no       |
| `open_ports` | `string` | The port of the running service for Beyla automatically instrumented with eBPF.          | `""`    | no       |
| `exe_path`   | `string` | The path of the running service for Beyla automatically instrumented with eBPF.          | `""`    | no       |

`exe_path` accepts a regular expression to be matched against the full executable command line, including the directory where the executable resides on the file system.

`name` defines a name for the matching instrumented service.
It's used to populate the `service.name` OTel property or the `service_name` Prometheus property in the exported metrics/traces.

`open_port` accepts a comma-separated list of ports (for example, `80,443`), and port ranges (for example, `8000-8999`).
If the executable matches only one of the ports in the list, it's considered to match the selection criteria.

#### `default_exclude_services`

The `default_exclude_services` is special services block that disables instrumentation of Grafana Alloy. The default value for `exe_path` is `"(?:^|\/)(beyla$|alloy$|otelcol[^\/]*$)"`. 
Set to empty to allow Alloy to instrument itself as well as these other components.

#### `kubernetes` services

This `kubernetes` block filters the services to instrument based on their Kubernetes metadata. If you specify other selectors in the same services entry,
the instrumented processes need to match all the selector properties.

| Name               | Type          | Description                                                                                                 | Default | Required |
| ------------------ | ------------- | ----------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `daemonset_name`   | `string`      | Regular expression of Kubernetes DaemonSets to match.                                                       | `""`    | no       |
| `deployment_name`  | `string`      | Regular expression of Kubernetes Deployments to match.                                                      | `""`    | no       |
| `namespace`        | `string`      | Regular expression of Kubernetes Namespaces to match.                                                       | `""`    | no       |
| `owner_name`       | `string`      | Regular expression of Kubernetes owners of running Pods to match.                                           | `""`    | no       |
| `pod_labels`       | `map(string)` | Key-value pairs of labels with keys matching Kubernetes Pods with the provided value as regular expression. | `{}`    | no       |
| `pod_name`         | `string`      | Regular expression of Kubernetes Pods to match.                                                             | `""`    | no       |
| `replicaset_name`  | `string`      | Regular expression of Kubernetes ReplicaSets to match.                                                      | `""`    | no       |
| `statefulset_name` | `string`      | Regular expression of Kubernetes StatefulSets to match.                                                     | `""`    | no       |

### `metrics`

The `metrics` block configures which metrics Beyla collects.

| Name                                   | Type           | Description                                                           | Default           | Required |
| -------------------------------------- | -------------- | --------------------------------------------------------------------- | ----------------- | -------- |
| `features`                             | `list(string)` | List of features to enable for the metrics.                           | `["application"]` | no       |
| `instrumentations`                     | `list(string)` | List of instrumentations to enable for the metrics.                   | `["*"]`           | no       |
| `allow_service_graph_self_references`  | `bool`        | Allow service graph metrics to reference the same service.            | `false`           | no       |

`features` is a list of features to enable for the metrics. The following features are available:

* `application` exports application-level metrics.
* `application_process` exports metrics about the processes that run the instrumented application.
* `application_service_graph` exports application-level service graph metrics.
* `application_span` exports application-level metrics in traces span metrics format.
* `network` exports network-level metrics.

`instrumentations` is a list of instrumentations to enable for the metrics. The following instrumentations are available:

* `*` enables all `instrumentations`. If `*` is present in the list, the other values are ignored.
* `grpc` enables the collection of gRPC application metrics.
* `http` enables the collection of HTTP/HTTPS/HTTP2 application metrics.
* `kafka` enables the collection of Kafka client/server message queue metrics.
* `redis` enables the collection of Redis client/server database metrics.
* `sql` enables the collection of SQL database client call metrics.

#### `network`

The `network` block configures network metrics options for Beyla. You must append `network` to the `features` list in the `metrics` block to enable network metrics.

| Name                  | Type          | Description                                                        | Default  | Required |
| --------------------- | ------------- | ------------------------------------------------------------------ | -------- | -------- |
| `enabled`            | `bool`        | Enable network metrics collection.                                  | `false`  | no       |
| `source`             | `string`      | Linux Kernel feature used to source the network events Beyla reports.                       | `"socket_filter"`     | no       |
| `agent_ip`           | `string`      | Allows overriding the reported `beyla.ip` attribute on each metric.                                           | `""`     | no       |
| `agent_ip_iface`     | `string`      | Network interface to get agent IP from.                            | `"external"`| no       |
| `agent_ip_type`      | `string`      | Type of IP address to use.                     | `"any"` | no       |
| `interfaces`         | `list(string)`| List of network interfaces to monitor.                             | `[]`     | no       |
| `exclude_interfaces` | `list(string)`| List of network interfaces to exclude from monitoring.             | `["lo"]`     | no       |
| `protocols`          | `list(string)`| List of protocols to monitor.                | `[]`     | no       |
| `exclude_protocols`  | `list(string)`| List of protocols to exclude from monitoring.                      | `[]`     | no       |
| `cache_max_flows`    | `int`         | Maximum number of flows to cache.                                  | `5000`   | no       |
| `cache_active_timeout`| `duration`   | Timeout for active flow cache entries.                            | `5s`    | no       |
| `direction`          | `string`      | Direction of traffic to monitor.          | `"both"`     | no       |
| `sampling`           | `int`         | Sampling rate for network metrics.                                 | `0` (disabled)      | no       |
| `cidrs`             | `list(string)`| List of CIDR ranges to monitor.                                   | `[]`     | no       |

`source` can be set to `socket_filter` or `tc`.
* `socket_filter` is used as an event source, Beyla installs an eBPF Linux socket filter to capture the network events
* `tc` is used as a kernel module, Beyla uses the Linux Traffic Control ingress and egress filters to capture the network events, in a direct action mode.

`agent_ip_iface` can be set to `external` (default), `local`, or `name:<interface name>` (e.g. `name:eth0`).

`agent_ip_type` can be set to `ipv4`, `ipv6`, or `any` (default).

`protocols` and `exclude_protocols` are defined in the Linux enumeration of [Standard well-defined IP protocols](https://elixir.bootlin.com/linux/v6.8.7/source/include/uapi/linux/in.h#L28), and can be:
`TCP`, `UDP`, `IP`, `ICMP`, `IGMP`, `IPIP`, `EGP`, `PUP`, `IDP`, `TP`, `DCCP`, `IPV6`, `RSVP`, `GRE`, `ESP`, `AH`,
`MTP`, `BEETPH`, `ENCAP`, `PIM`, `COMP`, `L2TP`, `SCTP`, `UDPLITE`, `MPLS`, `ETHERNET`, `RAW`.

`direction` can be set to `ingress`, `egress`, or `both` (default).

`sampling` rate at which packets should be sampled and sent to the target collector. For example, if set to 100, one out of 100 packets, on average, are sent to the target collector.



### `routes`

The `routes` block configures the routes to match HTTP paths into user-provided HTTP routes.

| Name              | Type           | Description                                                                              | Default       | Required |
| ----------------- | -------------- | ---------------------------------------------------------------------------------------- | ------------- | -------- |
| `ignore_mode`     | `string`       | The mode to use when ignoring patterns.                                                  | `""`          | no       |
| `ignore_patterns` | `list(string)` | List of provided URL path patterns to ignore from `http.route` trace/metric property.    | `[]`          | no       |
| `patterns`        | `list(string)` | List of provided URL path patterns to set the `http.route` trace/metric property.        | `[]`          | no       |
| `unmatched`       | `string`       | Specifies what to do when a trace HTTP path doesn't match any of the `patterns` entries. | `"heuristic"` | no       |
| `wildcard_char`   | `string`       | Character to use as wildcard in patterns.                                                | `"*"`         | no       |

`ignore_mode` properties are:

* `all` discards metrics and traces matching the `ignored_patterns`.
* `metrics` discards only the metrics that match the `ignored_patterns`. No trace events are ignored.
* `traces` discards only the traces that match the `ignored_patterns`. No metric events are ignored.

`patterns` and `ignore_patterns` are a list of patterns which a URL path with specific tags which allow for grouping path segments (or ignored them).
The matcher tags can be in the `:name` or `{name}` format.

`unmatched` properties are:

* `heuristic` automatically derives the `http.route` field property from the path value based on the following rules:
  * Any path components that have numbers or characters outside of the ASCII alphabet (or `-` and _), are replaced by an asterisk `*`.
  * Any alphabetical components that don't look like words are replaced by an asterisk `*`.
* `path` copies the `http.route` field property to the path value.
  {{< admonition type="caution" >}}
  This property could lead to a cardinality explosion on the ingester side.
  {{< /admonition >}}
* `unset` leaves the `http.route` property as unset.
* `wildcard` sets the `http.route` field property to a generic asterisk-based `/**` value.

### `ebpf`

The `ebpf` block configures eBPF-specific settings.

| Name                          | Type          | Description                                                                | Default | Required |
| ----------------------------- | ------------- | -------------------------------------------------------------------------- | ------- | -------- |
| `wakeup_len`                  | `int`         | Number of messages to accumulate before wakeup request.                    | `""`    | no       |
| `track_request_headers`       | `bool`        | Enable tracking of request headers for Traceparent fields.                 | `false` | no       |
| `http_request_timeout`        | `duration`    | Timeout for HTTP requests.                                                 | `30s`   | no       |
| `enable_context_propagation`  | `bool`        | Enable context propagation using Linux Traffic Control probes.             | `false` | no       |
| `high_request_volume`         | `bool`        | Optimize for immediate request information when response is seen.          | `false` | no       |
| `heuristic_sql_detect`        | `bool`        | Enable heuristic-based detection of SQL requests.                         | `false` | no       |

### `filters`

The `filters` block configures filtering of attributes.

It contains the following blocks:

#### `application`

The `application` block configures filtering of application attributes. Each attribute filter is defined as a labeled block with the attribute name as the label.

| Name        | Type     | Description                                          | Required |
| ----------- | -------- | ---------------------------------------------------- | -------- |
| `match`     | `string` | String to match attribute values.        | no       |
| `not_match` | `string` | String to exclude matching values.       | no       |


Both properties accept a
[glob-like](https://github.com/gobwas/glob) string (it can be a full value or include
wildcards).

#### `network`

The `network` block configures filtering of network attributes. Each attribute filter is defined as a labeled block with the attribute name as the label.

| Name        | Type     | Description                                          | Required |
| ----------- | -------- | ---------------------------------------------------- | -------- |
| `match`     | `string` | String to match attribute values.        | no       |
| `not_match` | `string` | String to exclude matching values.       | no       |

Both properties accept a
[glob-like](https://github.com/gobwas/glob) string (it can be a full value or include
wildcards).

## Exported fields

The following fields are exported and can be referenced by other components.

| Name      | Type                | Description                                                                         |
| --------- | ------------------- | ----------------------------------------------------------------------------------- |
| `targets` | `list(map(string))` | The targets that can be used to collect metrics of instrumented services with eBPF. |

For example, the `targets` can either be passed to a `discovery.relabel` component to rewrite the targets' label sets or to a `prometheus.scrape` component that collects the exposed metrics.

The exported targets use the configured [in-memory traffic][] address specified by the [run command][].

## Component health

`beyla.ebpf` is only reported as unhealthy if given an invalid configuration.

## Debug information

`beyla.ebpf` doesn't expose any component-specific debug information.

## Examples

The following examples show you how to collect metrics and traces from `beyla.ebpf`.

### Metrics

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `beyla.ebpf` of the specified port:

```alloy
beyla.ebpf "default" {
    open_port = <OPEN_PORT>
}

prometheus.scrape "beyla" {
  targets = beyla.ebpf.default.targets
  honor_labels = true // required to keep job and instance labels
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

* _`<OPEN_PORT>`_: The port of the running service for Beyla automatically instrumented with eBPF.
* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

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
        endpoint = sys.env("<OTLP_ENDPOINT>")
    }
}
```

Replace the following:

* _`<OPEN_PORT>`_: The port of the running service for Beyla automatically instrumented with eBPF.
* _`<OTLP_ENDPOINT>`_: The endpoint of the OpenTelemetry Collector to send traces to.

[Grafana Beyla]: https://github.com/grafana/beyla
[eBPF]: https://ebpf.io/
[routes]: #routes
[attributes]: #attributes
[kubernetes attributes]: #kubernetes-attributes
[kubernetes services]: #kubernetes-services
[discovery]: #discovery
[services]: #services
[metrics]: #metrics
[network]: #network
[output]: #output
[in-memory traffic]: ../../../../get-started/component_controller/#in-memory-traffic
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