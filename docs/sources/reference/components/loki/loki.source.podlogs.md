---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.podlogs/
aliases:
  - ../loki.source.podlogs/ # /docs/alloy/latest/reference/components/loki.source.podlogs/
description: Learn about loki.source.podlogs
labels:
  stage: general-availability
  products:
    - oss
title: loki.source.podlogs
---

# `loki.source.podlogs`

`loki.source.podlogs` discovers `PodLogs` resources on Kubernetes.
The `PodLogs` resources provide rules for which Kubernetes Pods to discover on your cluster.

`loki.source.podlogs` uses the Kubernetes API to tail the logs from the discovered Kubernetes Pods.

`loki.source.podlogs` is similar to `loki.source.kubernetes`, but uses custom resources rather than being fed targets from another component.

{{< admonition type="note" >}}
Because `loki.source.podlogs` uses the Kubernetes API to tail logs, it uses more network traffic and CPU consumption of Kubelets than `loki.source.file`.
{{< /admonition >}}

You can specify multiple `loki.source.podlogs` components by giving them different labels.

## Usage

```alloy
loki.source.podlogs "<LABEL>" {
  forward_to = <RECEIVER_LIST>
}
```

## Arguments

The component starts a new reader for each of the given `targets` and fans out log entries to the list of receivers passed in `forward_to`.

You can use the following arguments with `loki.source.podlogs`:

| Name                        | Type                 | Description                                                                       | Default | Required |
| --------------------------- | -------------------- | --------------------------------------------------------------------------------- | ------- | -------- |
| `forward_to`                | `list(LogsReceiver)` | List of receivers to send log entries to.                                         |         | yes      |
| `preserve_discovered_labels` | `bool`               | Preserve discovered pod metadata labels for use by downstream components.        | `false` | no       |
| `tail_from_end`             | `bool`               | Start reading from the end of the log stream for newly discovered Pod containers. | `false` | no       |

`loki.source.podlogs` searches for `PodLogs` resources on Kubernetes.
Each `PodLogs` resource describes a set of pods to tail logs from.

When `tail_from_end` is `false` (the default), `loki.source.podlogs` reads all available logs from the Kubernetes API for newly discovered Pod containers.
For long-running Pods, this can result in a large volume of logs being processed, which may be rejected by the downstream Loki instance if they are too old.
Set `tail_from_end` to `true` to only read new logs from the point of discovery, ignoring the historical log buffer.
If a last-read offset is already saved for a Pod, `loki.source.podlogs` will resume from that position and ignore the `tail_from_end` argument.

When `preserve_discovered_labels` is `true`, `loki.source.podlogs` preserves discovered Pod metadata labels so they can be accessed by downstream components.
This enables component chaining where discovered Pod metadata labels can be accessed by components like `loki.relabel`.
The preserved labels include all the Pod metadata labels that are available for relabeling within PodLogs custom resources.

## `PodLogs` custom resource

The `PodLogs` resource describes a set of Pods to collect logs from.

{{< admonition type="note" >}}
`loki.source.podlogs` looks for `PodLogs` of `monitoring.grafana.com/v1alpha2`, and isn't compatible with `PodLogs` from the Agent Operator, which are version `v1alpha1`.
{{< /admonition >}}

| Field        | Type            | Description                                   |
| ------------ | --------------- | --------------------------------------------- |
| `apiVersion` | string          | `monitoring.grafana.com/v1alpha2`             |
| `kind`       | string          | `PodLogs`                                     |
| `metadata`   | [ObjectMeta][]  | Metadata for the PodLogs.                     |
| `spec`       | [PodLogsSpec][] | Definition of what Pods to collect logs from. |

[ObjectMeta]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta
[PodLogsSpec]: #podlogsspec

### `PodLogsSpec`

`PodLogsSpec` describes a set of Pods to collect logs from.

| Field               | Type              | Description                                                  |
| ------------------- | ----------------- | ------------------------------------------------------------ |
| `selector`          | [LabelSelector][] | Label selector of Pods to collect logs from.                 |
| `namespaceSelector` | [LabelSelector][] | Label selector of Namespaces that Pods can be discovered in. |
| `relabelings`       | [RelabelConfig][] | Relabel rules to apply to discovered Pods.                   |

If `selector` is left as the default value, all Pods are discovered.
If `namespaceSelector` is left as the default value, all Namespaces are used for Pod discovery.

The `relabelings` field can be used to modify labels from discovered Pods.
The following meta labels are available for relabeling:

