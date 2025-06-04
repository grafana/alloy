---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.operator.servicemonitors/
aliases:
  - ../prometheus.operator.servicemonitors/ # /docs/alloy/latest/reference/components/prometheus.operator.servicemonitors/
description: Learn about prometheus.operator.servicemonitors
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.operator.servicemonitors
---

# `prometheus.operator.servicemonitors`

`prometheus.operator.servicemonitors` discovers [ServiceMonitor](https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.ServiceMonitor) resources in your Kubernetes cluster and scrapes the targets they reference.
This component performs three main functions:

1. Discover ServiceMonitor resources from your Kubernetes cluster.
1. Discover Services and Endpoints in your cluster that match those ServiceMonitors.
1. Scrape metrics from those Endpoints, and forward them to a receiver.

The default configuration assumes {{< param "PRODUCT_NAME" >}} is running inside a Kubernetes cluster, and uses the in-cluster configuration to access the Kubernetes API.
You can run it from outside the cluster by supplying connection info in the `client` block, but network level access to discovered endpoints is required to scrape metrics from them.

ServiceMonitors may reference secrets for authenticating to targets to scrape them.
In these cases, the secrets are loaded and refreshed only when the ServiceMonitor is updated or when this component refreshes its' internal state, which happens on a 5-minute refresh cycle.

## Usage

```alloy
prometheus.operator.servicemonitors "<LABEL>" {
    forward_to = <RECEIVER_LIST>
}
```

## Arguments

You can use the following arguments with `prometheus.operator.servicemonitors`:

| Name                    | Type                    | Description                                                                                               | Default       | Required |
| ----------------------- | ----------------------- | --------------------------------------------------------------------------------------------------------- | ------------- | -------- |
| `forward_to`            | `list(MetricsReceiver)` | List of receivers to send scraped metrics to.                                                             |               | yes      |
| `informer_sync_timeout` | `duration`              | Timeout for initial sync of ServiceMonitor resources.                                                     | `"1m"`        | no       |
| `kubernetes_role`       | `string`                | The Kubernetes role used for discovery. Supports `endpoints` or `endpointslice`.                          | `"endpoints"` | no       |
| `namespaces`            | `list(string)`          | List of namespaces to search for ServiceMonitor resources. If not specified, all namespaces are searched. |               | no       |

## Blocks

You can use the following blocks with `prometheus.operator.servicemonitors`:

| Name                                                | Description                                                                                 | Required |
| --------------------------------------------------- | ------------------------------------------------------------------------------------------- | -------- |
| [`client`][client]                                  | Configures Kubernetes client used to find ServiceMonitors.                                  | no       |
| `client` > [`authorization`][authorization]         | Configure generic authorization to the Kubernetes API.                                      | no       |
| `client` > [`basic_auth`][basic_auth]               | Configure basic authentication to the Kubernetes API.                                       | no       |
| `client` > [`oauth2`][oauth2]                       | Configure OAuth 2.0 for authenticating to the Kubernetes API.                               | no       |
| `client` > `oauth2` > [`tls_config`][tls_config]    | Configure TLS settings for connecting to the Kubernetes API.                                | no       |
| `client` > [`tls_config`][tls_config]               | Configure TLS settings for connecting to the Kubernetes API.                                | no       |
| [`clustering`][clustering]                          | Configure the component for when {{< param "PRODUCT_NAME" >}} is running in clustered mode. | no       |
| [`rule`][rule]                                      | Relabeling rules to apply to discovered targets.                                            | no       |
| [`scrape`][scrape]                                  | Default scrape configuration to apply to discovered targets.                                | no       |
| [`selector`][selector]                              | Label selector for which ServiceMonitors to discover.                                       | no       |
| `selector` > [`match_expression`][match_expression] | Label selector expression for which ServiceMonitors to discover.                            | no       |

The > symbol indicates deeper levels of nesting.
For example, `client` > `basic_auth` refers to a `basic_auth` block defined inside a `client` block.

[client]: #client
[basic_auth]: #basic_auth
[authorization]: #authorization
[oauth2]: #oauth2
[tls_config]: #tls_config
[selector]: #selector
[match_expression]: #match_expression
[rule]: #rule
[scrape]: #scrape
[clustering]: #clustering

### `client`

The `client` block configures the Kubernetes client used to discover ServiceMonitors.
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

| Name      | Type   | Description                                       | Default | Required |
| --------- | ------ | ------------------------------------------------- | ------- | -------- |
| `enabled` | `bool` | Enables sharing targets with other cluster nodes. | `false` | yes      |

