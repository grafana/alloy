---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.heroku/
aliases:
  - ../loki.source.heroku/ # /docs/alloy/latest/reference/components/loki.source.heroku/
description: Learn about loki.source.heroku
title: loki.source.heroku
---

# loki.source.heroku

`loki.source.heroku` listens for Heroku messages over TCP connections
and forwards them to other `loki.*` components.

The component starts a new heroku listener for the given `listener`
block and fans out incoming entries to the list of receivers in `forward_to`.

Before using `loki.source.heroku`, Heroku should be configured with the URL where {{< param "PRODUCT_NAME" >}} will be listening.
Follow the steps in [Heroku HTTPS Drain docs](https://devcenter.heroku.com/articles/log-drains#https-drains) for using the Heroku CLI with a command like the following:

```shell
heroku drains:add [http|https]://HOSTNAME:PORT/heroku/api/v1/drain -a HEROKU_APP_NAME
```

Multiple `loki.source.heroku` components can be specified by giving them
different labels.

## Usage

```alloy
loki.source.heroku "LABEL" {
    http {
        listen_address = "LISTEN_ADDRESS"
        listen_port    = LISTEN_PORT
    }
    forward_to = RECEIVER_LIST
}
```

## Arguments

`loki.source.heroku` supports the following arguments:

Name                        | Type                 | Description                                                                        | Default | Required
----------------------------|----------------------|------------------------------------------------------------------------------------|---------|---------
`use_incoming_timestamp`    | `bool`               | Whether or not to use the timestamp received from Heroku.                          | `false` | no
`labels`                    | `map(string)`        | The labels to associate with each received Heroku record.                          | `{}`    | no
`forward_to`                | `list(LogsReceiver)` | List of receivers to send log entries to.                                          |         | yes
`relabel_rules`             | `RelabelRules`       | Relabeling rules to apply on log entries.                                          | `{}`    | no
`graceful_shutdown_timeout` | `duration`           | Timeout for servers graceful shutdown. If configured, should be greater than zero. | "30s"   | no

The `relabel_rules` field can make use of the `rules` export value from a
`loki.relabel` component to apply one or more relabeling rules to log entries
before they're forwarded to the list of receivers in `forward_to`.

## Blocks

The following blocks are supported inside the definition of `loki.source.heroku`:

Hierarchy | Name     | Description                                        | Required
----------|----------|----------------------------------------------------|---------
`http`    | [http][] | Configures the HTTP server that receives requests. | no
`grpc`    | [grpc][] | Configures the gRPC server that receives requests. | no

[http]: #http
[grpc]: #grpc

### http

{{< docs/shared lookup="reference/components/loki-server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

### grpc

{{< docs/shared lookup="reference/components/loki-server-grpc.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Labels

The `labels` map is applied to every message that the component reads.

The following internal labels all prefixed with `__` are available but will be discarded if not relabeled:
- `__heroku_drain_host`
- `__heroku_drain_app`
- `__heroku_drain_proc`
- `__heroku_drain_log_id`

All url query params will be translated to `__heroku_drain_param_<name>`

If the `X-Scope-OrgID` header is set it will be translated to `__tenant_id__`

## Exported fields

`loki.source.heroku` does not export any fields.

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
