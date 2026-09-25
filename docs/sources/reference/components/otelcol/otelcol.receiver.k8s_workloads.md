---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.k8s_workloads/
description: Learn about otelcol.receiver.k8s_workloads
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.k8s_workloads
---

# `otelcol.receiver.k8s_workloads`

`otelcol.receiver.k8s_workloads` emits Kubernetes inventory and change events as OpenTelemetry logs.

For namespaces, it emits three [events](#namespace-events):

- `snapshot`: A complete list of namespaces, collected at startup and at the interval configured in the [`snapshots` block](#snapshots).
- `created`: A namespace is created after the initial watch listing.
- `deleted`: A namespace is deleted.

For Deployments, it emits five [events](#deployment-events):

- `snapshot`: A complete list of Deployments in each namespace, including container images, collected on the same [snapshot schedule](#snapshots).
- `created`: A Deployment is created after the initial watch listing.
- `deleted`: A Deployment is deleted.
- `succeeded`: A Deployment reaches the derived `healthy` status.
- `stalled`: A Deployment reports that its progress deadline has been exceeded.

The shared [`grafana.sdlc.reporting.error` event](#error-reports) reports collection and delivery failures, including events that reach the [maximum payload size](#snapshots).
ReplicaSets and Pods enrich Deployment snapshots but don't produce separate events.

{{< admonition type="caution" >}}
This component is experimental and its event schema can change without notice.
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.k8s_workloads "<LABEL>" {
  output {
    logs = <OTEL_LOG_CONSUMER_LIST>
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.k8s_workloads`:

| Name           | Type     | Description                                      | Default | Required |
| -------------- | -------- | ------------------------------------------------ | ------- | -------- |
| `cluster_name` | `string` | Human-readable name of the Kubernetes cluster.   | `""`    | no       |
| `cluster_uid`  | `string` | Stable identifier for the Kubernetes cluster.    | `""`    | no       |

When `cluster_uid` is empty, the component uses the UID of the `kube-system` Namespace as `k8s.cluster.uid`.

## Blocks

You can use the following blocks with `otelcol.receiver.k8s_workloads`:

{{< docs/alloy-config >}}

| Block                                       | Description                                                  | Required |
| ------------------------------------------- | ------------------------------------------------------------ | -------- |
| [`client`][client]                          | Configures the Kubernetes client.                                      | no       |
| `client` > [`authorization`][authorization] | Configures generic authorization for Kubernetes API requests.          | no       |
| `client` > [`basic_auth`][basic_auth]       | Configures basic authentication for Kubernetes API requests.           | no       |
| `client` > [`oauth2`][oauth2]               | Configures OAuth 2.0 authentication for Kubernetes API requests.       | no       |
| `client` > [`tls_config`][tls_config]       | Configures TLS for connections to the Kubernetes API server.           | no       |
| [`clustering`][clustering]                  | Configures cluster-wide ownership of the Kubernetes watcher.           | no       |
| [`snapshots`][snapshots] | Configures scan frequency and the serialized event size limit. | no |
| [`output`][output]                          | Configures where to send workload events.                               | yes      |

[authorization]: #authorization
[basic_auth]: #basic_auth
[client]: #client
[clustering]: #clustering
[oauth2]: #oauth2
[snapshots]: #snapshots
[output]: #output
[tls_config]: #tls_config

{{< /docs/alloy-config >}}

### `snapshots`

The optional `snapshots` block configures the inventory scan interval and the size limit for inventory and notification events:

| Name | Type | Description | Default | Required |
| --- | --- | --- | --- | --- |
| `interval` | `duration` | Positive interval between inventory scans. | `"1m"` | no |
| `max_size_bytes` | `number` | Exclusive upper limit on a serialized one-record OTLP envelope. Must be at least 8192. | `524288` | no |

The component performs its first scan after the namespace and Deployment watches synchronize.
It performs subsequent scans serially and coalesces ticks if collection takes longer than the interval.
Resource changes don't trigger extra snapshots.
Unchanged and empty inventories still produce snapshots.

The component checks the sizes of OTLP envelopes encoded as Protocol Buffers or JSON before delivery.
An event at or above `max_size_bytes` produces an [error report](#error-reports) instead of the original event.
The component never truncates or splits an inventory.

### `client`

The `client` block configures the Kubernetes client.
If the `client` block isn't provided, the component uses in-cluster configuration and the service account of the {{< param "PRODUCT_NAME" >}} Pod.
The `authorization`, `basic_auth`, `oauth2`, and `tls_config` blocks described in the following sections must be nested inside `client`.
They configure requests to the Kubernetes API server and don't configure the component's `output` destination.

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

#### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

#### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

#### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

#### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `clustering`

The `clustering` block assigns collection to one {{< param "PRODUCT_NAME" >}} cluster member:

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

The component watches namespaces and Deployments and lists ReplicaSets and Pods across all namespaces.
Grant its service account the following permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: alloy-k8s-workloads
rules:
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods", "namespaces"]
    verbs: ["get", "list", "watch"]
```

Namespace get/list/watch permissions are required even when you configure `cluster_uid`, because namespace inventory and notifications are part of the event contract.

## Event schema

Events use OpenTelemetry semantic convention attributes where applicable and the `grafana.sdlc.*` namespace for experimental fields.

### Inventory and notification events

Each event is one OTLP log record with a JSON string body.
Resource attributes include `k8s.cluster.uid` and, when configured, `k8s.cluster.name`.
Namespace notifications and Deployment snapshots include `k8s.namespace.name` and `k8s.namespace.uid`.
Deployment notifications include `k8s.namespace.name` and `k8s.deployment.uid`, but not the namespace UID.
Each record has a `grafana.sdlc.event.id`, `grafana.sdlc.schema.version` set to `1`, and its collection or observation timestamp in `timeUnixNano`.

The component reads membership from the Kubernetes API instead of a local cache that might be out of sync.
Created notifications exclude objects returned by the initial watch listing: startup snapshots represent those objects.
Notifications describe changes observed by the watcher; snapshots contain the current inventory.

#### Namespace events

Namespace events describe the namespace inventory, creation, and deletion.

| Event name | Scope | JSON body |
| --- | --- | --- |
| `grafana.sdlc.k8s.namespace.snapshot` | Cluster | `complete`, `collected_at`, `resource_version`, and a `namespaces` array. |
| `grafana.sdlc.k8s.namespace.created` | Namespace | `name`, `uid`, and `collected_at`. |
| `grafana.sdlc.k8s.namespace.deleted` | Namespace | `name`, `uid`, and `collected_at`. |

Namespace snapshots contain the UID, name, labels, phase, creation time, and optional deletion timestamp of each namespace.
An empty namespace collection is valid and contains `namespaces: []` with `complete: true`.

#### Deployment events

Deployment events describe the Deployment inventory, creation, deletion, and terminal results.

| Event name | Scope | JSON body |
| --- | --- | --- |
| `grafana.sdlc.k8s.deployment.snapshot` | Namespace | `complete`, `collected_at`, `resource_version`, and a `deployments` array. |
| `grafana.sdlc.k8s.deployment.created` | Deployment | `name`, `uid`, and `collected_at`. |
| `grafana.sdlc.k8s.deployment.deleted` | Deployment | `name`, `uid`, and `collected_at`. |
| `grafana.sdlc.k8s.deployment.succeeded` | Deployment | `name`, `uid`, `collected_at`, `status`, `generation`, and `observed_generation`. |
| `grafana.sdlc.k8s.deployment.stalled` | Deployment | `name`, `uid`, `collected_at`, `status`, `generation`, and `observed_generation`. |

Deployment snapshots contain UID, name, labels, creation/deletion times, resource version, generation, observed generation, paused state, replica counts, Kubernetes conditions, and container information.
Container entries include `name`, `init`, configured `image`, parsed `image_name`/`tag`, and a `resolved` array with runtime `id`, `digest` when available, and `replicaset_uid`.
The resolved array can be empty, or contain multiple images from ReplicaSets running during an update.

The derived Deployment `status` is one of `progressing`, `healthy`, `degraded`, `stalled`, `paused`, or `terminating`.
These are presentation labels, not native Kubernetes phases.
Use the original conditions and counters when you need more detail.
A Deployment whose controller hasn't observed its current generation is `progressing`.

ReplicaSets and Pods are separate enrichment reads, so their fields aren't an atomic cross-resource view.
If any required list fails, the component emits an error instead of a partial Deployment inventory.
An empty Deployment collection is valid and contains `deployments: []` with `complete: true`.

A `succeeded` notification reports `healthy`: the controller has observed the current generation, all desired replicas are updated and available, and none are unavailable.
A `stalled` notification reports a `Progressing=False` condition with reason `ProgressDeadlineExceeded` after the controller observes the current generation.
Paused or terminating Deployments don't emit these notifications.
The watcher remembers the last result for each Deployment UID and Pod template, so repeated updates and replica-only scaling don't repeat the same result.
A new Pod template resets this state; a change between `succeeded` and `stalled` emits a new notification.
The initial watch listing records existing results without emitting notifications, and this state doesn't survive restarts.
Only snapshots include container images; notifications don't.

### Error reports

The shared `grafana.sdlc.reporting.error` event reports failures to collect or deliver namespace and Deployment events.

| Event name | Scope | JSON body |
| --- | --- | --- |
| `grafana.sdlc.reporting.error` | Failed operation's scope | `operation`, `entity_kind`, `attempt_id`, `reason`, `message`, and `failed_collected_at`. |

The `reason` field identifies the failure:

- `payload_too_large`: The event reaches or exceeds `snapshots.max_size_bytes`. The error includes `measured_bytes` and `limit_bytes`.
- `collection_failed`: A required Kubernetes API read fails, or the namespace disappears or is recreated during collection.
- `delivery_failed`: A configured output returns an error. The component retries the original event.

Error events omit the rejected payload and have a separate 8192-byte JSON envelope limit.
They use the same output as other events, so delivery isn't guaranteed during an outage.
The component logs failures and increments an error counter; failure to send an error produces only a log message.

## Delivery semantics

The component retries failed deliveries in memory, preserving the original payload, event ID, and timestamp.
Events at or above the size limit aren't retried; the next scan collects a new snapshot.
Queued events don't survive a process restart.
Retries can be emitted after newer events, and changes in cluster ownership can produce duplicate notifications.
Event timestamps use the collector's local clock.

Clustering selects one collection owner but doesn't prevent overlapping collectors during a network partition.
Without clustering, each configured instance watches the Kubernetes cluster independently.

## Exported fields

`otelcol.receiver.k8s_workloads` doesn't export any fields.

## Component health

`otelcol.receiver.k8s_workloads` is reported as unhealthy if its configuration is invalid.
Collection and delivery failures produce error reports and logs.
Watcher startup failures are logged and retried.

## Debug information

`otelcol.receiver.k8s_workloads` doesn't expose component-specific debug information.

## Debug metrics

The component exposes these metrics:

* `k8s_workloads_reporting_errors_total`: Reporting failures labeled by operation and reason.
* `k8s_workloads_snapshot_last_success_timestamp_seconds`: Time of the last snapshot for which all configured outputs returned successfully, labeled by entity kind and namespace.

## Examples

The following examples show how to send workload events to a local debug exporter, a custom
OTLP/HTTP endpoint, or a Grafana Cloud OTLP/HTTP endpoint.

### Log events locally

This example logs workload events with the OpenTelemetry debug exporter:

```alloy
otelcol.receiver.k8s_workloads "default" {
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

This example sends workload events to a configurable OTLP/HTTP endpoint:

```alloy
otelcol.receiver.k8s_workloads "default" {
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
Only this receiver is connected to that exporter, so the endpoint doesn't affect other
telemetry pipelines in the same {{< param "PRODUCT_NAME" >}} configuration.

When {{< param "PRODUCT_NAME" >}} runs in Kubernetes, the endpoint must be reachable from the
{{< param "PRODUCT_NAME" >}} Pod.
An endpoint on `localhost` refers to the Pod itself, not to the workstation running `kubectl`.

The component watches resources in all namespaces regardless of the namespace where
{{< param "PRODUCT_NAME" >}} runs.
The service account must have the cluster-scoped permissions described in
[Kubernetes permissions](#kubernetes-permissions).

For a complete custom endpoint configuration, use
`example/k8s-workloads/local-api.alloy`.

### Send events to Grafana Cloud

This example sends workload events to a Grafana Cloud API that accepts OTLP/HTTP logs at `/v1/logs`:

```alloy
otelcol.receiver.k8s_workloads "default" {
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

This example configures retries and a persistent sending queue in the exporter.
For exporter options, refer to [`otelcol.exporter.otlphttp`](../otelcol.exporter.otlphttp/).

For a reusable opt-in custom component, use the module in `example/k8s-workloads/module.alloy`.
Importing the file only defines the custom component; instantiate `deployment_rollouts` to start the watcher.
For a complete Grafana Cloud configuration, use `example/k8s-workloads/grafana-cloud.alloy`.
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.k8s_workloads` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
