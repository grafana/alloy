---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.sdlc/
description: Learn about otelcol.receiver.sdlc
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.sdlc
---

# `otelcol.receiver.sdlc`

`otelcol.receiver.sdlc` emits Kubernetes Deployment, StatefulSet, and DaemonSet rollouts as `grafana.sdlc.change` events, encoded as OpenTelemetry logs.
It watches workloads across all namespaces and reports their creation and four rollout transitions: `rollout_started`, `rollout_succeeded`, `rollout_stalled`, and `rollout_superseded`.
Replica-only scaling doesn't start a rollout or open image collection.
Shared Pod, ReplicaSet, and ControllerRevision watches collect runtime image identities for newly observed rollouts.
It also watches ConfigMaps and Secrets across all namespaces and reports their creation and updates.
Secret content is never read.

You can specify multiple `otelcol.receiver.sdlc` components by giving them different labels.

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Usage

```alloy
otelcol.receiver.sdlc "<LABEL>" {
  output {
    logs = <OTEL_LOG_CONSUMER_LIST>
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.sdlc`:

| Name           | Type     | Description                                      | Default | Required |
| -------------- | -------- | ------------------------------------------------ | ------- | -------- |
| `cluster_name` | `string` | Human-readable name of the Kubernetes cluster. Used as the provider ID. | `""`    | no       |
| `cluster_uid`  | `string` | Stable identifier for the Kubernetes cluster.    | `""`    | no       |

When `cluster_uid` is empty, the component uses the UID of the `kube-system` Namespace as the cluster UID.
When `cluster_name` is empty, the component uses the cluster UID as the provider ID.
Set `cluster_name` for a readable provider ID.
Adding it later changes the provider ID of subsequent events.

## Blocks

You can use the following blocks with `otelcol.receiver.sdlc`:

{{< docs/alloy-config >}}

| Block                                       | Description                                                  | Required |
| ------------------------------------------- | ------------------------------------------------------------ | -------- |
| [`client`][client]                          | Configures the Kubernetes client.                                      | no       |
| `client` > [`authorization`][authorization] | Configures generic authorization for Kubernetes API requests.          | no       |
| `client` > [`basic_auth`][basic_auth]       | Configures basic authentication for Kubernetes API requests.           | no       |
| `client` > [`oauth2`][oauth2]               | Configures OAuth 2.0 authentication for Kubernetes API requests.       | no       |
| `client` > [`tls_config`][tls_config]       | Configures TLS for connections to the Kubernetes API server.           | no       |
| [`clustering`][clustering]                  | Configures cluster-wide ownership of the Kubernetes watcher.           | no       |
| [`configmaps`][configmaps]                  | Configures ConfigMap change tracking.                                  | no       |
| [`daemonsets`][daemonsets]                  | Configures DaemonSet rollout tracking.                                 | no       |
| [`output`][output]                          | Configures where to send workload events.                               | yes      |
| [`secrets`][secrets]                        | Configures Secret change tracking.                                     | no       |
| [`statefulsets`][statefulsets]              | Configures StatefulSet rollout tracking.                               | no       |

[authorization]: #authorization
[basic_auth]: #basic_auth
[client]: #client
[clustering]: #clustering
[configmaps]: #configmaps
[daemonsets]: #daemonsets
[oauth2]: #oauth2
[output]: #output
[secrets]: #secrets
[statefulsets]: #statefulsets
[tls_config]: #tls_config

{{< /docs/alloy-config >}}

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
The replacement member establishes a fresh baseline from the Kubernetes API without replaying existing rollouts.
Before starting the watcher, a new owner waits for 30 seconds without a cluster membership notification, up to a maximum of 90 seconds.
This delay reduces watcher restarts while members join, but doesn't guarantee exclusive ownership during a network partition.

If {{< param "PRODUCT_NAME" >}} isn't running in clustered mode, the block has no effect and every configured instance watches the Kubernetes cluster.