When {{< param "PRODUCT_NAME" >}} is using [clustering][cluster], and `enabled` is set to true, then this component instance opts-in to participating in the cluster to distribute scrape load between all cluster nodes.

Clustering assumes that all cluster nodes are running with the same configuration file, and that all `prometheus.operator.servicemonitors` components that have opted-in to using clustering, over the course of a scrape interval have the same configuration.

All `prometheus.operator.servicemonitors` components instances opting in to clustering use target labels and a consistent hashing algorithm to determine ownership for each of the targets between the cluster peers.
Then, each peer only scrapes the subset of targets that it's responsible for, so that the scrape load is distributed.
When a node joins or leaves the cluster, every peer recalculates ownership and continues scraping with the new target set.
This performs better than hashmod sharding where _all_ nodes have to be re-distributed, as only 1/N of the target's ownership is transferred, but is eventually consistent (rather than fully consistent like hashmod sharding is).

If {{< param "PRODUCT_NAME" >}} is _not_ running in clustered mode, then the block is a no-op, and `prometheus.operator.servicemonitors` scrapes every target it receives in its arguments.

[cluster]: ../../../../get-started/clustering/

### `rule`

{{< docs/shared lookup="reference/components/rule-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `scrape`

{{< docs/shared lookup="reference/components/prom-operator-scrape.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `selector`

The `selector` block describes a Kubernetes label selector for ServiceMonitors.

The following arguments are supported:

| Name           | Type          | Description                                       | Default | Required |
| -------------- | ------------- | ------------------------------------------------- | ------- | -------- |
| `match_labels` | `map(string)` | Label keys and values used to discover resources. | `{}`    | no       |

When the `match_labels` argument is empty, all ServiceMonitor resources are matched.

### `match_expression`

The `match_expression` block describes a Kubernetes label matcher expression for ServiceMonitors discovery.

The following arguments are supported:

| Name       | Type           | Description                        | Default | Required |
| ---------- | -------------- | ---------------------------------- | ------- | -------- |
| `key`      | `string`       | The label name to match against.   |         | yes      |
| `operator` | `string`       | The operator to use when matching. |         | yes      |
| `values`   | `list(string)` | The values used when matching.     |         | no       |

The `operator` argument must be one of the following strings:

* `"In"`
* `"NotIn"`
* `"Exists"`
* `"DoesNotExist"`

If there are multiple `match_expressions` blocks inside of a `selector` block, they're combined together with AND clauses.

## Exported fields

`prometheus.operator.servicemonitors` doesn't export any fields. It forwards all metrics it scrapes to the receiver configures with the `forward_to` argument.

## Component health

`prometheus.operator.servicemonitors` is reported as unhealthy when given an invalid configuration, Prometheus components fail to initialize, or the connection to the Kubernetes API couldn't be established properly.

## Debug information

`prometheus.operator.servicemonitors` reports the status of the last scrape for each configured scrape job on the component's debug endpoint, including discovered labels, and the last scrape time.

It also exposes some debug information for each ServiceMonitor it has discovered, including any errors found while reconciling the scrape configuration from the ServiceMonitor.

## Debug metrics

`prometheus.operator.servicemonitors` doesn't expose any component-specific debug metrics.

## Example

The following example discovers all ServiceMonitors in your cluster, and forwards collected metrics to a `prometheus.remote_write` component.

```alloy
prometheus.remote_write "staging" {
  // Send metrics to a locally running Mimir.
  endpoint {
    url = "http://mimir:9009/api/v1/push"

    basic_auth {
      username = "example-user"
      password = "example-password"
    }
  }
}

prometheus.operator.servicemonitors "services" {
    forward_to = [prometheus.remote_write.staging.receiver]
}
```

The following example limits discovered ServiceMonitors to ones with the label `team=ops` in a specific namespace: `my-app`.

```alloy
prometheus.operator.servicemonitors "services" {
    forward_to = [prometheus.remote_write.staging.receiver]
    namespaces = ["my-app"]
    selector {
        match_expression {
            key = "team"
            operator = "In"
            values = ["ops"]
        }
    }
}
```

The following example applies additional relabel rules to discovered targets to filter by hostname.
This may be useful if running {{< param "PRODUCT_NAME" >}} as a DaemonSet.

```alloy
prometheus.operator.servicemonitors "services" {
    forward_to = [prometheus.remote_write.staging.receiver]
    rule {
      action = "keep"
      regex = sys.env("HOSTNAME")
      source_labels = ["__meta_kubernetes_pod_node_name"]
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.operator.servicemonitors` can accept arguments from the following components:

- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
