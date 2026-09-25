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

`otelcol.receiver.k8s_workloads` emits complete Kubernetes namespace and Deployment inventories as OpenTelemetry log events.
It collects inventory on startup and at a configurable interval, and emits immediate namespace and Deployment created/deleted notifications plus Deployment succeeded/stalled notifications.
ReplicaSet and Pod data enrich Deployment snapshots with runtime image IDs and digests; they don't produce separate events.

The component is opt-in.
It doesn't watch Kubernetes resources unless you add it to the {{< param "PRODUCT_NAME" >}} configuration.

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
| [`output`][output]                          | Configures where to send inventory events.                               | yes      |

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

The optional `snapshots` block configures the authoritative inventories:

| Name | Type | Description | Default | Required |
| --- | --- | --- | --- | --- |
| `interval` | `duration` | Positive interval between inventory scans. | `"1m"` | no |
| `max_size_bytes` | `number` | Exclusive upper limit on a serialized one-record OTLP envelope. Must be at least 8192. | `524288` | no |

The component performs its first scan after the namespace and Deployment watches synchronize.
It performs subsequent scans serially and coalesces ticks if collection takes longer than the interval.
Updates, status transitions, and image resolution don't trigger extra snapshots.
Unchanged and empty inventories still produce snapshots.

The component checks both OTLP protobuf and OTLP JSON envelope sizes before delivery.
An event at or above `max_size_bytes` is replaced with a bounded `reporting.error` event; the component never truncates inventories or splits them into multiple records.
Error events have an independent 8192-byte JSON limit and don't include the rejected payload.
Keep downstream HTTP request and Kafka limits large enough for these envelopes and any exporter batching or gateway encoding overhead.
A gateway must check its own final Kafka serialization if it changes encoding.

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

It uses OpenTelemetry semantic convention attributes where applicable and uses the `grafana.sdlc.*` namespace for experimental fields.

### Inventory and notification events

Each event is one OTLP log record. Its body is a JSON string, so an OTLP gateway can split log records without splitting the inventory inside a record.
The resource attributes identify `k8s.cluster.uid`, optional `k8s.cluster.name`, and the namespace name/UID when applicable.
Deployment notifications additionally identify `k8s.deployment.uid`.
The record includes `grafana.sdlc.event.id`, `grafana.sdlc.schema.version=1`, and a collection timestamp in `timeUnixNano`.

| Event name | Scope | JSON body |
| --- | --- | --- |
| `grafana.sdlc.k8s.namespace.snapshot` | Cluster | `complete`, `collected_at`, `resource_version`, and a `namespaces` array. |
| `grafana.sdlc.k8s.deployment.snapshot` | Namespace | `complete`, `collected_at`, `resource_version`, and a `deployments` array. |
| `grafana.sdlc.k8s.namespace.created` / `.deleted` | Namespace | `name`, `uid`, and `collected_at`. |
| `grafana.sdlc.k8s.deployment.created` / `.deleted` | Deployment | `name`, `uid`, and `collected_at`. |
| `grafana.sdlc.k8s.deployment.succeeded` / `.stalled` | Deployment | `name`, `uid`, `collected_at`, `status`, `generation`, and `observed_generation`. |
| `grafana.sdlc.reporting.error` | Failed operation's scope | `operation`, `entity_kind`, `attempt_id`, `reason`, `message`, and `failed_collected_at`. |

Namespace snapshots contain each namespace's UID, name, labels, phase, creation time, and optional deletion timestamp.
Deployment snapshots contain UID, name, labels, creation/deletion times, resource version, generation, observed generation, paused state, replica counts, Kubernetes conditions, and container information.
Container entries include `name`, `init`, configured `image`, parsed `image_name`/`tag`, and a `resolved` array with runtime `id`, `digest` when available, and `replicaset_uid`.
The resolved array can be empty, or contain multiple images from ReplicaSets running during an update.

The derived Deployment `status` is one of `progressing`, `healthy`, `degraded`, `stalled`, `paused`, or `terminating`.
These are presentation labels, not native Kubernetes phases.
Use the original conditions and counters when you need more detail.
A Deployment whose controller hasn't observed its current generation is `progressing`.

Membership is read from the Kubernetes API, not an unsynchronized local cache.
ReplicaSets and Pods are separate enrichment reads, so their fields aren't an atomic cross-resource view.
If any required list fails, the component emits an error instead of a partial Deployment inventory.
An empty collection is valid and contains `namespaces: []` or `deployments: []` with `complete: true`.