* `__meta_kubernetes_namespace`: The namespace of the Pod.
* `__meta_kubernetes_pod_annotation_<annotationname>`: Each annotation from the Pod.
* `__meta_kubernetes_pod_annotationpresent_<annotationname>`: `true` for each annotation from the Pod.
* `__meta_kubernetes_pod_container_image`: The image the container is using.
* `__meta_kubernetes_pod_container_init`: `true` if the container is an `InitContainer`.
* `__meta_kubernetes_pod_container_name`: Name of the container.
* `__meta_kubernetes_pod_controller_kind`: Object kind of the Pod's controller.
* `__meta_kubernetes_pod_controller_name`: Name of the Pod's controller.
* `__meta_kubernetes_pod_host_ip`: The current host IP of the Pod object.
* `__meta_kubernetes_pod_ip`: The Pod IP of the Pod.
* `__meta_kubernetes_pod_label_<labelname>`: Each label from the Pod.
* `__meta_kubernetes_pod_labelpresent_<labelname>`: `true` for each label from the Pod.
* `__meta_kubernetes_pod_name`: The name of the Pod.
* `__meta_kubernetes_pod_node_name`: The name of the node the Pod is scheduled onto.
* `__meta_kubernetes_pod_phase`: Set to `Pending`, `Running`, `Succeeded`, `Failed` or `Unknown` in the lifecycle.
* `__meta_kubernetes_pod_ready`: Set to `true` or `false` for the Pod's ready state.
* `__meta_kubernetes_pod_uid`: The UID of the Pod.

In addition to the meta labels, the following labels are exposed to tell `loki.source.podlogs` which container to tail:

* `__pod_container_name__`: The container name within the Pod.
* `__pod_name__`: The name of the Pod.
* `__pod_namespace__`: The namespace of the Pod.
* `__pod_uid__`: The UID of the Pod.

[LabelSelector]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta
[RelabelConfig]: https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.RelabelConfig

## Blocks

You can use the following blocks with `loki.source.podlogs`:

| Block                                                         | Description                                                                                 | Required |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------- | -------- |
| [`client`][client]                                            | Configures Kubernetes client used to tail logs.                                             | no       |
| `client` > [`authorization`][authorization]                   | Configure generic authorization to the endpoint.                                            | no       |
| `client` > [`basic_auth`][basic_auth]                         | Configure `basic_auth` for authenticating to the endpoint.                                  | no       |
| `client` > [`oauth2`][oauth2]                                 | Configure OAuth 2.0 for authenticating to the endpoint.                                     | no       |
| `client` > `oauth2` > [`tls_config`][tls_config]              | Configure TLS settings for connecting to the endpoint.                                      | no       |
| `client` > [`tls_config`][tls_config]                         | Configure TLS settings for connecting to the endpoint.                                      | no       |
| [`clustering`][clustering]                                    | Configure the component for when {{< param "PRODUCT_NAME" >}} is running in clustered mode. | no       |
| [`namespace_selector`][selector]                              | Label selector for which namespaces to discover `PodLogs` in.                               | no       |
| `namespace_selector` > [`match_expression`][match_expression] | Label selector expression for which namespaces to discover `PodLogs` in.                    | no       |
| [`node_filter`][node_filter]                                  | Filter Pods by node to limit discovery scope.                                               | no       |
| [`relabel_configs`][relabel_configs]                          | Relabel rules to apply to discovered targets before forwarding to downstream components.    | no       |
| [`selector`][selector]                                        | Label selector for which `PodLogs` to discover.                                             | no       |
| `selector` > [`match_expression`][match_expression]           | Label selector expression for which `PodLogs` to discover.                                  | no       |

The > symbol indicates deeper levels of nesting.
For example, `client` > `basic_auth` refers to a `basic_auth` block defined inside a `client` block.

[client]: #client
[authorization]: #authorization
[basic_auth]: #basic_auth
[clustering]: #clustering
[match_expression]: #match_expression
[node_filter]: #node_filter
[oauth2]: #oauth2
[selector]: #selector-and-namespace_selector
[tls_config]: #tls_config

### `client`

The `client` block configures the Kubernetes client used to tail logs from containers.
If the `client` block isn't provided, the default in-cluster configuration with the service account of the running {{< param "PRODUCT_NAME" >}} Pod is used.

The following arguments are supported:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `api_server`             | `string`            | URL of the Kubernetes API server.                                                                |         | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |         | no       |
| `kubeconfig_file`        | `string`            | Path of the `kubeconfig` file to use for connecting to Kubernetes.                               |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |

At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth]block
* [`bearer_token_file`][client] argument
* [`bearer_token`][client] argument
* [`oauth2`][oauth2] block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `clustering`

| Name      | Type   | Description                                         | Default | Required |
|-----------|--------|-----------------------------------------------------|---------|----------|
| `enabled` | `bool` | Distribute log collection with other cluster nodes. |         | yes      |

When {{< param "PRODUCT_NAME" >}} is [using clustering][], and `enabled` is set to true, then this
`loki.source.podlogs` component instance opts-in to participating in the
cluster to distribute the load of log collection between all cluster nodes.

If {{< param "PRODUCT_NAME" >}} is _not_ running in clustered mode, then the block is a no-op and
`loki.source.podlogs` collects logs based on every PodLogs resource discovered.

Clustering looks only at the following labels for determining the shard key:

* `__pod_namespace__`
* `__pod_name__`
* `__pod_container_name__`
* `__pod_uid__`
* `__meta_kubernetes_namespace`
* `__meta_kubernetes_pod_name`
* `__meta_kubernetes_pod_container_name`
* `__meta_kubernetes_pod_uid`
* `container`
* `pod`
* `job`
* `namespace`

[using clustering]: ../../../../get-started/clustering/

### `node_filter`

The `node_filter` block configures node-based filtering for Pod discovery.

The following arguments are supported:

| Name        | Type     | Description                                                                               | Default | Required |
| ----------- | -------- | ----------------------------------------------------------------------------------------- | ------- | -------- |
| `enabled`   | `bool`   | Enable node-based filtering for Pod discovery.                                            | `false` | no       |
| `node_name` | `string` | Node name to filter Pods by. Falls back to the `NODE_NAME` environment variable if empty. | `""`    | no       |

When you set `enabled` to `true`, `loki.source.podlogs` only discovers and collects logs from Pods running on the specified node.
This is particularly useful when running {{< param "PRODUCT_NAME" >}} as a DaemonSet to avoid collecting logs from Pods on other nodes.

If you don't specify `node_name`, `loki.source.podlogs` attempts to use the `NODE_NAME` environment variable.
This allows for easy configuration in DaemonSet deployments where you can inject the node name with the [Kubernetes downward API][].

[Kubernetes downward API]: https://kubernetes.io/docs/concepts/workloads/pods/downward-api/

Example DaemonSet configuration:

```yaml
env:
  - name: NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName
```

Node filtering significantly reduces API server load and network traffic by limiting Pod discovery to only the local node, making it highly recommended for DaemonSet deployments in large clusters.

### `match_expression`

The `match_expression` block describes a Kubernetes label match expression for `PodLogs` or Namespace discovery.

The following arguments are supported:

| Name       | Type           | Description                        | Default | Required |
|------------|----------------|------------------------------------|---------|----------|
| `key`      | `string`       | The label name to match against.   |         | yes      |
| `operator` | `string`       | The operator to use when matching. |         | yes      |
| `values`   | `list(string)` | The values used when matching.     |         | no       |

The `operator` argument must be one of the following strings:

* `"In"`
* `"NotIn"`
* `"Exists"`
* `"DoesNotExist"`

Both `selector` and `namespace_selector` can make use of multiple
`match_expression` inner blocks which are treated as AND clauses.

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `selector` and `namespace_selector`

The `selector` and `namespce_selector` blocks describe a Kubernetes label selector for `PodLogs` or Namespace discovery.

The following arguments are supported:

| Name           | Type          | Description                                       | Default | Required |
|----------------|---------------|---------------------------------------------------|---------|----------|
| `match_labels` | `map(string)` | Label keys and values used to discover resources. | `{}`    | no       |

When the `match_labels` argument is empty, all resources are matched.

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields


When `preserve_discovered_labels` is set to `false`, `loki.source.podlogs` doesn't export any discovered fields (`__meta` labels).

When `preserve_discovered_labels` is set to `true`, `loki.source.podlogs` sends the discovered labels to the downstream components, and then the labels are dropped by either `loki.process` or `loki.write` when the pipeline is completed.

## Component health

`loki.source.podlogs` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.source.podlogs` exposes some target-level debug information per target:

* The labels associated with the target.
* The full set of labels which were found during service discovery.
* The most recent time a log line was read and forwarded to the next components in the pipeline.
* The most recent error from tailing, if any.

## Debug metrics

`loki.source.podlogs` doesn't expose any component-specific debug metrics.

## Example

This example discovers all `PodLogs` resources and forwards collected logs to a `loki.write` component so they're written to Loki.

```alloy
loki.source.podlogs "default" {
  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = sys.env("LOKI_URL")
  }
}
```

This example shows how to preserve discovered Pod labels for use by downstream components:

```alloy
loki.source.podlogs "with_label_preservation" {
  forward_to                  = [loki.write.local.receiver]
  preserve_discovered_labels  = true
}

loki.relabel "pod_relabeling" {
  forward_to = [loki.write.local.receiver]

  rule {
    source_labels = ["__meta_kubernetes_pod_label_app"]
    target_label  = "app"
  }

  rule {
    source_labels = ["__meta_kubernetes_pod_annotation_version"]
    target_label  = "version"
  }
}

loki.write "local" {
  endpoint {
    url = sys.env("LOKI_URL")
  }
}
```

This example shows how to use node filtering for DaemonSet deployments to collect logs only from Pods running on the current node:

```alloy
loki.source.podlogs "daemonset" {
  forward_to = [loki.write.local.receiver]

  node_filter {
    enabled = true
    // node_name will be automatically read from NODE_NAME environment variable
  }
}

loki.write "local" {
  endpoint {
    url = sys.env("LOKI_URL")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.podlogs` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
