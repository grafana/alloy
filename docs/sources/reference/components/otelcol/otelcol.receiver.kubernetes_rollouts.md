---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.kubernetes_rollouts/
description: Learn about otelcol.receiver.kubernetes_rollouts
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.kubernetes_rollouts
---

# `otelcol.receiver.kubernetes_rollouts`

`otelcol.receiver.kubernetes_rollouts` watches Kubernetes Deployments and emits OpenTelemetry log events when rollouts start, finish, stall, or are superseded.
The component also emits an event when a container image digest becomes available from Pod status.

The component is opt-in.
It doesn't watch Kubernetes resources unless you add it to the {{< param "PRODUCT_NAME" >}} configuration.

{{< admonition type="caution" >}}
This component is experimental and its event schema can change without notice.
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.kubernetes_rollouts "<LABEL>" {
  output {
    logs = <OTEL_LOG_CONSUMER_LIST>
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.kubernetes_rollouts`:

| Name           | Type     | Description                                      | Default | Required |
| -------------- | -------- | ------------------------------------------------ | ------- | -------- |
| `cluster_name` | `string` | Human-readable name of the Kubernetes cluster.   | `""`    | no       |
| `cluster_uid`  | `string` | Stable identifier for the Kubernetes cluster.    | `""`    | no       |

When `cluster_uid` is empty, the component uses the UID of the `kube-system` Namespace as `k8s.cluster.uid`.

## Blocks

You can use the following blocks with `otelcol.receiver.kubernetes_rollouts`:

{{< docs/alloy-config >}}

| Block                                       | Description                                                  | Required |
| ------------------------------------------- | ------------------------------------------------------------ | -------- |
| [`client`][client]                          | Configures the Kubernetes client.                            | no       |
| `client` > [`authorization`][authorization] | Configures generic authorization to the endpoint.            | no       |
| `client` > [`basic_auth`][basic_auth]       | Configures basic authentication to the endpoint.             | no       |
| `client` > [`oauth2`][oauth2]               | Configures OAuth 2.0 authentication to the endpoint.         | no       |
| `client` > [`tls_config`][tls_config]       | Configures TLS settings for connecting to the endpoint.      | no       |
| [`clustering`][clustering]                  | Configures cluster-wide ownership of the Kubernetes watcher. | no       |
| [`output`][output]                          | Configures where to send rollout events.                     | yes      |

[authorization]: #authorization
[basic_auth]: #basic_auth
[client]: #client
[clustering]: #clustering
[oauth2]: #oauth2
[output]: #output
[tls_config]: #tls_config

{{< /docs/alloy-config >}}

### `client`

The `client` block configures the Kubernetes client.
If the `client` block isn't provided, the component uses in-cluster configuration and the service account of the {{< param "PRODUCT_NAME" >}} Pod.

The following arguments are supported:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `api_server`             | `string`            | URL of the Kubernetes API server.                                                                |         | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to send with each request.                                                    |         | no       |
| `kubeconfig_file`        | `string`            | Path of the `kubeconfig` file to use for connecting to Kubernetes.                               |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `proxy_connect_header`   | `map(list(secret))` | Headers to send to proxies during CONNECT requests.                                              |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |

At most one of the following can be provided:

* [`authorization`](#authorization) block
* [`basic_auth`](#basic_auth) block
* [`bearer_token_file`](#client) argument
* [`bearer_token`](#client) argument
* [`oauth2`](#oauth2) block

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

| Name      | Type   | Description                                               | Default | Required |
| --------- | ------ | --------------------------------------------------------- | ------- | -------- |
| `enabled` | `bool` | Assign the watcher to one {{< param "PRODUCT_NAME" >}} cluster member. | `false` | yes      |

When clustering is enabled, consistent hashing assigns the component to one {{< param "PRODUCT_NAME" >}} cluster member.
Ownership moves to another member when the owner stops or the cluster membership changes.
The replacement member rebuilds its state from the Kubernetes API and can emit duplicate events.

If {{< param "PRODUCT_NAME" >}} isn't running in clustered mode, the block has no effect and every configured instance watches the Kubernetes cluster.

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block-logs.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Kubernetes permissions

The component watches Deployments, ReplicaSets, and Pods across all namespaces.
Grant its service account the following permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: alloy-kubernetes-rollouts
rules:
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get"]
```

The `namespaces` permission isn't required when you configure `cluster_uid`.

## Event schema

The component emits one log record for each container image in a rollout.
It uses OpenTelemetry semantic convention attributes where applicable and uses the `grafana.sdlc.*` namespace for experimental fields.

Event names have the form `grafana.sdlc.k8s.deployment.rollout.<PHASE>`.
The supported phases are:

* `started`: The component observes a new Deployment generation that hasn't completed.
* `succeeded`: All desired replicas for the generation are updated and available.
* `stalled`: Kubernetes reports `ProgressDeadlineExceeded` or `ReplicaSetCreateError`.
* `superseded`: A new generation replaces a rollout that was still in progress.
* `image_resolved`: Pod status exposes a digest that wasn't previously observed for the rollout.

Every record contains these resource attributes:

* `k8s.cluster.uid`
* `k8s.cluster.name`, when `cluster_name` is configured
* `k8s.namespace.name`
* `k8s.deployment.name`
* `k8s.deployment.uid`

Image records include `k8s.container.name`, `container.image.name`, `container.image.tags`, `container.image.id`, and `container.image.repo_digests` when those values are available.
The original image reference is available as `grafana.sdlc.container.image.reference`.

`grafana.sdlc.event.id` is deterministic for a rollout phase, container, and digest.
Consumers can use it as a deduplication key.

## Future scope

The component currently observes Kubernetes Deployments only.
The experimental event schema should remain usable for other workload controllers, including:

* StatefulSets
* DaemonSets
* Jobs and CronJobs
* Argo Rollouts `Rollout` resources and other custom workload controllers that manage ReplicaSets or Pods directly

Future controller support should continue to use generic attributes such as `deployment.id`, `deployment.status`, image attributes, and `grafana.sdlc.rollout.phase`.
Controller-specific metadata should use the applicable `k8s.*` attributes.

## Delivery semantics

Events use at-least-once delivery while the component is running.
The component updates its local rollout state only after downstream consumers accept an event batch.
If delivery fails, it retries the Deployment through a rate-limited work queue.

State is held in memory.
After a restart or cluster ownership change, the component reconstructs current state from Kubernetes informer caches and can emit duplicate events.
Downstream consumers should deduplicate using `grafana.sdlc.event.id`.

## Exported fields

`otelcol.receiver.kubernetes_rollouts` doesn't export any fields.

## Component health

`otelcol.receiver.kubernetes_rollouts` is reported as unhealthy if its configuration is invalid.
Runtime Kubernetes API and downstream delivery errors are logged and retried.

## Debug information

`otelcol.receiver.kubernetes_rollouts` doesn't expose component-specific debug information.

## Debug metrics

`otelcol.receiver.kubernetes_rollouts` doesn't expose component-specific debug metrics.

## Examples

The following examples show how to send rollout events to a local debug exporter, a custom
OTLP/HTTP endpoint, or a Grafana Cloud OTLP/HTTP endpoint.

### Log events locally

This example logs rollout events with the OpenTelemetry debug exporter:

```alloy
otelcol.receiver.kubernetes_rollouts "default" {
  cluster_name = "production-eu"

  clustering {
    enabled = true
  }

  output {
    logs = [otelcol.exporter.debug.rollouts.input]
  }
}

otelcol.exporter.debug "rollouts" {
  verbosity = "detailed"
}
```

### Send events to a custom OTLP/HTTP endpoint

This example sends only rollout events to a configurable OTLP/HTTP endpoint:

```alloy
otelcol.receiver.kubernetes_rollouts "default" {
  cluster_name = sys.env("K8S_CLUSTER_NAME")

  output {
    logs = [otelcol.exporter.otlphttp.rollout_api.input]
  }
}

otelcol.exporter.otlphttp "rollout_api" {
  client {
    endpoint = sys.env("ROLLOUT_OTLP_ENDPOINT")
  }

  retry_on_failure {
    max_elapsed_time = "0s"
  }
}
```

Set `ROLLOUT_OTLP_ENDPOINT` to the base URL of an API that accepts OTLP/HTTP logs.
The exporter sends requests to `<ROLLOUT_OTLP_ENDPOINT>/v1/logs`.
To use a different path, set the exporter's `logs_endpoint` argument to the complete URL.

The endpoint is scoped to the `rollout_api` exporter.
Only the rollout receiver is connected to that exporter, so the endpoint doesn't affect other
telemetry pipelines in the same {{< param "PRODUCT_NAME" >}} configuration.

When {{< param "PRODUCT_NAME" >}} runs in Kubernetes, the endpoint must be reachable from the
{{< param "PRODUCT_NAME" >}} Pod.
An endpoint on `localhost` refers to the Pod itself, not to the workstation running `kubectl`.

The component watches resources in all namespaces regardless of the namespace where
{{< param "PRODUCT_NAME" >}} runs.
The service account must have the cluster-scoped permissions described in
[Kubernetes permissions](#kubernetes-permissions).

For a complete custom endpoint configuration, use
`example/kubernetes-rollouts/local-api.alloy`.

### Send events to Grafana Cloud

This example sends rollout events to a Grafana Cloud API that accepts OTLP/HTTP logs at `/v1/logs`:

```alloy
otelcol.receiver.kubernetes_rollouts "default" {
  cluster_name = sys.env("K8S_CLUSTER_NAME")

  clustering {
    enabled = true
  }

  output {
    logs = [otelcol.exporter.otlphttp.grafana_cloud.input]
  }
}

otelcol.exporter.otlphttp "grafana_cloud" {
  client {
    endpoint = sys.env("GRAFANA_CLOUD_OTLP_ENDPOINT")
    auth     = otelcol.auth.basic.grafana_cloud.handler
  }

  retry_on_failure {
    max_elapsed_time = "0s"
  }

  sending_queue {
    block_on_overflow = true
    num_consumers     = 1
    storage           = otelcol.storage.file.rollouts.handler
  }
}

otelcol.auth.basic "grafana_cloud" {
  client_auth {
    username = sys.env("GRAFANA_CLOUD_INSTANCE_ID")
    password = sys.env("GRAFANA_CLOUD_API_KEY")
  }
}

otelcol.storage.file "rollouts" {
  fsync = true
}
```

Set the following environment variables:

* `K8S_CLUSTER_NAME`: Human-readable name of the Kubernetes cluster.
* `GRAFANA_CLOUD_OTLP_ENDPOINT`: Base URL of the OTLP/HTTP endpoint without `/v1/logs`.
  `otelcol.exporter.otlphttp` appends `/v1/logs` when it sends log records.
* `GRAFANA_CLOUD_INSTANCE_ID`: Grafana Cloud stack or instance ID used as the basic authentication
  username.
* `GRAFANA_CLOUD_API_KEY`: Grafana Cloud access policy token authorized to write to the endpoint.

The exporter retries failed requests until they succeed because `max_elapsed_time` is `"0s"`.
The sending queue uses one consumer to preserve request order and applies backpressure when the queue is full.
The queue uses `otelcol.storage.file` with `fsync` enabled so accepted batches survive an
{{< param "PRODUCT_NAME" >}} process restart.
When you run {{< param "PRODUCT_NAME" >}} in Kubernetes, mount the storage path on persistent storage
if batches must also survive Pod replacement or rescheduling.

For a reusable opt-in custom component, use the module in `example/kubernetes-rollouts/module.alloy`.
Importing the file only defines the custom component; instantiate `deployment_rollouts` to start the watcher.
For a complete Grafana Cloud configuration, use `example/kubernetes-rollouts/grafana-cloud.alloy`.
