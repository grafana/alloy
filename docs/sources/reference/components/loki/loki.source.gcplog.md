---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.gcplog/
aliases:
  - ../loki.source.gcplog/ # /docs/alloy/latest/reference/components/loki.source.gcplog/
description: Learn about loki.source.gcplog
title: loki.source.gcplog
---

# loki.source.gcplog

`loki.source.gcplog` retrieves logs from cloud resources such as GCS buckets,
load balancers, or Kubernetes clusters running on GCP by making use of Pub/Sub
[subscriptions](https://cloud.google.com/pubsub/docs/subscriber).

The component uses either the 'push' or 'pull' strategy to retrieve log
entries and forward them to the list of receivers in `forward_to`.

Multiple `loki.source.gcplog` components can be specified by giving them different labels.

## Usage

```alloy
loki.source.gcplog "LABEL" {
  pull {
    project_id   = "PROJECT_ID"
    subscription = "SUB_ID"
  }

  forward_to = RECEIVER_LIST
}
```

## Arguments

`loki.source.gcplog` supports the following arguments:

| Name            | Type                 | Description                               | Default | Required |
|-----------------|----------------------|-------------------------------------------|---------|----------|
| `forward_to`    | `list(LogsReceiver)` | List of receivers to send log entries to. |         | yes      |
| `relabel_rules` | `RelabelRules`       | Relabeling rules to apply on log entries. | "{}"    | no       |

## Blocks

The following blocks are supported inside the definition of
`loki.source.gcplog`:

| Hierarchy   | Name     | Description                                                                   | Required |
|-------------|----------|-------------------------------------------------------------------------------|----------|
| pull        | [pull][] | Configures a target to pull logs from a GCP Pub/Sub subscription.             | no       |
| push        | [push][] | Configures a server to receive logs as GCP Pub/Sub push requests.             | no       |
| push > http | [http][] | Configures the HTTP server that receives requests when using the `push` mode. | no       |
| push > grpc | [grpc][] | Configures the gRPC server that receives requests when using the `push` mode. | no       |

The `pull` and `push` inner blocks are mutually exclusive; a component must
contain exactly one of the two in its definition. The `http` and `grpc` block
are just used when the `push` block is configured.

[pull]: #pull-block
[push]: #push-block
[http]: #http
[grpc]: #grpc

### pull block

The `pull` block defines which GCP project ID and subscription to read log
entries from.

The following arguments can be used to configure the `pull` block. Any omitted
fields take their default values.

| Name                     | Type          | Description                                                               | Default | Required |
|--------------------------|---------------|---------------------------------------------------------------------------|---------|----------|
| `project_id`             | `string`      | The GCP project id the subscription belongs to.                           |         | yes      |
| `subscription`           | `string`      | The subscription to pull logs from.                                       |         | yes      |
| `labels`                 | `map(string)` | Additional labels to associate with incoming logs.                        | `"{}"`  | no       |
| `use_incoming_timestamp` | `bool`        | Whether to use the incoming log timestamp.                                | `false` | no       |
| `use_full_line`          | `bool`        | Send the full line from Cloud Logging even if `textPayload` is available. | `false` | no       |

To make use of the `pull` strategy, the GCP project must have been
[configured](/docs/loki/next/clients/promtail/gcplog-cloud/)
to forward its cloud resource logs onto a Pub/Sub topic for
`loki.source.gcplog` to consume.

Typically, the host system also needs to have its GCP
[credentials](https://cloud.google.com/docs/authentication/application-default-credentials)
configured. One way to do it is to point the `GOOGLE_APPLICATION_CREDENTIALS`
environment variable to the location of a credential configuration JSON file or
a service account key.

### push block

The `push` block defines the configuration of the server that receives
push requests from GCP's Pub/Sub servers.

The following arguments can be used to configure the `push` block. Any omitted
fields take their default values.

| Name                        | Type          | Description                                                                        | Default | Required |
|-----------------------------|---------------|------------------------------------------------------------------------------------|---------|----------|
| `graceful_shutdown_timeout` | `duration`    | Timeout for servers graceful shutdown. If configured, should be greater than zero. | "30s"   | no       |
| `push_timeout`              | `duration`    | Sets a maximum processing time for each incoming GCP log entry.                    | `"0s"`  | no       |
| `labels`                    | `map(string)` | Additional labels to associate with incoming entries.                              | `"{}"`  | no       |
| `use_incoming_timestamp`    | `bool`        | Whether to use the incoming entry timestamp.                                       | `false` | no       |
| `use_full_line`             | `bool`        | Send the full line from Cloud Logging even if `textPayload` is available. By default, if `textPayload` is present in the line, then it's used as log line | `false` | no  |

The server listens for POST requests from GCP's Push subscriptions on `HOST:PORT/gcp/api/v1/push`.

By default, for both strategies the component assigns the log entry timestamp
as the time it was processed, except if `use_incoming_timestamp` is set to true.

The `labels` map is applied to every entry that passes through the component.

### http

{{< docs/shared lookup="reference/components/loki-server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

### grpc

{{< docs/shared lookup="reference/components/loki-server-grpc.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`loki.source.gcplog` does not export any fields.

## Component health

`loki.source.gcplog` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`loki.source.gcplog` exposes some debug information per gcplog listener:
* The configured strategy.
* Their label set.
* When using a `push` strategy, the listen address.

## Debug metrics

When using the `pull` strategy, the component exposes the following debug
metrics:
* `loki_source_gcplog_pull_entries_total` (counter): Number of entries received by the gcplog target.
* `loki_source_gcplog_pull_parsing_errors_total` (counter): Total number of parsing errors while receiving gcplog messages.
* `loki_source_gcplog_pull_last_success_scrape` (gauge): Timestamp of target's last successful poll.

When using the `push` strategy, the component exposes the following debug
metrics:
* `loki_source_gcplog_push_entries_total` (counter): Number of entries received by the gcplog target.
* `loki_source_gcplog_push_entries_total` (counter): Number of parsing errors while receiving gcplog messages.


## Example

This example listens for GCP Pub/Sub PushRequests on `0.0.0.0:8080` and
forwards them to a `loki.write` component.

```alloy
loki.source.gcplog "local" {
  push {}

  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```

On the other hand, if we need the server to listen on `0.0.0.0:4040`, and forwards them
to a `loki.write` component.

```alloy
loki.source.gcplog "local" {
  push {
    http {
        listen_port = 4040
    }
  }

  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.gcplog` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