### `configmaps`

The `configmaps` block configures ConfigMap change tracking:

| Name      | Type     | Description                                               | Default  | Required |
| --------- | -------- | --------------------------------------------------------- | -------- | -------- |
| `content` | `string` | How much of the `data` and `binaryData` fields to report. | `"keys"` | no       |
| `enabled` | `bool`   | Track ConfigMaps in all namespaces.                       | `true`   | no       |

`content` accepts the following values:

* `all`: Report changed keys with their `before` and `after` values.
* `keys`: Report changed keys without values.
* `metadata`: Don't report `data` or `binaryData`. The component reads ConfigMaps through the metadata API, so their content never reaches {{< param "PRODUCT_NAME" >}}.

ConfigMaps can contain credentials.
Use `all` only when every ConfigMap in the cluster can be exported to the `output` destination.
With `all`, {{< param "PRODUCT_NAME" >}} keeps the content of every ConfigMap in memory.

### `daemonsets`

The `daemonsets` block configures DaemonSet rollout tracking:

| Name      | Type   | Description                                 | Default | Required |
| --------- | ------ | ------------------------------------------- | ------- | -------- |
| `enabled` | `bool` | Track DaemonSet rollouts in all namespaces. | `true`  | no       |

DaemonSet revisions are stored as ControllerRevisions, so the component also watches ControllerRevisions while DaemonSet tracking is enabled.

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block-logs.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `secrets`

The `secrets` block configures Secret change tracking:

| Name      | Type   | Description                      | Default | Required |
| --------- | ------ | -------------------------------- | ------- | -------- |
| `enabled` | `bool` | Track Secrets in all namespaces. | `true`  | no       |

The component reads Secrets through the metadata API, so Secret content never reaches {{< param "PRODUCT_NAME" >}}.
The API server still requires `list` and `watch` permissions on Secrets.

### `statefulsets`

The `statefulsets` block configures StatefulSet rollout tracking:

| Name      | Type   | Description                                   | Default | Required |
| --------- | ------ | --------------------------------------------- | ------- | -------- |
| `enabled` | `bool` | Track StatefulSet rollouts in all namespaces. | `true`  | no       |

StatefulSet revisions are stored as ControllerRevisions, so the component also watches ControllerRevisions while StatefulSet tracking is enabled.

## Kubernetes permissions

The component lists and watches Deployments, ReplicaSets, StatefulSets, DaemonSets, ControllerRevisions, Pods, ConfigMaps, and Secrets across all namespaces.
If `cluster_uid` is empty, it also reads the `kube-system` Namespace once to discover the cluster UID.
Grant its service account the following permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: alloy-sdlc
rules:
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets", "statefulsets", "daemonsets", "controllerrevisions"]
    verbs: ["list", "watch"]
  - apiGroups: [""]
    resources: ["pods", "configmaps", "secrets"]
    verbs: ["list", "watch"]
  - apiGroups: [""]
    resources: ["namespaces"]
    resourceNames: ["kube-system"]
    verbs: ["get"]
