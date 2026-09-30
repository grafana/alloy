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

`otelcol.receiver.k8s_workloads` emits Kubernetes Deployment rollout events as OpenTelemetry logs.
It watches Deployments across all namespaces and reports four transitions: `started`, `succeeded`, `stalled`, and `superseded`.
Replica-only scaling doesn't start a rollout.

You can specify multiple `otelcol.receiver.k8s_workloads` components by giving them different labels.

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

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
| `max_event_bytes` | `number` | Exclusive upper limit for a serialized event; at least 8192. | `524288` | no |

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
| [`output`][output]                          | Configures where to send workload events.                               | yes      |

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

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block-logs.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Kubernetes permissions

The component lists and watches Deployments across all namespaces.
If `cluster_uid` is empty, it also reads the `kube-system` Namespace once to discover the cluster UID.
Grant its service account the following permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: alloy-k8s-workloads
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["list", "watch"]
  - apiGroups: [""]
    resources: ["namespaces"]
    resourceNames: ["kube-system"]
    verbs: ["get"]
```

You can omit the Namespace permission when you configure `cluster_uid`.

## Event schema

Each event is one OTLP log record with a JSON string body.
Resource attributes include `k8s.cluster.uid`, `k8s.namespace.name`, `k8s.deployment.uid`, and `k8s.deployment.name`.
When configured, `k8s.cluster.name` is also present.
Each record has a `grafana.sdlc.event.id` and `grafana.sdlc.schema.version` set to `1`.
The record timestamp and body `observed_at` are the watcher's observation time, not an exact Kubernetes transition time.

### Rollout events

The component emits only the following event types:

| Event name | Meaning |
| --- | --- |
| `grafana.sdlc.k8s.deployment.rollout.started` | The watcher observes a new controller-assigned revision after the controller observes the current generation. |
| `grafana.sdlc.k8s.deployment.rollout.succeeded` | The controller reports `NewReplicaSetAvailable`, all desired replicas are updated and available, and no old replicas remain in the replica count. |
| `grafana.sdlc.k8s.deployment.rollout.stalled` | The controller reports `Progressing=False` with reason `ProgressDeadlineExceeded`. |
| `grafana.sdlc.k8s.deployment.rollout.superseded` | A newer revision replaces an unfinished rollout. |

Every body contains `name`, `uid`, `rollout_id`, `revision`, `status`, `observed_at`, and `containers`.
Container entries contain `name`, `init`, and the configured `image` from that rollout's template.
The component doesn't read Pods to resolve runtime image digests.
A `superseded` event also contains `superseded_by`, the replacement rollout ID.

`rollout_id` is a deterministic hash of the cluster UID, Deployment UID, and controller-assigned revision.
`grafana.sdlc.event.id` is a deterministic hash of the rollout ID and event status.
Consumers can deduplicate using stack ID and event ID.
A rollback receives a new revision and therefore a new rollout ID, even when it reuses an older template.

A stalled rollout can subsequently succeed or be superseded.
Each status is emitted at most once per tracked rollout.
Success closes the rollout; later replica scaling or availability changes don't reopen it.
Paused Deployments and generations the controller hasn't observed don't emit transitions.
A template change while paused starts a rollout only after the controller applies it on resume.

### Change summaries

The `started` body includes `changes_known`, which indicates whether a previous template is available for comparison.
When available, `change_types` contains any of `image`, `resources`, `container`, and `configuration`, and `changes` describes the changed fields.
Empty change lists are omitted.
The baseline is the previously observed rollout template; changes aren't reconstructed from historical ReplicaSets.

Each change contains `field` and `operation` (`added`, `removed`, or `modified`).
Container changes also contain `container`, with `init: true` for init containers.
Image and resource request/limit changes include `before` and `after` values when present.
For these fields, an omitted value indicates absence.
Other configuration changes report enclosing field names without values, such as `env`, `args`, `metadata.annotations`, or `spec.volumes`.
Container additions and removals use the `container` field; reordering uses `spec.containers.order` or `spec.initContainers.order`.

For example, an image update and a CPU request increase produce these change fields:

```json
{
  "changes_known": true,
  "change_types": ["image", "resources"],
  "changes": [
    {"container": "api", "field": "image", "operation": "modified", "before": "api:v1", "after": "api:v2"},
    {"container": "api", "field": "resources.requests.cpu", "operation": "modified", "before": "250m", "after": "500m"}
  ]
}
```

### Delivery behavior

The initial watch listing establishes an in-memory baseline without emitting events.
An in-progress rollout found during startup can subsequently emit an outcome without a preceding `started` event.
Restarts and ownership changes don't replay missed events, and intermediate revisions can be missed between observations.
Deleting a Deployment discards its tracked state without emitting an event.
The component doesn't persist state or reconcile historical rollouts.

Downstream delivery failures are logged and retried with the original event ID and timestamp.
The queue is in memory and is lost on restart.
Retries can arrive out of order, so consumers must use rollout IDs instead of assuming arrival order.
Events whose JSON or Protocol Buffers OTLP encoding reaches `max_event_bytes` are dropped with a local error log.
The component doesn't emit reporting-error events.

## Exported fields

`otelcol.receiver.k8s_workloads` doesn't export any fields.

## Component health

`otelcol.receiver.k8s_workloads` is reported as unhealthy if its configuration is invalid.
Delivery failures are logged and retried.
Watcher startup failures are logged and retried.

## Debug information

`otelcol.receiver.k8s_workloads` doesn't expose component-specific debug information.

## Debug metrics

`otelcol.receiver.k8s_workloads` doesn't expose component-specific debug metrics.

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
Grafana Cloud access policy token with `logs:write`. The policy can have one stack
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
`example/k8s-workloads/local-api.alloy`.

### Send events to Grafana Cloud

This example sends workload events to the SDLC prototype ingester at `/workloads/v1/logs` using a Grafana Cloud access policy token:

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
* `GRAFANA_CLOUD_API_KEY`: Grafana Cloud access policy token with `logs:write` and one stack realm matching `GRAFANA_CLOUD_STACK_ID`, or one org realm for the organization that owns that stack.

The prototype ingester validates the token and adds the trusted stack ID as the
`grafana.stack.id` resource attribute before publishing events.
For an org-wide token, the ingester additionally checks the target stack's ownership
against Auth Cache. It rejects requests for a stack owned by another organization.
The ingester needs `AUTH_CACHE_URL` configured for org-wide tokens; no additional
CAP scopes beyond `logs:write` are required.
Use a token from the same environment as the ingester's Auth API.

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