Created notifications exclude objects returned by the initial watch listing: startup snapshots represent those objects.
Deployment `succeeded` notifications report the derived `healthy` status; `stalled` notifications report `stalled`.
The watcher remembers the last terminal result for each Deployment UID and Pod template.
A new Pod template resets that result, and changes between terminal results emit notifications.
Replica-only scaling and repeated status updates don't repeat the same terminal result.
The initial watch listing seeds this state without emitting terminal notifications; a Deployment initially progressing can emit a notification when it finishes.
This state is held in memory and doesn't provide durable notification deduplication across restarts.
Notifications are best-effort hints for responsiveness; snapshots repair missed notifications.
Only Deployment snapshots contain container images and resolved runtime image IDs/digests.
Created, deleted, succeeded, and stalled notifications don't contain image enrichment.
This schema replaces the earlier experimental `deployment.observed`, rollout phase, and image-resolution event schema.
Consumers of that schema must migrate to namespace-scoped snapshot bodies.

### Reporting errors

The shared `reporting.error` event applies to snapshots and created/deleted/succeeded/stalled notifications.
Reasons include `payload_too_large`, `collection_failed`, and `delivery_failed`.
Size failures include `measured_bytes` and `limit_bytes`.
The component logs errors with a per-scope rate limit and increments an error counter.
If delivering an error itself fails, the component logs the failure without generating another error event.
The error path shares the output transport, so it cannot guarantee notification during an outage.

Consumers must retain the last successful inventory when reporting fails.
Error events aren't workload statuses, complete inventories, or evidence of deletion.

## Ordering and reconciliation contract

Collection timestamps provide prototype ordering under a single active collector per cluster and observation scope.
The minute-scale scan interval is expected to exceed typical node clock skew, but clock rollback during restart or rescheduling can temporarily cause fresh snapshots to compare older.
A durable monotonic epoch/sequence would provide stronger failover guarantees.
Don't numerically compare Kubernetes resource versions as a replacement for this ordering contract.

A future consumer should persist the latest accepted timestamp separately for cluster namespace inventories and each namespace UID's Deployment inventory.
Apply only newer complete snapshots; deduplicate event IDs and ignore older deliveries.
Increment an entity's missing counter only when a newer accepted parent inventory omits its UID, and reset it when present.
Missing messages, failed scans, duplicates, and stale snapshots don't advance deletion counters.
Confirming a namespace's deletion also removes its Deployments.
Retain explicit deletion tombstones so late snapshots can't resurrect deleted UIDs.
These consumer behaviors aren't implemented by this receiver or the prototype forwarding ingester.

## Delivery semantics

The component retains failed deliveries in memory and retries the original payload, ID, and collection timestamp.
It copies pdata for each delivery attempt so downstream mutation doesn't change a retry.
Oversized payloads aren't retried unchanged; the next scheduled scan retries collection.
Failed snapshot deliveries can arrive after newer ones, so consumers must enforce timestamp ordering.
Queued events don't survive a process restart unless the downstream exporter has accepted them into durable storage.

Restarting or acquiring cluster ownership synchronizes watches and immediately collects fresh inventories.
A missed Deployment deletion is repaired by its absence from namespace inventory; a missed namespace deletion is repaired by cluster inventory.
Neither recovery requires remembering the deleted object's name inside Alloy.
Run one collection owner for each scope; clustering selects one peer, but doesn't provide strict fencing during a partition.
Without clustering, prevent overlapping collectors during Pod replacement, for example with a Deployment `Recreate` strategy.

## Exported fields

`otelcol.receiver.k8s_workloads` doesn't export any fields.

## Component health

`otelcol.receiver.k8s_workloads` is reported as unhealthy if its configuration is invalid.
Runtime failures produce reporting errors and logs. Failed deliveries retry; collection retries on the next scan.

## Debug information

`otelcol.receiver.k8s_workloads` doesn't expose component-specific debug information.

## Debug metrics

The component exposes these metrics:

* `k8s_workloads_reporting_errors_total`: Reporting failures labeled by operation and reason.
* `k8s_workloads_snapshot_last_success_timestamp_seconds`: Last snapshot accepted by the downstream pipeline, labeled by entity kind and namespace. Acceptance doesn't imply Kafka or application acknowledgment.

## Examples

The following examples show how to send inventory events to a local debug exporter, a custom
OTLP/HTTP endpoint, or a Grafana Cloud OTLP/HTTP endpoint.

### Log events locally

This example logs inventory events with the OpenTelemetry debug exporter:

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

This example sends only inventory events to a configurable OTLP/HTTP endpoint:

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
Only the inventory receiver is connected to that exporter, so the endpoint doesn't affect other
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

This example sends inventory events to a Grafana Cloud API that accepts OTLP/HTTP logs at `/v1/logs`:

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

The exporter retries failed requests until they succeed because `max_elapsed_time` is `"0s"`.
The sending queue uses one consumer to preserve request order and applies backpressure when the queue is full.
The queue uses `otelcol.storage.file` with `fsync` enabled so accepted batches survive an
{{< param "PRODUCT_NAME" >}} process restart.
When you run {{< param "PRODUCT_NAME" >}} in Kubernetes, mount the storage path on persistent storage
if batches must also survive Pod replacement or rescheduling.

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
