---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.kubernetes/
aliases:
  - ../loki.source.kubernetes/ # /docs/alloy/latest/reference/components/loki.source.kubernetes/
description: Learn about loki.source.kubernetes
labels:
  stage: general-availability
  products:
    - oss
title: loki.source.kubernetes
---

# `loki.source.kubernetes`

`loki.source.kubernetes` tails logs from Kubernetes containers using the Kubernetes API.

{{< admonition type="note" >}}
This component collects logs from Kubernetes Pods. You can't use this component to collect logs from Kubernetes Nodes.
{{< /admonition >}}

This component has the following benefits over `loki.source.file`:

* It works without a privileged container.
* It works without a root user.
* It works without needing access to the filesystem of the Kubernetes node.
* It doesn't require a DaemonSet to collect logs, so one {{< param "PRODUCT_NAME" >}} could collect logs for the whole cluster.

{{< admonition type="note" >}}
Because `loki.source.kubernetes` uses the Kubernetes API to tail logs, it uses more network traffic and CPU consumption of Kubelets than `loki.source.file`.
{{< /admonition >}}

You can specify multiple `loki.source.kubernetes` components by giving them different labels.

## Usage

```alloy
loki.source.kubernetes "<LABEL>" {
  targets    = <TARGET_LIST>
  forward_to = <RECEIVER_LIST>
}
```

## Arguments

The component starts a new reader for each of the given `targets` and fans out log entries to the list of receivers passed in `forward_to`.

You can use the following arguments with `loki.source.kubernetes`:

| Name         | Type                 | Description                               | Default | Required |
| ------------ | -------------------- | ----------------------------------------- | ------- | -------- |
| `forward_to` | `list(LogsReceiver)` | List of receivers to send log entries to. |         | yes      |
| `targets`    | `list(map(string))`  | List of targets to tail logs from.        |         | yes      |

Each target in `targets` must have the following labels:

* `__meta_kubernetes_namespace` or `__pod_namespace__` to specify the namespace of the Pod to tail.
* `__meta_kubernetes_pod_container_name` or `__pod_container_name__` to specify the container within the Pod to tail.
* `__meta_kubernetes_pod_name` or `__pod_name__` to specify the name of the Pod to tail.
* `__meta_kubernetes_pod_uid` or `__pod_uid__` to specify the UID of the Pod to tail.

By default, all of these labels are present when the output `discovery.kubernetes` is used.

A log tailer is started for each unique target in `targets`.
Log tailers reconnect with exponential backoff to Kubernetes if the log stream returns before the container has permanently terminated.

## Blocks

You can use the following blocks with `loki.source.kubernetes`:

| Block                                            | Description                                                                                 | Required |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------- | -------- |
| [`client`][client]                               | Configures Kubernetes client used to tail logs.                                             | no       |
| `client` > [`authorization`][authorization]      | Configure generic authorization to the endpoint.                                            | no       |
| `client` > [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint.                                  | no       |
| `client` > [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.                                     | no       |
| `client` > `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.                                      | no       |
| `client` > [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.                                      | no       |
| [`clustering`][clustering]                       | Configure the component for when {{< param "PRODUCT_NAME" >}} is running in clustered mode. | no       |

The > symbol indicates deeper levels of nesting.
For example, `client` > `basic_auth` refers to a `basic_auth` block defined inside a `client` block.

[client]: #client
[authorization]: #authorization
[basic_auth]: #basic_auth
[clustering]: #clustering
[oauth2]: #oauth2
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
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][client] argument
* [`bearer_token`][client] argument
* [`oauth2`][oauth2] block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `clustering`

| Name      | Type   | Description                                         | Default | Required |
| --------- | ------ | --------------------------------------------------- | ------- | -------- |
| `enabled` | `bool` | Distribute log collection with other cluster nodes. |         | yes      |

When {{< param "PRODUCT_NAME" >}} is [using clustering][], and `enabled` is set to true, then this `loki.source.kubernetes` component instance opts-in to participating in the cluster to distribute the load of log collection between all cluster nodes.

If {{< param "PRODUCT_NAME" >}} is _not_ running in clustered mode, then the block is a no-op and `loki.source.kubernetes` collects logs from every target it receives in its arguments.

Clustering looks only at the following labels for determining the shard key:

* `__meta_kubernetes_namespace`
* `__meta_kubernetes_pod_container_name`
* `__meta_kubernetes_pod_name`
* `__meta_kubernetes_pod_uid`
* `__pod_container_name__`
* `__pod_name__`
* `__pod_namespace__`
* `__pod_uid__`
* `container`
* `job`
* `namespace`
* `pod`

[using clustering]: ../../../../get-started/clustering/

## Exported fields

`loki.source.kubernetes` doesn't export any fields.

## Component health

`loki.source.kubernetes` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.source.kubernetes` exposes some target-level debug information per target:

* The labels associated with the target.
* The full set of labels which were found during service discovery.
* The most recent time a log line was read and forwarded to the next components in the pipeline.
* The most recent error from tailing, if any.

## Debug metrics

`loki.source.kubernetes` doesn't expose any component-specific debug metrics.

## Example

This example collects logs from all Kubernetes Pods and forwards them to a `loki.write` component so they're written to Loki.

```alloy
discovery.kubernetes "pods" {
  role = "pod"
}

loki.source.kubernetes "pods" {
  targets    = discovery.kubernetes.pods.targets
  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = sys.env("<LOKI_URL>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.kubernetes` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