```

You can omit the Namespace permission when you configure `cluster_uid`.
You can omit the ConfigMap or Secret permission when you disable the `configmaps` or `secrets` block.
You can omit the StatefulSet or DaemonSet permission when you disable the `statefulsets` or `daemonsets` block.
You can omit the ControllerRevision permission when you disable both blocks.
Without a required permission, the watcher doesn't become ready and emits no events.

## Event schema

Each event is one `grafana.sdlc.change` OTLP log record, version `1` of the SDLC change contract.
The log record has no body.
All attributes use the `grafana.sdlc.` prefix, which the following tables omit.

The component doesn't set a stack ID.
Send events to an ingest endpoint that is specific to your Grafana Cloud stack.

### Resource attributes

| Attribute | Value |
| --- | --- |
| `provider.type` | `k8s`. |
| `provider.id` | The `cluster_name`, or the cluster UID when `cluster_name` isn't set. |
| `provider.resource.type` | `deployment`, `statefulset`, `daemonset`, `configmap`, or `secret`. |
| `provider.resource.id` | `<namespace>/<name>` of the object. |
| `provider.resource.change.type` | The kind of change, described in the following sections. |

### Log record fields and attributes

| Field or attribute | Value |
| --- | --- |
| `eventName` | `grafana.sdlc.change`. |
| `timeUnixNano` | The event end time: the watcher's observation time, not an exact Kubernetes transition time. |
| `observedTimeUnixNano` | The same observation time. |
| `traceId` | For rollouts, derived from the rollout ID, so all events of one rollout share one trace. Not set for workload `created` events, ConfigMaps, and Secrets. `spanId` isn't set. |
| `schema.version` | `"1"`. |
| `provider.version` | The {{< param "PRODUCT_NAME" >}} version. |
| `event.id` | A deterministic hash that identifies the event. Retries preserve it. |
| `status` | Always `UNSET`. The kind of change is in `provider.resource.change.type`. |
| `start_time_unix_nano` | The event start time. |
| `type` | `intent` for `rollout_started`. `effect` for every other event. |
| `change.summary` | A human-readable description of the change. It doesn't include the resource name. |
| `change.version.before` | The previous workload revision, or the previous ConfigMap or Secret resource version. Omitted when unknown. |
| `change.version.after` | The workload revision of this rollout, the workload generation for a workload `created` event, or the ConfigMap or Secret resource version. |
| `change.diff` | A map of change details, described in the following sections. Omitted when it has no entries. |

The component doesn't set `service.ids`, `environment.id`, `owners`, `actor`, `tracing.enabled`, `logging.enabled`, `change.url`, or `change.git.*`.
Because `tracing.enabled` isn't set, the backend emits a span for each event.
Each rollout event is a span in the rollout's trace.

### Workload creation events

When the watcher observes a Deployment, StatefulSet, or DaemonSet that didn't exist in its baseline, it emits an event with `provider.resource.change.type` set to `created` and `change.summary` set to `Created`.
The event is a point in time and has no `change.diff`.
Its first rollout is reported separately by a `rollout_started` event.
The `event.id` is a deterministic hash of the cluster UID, workload UID, and event type.

### Rollout events

The component emits the following lifecycle events:

| `provider.resource.change.type` | `change.summary` | Meaning |
| --- | --- | --- |
| `rollout_started` | `Rollout started` | The watcher observes a new controller-assigned revision after the controller observes the current generation. |
| `rollout_succeeded` | `Rollout succeeded` | The rollout has completed, as described in the following list. |
| `rollout_stalled` | `Rollout stalled: ProgressDeadlineExceeded` | Deployments only. The controller reports `Progressing=False` with reason `ProgressDeadlineExceeded`. |
| `rollout_superseded` | `Rollout superseded by revision <N>` | A newer revision replaces an unfinished rollout. |

A rollout succeeds under the following conditions:

* Deployment: the controller reports `NewReplicaSetAvailable`, all desired replicas are updated and available, and no old replicas remain in the replica count.
* StatefulSet: all desired replicas are available, and every Pod the update strategy replaces runs the update revision.
  With a `RollingUpdate` strategy and no partition, the controller must also promote the update revision to the current revision.
  With a partition, only ordinals at or above the partition are updated, so the rollout succeeds once those Pods run the update revision.
  With an `OnDelete` strategy, Pods change only when they're deleted, so the rollout can stay open for a long time.
* DaemonSet: every node that should run the DaemonSet runs an updated, available Pod.
  A DaemonSet scheduled on no nodes succeeds immediately.
  With an `OnDelete` strategy, the rollout can stay open for a long time, as for StatefulSets.

StatefulSets and DaemonSets have no progress deadline, so their rollouts never stall.
They remain open until they succeed or are superseded.

`rollout_started` is a point in time: `start_time_unix_nano` equals `timeUnixNano`.
The other events start when the watcher observed the rollout start, so their duration covers the rollout.
An in-progress rollout found during startup has no recorded start, and its events are points in time.

Every `change.diff` contains `revision`, `rollout_id`, `namespace`, and `containers`.
When configured, it also contains `cluster_name`.
Container entries contain `name`, `init`, and the configured `image` from that rollout's template.
Runtime image identities are reported separately by the image collection event.
A `rollout_superseded` event also contains `superseded_by`, the replacement rollout ID.
StatefulSet and DaemonSet events also contain `update_strategy`.
StatefulSet events also contain `partition`, the lowest ordinal that receives the update.

The revision of a Deployment is its `deployment.kubernetes.io/revision` annotation.
The revision of a StatefulSet is the `revision` number of the ControllerRevision named by its update revision.
The revision of a DaemonSet is the highest `revision` number among its ControllerRevisions.
`rollout_id` is a deterministic hash of the cluster UID, workload UID, and controller-assigned revision.
Consumers can deduplicate using the event ID.
A rollback receives a new revision and therefore a new rollout ID, even when it reuses an older template.

A stalled rollout can subsequently succeed or be superseded.
Each event is emitted at most once per tracked rollout.
Success closes the rollout; later replica scaling or availability changes don't reopen it.
Paused Deployments and generations the controller hasn't observed don't emit transitions.
A template change while paused starts a rollout only after the controller applies it on resume.

### Change details

The `rollout_started` `change.diff` includes `changes_known`, which indicates whether a previous template is available for comparison.
When a previous template is available, `changes` describes the changed fields.
`changes` is omitted when no tracked field changed, including when no previous template is available.
The baseline is the previously observed rollout template; changes aren't reconstructed from historical ReplicaSets.

`changes` is a map keyed by field path.
Container fields use `containers.<name>.<field>`, or `initContainers.<name>.<field>` for init containers.
Each value contains `operation` (`added`, `removed`, or `modified`) and, for fields that are safe to report, `before` and `after`.
An omitted value indicates absence.

The following fields report values:

* Container `image`, `imagePullPolicy`, `ports`, and `resources.requests.*` and `resources.limits.*`.
* Pod `metadata.labels.<key>`, `spec.serviceAccountName`, and `spec.nodeSelector.<key>`.

Environment variables report only which variable changed, as `containers.<name>.env.<NAME>`, without values.
Other configuration changes report enclosing field names without values, such as `args`, `metadata.annotations`, or `spec.volumes`.
Container additions and removals use the container path, such as `containers.<name>`.
Reordering uses `spec.containers.order` or `spec.initContainers.order`.

For example, an image update and a CPU request increase produce this `change.diff`:

```json
{
  "revision": 2,
  "rollout_id": "<hash>",
  "namespace": "shop",
  "containers": [{"name": "api", "init": false, "image": "api:v2"}],
  "changes_known": true,
  "changes": {
    "containers.api.image": {"operation": "modified", "before": "api:v1", "after": "api:v2"},
    "containers.api.resources.requests.cpu": {"operation": "modified", "before": "250m", "after": "500m"}
  }
}
```

### ConfigMap and Secret events

ConfigMap and Secret events are points in time with the following values:

| `provider.resource.change.type` | `change.summary` | Meaning |
| --- | --- | --- |
| `created` | `Created` | The watcher observes an object that didn't exist in its baseline. |
| `updated` | `Updated` | A tracked field of the object changes. |

Deleting a ConfigMap or Secret doesn't emit an event.
The `event.id` is a deterministic hash of the cluster UID, object UID, resource version, and event type.

`change.diff` contains only `changes`, a map keyed by field path in the same format as rollout changes.
A `created` event reports every tracked field as `added`.
A `created` event has no `change.diff` when the object has no tracked fields, such as a Secret without labels or annotations.
The following fields are tracked:

* `metadata.labels.<key>`: Includes `before` and `after` values.
* `metadata.annotations.<key>`: Never includes values. Annotations such as `kubectl.kubernetes.io/last-applied-configuration` can contain the full manifest, including Secret data.
* `data.<key>` and `binaryData.<key>`: Include `before` and `after` values only when the [`configmaps`][configmaps] `content` is `all`. `binaryData` values are base64-encoded.
* `metadata.ownerReferences`, `metadata.finalizers`, and `metadata.deletionTimestamp`: Never include values.
* `immutable`: Includes `before` and `after` values.

For example, a label change and a data change with `content` set to `keys` produce this `change.diff`:

```json
{
  "changes": {
    "metadata.labels.team": {"operation": "modified", "before": "payments", "after": "checkout"},
    "data.LOG_LEVEL": {"operation": "modified"}
  }
}
```

The metadata API doesn't return content.
For Secrets, and for ConfigMaps with `content` set to `metadata`, a new resource version without a tracked metadata change is reported as a `data` change with operation `modified`.
This change is inferred: an update that only changes managed fields is also reported this way, and a content change made together with a metadata change reports only the metadata change.

When `content` is `all` and an event would exceed 2 MiB, the component omits the `data` and `binaryData` values and reports only their operations.

### Resolved container images

The component emits one image event when image collection closes for a newly observed rollout.
Its `provider.resource.change.type` is `rollout_images_resolved`.
Its `change.summary` is `Resolved runtime images: <container>=<image ID>, ...`, or `Partially resolved runtime images: ...` when `complete` is false.
Init containers are prefixed with `init:`.
The summary is limited to 1024 bytes; `change.diff` has the complete list.

Collection opens when a new revision is observed, remains open while stalled, and
closes on success or supersession. Initial discovery and later replica scaling
don't open collection.

Deployment Pods are associated through their controller ReplicaSet and its Deployment owner
and revision. StatefulSet and DaemonSet Pods are associated through their workload owner and
their `controller-revision-hash` label. The receiver collects distinct runtime image IDs for each regular
and init container. At closure, bounded API reads collect available identities
that may not yet have reached the watches. Lifecycle events don't wait for these
reads. Pods created after the closing observation are excluded.

The `change.diff` contains the common rollout fields, `complete`, and `resolved_containers`.
Each resolved container has `name`, `init`, `image` (the requested image),
and `runtime_image_ids` (a sorted array of distinct Kubernetes image IDs).
`complete` is false on supersession, failed final reads, missing identities,
insufficient observed Pods, or a zero-replica rollout. The field describes the
closing observation, not a guarantee that every future replica uses those images.

Runtime image IDs are preserved as reported by Kubernetes. They aren't guaranteed
to identify the same manifest or index as a build's provenance digest.
The event ID is deterministic for the rollout; delivery retries preserve its content.
Later scaling doesn't reopen collection or emit another image event.

### Delivery behavior

The initial watch listing establishes an in-memory baseline without emitting events.
An in-progress rollout found during startup can subsequently emit an outcome without a preceding `rollout_started` event.
Restarts and ownership changes don't replay missed events, and intermediate revisions can be missed between observations.
Objects created while the watcher is stopped become part of the baseline and don't emit a `created` event.
Deleting a workload emits no event and discards its tracked rollout and image collection state.
ConfigMap and Secret changes made while the watcher is stopped aren't reported, and consecutive updates between observations are reported as one change.
The component doesn't persist state or reconcile historical rollouts.

Downstream delivery failures are logged and retried with the original event ID and timestamp.
The queue is in memory and is lost on restart.
Retries can arrive out of order, so consumers must use rollout IDs instead of assuming arrival order.
The component doesn't emit reporting-error events or heartbeats.

## Exported fields

`otelcol.receiver.sdlc` doesn't export any fields.

## Component health

`otelcol.receiver.sdlc` is reported as unhealthy if its configuration is invalid.
Delivery failures are logged and retried.
Watcher startup failures are logged and retried.

## Debug information

`otelcol.receiver.sdlc` doesn't expose component-specific debug information.

## Debug metrics

`otelcol.receiver.sdlc` doesn't expose component-specific debug metrics.

## Examples

The following examples show how to send workload events to a local debug exporter, a custom
OTLP/HTTP endpoint, or a Grafana Cloud OTLP/HTTP endpoint.

### Log events locally

This example logs workload events with the OpenTelemetry debug exporter:

```alloy
otelcol.receiver.sdlc "default" {
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
otelcol.receiver.sdlc "default" {
  cluster_name = sys.env("K8S_CLUSTER_NAME")

  output {
    logs = [otelcol.exporter.otlphttp.rollout_api.input]
  }
}

otelcol.exporter.otlphttp "rollout_api" {
  client {
    endpoint = sys.env("ROLLOUT_OTLP_ENDPOINT")
    auth     = otelcol.auth.basic.rollout_api.handler
  }

  retry_on_failure {
    max_elapsed_time = "0s"
  }
}

otelcol.auth.basic "rollout_api" {
  client_auth {
    username = sys.env("SDLC_STACK_ID")
    password = sys.env("SDLC_CAP_TOKEN")
  }
}
```

For the SDLC prototype ingester, set `ROLLOUT_OTLP_ENDPOINT` to a base URL that
includes `/workloads`, such as `http://localhost:4318/workloads` for local testing.
Set `SDLC_STACK_ID` to the target Grafana Cloud stack ID and `SDLC_CAP_TOKEN` to a
Grafana Cloud access policy token with `traces:write`. The policy can have one stack
realm matching that stack, or one org realm for its owning organization.
Use HTTPS when sending the token over a network.
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
`example/sdlc/local-api.alloy`.

### Send events to Grafana Cloud

This example sends workload events to the SDLC prototype ingester at `/workloads/v1/logs` using a Grafana Cloud access policy token:

```alloy
otelcol.receiver.sdlc "default" {
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
    username = sys.env("GRAFANA_CLOUD_STACK_ID")
    password = sys.env("GRAFANA_CLOUD_API_KEY")
  }
}

otelcol.storage.file "rollouts" {
  fsync = true
}
```

Set the following environment variables:

* `K8S_CLUSTER_NAME`: Human-readable name of the Kubernetes cluster.
* `GRAFANA_CLOUD_OTLP_ENDPOINT`: Ingester base URL including `/workloads`, for example `https://sdlc.example.com/workloads`.
  `otelcol.exporter.otlphttp` appends `/v1/logs`, producing `/workloads/v1/logs`.
* `GRAFANA_CLOUD_STACK_ID`: Grafana Cloud stack ID used as the basic authentication username.
  Use the target stack ID, not a Loki tenant or instance ID.
* `GRAFANA_CLOUD_API_KEY`: Grafana Cloud access policy token with `traces:write` and one stack realm matching `GRAFANA_CLOUD_STACK_ID`, or one org realm for the organization that owns that stack.

The prototype ingester validates the token. Events carry no stack ID, so the endpoint
must be specific to the target stack.
For an org-wide token, the ingester additionally checks the target stack's ownership
against Auth Cache. It rejects requests for a stack owned by another organization.
The ingester needs `AUTH_CACHE_URL` configured for org-wide tokens; no additional
CAP scopes beyond `traces:write` are required.
Use a token from the same environment as the ingester's Auth API.

This example configures retries and a persistent sending queue in the exporter.
For exporter options, refer to [`otelcol.exporter.otlphttp`](../otelcol.exporter.otlphttp/).

For a reusable opt-in custom component, use the module in `example/sdlc/module.alloy`.
Importing the file only defines the custom component; instantiate `deployment_rollouts` to start the watcher.
For a complete Grafana Cloud configuration, use `example/sdlc/grafana-cloud.alloy`.
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.sdlc` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
