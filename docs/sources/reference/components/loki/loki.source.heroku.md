---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.heroku/
aliases:
  - ../loki.source.heroku/ # /docs/alloy/latest/reference/components/loki.source.heroku/
description: Learn about loki.source.heroku
labels:
  stage: general-availability
  products:
    - oss
title: loki.source.heroku
---

# `loki.source.heroku`

`loki.source.heroku` listens for Heroku messages over TCP connections and forwards them to other `loki.*` components.

The component starts a new Heroku listener for the given `listener` block and fans out incoming entries to the list of receivers in `forward_to`.

Before using `loki.source.heroku`, Heroku should be configured with the URL where {{< param "PRODUCT_NAME" >}} is listening.
Follow the steps in [Heroku HTTPS Drain docs](https://devcenter.heroku.com/articles/log-drains#https-drains) for using the Heroku CLI with a command like the following:

```shell
heroku drains:add [http|https]://HOSTNAME:PORT/heroku/api/v1/drain -a HEROKU_APP_NAME
```

You can specify multiple `loki.source.heroku` components by giving them different labels.

## Usage

```alloy
loki.source.heroku "<LABEL>" {
    http {
        listen_address = "<LISTEN_ADDRESS>"
        listen_port    = "<LISTEN_PORT>"
    }
    forward_to = <RECEIVER_LIST>
}
```

## Arguments

You can use the following arguments with `loki.source.heroku`:

| Name                        | Type                 | Description                                                                        | Default | Required |
| --------------------------- | -------------------- | ---------------------------------------------------------------------------------- | ------- | -------- |
| `forward_to`                | `list(LogsReceiver)` | List of receivers to send log entries to.                                          |         | yes      |
| `graceful_shutdown_timeout` | `duration`           | Timeout for servers graceful shutdown. If configured, should be greater than zero. | `"30s"` | no       |
| `labels`                    | `map(string)`        | The labels to associate with each received Heroku record.                          | `{}`    | no       |
| `relabel_rules`             | `RelabelRules`       | Relabeling rules to apply on log entries.                                          | `{}`    | no       |
| `use_incoming_timestamp`    | `bool`               | Whether to use the timestamp received from Heroku.                                 | `false` | no       |

The `relabel_rules` field can make use of the `rules` export value from a `loki.relabel` component to apply one or more relabeling rules to log entries before they're forwarded to the list of receivers in `forward_to`.

## Blocks

You can use the following blocks with `loki.source.heroku`:

| Name                  | Description                                        | Required |
| --------------------- | -------------------------------------------------- | -------- |
| [`grpc`][grpc]        | Configures the gRPC server that receives requests. | no       |
| `gprc` > [`tls`][tls] | Configures TLS for the gRPC server.                | no       |
| [`http`][http]        | Configures the HTTP server that receives requests. | no       |
| `http` > [`tls`][tls] | Configures TLS for the HTTP server.                | no       |

The > symbol indicates deeper levels of nesting.
For example, `http` > `tls` refers to a `tls` block defined inside an `http` block.

[http]: #http
[grpc]: #grpc
[tls]: #tls

### `grpc`

{{< docs/shared lookup="reference/components/loki-server-grpc.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `http`

{{< docs/shared lookup="reference/components/server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS for the HTTP and gRPC servers.

{{< docs/shared lookup="reference/components/server-tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Labels

The `labels` map is applied to every message that the component reads.

The following internal labels all prefixed with `__` are available but are discarded if not relabeled:

* `__heroku_drain_app`
* `__heroku_drain_host`
* `__heroku_drain_log_id`
* `__heroku_drain_proc`

All URL query parameters are translated to `__heroku_drain_param_<name>`

If the `X-Scope-OrgID` header is set it's translated to `__tenant_id__`

## Exported fields

`loki.source.heroku` doesn't export any fields.

## Component health

`loki.source.heroku` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`loki.source.heroku` exposes some debug information per Heroku listener:

* Whether the listener is currently running.
* The listen address.

## Debug metrics

* `loki_source_heroku_drain_entries_total` (counter): Number of successful entries received by the Heroku target.
* `loki_source_heroku_drain_parsing_errors_total` (counter): Number of parsing errors while receiving Heroku messages.

## Example

This example listens for Heroku messages over TCP in the specified port and forwards them to a `loki.write` component using the Heroku timestamp.

```alloy
loki.source.heroku "local" {
    http {
        listen_address = "0.0.0.0"
        listen_port    = 4040
    }
    use_incoming_timestamp = true
    labels                 = {component = "loki.source.heroku"}
    forward_to             = [loki.write.local.receiver]
}

loki.write "local" {
    endpoint {
        url = "loki:3100/api/v1/push"
    }
}
```

When using the default `http` block settings, the server listen for new connection on port `8080`.

```alloy
loki.source.heroku "local" {
    use_incoming_timestamp = true
    labels                 = {component = "loki.source.heroku"}
    forward_to             = [loki.write.local.receiver]
}

loki.write "local" {
    endpoint {
        url = "loki:3100/api/v1/push"
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.heroku` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
