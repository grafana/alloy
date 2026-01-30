---
canonical: https://grafana.com/docs/alloy/latest/reference/components/beyla/beyla.ebpf/
aliases:
  - ../beyla.ebpf/ # /docs/alloy/latest/reference/components/beyla.ebpf/
description: Learn about beyla.ebpf
labels:
  stage: general-availability
  products:
    - oss
title: beyla.ebpf
---

# `beyla.ebpf`

{{< admonition type="note" >}}
The `beyla.ebpf` component uses Grafana Beyla version {{< param "BEYLA_VERSION" >}}.
{{< /admonition >}}

The `beyla.ebpf` component is a wrapper for [Grafana Beyla][] which uses [eBPF][eBPF website] to automatically inspect application executables and the OS networking layer, and capture trace spans related to web transactions and Rate Errors Duration (RED) metrics for Linux HTTP/S and gRPC services.
You can configure the component to collect telemetry data from a specific port or executable path, and other criteria from Kubernetes metadata.
The component exposes metrics that can be collected by a Prometheus scrape component, and traces that can be forwarded to an OTel exporter component.

{{< admonition type="note" >}}
To run this component, {{< param "PRODUCT_NAME" >}} requires administrative privileges, or at least it needs to be granted the following capabilities: `BPF`, `SYS_PTRACE`, `NET_RAW`, `CAP_CHECKPOINT_RESTORE`, `DAC_READ_SEARCH`, and `PERFMON`.
The number of required capabilities depends on the specific use case.
Refer to the [Beyla capabilities](https://grafana.com/docs/beyla/latest/security/#list-of-capabilities-required-by-beyla) for more information.

In Kubernetes environments, the [AppArmor profile must be `Unconfined`](https://kubernetes.io/docs/tutorials/security/apparmor/#securing-a-pod) for the Deployment or DaemonSet running {{< param "PRODUCT_NAME" >}}.
You must also set the `hostPID` flag to `true` in the Pod spec so that in can access all the processes running on the host.
{{< /admonition >}}

## Usage

```alloy
beyla.ebpf "<LABEL>" {

}
```

## Arguments

You can use the following arguments with `beyla.ebpf`:

| Name               | Type     | Description                                                    | Default      | Required |
|--------------------|----------|----------------------------------------------------------------|--------------|----------|
| `debug`            | `bool`   | Enable debug mode for Beyla.                                   | `false`      | no       |
| `enforce_sys_caps` | `bool`   | Enforce system capabilities required for eBPF instrumentation. | `false`      | no       |
| `trace_printer`    | `string` | Format for printing trace information.                         | `"disabled"` | no       |


`debug` enables debug mode for Beyla. This mode logs BPF logs, network logs, trace representation logs, and other debug information.

When `enforce_sys_caps`  is set to true and the required system capabilities aren't present, Beyla aborts its startup and logs a list of the missing capabilities.

`trace_printer` is used to print the trace information in a specific format.
The following formats are supported:

* `disabled`: Disables trace printing.
* `counter`: Prints the trace information in a counter format.
* `text`: Prints the trace information in a text format.
* `json`: Prints the trace information in a JSON format.
* `json_indent`: Prints the trace information in a JSON format with indentation.

## Blocks

You can use the following blocks with `beyla.ebpf`:

| Block                                                                  | Description                                                                                        | Required |
|------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------|----------|
| [`output`][output]                                                     | Configures where to send received telemetry data.                                                  | yes      |
| [`attributes`][attributes]                                             | Configures the Beyla attributes for the component.                                                 | no       |
| `attributes` > [`kubernetes`][kubernetes attributes]                   | Configures decorating of the metrics and traces with Kubernetes metadata of the instrumented Pods. | no       |
| `attributes` > [`instance_id`][instance_id]                            | Configures instance ID settings.                                                                   | no       |
| `attributes` > [`select`][select]                                      | Configures which attributes to include or exclude for specific sections.                           | no       |
| [`discovery`][discovery]                                               | Configures the discovery for processes to instrument matching given criteria.                      | no       |
| `discovery` > [`instrument`][services]                                 | Configures the services to discover and instrument for the component.                              | no       |
| `discovery` > `instrument` > [`kubernetes`][kubernetes services]       | Configures the Kubernetes services to discover and instrument for the component.                   | no       |
| `discovery` > `instrument` > [`sampler`][sampler]                      | Configures trace sampling for the service.                                                         | no       |
| `discovery` > [`exclude_instrument`][services]                         | Configures the services to exclude from instrumentation for the component.                         | no       |
| `discovery` > `exclude_instrument` > [`kubernetes`][kubernetes services] | Configures the Kubernetes services to exclude from instrumentation for the component.             | no       |
| `discovery` > [`default_exclude_instrument`][services]                 | Configures the default services to exclude from instrumentation for the component.                 | no       |
| `discovery` > `default_exclude_instrument` > [`kubernetes`][kubernetes services] | Configures the default Kubernetes services to exclude from instrumentation for the component.     | no       |
| `discovery` > [`survey`][services]                                     | Configures the surveying mechanism for the component.                                              | no       |
| `discovery` > `survey` > [`kubernetes`][kubernetes services]           | Configures the Kubernetes surveying mechanism for the component.                                   | no       |
| [`ebpf`][ebpf]                                                         | Configures eBPF-specific settings.                                                                 | no       |
| [`filters`][filters]                                                   | Configures filtering of attributes.                                                                | no       |
| `filters` > [`application`][application filters]                       | Configures filtering of application attributes.                                                    | no       |
| `filters` > [`network`][network filters]                               | Configures filtering of network attributes.                                                        | no       |
| [`metrics`][metrics]                                                   | Configures which metrics Beyla exposes.                                                            | no       |
| `metrics` > [`network`][network metrics]                               | Configures network metrics options for Beyla.                                                      | no       |
| [`traces`][traces]                                                     | Configures trace collection and sampling options for all services instrumented by the component.   | no       |
| `traces` > [`sampler`][sampler]                                        | Configures global trace sampling settings                                                          | no       |
| [`routes`][routes]                                                     | Configures the routes to match HTTP paths into user-provided HTTP routes.                          | no       |

The > symbol indicates deeper levels of nesting.
For example, `attributes` > `kubernetes` refers to a `kubernetes` block defined inside an `attributes` block.

[routes]: #routes
[traces]: #traces
[attributes]: #attributes
[kubernetes attributes]: #kubernetes-attributes
[kubernetes services]: #kubernetes-services
[discovery]: #discovery
[services]: #services
[sampler]: #sampler
[instance_id]: #instance_id
[select]: #select
[ebpf]: #ebpf
[filters]: #filters
[application filters]: #application
[metrics]: #metrics
[network metrics]: #network-metrics
[network filters]: #network-filters
[output]: #output

### `output`

{{< badge text="Required" >}}

The `output` block configures a set of components to forward the resulting telemetry data to.

The following arguments are supported:

| Name     | Type                     | Description                          | Default | Required |
|----------|--------------------------|--------------------------------------|---------|----------|
| `traces` | `list(otelcol.Consumer)` | List of consumers to send traces to. | `[]`    | no       |

You must specify the `output` block, but all its arguments are optional.
By default, telemetry data is dropped.
Configure the `traces` argument to send traces data to other components.

### `attributes`

The `attributes` block configures how some attributes for metrics and traces are decorated.

It contains the following blocks:

#### `kubernetes` attributes

This `kubernetes` block configures the decorating of the metrics and traces with Kubernetes metadata from the instrumented Pods.

| Name                       | Type           | Description                                            | Default | Required |
|----------------------------|----------------|--------------------------------------------------------|---------|----------|
| `cluster_name`             | `string`       | The name of the Kubernetes cluster.                    | `""`    | no       |
| `disable_informers`        | `list(string)` | List of Kubernetes informers to disable.               | `[]`    | no       |
| `enable`                   | `string`       | Enable the Kubernetes metadata decoration.             | `autodetect` | no       |
| `informers_resync_period`  | `duration`     | Period for Kubernetes informers resynchronization.     | `"30m"` | no       |
| `informers_sync_timeout`   | `duration`     | Timeout for Kubernetes informers synchronization.      | `"30s"` | no       |
| `meta_cache_address`       | `string`       | Address of the Kubernetes metadata cache service.      | `""`    | no       |
| `meta_restrict_local_node` | `bool`         | Restrict Kubernetes metadata collection to local node. | `false` | no       |

If `cluster_name` isn't set, Beyla tries to detect the cluster name from the Kubernetes API.

If `enable` is set to `true`, Beyla decorates the metrics and traces with Kubernetes metadata.
The following labels are added:

* `k8s.daemonset.name`
* `k8s.deployment.name`
* `k8s.namespace.name`
* `k8s.node.name`
* `k8s.pod.name`
* `k8s.pod.start_time`
* `k8s.pod.uid`
* `k8s.replicaset.name`
* `k8s.statefulset.name`

If `enable` is set to `false`, the Kubernetes metadata decorator is disabled.

If `enable` is set to `autodetect`, Beyla tries to detect if it's running inside Kubernetes, and enables the metadata decoration if that's the case.

In `disable_informers`, you can specify the Kubernetes informers to disable. The accepted value is a list that might contain `node` and `service`.

#### `instance_id`

The `instance_id` block configures instance ID settings.

| Name                | Type     | Description                                             | Default | Required |
|---------------------|----------|---------------------------------------------------------|---------|----------|
| `dns`               | `bool`   | Enable DNS resolution for hostname.                     | `true`  | no       |
| `override_hostname` | `string` | Override the hostname used for instance identification. | `""`    | no       |

#### `select`

The `select` block configures which attributes to include or exclude for specific metric/trace sections.

| Name      | Type           | Description                                            | Default | Required |
|-----------|----------------|--------------------------------------------------------|---------|----------|
| `attr`    | `string`       | The attribute name to select.                          | `[]`    | yes      |
| `exclude` | `list(string)` | List of attributes to exclude.                         | `[]`    | no       |
| `include` | `list(string)` | List of attributes to include. Use `*` to include all. | `[]`    | no       |

`include` is a list of attributes that need to be reported.
Each attribute can be an attribute name or a wildcard, for example, `k8s.dst.*` to include all the attributes starting with `k8s.dst`.

`exclude` is a list to of attribute names/wildcards containing the attributes to remove from the `include` list, or from the default attribute set.

The following example shows how you can include and exclude specific attributes:

```alloy
beyla.ebpf "default" {
attributes {
    select {
        attr = "sql_client_duration"
        include = ["*"]
        exclude = ["db_statement"]
    }
  }
}
```

Additionally, you can use `*` wildcards as metric names to add and exclude attributes for groups of metrics having the same name.
For example:

```alloy
beyla.ebpf "default" {
  attributes {
    select {
        attr = "http_*"
        include = ["*"]
        exclude = ["http_path", "http_route"]
    }
    select {
        attr = "http_client_*"
        // override http_* exclusion
        include = ["http_path"]
    }
  }
}
```

In the previous example, all the metrics with a name starting with `http_` or `http.` would include all the possible attributes but `http_path` and `http_route` or `http.path` and `http.route`.
The `http_client_*` section would override the base configuration, enabling the `http_path` attribute for the HTTP client metrics and `http_route` for the HTTP server metrics.

### `discovery`

The `discovery` block configures the discovery for processes to instrument matching given criteria.

| Name                                 | Type   | Description                                                        | Default | Required |
|--------------------------------------|--------|--------------------------------------------------------------------|---------|----------|
| `exclude_otel_instrumented_services` | `bool` | Exclude services that are already instrumented with OpenTelemetry. | `true`  | no       |
| `skip_go_specific_tracers`           | `bool` | Skip Go-specific tracers during discovery.                         | `false` | no       |

It contains the following blocks:

#### `instrument`

The `instrument` block configures the services to discover and instrument using [glob patterns](https://github.com/gobwas/glob).

| Name              | Type           | Description                                                                     | Default | Required |
|-------------------|----------------|---------------------------------------------------------------------------------|---------|----------|
| `name`            | `string`       | The name of the service to match.                                               | `""`    | no       |
| `namespace`       | `string`       | The namespace of the service to match.                                          | `""`    | no       |
| `open_ports`      | `string`       | The port of the running service for Beyla automatically instrumented with eBPF. | `""`    | no       |
| `exe_path`        | `string`       | The path of the running service for Beyla automatically instrumented with eBPF. | `""`    | no       |
| `containers_only` | `bool`         | Restrict the discovery to processes which are running inside a container.       | `false` | no       |
| `exports`         | `list(string)` | Export modes for the service. Valid values: `"metrics"`, `"traces"`.            | `[]`    | no       |

`exe_path` accepts a glob pattern to be matched against the full executable command line, including the directory where the executable resides on the file system.
Common glob patterns include `*` (matches any sequence of characters) and `?` (matches any single character).

`name` defines a name for the matching instrumented service.
It's used to populate the `service.name` OTel property or the `service_name` Prometheus property in the exported metrics/traces.

`open_port` accepts a comma-separated list of ports (for example, `80,443`), and port ranges (for example, `8000-8999`).
If the executable matches only one of the ports in the list, it's considered to match the selection criteria.

`exports` specifies what types of telemetry data to export for the matching service.
You can specify `"metrics"`, `"traces"`, or both.
If empty, the service will export both metrics and traces by default.

#### `exclude_instrument`

The `exclude_instrument` block configures services to exclude from instrumentation using glob patterns.
Services matching these criteria won't be instrumented even if they match the `instrument` selection.

The `exclude_instrument` block uses the same configuration options as the `instrument` block.

#### `default_exclude_instrument`

The `default_exclude_instrument` block disables instrumentation of Grafana Alloy and related components by default.
The default value for `exe_path` uses a glob pattern that matches `beyla`, `alloy`, and `otelcol*` executables.
Set to empty to allow Alloy to instrument itself as well as these other components.

#### `survey`

The `survey` block configures services for discovery without instrumentation using glob patterns.
Instead of instrumenting matching services, the component will only emit a `survey_info` metric for each discovered service.
This can be helpful for informing external applications of the services available for instrumentation.

The `survey` block uses the same configuration options as the `instrument` block.

#### `kubernetes` services

This `kubernetes` block filters the services to instrument based on their Kubernetes metadata. If you specify other selectors in the same services entry,
the instrumented processes need to match all the selector properties.

When used with `instrument`, `exclude_instrument`, `default_exclude_instrument`, or `survey` blocks, the patterns use glob syntax.

| Name               | Type          | Description                                                                                                        | Default | Required |
|--------------------|---------------|--------------------------------------------------------------------------------------------------------------------|---------|----------|
| `daemonset_name`   | `string`      | Pattern to match Kubernetes DaemonSets.                                                              | `""`    | no       |
| `deployment_name`  | `string`      | Pattern to match Kubernetes Deployments.                                                             | `""`    | no       |
| `namespace`        | `string`      | Pattern to match Kubernetes Namespaces.                                                              | `""`    | no       |
| `owner_name`       | `string`      | Pattern to match Kubernetes owners of running Pods.                                                  | `""`    | no       |
| `pod_labels`       | `map(string)` | Key-value pairs of labels with keys matching Kubernetes Pods with the provided value as pattern.        | `{}`    | no       |
| `pod_annotations`  | `map(string)` | Key-value pairs of labels with keys matching Kubernetes annotations with the provided value as pattern. | `{}`    | no       |
| `pod_name`         | `string`      | Pattern to match Kubernetes Pods.                                                                    | `""`    | no       |
| `replicaset_name`  | `string`      | Pattern to match Kubernetes ReplicaSets.                                                             | `""`    | no       |
| `statefulset_name` | `string`      | Pattern to match Kubernetes StatefulSets.                                                            | `""`    | no       |

Example:

``` alloy
beyla.ebpf "default" {
  discovery {
    // Instrument all services with 8080 open port
    instrument {
      open_ports = "8080"
    }
    // Instrument all services from the default namespace
    instrument {
      kubernetes {
        namespace = "default"
      }
    }
    // Exclude all services from the kube-system namespace
    exclude_instrument {
      kubernetes {
        namespace = "kube-system"
      }
    }
  }
}
```

### `traces`

The `traces` block configures trace collection and sampling options for the beyla.ebpf component.

{{< admonition type="note" >}}
To export traces, you must also configure the [`output`][output] block with a `traces` destination.
Without an output configuration, traces are collected but not exported.
{{< /admonition >}}

| Name              | Type           | Description                                                      | Default | Required |
|-------------------|----------------|------------------------------------------------------------------|---------|----------|
| `instrumentations` | `list(string)` | List of instrumentations to enable for trace collection.        | `["*"]` | no       |


The supported values for `instrumentations` are:

* `*`: Enables all `instrumentations`. If `*` is present in the list, the other values are ignored.
* `grpc`: Enables the collection of gRPC traces.
* `gpu`: Enables the collection of GPU performance traces.
* `http`: Enables the collection of HTTP/HTTPS/HTTP2 traces.
* `kafka`: Enables the collection of Kafka client/server traces.
* `mongo`: Enables the collection of MongoDB database traces.
* `redis`: Enables the collection of Redis client/server database traces.
* `sql`: Enables the collection of SQL database client call traces.

Example:

```alloy
beyla.ebpf "default" {
  traces {
    instrumentations = ["http", "grpc", "sql"]
    sampler {
      name = "traceidratio"
      arg = "0.1"  // Global 10% sampling rate for all traces
    }
  }
  output {
    traces = [otelcol.processor.batch.default.input]
  }
}
```

For per-service sampling configuration, use the `sampler` block within the `discovery` > `services` section instead.

### `sampler`

The `sampler` block configures trace sampling settings. This block can be used in two contexts:

1. **Per-service sampling** - as a sub-block of `discovery` > `services` to configure sampling for individual discovered services
1. **Global sampling** - as a sub-block of `traces` to configure sampling for all traces collected by the component

The following arguments are supported: 

| Name   | Type     | Description                               | Default | Required |
|--------|----------|-------------------------------------------|---------|----------|
| `arg`  | `string` | The argument for the sampling strategy.   | `""`    | no       |
| `name` | `string` | The name of the sampling strategy to use. | `""`    | no       |

The supported values for `name` are:

* `traceidratio`: Samples traces based on a ratio of trace IDs. The `arg` must be a decimal value between 0 and 1. For example, `"0.1"` for 10% sampling.
* `always_on`: Always samples traces. No `arg` required.
* `always_off`: Never samples traces. No `arg` required.
* `parentbased_always_on`: Uses parent-based sampling that always samples when there's no parent span. This is the default behavior.
* `parentbased_always_off`: Uses parent-based sampling that never samples when there's no parent span.
* `parentbased_traceidratio`: Uses parent-based sampling with trace ID ratio-based sampling for root spans. The `arg` must be a decimal value between 0 and 1.

#### Examples

Per-service sampling (configured within `discovery` > `instrument`):

```alloy
beyla.ebpf "default" {
  discovery {
    instrument {
      open_ports = "8080"
      sampler {
        name = "traceidratio"
        arg = "0.1"  // 10% sampling rate for this specific service
      }
    }
  }
}
```

Global sampling (configured within `traces`):

```alloy
beyla.ebpf "default" {
  traces {
    instrumentations = ["http", "grpc", "sql"]
    sampler {
      name = "traceidratio"
      arg = "0.1"  // Global 10% sampling rate for all traces
    }
  }
  output {
    traces = [otelcol.processor.batch.default.input]
  }
}
```

### `ebpf`

The `ebpf` block configures eBPF-specific settings.

| Name                    | Type       | Description                                                                   | Default      | Required |
|-------------------------|------------|-------------------------------------------------------------------------------|--------------|----------|
| `wakeup_len`            | `int`      | Number of messages to accumulate before wakeup request.                       | `""`         | no       |
| `track_request_headers` | `bool`     | Enable tracking of request headers for Traceparent fields.                    | `false`      | no       |
| `http_request_timeout`  | `duration` | Timeout for HTTP requests.                                                    | `"30s"`      | no       |
| `context_propagation`   | `string`   | Enables injecting of the Traceparent header value for outgoing HTTP requests. | `"disabled"` | no       |
| `high_request_volume`   | `bool`     | Optimize for immediate request information when response is seen.             | `false`      | no       |
| `heuristic_sql_detect`  | `bool`     | Enable heuristic-based detection of SQL requests.                             | `false`      | no       |


#### `context_propagation`

`context_propagation` allows Beyla to propagate any incoming context to downstream services. 
This context propagation support works for any programming language.

For TLS encrypted HTTP requests (HTTPS), the Traceparent header value is encoded at TCP/IP packet level, 
and requires that Beyla is present on both sides of the communication.

The TCP/IP packet level encoding uses Linux Traffic Control (TC). 
eBPF programs that also use TC need to chain correctly with Beyla. 
For more information about chaining programs, refer to the [Cilium compatibility][cilium] documentation.

You can disable the TCP/IP level encoding and TC programs by setting `context_propagation` to `"headers"`. 
This context propagation support is fully compatible with any OpenTelemetry distributed tracing library.

`context_propagation` can be set to either one of the following values:

* `all`: Enable both HTTP and IP options context propagation.
* `headers`: Enable context propagation via the HTTP headers only.
* `ip`: Enable context propagation via the IP options field only.
* `disabled`: Disable trace context propagation.

[cilium]: https://grafana.com/docs/beyla/latest/cilium-compatibility/

### `filters`

The `filters` block allows you to filter both application and network metrics by attribute values.

For a list of metrics under the application and network family, as well as their attributes, refer to the [Beyla exported metrics][].

It contains the following blocks:

#### `application`

The `application` block configures filtering of application attributes.

| Name        | Type     | Description                         | Required |
|-------------|----------|-------------------------------------|----------|
| `attr`      | `string` | The name of the attribute to match. | yes      |
| `match`     | `string` | String to match attribute values.   | no       |
| `not_match` | `string` | String to exclude matching values.  | no       |

Both properties accept a
[glob-like](https://github.com/gobwas/glob) string (it can be a full value or include
wildcards).

#### `network` filters

The `network` block configures filtering of network attributes.

| Name        | Type     | Description                         | Required |
|-------------|----------|-------------------------------------|----------|
| `attr`      | `string` | The name of the attribute to match. | yes      |
| `match`     | `string` | String to match attribute values.   | no       |
| `not_match` | `string` | String to exclude matching values.  | no       |

Both properties accept a
[glob-like](https://github.com/gobwas/glob) string (it can be a full value or include
wildcards).

Example:

```alloy
beyla.ebpf "default" {
  filters {
    application {
      attr = "url.path"
      match = "/user/*"
    }
    network {
      attr = "k8s.src.owner.name"
      match = "*"
    }
  }
}
```

### `metrics`

The `metrics` block configures which metrics Beyla collects.

| Name                                  | Type           | Description                                                | Default           | Required |
|---------------------------------------|----------------|------------------------------------------------------------|-------------------|----------|
| `allow_service_graph_self_references` | `bool`         | Allow service graph metrics to reference the same service. | `false`           | no       |
| `features`                            | `list(string)` | List of features to enable for the metrics.                | `["application"]` | no       |
| `instrumentations`                    | `list(string)` | List of instrumentations to enable for the metrics.        | `["*"]`           | no       |
| `extra_resource_labels`               | `list(string)` | List of OTEL resource labels to include on `target_info`.  | `[]`              | no       |
| `extra_span_resource_labels`          | `list(string)` | List of OTEL resource labels to include on span metrics.   | `["k8s.cluster.name", "k8s.namespace.name", "service.version", "deployment.environment"]`           | no       |

`features` is a list of features to enable for the metrics. The following features are available:

* `application` exports application-level metrics.
* `application_process` exports metrics about the processes that run the instrumented application.
* `application_service_graph` exports application-level service graph metrics.
* `application_span` exports application-level metrics in traces span metrics format.
* `application_span_otel` exports OpenTelemetry-compatible span metrics.
* `application_span_sizes` exports span size metrics for trace analysis.
* `application_host` exports application-level host metrics for host-based pricing.
* `network` exports network-level metrics.
* `network_inter_zone` exports network-level inter-zone metrics.

`instrumentations` is a list of instrumentations to enable for the metrics. The following instrumentations are available:

* `*` enables all `instrumentations`. If `*` is present in the list, the other values are ignored.
* `grpc` enables the collection of gRPC application metrics.
* `gpu` enables the collection of GPU performance metrics.
* `http` enables the collection of HTTP/HTTPS/HTTP2 application metrics.
* `kafka` enables the collection of Kafka client/server message queue metrics.
* `mongo` enables the collection of MongoDB database metrics.
* `redis` enables the collection of Redis client/server database metrics.
* `sql` enables the collection of SQL database client call metrics.

`extra_resource_labels` is a list of OTEL resource labels, supplied through the `OTEL_RESOURCE_ATTRIBUTES` environment variable 
on the service, that you want to include on the `target_info` metric.

`extra_span_resource_labels` is a list of OTEL resource labels, supplied through the `OTEL_RESOURCE_ATTRIBUTES` environment variable 
on the service, that you want to include on the span metrics. The default list includes:

* `k8s.cluster.name`
* `k8s.namespace.name` 
* `service.version`
* `deployment.environment`

The default list of `extra_span_resource_labels` is set to match the defaults chosen by Application Observability plugin in 
Grafana Cloud.

#### `network` metrics

The `network` block configures network metrics options for Beyla. You must append `network` to the `features` list in the `metrics` block to enable network metrics.

| Name                   | Type           | Description                                                           | Default           | Required |
|------------------------|----------------|-----------------------------------------------------------------------|-------------------|----------|
| `agent_ip_iface`       | `string`       | Network interface to get agent IP from.                               | `"external"`      | no       |
| `agent_ip_type`        | `string`       | Type of IP address to use.                                            | `"any"`           | no       |
| `agent_ip`             | `string`       | Allows overriding the reported `beyla.ip` attribute on each metric.   | `""`              | no       |
| `cache_active_timeout` | `duration`     | Timeout for active flow cache entries.                                | `"5s"`            | no       |
| `cache_max_flows`      | `int`          | Maximum number of flows to cache.                                     | `5000`            | no       |
| `cidrs`                | `list(string)` | List of CIDR ranges to monitor.                                       | `[]`              | no       |
| `direction`            | `string`       | Direction of traffic to monitor.                                      | `"both"`          | no       |
| `exclude_interfaces`   | `list(string)` | List of network interfaces to exclude from monitoring.                | `["lo"]`          | no       |
| `exclude_protocols`    | `list(string)` | List of protocols to exclude from monitoring.                         | `[]`              | no       |
| `interfaces`           | `list(string)` | List of network interfaces to monitor.                                | `[]`              | no       |
| `protocols`            | `list(string)` | List of protocols to monitor.                                         | `[]`              | no       |
| `sampling`             | `int`          | Sampling rate for network metrics.                                    | `0` (disabled)    | no       |
| `source`               | `string`       | Linux Kernel feature used to source the network events Beyla reports. | `"socket_filter"` | no       |

You can set `source` to `socket_filter` or `tc`.

* `socket_filter` is used as an event source.
   Beyla installs an eBPF Linux socket filter to capture the network events.
* `tc` is used as a kernel module.
   Beyla uses the Linux Traffic Control ingress and egress filters to capture the network events, in a direct action mode.

You can set `agent_ip_iface` to `external` (default), `local`, or `name:<interface name>`, for example `name:eth0`.

You can set `agent_ip_type` to `ipv4`, `ipv6`, or `any` (default).

`protocols` and `exclude_protocols` are defined in the Linux enumeration of [Standard well-defined IP protocols](https://elixir.bootlin.com/linux/v6.8.7/source/include/uapi/linux/in.h#L28), and can be:

{{< column-list >}}

* `AH`
* `BEETPH`
* `COMP`
* `DCCP`
* `EGP`
* `ENCAP`
* `ESP`
* `ETHERNET`
* `GRE`
* `ICMP`
* `IDP`
* `IGMP`
* `IP`
* `IPIP`
* `IPV6`
* `L2TP`
* `MPLS`
* `MTP`
* `PIM`
* `PUP`
* `RAW`
* `RSVP`
* `SCTP`
* `TCP`
* `TP`
* `UDP`
* `UDPLITE`

{{< /column-list >}}

You can set `direction` to `ingress`, `egress`, or `both` (default).

`sampling` defines the rate at which packets should be sampled and sent to the target collector. For example, if you set it to 100, one out of 100 packets, on average, are sent to the target collector.

### `routes`

The `routes` block configures the routes to match HTTP paths into user-provided HTTP routes.

| Name                           | Type           | Description                                                                              | Default       | Required |
|--------------------------------|----------------|------------------------------------------------------------------------------------------|---------------|----------|
| `ignore_mode`                  | `string`       | The mode to use when ignoring patterns.                                                  | `""`          | no       |
| `ignored_patterns`             | `list(string)` | List of provided URL path patterns to ignore from `http.route` trace/metric property.    | `[]`          | no       |
| `patterns`                     | `list(string)` | List of provided URL path patterns to set the `http.route` trace/metric property.        | `[]`          | no       |
| `unmatched`                    | `string`       | Specifies what to do when a trace HTTP path doesn't match any of the `patterns` entries. | `"heuristic"` | no       |
| `wildcard_char`                | `string`       | Character to use as wildcard in patterns.                                                | `"*"`         | no       |
| `max_path_segment_cardinality` | `number`       | Maximum allowed path segment cardinality (per service) for the heuristic matcher.        | `10`          | no       |

`ignore_mode` properties are:

* `all` discards metrics and traces matching the `ignored_patterns`.
* `metrics` discards only the metrics that match the `ignored_patterns`. No trace events are ignored.
* `traces` discards only the traces that match the `ignored_patterns`. No metric events are ignored.

`patterns` and `ignored_patterns` are a list of patterns which a URL path with specific tags which allow for grouping path segments (or ignored them).
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

## Exported fields

The following fields are exported and can be referenced by other components.

| Name      | Type                | Description                                                                         |
|-----------|---------------------|-------------------------------------------------------------------------------------|
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
  discovery {
    instrument {
      open_ports = <OPEN_PORT>
    }
  }

  metrics {
    features = [
     "application", 
    ]
  }
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

#### Kubernetes

This example gets metrics from `beyla.ebpf` for the specified namespace and Pods running in a Kubernetes cluster:

```alloy
beyla.ebpf "default" {
  discovery {
    instrument {
     kubernetes {
      namespace = "<NAMESPACE>"
      pod_name = "<POD_NAME>"
     }
    }
  }
  metrics {
    features = [
     "application", 
    ]
  }
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
* _`<NAMESPACE>`_: The namespaces of the applications running in a Kubernetes cluster.
* _`<POD_NAME>`_: The name of the Pods running in a Kubernetes cluster.
* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Traces

This example gets traces from `beyla.ebpf` and forwards them to `otlp`:

```alloy
beyla.ebpf "default" {
  discovery {
    instrument {
      open_ports = <OPEN_PORT>
    }
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
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

Replace the following:

* _`<OPEN_PORT>`_: The port of the running service for Beyla automatically instrumented with eBPF.
* _`<OTLP_ENDPOINT>`_: The endpoint of the OpenTelemetry Collector to send traces to.

[Grafana Beyla]: https://github.com/grafana/beyla
[eBPF website]: https://ebpf.io/
[in-memory traffic]: ../../../../get-started/component_controller/#in-memory-traffic
[run command]: ../../../cli/run/
[scrape]: ../../prometheus/prometheus.scrape/
[Distributed traces with Beyla]: /docs/beyla/latest/distributed-traces/
[Beyla exported metrics]: /docs/beyla/latest/metrics/

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
